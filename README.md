<p align="center">
  <img width="80" height="80" alt="Porter Logo" src="/assets/porterlogo.png" />
</p>

# Porter

```
██████╗  ██████╗ ██████╗ ████████╗███████╗██████╗
██╔══██╗██╔═══██╗██╔══██╗╚══██╔══╝██╔════╝██╔══██╗
██████╔╝██║   ██║██████╔╝   ██║   █████╗  ██████╔╝
██╔═══╝ ██║   ██║██╔══██╗   ██║   ██╔══╝  ██╔══██╗
██║     ╚██████╔╝██║  ██║   ██║   ███████╗██║  ██║
╚═╝      ╚═════╝ ╚═╝  ╚═╝   ╚═╝   ╚══════╝╚═╝  ╚═╝
```

> **The self-hosted PaaS.** Deploy Docker images and `docker-compose` apps as kernel-isolated microVMs — automatic DNS, instant SSH, live logs, real-time traffic, and Vercel-style previews. Your own **Vercel** or **Fly.io**, on one box, without Docker or Kubernetes.

Better isolation than plain Docker (each deploy is its own microVM), far simpler than Kubernetes (one binary, one host, zero cluster). The engine is [`firecracker-containerd`](https://github.com/firecracker-microvm/firecracker-containerd) — the same runtime AWS runs Fargate and Lambda on — so every deploy gets a hardware-isolated microVM with near-container boot times (~125 ms).

Think Vercel or Fly.io, self-hosted. Porter is the control-plane UI/API on top of that VM engine — not a from-scratch VMM orchestrator, and not just a "microVM spinner." The isolation is the machinery; the platform (deploys, domains, logs, traffic, scaling) is the point.

MIT licensed. Self-hosted. Single-tenant by permanent design.

> **This README is the single source of truth for Porter** — it consolidates what previously lived across several auxiliary docs into one reference. The product roadmap lives in [`versions.md`](./versions.md).

---

## Contents

1. [What Porter Does](#-what-porter-does)
2. [Quickstart](#-quickstart)
3. [Runtime Architecture](#-runtime-architecture)
4. [Current Code State (Migration Status)](#-current-code-state-migration-status)
5. [API Reference](#-api-reference)
6. [Compose Mapping Rules](#-compose-mapping-rules)
7. [Domains & Traffic Log](#-domains--traffic-log)
8. [SSH Access](#-ssh-access)
9. [Dashboard UI Spec](#-dashboard-ui-spec)
10. [Installation, Deployment & Local Development](#-installation-deployment--local-development)
11. [Configuration Reference](#-configuration-reference)
12. [Roadmap](#-roadmap)
13. [OSS & Future SaaS Strategy](#-oss--future-saas-strategy)
14. [License](#-license)

---

## 🚀 What Porter Does

Porter brings the pieces of Fargate, Kubernetes, Vercel, and Fly.io that actually matter for a single self-hosted box, into one pure-Go binary:

- **Scale like Fargate** — `deploy.replicas: 3` in your compose file, or `porter scale api 5` any time, and Porter boots identical, isolated microVMs to match
- **Heal like Kubernetes** — declare a `healthcheck:`, set `restart: on-failure`, and Porter probes every replica, drains unhealthy ones out of traffic immediately, and replaces them automatically
- **Discover like Kubernetes DNS** — every service gets a real name (`db.my-app.local`) that resolves to its healthy replicas, no manual IP wiring
- **Deploy like Vercel** — wildcard domains, a stable URL for the live version, a unique preview URL for every deploy, real-time traffic view, all with zero DNS busywork after the first wildcard record
- **Run like Fly.io** — single binary, single host, `porter up` and you're live
- **Speak Compose natively** — bring your existing `docker-compose.yml`, each service becomes one or more real, kernel-isolated microVMs instead of containers
- **Pure Go, no Docker required** — no Docker daemon anywhere in the loop, host or guest

**Not** a Docker-in-VM system, **not** a full multi-host Kubernetes replacement, and **not** multi-tenant (see [OSS & Future SaaS Strategy](#-oss--future-saas-strategy) for why that last one is permanent, not a v1 limitation).

### Why Porter isn't "just plain containers"

This project deliberately trades container-level density for **VM-grade isolation with near-container boot times**, because Firecracker's whole reason to exist is exactly that trade-off (it's what AWS Lambda/Fargate run on). If the goal were maximum density and you trusted all workloads equally, plain containerd with the default `runc` shim would be the simpler answer. Porter configures containerd with the `firecracker-containerd` shim specifically for cases where per-service kernel isolation actually matters — while still getting containerd's mature image pull, snapshot, and task-lifecycle machinery for free instead of rebuilding it.

### Why firecracker-containerd, not a hand-rolled pipeline

An earlier draft specified a fully custom pipeline: a hand-rolled OCI image puller/flattener, a from-scratch `guest-init` PID-1 binary (mount filesystems, bring up networking, reap zombies, run an embedded sshd), and manual jailer wiring. All three were flagged as the riskiest unbuilt parts of the whole plan — `guest-init` in particular is genuine, unforgiving systems programming with no partial-credit failure mode (get PID 1 wrong and nothing boots).

Standardizing on `firecracker-containerd` deletes all three at once:

- The custom OCI puller/flattener → containerd's existing pull + snapshotter
- The custom `guest-init` agent → `firecracker-containerd`'s own in-VM agent
- Manual jailer wiring → the shim's own jailer integration, which is its documented default mode

What's left for Porter to actually build is genuinely smaller: a control API, a compose mapper, a gateway, an SSH gateway that rides the existing vsock task-exec channel, and a thin client over containerd's task API. The remaining open question worth resourcing early is the **kernel image** — `firecracker-containerd` still needs a `vmlinux` built/configured for Firecracker guest boot, shared across all VMs; that's still a real one-time setup step and worth a smoke-tested "does a task actually boot and reach `running`" checklist before layering on domains/SSH/compose.

---

## ⚡ Quickstart

```bash
# 1. Install the Porter agent (one binary, needs KVM + root)
curl -fsSL https://get.porter.dev | sh

# 2. Point Porter at a kernel image (one-time)
porter kernel set ./vmlinux-5.10

# 3. Point your domain's wildcard DNS at this host (one-time)
#    *.example.com  A  <this-host-ip>
porter domain set-base example.com

# 4. Deploy a single image
porter up --image redis:7 --name cache
# → live at cache.example.com

# 5. Deploy a docker-compose.yml (with replicas, healthchecks, restart policy)
porter up -f docker-compose.yml --name my-app
# → each service live at <service>.my-app.example.com, load-balanced
#   across its healthy replicas
# → services reach each other via db.my-app.local, api.my-app.local, etc.

# 6. Scale a service up or down, any time
porter scale api 5

# 7. SSH into any service by name (or a specific replica: my-app-api-2)
porter ssh my-app-api

# 8. Attach your own domain to a service, any time
porter domains add shop.mybrand.com --service my-app-api

# 9. Open the dashboard
porter dashboard   # http://localhost:3000
```

**Honesty note:** those `porter` CLI commands describe the **target/planned** experience — the CLI itself lands in **v0.2.0-beta** per [`versions.md`](./versions.md). The REST API and dashboard are built today; until the CLI ships you drive the API directly (see [API Reference](#-api-reference)).

---

## 🏗️ Architecture

### Execution model

Porter runs **one Firecracker microVM per replica** of each compose service (or standalone deploy, which is a pool of size 1). It does **not** drive a `firecracker` process or talk to the Firecracker HTTP API itself. Instead it controls **containerd**, which has the [`firecracker-containerd`](https://github.com/firecracker-microvm/firecracker-containerd) shim registered as runtime `aws.firecracker`:

```
Porter (control plane UI/API)
   │  containerd Go client (task API, over containerd socket)
   ▼
containerd  ─── runtime: "aws.firecracker" ───▶  firecracker-containerd shim
   │  (pull OCI image, snapshotter/devmapper)        │  invokes jailer (chroot+cgroups+seccomp)
   │                                               │  boots Firecracker microVM
   │                                               │  runs its own in-VM agent as guest PID 1
   └── OCI registry (any: Docker Hub, GHCR, …)      └─ in-VM agent (firecracker-containerd's, not Porter's)
```

So **image pull, rootfs layout, the in-VM agent, and jailer wiring are all `firecracker-containerd`'s job** — Porter is a thin, pure-Go control plane in front of it. There is no Docker daemon, no `firecracker-go-sdk`, and no raw `firecracker` process anywhere in Porter's own logic.

> **Current-code caveat:** the checked-in backend was built against an earlier "direct Firecracker" API. The statement above, and everything in *Architecture*, is the **target** (the firecracker-containerd rewrite, per the migration plan). See [Current Code State](#-current-code-state-migration-status) before changing code.

### 1.1 Design principles

1. **One microVM per replica.** A compose service, or a single `--image` deploy, maps to one or more identical Firecracker microVMs — one per replica (`deploy.replicas`, default 1). This keeps the guest attack surface small, boot times fast (~125ms typical Firecracker boot), and the mental model simple: *VM = one replica of one service*.
2. **Don't reinvent what `firecracker-containerd` already solved.** Porter runs the same runtime AWS uses (Fargate/Lambda lineage) as its execution engine. Image pull, snapshotting, the in-VM agent, and jailer wiring are `firecracker-containerd`'s job, not Porter's. Porter is a control-plane UI/API in front of it.
3. **One front door.** Users never need to track per-VM IPs. HTTP goes through the gateway; SSH goes through the SSH gateway. Both route by name.
4. **Domains work like Vercel.** Point one wildcard DNS record at Porter once. Every deploy is then reachable immediately — a stable subdomain for the live version, a unique one per deploy for previews. Fully-owned custom domains attach on top via CNAME.
5. **Boring persistence.** State is a simple store (JSON or SQLite), swappable for Postgres later without touching the API surface.
6. **UX target: Vercel, not vSphere.** Two top-level concepts only — **Projects** (a compose file or a single image) and **Deployments** (the resulting VM(s)). No cluster/node/pool concepts in v1.
7. **Single-tenant, permanently.** Porter is one operator's tool for their own host(s) — not a foundation for serving multiple untrusted customers from one instance. Fixed design stance, not a v1 shortcut. See [OSS & Future SaaS Strategy](#-oss--future-saas-strategy).

### 1.2 Components

| Component | Where | Responsibility |
|---|---|---|
| **Control API** | `backend/internal/api` | Stateless HTTP server (Go). Project/VM CRUD, compose upload/parse, domain endpoints, SSE event stream for live state, auth (single bearer token). |
| **VM Manager** | `backend/internal/vmmanager` | Thin wrapper over a containerd client talking to the `firecracker-containerd` shim. Boot: `client.Pull()` → `container.NewTask()` → `task.Start()`; graceful/hard stop; teardown. Subscribes to task-exit events to flip state. *Not* responsible for image pull, in-VM init, or jailer wiring. |
| **Compose Mapper** | `backend/internal/compose` | Parses a constrained subset of Compose v3; each service → one containerd task → one VM. Topological sort on `depends_on`; rejects cycles. |
| **Network Manager** | `backend/internal/netmgr` | Provides per-project CNI network config (the shim's `tc-redirect-tap` plugin wires the tap). Allocates a `/24` bridge subnet per project (base `10.42.0.0/16`). Deterministic MAC per VM ID. |
| **Gateway** | `backend/internal/gateway` | Single HTTP front door. Routing by `Host` header; Vercel-style domains; in-memory traffic log; injects `X-Porter-VM-ID`/`X-Porter-Project-ID` on proxied requests. |
| **L7 Service Pool** | `backend/internal/gateway` | Round-robin a hostname across a pool of VM IDs per service; memberships update atomically as replicas boot/crash/are drained. |
| **Health Checker** | `backend/internal/health` | Per-VM goroutine probing `healthcheck:` (HTTP or TCP), draining unhealthy VMs from the pool and (with `restart:` policy) replacing them. Tracks `healthy`/`unhealthy`/`checking`. |
| **Embedded DNS** | `backend/internal/dns` | Tiny authoritative DNS per project bridge, `.local` zone: `<svc>.<project>.local` (healthy pool, round-robin) and `<svc>-<n>.<project>.local` (pinned replica). Online names stay correct as pools change. |
| **SSH Gateway** | `backend/internal/sshgw` | One SSH server; terminates the operator session and translates it into a containerd `task.Exec()` over the vsock channel. No in-guest sshd. |
| **State Store** | `backend/internal/store` | In-memory maps guarded by `sync.RWMutex`, persisted to a single JSON file on every write. Holds `VM` (incl. containerd `ContainerID`/`TaskID`, health, replica index), `Project`, `Domain`. |
| **Dashboard** | `frontend/` | Next.js + React, Vercel-inspired IA. Talks only to the Control API (REST + SSE) — never touches the host, containerd, or FC directly. |

### Data flow: booting a single image

```
User (CLI/UI)
  → POST /vms {image: "redis:7"}, name lands in "cache"
  → API creates VM record (state=pending), returns 202 immediately
  → runs async: VM Manager.CreateAndStart()
       1. containerd client.Pull("redis:7")   // not already cached
       2. Network Manager provisions the project's CNI config + IP
       3. container.NewTask(...) with FirecrackerMachineConfiguration
          → task.Start()                      // shim boots the microVM
       4. store records task/container IDs + IP, state → running
       5. Gateway registers cache.<base-domain> (stable subdomain)
  → Dashboard/CLI observes via SSE, shows "running" + URL + SSH command
```

### Data flow for a compose project

Parallel to the above, but per service: parse → topo-sort → boot services in order → per replica, create N VM records tagged ReplicaIndex (`0..N-1`), boot each, start its health goroutine, add healthy replicas to the service pool, register embedded-DNS names and SSH gateway entries. All services share one project bridge subnet.

### Failure handling

| Failure | Behavior |
|---|---|
| Image pull fails | VM → `failed`, containerd's error surfaced verbatim; no partial task |
| `task.Start()` fails (jailer/shim error) | VM → `failed` before a VMM exists, error from the shim's create-response |
| Task dies unexpectedly | Containerd `TaskExit` event → state → `stopped`, "crashed" badge if not user-initiated |
| Compose service fails mid-boot | Boot halts; already-booted services keep running; project shows `degraded` |
| Network/CNI setup fails | VM → `failed` before `task.Start()`, no leftover task |
| Host reboot | v1 does **not** auto-resume VMs (deferred). All show `stopped` until re-deployed. |
| Replica fails health checks | Drained from pool + DNS immediately; replaced if `restart` policy allows, else left `unhealthy` for manual fix. |

### Security notes (v1)

- Single static API token — treat the Control API as trusted-network-only; do not expose port 8080 publicly without a real auth layer in front.
- Single-tenant by design — everyone who can reach the token is fully trusted with everything on the host.
- The SSH gateway is the only intended to be internet-facing; since v1's SSH is `task.Exec()` over vsock, there is no guest-side network sshd to expose.
- The Firecracker jailer (chroot + cgroups + seccomp) is configured and invoked by `firecracker-containerd`'s shim — its documented recommended mode, Porter just turns it on.

---

## 🔄 Current Code State (Migration Status)

> **Read before changing code.** The checked-in code and the architecture above describe two different states of the same project. Keep them straight.

### The direction (target)

Per [`versions.md`](./versions.md) and the migration plan, Porter's intended runtime is **`firecracker-containerd`**: Porter talks only to containerd's task API; the `aws.firecracker` shim boots Firecracker. Porter never spawns or calls a `firecracker` binary directly.

### What's implemented today

The current backend uses an **earlier direct-Firecracker** design and is **mid-migration**. Concretely today:

- **`backend/`** is a single Go package `main`, stdlib + `modernc.org/sqlite`, one responsibility per file:
  - `api.go` — net/http handler, routes on the Go 1.22+ `ServeMux`, bearer-token auth via `Authorization: Bearer`. Endpoints: `GET /health`, `POST /login`, `GET|POST /vms`, `POST /vms/{id}/stop|start`, `DELETE /vms/{id}`, project routes, `GET /events` (SSE).
  - `vmmanager.go` — spawns a `firecracker --api-sock <path>` process per VM, drives lifecycle over the API socket, `probeHealth` TCP-check.
  - `fcapi.go` — minimal `net/http` client for the Firecracker API over the per-VM Unix socket (it **has** both a socket dialer + tiny REST client). Uses boot source, drives, network interface, machine config, `InstanceStart`, `SendCtrlAltDel`.
  - `store.go` — SQLite (`porter.db`), `id → JSON blob` per table, no heavy ORM; keeps VMs, projects, domains; plus an in-memory traffic ring buffer.
  - `compose.go` — hand-rolled YAML-subset block with topo-sort (tests in `compose_test.go`).
  - `netmgr.go` — per-project `/24` subnets + `tap` device creation via `ip tuntap`.
  - `sse.go` — SSE `Hub` to push VM-state events. 
  - `config.go`, `toml.go`, `main.go` — load `porter.toml` with env-var override; `main.go` embeds the built frontend via `go:embed web/dist`.
- **Direct-Firecracker constraints (known, documented):** booting relies on a `vmlinux` kernel + `rootfs.ext4` per service (no OCI pull in the running code); networking is half-wired (`tap` only, no NAT/bridge by default); domains/DNS/SSH-gateway not implemented; healthcheck is TCP-connect only; SQLite state doesn't reconcile with host reality after a restart.

### What the migration plan changes

The `v0.1.0-beta` migration plan moves from direct-Firecracker to `firecracker-containerd`:

| # | Change |
|---|---|
| 1 | **`fcapi.go` deleted** — Porter never calls the FC HTTP API directly. |
| 2 | **`go.mod`** — add `github.com/containerd/containerd v1.7.x` (client) + OCI spec deps; `go mod tidy` on Linux first. |
| 3 | **`vmmanager.go` rewritten** — talk to containerd: set namespace → `client.Pull(WithPullUnpack, WithSnapshotter("devmapper"))` → `NewContainer(... WithRuntime("aws.firecracker"))` → `NewTask(cio.LogFile(...))` → `task.Start()`; record `ContainerID`/`TaskID`, start health goroutine, `task.Wait()`. Stop: SIGTERM→5s→SIGKILL, `task.Delete`, `container.Delete(WithSnapshotCleanup)`. Logs → `/var/log/porter/<id>.log` + stream via API/SSE. |
| 4 | **`config.go`/`types.go`** — `FCConfig` now `{ContainerdSocket, Snapshotter, Namespace}`; `VM` gains `ContainerID`/`TaskID`; add `ImageManifest`, log ring constants. |
| 5 | **`store.go`** — add `AppendLog`/`TailLogs` (fit 500 buffer). |
| 6 | **`api.go`** — add `GET /images`, `GET /vms/{id}/logs?tail=200`, `POST /vms/{id}/restart`, `GET /overview`. |
| 7 | **`netmgr.go`** — CNI actually provides `tap`s; Porter writes per-project CNI config, tracks IPs/MACs, drops manual `ip tuntap`. |
| 8 | **Frontend** — swap `events.js` for a Vue composable, add components for traffic/domains/logs/SSH/image-library. |

The target deployment topology and the exact `porter.toml` the migration converges on are described in [Real-world Deployment (target)](#-real-world-deployment-target) and [Configuration Reference](#-configuration-reference) below.

---

## 📡 API Reference

- Base URL: `http://<host>:8080`
- Auth: `Authorization: Bearer <PORTER_API_TOKEN>` on every request except `/health`.
- All input/output is JSON. Errors uniform: `{ "error": "..." }`.

### `GET /health`
No auth. `→ 200 {"status": "ok"}`

### `GET /vms`
List all VMs (standalone + part of projects).

**200**
```json
[
  { "id": "5b2e…", "name": "cache", "project_id": "", "service_name": "", "state": "running",
    "health_status": "healthy", "replica_index": 0, "image": "redis:7", "vcpus": 1, "mem_mib": 256,
    "ip_address": "10.42.0.5", "ports": [], "created_at": "…", "started_at": "…" }
]
```

### `POST /vms`
Create + boot a single VM from an image.

```json
{ "name": "cache", "image": "redis:7", "vcpus": 1, "mem_mib": 256,
  "env": {}, "ports": [{ "container_port": 6379, "protocol": "tcp" }] }
```
`→ 202 Accepted` with VM record (state `pending`); boot continues asynchronously.

### `GET /vms/{id}` — 200 VM record, or 404.
### `POST /vms/{id}/stop` — graceful stop (ACPI, hard-total after 5s). `200 {"status":"stopped"}`.
### `POST /vms/{id}/start` — re-boots, `202` (state `booting`).
### `DELETE /vms/{id}` — stop + remove record + its domains/SSH entries. `200 {"status":"deleted"}`.

### `GET /projects`

### `POST /projects/compose
```json
{ "name": "my-app", "compose_yaml": "version: '3'\nservices:\n api:\n  image: …\n  ports:\n   ..." }
```
`202` returns project + `service_pools` (`{ "<svc>": { desired, healthy, vms } }`). `400` returns a verbatim parse error.

`GET /projects/{id}`, `GET /projects/{id}?expand=vms`, `PATCH /projects/{id}/services/{svc}/scale`, `DELETE /projects/{id}/services/{svc}`, `DELETE /projects/{id}`.

### Domains
`GET /vms/{id}/domains` — stable/preview/custom. `POST /vms/{id}/domains { "domain": … }` → `202` + `required_record` (CNAME). `DELETE /vms/{id}/domains/{domain}`.

### Traffic
`GET /vms/{id}/traffic?limit=200` — most recent ring entries (metadata only).

### Volumes & port mapping (v0.1.0 fold-in)

Fold-ins from the Phase 6 (Networking) / Phase 7 (Storage) workstreams:

- **Port mapping** — a VM's `ports` may now carry an optional `host_port`
  (defaults to `container_port`), mapping **host → guest** so the service is
  reachable on the host: `{"container_port": 6379, "host_port": 16379, "protocol": "tcp"}`.
- **Volumes** — a scaffolded `/volumes` API for persistent storage:
  `POST /volumes { "name": "db", "size_mib": 2048 }` creates one, `GET /volumes`
  lists, `DELETE /volumes/{id}` removes; attach a volume by name on `POST /vms`
  and it is mounted before the microVM boots.

### Events (SSE)
`GET /events` → server stream: `vm.state`, `project.progress`, `traffic.request`, `domain.status`, `replica.health`, `pool.updated`.

### SSH
`GET /vms/{id}/ssh-info` — gateway host/port + command. `POST /vms/{id}/ssh-cert { public_key }` → short-lived cert signed by the CA.

### Status codes
`200`, `202`, `400`, `401`, `404`, `409`, `500`.

**SSE event payloads** (`GET /events`, server-sent, `data: <json>\n\n`):

| Event | Payload |
|---|---|
| `vm.state` | `{ "vm_id": "…", "state": "running", "health_status": "healthy" }` |
| `project.progress` | `{ "project_id": "…", "step": "boot", "completed": 2, "total": 3 }` |
| `traffic.request` | `{ "vm_id": "…", "host": "…", "path": "…", "status": 200, "duration_ms": 12, "ts": "…" }` |
| `domain.status` | `{ "vm_id": "…", "domain": "…", "status": "active" }` |
| `replica.health` | `{ "vm_id": "…", "service": "…", "health_status": "unhealthy" }` |
| `pool.updated` | `{ "service": "…", "desired": 3, "healthy": 2, "vms": ["…"] }` |

`200`, `202`, `400`, `401`, `404`, `409`, `500`.

---

## 📦 Compose Mapping Rules

`backend/compose.go` (`ParseCompose`) hand-parses a **deliberately constrained Compose v3 subset** — one service → one containerd task → one VM (more replicas → more identical VMs). No external YAML dependency. The mapping below is what the parser actually implements.

### Supported keys

| Key | Meaning | Notes |
|---|---|---|
| `image:` | OCI image ref each replica boots from | **required** per service |
| `restart:` | Restart policy (`on-failure`, etc.) | passed through to the VM lifecycle |
| `ports:` | `"<host>:<container>/<proto>"` or `"<host>:<container>"` | last segment = container port; `/tcp` assumed unless `/udp` given |
| `environment:` | `KEY=value` or `KEY:"value"` | flat key/value map |
| `depends_on:` | list of services to boot first | topo-sorted (see below) |
| `deploy:` | `replicas: N` | default `1`; validated as integer |
| `healthcheck:` | `test:` and `interval:` | `test:` ⇒ HTTP check, else TCP; `interval` in seconds (`30s`) |

### Explicitly rejected

- **`build:`** — image-based services only. `ParseCompose` returns `compose parse error: … only image-based services are supported (no "build:")`.
- **Circular `depends_on`** — refused with a `circular dependency` error.
- **`depends_on` of an unknown service** — refused.
- Any service missing `image:` — refused.

### Boot ordering

`depends_on` is resolved by a DFS topological sort (declaration order used as the tiebreak; each service's deps sorted for determinism). Services boot in topo order; a circular or unknown dependency halts parsing. For `ports`, only the container port and protocol are recorded (`parsePort` takes the last `:` segment; the `host`/left-hand part is ignored).

### Current parse constraints worth knowing

- Tabs are normalized to 4 spaces; comment stripping is `(^|\s)#…`.
- One section list/map (`ports`/`environment`/`depends_on`/`deploy`/`healthcheck`) per indent depth; items must sit one level deeper than their section header.
- A new top-level key (`networks:`, `volumes:`…) ends the `services:` block.
- Empty `services:` yields `no services found under services:`.
- `replicas: 0` or negative is clamped to `1`.

Tests in `backend/compose_test.go`.

> **Embedded-DNS names / preview / stable domains**: these are *target* behaviors of the firecracker-containerd rewrite (see [Runtime Architecture](#-runtime-architecture) and [SSH Access](#-ssh-access)); the current `compose.go` produces only parsed services, ports, env, replicas, deps, healthcheck, and restart.

---

## 🔑 SSH Access

### Model: single gateway, cert-based, backed by `task.Exec()`

```
operator → ssh my-app-api@gateway.example.com -p 2222
            │  (auth: short-lived cert OR static key)
            ▼
          SSH Gateway (sshgw)   ← the only SSH-facing service
            │  look up VM's containerd task ID in the store
            │  call containerd task.Exec(shell), pipe stdio
            ▼
      firecracker-containerd in-VM agent (ships with the shim)
```

No SSH server runs inside the guest. The gateway terminates the SSH session and turns it into a `task.Exec()` against that VM's task — same mechanism `ctr task exec` uses. Every VM is thus SSH-reachable by default (even a bare `redis:7`), with zero baking into the image.

**Replicas:** `porter ssh api` → replica `0`; `porter ssh api-2` → exact replica. Targets match the embedded-DNS `<svc>-<n>` names.

**Auth — Option A (recommended, default for `porter ssh`):** CLI generates an ephemeral ed25519 one; `POST /vms/{id}/ssh-cert`; gateway CA signs a 10-min cert; CLI connects with it; cert verifies; `task.Exec()` to a shell. **Option B (static key, opt-in):** add a key once via `porter auth add-key`, then plain `ssh my-api@gateway…` works; keys listable/revocable.

**Guest-side setup: none.** `task.Exec()` is provided out of the box by `firecracker-containerd`'s agent; no binary/CA/host-key is baked into any image.

**Limitations (v1):** no SSH to a `stopped` VM; no `-L`/`-R` port forwarding; SFTP/SCP best-effort only; gateway is single-process (flagged for HA). `deployment` meta logged per connection, full session recording deferred.

---

## 🧱 Dashboard (frontend/)

The dashboard is a Vercel-inspired **Vue 3 + Vite** app in `frontend/`. It talks **only** to the Control API (HTTP + SSE) — never the host/containerd/FC. Two views in v1 — Projects had an earlier React/Next spec, but the current implementation is Vue 3; follow the frontend in `frontend/src`. `vite.config.js` proxies `/login`, `/health`, `/vms`, `/projects`, `/events` → `localhost:8080` in dev. Auth token stored client-side and sent as `Authorization: Bearer`.

Key UI building blocks (current + roadmap from the dashboard spec):
- Status badges (pending/booting/running/stopping/stopped/failed) with consistent colors
- Project cards + detail (scale, redeploy, per-replica view, compose source viewer, network panel)
- Single-image deploy form; compose editor tab
- Domains panel (stable/preview/custom + add-domain + CNAME record + verify status)
- Traffic tab (live table + requests/sec sparkline, client-side filters)
- Logs drawer (`GET /vms/{id}/logs?follow=true`)
- SSH modal (copy the `porter ssh` / raw `ssh` command)

---

## 🛠️ Build, Test, Run (development)

The root `Makefile` drives everything. The Go binary **embeds** the built frontend (`go:embed web/dist`), so the frontend builds first.

```bash
make frontend   # npm install + vite build → backend/web/dist
make backend    # go build → backend/porter
make build      # frontend then backend
make run        # build then run in the foreground
make dev        # backend only: go run . (run `npm run dev` in a 2nd shell for hot reload)
make clean      # remove artifacts, db, node_modules
```

Backend tests (only tests in the repo): `cd backend && go test ./...` (single: `go test -run TestParse -v .`).

The backend currently has **one Go dependency** (`modernc.org/sqlite`, pure-Go, no cgo). The migration adds `containerd` + OCI deps (run `go mod tidy` on Linux after).

Run requirements: Linux + KVM, a `firecracker` binary (or containerd + shim after migration), a shared `vmlinux`, and a `rootfs.ext4` (current build) — see `_10_Config`.

**Local dev:** terminal 1 `go run .`, terminal 2 `npm run dev` (Vite proxies to `localhost:8080`).

---

## ⚡ Real-world Deployment (target)

Install & configure **containerd + firecracker-containerd shim** first (devmapper thin-pool + runtime registration; `ctr run --runtime aws.firecracker …` should boot a test VM). One binary bundles the Control API, gateway, and SSH gateway. Then:

- `porter kernel --path /path/to/vmlinux` (one-time)
- `porter domain set-base example.com` (wildcard, one-time)
- export `PORTER_API_TOKEN` + `PORTER_DATA_DIR=/var/lib/porter`
- run as a systemd service (unit under `deploy/systemd/`)
- for real network reachability, wire bridging/NAT (e.g. `br0` + `iptables` MASQUERADE) yourself.

> Full config reference — the environment variables and the current + migration-target `porter.toml` — is in [Configuration Reference](#-configuration-reference) below. (The `api_addr`/`gateway_addr`/`ssh_gateway_addr`/`bridge_base`/`ssh_cert_ttl`/`domain_verify_interval`/`traffic_log_size`/healthcheck tunables are part of the migration's `FCConfig` rewrite, not the current `config.go`.)

---

## ⚙️ Configuration Reference

Config comes from `porter.toml` (`[server]`, `[firecracker]`, `[admin]`), layered over by `PORTER_*` environment variables (env wins when set). `main.go` refuses to start unless `api_token` and an admin `password` are set. The parser is `backend/toml.go`; loading logic is `backend/config.go`.

### Environment variables (current `config.go`)

| Var | Overrides | Default |
|---|---|---|
| `PORTER_LISTEN_ADDR` | `[server] listen_addr` | `:8080` |
| `PORTER_BASE_DOMAIN` | `[server] base_domain` | — |
| `PORTER_STATE_FILE` | `[server] state_file` | `porter.db` |
| `PORTER_API_TOKEN` | `[server] api_token` | *(required — refuse to start if unset)* |
| `PORTER_KERNEL_IMAGE` | `[firecracker] kernel_image` | — |
| `PORTER_ROOTFS_PATH` | `[firecracker] rootfs_path` | — |
| `PORTER_FIRECRACKER_BIN` | `[firecracker] firecracker_bin` | `firecracker` |
| `PORTER_ADMIN_USERNAME` | `[admin] username` | `admin` |
| `PORTER_ADMIN_PASSWORD` | `[admin] password` | *(required)* |

### `porter.toml` — current (direct-Firecracker)

```toml
[server]
listen_addr  = ":8080"
base_domain  = ""
state_file   = "porter.db"
api_token    = "change-me"

[firecracker]
kernel_image     = "/path/to/vmlinux"
rootfs_path      = "/path/to/rootfs.ext4"
firecracker_bin  = "firecracker"

[admin]
username = "admin"
password = "change-me"
```

### `porter.toml` — migration target (firecracker-containerd)

The migration replaces the raw Firecracker fields with containerd wiring; `images_dir` points at the image-catalog manifests under `vms/images/`:

```toml
[server]
listen_addr  = ":8080"
base_domain  = ""
state_file   = "porter.db"
api_token    = "dev-token-change-me"

[firecracker]
containerd_socket = "/run/containerd/containerd.sock"
snapshotter       = "devmapper"
namespace         = "porter"
images_dir        = "vms/images"

[admin]
username = "admin"
password = "change-me"
```

> The migration's `FCConfig` struct becomes `{ ContainerdSocket, Snapshotter, Namespace }` — `kernel_image`/`rootfs_path`/`firecracker_bin` are owned by the shim config (`/etc/containerd/firecracker-runtime.json`), not Porter.

---

## 🗺️ Roadmap & Versions

The full, authoritative roadmap — `v0.1.0-beta` through `v0.x`, Phase II `v1.0.0`, Phase III cloud platform, and the long-term v2.x vision — lives in **[`versions.md`](./versions.md)**. `v0.1..beta` (current target) covers: Firecracker + firecracker-containerd integration, VM lifecycle, boot kernels, rootFS/OCI image support, compose + single-image deploys, login/dashboard, VM mgmt + logs, private bridge/static IP/port mapping, REST API. See the file for the phased breakdown.

---

## 🏛️ OSS & Future SaaS Strategy

Porter is **self-hosted, MIT-licensed**, single-tenant **by design, permanently** (not a v1 shortcut). The self-hosted core is the product being built first; a hosted /Sside is a possibility kept open (so the door isn't accidentally welded shut), not a plan being executed.

**Implications:** no tenant-ID–style, no per-resource ownership/ACL/billing hooks, no `MULTI_TENANT` mode flags in the OSS core. If a hosted product happens this is a **separate closed-source layer** orchestrating many isolated single-tenant Porter instances — the core never grows multi-tenant auth/billing/quota code. State store and API auth are already postgres/AuthZ-swappable behind small interfaces. IDs are already UUIDs.

**Why MIT (over AGPL):** maximum adoption, no copyleft obligations for self-hosters. Accepts that anyone can fork/run a competing hosted version; the bet is adoption+trust outweigh that, same as Supabase/PostHog/Traefik.

**Details of the stance:** the JSON-file / SQLite state store and the single static bearer token stay swappable behind small interfaces; secrets live in the host environment (systemd/CI), never hard-coded. The hosted-vs-core split keeps no multi-tenant flags in the OSS core — any hosted product is a separate closed-source controller that merely orchestrates many isolated single-tenant Porter instances.

---

## 📄 License

MIT. 