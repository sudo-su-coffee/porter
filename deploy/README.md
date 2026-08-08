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
- Boots real microVMs via containerd + Firecracker on a host with `/dev/kvm`.
  The control plane + dashboard run anywhere; VM boots need the host runtime.
- Dashboard: http://localhost:8080  (admin / change-me).

## Prod (single Linux + KVM host)

```bash
sudo bash backend/deploy/install.sh      # everything, self-contained
/usr/local/bin/porter kernel set <path|URL>   # provision vmlinux
systemctl start porter
```

`install.sh` provisions PostgreSQL, containerd + devmapper, the `aws.firecracker`
shim, the firecracker VMM, CNI, builds the binary, and installs a systemd unit.
Config lands in `/etc/porter/porter.toml` (edit `api_token` + `admin_password`).

## Where does PostgreSQL run in production?

**On the host — not inside a Firecracker microVM.** MicroVMs isolate *untrusted
tenant workloads*; the DB + control plane are trusted infrastructure. Postgres
is stateful + high-IOPS (WAL, fsync) and Firecracker has no persistent disk by
design — running a DB there means fighting the platform. Host Postgres (or
managed RDS/Neon) keeps backups, WAL archiving, and replication simple.
