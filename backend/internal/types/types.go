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
	Protocol      string `json:"protocol"`
}

// ImageManifest is one entry in the on-disk image catalog (vms/images).
type ImageManifest struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Description string            `json:"description"`
	Image       string            `json:"image"`
	VCPUs       int               `json:"vcpus"`
	MemMiB      int               `json:"mem_mib"`
	Ports       []Port            `json:"ports"`
	Env         map[string]string `json:"env"`
	Tags        []string          `json:"tags"`
	Logo        string            `json:"logo"`
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
	CreatedAt    time.Time         `json:"created_at"`
	StartedAt    *time.Time        `json:"started_at,omitempty"`
}

type ServicePool struct {
	Desired int      `json:"desired"`
	Healthy int      `json:"healthy"`
	VMs     []string `json:"vms"`
}

type Project struct {
	ID           string                  `json:"id"`
	Name         string                  `json:"name"`
	Source       string                  `json:"source"`
	Network      string                  `json:"network"`
	VMIDs        []string                `json:"vm_ids"`
	ServicePools map[string]*ServicePool `json:"service_pools,omitempty"`
	ComposeYAML  string                  `json:"-"`
	CreatedAt    time.Time               `json:"created_at"`
}

type Domain struct {
	VMID   string `json:"-"`
	Domain string `json:"domain"`
	Type   string `json:"type"`
	Status string `json:"status"`
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