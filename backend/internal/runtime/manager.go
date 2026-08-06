// Package runtime drives VM lifecycle. For v0.1.0 this boots OCI images as
// Firecracker microVMs through containerd using the `aws.firecracker` shim:
// image pull, devmapper snapshots, jailer wiring, and the in-VM agent are
// all the shim's job. Porter stays a thin control plane (see README).
package runtime

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/oci"

	"porter/internal/event"
	netmgr "porter/internal/net"
	"porter/internal/store"
	"porter/internal/types"
)

// FCConfig holds the containerd wiring every VM needs to boot. The kernel
// (vmlinux), devmapper snapshot pool, jailer, and in-VM agent live on the
// host in /etc/containerd/firecracker-runtime.json — Porter does not own
// them. Porter only needs to reach the containerd socket and pick a
// snapshotter + namespace.
type FCConfig struct {
	ContainerdSocket string // e.g. /run/containerd/containerd.sock
	Snapshotter      string // snapshotter to pull/unpack into (devmapper on a real host)
	Namespace        string // containerd namespace, e.g. "porter"
	LogsDir          string // where per-VM stdio logs land (e.g. /var/log/porter)
	Simulate         bool   // fake the lifecycle (pending->running) for dev/demo, no containerd needed
}

// runningVM tracks the live containerd handles for one booted VM.
type runningVM struct {
	client    *containerd.Client
	container containerd.Container
	task      containerd.Task
	cancel    context.CancelFunc
}

// VMManager drives microVM lifecycle through containerd + the
// `aws.firecracker` runtime. One container per VM.
type VMManager struct {
	cfg   FCConfig
	store *store.Store
	hub   *event.Hub

	mu  sync.Mutex
	vms map[string]*runningVM
}

// NewVMManager builds a VM manager. This validates nothing: the server must
// be able to come up and show the dashboard even before containerd is
// provisioned. Containerd reachability is checked lazily inside Boot().
func NewVMManager(cfg FCConfig, store *store.Store, hub *event.Hub) *VMManager {
	return &VMManager{
		cfg:   cfg,
		store: store,
		hub:   hub,
		vms:   make(map[string]*runningVM),
	}
}

// Close force-stops every VM still tracked in memory. Called on shutdown.
func (m *VMManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, rv := range m.vms {
		m.terminateVM(rv)
		if rv.client != nil {
			_ = rv.client.Close()
		}
		delete(m.vms, id)
	}
}

// Boot starts one `aws.firecracker` VM from an OCI image via containerd.
// Runs in the background; state transitions are pushed to the store + SSE
// hub as they happen. spec carries the planned network identity (the shim's
// CNI provides the real tap/host device).
func (m *VMManager) Boot(vm *types.VM, spec netmgr.BootSpec) {
	m.setState(vm, types.StateBooting, "")
	go func() {
		if m.cfg.Simulate {
			m.simulateBoot(vm, spec)
			return
		}
		// Host prerequisites are validated at boot time, not at server
		// startup, so `porter server` can come up on a fresh box.
		if m.cfg.ContainerdSocket == "" {
			m.setState(vm, types.StateFailed, `no containerd socket configured — set [firecracker] containerd_socket / PORTER_CONTAINERD_SOCKET`)
			return
		}
		if _, err := os.Stat(m.cfg.ContainerdSocket); err != nil {
			m.setState(vm, types.StateFailed, fmt.Sprintf("containerd socket not found at %s: %v — is containerd running? (run `deploy/install.sh` on the host, or set simulate = true in porter.toml for a dev/demo run)", m.cfg.ContainerdSocket, err))
			return
		}
		if vm.Image == "" {
			m.setState(vm, types.StateFailed, `no image specified on the VM/service (e.g. "redis:7-alpine")`)
			return
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		clnt, err := containerd.New(m.cfg.ContainerdSocket, containerd.WithDefaultNamespace(m.cfg.Namespace))
		if err != nil {
			m.setState(vm, types.StateFailed, fmt.Sprintf("connect containerd: %v", err))
			return
		}
		defer clnt.Close()

		// Pull + recreate the image once; the snapshot is materialized per-VM
		// via WithNewSnapshot below.
		img, err := clnt.Pull(ctx, vm.Image,
			containerd.WithPullUnpack,
			containerd.WithPullSnapshotter(m.cfg.Snapshotter),
		)
		if err != nil {
			m.setState(vm, types.StateFailed, fmt.Sprintf("pull image %q (is the snapshotter %q configured on the host?): %v", vm.Image, m.cfg.Snapshotter, err))
			return
		}

		// Make sure the per-VM stdio log directory exists before NewTask
		// opens a path inside it; on a fresh host this dir isn't created yet.
		if err := os.MkdirAll(m.cfg.LogsDir, 0o755); err != nil {
			m.setState(vm, types.StateFailed, fmt.Sprintf("create logs dir %s: %v", m.cfg.LogsDir, err))
			return
		}

		containerID := fmt.Sprintf("porter-%s", vm.ID)
		container, err := clnt.NewContainer(ctx, containerID,
			containerd.WithNewSnapshot(containerID, img),
			containerd.WithNewSpec(
				oci.WithImageConfig(img),
				oci.WithEnv(envSlice(vm.Env)),
			),
			containerd.WithRuntime("aws.firecracker", nil),
		)
		if err != nil {
			m.setState(vm, types.StateFailed, fmt.Sprintf("create container (is the aws.firecracker shim registered?): %v", err))
			return
		}

		logPath := filepath.Join(m.cfg.LogsDir, containerID+".log")
		task, err := container.NewTask(ctx, cio.LogFile(logPath))
		if err != nil {
			_ = container.Delete(ctx)
			m.setState(vm, types.StateFailed, fmt.Sprintf("create task: %v", err))
			return
		}

		statusC, err := task.Wait(ctx)
		if err != nil {
			_, _ = task.Delete(ctx)
			_ = container.Delete(ctx)
			m.setState(vm, types.StateFailed, fmt.Sprintf("wait task: %v", err))
			return
		}

		if err := task.Start(ctx); err != nil {
			_, _ = task.Delete(ctx)
			_ = container.Delete(ctx)
			m.setState(vm, types.StateFailed, fmt.Sprintf("start task: %v", err))
			return
		}

		rv := &runningVM{client: clnt, container: container, task: task, cancel: cancel}
		m.mu.Lock()
		m.vms[vm.ID] = rv
		m.mu.Unlock()

		now := time.Now().UTC()
		vm.StartedAt = &now
		vm.ContainerID = containerID
		vm.TaskID = task.ID()
		// Best-effort guest IP from the planned subnet; the shim's CNI owns
		// the real device/IP at boot.
		if ip, _, err := net.ParseCIDR(spec.CIDR); err == nil {
			vm.IPAddress = ip.String()
		}
		m.setState(vm, types.StateRunning, "")

		if vm.Healthcheck != nil {
			m.setHealth(vm, types.HealthChecking)
			go m.runHealthcheck(vm)
		} else {
			m.setHealth(vm, types.HealthHealthy)
		}

		// Block here until the task exits; reconcile state if that wasn't a
		// user-initiated Stop().
		st := <-statusC
		code, _, serr := st.Result()
		m.mu.Lock()
		_, stillTracked := m.vms[vm.ID]
		if stillTracked {
			delete(m.vms, vm.ID)
		}
		m.mu.Unlock()
		if !stillTracked {
			return // Stop() already removed it and owns the state transition
		}

		// The VM exited on its own (guest shutdown / crash). Release its
		// container + snapshot so a natural exit doesn't leak a container
		// per boot — Stop()/Close() normally own this cleanup.
		delCtx, delCancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, _ = rv.task.Delete(delCtx)
		_ = rv.container.Delete(delCtx, containerd.WithSnapshotCleanup)
		delCancel()
		if rv.client != nil {
			_ = rv.client.Close()
		}

		current, ok := m.store.GetVM(vm.ID)
		if !ok {
			return
		}
		msg := "VM exited"
		switch {
		case serr != nil:
			msg = fmt.Sprintf("VM exited with error: %v", serr)
		case code != 0:
			msg = fmt.Sprintf("VM exited with code %d", code)
		}
		m.setState(current, types.StateFailed, msg)
	}()
}

// envSlice renders a map of env vars as the []string the OCI spec wants.
// simulateBoot fakes a VM lifecycle (pending->booting->running) with no
// containerd, so the API + dashboard are fully explorable on a box without
// the runtime. Controlled by [firecracker] simulate = true / PORTER_SIMULATE.
// No live handle is stored, so Stop() simply flips state to stopped.
func (m *VMManager) simulateBoot(vm *types.VM, spec netmgr.BootSpec) {
	time.Sleep(1200 * time.Millisecond)
	if ip, _, err := net.ParseCIDR(spec.CIDR); err == nil {
		vm.IPAddress = ip.String()
	}
	now := time.Now().UTC()
	vm.StartedAt = &now
	vm.ContainerID = "sim-" + vm.ID
	vm.TaskID = "sim-" + vm.ID
	m.setState(vm, types.StateRunning, "")
	m.setHealth(vm, types.HealthHealthy)
}

func envSlice(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}

// Stop shuts a VM down through the shim: SIGTERM for a clean guest shutdown,
// escalate to SIGKILL after a short grace, then release the container and
// its snapshot.
func (m *VMManager) Stop(vm *types.VM) {
	m.setState(vm, types.StateStopping, "")
	go func() {
		m.mu.Lock()
		rv, ok := m.vms[vm.ID]
		delete(m.vms, vm.ID)
		m.mu.Unlock()

		if ok {
			m.terminateVM(rv)
			if rv.client != nil {
				_ = rv.client.Close()
			}
		}

		vm.IPAddress = ""
		m.setState(vm, types.StateStopped, "")
		m.setHealth(vm, types.HealthChecking)
	}()
}

// terminateVM sends SIGTERM, escalates to SIGKILL after a short grace, then
// deletes the task + container (freeing the snapshot). Safe once per VM.
func (m *VMManager) terminateVM(rv *runningVM) {
	if rv == nil {
		return
	}
	sigCtx, sigCancel := context.WithTimeout(context.Background(), 2*time.Second)
	_ = rv.task.Kill(sigCtx, syscall.SIGTERM)
	sigCancel()

	// Short grace for the guest to shut down, then SIGKILL to force it.
	time.Sleep(4 * time.Second)
	killCtx, killCancel := context.WithTimeout(context.Background(), 2*time.Second)
	_ = rv.task.Kill(killCtx, syscall.SIGKILL)
	killCancel()

	delCtx, delCancel := context.WithTimeout(context.Background(), 10*time.Second)
	_, _ = rv.task.Delete(delCtx)
	_ = rv.container.Delete(delCtx, containerd.WithSnapshotCleanup)
	delCancel()
}

func (m *VMManager) setState(vm *types.VM, state, errMsg string) {
	vm.State = state
	vm.Error = errMsg
	m.store.PutVM(vm)
	m.hub.Broadcast("vm.state", map[string]any{
		"vm_id":      vm.ID,
		"state":      vm.State,
		"ip_address": vm.IPAddress,
		"error":      vm.Error,
	})
}

func (m *VMManager) setHealth(vm *types.VM, health string) {
	vm.HealthStatus = health
	m.store.PutVM(vm)
	m.hub.Broadcast("replica.health", map[string]any{
		"vm_id":         vm.ID,
		"service":       vm.ServiceName,
		"health_status": vm.HealthStatus,
	})
}

func (m *VMManager) runHealthcheck(vm *types.VM) {
	hc := vm.Healthcheck
	interval := time.Duration(hc.IntervalSec) * time.Second
	if interval <= 0 {
		interval = 10 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	failures := 0
	for range ticker.C {
		m.mu.Lock()
		_, ok := m.vms[vm.ID]
		m.mu.Unlock()
		if !ok {
			return
		}
		current, ok := m.store.GetVM(vm.ID)
		if !ok || current.State != types.StateRunning {
			return
		}
		if probeHealth(current, hc) {
			failures = 0
			if current.HealthStatus != types.HealthHealthy {
				m.setHealth(current, types.HealthHealthy)
			}
		} else {
			failures++
			if failures >= 3 {
				m.setHealth(current, types.HealthUnhealthy)
			}
		}
	}
}

func probeHealth(vm *types.VM, hc *types.Healthcheck) bool {
	if vm.IPAddress == "" {
		return false
	}
	c := &net.Dialer{Timeout: 2 * time.Second}
	conn, err := c.Dial("tcp", net.JoinHostPort(vm.IPAddress, strconv.Itoa(hc.Port)))
	if err != nil {
		return false
	}
	conn.Close()
	return true
}