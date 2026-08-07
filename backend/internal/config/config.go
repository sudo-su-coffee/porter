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
	APIToken   string

	// PostgreSQL state store. DatabaseURL is required — LoadConfig refuses to
	// start without it. AutoMigrate runs pending SQL migrations at startup.
	DatabaseURL string
	AutoMigrate bool

	// Linux-host VM wiring. Porter boots OCI images through containerd +
	// the `aws.firecracker` shim; kernel/rootfs/jailer live in the host's
	// /etc/containerd/firecracker-runtime.json (see deploy/host/).
	ContainerdSocket string // e.g. /run/containerd/containerd.sock
	Snapshotter      string // containerd snapshotter to pull/unpack into (devmapper on a real host)
	Namespace        string // containerd namespace, e.g. "porter"
	LogsDir          string // where per-VM stdio logs land, e.g. /var/log/porter
	Simulate         bool   // fake the lifecycle for dev/demo (no containerd needed)

	// Kernel/rootfs/firecracker are consumed by the host (the shim's
	// /etc/containerd/firecracker-runtime.json), not by Porter at runtime;
	// kept for `porter kernel set` and documentation. The containerd fields
	// above are what Porter actually uses.
	KernelImage    string // shared vmlinux for the shim (provision with `porter kernel set`)
	RootfsPath     string
	FirecrackerBin string

	ImagesDir string // directory of vms/images/*.json image-catalog manifests

	AdminUsername string
	AdminPassword string
}

// LoadConfig reads the TOML file at path (if present) and layers
// PORTER_* environment variables on top of it. A missing file is not an
// error — env vars (or the built-in defaults below) can carry the whole
// config.
func LoadConfig(path string) (*Config, error) {
	cfg := &Config{
		ListenAddr:       ":8080",
		DatabaseURL:      "postgres://porter:porter@localhost:5432/porter?sslmode=disable",
		AutoMigrate:      true,
		ContainerdSocket: "/run/containerd/containerd.sock",
		Snapshotter:      "devmapper",
		Namespace:        "porter",
		LogsDir:          "/var/log/porter",
		ImagesDir:        "vms/images",
		AdminUsername:    "admin",
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
		cfg.APIToken = tomlGet(sections, "server", "api_token", cfg.APIToken)
		cfg.DatabaseURL = tomlGet(sections, "database", "url", cfg.DatabaseURL)
		cfg.AutoMigrate = tomlBool(sections, "database", "auto_migrate", cfg.AutoMigrate)
		cfg.KernelImage = tomlGet(sections, "firecracker", "kernel_image", cfg.KernelImage)
		cfg.RootfsPath = tomlGet(sections, "firecracker", "rootfs_path", cfg.RootfsPath)
		cfg.FirecrackerBin = tomlGet(sections, "firecracker", "firecracker_bin", cfg.FirecrackerBin)
		cfg.ContainerdSocket = tomlGet(sections, "firecracker", "containerd_socket", cfg.ContainerdSocket)
		cfg.Snapshotter = tomlGet(sections, "firecracker", "snapshotter", cfg.Snapshotter)
		cfg.Namespace = tomlGet(sections, "firecracker", "namespace", cfg.Namespace)
		cfg.LogsDir = tomlGet(sections, "firecracker", "logs_dir", cfg.LogsDir)
		cfg.Simulate = tomlBool(sections, "firecracker", "simulate", cfg.Simulate)
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
	cfg.APIToken = envOr("PORTER_API_TOKEN", cfg.APIToken)
	cfg.DatabaseURL = envOr("PORTER_DATABASE_URL", cfg.DatabaseURL)
	cfg.AutoMigrate = envBool("PORTER_AUTO_MIGRATE", cfg.AutoMigrate)
	cfg.KernelImage = envOr("PORTER_KERNEL_IMAGE", cfg.KernelImage)
	cfg.RootfsPath = envOr("PORTER_ROOTFS_PATH", cfg.RootfsPath)
	cfg.FirecrackerBin = envOr("PORTER_FIRECRACKER_BIN", cfg.FirecrackerBin)
	cfg.LogsDir = envOr("PORTER_LOGS_DIR", cfg.LogsDir)
	cfg.Simulate = envBool("PORTER_SIMULATE", cfg.Simulate)
	cfg.ImagesDir = envOr("PORTER_IMAGES_DIR", cfg.ImagesDir)
	cfg.AdminUsername = envOr("PORTER_ADMIN_USERNAME", cfg.AdminUsername)
	cfg.AdminPassword = envOr("PORTER_ADMIN_PASSWORD", cfg.AdminPassword)

	if cfg.APIToken == "" {
		return nil, fmt.Errorf("no API token configured — set [server] api_token in %s or PORTER_API_TOKEN", path)
	}
	if cfg.AdminPassword == "" {
		return nil, fmt.Errorf("no admin password configured — set [admin] password in %s or PORTER_ADMIN_PASSWORD", path)
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("no database url configured — set [database] url in %s or PORTER_DATABASE_URL", path)
	}
	return cfg, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// envBool parses a PORTER_* boolean (true/1/yes) with a default.
func envBool(key string, def bool) bool {
	switch os.Getenv(key) {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	}
	return def
}

// tomlBool reads a [section] key as a boolean with a default.
func tomlBool(sections map[string]map[string]string, section, key string, def bool) bool {
	v := tomlGet(sections, section, key, "")
	if v == "" {
		return def
	}
	switch v {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	}
	return def
}