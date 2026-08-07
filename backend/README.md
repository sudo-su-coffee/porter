# Porter backend

Porter is a self-hosted **control-plane UI/API for Firecracker microVMs**: it
deploys Docker/OCI images as kernel-isolated microVMs with automatic DNS, SSH,
healthchecking/auto-replacement, and Vercel-style preview deploys. This folder
is the entire backend — pure Go, one binary, one entrypoint (`cmd/porter`).

```
backend/
├── cmd/porter/main.go      entrypoint: `porter server|worker|kernel|version`
├── internal/               packages (see Layout below)
├── migrations/             SQL migrations, embedded into the binary (*.sql)
├── web/dist/               built Vue dashboard, embedded via go:embed
├── deploy/                 install.sh (prod) + dev.sh (dev) + host/systemd
├── Makefile                build / run / dev / test / clean
├── go.mod / go.sum
└── porter.toml             optional runtime config (env vars override it)
```

## Quick start

**Dev** (Docker for Postgres, real microVM boots):

```bash
cd backend
bash deploy/dev.sh up        # starts Postgres container, builds, runs server
# dashboard: http://localhost:8080  (admin / change-me)
```

**Prod** (single Linux + KVM host):

```bash
sudo bash backend/deploy/install.sh
/usr/local/bin/porter kernel set <vmlinux-path-or-url>
systemctl start porter
```

**Manual build** (frontend must be built first — the binary embeds `web/dist`):

```bash
make frontend   # from backend/: cd ../frontend && npm install && npm run build
make build      # -> backend/bin/porter
```

**Tests:**

```bash
cd backend && go test ./...
```

## Layout

| Package | Responsibility |
|---|---|
| `internal/api` | HTTP control-plane API — all routes + handlers (orgs, projects, replicas, domains, env, traffic, logs, …) |
| `internal/store` | PostgreSQL persistence (`pgx/v5`), migrations, in-memory traffic/log rings. Only package allowed to touch pgx. |
| `internal/vmmanager` | containerd + `aws.firecracker` shim lifecycle (pull → create → start → stop). Only package allowed to touch containerd. |
| `internal/netmgr` | per-project `/24` subnets, static replica IPs, deterministic MACs, CNI config for `tc-redirect-tap` |
| `internal/gateway` | host-routing reverse proxy (Vercel-style) + per-VM traffic ring; records traffic to the store for the dashboard |
| `internal/dns` | embedded `.local` resolver — `<svc>.<project>.local` / `<svc>-<n>` → replica IPs |
| `internal/health` | HTTP/TCP healthchecker; auto-replaces unhealthy VMs via `vmmanager` |
| `internal/sshgw` | SSH gateway (short-lived CA-signed user certs); needs a `task.Exec` bridge (not yet wired) |
| `internal/event` | Server-Sent-Events hub for live dashboard updates |
| `internal/config` | `porter.toml` + `PORTER_*` env overrides |
| `internal/compose` | constrained Compose v3 parser + topological sort (no YAML dep) |
| `internal/imagecatalog` | on-disk image library for the dashboard picker |
| `internal/runtime` | minimal direct-Firecracker client for *bare* (rootfs) images |

### Wiring status

- **Live in `porter server`:** control API, dashboard, embedded workers, Postgres
  migrations, VM lifecycle (containerd for OCI, direct Firecracker for rootfs), netmgr.
- **Opt-in via config:** host-routing `gateway` (`[gateway] enabled`), `.local`
  `dns`, and `health` auto-replace. The gateway gets its own listener so tenant
  traffic never shares the control-plane port.
- **Behind a flag / not yet wired:** `sshgw` needs a `containerd task.Exec`
  bridge; it is intentionally not started until that adapter lands.

## Configuration

Config comes from `porter.toml` (default `porter.toml`, override with
`-config` or `PORTER_CONFIG`), layered with `PORTER_*` env vars. A missing file
is fine — env vars carry the whole config. `api_token` and `admin_password`
are **required**.

| Env var | Default | Meaning |
|---|---|---|
| `PORTER_LISTEN_ADDR` | `:8080` | control-plane API + dashboard |
| `PORTER_BASE_DOMAIN` | — | base domain for preview domains |
| `PORTER_API_TOKEN` | — | **required** — token for all API calls |
| `PORTER_DATABASE_URL` | `postgres://porter:porter@localhost:5432/porter?sslmode=disable` | PostgreSQL DSN (always required) |
| `PORTER_AUTO_MIGRATE` | `true` | run `migrations/*.sql` at startup |
| `PORTER_CONTAINERD_SOCKET` | `/run/containerd/containerd.sock` | containerd socket for real boots |
| `PORTER_SNAPSHOTTER` | `devmapper` | containerd snapshotter |
| `PORTER_NAMESPACE` | `porter` | containerd namespace |
| `PORTER_LOGS_DIR` | `/var/log/porter` | per-VM stdio logs |
| `PORTER_IMAGES_DIR` | `vms/images` | image-catalog manifests dir |
| `PORTER_ADMIN_USERNAME` / `PORTER_ADMIN_PASSWORD` | `admin` / — | dashboard login (password required) |
| `PORTER_GATEWAY_ENABLED` / `PORTER_GATEWAY_LISTEN_ADDR` | `false` / `:80` | host-routing traffic gateway |
| `PORTER_DNS_ENABLED` | `false` | `.local` resolution for the gateway |
| `PORTER_HEALTH_ENABLED` | `false` | healthcheck + auto-replace |
| `PORTER_SSH_ENABLED` / `PORTER_SSH_LISTEN_ADDR` | `false` / `:2222` | SSH gateway (not yet wired) |

## Commands

```
porter server            API + embedded workers (default 1)
porter server -workers 0 API only
porter worker            lifecycle workers only, no HTTP
porter kernel set <url|path>   provision the shared vmlinux
porter version / --version
```

## PostgreSQL in production?

Run PostgreSQL **on the host** (what `deploy/install.sh` provisions) or point
`PORTER_DATABASE_URL` at managed Postgres. Do **not** run it inside a Firecracker
microVM — microVMs isolate *untrusted tenant workloads*; the DB + control plane
are trusted infrastructure, and Firecracker's diskless, read-only-rootfs model is
a poor fit for a stateful, high-IOPS database. Full reasoning in `deploy/README.md`.
