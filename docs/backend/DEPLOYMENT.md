# Porter deploy

The Backend workstream keeps its operational scripts in one group:

```
scripts/backend/
├── install.sh             production install (Linux + KVM, run as root)
├── install-linux.sh       source-tree Linux daemon installer with embedded Vue dashboard
├── dev.sh                 local dev (Docker for Postgres, real microVM boots)
├── install-firecracker.sh checksum-pinned Firecracker helper
├── install-porter.sh      verified GitHub Release installer
└── build-release.sh       daemon and base-image release builder
```

## Dev

```bash
cd /path/to/porter
bash scripts/backend/dev.sh up        # Docker Postgres + build + run (real boots)
bash scripts/backend/dev.sh down      # stop postgres (data kept)
bash scripts/backend/dev.sh clean     # down + remove ./bin
```

- Requires **Docker** (Postgres container) and **Go 1.25+**.
- Boots real microVMs through the official Firecracker HTTP API over per-VM Unix
  sockets on a host with `/dev/kvm`. There is no containerd, OCI runtime, or
  Firecracker shim in the Porter runtime path.
- Dashboard: http://localhost:8080. The initial database-seeded account is
  initialized with `PORTER_BOOTSTRAP_ADMIN_PASSWORD`.

## Linux beta (single Linux + KVM host)

For a published GitHub Release, the shortest path is one command:

```bash
curl -fsSL https://raw.githubusercontent.com/sudo-su-coffee/porter/main/scripts/backend/install-from-github.sh | sudo bash
```

The bootstrap downloads the architecture-specific Porter package and its
checksum sidecar, verifies the package, and delegates to the same installer
used by the source checkout. Set `PORTER_RELEASE_TAG` for a different release,
or set `PORTER_RELEASE_PACKAGE_SHA256` explicitly when the checksum sidecar is
not reachable. The release must include a real base-image bundle; the installer
will not fabricate one.

To create those release assets, use the repository’s **Actions → Porter Linux
release** workflow. Supply a pre-published GitHub Release URL for a real
`vmlinux`/`rootfs.ext4` bundle and its SHA-256 digest, or configure
`PORTER_BASE_IMAGE_URL` and `PORTER_BASE_IMAGE_SHA256` repository secrets before
pushing a `v*` tag. The workflow intentionally fails when that input is absent.

For a source checkout, use the daemon installer as root:

```bash
sudo PORTER_BASE_IMAGE_DIR=/var/porter/base-images/default \
  bash scripts/backend/install-linux.sh
```

The installer builds `frontend/` first, writes the result to `backend/web/dist`,
compiles the Go daemon so those assets are embedded through `backend/embed.go`,
installs the checksum-pinned official Firecracker binary from GitHub, creates a
`porter` system user, installs `release/porter.service`, and enables the daemon.
It refuses to claim a working microVM runtime unless the configured directory
contains real, non-empty `vmlinux` and `rootfs.ext4` artifacts. For control-plane
development only, `PORTER_ALLOW_MISSING_BASE_IMAGE=1` can bypass that refusal;
replicas will not boot in that mode.

After installation, the dashboard is served by the same Go process at
`http://127.0.0.1:8080` by default. The database-seeded super-admin username is
`admin`. The installer generates a random bootstrap password on first install,
stores it in `/var/porter/porter.env` with root ownership and daemon-group-only
read access, and prints it once at the end. The editable runtime configuration
is `/var/porter/porter.toml`; it contains paths and non-secret settings, while
database credentials and key material remain in the protected environment file.
There is **no reusable default password in the repository**. Rotate it after
the first login.

```bash
systemctl status porter.service
journalctl -u porter.service -f
```

## Prod (single Linux + KVM host)

```bash
sudo -E bash scripts/backend/install.sh  # everything, self-contained
/usr/local/bin/porter kernel set <path|URL>   # provision vmlinux
systemctl start porter
```

`install.sh` provisions PostgreSQL for development, checks KVM and TAP
prerequisites, downloads a checksum-pinned official Firecracker binary from
GitHub, validates a real Porter base-image bundle when configured, builds the
Go daemon, and writes direct-runtime configuration. Config lands in the local
installer state directory; credentials are supplied through
`PORTER_BOOTSTRAP_ADMIN_PASSWORD` and `PORTER_SECRET_KEY`, never TOML.

For a compiled GitHub release, run `scripts/backend/build-release.sh` with
`PORTER_BASE_IMAGE_DIR` pointing to a directory containing real `vmlinux` and
`rootfs.ext4`. The builder first compiles the Vue dashboard and embeds it in the
Linux Go binary, then creates a daemon package and a separate base-image package.
Upload both assets to the Porter GitHub Release, then install with
`scripts/backend/install-porter.sh` and a mandatory `PORTER_RELEASE_PACKAGE_SHA256`. No AWS
bucket or arbitrary mirror is used.

The current development sandbox does not expose `/dev/kvm` and does not contain
a user-supplied `vmlinux`/`rootfs.ext4` pair, so it can validate compilation,
embedding, checksums, scripts, and package structure but cannot honestly perform
a privileged Firecracker boot smoke test or produce a complete bootable guest
release without those real artifacts.

PostgreSQL stores Porter’s control-plane state. The Linux installer asks whether
to use PostgreSQL on the same host or a remote operator-managed PostgreSQL
service; it does not use Docker for the Linux installation and does not place
the database in a Firecracker guest. `/var/porter` contains the editable TOML,
protected environment file, image/artifact state, and local volume backing data.

## Where does PostgreSQL run in production?

**On the host — not inside a Firecracker microVM.** MicroVMs isolate *untrusted
tenant workloads*; the DB + control plane are trusted infrastructure. Postgres
is stateful + high-IOPS (WAL, fsync) and Firecracker has no persistent disk by
design — running a DB there means fighting the platform. Host Postgres (or
managed RDS/Neon) keeps backups, WAL archiving, and replication simple.
