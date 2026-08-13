// Package config loads Porter's runtime configuration from a porter.toml
// file with PORTER_* environment-variable overrides layered on top. A
// missing file is not an error — env vars or built-in defaults can carry
// the whole config.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds every setting Porter needs to start: server/network settings,
// Firecracker paths, and a one-time database bootstrap secret.
type Config struct {
	ListenAddr string
	BaseDomain string
	SecretKey  string // environment-only key material for project-secret encryption

	// PostgreSQL state store. DatabaseURL is required — LoadConfig refuses to
	// start without it. AutoMigrate runs pending SQL migrations at startup.
	DatabaseURL string
	AutoMigrate bool

	// Linux-host VM wiring for direct Firecracker. Each VM receives a kernel,
	// rootfs.ext4, TAP device, and private API Unix socket.
	RuntimeMode          string // direct only
	BaseImageRef         string // canonical default, e.g. base://default
	FirecrackerSocketDir string // per-VM API sockets, e.g. /run/porter/firecracker
	LogsDir              string // per-VM Firecracker stdout/stderr logs

	KernelImage    string // shared vmlinux (provision with `porter kernel set`)
	RootfsPath     string
	FirecrackerBin string
	JailerBin      string // optional: jailer binary for the startup sanity check

	ImagesDir string // directory of vms/images/*.json image-catalog manifests

	// Direct-Firecracker (bare microVM) settings. CustomImagesDir is where
	// user-uploaded .zip microVM images (rootfs.ext4 + vmlinux) are unpacked.
	CustomImagesDir string // e.g. "vms/custom"

	// RateLimitPerMin caps control-plane requests per client IP per minute
	// (0 disables). Applied to auth'd routes as a token bucket.
	RateLimitPerMin int

	// Optional control-plane services wired into `porter server`. All default
	// to off so a bare bootstrap keeps working with zero extra listeners.
	GatewayEnabled    bool   // host-routing reverse proxy + traffic logger
	GatewayListenAddr string // e.g. ":80" — faces *.local / project domains
	DNSEnabled        bool   // resolve <svc>.<project>.local for the gateway
	HealthEnabled     bool   // healthcheck + auto-replace of unhealthy VMs
	SSHEnabled        bool   // SSH gateway (needs a task.Exec bridge; off by default)
	SSHListenAddr     string

	// Optional Redis read-through cache. Disabled by default — every cache
	// call is a no-op and the store never touches Redis. When enabled, hot
	// read paths (projects, VMs/replicas, orgs, domains, deployments, builds,
	// image library) are served from Redis for CacheTTLSeconds before falling
	// through to Postgres.
	CacheEnabled    bool
	CacheURL        string // e.g. redis://localhost:6379/0
	CacheTTLSeconds int    // seconds cached reads stay fresh (<=0 keeps default)

	// TLS/ACME: automatic Let's Encrypt certificates.
	TLSEnabled bool   // enable HTTPS via ACME
	ACMEEmail  string // email for Let's Encrypt account
	GatewayIP  string // IP for DNS A records (e.g., public IP of this host)

	// VolumesDir is where real persistent volume data lives (one dir per volume
	// with a sparse data.img of the requested size). Defaults to ./volumes.
	VolumesDir string

	// AutoscaleEnabled turns on the horizontal autoscaler (polls project load
	// and adjusts replica pools per each project's AutoscalePolicy).
	AutoscaleEnabled bool

	// SMTP email notifications ([notify]) for alerts and operational events.
	NotifyEnabled   bool   // master switch; when false Send is a no-op
	SMTPHost        string // e.g. smtp.example.com
	SMTPPort        int    // e.g. 587
	SMTPUser        string
	SMTPPassword    string
	SMTPFrom        string // From address
	NotifyDefaultTo string // recipients when an alert has no destination

	// BootstrapAdminPassword is consumed only when the migration-seeded admin
	// row has no password hash. It is never written to TOML or used as a
	// privileged authorization fallback.
	BootstrapAdminPassword string
}

// LoadConfig reads the TOML file at path (if present) and layers
// PORTER_* environment variables on top of it. A missing file is not an
// error — env vars (or the built-in defaults below) can carry the whole
// config.
func LoadConfig(path string) (*Config, error) {
	cfg := &Config{
		ListenAddr:           ":8080",
		DatabaseURL:          "postgres://porter:porter@localhost:5432/porter?sslmode=disable",
		AutoMigrate:          true,
		RuntimeMode:          "direct",
		BaseImageRef:         "base://default",
		FirecrackerSocketDir: "/run/porter/firecracker",
		LogsDir:              "/var/log/porter",
		ImagesDir:            "vms/images",
		CustomImagesDir:      "vms/custom",
		FirecrackerBin:       "firecracker",
		GatewayListenAddr:    ":80",
		SSHListenAddr:        ":2222",
		CacheURL:             "redis://localhost:6379/0",
		CacheTTLSeconds:      15,
		VolumesDir:           "volumes",
		SMTPPort:             587,
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
		cfg.DatabaseURL = tomlGet(sections, "database", "url", cfg.DatabaseURL)
		cfg.AutoMigrate = tomlBool(sections, "database", "auto_migrate", cfg.AutoMigrate)
		cfg.KernelImage = tomlGet(sections, "firecracker", "kernel_image", cfg.KernelImage)
		cfg.RuntimeMode = tomlGet(sections, "firecracker", "runtime_mode", cfg.RuntimeMode)
		cfg.BaseImageRef = tomlGet(sections, "firecracker", "base_image_ref", cfg.BaseImageRef)
		cfg.RootfsPath = tomlGet(sections, "firecracker", "rootfs_path", cfg.RootfsPath)
		cfg.FirecrackerBin = tomlGet(sections, "firecracker", "firecracker_bin", cfg.FirecrackerBin)
		cfg.FirecrackerSocketDir = tomlGet(sections, "firecracker", "api_socket_dir", cfg.FirecrackerSocketDir)
		cfg.LogsDir = tomlGet(sections, "firecracker", "logs_dir", cfg.LogsDir)
		cfg.ImagesDir = tomlGet(sections, "firecracker", "images_dir", cfg.ImagesDir)
		cfg.CustomImagesDir = tomlGet(sections, "firecracker", "custom_images_dir", cfg.CustomImagesDir)
		cfg.FirecrackerBin = tomlGet(sections, "firecracker", "firecracker_bin", cfg.FirecrackerBin)
		cfg.JailerBin = tomlGet(sections, "firecracker", "jailer_bin", cfg.JailerBin)
		cfg.RateLimitPerMin = tomlInt(sections, "server", "rate_limit_per_min", cfg.RateLimitPerMin)
		cfg.GatewayEnabled = tomlBool(sections, "gateway", "enabled", cfg.GatewayEnabled)
		cfg.GatewayListenAddr = tomlGet(sections, "gateway", "listen_addr", cfg.GatewayListenAddr)
		cfg.DNSEnabled = tomlBool(sections, "dns", "enabled", cfg.DNSEnabled)
		cfg.HealthEnabled = tomlBool(sections, "health", "enabled", cfg.HealthEnabled)
		cfg.SSHEnabled = tomlBool(sections, "ssh", "enabled", cfg.SSHEnabled)
		cfg.SSHListenAddr = tomlGet(sections, "ssh", "listen_addr", cfg.SSHListenAddr)
		cfg.TLSEnabled = tomlBool(sections, "tls", "enabled", cfg.TLSEnabled)
		cfg.ACMEEmail = tomlGet(sections, "tls", "acme_email", cfg.ACMEEmail)
		cfg.GatewayIP = tomlGet(sections, "dns", "gateway_ip", cfg.GatewayIP)
		cfg.VolumesDir = tomlGet(sections, "server", "volumes_dir", cfg.VolumesDir)
		cfg.AutoscaleEnabled = tomlBool(sections, "autoscale", "enabled", cfg.AutoscaleEnabled)
		cfg.CacheEnabled = tomlBool(sections, "cache", "enabled", cfg.CacheEnabled)
		cfg.CacheURL = tomlGet(sections, "cache", "url", cfg.CacheURL)
		cfg.CacheTTLSeconds = tomlInt(sections, "cache", "ttl_seconds", cfg.CacheTTLSeconds)
		cfg.NotifyEnabled = tomlBool(sections, "notify", "enabled", cfg.NotifyEnabled)
		cfg.SMTPHost = tomlGet(sections, "notify", "smtp_host", cfg.SMTPHost)
		cfg.SMTPPort = tomlInt(sections, "notify", "smtp_port", cfg.SMTPPort)
		cfg.SMTPUser = tomlGet(sections, "notify", "smtp_user", cfg.SMTPUser)
		cfg.SMTPPassword = tomlGet(sections, "notify", "smtp_password", cfg.SMTPPassword)
		cfg.SMTPFrom = tomlGet(sections, "notify", "from", cfg.SMTPFrom)
		cfg.NotifyDefaultTo = tomlGet(sections, "notify", "default_to", cfg.NotifyDefaultTo)
	case os.IsNotExist(err):
		// No porter.toml — fine, env vars / defaults carry the config.
	default:
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	// Environment variables win when set — handy for secrets injected
	// by systemd/CI rather than written to disk.
	cfg.ListenAddr = envOr("PORTER_LISTEN_ADDR", cfg.ListenAddr)
	cfg.BaseDomain = envOr("PORTER_BASE_DOMAIN", cfg.BaseDomain)
	cfg.SecretKey = envOr("PORTER_SECRET_KEY", cfg.SecretKey)
	cfg.DatabaseURL = envOr("PORTER_DATABASE_URL", cfg.DatabaseURL)
	cfg.AutoMigrate = envBool("PORTER_AUTO_MIGRATE", cfg.AutoMigrate)
	cfg.RuntimeMode = envOr("PORTER_RUNTIME_MODE", cfg.RuntimeMode)
	cfg.BaseImageRef = envOr("PORTER_BASE_IMAGE_REF", cfg.BaseImageRef)
	cfg.FirecrackerSocketDir = envOr("PORTER_FIRECRACKER_API_SOCKET_DIR", cfg.FirecrackerSocketDir)
	cfg.KernelImage = envOr("PORTER_KERNEL_IMAGE", cfg.KernelImage)
	cfg.RootfsPath = envOr("PORTER_ROOTFS_PATH", cfg.RootfsPath)
	cfg.FirecrackerBin = envOr("PORTER_FIRECRACKER_BIN", cfg.FirecrackerBin)
	cfg.LogsDir = envOr("PORTER_LOGS_DIR", cfg.LogsDir)
	cfg.ImagesDir = envOr("PORTER_IMAGES_DIR", cfg.ImagesDir)
	cfg.CustomImagesDir = envOr("PORTER_CUSTOM_IMAGES_DIR", cfg.CustomImagesDir)
	cfg.FirecrackerBin = envOr("PORTER_FIRECRACKER_BIN", cfg.FirecrackerBin)
	cfg.RateLimitPerMin = envInt("PORTER_RATE_LIMIT_PER_MIN", cfg.RateLimitPerMin)
	cfg.GatewayEnabled = envBool("PORTER_GATEWAY_ENABLED", cfg.GatewayEnabled)
	cfg.GatewayListenAddr = envOr("PORTER_GATEWAY_LISTEN_ADDR", cfg.GatewayListenAddr)
	cfg.DNSEnabled = envBool("PORTER_DNS_ENABLED", cfg.DNSEnabled)
	cfg.HealthEnabled = envBool("PORTER_HEALTH_ENABLED", cfg.HealthEnabled)
	cfg.SSHEnabled = envBool("PORTER_SSH_ENABLED", cfg.SSHEnabled)
	cfg.SSHListenAddr = envOr("PORTER_SSH_LISTEN_ADDR", cfg.SSHListenAddr)
	cfg.BootstrapAdminPassword = envOr("PORTER_BOOTSTRAP_ADMIN_PASSWORD", cfg.BootstrapAdminPassword)
	cfg.TLSEnabled = envBool("PORTER_TLS_ENABLED", cfg.TLSEnabled)
	cfg.ACMEEmail = envOr("PORTER_ACME_EMAIL", cfg.ACMEEmail)
	cfg.GatewayIP = envOr("PORTER_GATEWAY_IP", cfg.GatewayIP)
	cfg.VolumesDir = envOr("PORTER_VOLUMES_DIR", cfg.VolumesDir)
	cfg.AutoscaleEnabled = envBool("PORTER_AUTOSCALE_ENABLED", cfg.AutoscaleEnabled)
	cfg.CacheEnabled = envBool("PORTER_CACHE_ENABLED", cfg.CacheEnabled)
	cfg.CacheURL = envOr("PORTER_CACHE_URL", cfg.CacheURL)
	cfg.CacheTTLSeconds = envInt("PORTER_CACHE_TTL_SECONDS", cfg.CacheTTLSeconds)
	cfg.NotifyEnabled = envBool("PORTER_NOTIFY_ENABLED", cfg.NotifyEnabled)
	cfg.SMTPHost = envOr("PORTER_SMTP_HOST", cfg.SMTPHost)
	cfg.SMTPPort = envInt("PORTER_SMTP_PORT", cfg.SMTPPort)
	cfg.SMTPUser = envOr("PORTER_SMTP_USER", cfg.SMTPUser)
	cfg.SMTPPassword = envOr("PORTER_SMTP_PASSWORD", cfg.SMTPPassword)
	cfg.SMTPFrom = envOr("PORTER_SMTP_FROM", cfg.SMTPFrom)
	cfg.NotifyDefaultTo = envOr("PORTER_NOTIFY_DEFAULT_TO", cfg.NotifyDefaultTo)

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("no database url configured — set [database] url in %s or PORTER_DATABASE_URL", path)
	}
	if _, err := parseRuntimeMode(cfg.RuntimeMode); err != nil {
		return nil, err
	}
	return cfg, nil
}

func parseRuntimeMode(raw string) (string, error) {
	if raw == "" || raw == "direct" {
		return "direct", nil
	}
	return "", fmt.Errorf("invalid firecracker runtime_mode %q (want direct)", raw)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// tomlInt reads a [section] key as an int with a default (0 on parse error).
func tomlInt(sections map[string]map[string]string, section, key string, def int) int {
	v := tomlGet(sections, section, key, "")
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// envInt parses a PORTER_* int with a default.
func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
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
