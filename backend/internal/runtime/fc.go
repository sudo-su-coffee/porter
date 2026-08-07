// Minimal direct-Firecracker client used ONLY for BARE microVM images (a
// rootfs.ext4 + vmlinux registered in the catalog with a "rootfs"/"kernel"
// manifest). OCI/docker images boot via containerd (see manager.go); bare
// images talk to a spawned `firecracker` process over its Unix API socket
// because there is no containerd OCI image to pull.
package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

// fccmd dials the per-VM Firecracker API socket.
type fcClient struct{ sock string }

func newFCClient(sock string) *fcClient { return &fcClient{sock: sock} }

func (c *fcClient) put(ctx context.Context, path string, body any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	url := "http://localhost" + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	// net/http cannot dial unix sockets over a plain URL; use a custom DialContext.
	tr := &http.Transport{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", c.sock)
		},
	}
	cli := &http.Client{Transport: tr}
	resp, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("PUT %s: %s", path, resp.Status)
	}
	return nil
}

func (c *fcClient) SetBootSource(ctx context.Context, kernelPath, bootArgs string) error {
	return c.put(ctx, "/boot-source", struct {
		KernelImagePath string `json:"kernel_image_path"`
		BootArgs        string `json:"boot_args"`
	}{kernelPath, bootArgs})
}

func (c *fcClient) SetRootDrive(ctx context.Context, path string, readOnly bool) error {
	return c.put(ctx, "/drives/rootfs", struct {
		DriveID      string `json:"drive_id"`
		PathOnHost   string `json:"path_on_host"`
		IsRootDevice bool   `json:"is_root_device"`
		IsReadOnly   bool   `json:"is_read_only"`
	}{"rootfs", path, true, readOnly})
}

func (c *fcClient) SetNetworkInterface(ctx context.Context, ifaceID, mac, hostDev string) error {
	return c.put(ctx, "/network-interfaces/"+ifaceID, struct {
		IfaceID    string `json:"iface_id"`
		GuestMAC   string `json:"guest_mac"`
		HostDevName string `json:"host_dev_name"`
	}{ifaceID, mac, hostDev})
}

func (c *fcClient) SetMachineConfig(ctx context.Context, vcpus, memMiB int) error {
	return c.put(ctx, "/machine-config", struct {
		VcpuCount  int `json:"vcpu_count"`
		MemSizeMiB int `json:"mem_size_mib"`
	}{vcpus, memMiB})
}

func (c *fcClient) InstanceStart(ctx context.Context) error {
	return c.put(ctx, "/actions", struct {
		ActionType string `json:"action_type"`
	}{"InstanceStart"})
}

// waitForSocket polls until the Firecracker API Unix socket exists.
func waitForSocket(ctx context.Context, path string) error {
	t := time.NewTicker(50 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			if _, err := net.Dial("unix", path); err == nil {
				return nil
			}
		}
	}
}