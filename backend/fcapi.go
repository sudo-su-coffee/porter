package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// FCClient is a minimal client for the Firecracker VMM's own HTTP API,
// spoken directly over the per-VM Unix domain socket that `firecracker
// --api-sock <path>` opens.
//
// This talks to Firecracker exactly the way `curl --unix-socket
// fc.sock http://localhost/...` would. It replaces firecracker-go-sdk
// on purpose: the SDK pulls in containerd, CNI, go-openapi, grpc, and a
// dozen other transitive modules for what is, underneath, six small PUT
// requests against a well-documented REST API. Using only net/http +
// net (stdlib) keeps `go.mod` dependency-free — nothing to `go mod
// tidy` or vendor, nothing that needs network access to build.
type FCClient struct {
	http *http.Client
}

func newFCClient(sockPath string) *FCClient {
	return &FCClient{
		http: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					var d net.Dialer
					return d.DialContext(ctx, "unix", sockPath)
				},
			},
		},
	}
}

func (c *FCClient) put(ctx context.Context, path string, body any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode %s body: %w", path, err)
	}
	// The host/authority in this URL is meaningless — every request is
	// dialed straight to the Unix socket above — but net/http requires
	// a well-formed URL, so "unix" is used as a readable placeholder.
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, "http://unix"+path, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("PUT %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("firecracker API PUT %s: %s: %s", path, resp.Status, bytesToShortString(b))
	}
	return nil
}

func bytesToShortString(b []byte) string {
	const max = 300
	s := string(b)
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

// --- Firecracker API request bodies (only the fields Porter sets) ---

type fcBootSource struct {
	KernelImagePath string `json:"kernel_image_path"`
	BootArgs        string `json:"boot_args,omitempty"`
}

type fcDrive struct {
	DriveID      string `json:"drive_id"`
	PathOnHost   string `json:"path_on_host"`
	IsRootDevice bool   `json:"is_root_device"`
	IsReadOnly   bool   `json:"is_read_only"`
}

type fcNetworkInterface struct {
	IfaceID     string `json:"iface_id"`
	GuestMAC    string `json:"guest_mac,omitempty"`
	HostDevName string `json:"host_dev_name"`
}

type fcMachineConfig struct {
	VCPUCount  int `json:"vcpu_count"`
	MemSizeMib int `json:"mem_size_mib"`
}

type fcAction struct {
	ActionType string `json:"action_type"`
}

// SetBootSource points the microVM at a vmlinux kernel image plus the
// kernel command line (this is also how the guest's static IP is
// injected — see BootSpec in vmmanager.go).
func (c *FCClient) SetBootSource(ctx context.Context, kernelPath, bootArgs string) error {
	return c.put(ctx, "/boot-source", fcBootSource{KernelImagePath: kernelPath, BootArgs: bootArgs})
}

// SetRootDrive attaches a rootfs.ext4 image as the VM's root block device.
func (c *FCClient) SetRootDrive(ctx context.Context, driveID, path string, readOnly bool) error {
	return c.put(ctx, "/drives/"+driveID, fcDrive{
		DriveID: driveID, PathOnHost: path, IsRootDevice: true, IsReadOnly: readOnly,
	})
}

// SetNetworkInterface attaches a host tap device as the VM's NIC.
func (c *FCClient) SetNetworkInterface(ctx context.Context, ifaceID, mac, hostDev string) error {
	return c.put(ctx, "/network-interfaces/"+ifaceID, fcNetworkInterface{
		IfaceID: ifaceID, GuestMAC: mac, HostDevName: hostDev,
	})
}

// SetMachineConfig sets vCPU count and memory size for the VM.
func (c *FCClient) SetMachineConfig(ctx context.Context, vcpus, memMiB int) error {
	return c.put(ctx, "/machine-config", fcMachineConfig{VCPUCount: vcpus, MemSizeMib: memMiB})
}

// InstanceStart boots the configured VM (this is the "power on" call —
// everything above just loads config into the not-yet-running machine).
func (c *FCClient) InstanceStart(ctx context.Context) error {
	return c.put(ctx, "/actions", fcAction{ActionType: "InstanceStart"})
}

// SendCtrlAltDel asks the guest kernel to shut down cleanly, the same
// signal a physical Ctrl+Alt+Del would send. Used for graceful stop
// before falling back to killing the firecracker process.
func (c *FCClient) SendCtrlAltDel(ctx context.Context) error {
	return c.put(ctx, "/actions", fcAction{ActionType: "SendCtrlAltDel"})
}

// waitForSocket polls until the Firecracker API's Unix socket exists and
// accepts a connection, or ctx is done. Firecracker creates this socket
// a few milliseconds after the process starts, so callers can't dial it
// immediately after exec.Cmd.Start() returns.
func waitForSocket(ctx context.Context, path string) error {
	var lastErr error
	for {
		conn, err := (&net.Dialer{}).DialContext(ctx, "unix", path)
		if err == nil {
			conn.Close()
			return nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for firecracker socket %s: %w", path, lastErr)
		case <-time.After(25 * time.Millisecond):
		}
	}
}
