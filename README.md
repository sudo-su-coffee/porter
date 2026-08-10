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

> **The self-hosted PaaS.** Deploy Docker images as kernel-isolated microVMs, all from a dashboard — no CLI, no YAML file required. Your own **Vercel** or **Fly.io**, on one box, without a Docker daemon or Kubernetes.

Better isolation than plain Docker (each deploy is its own microVM), far simpler than Kubernetes (one binary, one host, zero cluster). The engine is [`firecracker-containerd`](https://github.com/firecracker-microvm/firecracker-containerd) — the same runtime AWS runs Fargate and Lambda on — so every deploy gets a hardware-isolated microVM.

Think Vercel or Fly.io, self-hosted. Porter is the control-plane UI/API on top of that VM engine — not a from-scratch VMM orchestrator, and not just a "microVM spinner." The isolation is the machinery; the platform (deploys, logs, traffic, scaling) is the point.

MIT licensed. Self-hosted. Single-tenant by permanent design.

> **Honesty note on this README:** it's written to match what is actually implemented in the checked-in code as of this revision, verified by reading the source — not the aspirational end state. Anything not yet built lives in the [Roadmap](#-roadmap) or the clearly-marked [Planned / In Progress](#-planned--in-progress) section instead of being described here as working. The full phased roadmap, feature-by-feature status tags, and security/trust-model deep dive live in [`PLAN.md`](./PLAN.md) — if the two files ever disagree, treat this README as authoritative, since it's the one re-verified against source on every revision.

---

## Contents

1. [What Porter Does](#-what-porter-does)
2. [Quickstart](#-quickstart)
3. [Runtime Architecture](#-runtime-architecture)
4. [Security & Trust Model](#-security--trust-model)
5. [Current Code State (Migration Status)](#-current-code-state-migration-status)
6. [API Reference](#-api-reference)
7. [Compose Mapping Rules](#-compose-mapping-rules)
8. [Domains & Traffic Log](#-domains--traffic-log)
9. [SSH Access](#-ssh-access)
10. [Dashboard UI Spec](#-dashboard-ui-spec)
11. [Installation, Deployment & Local Development](#-installation-deployment--local-development)
12. [Configuration Reference](#-configuration-reference)
13. [Roadmap](#-roadmap)
14. [OSS & Future SaaS Strategy](#-oss--future-saas-strategy)
15. [License](#-license)

---

## 🚀 What Porter Does

Porter brings the pieces of Fargate, Kubernetes, and Fly.io that actually matter for a single self-hosted box, into one pure-Go binary:

- **Scale like Fargate** — `deploy.replicas: 3` in your compose file, or `PATCH /projects/{id}/scale` any time, and Porter boots identical, isolated microVMs to match
- **Heal like Kubernetes** — declare a `healthcheck:`, and Porter probes every replica over HTTP or TCP, marks it `healthy`/`unhealthy`, and triggers a replace hook on failure
- **Discover like Kubernetes DNS** — the gateway resolves `<svc>.<project>.local` to a healthy replica's IP for its own internal routing decisions
- **Multi-service apps, no YAML required** — define services (image, replicas, ports, env, `depends_on`) directly in the dashboard; each service becomes one or more real, kernel-isolated microVMs instead of containers. Importing an existing `docker-compose.yml` (image-based services only, see [Compose Mapping Rules](#-compose-mapping-rules)) is one way in, not the primary interface — see the note on direction below.
- **Pure Go, no Docker required** — no Docker daemon anywhere in Porter's own logic; VMs boot through containerd + the `aws.firecracker` shim, or by driving the Firecracker API socket directly for golden/rootfs images

**No CLI, by design.** Porter is UI-first and UI-only: deploying, scaling, starting/stopping the daemon, and every other operation happen through the dashboard or the REST API it calls — there is no `porter up`/`porter ssh`/`porter scale` command line planned. `cmd/porter` stays a minimal binary entrypoint (the HTTP server itself, plus `porter kernel set` and `porter version` as host-provisioning utilities only).

**Not yet shipped** (see [Planned / In Progress](#-planned--in-progress)): the UI-native multi-service form (compose import is currently the only way to define multi-service apps), wildcard/preview domains, real persistent volumes mounted into the guest, and multi-node clustering. Host-reachable port mapping, TLS/ACME, and Git-based deploy are shipped.

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

There is no CLI, and there won't be one — Porter is UI-first by design (see [What Porter Does](#-what-porter-does)). Everything below is the real REST API the dashboard itself calls, and the real binary, as implemented today.

```bash
# 1. Build and run the installer on a Linux host with KVM (/dev/kvm)
sudo bash backend/deploy/install.sh
#    Installs PostgreSQL, containerd + devmapper, the aws.firecracker shim,
#    the firecracker binary, CNI, builds the porter binary, installs a
#    systemd unit. See backend/deploy/README.md for what it does step by step.

# 2. Provision the shared guest kernel (one-time)
porter kernel set ./vmlinux
# or: porter kernel set https://.../vmlinux

# 3. Edit /etc/porter/porter.toml — set [server] api_token and [admin] password
#    to real secrets (there are no working defaults, on purpose — see
#    Security & Trust Model), then start the service
systemctl start porter
# Dashboard: http://localhost:8080 (login with the [admin] credentials)

# 4. Deploy a single image via the API
curl -X POST localhost:8080/projects \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"cache","image":"redis:7-alpine"}'

# 5. Multi-service apps: today this means importing a docker-compose.yml
#    (image-based services only). A UI-native form for defining services
#    directly (no YAML) is planned — see Planned / In Progress.
curl -X POST localhost:8080/projects/compose \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"myapp","compose_yaml":"services:\n  api:\n    image: nginx\n"}'
```

**Not yet available:** wildcard/preview domains, real host→guest port mapping (see [Planned / In Progress](#-planned--in-progress)). There is no CLI and none is planned — the dashboard and this API are the only interfaces.

**Before exposing anything beyond a trusted LAN**, read [Security & Trust Model](#-security--trust-model) — the Control API's auth model and the gateway's lack of TLS termination both have real implications for how you deploy this.

---

## 🏗️ Architecture

### Execution model

Porter runs **one Firecracker microVM per replica** of each compose service (or standalone deploy, which is a pool of size 1). It does **not** drive a `firecracker` process or talk to the Firecracker HTTP API itself for OCI-image boots. Instead it controls **containerd**, which has the [`firecracker-containerd`](https://github.com/firecracker-microvm/firecracker-containerd) shim registered as runtime `aws.firecracker`:

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

So **image pull, rootfs layout, the in-VM agent, and jailer wiring are all `firecracker-containerd`'s job** for OCI-image boots — Porter is a thin, pure-Go control plane in front of it. There is no Docker daemon anywhere in Porter's own logic. Golden/rootfs-only "bare" microVM images are the one exception: `internal/runtime` drives the Firecracker API socket directly for those, since there's no OCI image to hand to containerd — which also means the jailer/seccomp guarantees below apply differently to that path (see Security & Trust Model).

### 1.1 Design principles

1. **One microVM per replica.** A compose service, or a single-image deploy, maps to one or more identical Firecracker microVMs — one per replica. This keeps the guest attack surface small and the mental model simple: *VM = one replica of one service*.
2. **Don't reinvent what `firecracker-containerd` already solved.** For OCI-image boots, Porter runs the same runtime AWS uses (Fargate/Lambda lineage) as its execution engine. Image pull, snapshotting, and jailer wiring are `firecracker-containerd`'s job, not Porter's.
3. **One front door for HTTP.** `internal/gateway` is a Host-header reverse proxy so users don't have to track per-VM IPs for web traffic. It currently matches a request's `Host` against stored domain records, falls back to `.local` resolution, and falls back to "any healthy VM" for single-service dev setups.
4. **Boring persistence.** State lives in PostgreSQL via `pgx` (`internal/store`); two ring buffers (traffic, per-VM logs) are deliberately kept in-memory only, not persisted.
5. **Single-tenant, permanently.** Porter is one operator's tool for their own host(s) — not a foundation for serving multiple untrusted customers from one instance. Fixed design stance, not a v1 shortcut. See [Security & Trust Model](#-security--trust-model) and [OSS & Future SaaS Strategy](#-oss--future-saas-strategy).

### 1.2 Components

| Component | Where | Responsibility |
|---|---|---|
| **Control API** | `backend/internal/api` | `net/http` server on Go's `ServeMux`, bearer-token auth. Project/VM/replica CRUD, compose upload/parse, domains, volumes (see caveat below), SSE event stream. This package is intentionally the largest in the codebase and includes a broad Vercel-style surface (analytics, redirects, crons, firewall rules, etc.) — see [Planned / In Progress](#-planned--in-progress) for which parts of that surface are real vs. scaffolded. |
| **VM Runtime** | `backend/internal/runtime` | The live VM engine, wired in `cmd/porter/main.go`. Boots OCI images via containerd + the `aws.firecracker` runtime (pull → `NewContainer` → `NewTask` → `Start`); boots "bare" rootfs images by spawning `firecracker` directly and driving its API socket. Graceful stop is SIGTERM → 4s grace → SIGKILL, then task/container/snapshot cleanup. |
| **Compose Mapper** | `backend/internal/compose` | Hand-rolled parser for a constrained Compose v3 subset (no external YAML dependency); one service → one or more VMs. Topological sort on `depends_on`, rejects cycles and `build:`. |
| **Network Manager** | `backend/internal/netmgr` and `backend/internal/net` | Two independent allocators currently exist side by side — see [Planned / In Progress](#-planned--in-progress). `internal/net` is the one actually driving VM boot (raw `ip tuntap` tap devices, per-project `/24` subnets); `internal/netmgr` is a separate CNI-config-oriented allocator wired into the API layer for a cosmetic subnet field on `Project.Network`. |
| **Gateway** | `backend/internal/gateway` | HTTP front door: `Host`-header reverse proxy, round-robins across healthy replicas, logs every request to an in-memory ring and to the store. HTTP only today — no TLS termination yet. |
| **Health Checker** | `backend/internal/health` | Per-VM goroutine probing a declared `healthcheck:` over HTTP or TCP, flips `healthy`/`unhealthy`, and calls a replace hook (wired to VM restart) on failure. |
| **`.local` Resolver** | `backend/internal/dns` | An in-process Go function the gateway calls to resolve `<svc>.<project>.local` to a VM IP. This is **not** a DNS server — it never listens on the network or answers real DNS queries. A real wire-level authoritative DNS server is planned; see [Planned / In Progress](#-planned--in-progress). |
| **SSH Gateway** | `backend/internal/sshgw` | SSH server that bridges a session into a containerd task via `task.Exec()`. No sshd runs inside the guest. |
| **State Store** | `backend/internal/store` | PostgreSQL via `pgx`/`pgxpool`, migrations embedded and run at startup. Traffic and per-VM log rings are in-memory only (explicitly not persisted, by design — high write rate, low durability need). |
| **Dashboard** | `frontend/` | Vue 3 + Vite + Tailwind + Pinia + TanStack Query. Talks only to the Control API (REST + SSE). Current views: Login, ProjectDetail, DeploymentsList, VmDetail, Domains, Logs, Traffic, Images, Servers, Teams, Settings. |

> **Dead code note:** `backend/internal/vmmanager` is a second, complete VM-lifecycle implementation that is not imported anywhere in the codebase. `internal/runtime` is the one actually wired into `main.go`. `internal/vmmanager` is slated for removal — see [Planned / In Progress](#-planned--in-progress).

### Data flow: booting a single image

```
User (dashboard/API)
  → POST /projects {image: "redis:7"}, name "cache"
  → API creates Project + VM record(s) (state=pending), returns 202
  → runs async: vmEngine.Boot()
       1. Allocate/reuse the project's /24 subnet + this replica's IP, MAC,
          tap device (internal/net.NetManager)
       2. containerd client.Pull("redis:7") if not already cached
       3. NewContainer(... WithRuntime("aws.firecracker")) → NewTask → Start
       4. store records ContainerID/TaskID + IP, state → running
       5. If a healthcheck is declared, start the health-probe goroutine
  → Dashboard observes via SSE (GET /events), shows the new state
```

### Data flow for a compose project

Parallel to the above, but per service: parse → topo-sort on `depends_on` → boot services in order → per replica, create a VM record tagged with its replica index, boot each, start its health goroutine if a healthcheck is declared. All services in a project share one allocated subnet.

### Failure handling

| Failure | Behavior |
|---|---|
| Image pull fails | VM → `failed`, containerd's error surfaced verbatim |
| `task.Start()` / container-create fails | VM → `failed`, error message includes a hint (e.g. "is the aws.firecracker shim registered?") |
| Task exits on its own | Reconciled: container/snapshot cleaned up, VM → `failed` with the exit code or wait error |
| Host process restarts while VMs were running | On the next `porter server` start, any VM left in `booting`/`running` state is marked `failed` with "host restarted while this VM was up — press Start to retry" (no automatic resume) |
| Replica fails health checks 3 times in a row | Marked `unhealthy`; the configured replace hook (VM restart) is invoked |
| containerd socket missing at startup | Server still starts and serves the dashboard; a warning is logged, and any real VM boot will fail with a clear error until containerd is running |

---

## 🔒 Security & Trust Model

Porter's security posture is intentionally simple — read this before deploying anywhere beyond a private lab box. (The phased hardening plan for items marked "planned" below lives in [`PLAN.md`](./PLAN.md#security--trust-model).)

**Trust boundary**

- **Single static bearer token, single admin account.** Everyone who can present `Authorization: Bearer <token>` (or the admin login) is fully trusted with the entire host — every project, every VM, SSH access, and the ability to boot arbitrary images. There is no per-user RBAC yet.
- **Do not expose the Control API to the public internet directly.** Treat it as trusted-network-only (LAN, VPN, or WireGuard tunnel) until a real auth layer — beyond the single token — sits in front of it.
- **`porter.toml` refuses to start without explicit secrets.** `api_token`, `admin.password`, and a database URL must all be set; the token and password have **no working default**, so a forgotten config can't silently ship with a known credential. The database URL does have a default (`postgres://porter:porter@localhost...`) intended for local dev only — change it for anything else.

**Workload isolation vs. control-plane isolation**

- Firecracker + jailer gives each **replica** real, hardware-backed kernel isolation from other replicas. That is a genuine, strong guarantee for the workloads themselves.
- It does **not** give the **operator** isolation from their own workloads, and it does **not** make the Porter control plane safe to run as a multi-tenant SaaS backend for untrusted customers. Those are separate claims — see [OSS & Future SaaS Strategy](#-oss--future-saas-strategy) for why the latter is a permanent non-goal, not a v1 gap.
- **Jailer configuration is the shim's job, not Porter's.** The `aws.firecracker` containerd shim invokes the jailer according to `/etc/containerd/firecracker-runtime.json`; Porter neither sets nor verifies those flags (seccomp profile, chroot, cgroup limits) at runtime. If you edit that file, re-verify a VM actually boots under the jailer as expected — Porter won't detect a silent misconfiguration there.
- **The "bare" rootfs boot path bypasses containerd/jailer entirely** — `internal/runtime` drives the Firecracker API socket directly for golden images. Confirm your own jailer/seccomp wrapping (or lack of it) is what you intend before using golden images for anything untrusted.

**Network exposure, today**

- The gateway (`internal/gateway`) terminates **HTTP only** — no TLS. Put a TLS-terminating reverse proxy or a WireGuard tunnel in front of it for anything beyond a trusted LAN, until Automatic TLS (ACME/DNS-01) ships.
- `handleVerifyDomain` performs a real DNS probe — it resolves the domain's A/AAAA records and reports `"verified"` only when the name resolves (and, for subdomains, points at the platform's base domain). It is an ownership probe, not a DNS-01 challenge; do not treat a "verified" status as cryptographic proof of zone control.
- The SSH gateway (`internal/sshgw`, off by default) is a `task.Exec` bridge into the containerd task, authenticated by a self-generated CA/cert issued per-request — not a general-purpose bastion. It has no port-forwarding or SFTP, and only reaches OCI-image (containerd-backed) VMs; treat it as an admin/debug channel.

**Data at rest**

- PostgreSQL via `pgx` is the durable store for everything except two explicitly ephemeral in-memory rings (traffic log, per-VM log tail), which are lost on restart by design — back up Postgres, not the rings.
- Compose `environment:` values are stored as-is; if you're putting real secrets in them today, treat the database itself as the trust boundary — dedicated secrets encryption-at-rest is not yet a separate, verified guarantee (see `[server]`/`[database]` config below for what's currently protected: the token/password have no default, the values inside compose files are not separately encrypted).

**What's explicitly out of scope, by design, not by omission**

- Multi-tenant SaaS use of a single Porter instance — permanent non-goal, see [OSS & Future SaaS Strategy](#-oss--future-saas-strategy).
- A CLI attack surface — there isn't one, and none is planned; the API + dashboard are the only interfaces, which keeps the credential surface to just the bearer token and admin session.

---

## ✅ Planned / In Progress

Everything in this section is scoped and being actively worked on, but **is not yet in the code** — treat any claim elsewhere in this README about these topics as the target, not the current state. Status tags matching [`PLAN.md`](./PLAN.md) are included for quick cross-reference. Several items once listed here have shipped and moved to the API/source tables above.

**Shipped since this list was written:** real host-reachable port mapping (gateway `PortForwarder` binds compose `HostPort` → VM container port), a wire-level authoritative DNS server (`internal/dns/server.go`, `miekg/dns`), automatic TLS via ACME (`internal/tls/autocert.go`), Git-repo deploy with real OCI build bridge (`docker/buildctl` → containerd import), networking consolidation (`internal/net` removed; `netmgr` is the single allocator), removal of dead `internal/vmmanager`, per-user RBAC (per-user API tokens + `project_members`/`org_members` PG roles + fine-grained permission codes on every route), and a startup sanity check that fails loudly on misconfigured shim/jailer paths.

Remaining on this list:

- **A UI-native way to define multi-service apps.** Right now the *only* way to define a multi-service app is `POST /projects/compose` with a full `docker-compose.yml`. The intended primary path is dashboard forms — add a service, set its image/replicas/ports/env/`depends_on` directly in the UI, no YAML file to hand-write — with compose import kept as a convenience for bringing in an existing file, not the main interface. This is a real gap today, not just a framing preference: there's no form-driven way yet to build up a multi-service project piece by piece in the dashboard. **[PLANNED]**
- **Real persistent volumes mounted into the guest.** `POST /volumes` creates a real host dir + sparse `data.img` and delete/usage hit disk, but the block-device mount is not yet wired into VM boot (the `internal/volumes` path ends at the data file, not an attached drive). **[PARTIAL]**
- **Per-project email notifications for alerts.** The `[notify]` SMTP mailer ships (alerts fire out-of-band email when configured), but per-user email + opt-in resolution is a planned follow-up (`store.ProjectNotifyEmails` currently falls back to the configured default recipient). **[PARTIAL]**

If you're reading the code and something looks unfinished, check this list first — it's kept in sync with what's actually been verified against the source, not just filed as a wish.

---

## 📡 API Reference

- Base URL: `http://<host>:8080`
- Auth: `Authorization: Bearer <PORTER_API_TOKEN>` on every route except `GET /health`, `GET /csrf`, and the auth endpoints themselves.
- All input/output is JSON. Errors are `{ "error": "..." }`.
- Routes are registered in `backend/internal/api/api.go` on Go's `net/http` `ServeMux`; handler bodies live in `backend/internal/api/handlers_impl.go`. There are **277 registered routes** as of this revision — far more than the v0.1.0-beta scope in `PLAN.md` describes. Read the source for the authoritative list; the groups below are the ones worth knowing as a new contributor.

> **Route reality:** as of this revision every registered route has a real
> handler (verified by the two-way coverage test in `api/coverage_test.go` and
> an audit for empty-JSON stubs). Analytics/usage endpoints aggregate the real
> traffic ring; web-vitals ingest real beacon data; firewall rules are enforced
> by the gateway; crons fire real job microVMs; cache-purge paths work. Some
> routes (e.g. `POST /auth/signup`) intentionally return a single-tenant notice
> rather than creating a multi-tenant account — that is the designed behavior,
> not a stub. When in doubt, read the handler source.

### Core routes that are genuinely implemented

| Group | Examples |
|---|---|
| Health / auth | `GET /health`, `POST /auth/login`, `GET /auth/session` |
| Projects | `GET/POST /projects`, `POST /projects/compose`, `GET/PATCH/DELETE /projects/{id}`, `POST /projects/{id}/redeploy` |
| Scale | `GET/PATCH /projects/{id}/scale` |
| Replicas | `GET /projects/{id}/replicas`, `POST /projects/{id}/replicas/{n}/start\|stop\|restart`, `DELETE /projects/{id}/replicas/{n}` |
| Logs | `GET /projects/{id}/logs`, `GET /projects/{id}/replicas/{n}/logs?tail=200` |
| Domains | `GET/POST /projects/{id}/domains`, `DELETE /projects/{id}/domains/{id}`, `POST /projects/{id}/domains/{id}/verify` (currently a stub — see [Security & Trust Model](#-security--trust-model)) |
| Traffic | `GET /projects/{id}/traffic`, `GET /traffic` (global ring) |
| Volumes | `GET/POST /volumes`, `DELETE /volumes/{id}` (database rows only today — see [Planned / In Progress](#-planned--in-progress)) |
| Images | `GET /images` (catalog + golden images) |
| Events (SSE) | `GET /events` |
| Overview | `GET /overview`, `GET /host/overview` |

### Events (SSE)

`GET /events` streams `vm.state`, `replica.health`, `domain.status`, and related events as `data: <json>\n\n`.

### Compose

`POST /projects/compose { "name": "...", "compose_yaml": "..." }` parses and boots a Compose file (image-based services only — see [Compose Mapping Rules](#-compose-mapping-rules)). `400` on a parse error, with the parser's error message verbatim.

### Status codes

`200`, `201`, `202`, `400`, `401`, `404`, `409`, `500` — used fairly conventionally; `202` on routes that kick off an async boot/deploy.

---

## 📦 Compose Mapping Rules

`backend/internal/compose/compose.go` (`ParseCompose`) hand-parses a **deliberately constrained Compose v3 subset** — one service → one or more VMs (more replicas → more identical VMs). No external YAML dependency.

### Supported keys

| Key | Meaning | Notes |
|---|---|---|
| `image:` | OCI image ref each replica boots from | **required** per service |
| `restart:` | Restart policy (`on-failure`, etc.) | passed through to the VM lifecycle |
| `ports:` | `"<host>:<container>/<proto>"` or `"<host>:<container>"` | see the known bug below |
| `environment:` | `KEY=value` or `KEY: value` | flat key/value map — not encrypted at rest beyond the database itself, see [Security & Trust Model](#-security--trust-model) |
| `depends_on:` | list of services to boot first | topo-sorted (see below) |
| `deploy:` | `replicas: N` | default `1`; validated as integer |
| `healthcheck:` | `test:` and `interval:` | `test:` ⇒ HTTP check, else TCP; `interval` in seconds (`30s`) |

### Explicitly rejected

- **`build:`** — image-based services only. `ParseCompose` returns `compose parse error: … only image-based services are supported (no "build:")`. (Git-based build-from-source is planned separately — see [Planned / In Progress](#-planned--in-progress) — and will not go through this parser's `build:` key even once it lands.)
- **Circular `depends_on`** — refused with a `circular dependency` error.
- **`depends_on` of an unknown service** — refused.
- Any service missing `image:` — refused.

### Boot ordering

`depends_on` is resolved by a DFS topological sort (declaration order used as the tiebreak; each service's deps sorted for determinism). Services boot in topo order; a circular or unknown dependency halts parsing.

### ✅ Host ports: `ports:` mapping is honored

`parsePort` keeps both sides of a ports entry — `"8080:80"` parses to
`types.Port{HostPort: 8080, ContainerPort: 80}`. When the gateway is enabled,
a `PortForwarder` (`internal/gateway/portforward.go`) binds each running VM's
declared `HostPort` on the host and proxies TCP connections to the VM's
container port — so a compose `ports: ["8080:80"]` gets a real host listener.
The HTTP gateway upstream always targets the VM's **container** port (the port
the app listens on inside the microVM), never `HostPort`.

### Current parse constraints worth knowing

- Tabs are normalized to 4 spaces; comment stripping is `(^|\s)#…`.
- One section list/map (`ports`/`environment`/`depends_on`/`deploy`/`healthcheck`) per indent depth; items must sit one level deeper than their section header.
- A new top-level key (`networks:`, `volumes:`…) ends the `services:` block.
- Empty `services:` yields `no services found under services:`.
- `replicas: 0` or negative is clamped to `1`.

Tests in `backend/internal/compose/compose_test.go`.

---

## 🔑 SSH Access

Implemented in `backend/internal/sshgw`, gated off by default (`[ssh] enabled = false` in `porter.toml`).

```
operator → ssh <target> -p 2222
            │  (auth: cert signed by the gateway's own CA, generated on first run)
            ▼
          SSH Gateway (sshgw)   ← the only intended internet-facing SSH service
            │  look up the VM's containerd task in the store
            │  containerd task.Exec("/bin/sh"), pipe stdio
            ▼
        the VM's containerd task
```

No SSH server runs inside the guest — the gateway terminates the SSH session and bridges it into the VM's containerd task via `task.Exec()`, the same mechanism `ctr task exec` uses. This only works for containerd-booted (OCI image) VMs; "bare" rootfs-only VMs have no containerd task and `Exec` returns a clear error for those.

**Auth:** the gateway generates and persists its own CA and host key on first run (`ed25519`, stored under the configured data dir). `POST /vms/{id}/ssh-cert` (or the replica-scoped equivalent) issues a short-lived signed cert for a submitted public key. See [Security & Trust Model](#-security--trust-model) for why this should be treated as an admin/debug channel, not a general bastion, in its current form.

**Not yet implemented:** a CLI to drive any of the above, `-L`/`-R` port forwarding, SFTP/SCP, session recording. There is no `porter ssh` or `porter auth add-key` command — those would need the (not-yet-built) CLI.

---

## 🧱 Dashboard (frontend/)

The dashboard is a Vue 3 + Vite + Tailwind app in `frontend/` (Pinia for state, TanStack Query for data fetching, Reka UI components). It talks only to the Control API (HTTP + SSE). Views live in `frontend/src/views`: `Login`, `ProjectDetail`, `DeploymentsList`, `VmDetail`, `Domains`, `Logs`, `Traffic`, `Images`, `Servers`, `Teams`, `Settings`. Components include `ServiceCard`, `StatusBadge`, `HealthPill`, `Sparkline`, `ScaleModal`, `NewProjectModal`, `AddDomainModal`.

`vite.config.js` proxies API calls to `localhost:8080` in dev. Auth token stored client-side, sent as `Authorization: Bearer`.

---

## 🛠️ Build, Test, Run (development)

The root `Makefile` drives everything. The Go binary **embeds** the built frontend (`go:embed web/dist`), so the frontend builds first.

```bash
make frontend   # npm install + vite build → backend/web/dist
make backend    # go build ./cmd/porter → backend/porter
make build      # frontend then backend
make run        # build then run in the foreground (./backend/porter server $(ARGS))
make dev        # prints the two-terminal dev loop (go run ./cmd/porter + npm run dev)
make migrate    # run pending SQL migrations with golang-migrate
make test       # cd backend && go test ./...
make clean      # remove build artifacts
```

> `make backend`/`make build`/`make run` previously referenced a non-existent `./cmd/server` package (the real entrypoint is `./cmd/porter`) — this was a real bug that made a from-scratch `make build` fail outright. Fixed in this revision; flagging it here in case you have an older checkout or cached notes referencing `cmd/server`.

For local development against real microVM boots, see `backend/deploy/dev.sh` (spins up PostgreSQL in Docker, builds, and runs the binary against your host's containerd/KVM). For a full production install on a fresh Linux host, see `backend/deploy/install.sh` and `backend/deploy/README.md` — it installs PostgreSQL, containerd + devmapper, the `aws.firecracker` shim, the `firecracker` binary, and CNI, then builds and installs a systemd unit.

Run requirements: Linux + KVM (`/dev/kvm`), containerd with devmapper + the `aws.firecracker` runtime registered, a shared `vmlinux` kernel (`porter kernel set`).

---

## ⚙️ Configuration Reference

Config comes from `porter.toml`, layered over by `PORTER_*` environment variables (env wins when set). `LoadConfig` refuses to start unless `api_token`, `admin.password`, and a database URL are all set (each has a safe-looking default for the DB URL only — token and password have none, on purpose — see [Security & Trust Model](#-security--trust-model)). Parser: `backend/internal/config/toml.go`. Loading logic: `backend/internal/config/config.go`.

### `porter.toml` — actual current shape

```toml
[server]
listen_addr      = ":8080"
base_domain      = "porter.test"
api_token        = "change-me"        # REQUIRED — set a real secret, no working default
rate_limit_per_min = 0                # 0 disables

[database]
url          = "postgres://porter:porter@localhost:5432/porter?sslmode=disable"
auto_migrate = true

[firecracker]
containerd_socket  = "/run/containerd/containerd.sock"
snapshotter        = "devmapper"
namespace          = "porter"
logs_dir           = "/var/log/porter"
images_dir         = "vms/images"
custom_images_dir  = "vms/custom"
firecracker_bin    = "firecracker"
# kernel_image / rootfs_path: consumed by porter kernel set + docs; the
# host's /etc/containerd/firecracker-runtime.json is the shim's real source
# for jailer/seccomp settings — see Security & Trust Model.

[admin]
username = "admin"
password = "change-me"                # REQUIRED — set a real secret, no working default

# Everything below defaults OFF so a bare install keeps working with zero
# extra listeners.
[gateway]
enabled     = false   # Host-header reverse proxy + traffic logger; HTTP only,
                       # no TLS termination — see Security & Trust Model
listen_addr = ":80"

[dns]
enabled = false        # in-process .local resolver used by the gateway
                        # (not a real DNS server yet — see Planned / In Progress)

[health]
enabled = true          # healthcheck probes + auto-replace of unhealthy VMs

[ssh]
enabled     = false      # SSH gateway (task.Exec bridge) — admin/debug channel,
                          # see Security & Trust Model
listen_addr = ":2222"
```

### Environment variables

| Var | Overrides |
|---|---|
| `PORTER_LISTEN_ADDR` | `[server] listen_addr` |
| `PORTER_BASE_DOMAIN` | `[server] base_domain` |
| `PORTER_API_TOKEN` | `[server] api_token` *(required)* |
| `PORTER_RATE_LIMIT_PER_MIN` | `[server] rate_limit_per_min` |
| `PORTER_DATABASE_URL` | `[database] url` |
| `PORTER_AUTO_MIGRATE` | `[database] auto_migrate` |
| `PORTER_CONTAINERD_SOCKET` | *(not currently wired — set via `porter.toml` only; see note below)* |
| `PORTER_KERNEL_IMAGE` | `[firecracker] kernel_image` |
| `PORTER_ROOTFS_PATH` | `[firecracker] rootfs_path` |
| `PORTER_FIRECRACKER_BIN` | `[firecracker] firecracker_bin` |
| `PORTER_LOGS_DIR` | `[firecracker] logs_dir` |
| `PORTER_IMAGES_DIR` | `[firecracker] images_dir` |
| `PORTER_CUSTOM_IMAGES_DIR` | `[firecracker] custom_images_dir` |
| `PORTER_GATEWAY_ENABLED` / `PORTER_GATEWAY_LISTEN_ADDR` | `[gateway]` |
| `PORTER_DNS_ENABLED` | `[dns] enabled` |
| `PORTER_HEALTH_ENABLED` | `[health] enabled` |
| `PORTER_SSH_ENABLED` / `PORTER_SSH_LISTEN_ADDR` | `[ssh]` |
| `PORTER_ADMIN_USERNAME` / `PORTER_ADMIN_PASSWORD` | `[admin]` |

> **Note:** `[firecracker] containerd_socket` in `porter.toml` is read correctly, but there is currently no matching `PORTER_CONTAINERD_SOCKET` environment-variable override in `config.go` even though the troubleshooting docs (`PLAN.md`) reference one — set it via the TOML file for now.

> **Secrets hygiene:** prefer the `PORTER_API_TOKEN` / `PORTER_ADMIN_PASSWORD` environment variables (e.g. injected by systemd's `EnvironmentFile=` or your secrets manager) over committing real values into `porter.toml` on disk, especially if the host's config directory is backed up or version-controlled anywhere.

---

## 🗺️ Roadmap & Versions

The full, authoritative roadmap — including per-feature `[DONE]` / `[PARTIAL]` / `[PLANNED]` / `[STUBBED]` status tags and the Git-to-VM pipeline scoping notes — lives in **[`PLAN.md`](./PLAN.md)**. Our singular focus is **v1.0.0**: Firecracker + firecracker-containerd integration, Git-to-VM deployments, VM lifecycle, boot kernels, rootFS/OCI image support, UI-native multi-service deploys, login/dashboard, VM mgmt + logs, private bridge/static IP/port mapping, and the REST API. See the file for the phased breakdown.

---

## 🏛️ Platform Strategy

Porter is **your own Vercel-like platform.**

Anyone can download it to a VPS or bare metal server and instantly run their own SaaS-like system. It combines the orchestration power of Kubernetes (replicas, self-healing), the seamless deploys of Vercel and Render, the serverless scaling of AWS Fargate, and the simplicity of Docker (easy port sharing), but uses Firecracker microVMs under the hood instead of shared containers. You get the premium SaaS experience — complete with cold/hot start metrics, global traffic routing, live logs, SSH, analytics, Organizations, and Teams — fully self-hosted, with complete control over your own infrastructure, subject to the current implementation status documented above.

**Why MIT (over AGPL):** Maximum adoption. We want anyone to be able to run their own PaaS without copyleft obligations.

## 🧭 OSS & Future SaaS Strategy

Porter's control plane is **single-tenant by permanent design**, not a v1 limitation waiting to be lifted. The reasoning, in short: everyone holding the bearer token or admin credentials is fully trusted with the entire host (see [Security & Trust Model](#-security--trust-model)), and that trust model is what keeps the codebase simple enough to stay a one-operator, one-binary tool. A future hosted/SaaS offering, if it ever exists, would be a separate control plane built *on top of* many single-tenant Porter instances (one per customer host) — not a multi-tenancy retrofit of this codebase's auth model.

---

## 📄 License

MIT.