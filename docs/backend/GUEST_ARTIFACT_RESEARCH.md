# Firecracker guest-artifact research

## Finding

The official Firecracker project does not publish a universal production-ready
`vmlinux` plus `rootfs.ext4` bundle as part of the Firecracker hypervisor
release. The official documentation treats the guest kernel and root filesystem
as separate artifacts that the operator builds or obtains and validates for a
specific architecture and workload.

The upstream getting-started guide is:

<https://github.com/firecracker-microvm/firecracker/blob/main/docs/getting-started.md>

The upstream kernel/rootfs setup guide is:

<https://github.com/firecracker-microvm/firecracker/blob/main/docs/rootfs-and-kernel-setup.md>

That guide states that x86_64 Firecracker supports uncompressed `vmlinux` and
that a rootfs is a filesystem image with at least an init system. It describes
building kernels with the upstream `resources/rebuild.sh`/`devtool` recipes and
creating an ext4 image separately. Its example rootfs population path uses an
Alpine container as a build convenience; this does not imply that containerd or
an OCI runtime is used to boot the final VM.

## Porter release consequence

Porter must not use the Firecracker VMM release URL as the base-image URL. A
valid Porter base-image asset must contain both compatible `vmlinux` and
`rootfs.ext4` files, have a recorded SHA-256 digest, and be hosted at a verified
GitHub Release URL. The GitHub Actions release workflow therefore requires that
bundle as an explicit input and refuses to fabricate it.
