//go:build go1.16

// Porter - single binary control plane.
// Subcommands: server (default), version, help.
package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"porter" // embedded web/dist assets (module root)
	"porter/internal/api"
	"porter/internal/cache"
	"porter/internal/config"
	cronrunner "porter/internal/cron"
	"porter/internal/dns"
	"porter/internal/event"
	"porter/internal/gateway"
	"porter/internal/health"
	"porter/internal/imagecatalog"
	"porter/internal/metrics"
	"porter/internal/netmgr"
	"porter/internal/notify"
	rt "porter/internal/runtime"
	"porter/internal/sshgw"
	"porter/internal/startup"
	"porter/internal/store"
	portertls "porter/internal/tls"
	"porter/internal/types"
	"porter/internal/volumes"
)

// Version is overridden at build time with -ldflags "-X main.Version=..."
var Version = "v1.0.0-beta"

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
	if len(args) > 0 && args[0] == "migrate" {
		os.Exit(runMigrate())
	}
	os.Exit(runServer(args))
}

// runMigrate applies all pending migrations, then seeds the minimum default
// data (the config-admin's default org) so the control plane is immediately
// usable. It does NOT start any VM engine or the HTTP listener.
func runMigrate() int {
	configPath := getenv("PORTER_CONFIG", "porter.toml")
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("config error: %v", err)
	}
	st := store.NewStore(cfg.DatabaseURL) // NewStore runs all pending Migrate calls
	defer st.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := st.EnsureDefaultOrg(ctx, "admin", "default"); err != nil {
		log.Fatalf("migrate: seed default org: %v", err)
	}
	log.Printf("migrate: schema up to date and default org seeded (db=%s)", cfg.DatabaseURL)
	return 0
}

func printUsage() {
	fmt.Print(`Porter - the self-hosted PaaS (Firecracker microVMs)

Run the app (control plane + dashboard):

  porter                      # start the API + embedded lifecycle workers

Utilities:
  porter migrate               # run pending DB migrations + seed default org
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

	// Startup sanity check: report direct Firecracker prerequisites before the
	// first VM boot instead of hiding host failures behind an API request.
	for _, c := range startup.Check(cfg) {
		status := "OK  "
		if !c.OK {
			status = "FAIL"
		}
		log.Printf("startup: [%s] %-18s %s", status, c.Name, c.Message)
		if !c.OK && c.Fatal {
			log.Fatalf("startup: fatal prerequisite missing: %s (%s)", c.Name, c.Message)
		}
	}

	st := store.NewStore(cfg.DatabaseURL)
	defer st.Close()
	if err := st.EnsureSeededAdmin(cfg.BootstrapAdminPassword); err != nil {
		log.Fatalf("auth bootstrap error: %v", err)
	}

	// Optional Redis read-through cache for hot read paths (config [cache]).
	if cfg.CacheEnabled && cfg.CacheURL != "" {
		cctx, ccancel := context.WithTimeout(context.Background(), 5*time.Second)
		rc, err := cache.Open(cctx, cfg.CacheURL)
		ccancel()
		if err != nil {
			log.Printf("cache: Redis disabled (%v)", err)
		} else {
			st.SetCache(rc)
			defer rc.Close()
			log.Printf("cache: Redis read-through cache enabled (%s)", cfg.CacheURL)
		}
	}
	hub := event.NewHub()

	vmm := newVMEngine(rt.FCConfig{
		Mode:           rt.Mode(cfg.RuntimeMode),
		FirecrackerBin: cfg.FirecrackerBin,
		KernelImage:    cfg.KernelImage,
		RootfsPath:     cfg.RootfsPath,
		SocketDir:      cfg.FirecrackerSocketDir,
		LogsDir:        cfg.LogsDir,
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
	if cfg.BaseImageRef != "" {
		baseName := strings.TrimPrefix(cfg.BaseImageRef, "base://")
		if baseName == "" {
			baseName = "default"
		}
		base := imagecatalog.ManifestFromArtifacts(baseName, cfg.BaseImageRef, "Configured Porter base microVM image", "base", cfg.RootfsPath, cfg.KernelImage, 1, 256)
		if err := st.PutGoldenImage(base); err != nil {
			log.Fatalf("base image registration error: %v", err)
		}
	}
	a := api.NewAPI(st, hub, vmm, netMgr, catalog, cfg.SecretKey, cfg.BaseDomain, Version)
	a.SetCustomImagesDir(cfg.CustomImagesDir)
	a.SetHostConfig(cfg)
	a.SetRateLimit(cfg.RateLimitPerMin)

	// Domain auto-assignment: create preview/prod domains for new projects.
	domainMgr := dns.NewDomainManager(st, cfg.BaseDomain, cfg.GatewayIP)
	a.SetDomainManager(domainMgr)

	// Real persistent volumes: host dirs + sparse backing images under volumes/.
	volMgr := volumes.NewManager(cfg.VolumesDir)
	_ = volMgr.EnsureRoot()
	a.SetVolumesManager(volMgr)

	// SMTP email notifications for alerts/events ([notify] config).
	a.SetMailer(notify.New(notify.SMTPConfig{
		Host: cfg.SMTPHost, Port: cfg.SMTPPort,
		User: cfg.SMTPUser, Password: cfg.SMTPPassword,
		From: cfg.SMTPFrom, DefaultTo: cfg.NotifyDefaultTo,
		Enabled: cfg.NotifyEnabled,
	}))

	// Cron scheduler: fires active crons on their 5-field schedule by booting
	// short-lived job microVMs through the same runtime as deploys.
	cronRunner := cronrunner.NewRunner(st, vmm, 30*time.Second)
	cronRunner.Start()
	defer cronRunner.Stop()

	// Metrics collector: samples CPU/memory for running VMs on an interval.
	metricsC := metrics.New(st, 30*time.Second)
	metricsC.Start()
	defer metricsC.Stop()

	// Horizontal autoscaler: adjusts replica pools per AutoscalePolicy.
	if cfg.AutoscaleEnabled {
		a.StartAutoscaler(30 * time.Second)
	}

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

	// Host-port forwarder: binds declared HostPorts (compose "8080:80") on the
	// host and proxies them to the running VM's container port. Independent of
	// the HTTP gateway — raw TCP forwarding for non-HTTP/protocol workloads.
	if cfg.GatewayEnabled {
		pf := gateway.NewPortForwarder(st)
		pf.Start()
		defer pf.Close()
		log.Printf("portforward: host-port forwarder started (binds compose HostPorts)")
	}

	// DNS server: authoritative resolver for *.baseDomain zones.
	// Listens on UDP/TCP port 53 and resolves queries to gateway IP.
	if cfg.DNSEnabled && cfg.BaseDomain != "" {
		gwIP := net.ParseIP(cfg.GatewayIP)
		if gwIP == nil {
			gwIP = net.ParseIP("127.0.0.1") // fallback
		}
		dnsSrv := dns.NewServer(st, cfg.BaseDomain, gwIP)
		if err := dnsSrv.Start(":53"); err != nil {
			log.Printf("dns: failed to start on :53: %v (may need root/cap_net_bind)", err)
		} else {
			log.Printf("dns: authoritative server started for *.%s", cfg.BaseDomain)
			defer dnsSrv.Shutdown()
		}
	}

	// TLS: automatic certificate management via Let's Encrypt ACME.
	// Certificates are cached on disk under certs/ (autocert.DirCache) and
	// renew automatically on demand.
	var tlsMgr *portertls.Manager
	if cfg.TLSEnabled && cfg.BaseDomain != "" && cfg.ACMEEmail != "" {
		tlsMgr = portertls.NewManager(cfg.BaseDomain, cfg.ACMEEmail, "certs")
		log.Printf("tls: ACME certificates enabled for *.%s (email: %s)", cfg.BaseDomain, cfg.ACMEEmail)
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

	// SSH gateway remains opt-in. Direct Firecracker VMs currently expose no
	// task.Exec bridge; a future guest-vsock agent can back this interface.
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

	// Enable TLS if configured.
	if tlsMgr != nil {
		srv.TLSConfig = tlsMgr.GetTLSConfig()
		// Wrap mux with ACME HTTP challenge handler
		srv.Handler = tlsMgr.HTTPHandler(mux)
		log.Printf("Porter %s — Control API listening on %s (HTTPS enabled)", Version, cfg.ListenAddr)
	} else {
		log.Printf("Porter %s — Control API listening on %s", Version, cfg.ListenAddr)
	}
	log.Printf("Dashboard: http://localhost%s", cfg.ListenAddr)
	log.Printf("Database: %s  Config: %s", cfg.DatabaseURL, configPath)
	st.AppendDaemonLog(fmt.Sprintf("=== Porter %s started  pid=%d  http://localhost%s ===", Version, os.Getpid(), cfg.ListenAddr))
	log.Printf("runtime: direct Firecracker over per-VM Unix sockets in %s", cfg.FirecrackerSocketDir)

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	serverErr := make(chan error, 1)
	go func() {
		if srv.TLSConfig != nil {
			// Use ListenAndServeTLS with empty cert/key since autocert handles them
			serverErr <- srv.ListenAndServeTLS("", "")
		} else {
			serverErr <- srv.ListenAndServe()
		}
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
// vmEngine adapts the direct-Firecracker runtime.VMManager to the API's
// VMRunner interface. It hands each VM a per-project /24 subnet + static IP,
// MAC, and host TAP device, then lets the runtime boot a kernel + rootfs.ext4.
// ----------------------------------------------------------------------
type vmEngine struct {
	rt   *rt.VMManager
	net  *netmgr.NetManager
	subs map[string]string // projectID -> allocated /24 subnet
}

func newVMEngine(cfg rt.FCConfig, st *store.Store, hub *event.Hub) *vmEngine {
	return &vmEngine{
		rt:   rt.NewVMManager(cfg, st, hub),
		net:  netmgr.NewNetManager(),
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
	spec, err := e.net.AllocateVMNetwork(subnet, vm.ReplicaIndex, vm.ID)
	if err != nil {
		return fmt.Errorf("boot: configure direct Firecracker network: %w", err)
	}
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

// Exec satisfies sshgw.Execer with the current direct-runtime limitation.
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
