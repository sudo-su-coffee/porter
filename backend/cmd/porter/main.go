//go:build go1.16

// Porter - single binary control plane.
// All subcommands (server, worker, kernel, version, help) are defined here.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
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
	"porter/internal/config"
	"porter/internal/event"
	"porter/internal/imagecatalog"
	"porter/internal/netmgr"
	"porter/internal/store"
	"porter/internal/types"
	"porter/internal/vmmanager"
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
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) < 1 {
		printUsage()
		return 1
	}
	switch args[0] {
	case "server":
		return runServer(args[1:])
	case "worker":
		return runWorker(args[1:])
	case "kernel":
		return runKernel(args[1:])
	case "version":
		fmt.Printf("Porter %s\n", Version)
		return 0
	case "help", "-h", "--help":
		printUsage()
		return 0
	default:
		fmt.Printf("Unknown command: %s\n\n", args[0])
		printUsage()
		return 1
	}
}

func printUsage() {
	fmt.Print(`Porter - the self-hosted PaaS (Firecracker microVMs)

Usage:
  porter <command> [options]

Commands:
  server    Start the API server (with optional embedded lifecycle workers)
  worker    Run lifecycle workers only (no HTTP server)
  kernel    Set the shared vmlinux kernel (local path or https:// URL)
  version   Print the version
  help      Show this help

Server options:
  -config string    Config file path (default "porter.toml", or $PORTER_CONFIG)
  -workers int      Number of embedded lifecycle workers (0 to disable) (default 1)

Worker options:
  -config string    Config file path (default "porter.toml", or $PORTER_CONFIG)
  -workers int      Number of workers to run (default 1)

Kernel options:
  -dest string   Destination file (default vms/vmlinux, or $PORTER_KERNEL_IMAGE)

Examples:
  porter server                  # API + 1 embedded worker
  porter server -workers 0       # API only (no workers)
  porter worker -workers 4       # 4 workers only (no API)
  porter kernel set ./vmlinux-5.10
  porter kernel set https://example.com/vmlinux
`)
}

// ----------------------------------------------------------------------
// Server subcommand
// ----------------------------------------------------------------------

func runServer(args []string) int {
	flags := flag.NewFlagSet("server", flag.ExitOnError)
	configPath := flags.String("config", getenv("PORTER_CONFIG", "porter.toml"), "Config file path")
	numWorkers := flags.Int("workers", 1, "Number of embedded lifecycle workers (0 to disable)")
	_ = flags.Parse(args)

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	st := store.NewStore(cfg.DatabaseURL)
	defer st.Close()
	hub := event.NewHub()

	vmm := vmmanager.New(vmmanager.Config{
		ContainerdSocket: cfg.ContainerdSocket,
		Snapshotter:      cfg.Snapshotter,
		Namespace:        cfg.Namespace,
		KernelImage:      cfg.KernelImage,
		LogsDir:          cfg.LogsDir,
		Simulate:         cfg.Simulate,
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
	log.Printf("Database: %s  Config: %s", cfg.DatabaseURL, *configPath)
	log.Printf("Embedded workers: %d", *numWorkers)
	st.AppendDaemonLog(fmt.Sprintf("=== Porter %s started  pid=%d  http://localhost%s ===", Version, os.Getpid(), cfg.ListenAddr))
	if cfg.Simulate {
		log.Printf("SIMULATE mode: on — no containerd needed; deployments are simulated for dev/demo.")
	} else if _, serr := os.Stat(cfg.ContainerdSocket); serr != nil {
		log.Printf("WARNING: containerd socket not found at %s (%v) — real VM boots will fail.", cfg.ContainerdSocket, serr)
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
// Worker subcommand
// ----------------------------------------------------------------------

func runWorker(args []string) int {
	flags := flag.NewFlagSet("worker", flag.ExitOnError)
	configPath := flags.String("config", getenv("PORTER_CONFIG", "porter.toml"), "Config file path")
	workerCount := flags.Int("workers", 1, "Number of workers to run")
	_ = flags.Parse(args)

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	st := store.NewStore(cfg.DatabaseURL)
	defer st.Close()
	hub := event.NewHub()
	vmm := vmmanager.New(vmmanager.Config{
		ContainerdSocket: cfg.ContainerdSocket,
		Snapshotter:      cfg.Snapshotter,
		Namespace:        cfg.Namespace,
		KernelImage:      cfg.KernelImage,
		LogsDir:          cfg.LogsDir,
		Simulate:         cfg.Simulate,
	}, st, hub)
	defer vmm.Close()

	log.Printf("Porter worker %s — %d worker(s)", Version, *workerCount)

	// Health sweep: periodically verify running VMs.
	sweep := func() {
		for _, vm := range st.ListVMs() {
			if vm.State != types.StateRunning || vm.Healthcheck == nil || vm.IPAddress == "" {
				continue
			}
			if !tcpReachable(vm.IPAddress, vm.Healthcheck.Port) {
				vm.HealthStatus = types.HealthUnhealthy
				st.PutVM(vm)
			}
		}
	}
	sweep()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case <-ticker.C:
			sweep()
		case <-shutdown:
			log.Printf("Worker shutting down")
			return 0
		}
	}
}

func tcpReachable(host string, port int) bool {
	if port <= 0 {
		return true
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// ----------------------------------------------------------------------
// Kernel subcommand
// ----------------------------------------------------------------------

func runKernel(args []string) int {
	sub := flag.NewFlagSet("kernel", flag.ExitOnError)
	dest := sub.String("dest", getenv("PORTER_KERNEL_IMAGE", "vms/vmlinux"), "Destination file to write to")
	_ = sub.Parse(args)
	rest := sub.Args()
	if len(rest) < 2 || rest[0] != "set" {
		fmt.Println("usage: porter kernel set <local-path|https://url> [-dest file]")
		return 1
	}
	src := rest[1]

	if err := os.MkdirAll(filepath.Dir(*dest), 0o755); err != nil {
		log.Fatalf("kernel: mkdir %s: %v", filepath.Dir(*dest), err)
	}

	switch {
	case strings.HasPrefix(src, "https://"), strings.HasPrefix(src, "http://"):
		log.Printf("kernel: downloading %s -> %s", src, *dest)
		if err := downloadFile(*dest, src); err != nil {
			log.Fatalf("kernel: download: %v", err)
		}
	default:
		log.Printf("kernel: copying %s -> %s", src, *dest)
		if err := copyFile(*dest, src); err != nil {
			log.Fatalf("kernel: copy: %v", err)
		}
	}

	fmt.Printf("Porter kernel set -> %s\n", *dest)
	fmt.Printf("Point [firecracker] kernel_image at it (or set PORTER_KERNEL_IMAGE) and start `porter server`.\n")
	return 0
}

func downloadFile(dst, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}

func copyFile(dst, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}