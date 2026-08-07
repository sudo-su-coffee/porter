//go:build go1.16

// Porter - single binary control plane.
// Subcommands: server (default), version, help.
package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"porter" // embedded web/dist assets (module root)
	fcnet "porter/internal/net"
	"porter/internal/api"
	"porter/internal/config"
	"porter/internal/dns"
	"porter/internal/event"
	"porter/internal/gateway"
	"porter/internal/health"
	"porter/internal/imagecatalog"
	"porter/internal/netmgr"
	rt "porter/internal/runtime"
	"porter/internal/sshgw"
	"porter/internal/store"
	"porter/internal/types"
)

// Version is overridden at build time with -ldflags "-X main.Version=..."
var Version = "v0.1.0-beta-dev"

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-v" {
			fmt.Println(Version)
			return
		}
	}
	// Dashboard-only: `porter` (no subcommand) runs the app. The only other
	// entrypoint is the version stamp.
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "server" {
		args = args[1:] // tolerate the old spelling
	}
	if len(args) > 0 && args[0] == "version" {
		fmt.Printf("Porter %s\n", Version)
		return
	}
	if len(args) > 0 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		printUsage()
		return
	}
	os.Exit(runServer(args))
}

func printUsage() {
	fmt.Print(`Porter - the self-hosted PaaS (Firecracker microVMs)

Run the app (control plane + dashboard):

  porter                      # start the API + embedded lifecycle workers

Utilities:
  porter version               # print the version

Config is read from $PORTER_CONFIG (default: porter.toml).

Everything else — deploy, manage VMs, traffic, teams — happens in the
dashboard at http://localhost:8080.
`)
}

// ----------------------------------------------------------------------
// Server subcommand
// ----------------------------------------------------------------------

func runServer(args []string) int {
	configPath := getenv("PORTER_CONFIG", "porter.toml")

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	st := store.NewStore(cfg.DatabaseURL)
	defer st.Close()
	hub := event.NewHub()

	vmm := newVMEngine(rt.FCConfig{
		ContainerdSocket: cfg.ContainerdSocket,
		Snapshotter:      cfg.Snapshotter,
		Namespace:        cfg.Namespace,
		LogsDir:          cfg.LogsDir,
		BareKernel:       cfg.KernelImage,
		FirecrackerBin:   cfg.FirecrackerBin,
	}, st, hub)
	defer vmm.Close()

	// Reconcile stale VMs from previous run.
	for _, vm := range st.ListVMs() {
		if vm.State == types.StateBooting || vm.State == types.StateRunning {
			vm.State = types.StateFailed
			vm.Error = "host restarted while this VM was up — press Start to retry"
			st.PutVM(vm)
		}
	}

	netMgr := netmgr.NewNetManager()
	catalog := imagecatalog.New(cfg.ImagesDir)
	a := api.NewAPI(st, hub, vmm, netMgr, catalog, cfg.APIToken, cfg.BaseDomain, cfg.AdminUsername, cfg.AdminPassword, Version)
	a.SetCustomImagesDir(cfg.CustomImagesDir)
	a.SetRateLimit(cfg.RateLimitPerMin)

	// Gateway: host-routing reverse proxy + live traffic logger on its own
	// listener, so the control plane (:8080) and the traffic-facing port stay
	// separate (Vercel-style: gateway faces *.local / project domains).
	if cfg.GatewayEnabled {
		gw := gateway.NewGateway(st)
		if cfg.DNSEnabled {
			gw.SetDNS(dns.New(st))
		}
		gsrv := &http.Server{
			Addr:              cfg.GatewayListenAddr,
			Handler:           gw,
			ReadHeaderTimeout: 15 * time.Second,
		}
		go func() {
			log.Printf("gateway: host-routing proxy listening on %s (control plane on %s)", cfg.GatewayListenAddr, cfg.ListenAddr)
			if err := gsrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("gateway server error: %v", err)
			}
		}()
	}

	// Health checker: watch running VMs that declare a healthcheck and
	// auto-replace unhealthy ones via the VM manager.
	if cfg.HealthEnabled {
		for _, vm := range st.ListVMs() {
			if vm.State != types.StateRunning || vm.Healthcheck == nil {
				continue
			}
			hc := vm.Healthcheck
			ck := health.New(st, hub, func(ctx context.Context, vmID string) {
				v, ok := st.GetVM(vmID)
				if !ok {
					return
				}
				if err := vmm.Restart(ctx, v); err != nil {
					log.Printf("health: replace vm %s: %v", vmID, err)
				}
			})
			go ck.Watch(context.Background(), vm.ID, health.HealthSpec{
				Type:        hc.Type,
				Path:        hc.Path,
				Port:        hc.Port,
				IntervalSec: hc.IntervalSec,
			})
			log.Printf("health: watching vm %s (%s)", vm.ID, vm.ServiceName)
		}
	}

	// SSH gateway: terminates SSH (short-lived CA-signed certs) and bridges
	// sessions into OCI VMs via containerd task.Exec (vmEngine.Exec). No sshd
	// inside the guest. Binds only when [ssh] enabled.
	if cfg.SSHEnabled {
		sg, err := sshgw.New(sshgw.Config{
			ListenAddr: cfg.SSHListenAddr,
			DataDir:    filepath.Join(cfg.LogsDir, "ssh"),
		}, vmm)
		if err != nil {
			log.Fatalf("[ssh] gateway init: %v", err)
		}
		go func() {
			log.Printf("sshgw: SSH gateway listening on %s", cfg.SSHListenAddr)
			if err := sg.ListenAndServe(context.Background()); err != nil {
				log.Printf("sshgw: %v", err)
			}
		}()
	}

	mux := http.NewServeMux()
	a.Routes(mux)
	// Embed dashboard if available.
	if sub, err := fs.Sub(assets.Dist, "web/dist"); err == nil {
		mux.Handle("/", http.FileServer(http.FS(sub)))
	} else {
		log.Printf("Dashboard assets not embedded; serving from ./web/dist if present")
		mux.Handle("/", http.FileServer(http.Dir("./web/dist")))
	}

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 15 * time.Second,
	}

	log.Printf("Porter %s — Control API listening on %s", Version, cfg.ListenAddr)
	log.Printf("Dashboard: http://localhost%s", cfg.ListenAddr)
	log.Printf("Database: %s  Config: %s", cfg.DatabaseURL, configPath)
	st.AppendDaemonLog(fmt.Sprintf("=== Porter %s started  pid=%d  http://localhost%s ===", Version, os.Getpid(), cfg.ListenAddr))
	if _, serr := os.Stat(cfg.ContainerdSocket); serr != nil {
		log.Printf("WARNING: containerd socket not found at %s (%v) — real VM boots will fail until containerd is running.", cfg.ContainerdSocket, serr)
	} else {
		log.Printf("containerd ready at %s (snapshotter=%s, namespace=%s).", cfg.ContainerdSocket, cfg.Snapshotter, cfg.Namespace)
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	case sig := <-shutdown:
		log.Printf("Received %s, shutting down...", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
	log.Printf("Shutdown complete")
	st.AppendDaemonLog(fmt.Sprintf("=== Porter %s stopped ===", Version))
	return 0
}

// ----------------------------------------------------------------------
// vmEngine — adapts the direct-Firecracker runtime.VMManager to the API's
// VMRunner interface. It hands each VM a per-project /24 subnet + static IP
// + MAC + host tap device, then lets the runtime boot it: bare (rootfs)
// images are booted by spawning `firecracker` directly; OCI images go
// through containerd (aws.firecracker shim); bare (rootfs) images boot a
// `firecracker` process directly.
// ----------------------------------------------------------------------
type vmEngine struct {
	rt   *rt.VMManager
	net  *fcnet.NetManager
	subs map[string]string // projectID -> allocated /24 subnet
}

func newVMEngine(cfg rt.FCConfig, st *store.Store, hub *event.Hub) *vmEngine {
	return &vmEngine{
		rt:   rt.NewVMManager(cfg, st, hub),
		net:  fcnet.NewNetManager(),
		subs: map[string]string{},
	}
}

func (e *vmEngine) Boot(ctx context.Context, vm *types.VM) error {
	if vm == nil {
		return fmt.Errorf("boot: nil vm")
	}
	subnet := e.subs[vm.ProjectID]
	if subnet == "" {
		subnet = e.net.AllocateProjectSubnet()
		e.subs[vm.ProjectID] = subnet
	}
	spec := e.net.AllocateVMNetwork(subnet, vm.ReplicaIndex, vm.ID)
	e.rt.Boot(vm, spec)
	return nil
}

func (e *vmEngine) Stop(ctx context.Context, vm *types.VM) error {
	if vm == nil {
		return fmt.Errorf("stop: nil vm")
	}
	e.rt.Stop(vm)
	return nil
}

func (e *vmEngine) Restart(ctx context.Context, vm *types.VM) error {
	if err := e.Stop(ctx, vm); err != nil {
		return err
	}
	return e.Boot(ctx, vm)
}

func (e *vmEngine) Delete(ctx context.Context, vm *types.VM) error {
	return e.Stop(ctx, vm)
}

// Exec satisfies sshgw.Execer: bridge an SSH session into a containerd-booted
// VM's task (OCI images). Bare (direct-Firecracker) VMs return an error.
func (e *vmEngine) Exec(ctx context.Context, vmID string, stdin, stdout interface{}) error {
	return e.rt.Exec(ctx, vmID, stdin, stdout)
}

func (e *vmEngine) Close() { e.rt.Close() }

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}