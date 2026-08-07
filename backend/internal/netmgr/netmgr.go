// Package netmgr owns the networking pieces Porter is responsible for: a
// per-project private /24 subnet, static IP assignment per replica, a
// deterministic MAC per VM, and the CNI config the containerd firecracker
// shim consumes (tc-redirect-tap plugin). Porter never reimplements what CNI
// already does — it only writes the config and owns the IPAM (Unified Spec §1).
package netmgr

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net"
	"os"
	"path/filepath"
	"sync"
)

// NetManager allocates subnets and IPs. It is a process-local allocator; the
// authoritative subnet per project is persisted in Postgres by the caller.
type NetManager struct {
	mu         sync.Mutex
	nextSubnet int // index into 10.42.0.0/16, incremented per allocation
}

const baseSubnet = "10.42.0.0/16"

// NewNetManager starts allocating from 10.42.0.0/24 upward.
func NewNetManager() *NetManager {
	return &NetManager{nextSubnet: 0}
}

// AllocateSubnet returns the next free /24 in 10.42.0.0/16, e.g. 10.42.3.0/24.
func (n *NetManager) AllocateSubnet() (net.IPNet, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.nextSubnet > 255 {
		return net.IPNet{}, fmt.Errorf("netmgr: exhausted 10.42.0.0/16 (255 subnets)")
	}
	sub := fmt.Sprintf("10.42.%d.0/24", n.nextSubnet)
	n.nextSubnet++
	_, ipnet, err := net.ParseCIDR(sub)
	if err != nil {
		return net.IPNet{}, fmt.Errorf("netmgr: parse %s: %w", sub, err)
	}
	return *ipnet, nil
}

// AllocateIP returns the static IP for a replica inside a /24 subnet. Hosts
// use .1, the VM replicas start at .10 so .2-.9 stay free for infrastructure.
func (n *NetManager) AllocateIP(subnet net.IPNet, replicaIndex int) (net.IP, error) {
	ip := subnet.IP.To4()
	if ip == nil {
		return nil, fmt.Errorf("netmgr: subnet %s is not IPv4", subnet.String())
	}
	if replicaIndex < 0 || replicaIndex > 245 {
		return nil, fmt.Errorf("netmgr: replica index %d out of range (0..245)", replicaIndex)
	}
	out := make(net.IP, 4)
	copy(out, ip)
	out[3] = byte(10 + replicaIndex) // .10, .11, ...
	return out, nil
}

// DeterministicMAC derives a stable MAC address from a VM id, so a VM keeps
// the same link-layer identity across restarts.
func DeterministicMAC(vmID string) (net.HardwareAddr, error) {
	h := fnv.New64a()
	_, _ = h.Write([]byte(vmID))
	sum := h.Sum64()
	// Use the 06: local-bit prefix and derive the rest from the hash.
	return net.HardwareAddr{
		0x06,
		byte(sum >> 40), byte(sum >> 32), byte(sum >> 24), byte(sum >> 16), byte(sum >> 8),
	}, nil
}

// CNIConfig is the tc-redirect-tap plugin config the firecracker shim needs.
type CNIConfig struct {
	CNIVersion string      `json:"cniVersion"`
	Name       string      `json:"name"`
	Plugins    []cniPlugin `json:"plugins"`
}

type cniPlugin struct {
	Type        string `json:"type"`
	TapIface    string `json:"tap_iface_name,omitempty"`
	MTU         int    `json:"mtu,omitempty"`
	IPAM        any    `json:"ipam,omitempty"`
	BinDir      string `json:"bin_dir,omitempty"`
	ConfDir     string `json:"conf_dir,omitempty"`
}

// WriteCNIConfig writes a per-project CNI network config to dir/name.conflist.
func (n *NetManager) WriteCNIConfig(dir, projectID string, subnet net.IPNet, ip net.IP, gw net.IP) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("netmgr: mkdir %s: %w", dir, err)
	}
	cfg := CNIConfig{
		CNIVersion: "0.4.0",
		Name:       projectID,
		Plugins: []cniPlugin{
			{
				Type:     "tc-redirect-tap",
				TapIface: "tap-" + shortName(projectID),
				MTU:      1500,
				IPAM: map[string]any{
					"type": "static",
					"addresses": []map[string]any{{
						"address": fmt.Sprintf("%s/%d", ip.String(), prefixLen(subnet)),
					}},
					"routes": []map[string]any{{"dst": "0.0.0.0/0"}},
					"gateway": gw.String(),
				},
			},
		},
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("netmgr: marshal cni: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, projectID+".conflist"), data, 0o644)
}

// GatewayIP returns the .1 host gateway for a /24 subnet.
func GatewayIP(subnet net.IPNet) net.IP {
	ip := subnet.IP.To4()
	if ip == nil {
		return nil
	}
	out := make(net.IP, 4)
	copy(out, ip)
	out[3] = 1
	return out
}

func prefixLen(subnet net.IPNet) int {
	ones, _ := subnet.Mask.Size()
	return ones
}

func shortName(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}
