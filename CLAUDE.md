# CLAUDE.md

Guidance for Claude Code when working in this repository.

## First principle (maintainer's directive)

**Never degrade — every change must make the project strictly better.**
No "shipping a stub that looks done," no dropping a working feature to widen
scope, no trading real functionality for a nicer-looking but hollow surface.
When a task can't be fully completed honestly, ship the smallest *real*
piece and mark the rest plainly — never a fake `200` + empty JSON standing in
for a feature. If something in this file contradicts what the code actually
does, trust the code and fix this file.

## What Porter is

A self-hosted **control-plane UI/API for Firecracker microVMs** — your own
Vercel / Fly.io, on one box. You give it a Docker/OCI image (or
`docker-compose.yml`); it boots each deploy as a **kernel-isolated microVM**
with DNS, SSH, healthchecking/auto-replace, traffic logging, and preview
domains — all from a Vue 3 dashboard + REST API. Pure-Go control plane, single
binary, single-tenant by permanent design.

**Execution engine:** OCI images boot through **containerd** using the
[`aws.firecracker`](https://github.com/firecracker-microvm/firecracker-containerd)
shim (image pull, snapshots, jailer wiring, in-VM agent = the shim's job — the
same runtime AWS Fargate/Lambda run on). "Bare" rootfs+vmlinux images are the
one exception: `internal/runtime` drives the Firecracker API socket directly.
There is **no Docker daemon** in Porter's own logic, and no Kubernetes.

## Docs (read first — they are the truth)

Only two root markdown files matter:

- `README.md` — the single, central, source-of-truth reference (architecture,
  API catalog, compose rules, config, failure handling, Roadmap/Planned).
  Re-verified against source on every revision → **wins any disagreement.**
- `PLAN.md` — the phased roadmap with per-feature `[DONE]` / `[PARTIAL]` /
  `[PLANNED]` / `[STUBBED]` tags. If it and README disagree, README wins.

**Before claiming "X is shipped," verify it** (mandatory): read the handler in
`backend/internal/api/handlers_impl.go` — empty JSON = stub; check `git log
--oneline -n 15`; check the migration ceiling in `backend/migrations/*.sql`.

## Golden rules (never break)

1. **PostgreSQL-only storage.** Every durable write goes through
   `internal/store` (pgx). No SQLite, no MySQL, no on-disk JSON stores.
2. **Firecracker is the only runtime.** Real microVMs via containerd +
   `aws.firecracker` shim (OCI) or direct Firecracker API socket (bare
   rootfs). No simulated/spoofed boots.
3. **Redis = cache/queue only** (`internal/cache`, off by default). Postgres
   stays the source of truth.
4. **Use proven Go libraries, don't hand-code.** DNS → `miekg/dns`; SSH
   bridge → `golang.org/x/crypto/ssh`; everything else existing in `go.mod`.
   Own code = control-plane glue, never reimplementations.
5. **UI-only product.** End users drive the dashboard; no user-facing CLI.
   `cmd/porter` subcommands are operator/installer internals.
6. **App runs offline** — images referenced by URL/OCI ref, never local paths.

## Build, test, run

The root `Makefile` drives everything. The Go binary **embeds** the built
frontend via `go:embed web/dist`, so the frontend must be built first.

```bash
make frontend   # cd frontend && npm install && npm run build → backend/web/dist
make backend    # cd backend && go build -o porter ./cmd/porter
make build      # frontend then backend (single binary)
make run        # build + run ./backend/porter server (default subcommand)
make dev        # prints the two-terminal loop: backend `go run ./cmd/porter` + Vite :5173
make migrate    # golang-migrate up against backend/migrations (PORTER_DATABASE_URL override ok)
make test       # cd backend && go test ./...
make clean      # remove artifacts, db, node_modules
```

Backend tests:

```bash
cd backend && go test ./...          # or go vet ./...
cd backend && go test -run TestParseComposeBasic -v ./internal/compose
```

Runtime requirements (Linux host): KVM (`/dev/kvm`), PostgreSQL, containerd +
the `aws.firecracker` shim (OCI boots), a `vmlinux` kernel + rootfs (bare
boots). Windows dev machine: code/compile/test fine; real VM boots need a
Linux host. Default DSN: `postgres://porter:porter@localhost:5432/porter?sslmode=disable`
(`deploy/dev.sh` provisions it). Go 1.25 (go.mod).

## Config & auth

- **Config:** `porter.toml` (root of `backend/`; see `porter.toml`) with
  `PORTER_*` env overrides. `config.LoadConfig` refuses to start without
  `[database] url`, `[server] api_token`, and `[admin] password`. Optional
  sections: `[gateway]`, `[dns]`, `[health]`, `[ssh]`, `[cache]` (Redis, off
  by default), `[tls]` (ACME), `[autoscale]`, `[notify]` (SMTP). Exact
  struct: `backend/internal/config/config.go`.
- **Auth:** `POST /login` checks the bootstrap admin (`[admin]` username/
  password from porter.toml) via `crypto/subtle`, returns a bearer token.
  Per-user accounts live in the `users` table; per-user API tokens + fine-
  grained permission codes guard routes (`internal/api/rbac`). The frontend
  sends `Authorization: Bearer <token>` **and** an `X-CSRF-Token` (fetched
  from `GET /csrf`) on writes. Trusted-network-only in production.

## Architecture (`backend/`, structured `internal/` packages)

Entrypoint `backend/cmd/porter/main.go` wires everything in `runServer`:
store → event hub → VM engine → gateway/DNS/TLS/health/SSH → HTTP mux on
:8080 (dashboard embedded). Key packages:

| Package | Responsibility |
|---|---|
| `internal/store` | Postgres store (pgx). Rows are `id → JSON blob` + typed columns for querying. Migrations in `backend/migrations/*.sql`. Redis read-through via `SetCache`. |
| `internal/api` | `api.go` (route registration ~281 routes, `Routes()`, auth/RBAC middleware, rate limit) + `handlers_impl.go` (the handlers — **check here for stubs**). |
| `internal/runtime` | `VMManager`: boot/stop/exec VMs, containerd OCI path + bare Firecracker path, per-project subnet wiring. |
| `internal/netmgr` | Single subnet/IP/MAC allocator (per-project /24, host tap). `internal/net` was removed — don't reintroduce. |
| `internal/compose` | Hand-rolled `ParseCompose` (no YAML dep): image-only services, acyclic `depends_on`, rejects `build:`. |
| `internal/event` | SSE hub — `GET /events`, live VM state to dashboard. |
| `internal/gateway` | Host-routing reverse proxy + traffic logger; `portforward.go` binds compose `HostPort`→VM. |
| `internal/dns` | `server.go` real UDP/TCP authoritative DNS (miekg/dns) for `*.baseDomain`; `domains.go` auto-assigns preview/prod. |
| `internal/tls` | `autocert.go` — Let's Encrypt ACME, HTTP-01, cert cache. |
| `internal/health` | Per-VM healthcheck probes + auto-replace. |
| `internal/cache` | Optional Redis read-through (no-op when off). |
| `internal/volumes` | Real host dir + sparse `data.img` persistent volumes. |
| `internal/imagecatalog` | Image library + golden seeds (redis/postgresql/mysql). |
| `internal/metrics` | CPU/mem sampler → `metrics_samples`. |
| `internal/cron` | 5-field cron scheduler booting job microVMs. |
| `internal/autoscale` | Horizontal autoscaler on replica pools. |
| `internal/notify` | SMTP email notifications. |
| `internal/sshgw` | SSH gateway bridging into VM tasks via `task.Exec` (no sshd in guest). |
| `internal/startup` | Startup sanity check (shim/jailer/KVM) — fails loudly. |
| `internal/types` | Canonical structs (`Project`, `VM`/replica, etc.) with JSON tags. |
| `internal/config` | TOML + env config load. |

**Frontend** (`frontend/`): Vue 3 + Vite + vue-router, dev-proxies `/api` to
:8080 (`vite.config.js`). Auth'd client in `src/api/`, SSE consumer in
`src/api/events.js`. Reusable components in `src/components/`. Design tokens
in `src/style.css`.

## Known stubs / partials (verify before claiming)

A meaningful slice of the huge API surface returns correct-but-empty JSON
rather than real logic — analytics/web-vitals, redirects, microfrontends,
cache-purge, and similar Vercel-parity routes. README § API + the handler
source are the truth. Don't "complete" a stub by faking data; wire real
subsystems or leave the honest stub.

## Contribution rules (maintainer's workflow)

- **Windows:** use Read/Glob/Grep for exploring code, not bash.
- Write code in one pass; **no compile in the loop**; run `go vet ./...` +
  `go test ./...` once at the end. Commit only when asked.
- Postgres only; images by URL/OCI ref (must work offline); honest status —
  never fake a feature.
- Front-end work also needs the SSE events + CSRF conventions above.
- A richer, always-current status guide lives in the
  `porter-paas-expert` skill (`.claude/skills/porter-paas-expert/SKILL.md`) —
  invoke `/porter-paas-expert` for feature-status answers.