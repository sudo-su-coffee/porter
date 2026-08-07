// Package vmmanager wraps the containerd client and boots Docker/OCI images as
// Firecracker microVMs through the `aws.firecracker` shim. This is the ONLY
// package allowed to import the containerd client (Unified Spec §6): every
// other module (health, gateway, sshgw) reaches VMs through here.
//
// Lifecycle: Pull → NewContainer(WithRuntime "aws.firecracker") → NewTask →
// Start. Stop is SIGTERM → 5s → SIGKILL. All state transitions are persisted
// through the store and broadcast on the SSE hub.
package vmmanager

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/opencontainers/runtime-spec/specs-go"

	"porter/internal/types"
)

// Store is the narrow persistence surface vmmanager needs. The concrete
// *store.Store satisfies it structurally.
type Store interface {
	GetVM(id string) (*types.VM, bool)
	PutVM(vm *types.VM)
}

// Hub broadcasts state transitions to the dashboard.
type Hub interface {
	Broadcast(event string, data any)
}

// Config carries the containerd wiring from porter.toml / env.
type Config struct {
	ContainerdSocket string // /run/containerd/containerd.sock
	Snapshotter      string // "devmapper"
	Namespace        string // "porter"
	KernelImage      string // shared vmlinux the shim boots
	LogsDir          string // where per-VM stdio logs land
	Simulate         bool   // dev/demo: fake the lifecycle, no containerd needed
}

type runningVM struct {
	container containerd.Container
	task      containerd.Task
}

// Manager boots, stops, and deletes VM replicas.
type Manager struct {
	cfg   Config
	store Store
	hub   Hub

	mu  sync.Mutex
	vms map[string]*runningVM // vm_id -> live containerd handles
	cl  *containerd.Client
}

// New builds a VM manager. It connects to containerd lazily on the first real
// boot (so simulate mode and tests never need a socket).
func New(cfg Config, store Store, hub Hub) *Manager {
	return &Manager{
		cfg:   cfg,
		store: store,
		hub:   hub,
		vms:   map[string]*runningVM{},
	}
}

func (m *Manager) client(ctx context.Context) (*containerd.Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cl != nil {
		return m.cl, nil
	}
	cl, err := containerd.New(m.cfg.ContainerdSocket, containerd.WithDefaultNamespace(m.cfg.Namespace))
	if err != nil {
		return nil, fmt.Errorf("connect containerd at %s: %w", m.cfg.ContainerdSocket, err)
	}
	m.cl = cl
	return cl, nil
}

func (m *Manager) publish(event string, data any) {
	if m.hub != nil {
		m.hub.Broadcast(event, data)
	}
}

// Boot starts a VM replica: pull its image, create a container with the
// aws.firecracker runtime, start the task, then persist state.
func (m *Manager) Boot(ctx context.Context, vm *types.VM) error {
	if vm == nil {
		return fmt.Errorf("boot: nil vm")
	}
	vm.State = types.StateBooting
	m.store.PutVM(vm)
	m.publish("vm.state", map[string]any{"vm_id": vm.ID, "state": vm.State, "health_status": vm.HealthStatus})

	if m.cfg.Simulate {
		log.Printf("vmmanager[simulate]: boot %s (%s)", vm.Name, vm.Image)
		select {
		case <-time.After(500 * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		}
		vm.State = types.StateRunning
		if vm.HealthStatus == "" {
			vm.HealthStatus = types.HealthHealthy
		}
		now := time.Now()
		vm.StartedAt = &now
		m.store.PutVM(vm)
		m.publish("vm.state", map[string]any{"vm_id": vm.ID, "state": vm.State, "health_status": vm.HealthStatus})
		return nil
	}

	cl, err := m.client(ctx)
	if err != nil {
		return m.fail(vm, err)
	}
	ctx = namespaces.WithNamespace(ctx, m.cfg.Namespace)

	// 1. Pull the OCI image (registry ref; the shim unpacks the rootfs).
	image, err := cl.Pull(ctx, vm.Image, containerd.WithPullUnpack, containerd.WithPullSnapshotter(m.cfg.Snapshotter))
	if err != nil {
		return m.fail(vm, fmt.Errorf("pull %s: %w", vm.Image, err))
	}

	// 2. Create the container with the firecracker shim runtime.
	ctrName := "vm-" + vm.ID
	opts := []containerd.NewContainerOpts{
		containerd.WithImage(image),
		containerd.WithRuntime("aws.firecracker", nil),
		containerd.WithSnapshotter(m.cfg.Snapshotter),
		containerd.WithNewSnapshot(m.cfg.Snapshotter, image),
		oci.WithEnv(envMap(vm.Env)),
	}
	if vm.VCPUs > 0 || vm.MemMiB > 0 {
		opts = append(opts, oci.WithResources(&specs.LinuxResources{
			CPU:    &specs.LinuxCPU{Quota: int64Ptr(int64(vm.VCPUs) * 100000)},
			Memory: &specs.LinuxMemory{Limit: int64Ptr(int64(vm.MemMiB) * 1024 * 1024)},
		}))
	}
	ctr, err := cl.NewContainer(ctx, ctrName, opts...)
	if err != nil {
		return m.fail(vm, fmt.Errorf("create container: %w", err))
	}

	// 3. Task + Start.
	task, err := ctr.NewTask(ctx, cio.Discard)
	if err != nil {
		_ = ctr.Delete(ctx, containerd.WithSnapshotCleanup)
		return m.fail(vm, fmt.Errorf("create task: %w", err))
	}
	if err := task.Start(ctx); err != nil {
		_, _ = task.Delete(ctx, containerd.WithProcessKill)
		_ = ctr.Delete(ctx, containerd.WithSnapshotCleanup)
		return m.fail(vm, fmt.Errorf("task start: %w", err))
	}

	m.mu.Lock()
	m.vms[vm.ID] = &runningVM{container: ctr, task: task}
	m.mu.Unlock()

	vm.ContainerID = ctrName
	vm.TaskID = task.ID()
	vm.State = types.StateRunning
	now := time.Now()
	vm.StartedAt = &now
	m.store.PutVM(vm)
	m.publish("vm.state", map[string]any{"vm_id": vm.ID, "state": vm.State, "health_status": vm.HealthStatus})

	go m.watchTask(ctx, vm.ID, task)
	return nil
}

// fail marks a VM failed and broadcasts the transition.
func (m *Manager) fail(vm *types.VM, err error) error {
	vm.State = types.StateFailed
	vm.Error = err.Error()
	m.store.PutVM(vm)
	m.publish("vm.state", map[string]any{"vm_id": vm.ID, "state": vm.State, "health_status": vm.HealthStatus})
	return err
}

// watchTask listens for the task to exit and records crashes.
func (m *Manager) watchTask(ctx context.Context, vmID string, task containerd.Task) {
	exitCh, errCh := task.Wait(ctx)
	select {
	case <-exitCh:
		cur, ok := m.store.GetVM(vmID)
		if !ok {
			return
		}
		cur.State = types.StateStopped
		cur.Crashed = true
		m.store.PutVM(cur)
		m.publish("vm.state", map[string]any{"vm_id": vmID, "state": cur.State, "health_status": cur.HealthStatus})
	case <-errCh:
	case <-ctx.Done():
	}
}

// Stop gracefully stops a VM: SIGTERM, wait 5s, then SIGKILL.
func (m *Manager) Stop(ctx context.Context, vm *types.VM) error {
	if vm == nil {
		return fmt.Errorf("stop: nil vm")
	}
	if m.cfg.Simulate {
		vm.State = types.StateStopped
		m.store.PutVM(vm)
		m.publish("vm.state", map[string]any{"vm_id": vm.ID, "state": vm.State})
		return nil
	}

	m.mu.Lock()
	rvm, ok := m.vms[vm.ID]
	m.mu.Unlock()
	if !ok || rvm == nil {
		// No live handle (e.g. was never booted in this process) — just mark stopped.
		vm.State = types.StateStopped
		m.store.PutVM(vm)
		m.publish("vm.state", map[string]any{"vm_id": vm.ID, "state": vm.State})
		return nil
	}

	vm.State = types.StateStopping
	m.store.PutVM(vm)
	m.publish("vm.state", map[string]any{"vm_id": vm.ID, "state": vm.State})

	_ = rvm.task.Kill(ctx, syscall.SIGTERM)
	select {
	case <-time.After(5 * time.Second):
		_ = rvm.task.Kill(ctx, syscall.SIGKILL)
	case <-ctx.Done():
	}

	m.mu.Lock()
	delete(m.vms, vm.ID)
	m.mu.Unlock()
	vm.State = types.StateStopped
	m.store.PutVM(vm)
	m.publish("vm.state", map[string]any{"vm_id": vm.ID, "state": vm.State})
	return nil
}

// Restart stops then boots the VM again.
func (m *Manager) Restart(ctx context.Context, vm *types.VM) error {
	if err := m.Stop(ctx, vm); err != nil {
		return err
	}
	return m.Boot(ctx, vm)
}

// Delete stops the VM, removes the task and container (with snapshot cleanup),
// and clears any live handles.
func (m *Manager) Delete(ctx context.Context, vm *types.VM) error {
	if vm == nil {
		return nil
	}
	if err := m.Stop(ctx, vm); err != nil {
		log.Printf("vmmanager: stop during delete %s: %v", vm.ID, err)
	}
	if !m.cfg.Simulate {
		m.mu.Lock()
		rvm := m.vms[vm.ID]
		m.mu.Unlock()
		if rvm != nil {
			ctx2, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			_, _ = rvm.task.Delete(ctx2, containerd.WithProcessKill)
			_ = rvm.container.Delete(ctx2, containerd.WithSnapshotCleanup)
		}
	}
	m.mu.Lock()
	delete(m.vms, vm.ID)
	m.mu.Unlock()
	m.publish("vm.state", map[string]any{"vm_id": vm.ID, "state": "deleted"})
	return nil
}

// Close shuts down the manager and releases the containerd client.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cl != nil {
		_ = m.cl.Close()
		m.cl = nil
	}
	return nil
}

// --- helpers ---

func envMap(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}

func int64Ptr(v int64) *int64 { return &v }

// ensureLogDir creates the per-VM log dir if needed (used by callers that
// attach real stdio files).
func ensureLogDir(logsDir string) {
	if logsDir != "" {
		_ = os.MkdirAll(logsDir, 0o755)
	}
}

func logPathFor(logsDir, vmID string) string {
	return filepath.Join(logsDir, vmID+".log")
}
