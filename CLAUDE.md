# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What Porter is

Porter is a self-hosted **control-plane UI/API for Firecracker microVMs**. It deploys Docker/OCI images as kernel-isolated microVMs with automatic DNS, SSH, healthchecking/auto-replacement, and Vercel-style preview deploys. Pure-Go control plane, single binary, single-tenant by permanent design.

**Execution engine (the direction):** Porter controls **containerd** (not raw Firecracker) and uses the [`firecracker-containerd](https://github.com/firecracker-microvm/firecracker-containerd) shim (runtime `aws.firecracker`) to boot microVMs — image pull, snapshotting, jailer wiring and the in-VM agent are the shim's job. `firecracker-containerd` is AWS's own Fargate/Lambda runtime.

## Docs you can rely on (read these first, they are the truth)

There are only three root markdown files. `README.md` is the **single, central, full-detail reference** (architecture, API, compose rules, config, deployment, strategy). `versions.md` is the standalone roadmap (v0.1.0 → v1.0.0 → vision). `CLAUDE.md` is this file.

- `README.md` — **the** source of truth. Read the *Current Code State (Migration Status)* section before changing code: it distinguishes the **current** direct-Firecracker `backend/` from the **target** firecracker-containerd rewrite.
- `versions.md` — roadmap. v0.1.0-beta ("Core Runtime & First Deployment") is the current target.
- The old auxiliary docs used to live in `ARCHITECTURE.md`/`BUILD.md`/`INSTALL.md`/`implementation_plan.md` (now deleted); their content is consolidated into `README.md`. **Do not re-create them.**

## Direction & coding standards (what the maintainer wants)

- The backend should be written in Zerodha-style Go: a `server`/`worker` command split, `internal/...` packages (api, vmmanager, compose, config, store, netmgr, sse/sender/health, dns, gateway), an explicit `config.Load`, structured logging, a piped middleware chain, and graceful shutdown. The current `backend/` is a flat single `package main`, one responsibility per file — the **migration/rewrite should move toward the structured layout**. The README's architecture section describes that target.
- Single front door `backend/`, single binary, pure Go. Everything still builds/tests the same way.

## Build, test, run

The root `Makefile` drives everything. The Go binary **embeds** the built frontend via `go:embed web/dist`, so the frontend must be built first.

```bash
make frontend   # npm install + vite build → writes ../backend/web/dist (source-only, no bundle checked in)
make backend    # go build → backend/porter
make build      # frontend then backend
make run        # build + run backend/porter in foreground
make dev        # backend only: cd backend && go run . (run `npm run dev` separately for Vite hot-reload + API proxy)
make clean      # remove build artifacts, db, node_modules
```

Backend tests (the only tests in the repo):

```bash
cd backend && go test ./...
# single test:
cd backend && go test -run TestParseComposeBasic -v .
```

Runtime requirements (Linux host): KVM (`/dev/kvm`), a `firecracker` binary (current build) — or containerd + the `aws.firecracker` shim (after migration), a `vmlinux` kernel, and one `rootfs.ext4` per service (current build). `.env` in `backend/` documents the `PORTER_*` env vars (not auto-loaded; `export` them or use `direnv`). Build with Go 1.22+ (go.mod says 1.25.0). Currently one Go dependency (`modernc.org/sqlite`, pure-Go, no cgo); the migration adds `containerd` + OCI deps (`go mod tidy` on Linux first).

## Auth model (important)

UI and API share the same auth: **admin login gates a bearer token.** `POST /login` (`api.go`) checks the single `[admin]` username/password from `porter.toml` via `crypto/subtle` constant-time compare, then returns `api_token`. Every protected route is wrapped by `a.auth()` and requires `Authorization: Bearer <token>` against that same `api_token`. No user database, no session expiry, no lockout (just a fixed 300ms delay on bad login). Frontend stores the token in `localStorage` (`frontend/src/api/client.js`). In production the Control API must be trusted-network-only.

## Architecture — how it's actually put together right now

The checked-in backend is one Go package `main` in `backend/`, one component per file (stdlib only plus `modernc.org/sqlite`) — **currently the older direct-Firecracker design, mid-migration** to the firecracker-containerd + structured layout described in the README:

- `api.go` — HTTP server, Go 1.22+ pattern routing on `net/http.ServeMux` (no router dep). Registers all routes in `Routes()`, wires `a.auth()`, defines the `API` struct holding every dependency. Endpoints include `GET/POST /vms`, `POST /vms/{id}/stop|start`, `DELETE /vms/{id}`, projects, `/login`, `/events`.
- `vmmanager.go` — spawns one `firecracker --api-sock <path>` OS process per VM, configures it through `fcapi.go`, tracks a `runningVM` (cmd + socket), handles boot (boot source, rootfs drive, network interface, machine config, `InstanceStart`) and graceful stop (CtrlAltDel → SIGTERM → SIGKILL). Health is TCP-connect-only via `probeHealth`.
- `fcapi.go` — `FCClient`, a minimal `net/http` client over each VM's Unix-domain API socket (six small PUT requests; replaces `firecracker-go-sdk`).
- `store.go` — `Store` over SQLite (`porter.db`, `modernc.org/sqlite`, no cgo). Each table is `id → JSON blob` (not normalized) so a field addition is one-place. Also an in-memory traffic ring buffer per VM.
- `compose.go` — `ParseCompose`, a hand-rolled indentation parser for a constrained Compose v3 subset (no YAML dep), topological sort on `depends_on`, rejects `build:` and circular deps. `compose_test.go` covers it.
- `netmgr.go` — per-project `/24` subnets (`10.42.N.0/24`) and `tap` device creation via `ip tuntap`. Networking is **half-wired** (tap only; no bridge/NAT by default).
- `sse.go` — `Hub` for Server-Sent-Events, broadcasts live VM state to the dashboard over `GET /events`.
- `config.go` + `toml.go` — config from `porter.toml` (`[server]`, `[firecracker]`, `[admin]`) with `PORTER_*` env overrides; `ParseTOML` is a hand-rolled subset parser. `main.go` refuses to start without `api_token` + admin password.
- `types.go` — the canonical `VM`/`Project`/`Domain`/etc. structs with JSON tags. VM states: `pending`/`booting`/`running`/`stopping`/`stopped`/`failed`; health: `healthy`/`unhealthy`/`checking`.

**Frontend** (`frontend/`): Vue 3 + Vite + vue-router, dev-proxying API to `localhost:8080` (see `vite.config.js`). `src/api/client.js` is the auth'd `fetch` wrapper (401 → `/login` redirect); `src/api/events.js` consumes the SSE stream. Views: `DeploymentsList`, `ProjectDetail`, `VmDetail`, `Login`. Reusable components in `src/components/`. The README's Dashboard spec lists the target UI (traffic table, domains panel, logs, image picker, log/status UI).

## When changing code, respect the migration seam

- Read `README.md → Current Code State` before touching `vmmanager.go`/`fcapi.go`/`config.go`: those are the ones slated to change in the firecracker-containerd migration.
- Don't spread the complexity across files; match the Zerodha-style `internal/` package move if you're restructuring.
- Keep the README's API + compose-mapping docs accurate when you change behavior — their tests (`compose_test.go`) must stay green.