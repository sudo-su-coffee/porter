package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// FCConfig holds the pieces every VM needs to boot: a kernel image
// (vmlinux), a default rootfs (rootfs.ext4) used when a VM doesn't
// specify its own, and the path to the `firecracker` binary itself.
type FCConfig struct {
	KernelImagePath string // vmlinux
	RootfsPath      string // rootfs.ext4 (default, per-VM RootfsPath can override)
	FirecrackerBin  string // path to the firecracker binary
}

// runningVM tracks the live OS process + API socket for one booted VM.
type runningVM struct {
	cmd      *exec.Cmd
	sockPath string
	cancel   context.CancelFunc
}

// VMManager drives `firecracker` processes directly over Firecracker's
// own HTTP API (see fcapi.go). There is no containerd, no Docker, and
// no image-pulling step here on purpose: every VM boots straight from a
// pre-built rootfs.ext4 + the shared vmlinux kernel. One `firecracker`
// process per VM, each with its own API socket, tap device, and rootfs.
type VMManager struct {
	cfg   FCConfig
	store *Store
	hub   *Hub

	mu  sync.Mutex
	vms map[string]*runningVM
}

func NewVMManager(cfg FCConfig, store *Store, hub *Hub) (*VMManager, error) {
	if cfg.KernelImagePath == "" {
		return nil, fmt.Errorf("PORTER_KERNEL_IMAGE must be set (path to a vmlinux built for Firecracker guest boot)")
	}
	if _, err := os.Stat(cfg.KernelImagePath); err != nil {
		return nil, fmt.Errorf("PORTER_KERNEL_IMAGE %q: %w", cfg.KernelImagePath, err)
	}
	if cfg.FirecrackerBin == "" {
		cfg.FirecrackerBin = "firecracker"
	}
	if _, err := exec.LookPath(cfg.FirecrackerBin); err != nil {
		return nil, fmt.Errorf("firecracker binary %q not found in PATH: %w", cfg.FirecrackerBin, err)
	}
	return &VMManager{
		cfg:   cfg,
		store: store,
		hub:   hub,
		vms:   make(map[string]*runningVM),
	}, nil
}

// Close force-stops every VM still tracked in memory. Called once, on
// process shutdown.
func (m *VMManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, rv := range m.vms {
		killVM(rv)
		delete(m.vms, id)
	}
}

// BootSpec is the per-VM network identity handed out by NetManager.
type BootSpec struct {
	MacAddress  string
	HostDevName string // host-side tap device name
	CIDR        string // guest IP + prefix, e.g. "10.42.1.5/24"
	GatewayAddr string
}

// Boot starts one `firecracker` process for vm and configures it over
// the API socket: kernel + boot args, root drive, network interface,
// machine config, then InstanceStart. Runs in the background; state
// transitions are pushed to the store + SSE hub as they happen.
func (m *VMManager) Boot(vm *VM, spec BootSpec) {
	m.setState(vm, StateBooting, "")
	go func() {
		rootfs := vm.RootfsPath
		if rootfs == "" {
			rootfs = m.cfg.RootfsPath
		}
		if rootfs == "" {
			m.setState(vm, StateFailed, "no rootfs specified (set \"rootfs\" on the VM/service or PORTER_ROOTFS_PATH)")
			return
		}
		if _, err := os.Stat(rootfs); err != nil {
			m.setState(vm, StateFailed, fmt.Sprintf("rootfs not found: %v", err))
			return
		}

		ip, ipnet, err := net.ParseCIDR(spec.CIDR)
		if err != nil {
			m.setState(vm, StateFailed, fmt.Sprintf("invalid CIDR: %v", err))
			return
		}
		mask := net.IP(ipnet.Mask).String()

		sockPath := fmt.Sprintf("/tmp/porter-fc-%s.sock", vm.ID)
		_ = os.Remove(sockPath) // clear a stale socket left by a previous run

		ctx, cancel := context.WithCancel(context.Background())

		cmd := exec.Command(m.cfg.FirecrackerBin, "--api-sock", sockPath)
		if err := cmd.Start(); err != nil {
			cancel()
			m.setState(vm, StateFailed, fmt.Sprintf("failed to start firecracker: %v", err))
			return
		}

		rv := &runningVM{cmd: cmd, sockPath: sockPath, cancel: cancel}
		m.mu.Lock()
		m.vms[vm.ID] = rv
		m.mu.Unlock()

		// Watch independently for the process exiting on its own (bad
		// rootfs, bad kernel, OOM-killed, etc.) so a crash is reflected
		// even though nothing called Stop().
		procDone := make(chan error, 1)
		go func() { procDone <- cmd.Wait() }()

		readyCtx, readyCancel := context.WithTimeout(ctx, 3*time.Second)
		err = waitForSocket(readyCtx, sockPath)
		readyCancel()
		if err != nil {
			m.failBoot(vm, fmt.Sprintf("firecracker did not open its API socket: %v", err))
			return
		}

		fc := newFCClient(sockPath)
		apiCtx, apiCancel := context.WithTimeout(ctx, 5*time.Second)
		defer apiCancel()

		// Static guest networking is injected via the kernel command
		// line (the same mechanism firecracker-go-sdk uses under the
		// hood): ip=<client>:<server>:<gw>:<mask>:<host>:<dev>:<autoconf>
		bootArgs := fmt.Sprintf(
			"console=ttyS0 reboot=k panic=1 pci=off nomodules rw ip=%s::%s:%s::eth0:off",
			ip.String(), spec.GatewayAddr, mask,
		)

		steps := []struct {
			name string
			fn   func() error
		}{
			{"boot-source", func() error { return fc.SetBootSource(apiCtx, m.cfg.KernelImagePath, bootArgs) }},
			{"drive", func() error { return fc.SetRootDrive(apiCtx, "rootfs", rootfs, false) }},
			{"network-interface", func() error {
				return fc.SetNetworkInterface(apiCtx, "eth0", spec.MacAddress, spec.HostDevName)
			}},
			{"machine-config", func() error { return fc.SetMachineConfig(apiCtx, vm.VCPUs, vm.MemMiB) }},
			{"start", func() error { return fc.InstanceStart(apiCtx) }},
		}
		for _, s := range steps {
			if err := s.fn(); err != nil {
				m.failBoot(vm, fmt.Sprintf("%s: %v", s.name, err))
				return
			}
		}

		now := time.Now().UTC()
		vm.StartedAt = &now
		vm.IPAddress = ip.String()
		m.setState(vm, StateRunning, "")

		if vm.Healthcheck != nil {
			m.setHealth(vm, HealthChecking)
			go m.runHealthcheck(vm)
		} else {
			m.setHealth(vm, HealthHealthy)
		}

		// Block here (in this same goroutine) until the process exits,
		// then reconcile state if that wasn't a user-initiated Stop().
		exitErr := <-procDone
		m.mu.Lock()
		_, stillTracked := m.vms[vm.ID]
		if stillTracked {
			delete(m.vms, vm.ID)
		}
		m.mu.Unlock()
		if !stillTracked {
			return // Stop() already removed it and owns the state transition
		}
		_ = os.Remove(sockPath)
		current, ok := m.store.GetVM(vm.ID)
		if !ok {
			return
		}
		current.IPAddress = ""
		msg := "firecracker exited"
		if exitErr != nil {
			msg = fmt.Sprintf("firecracker exited unexpectedly: %v", exitErr)
		}
		m.setState(current, StateFailed, msg)
	}()
}

func (m *VMManager) failBoot(vm *VM, msg string) {
	m.mu.Lock()
	rv, ok := m.vms[vm.ID]
	delete(m.vms, vm.ID)
	m.mu.Unlock()
	if ok {
		killVM(rv)
		rv.cancel()
		_ = os.Remove(rv.sockPath)
	}
	m.setState(vm, StateFailed, msg)
}

// Stop shuts a VM down: SendCtrlAltDel for a clean guest shutdown, then
// SIGTERM/SIGKILL the firecracker process if it hasn't exited shortly
// after.
func (m *VMManager) Stop(vm *VM) {
	m.setState(vm, StateStopping, "")
	go func() {
		m.mu.Lock()
		rv, ok := m.vms[vm.ID]
		delete(m.vms, vm.ID)
		m.mu.Unlock()

		if ok {
			fc := newFCClient(rv.sockPath)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = fc.SendCtrlAltDel(ctx)
			cancel()

			done := make(chan struct{})
			go func() {
				_ = rv.cmd.Wait()
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(4 * time.Second):
				killVM(rv)
				<-done
			}
			rv.cancel()
			_ = os.Remove(rv.sockPath)
		}

		vm.IPAddress = ""
		m.setState(vm, StateStopped, "")
		m.setHealth(vm, HealthChecking)
	}()
}

func killVM(rv *runningVM) {
	if rv == nil || rv.cmd == nil || rv.cmd.Process == nil {
		return
	}
	_ = rv.cmd.Process.Signal(syscall.SIGTERM)
	go func(p *os.Process) {
		time.Sleep(2 * time.Second)
		_ = p.Kill()
	}(rv.cmd.Process)
}

func (m *VMManager) setState(vm *VM, state, errMsg string) {
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

func (m *VMManager) setHealth(vm *VM, health string) {
	vm.HealthStatus = health
	m.store.PutVM(vm)
	m.hub.Broadcast("replica.health", map[string]any{
		"vm_id":         vm.ID,
		"service":       vm.ServiceName,
		"health_status": vm.HealthStatus,
	})
}

func (m *VMManager) runHealthcheck(vm *VM) {
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
		if !ok || current.State != StateRunning {
			return
		}
		if probeHealth(current, hc) {
			failures = 0
			if current.HealthStatus != HealthHealthy {
				m.setHealth(current, HealthHealthy)
			}
		} else {
			failures++
			if failures >= 3 {
				m.setHealth(current, HealthUnhealthy)
			}
		}
	}
}

func probeHealth(vm *VM, hc *Healthcheck) bool {
	if vm.IPAddress == "" {
		return false
	}
	c := &net.Dialer{Timeout: 2 * time.Second}
	conn, err := c.Dial("tcp", fmt.Sprintf("%s:%d", vm.IPAddress, hc.Port))
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
