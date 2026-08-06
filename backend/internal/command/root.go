// Package command implements the Porter CLI entrypoints: `server`,
// `worker`, `version`, and `help`. It follows the pattern of a single
// binary with subcommands, wiring up the config -> store -> event hub ->
// VM manager -> API dependency chain in one place.
package command

import (
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"

	"porter/assets"
	"porter/internal/api"
	"porter/internal/config"
	"porter/internal/event"
	netmgr "porter/internal/net"
	"porter/internal/runtime"
	stores "porter/internal/store"
)

// Run dispatches on the first non-flag argument and executes the matching
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
  version   Print the version
  help      Show this help

Server options:
  -config string    Config file path (default "porter.toml", or $PORTER_CONFIG)
  -workers int      Number of embedded lifecycle workers (0 to disable) (default 1)

Worker options:
  -config string    Config file path (default "porter.toml", or $PORTER_CONFIG)
  -workers int      Number of workers to run (default 1)

Examples:
  porter server
  porter server -workers 0
  porter worker -workers 2
`)
}

// runServer starts the control-plane API, embedding the built dashboard
// and optionally running in-process lifecycle workers.
func runServer(args []string, version string) int {
	cfgPath := getenv("PORTER_CONFIG", "porter.toml")
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	st := store.NewStore(cfg.StateFile)
	hub := event.NewHub()

	vmm, err := runtime.NewVMManager(runtime.FCConfig{
		KernelImagePath: cfg.KernelImage,
		RootfsPath:      cfg.RootfsPath,
		FirecrackerBin:  cfg.FirecrackerBin,
	}, st, hub)
	if err != nil {
		log.Fatalf("failed to initialize VM manager: %v", err)
	}
	defer vmm.Close()

	netMgr := netmgr.NewNetManager()
	a := api.NewAPI(st, hub, vmm, netMgr, cfg.APIToken, cfg.BaseDomain, cfg.AdminUsername, cfg.AdminPassword, version)

	mux := http.NewServeMux()
	a.Routes(mux)

	sub, err := fs.Sub(assets.Dist, "web/dist")
	if err == nil {
		mux.Handle("/", http.FileServer(http.FS(sub)))
	}

	log.Printf("Porter %s — Control API listening on %s", version, cfg.ListenAddr)
	log.Printf("Dashboard: http://localhost%s", cfg.ListenAddr)
	log.Printf("State: %s (sqlite)  Config: %s", cfg.StateFile, cfgPath)
	log.Fatal(http.ListenAndServe(cfg.ListenAddr, mux))
	return 0
}

// runWorker runs background lifecycle jobs with no HTTP server. For v0.1.0
// this is a thin placeholder; the actual worker jobs (pending-boot queue,
// healthcheck sweep) are populated as part of the firecracker-containerd
// VM manager work.
func runWorker(args []string, version string) int {
	log.Printf("Porter worker %s — no background jobs registered yet", version)
	return 0
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}