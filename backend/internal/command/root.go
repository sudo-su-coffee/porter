// Package command implements the Porter CLI entrypoints: `server`,
// `worker`, `version`, and `help`. It follows the pattern of a single
// binary with subcommands: main dispatches on os.Args[1], each subcommand
// parses its own flags, and the server runs an HTTP listener that shuts
// down gracefully on SIGINT/SIGTERM.
package command

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"porter" // package assets — the embedded web/dist lives at the module root
	"porter/internal/api"
	"porter/internal/config"
	"porter/internal/event"
	"porter/internal/imagecatalog"
	netmgr "porter/internal/net"
	"porter/internal/runtime"
	"porter/internal/store"
)

// Run dispatches on the first argument and executes the matching
// subcommand. It returns a process exit code.
func Run(args []string, version string) int {
	if len(args) < 1 {
		printUsage()
		return 1
	}
	switch args[0] {
	case "server":
		return runServer(args[1:], version)
	case "worker":
		return runWorker(args[1:], version)
	case "kernel":
		return runKernel(args[1:], version)
	case "version":
		fmt.Printf("Porter %s\n", version)
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

Kernel options:
  -dest string   Destination file (default vms/vmlinux, or $PORTER_KERNEL_IMAGE)

Examples:
  porter kernel set ./vmlinux-5.10
  porter kernel set https://example.com/vmlinux


Server options:
  -config string    Config file path (default "porter.toml", or $PORTER_CONFIG)
  -workers int      Number of embedded lifecycle workers (0 to disable) (default 1)

Worker options:
  -config string    Config file path (default "porter.toml", or $PORTER_CONFIG)
  -workers int      Number of workers to run (default 1)

Examples:
  porter server                  # API + 1 embedded worker
  porter server -workers 0       # API only (no workers)
  porter server -workers 4       # API + 4 embedded workers
  porter worker -workers 4       # 4 workers only (no API)
`)
}

// runServer starts the control-plane Gateway, embedding the built
// dashboard and optionally running in-process lifecycle workers. It
// serves until SIGINT/SIGTERM, then tears down gracefully.
func runServer(args []string, version string) int {
	flags := flag.NewFlagSet("server", flag.ExitOnError)
	configPath := flags.String("config", getenv("PORTER_CONFIG", "porter.toml"), "Config file path")
	numWorkers := flags.Int("workers", 1, "Number of embedded lifecycle workers (0 to disable)")
	_ = flags.Parse(args)

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	st := store.NewStore(cfg.StateFile)
	hub := event.NewHub()

	vmm := runtime.NewVMManager(runtime.FCConfig{
		ContainerdSocket: cfg.ContainerdSocket,
		Snapshotter:      cfg.Snapshotter,
		Namespace:        cfg.Namespace,
		LogsDir:          cfg.LogsDir,
	}, st, hub)
	defer vmm.Close()

	netMgr := netmgr.NewNetManager()
	catalog := imagecatalog.New(cfg.ImagesDir)
	a := api.NewAPI(st, hub, vmm, netMgr, catalog, cfg.APIToken, cfg.BaseDomain, cfg.AdminUsername, cfg.AdminPassword, version)

	mux := http.NewServeMux()
	a.Routes(mux)
	if sub, err := fs.Sub(assets.Dist, "web/dist"); err == nil {
		mux.Handle("/", http.FileServer(http.FS(sub)))
	}

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 15 * time.Second,
	}

	log.Printf("Porter %s — Control API listening on %s", version, cfg.ListenAddr)
	log.Printf("Dashboard: http://localhost%s", cfg.ListenAddr)
	log.Printf("State: %s (sqlite)  Config: %s", cfg.StateFile, *configPath)
	log.Printf("Embedded workers: %d", *numWorkers)

	// Graceful shutdown on SIGINT/SIGTERM: stop workers first, then the HTTP
	// server, then the process-owned side effects (VM manager, store).
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.ListenAndServe()
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
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
	log.Printf("Shutdown complete")
	return 0
}

// runWorker runs background lifecycle jobs with no HTTP server. For
// v0.1.0 the worker side is a placeholder; the pending-boot queue and
// healthcheck sweep are populated alongside the containerd VM manager.
func runWorker(args []string, version string) int {
	flags := flag.NewFlagSet("worker", flag.ExitOnError)
	configPath := flags.String("config", getenv("PORTER_CONFIG", "porter.toml"), "Config file path")
	workerCount := flags.Int("workers", 1, "Number of workers to run")
	_ = flags.Parse(args)

	if _, err := config.LoadConfig(*configPath); err != nil {
		log.Fatalf("config error: %v", err)
	}
	log.Printf("Porter worker %s — %d worker(s), no background jobs registered yet", version, *workerCount)

	// Keep running until signalled to stop.
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	<-shutdown
	log.Printf("Worker shutting down")
	return 0
}

// runKernel provisions the shared Firecracker kernel (vmlinux) from
// either a local file path or a remote https:// URL. `porter kernel set
// ./vmlinux-5.10` copies a local file; `porter kernel set
// https://…/vmlinux` downloads it. The destination defaults to
// vms/vmlinux (override with -dest or PORTER_KERNEL_IMAGE).
func runKernel(args []string, version string) int {
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

// downloadFile streams a URL to dst.
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

// copyFile copies src to dst.
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