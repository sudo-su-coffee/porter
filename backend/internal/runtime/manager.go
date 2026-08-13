// Package runtime owns direct Firecracker lifecycle. Porter starts one
// Firecracker process per VM and configures it through that process's HTTP API
// Unix socket. OCI image managers are intentionally not part of this boundary:
// direct boots require a host kernel plus an unpacked rootfs.ext4 artifact.
package runtime

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"porter/internal/event"
	netmgr "porter/internal/netmgr"
	"porter/internal/store"
	"porter/internal/types"
)

// FCConfig contains only direct Firecracker host wiring.
type FCConfig struct {
	Mode           Mode
	FirecrackerBin string
	KernelImage    string
	RootfsPath     string
	SocketDir      string
	LogsDir        string
}

type runningVM struct {
	cmd      *exec.Cmd
	sockPath string
	cancel   context.CancelFunc
}

// VMManager drives one raw Firecracker process per VM.
type VMManager struct {
	cfg   FCConfig
	store *store.Store
	hub   *event.Hub

	mu  sync.Mutex
	vms map[string]*runningVM
}

func NewVMManager(cfg FCConfig, st *store.Store, hub *event.Hub) *VMManager {
	if cfg.Mode == "" {
		cfg.Mode = ModeDirect
	}
	if cfg.FirecrackerBin == "" {
		cfg.FirecrackerBin = "firecracker"
	}
	if cfg.SocketDir == "" {
		cfg.SocketDir = "/run/porter/firecracker"
	}
	if cfg.LogsDir == "" {
		cfg.LogsDir = "/var/log/porter"
	}
	return &VMManager{cfg: cfg, store: st, hub: hub, vms: make(map[string]*runningVM)}
}

// Close force-stops every tracked VMM process. It does not touch any process
// outside the manager's own VM map.
func (m *VMManager) Close() {
	m.mu.Lock()
	items := make([]*runningVM, 0, len(m.vms))
	for id, rv := range m.vms {
		items = append(items, rv)
		delete(m.vms, id)
	}
	m.mu.Unlock()
	for _, rv := range items {
		m.terminateVM(rv)
	}
}

// Boot starts a direct Firecracker VM asynchronously. A VM must resolve to a
// rootfs.ext4 path; arbitrary OCI references are rejected rather than silently
// routed through a hidden container runtime.
func (m *VMManager) Boot(vm *types.VM, spec netmgr.BootSpec) {
	m.setState(vm, types.StateBooting, "")
	if m.cfg.Mode != ModeDirect {
		m.setState(vm, types.StateFailed, fmt.Sprintf("unsupported runtime mode %q: direct Firecracker is the only backend mode", m.cfg.Mode))
		return
	}
	go m.bootDirect(vm, spec)
}

func (m *VMManager) bootDirect(vm *types.VM, spec netmgr.BootSpec) {
	rootfs := vm.RootfsPath
	if rootfs == "" {
		rootfs = m.cfg.RootfsPath
	}
	if rootfs == "" {
		m.setState(vm, types.StateFailed, "direct Firecracker requires VM rootfs_path or [firecracker] rootfs_path; OCI image references are not resolved by the backend")
		return
	}
	kernel := vm.Kernel
	if kernel == "" {
		kernel = m.cfg.KernelImage
	}
	for name, path := range map[string]string{"kernel": kernel, "rootfs": rootfs} {
		if path == "" {
			m.setState(vm, types.StateFailed, fmt.Sprintf("no %s configured for direct Firecracker boot", name))
			return
		}
		if _, err := os.Stat(path); err != nil {
			m.setState(vm, types.StateFailed, fmt.Sprintf("%s path %q is not available: %v", name, path, err))
			return
		}
	}
	if spec.HostDevName == "" || spec.MacAddress == "" {
		m.setState(vm, types.StateFailed, "direct Firecracker boot requires a host TAP device and guest MAC address")
		return
	}
	if err := os.MkdirAll(m.cfg.SocketDir, 0o755); err != nil {
		m.setState(vm, types.StateFailed, fmt.Sprintf("create Firecracker API socket directory %s: %v", m.cfg.SocketDir, err))
		return
	}
	if err := os.MkdirAll(m.cfg.LogsDir, 0o755); err != nil {
		m.setState(vm, types.StateFailed, fmt.Sprintf("create VM logs directory %s: %v", m.cfg.LogsDir, err))
		return
	}

	sockPath := socketPath(m.cfg.SocketDir, vm.ID)
	_ = removeSocket(sockPath)
	logPath := filepath.Join(m.cfg.LogsDir, "porter-"+safeVMID(vm.ID)+".log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
	if err != nil {
		m.setState(vm, types.StateFailed, fmt.Sprintf("open Firecracker log %s: %v", logPath, err))
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.Command(m.cfg.FirecrackerBin, "--api-sock", sockPath)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		cancel()
		m.setState(vm, types.StateFailed, fmt.Sprintf("start Firecracker %q: %v", m.cfg.FirecrackerBin, err))
		return
	}
	rv := &runningVM{cmd: cmd, sockPath: sockPath, cancel: cancel}
	m.mu.Lock()
	m.vms[vm.ID] = rv
	m.mu.Unlock()

	procDone := make(chan error, 1)
	go func() {
		err := cmd.Wait()
		_ = logFile.Close()
		procDone <- err
	}()

	readyCtx, readyCancel := context.WithTimeout(ctx, 5*time.Second)
	err = waitForSocket(readyCtx, sockPath)
	readyCancel()
	if err != nil {
		m.failDirect(vm, rv, fmt.Sprintf("Firecracker API socket %s was not ready: %v", sockPath, err))
		return
	}

	ip, ipnet, err := net.ParseCIDR(spec.CIDR)
	if err != nil {
		m.failDirect(vm, rv, fmt.Sprintf("invalid VM CIDR %q: %v", spec.CIDR, err))
		return
	}
	mask := net.IP(ipnet.Mask).String()
	bootArgs := fmt.Sprintf("console=ttyS0 reboot=k panic=1 pci=off nomodules rw root=/dev/vda ip=%s::%s:%s::eth0:off", ip.String(), spec.GatewayAddr, mask)
	apiCtx, apiCancel := context.WithTimeout(ctx, 10*time.Second)
	fc := newFCClient(sockPath)
	steps := []struct {
		name string
		fn   func() error
	}{
		{"boot-source", func() error { return fc.SetBootSource(apiCtx, kernel, bootArgs) }},
		{"root-drive", func() error { return fc.SetRootDrive(apiCtx, rootfs, false) }},
		{"network-interface", func() error { return fc.SetNetworkInterface(apiCtx, "eth0", spec.MacAddress, spec.HostDevName) }},
		{"machine-config", func() error { return fc.SetMachineConfig(apiCtx, vm.VCPUs, vm.MemMiB) }},
	}
	if vm.VolumeID != "" && m.store != nil {
		if vol, ok := m.store.GetVolume(vm.VolumeID); ok && vol.Path != "" {
			dataPath := filepath.Join(vol.Path, "data.img")
			steps = append(steps, struct {
				name string
				fn   func() error
			}{"data-drive", func() error { return fc.SetDataDrive(apiCtx, "data", dataPath) }})
		}
	}
	steps = append(steps, struct {
		name string
		fn   func() error
	}{"instance-start", func() error { return fc.InstanceStart(apiCtx) }})
	for _, step := range steps {
		if err := step.fn(); err != nil {
			apiCancel()
			m.failDirect(vm, rv, fmt.Sprintf("%s: %v", step.name, err))
			return
		}
	}
	apiCancel()

	now := time.Now().UTC()
	vm.StartedAt = &now
	vm.IPAddress = ip.String()
	vm.ContainerID = ""
	vm.TaskID = ""
	m.setState(vm, types.StateRunning, "")
	if vm.Healthcheck != nil {
		m.setHealth(vm, types.HealthChecking)
		go m.runHealthcheck(vm)
	} else {
		m.setHealth(vm, types.HealthHealthy)
	}

	exitErr := <-procDone
	m.mu.Lock()
	_, stillTracked := m.vms[vm.ID]
	if stillTracked {
		delete(m.vms, vm.ID)
	}
	m.mu.Unlock()
	_ = removeSocket(sockPath)
	if !stillTracked {
		return
	}
	if exitErr == nil {
		m.setState(vm, types.StateStopped, "")
		return
	}
	m.setState(vm, types.StateFailed, fmt.Sprintf("Firecracker exited: %v", exitErr))
}

func (m *VMManager) failDirect(vm *types.VM, rv *runningVM, msg string) {
	m.mu.Lock()
	if rv != nil && m.vms[vm.ID] == rv {
		delete(m.vms, vm.ID)
	}
	m.mu.Unlock()
	if rv != nil {
		m.terminateVM(rv)
	}
	m.setState(vm, types.StateFailed, msg)
}

func (m *VMManager) Stop(vm *types.VM) {
	m.setState(vm, types.StateStopping, "")
	go func() {
		m.mu.Lock()
		rv, ok := m.vms[vm.ID]
		if ok {
			delete(m.vms, vm.ID)
		}
		m.mu.Unlock()
		if ok {
			m.terminateVM(rv)
		}
		vm.IPAddress = ""
		m.setState(vm, types.StateStopped, "")
		m.setHealth(vm, types.HealthChecking)
	}()
}

func (m *VMManager) terminateVM(rv *runningVM) {
	if rv == nil {
		return
	}
	if rv.cmd != nil && rv.cmd.Process != nil {
		_ = rv.cmd.Process.Signal(syscall.SIGTERM)
		timer := time.NewTimer(4 * time.Second)
		<-timer.C
		_ = rv.cmd.Process.Kill()
		timer.Stop()
	}
	if rv.cancel != nil {
		rv.cancel()
	}
	_ = removeSocket(rv.sockPath)
}

// Exec is intentionally unavailable for direct Firecracker VMs. A future
// vsock agent can implement this without reintroducing a host-side task bridge.
func (m *VMManager) Exec(_ context.Context, vmID string, _, _ interface{}) error {
	return fmt.Errorf("direct Firecracker VM %s has no in-guest exec channel; use a guest-vsock agent", vmID)
}

func (m *VMManager) setState(vm *types.VM, state, errMsg string) {
	vm.State = state
	vm.Error = errMsg
	if m.store != nil {
		m.store.PutVM(vm)
	}
	if m.hub != nil {
		m.hub.Broadcast("vm.state", map[string]any{"vm_id": vm.ID, "state": vm.State, "ip_address": vm.IPAddress, "error": vm.Error})
	}
}

func (m *VMManager) setHealth(vm *types.VM, health string) {
	vm.HealthStatus = health
	if m.store != nil {
		m.store.PutVM(vm)
	}
	if m.hub != nil {
		m.hub.Broadcast("replica.health", map[string]any{"vm_id": vm.ID, "service": vm.ServiceName, "health_status": vm.HealthStatus})
	}
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
	_ = conn.Close()
	return true
}
