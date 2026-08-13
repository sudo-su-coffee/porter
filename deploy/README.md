# Porter deploy

Two entrypoints, that's it:

```
deploy/
├── install.sh      production install (Linux + KVM, run as root) — self-contained
├── dev.sh          local dev (Docker for Postgres, real microVM boots)
├── porter.toml     production config template
└── README.md       this file
```

## Dev

```bash
cd backend
bash deploy/dev.sh up        # Docker Postgres + build + run (real boots)
bash deploy/dev.sh down      # stop postgres (data kept)
bash deploy/dev.sh clean     # down + remove ./bin
```

- Requires **Docker** (Postgres container) and **Go 1.25+**.
- Boots real microVMs through the official Firecracker HTTP API over per-VM Unix
  sockets on a host with `/dev/kvm`. There is no containerd, OCI runtime, or
  Firecracker shim in the Porter runtime path.
- Dashboard: http://localhost:8080. The initial database-seeded account is
  initialized with `PORTER_BOOTSTRAP_ADMIN_PASSWORD`.

## Prod (single Linux + KVM host)

```bash
sudo bash backend/deploy/install.sh      # everything, self-contained
/usr/local/bin/porter kernel set <path|URL>   # provision vmlinux
systemctl start porter
```

`install.sh` provisions PostgreSQL for development, checks KVM and TAP
prerequisites, downloads a checksum-pinned official Firecracker binary from
GitHub, validates a real Porter base-image bundle when configured, builds the
Go daemon, and writes direct-runtime configuration. Config lands in the local
installer state directory; credentials are supplied through
`PORTER_BOOTSTRAP_ADMIN_PASSWORD` and `PORTER_SECRET_KEY`, never TOML.

For a compiled release, run `release/build-release.sh` with
`PORTER_BASE_IMAGE_DIR` pointing to a directory containing real `vmlinux` and
`rootfs.ext4`. It creates a daemon package and a separate base-image package.
Upload both assets to the Porter GitHub Release, then install with
`install-porter.sh` and a mandatory `PORTER_RELEASE_PACKAGE_SHA256`. No AWS
bucket or arbitrary mirror is used.

## Where does PostgreSQL run in production?

**On the host — not inside a Firecracker microVM.** MicroVMs isolate *untrusted
tenant workloads*; the DB + control plane are trusted infrastructure. Postgres
is stateful + high-IOPS (WAL, fsync) and Firecracker has no persistent disk by
design — running a DB there means fighting the platform. Host Postgres (or
managed RDS/Neon) keeps backups, WAL archiving, and replication simple.
