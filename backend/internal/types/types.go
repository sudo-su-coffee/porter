// Package types defines the canonical Porter domain types shared across
// the API, store, VM manager, and dashboard.
package types

import "time"

const (
	StatePending  = "pending"
	StateBooting  = "booting"
	StateRunning  = "running"
	StateStopping = "stopping"
	StateStopped  = "stopped"
	StateFailed   = "failed"
)

const (
	HealthHealthy   = "healthy"
	HealthUnhealthy = "unhealthy"
	HealthChecking  = "checking"
)

type Port struct {
	ContainerPort int    `json:"container_port"`
	HostPort      int    `json:"host_port,omitempty"` // host->guest mapping; defaults to container_port
	Protocol      string `json:"protocol"`
}

// ImageManifest is one entry in the on-disk image catalog (vms/images).
// For OCI images the shim pulls `image` from a registry (or local containerd
// store after `ctr images import`). For a BARE microVM image you can instead
// point `rootfs` (an ext4 image) and optionally `kernel` at host files — the
// containerd shim then boots that rootfs/kernel directly.
type ImageManifest struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Description string            `json:"description"`
	Image       string            `json:"image,omitempty"`   // OCI ref (registry or locally-imported)
	Rootfs      string            `json:"rootfs,omitempty"`  // bare ext4 rootfs path on the host
	Kernel      string            `json:"kernel,omitempty"`  // optional per-image vmlinux
	VCPUs       int               `json:"vcpus"`
	MemMiB      int               `json:"mem_mib"`
	Ports       []Port            `json:"ports"`
	Env         map[string]string `json:"env"`
	Tags        []string          `json:"tags"`
	Logo        string            `json:"logo"` // URL; the UI renders an offline fallback if it can't load
}

type Healthcheck struct {
	Type        string `json:"type,omitempty"`
	Path        string `json:"path,omitempty"`
	Port        int    `json:"port,omitempty"`
	IntervalSec int    `json:"interval_sec,omitempty"`
}

type VM struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	ProjectID    string            `json:"project_id"`
	ServiceName  string            `json:"service_name"`
	State        string            `json:"state"`
	HealthStatus string            `json:"health_status"`
	ReplicaIndex int               `json:"replica_index"`
	Image        string            `json:"image"`
	RootfsPath   string            `json:"rootfs_path,omitempty"`
	Kernel       string            `json:"kernel,omitempty"` // per-VM vmlinux (custom images); falls back to the shared kernel
	ContainerID  string            `json:"container_id,omitempty"`
	TaskID       string            `json:"task_id,omitempty"`
	VCPUs        int               `json:"vcpus"`
	MemMiB       int               `json:"mem_mib"`
	IPAddress    string            `json:"ip_address"`
	Ports        []Port            `json:"ports"`
	Env          map[string]string `json:"env,omitempty"`
	Healthcheck  *Healthcheck      `json:"healthcheck,omitempty"`
	Restart      string            `json:"restart,omitempty"`
	Error        string            `json:"error,omitempty"`
	Crashed      bool              `json:"crashed,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	StartedAt    *time.Time        `json:"started_at,omitempty"`
}

type ServicePool struct {
	Desired int      `json:"desired"`
	Healthy int      `json:"healthy"`
	VMs     []string `json:"vms"`
}

type Project struct {
	ID              string                  `json:"id"`
	OrgID           string                  `json:"org_id,omitempty"` // always set; resolves to the user's default org at create
	Name            string                  `json:"name"`
	Source          string                  `json:"source"`
	Image           string                  `json:"image,omitempty"` // OCI ref (plain text, never a local path)
	Network         string                  `json:"network"`
	VMIDs           []string                `json:"vm_ids"`
	ServicePools    map[string]*ServicePool `json:"service_pools,omitempty"`
	ComposeYAML     string                  `json:"-"`
	HostMountPath   string                  `json:"host_mount_path,omitempty"` // optional bind mount (no managed volumes)
	ReplicasDesired int                     `json:"replicas_desired,omitempty"` // replica pool size (>=1)
	RestartPolicy   string                  `json:"restart_policy,omitempty"`
	Healthcheck     *Healthcheck            `json:"healthcheck,omitempty"`
	Env             map[string]string       `json:"env,omitempty"`
	Tags            []string                `json:"tags,omitempty"`
	SSHEnabled      bool                    `json:"ssh_enabled,omitempty"` // SSH off by default; user turns it on per project
	Replicas        int                     `json:"replicas,omitempty"`    // alias for replica pool size
	StackID         string                  `json:"stack_id,omitempty"`    // parent compose stack, if this project is a compose service
	ComposeService  string                  `json:"compose_service,omitempty"` // service name inside its stack
	Model           string                  `json:"model,omitempty"`       // ML model ref (gpu/batch serving)
	GPU             string                  `json:"gpu,omitempty"`         // e.g. "nvidia-t4" or ""
	Networks        []string                `json:"networks,omitempty"`    // per-project bridge networks
	CreatedAt       time.Time               `json:"created_at"`
}

type Domain struct {
	ProjectID string `json:"project_id,omitempty"`
	VMID      string `json:"-"`
	Domain    string `json:"domain"`
	Type      string `json:"type"`
	Status    string `json:"status"`
}

type TrafficEntry struct {
	Timestamp  time.Time `json:"timestamp"`
	Method     string    `json:"method"`
	Host       string    `json:"host"`
	Path       string    `json:"path"`
	Status     int       `json:"status"`
	DurationMS int       `json:"duration_ms"`
	RemoteIP   string    `json:"remote_ip"`
}

// User is a non-bootstrap account stored in SQLite. The very first admin
// lives in porter.toml ([admin]); every additional user is persisted here
// so accounts can be managed without editing config. Passwords are stored
// as a salted hash (see api.hashPassword).
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Role         string    `json:"role"`
	PasswordHash string    `json:"-"`
	Salt         string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// Volume is a persistent storage volume (Phase 7 / v0.1.0 fold-in) that a VM
// can attach at create time. For v0.1.0 it maps to a host directory under the
// state dir; snapshots/backups/object-storage are later versions.
type Volume struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id,omitempty"`
	Name      string    `json:"name"`
	SizeMiB   int       `json:"size_mib"`
	Path      string    `json:"path,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Server is a registered compute host in a Porter cluster (Phase 8 scaffold).
// v0.1.0 only registers hosts; scheduling/migration stay deferred.
type Server struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// Service is one compose service (or the synthetic service created for a
// single-image deploy). One VM per replica.
type Service struct {
	ID              string            `json:"id"`
	ProjectID       string            `json:"project_id"`
	Name            string            `json:"name"`
	Image           string            `json:"image"`
	VCPUs           int               `json:"vcpus"`
	MemMiB          int               `json:"mem_mib"`
	ReplicasDesired int               `json:"replicas_desired"`
	RestartPolicy   string            `json:"restart_policy"`
	Healthcheck     *Healthcheck      `json:"healthcheck,omitempty"`
	Env             map[string]string `json:"env,omitempty"`
	DependsOn       []string          `json:"depends_on,omitempty"`
	Ports           []Port            `json:"ports"`
	CreatedAt       time.Time         `json:"created_at"`
}

// GoldenImage is a reusable VM template in the v0.4 image library. `Image` is
// always an OCI registry ref (never a local path) so the shim can pull it.
type GoldenImage struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	Description string            `json:"description"`
	VCPUs       int               `json:"vcpus"`
	MemMiB      int               `json:"mem_mib"`
	Ports       []Port            `json:"ports"`
	Env         map[string]string `json:"env,omitempty"`
	Tags        []string          `json:"tags"`
	Logo        string            `json:"logo"` // image URL for the dashboard tile
	Version     string            `json:"version"`
	Kind        string            `json:"kind,omitempty"` // oci | custom (user-uploaded microVM)
	Rootfs      string            `json:"rootfs,omitempty"` // host path to ext4 rootfs (custom)
	Kernel      string            `json:"kernel,omitempty"` // host path to vmlinux (custom)
	CreatedAt   time.Time         `json:"created_at"`
}

// Deployment is one revision of a project (v0.3 version history / rollback).
type Deployment struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	Revision    int       `json:"revision"`
	GitURL      string    `json:"git_url,omitempty"`
	GitCommit   string    `json:"git_commit,omitempty"`
	BuildStatus string    `json:"build_status"`
	ImageDigest string    `json:"image_digest,omitempty"`
	RollbackTo  string    `json:"rollback_to,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Secret is an encrypted per-project secret (v0.2). Value is stored as an
// opaque blob; encryption happens in the API layer.
type Secret struct {
	ID             string    `json:"id"`
	ProjectID      string    `json:"project_id"`
	Name           string    `json:"name"`
	ValueEncrypted []byte    `json:"-"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// MetricSample is one point in the v0.9 metrics time series.
type MetricSample struct {
	ID     string    `json:"id"`
	VMID   string    `json:"vm_id"`
	Metric string    `json:"metric"`
	Value  float64   `json:"value"`
	TS     time.Time `json:"ts"`
}

// HealthEvent is one recorded health transition for a VM (v0.9).
type HealthEvent struct {
	ID        string    `json:"id"`
	VMID      string    `json:"vm_id"`
	ProjectID string    `json:"project_id,omitempty"`
	ServiceID string    `json:"service_id,omitempty"`
	Status    string    `json:"status"`
	Detail    string    `json:"detail,omitempty"`
	TS        time.Time `json:"ts"`
}

// Org is a group that projects belong to (project = VM, grouped into orgs).
// Every user auto-gets a default org (is_default=true) on first run/signup.
type Org struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	OwnerID   string    `json:"owner_id,omitempty"`
	IsDefault bool      `json:"is_default,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Group is a lightweight folder for related projects, always scoped to an org.
type Group struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// DNSRecord is one DNS entry attached to a project (project = VM).
type DNSRecord struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Value     string    `json:"value"`
	TTL       int       `json:"ttl"`
	CreatedAt time.Time `json:"created_at"`
}

// Stack is a docker-compose project: a named group of service-projects. Each
// compose service is its OWN project (own microVM pool), grouped under a stack.
type Stack struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	OrgID       string    `json:"org_id,omitempty"`
	Source      string    `json:"source"`
	ComposeYAML string    `json:"compose_yaml,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// APIKey is a long-lived per-user token for programmatic access.
type APIKey struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	TokenHash string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	LastUsed  *time.Time `json:"last_used_at,omitempty"`
}

// Alert is a threshold rule over a project metric.
type Alert struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Name      string    `json:"name"`
	Metric    string    `json:"metric"`
	Threshold float64   `json:"threshold"`
	Op        string    `json:"op"`
	CooldownS int       `json:"cooldown_s"`
	Silenced  bool      `json:"silenced"`
	CreatedAt time.Time `json:"created_at"`
}

// Hook is an outbound webhook fired on project lifecycle events.
type Hook struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Events    []string  `json:"events"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

// Cron is a scheduled job that boots `job_image` as a short-lived microVM.
type Cron struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Name      string    `json:"name"`
	Schedule  string    `json:"schedule"`
	JobImage  string    `json:"job_image"`
	Active    bool      `json:"active"`
	LastRun   *time.Time `json:"last_run_at,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Drain ships a project's logs/events to an external endpoint.
type Drain struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Name      string    `json:"name"`
	Endpoint  string    `json:"endpoint"`
	Kind      string    `json:"kind"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

// Redirect is a domain redirect rule for a project.
type Redirect struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Source    string    `json:"source"`
	Target    string    `json:"target"`
	Permanent bool      `json:"permanent"`
	CreatedAt time.Time `json:"created_at"`
}

// FirewallRule is an ingress/egress allow/deny rule for a project.
type FirewallRule struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Direction string    `json:"direction"`
	Action    string    `json:"action"`
	Proto     string    `json:"proto"`
	Ports     string    `json:"ports"`
	Source    string    `json:"source"`
	Priority  int       `json:"priority"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

// Environment is a deploy environment (prod/staging/preview, Vercel-style).
type Environment struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Name      string    `json:"name"`
	Branch    string    `json:"branch"`
	URL       string    `json:"url"`
	EnvDomain string    `json:"env_domain"`
	CreatedAt time.Time `json:"created_at"`
}

// Build is a git-based build that produces an OCI image for a microVM deploy.
type Build struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	GitURL      string    `json:"git_url"`
	Branch      string    `json:"branch"`
	BuildStatus string    `json:"build_status"`
	Image       string    `json:"image"`
	Log         string    `json:"log"`
	CreatedAt   time.Time `json:"created_at"`
}

// Network is a per-project bridge network (docker-ecosystem parity).
type Network struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Name      string    `json:"name"`
	CIDR      string    `json:"cidr"`
	Driver    string    `json:"driver"`
	CreatedAt time.Time `json:"created_at"`
}

// ProjectMember is a membership/role row for a project team.
type ProjectMember struct {
	ProjectID string    `json:"project_id"`
	UserID    string    `json:"user_id"`
	Role      string    `json:"role"`
	Invited   bool      `json:"invited"`
	CreatedAt time.Time `json:"created_at"`
}