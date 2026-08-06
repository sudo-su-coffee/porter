// Package config loads Porter's runtime configuration from a porter.toml
// file with PORTER_* environment-variable overrides layered on top. A
// missing file is not an error — env vars or built-in defaults can carry
// the whole config.
package config

import (
	"fmt"
	"os"
)

// Config holds every setting Porter needs to start: server/network
// settings, Firecracker paths, and the single prototype admin account.
type Config struct {
	ListenAddr string
	BaseDomain string
	StateFile  string // SQLite database file path
	APIToken   string

	KernelImage    string
	RootfsPath     string
	FirecrackerBin string
	ImagesDir      string // directory of vms/images/*.json image-catalog manifests

	AdminUsername string
	AdminPassword string
}

// LoadConfig reads the TOML file at path (if present) and layers
// PORTER_* environment variables on top of it. A missing file is not an
// error — env vars (or the built-in defaults below) can carry the whole
// config.
func LoadConfig(path string) (*Config, error) {
	cfg := &Config{
		ListenAddr:     ":8080",
		StateFile:      "porter.db",
		FirecrackerBin: "firecracker",
		ImagesDir:      "vms/images",
		AdminUsername:  "admin",
	}

	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		sections, perr := ParseTOML(string(data))
		if perr != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, perr)
		}
		cfg.ListenAddr = tomlGet(sections, "server", "listen_addr", cfg.ListenAddr)
		cfg.BaseDomain = tomlGet(sections, "server", "base_domain", cfg.BaseDomain)
		cfg.StateFile = tomlGet(sections, "server", "state_file", cfg.StateFile)
		cfg.APIToken = tomlGet(sections, "server", "api_token", cfg.APIToken)
		cfg.KernelImage = tomlGet(sections, "firecracker", "kernel_image", cfg.KernelImage)
		cfg.RootfsPath = tomlGet(sections, "firecracker", "rootfs_path", cfg.RootfsPath)
		cfg.FirecrackerBin = tomlGet(sections, "firecracker", "firecracker_bin", cfg.FirecrackerBin)
		cfg.ImagesDir = tomlGet(sections, "firecracker", "images_dir", cfg.ImagesDir)
		cfg.AdminUsername = tomlGet(sections, "admin", "username", cfg.AdminUsername)
		cfg.AdminPassword = tomlGet(sections, "admin", "password", cfg.AdminPassword)
	case os.IsNotExist(err):
		// No porter.toml — fine, env vars / defaults carry the config.
	default:
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	// Environment variables win when set — handy for secrets injected
	// by systemd/CI rather than written to disk.
	cfg.ListenAddr = envOr("PORTER_LISTEN_ADDR", cfg.ListenAddr)
	cfg.BaseDomain = envOr("PORTER_BASE_DOMAIN", cfg.BaseDomain)
	cfg.StateFile = envOr("PORTER_STATE_FILE", cfg.StateFile)
	cfg.APIToken = envOr("PORTER_API_TOKEN", cfg.APIToken)
	cfg.KernelImage = envOr("PORTER_KERNEL_IMAGE", cfg.KernelImage)
	cfg.RootfsPath = envOr("PORTER_ROOTFS_PATH", cfg.RootfsPath)
	cfg.FirecrackerBin = envOr("PORTER_FIRECRACKER_BIN", cfg.FirecrackerBin)
	cfg.ImagesDir = envOr("PORTER_IMAGES_DIR", cfg.ImagesDir)
	cfg.AdminUsername = envOr("PORTER_ADMIN_USERNAME", cfg.AdminUsername)
	cfg.AdminPassword = envOr("PORTER_ADMIN_PASSWORD", cfg.AdminPassword)

	if cfg.APIToken == "" {
		return nil, fmt.Errorf("no API token configured — set [server] api_token in %s or PORTER_API_TOKEN", path)
	}
	if cfg.AdminPassword == "" {
		return nil, fmt.Errorf("no admin password configured — set [admin] password in %s or PORTER_ADMIN_PASSWORD", path)
	}
	return cfg, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}