# GitHub-Only Artifact Distribution Findings

The official Firecracker `v1.16.1` GitHub release exposes architecture-specific hypervisor archives and SHA-256 digests for `firecracker-v1.16.1-x86_64.tgz` and `firecracker-v1.16.1-aarch64.tgz`. It does **not** publish a universal Porter guest image containing both `vmlinux` and `rootfs.ext4`.

Firecracker’s own getting-started guide separates these concerns: the release page is the supported source for the Firecracker binary, while a microVM also requires an uncompressed Linux kernel and an ext4 root filesystem. The upstream guide’s demonstration guest artifacts are sourced from Firecracker CI storage, which is intentionally not suitable for Porter’s requested distribution policy.

Therefore the secure GitHub-only Porter contract is:

| Artifact | GitHub source | Verification |
|---|---|---|
| Firecracker hypervisor | Official `firecracker-microvm/firecracker` release asset | Pinned version plus official GitHub SHA-256 digest |
| Porter Go daemon | A release asset in the Porter repository | Release manifest SHA-256 digest |
| `vmlinux` | A versioned base-image asset in the Porter repository | Release manifest SHA-256 digest |
| `rootfs.ext4` | A versioned base-image asset in the Porter repository | Release manifest SHA-256 digest |
| Combined base bundle, if used | A versioned `.tar.zst`, `.tar.gz`, or `.zip` asset in the Porter repository | Release manifest SHA-256 digest plus extracted-file validation |

The installer must derive URLs only from GitHub repository/release coordinates or accept an explicitly provided `https://github.com/.../releases/download/...` URL. It must reject AWS, arbitrary HTTP mirrors, boot-time downloads, missing digests, and archives that do not contain regular non-empty `vmlinux` and `rootfs.ext4` files.

References:

1. [Firecracker v1.16.1 official GitHub release](https://github.com/firecracker-microvm/firecracker/releases/tag/v1.16.1)
2. [Firecracker official getting-started guide](https://github.com/firecracker-microvm/firecracker/blob/main/docs/getting-started.md)
