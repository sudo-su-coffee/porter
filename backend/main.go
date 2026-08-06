package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
)

//go:embed web/dist
var webDist embed.FS

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-v" {
			fmt.Println(Version)
			return
		}
	}

	cfgPath := getenv("PORTER_CONFIG", "porter.toml")
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	fcCfg := FCConfig{
		KernelImagePath: cfg.KernelImage,
		RootfsPath:      cfg.RootfsPath,
		FirecrackerBin:  cfg.FirecrackerBin,
	}

	store := NewStore(cfg.StateFile)
	hub := NewHub()

	vmm, err := NewVMManager(fcCfg, store, hub)
	if err != nil {
		log.Fatalf("failed to initialize VM manager: %v", err)
	}
	defer vmm.Close()

	netMgr := NewNetManager()
	api := NewAPI(store, hub, vmm, netMgr, cfg.APIToken, cfg.BaseDomain, cfg.AdminUsername, cfg.AdminPassword)

	mux := http.NewServeMux()
	api.Routes(mux)

	sub, err := fs.Sub(webDist, "web/dist")
	if err == nil {
		mux.Handle("/", http.FileServer(http.FS(sub)))
	}

	log.Printf("Porter %s — Control API listening on %s", Version, cfg.ListenAddr)
	log.Printf("Dashboard: http://localhost%s", cfg.ListenAddr)
	log.Printf("State: %s (sqlite)  Config: %s", cfg.StateFile, cfgPath)
	log.Fatal(http.ListenAndServe(cfg.ListenAddr, mux))
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
