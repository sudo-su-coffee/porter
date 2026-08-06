package main

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
	RootfsPath   string            `json:"rootfs_path,omitempty"` // NEW
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