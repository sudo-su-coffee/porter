// Package net allocates per-project subnets and per-VM network identity
// (IP, gateway, MAC). For v0.1.0 the tap device is created directly;
// after the firecracker-containerd migration the shim's CNI provides it.
package net

import (
	"crypto/md5"
	"fmt"
	"os/exec"
	"sync"
)

// BootSpec is the per-VM network identity handed out by the NetManager.
type BootSpec struct {
	MacAddress  string
	HostDevName string // host-side tap device name
	CIDR        string // guest IP + prefix, e.g. "10.42.1.5/24"
	GatewayAddr string
}

// NetManager hands out monotonic project subnets and per-VM identities.
type NetManager struct {
	mu      sync.Mutex
	nextOct int
}

func NewNetManager() *NetManager {
	return &NetManager{nextOct: 1}
}

// AllocateProjectSubnet returns the next /24 bridge subnet (10.42.N.0/24).
func (n *NetManager) AllocateProjectSubnet() string {
	n.mu.Lock()
	defer n.mu.Unlock()
	subnet := fmt.Sprintf("10.42.%d.0/24", n.nextOct)
	n.nextOct++
	return subnet
}

// AllocateVMNetwork derives a guest IP, gateway, MAC, and host tap device
// for one VM within the project subnet.
func (n *NetManager) AllocateVMNetwork(subnetCIDR string, replicaIndex int, vmID string) BootSpec {
	var a, b, c int
	fmt.Sscanf(subnetCIDR, "%d.%d.%d.", &a, &b, &c)
	ip := fmt.Sprintf("%d.%d.%d.%d", a, b, c, 5+replicaIndex)
	gw := fmt.Sprintf("%d.%d.%d.1", a, b, c)

	mac := deterministicMAC(vmID)
	tapName := "tap-" + shortID(vmID)

	// Create the TAP device and bring it up.
	_ = exec.Command("ip", "tuntap", "add", tapName, "mode", "tap").Run()
	_ = exec.Command("ip", "link", "set", tapName, "up").Run()
	// (You may also want to add to a bridge — leave that to the user.)

	return BootSpec{
		MacAddress:  mac,
		HostDevName: tapName,
		CIDR:        ip + "/24",
		GatewayAddr: gw,
	}
}

func deterministicMAC(vmID string) string {
	sum := md5.Sum([]byte(vmID))
	return fmt.Sprintf("02:FC:%02X:%02X:%02X:%02X", sum[0], sum[1], sum[2], sum[3])
}

func shortID(vmID string) string {
	if len(vmID) > 8 {
		return vmID[:8]
	}
	return vmID
}