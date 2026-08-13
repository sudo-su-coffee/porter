# Porter Firecracker and Base-Image Artifact Plan

Porter boots direct Firecracker microVMs. The release binary and the guest artifacts are separate concerns: **Firecracker is the hypervisor process**, while `vmlinux` and `rootfs.ext4` are the guest boot artifacts. Porter never pulls a Docker image and hands it directly to Firecracker.

## Where artifacts live

The installer keeps runtime-owned artifacts under the configured Porter state directory. In development, the direct installer defaults to `.dev/state`; in a host installation, use `/var/lib/porter` or another root-owned state directory.

| Artifact | Default location | Ownership | Used when |
|---|---|---|---|
| Versioned Firecracker binary | `$PORTER_STATE_DIR/bin/firecracker-v1.16.1-<arch>` | Porter install owner, executable | Every VM boot |
| Stable binary symlink | `$PORTER_STATE_DIR/bin/firecracker` | Porter install owner | TOML `firecracker_bin` |
| Shared base kernel | `$PORTER_STATE_DIR/base-images/default/vmlinux` | root-owned, readable by Porter | `base://default` boots |
| Shared base rootfs | `$PORTER_STATE_DIR/base-images/default/rootfs.ext4` | root-owned, readable by Porter | `base://default` boots |
| Custom image bundles | `$PORTER_STATE_DIR/custom/<name>/` | root-owned, writable only through the API upload boundary | `custom://<name>` boots |
| Image catalog manifests | `$PORTER_STATE_DIR/images/*.json` | root-owned | Catalog discovery and readiness |
| Per-VM API sockets | `$PORTER_STATE_DIR/sockets/` | Porter runtime owner | Firecracker HTTP API |
| Per-VM logs | `$PORTER_STATE_DIR/logs/` | Porter runtime owner | Operator logs and diagnostics |

The **database stores image metadata**, including reference, artifact paths, architecture, SHA-256 digests, status, and validation time. It does not store rootfs or kernel bytes. The state directory stores the large artifacts, and the database points to them.

## What happens during installation

`scripts/backend/install.sh` builds the Porter binary, creates the state directories, installs a pinned official Firecracker release through `scripts/backend/install-firecracker.sh`, writes TOML paths pointing to local files, and checks whether the base image exists. It does **not** download an arbitrary rootfs by default.

To provision a base image, provide a user-owned ZIP or tar archive containing exactly these required files at its root:

```text
vmlinux
rootfs.ext4
```

Set `PORTER_GITHUB_REPOSITORY`, `PORTER_RELEASE_TAG`, `PORTER_BASE_IMAGE_ASSET`, and the mandatory `PORTER_BASE_IMAGE_SHA256` before running the installer. The installer derives a URL of the form `https://github.com/<owner>/<repo>/releases/download/<tag>/<asset>`, downloads the bundle once, verifies its SHA-256 digest, extracts it into `base-images/default`, and refuses to boot until both files are regular, non-empty files. A direct `PORTER_BASE_IMAGE_URL` is accepted only when it is a GitHub Release download URL. The runtime never downloads from the network during a VM boot.

## Stable and fallback Firecracker versions

The repository pins official Firecracker **v1.16.1** as stable and **v1.16.0** as the fallback. Their architecture-specific release URLs and SHA-256 values are recorded in [`../../release/firecracker-versions.json`](../../release/firecracker-versions.json). The official release page lists v1.16.1 and provides `firecracker-v1.16.1-x86_64.tgz` and `firecracker-v1.16.1-aarch64.tgz` assets with checksum sidecars.[^1]

`scripts/backend/install-firecracker.sh` verifies an existing local binary first, then downloads the pinned archive, verifies the archive before extraction, installs a versioned binary, and creates a local `firecracker` symlink. A fallback version is only selected by the installer when explicitly enabled with `PORTER_ALLOW_FIRECRACKER_FALLBACK=1`; a checksum mismatch is always fatal.

## Release and upgrade policy

Porter’s compiled Go binary can be copied independently of the artifact directory, but a deployment is only bootable when the referenced kernel and rootfs remain at their recorded paths and their digests still match. Upgrades should therefore stage a new versioned binary or base-image directory, verify it, update the database manifest, and only then change the default `base_image_ref`. Never replace an in-use rootfs in place.

For offline installation, pre-populate the state directory and set `PORTER_FIRECRACKER_DIR`, `PORTER_KERNEL_PATH`, and `PORTER_ROOTFS_PATH` to the staged files. The installer validates local artifacts before attempting any remote download. If a remote base bundle is required, it must be an asset in the configured Porter GitHub Release and must have an explicit SHA-256 digest. No AWS bucket or arbitrary mirror is part of the release path.

`scripts/backend/build-release.sh` builds two GitHub Release assets when a real base image is supplied: `porter-<tag>-<arch>.tar.gz`, containing the compiled Go daemon, installer helpers, release metadata, and a copy of the verified base image; and `porter-base-image-<tag>-<arch>.tar.gz`, containing only `vmlinux` and `rootfs.ext4` for hosts that already have the daemon. The script refuses to create either release package without non-empty guest artifacts.

## Important boundary

Firecracker’s official release supplies the hypervisor binary, not a universal application rootfs. A Porter base rootfs must be built and maintained as a compatible Linux guest image with the required network, init, filesystem, and optional guest-agent behavior. Docker images can be used as a **source for building** such a rootfs in a separate image pipeline, but the resulting `rootfs.ext4` and matching `vmlinux` are what Porter registers and boots.

[^1]: [Firecracker official releases](https://github.com/firecracker-microvm/firecracker/releases)
