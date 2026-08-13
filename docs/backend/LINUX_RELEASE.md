# Porter Linux v1.0.0-beta release checklist

Porter’s Linux beta is a single Go daemon that serves the Vue dashboard from embedded `backend/web/dist` assets. The daemon listens on `:8080` by default, so the local dashboard URL is `http://127.0.0.1:8080`. The runtime is direct Firecracker over per-VM Unix-domain HTTP sockets; no containerd, OCI runtime, Firecracker shim, or claim that an arbitrary OCI image can boot as a microVM is part of this release.

## First-run identity

Database migration `0012_seed_rbac_admin` creates the username `admin` with an empty password hash. Migration `0015_seed_super_admin` promotes that persisted identity to the `super_admin` role and gives it owner membership in the default organization. On first daemon startup, `PORTER_BOOTSTRAP_ADMIN_PASSWORD` initializes the empty hash exactly once. Existing password hashes are never overwritten.

There is intentionally **no fixed default password**. `install-linux.sh` or `install-porter.sh` generates a random bootstrap password if one was not supplied, writes it to `/var/porter/porter.env`, and prints it once. The editable non-secret runtime configuration is `/var/porter/porter.toml`. The environment file is owned by `root:porter` with mode `0640`, and the user should rotate the password after the first login. For an operator-controlled password, export `PORTER_BOOTSTRAP_ADMIN_PASSWORD` before installation.

## One-command installation

After the release package and its checksum sidecar are published, a Linux user
can install without cloning the repository:

```bash
curl -fsSL https://raw.githubusercontent.com/sudo-su-coffee/porter/main/scripts/backend/install-from-github.sh | sudo bash
```

The script verifies the release archive, runs the PostgreSQL choice, installs
the Go daemon and embedded dashboard, verifies Firecracker, writes `/var/porter`,
enables systemd, and prints a readiness table. The final table separates API
and database startup from KVM/TAP and real guest-artifact readiness.

## Source-tree installation

```bash
sudo PORTER_BASE_IMAGE_DIR=/var/porter/base-images/default \
  bash scripts/backend/install-linux.sh
```

The installer performs the following deterministic steps. It builds the Vue dashboard, embeds it into the Go binary, creates the `porter` service account and writable state directories, verifies the official Firecracker release by pinned SHA-256, installs the systemd unit, writes environment/configuration files, and starts the daemon. It refuses to run as a complete microVM installation until real non-empty `vmlinux` and `rootfs.ext4` files are available.

The GitHub bootstrap caches the verified daemon archive and checksum under
`/var/cache/porter/releases` by default. A rerun reuses the archive when the
stored SHA-256 matches; an incomplete or corrupt archive is removed and fetched
again. Override the cache directory with `PORTER_CACHE_DIR` when needed. If the
installer is invoked through `curl | sudo bash`, PostgreSQL mode and remote URL
prompts are read from `/dev/tty` so they remain interactive.

## Data storage choices

Porter separates control-plane state from guest artifacts. PostgreSQL stores users, database-seeded RBAC, organizations, projects, deployments, domains, variables, settings, and other API resources. The installer asks whether PostgreSQL should be installed and managed on the same Linux host or whether the operator will provide a reachable remote PostgreSQL URL. The Linux installer does not start Docker and does not place PostgreSQL inside a Firecracker guest. A remote database is useful when the operator already has managed PostgreSQL or a separate database server, but the URL, credentials, firewall, TLS mode, backups, and availability remain the operator’s responsibility.

The local filesystem under `/var/porter` stores the editable `porter.toml`, protected `porter.env`, verified Firecracker/base-image material, image manifests, custom image bundles, runtime sockets through `/run/porter`, logs, and persistent volume backing data. A Porter tenant volume is host-side state attached to that tenant’s microVM; it is not automatically copied to, or shared with, another server. Multi-host storage requires an operator-managed shared storage layer and a reviewed attachment implementation, which is outside this beta.

## GitHub release packaging

The repository includes `.github/workflows/release.yml`. It supports two
triggers. A maintainer can use **Actions → Porter Linux release → Run workflow**
and provide the release tag and architecture. Or a maintainer can push a `v*`
tag. In both cases, the workflow reads the real guest files from
`release/guest-artifacts/<architecture>/vmlinux` and
`release/guest-artifacts/<architecture>/rootfs.ext4`, calculates their SHA-256
values, and creates the daemon and base-image archives plus checksum sidecars.

Because these files are around 50 MB, the preferred path is a separate GitHub
Release named `base-images-v1.0.0-beta-dev` with two assets named exactly
`vmlinux` and `rootfs.ext4`. The workflow downloads those assets when the
repository folder is empty and calculates SHA-256 values automatically. For
smaller files, Git LFS-managed files in
`release/guest-artifacts/<architecture>/` are also supported. Do not add
placeholders. The Firecracker VMM binary remains separate: the Linux installer
downloads and verifies the pinned official Firecracker release.

```bash
PORTER_RELEASE_TAG=v1.0.0-beta \
PORTER_BASE_IMAGE_DIR=/path/to/real/base-image \
  bash scripts/backend/build-release.sh v1.0.0-beta x86_64
```

The directory supplied through `PORTER_BASE_IMAGE_DIR` must contain the actual guest kernel and ext4 root filesystem. Firecracker’s own GitHub release contains the hypervisor binary, not a universal Porter guest rootfs. The builder produces a daemon archive, a separate base-image archive, a manifest, and SHA-256 sums. The base-image archive must be uploaded alongside the daemon archive to the GitHub Release before using `install-porter.sh`.

## Validation boundary

The repository can validate Go compilation, embedded dashboard assets, Vue production builds, Firecracker checksum metadata, shell syntax, migration wiring, and archive integrity in the sandbox. It cannot validate privileged KVM/TAP boot behavior here because `/dev/kvm` is unavailable, and it cannot fabricate a guest image. Before announcing a fully boot-tested beta, run the installer and one real deploy on an operator Linux host with KVM, TAP permissions, PostgreSQL, the pinned Firecracker binary, and a compatible `vmlinux`/`rootfs.ext4` pair.
