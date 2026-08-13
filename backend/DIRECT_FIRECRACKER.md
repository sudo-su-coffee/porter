# Direct Firecracker backend

The Porter backend now uses the official Firecracker process directly. One VMM process is started per replica with `--api-sock`; Porter sends the boot configuration as HTTP requests over that Unix domain socket and sends `InstanceStart` only after the kernel, root drive, network interface, and machine configuration are accepted. The control plane never opens a TCP Firecracker API port and does not require an OCI runtime or image daemon.

## Host artifact contract

Every deployable image must resolve to two host files:

```text
/var/lib/porter/vmlinux
/var/lib/porter/rootfs.ext4
```

The custom-image upload endpoint accepts a ZIP containing `vmlinux` and `rootfs.ext4`. Git deployments accept the same pair at the repository root or under `.porter/`; the backend copies them into `custom_images_dir` and registers a `custom://...` golden image. Dockerfiles, registry references, and source-only repositories are not converted automatically.

## Configuration

The direct runtime is configured in `porter.toml` or with environment overrides:

```toml
[firecracker]
runtime_mode = "direct"
api_socket_dir = "/run/porter/firecracker"
firecracker_bin = "firecracker"
kernel_image = "/var/lib/porter/vmlinux"
rootfs_path = "/var/lib/porter/rootfs.ext4"
logs_dir = "/var/log/porter"
custom_images_dir = "vms/custom"
```

The service account must be able to execute Firecracker, read the kernel and rootfs, create the API-socket and log directories, create TAP devices, and access `/dev/kvm`. The host must also assign the project gateway address to each TAP device and configure forwarding/NAT according to the host’s security policy.

## Replica boot sequence

1. The API resolves the selected direct golden image to `rootfs_path` and `kernel` metadata. A missing rootfs is rejected before an asynchronous boot is queued.
2. The network manager creates a deterministic TAP device, assigns the project gateway, and returns the guest CIDR and MAC address.
3. The runtime starts `firecracker --api-sock <per-vm-socket>` and waits for the Unix socket.
4. The runtime sends `PUT /boot-source`, `PUT /drives/rootfs`, `PUT /network-interfaces/eth0`, and `PUT /machine-config` over the Unix socket.
5. If a persistent volume is attached, the runtime sends an additional drive request.
6. The runtime sends `PUT /actions` with `{"action_type":"InstanceStart"}`, records the replica as running, and begins health checks.
7. Stop and shutdown terminate only the tracked Firecracker process and remove only its per-VM socket.

## Current boundaries

The backend intentionally does not provide in-guest command execution yet. SSH and console endpoints report that a guest-vsock agent is required. The next runtime extension should be a small guest agent over vsock, not a host-side task bridge. The sandbox can validate Unix-socket request shapes and lifecycle error handling, but real boot validation requires a Linux host with KVM, Firecracker, a compatible kernel/rootfs pair, and permissions to create TAP devices.

## Validation

Run the backend test suite with:

```bash
cd backend
gofmt -w cmd internal
go test ./...
```

The runtime tests use a temporary Unix socket HTTP server to verify Firecracker request payloads without launching a privileged microVM.

## References

1. [Firecracker official getting-started guide](https://github.com/firecracker-microvm/firecracker/blob/main/docs/getting-started.md)
