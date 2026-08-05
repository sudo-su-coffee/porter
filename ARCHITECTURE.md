# Porter — Architecture & API Reference (v1.0.0)

Full system design, API surface, compose-file mapping, domain model, and SSH access. For the short pitch and quickstart, see [`README.md`](./README.md). For UI spec, deployment, roadmap, and OSS strategy, see [`BUILD.md`](./BUILD.md).

## Contents

1. Architecture
2. API Reference
3. Compose Mapping Rules
4. Domains & Traffic Log
5. SSH Access

---

## 1. Architecture

### 1.1 Design principles

1. **One microVM per replica.** A compose service, or a single `--image` deploy, maps to one or more identical Firecracker microVMs — one per replica (`deploy.replicas`, default 1). This keeps the guest attack surface tiny, boot times fast (~125ms typical Firecracker boot), and the mental model simple: *VM = one replica of one service*.
2. **Don't reinvent what `firecracker-containerd` already solved.** Porter runs [`firecracker-containerd`](https://github.com/firecracker-microvm/firecracker-containerd) — the same runtime AWS itself uses (Fargate/Lambda lineage) — as its execution engine. `containerd` runs on the host as normal; a custom runtime shim (`runtime-type: "aws.firecracker"`) intercepts the standard containerd task lifecycle and boots one microVM per task instead of a namespaced process. Image pull, snapshotting, the in-VM agent, and jailer wiring are all `firecracker-containerd`'s job, not Porter's. Porter is a control-plane UI/API in front of it.
3. **Single front door.** Users never need to know or track per-VM IPs. HTTP goes through the gateway; SSH goes through the SSH gateway. Both route by name.
4. **Domains work like Vercel.** Point one wildcard DNS record at Porter, once. Every deploy is then reachable immediately with zero further DNS work — a stable subdomain for the live version, a unique one per deploy for previews. Fully-owned custom domains attach on top, whenever you want, via CNAME.
5. **Boring persistence.** State is a JSON-file-backed store in v1 (see `store` package) — no external DB dependency for a single-host deployment. Swappable for Postgres later without touching the API surface.
6. **UX target: Vercel, not vSphere.** Two top-level concepts only — **Projects** (a compose file or a single image) and **Deployments** (the resulting VM(s)). No cluster/node/pool concepts in v1.
7. **Single-tenant, permanently.** Porter is one operator's tool for their own host(s) — not a foundation for serving multiple untrusted customers from one instance. This isn't a v1.0.0 shortcut to be revisited; it's a fixed design stance. See BUILD.md, Section 4 (OSS & Future SaaS Strategy).

### 1.2 Components

#### 1.2.1 Control API (`backend/internal/api`)
Stateless HTTP server (Go, `chi` router). Owns:
- Project/VM CRUD endpoints (see Section 2 (API Reference))
- Compose file upload + parse trigger
- Domain attach/verify endpoints
- Server-Sent Events (SSE) stream for live VM state, domain status, and traffic events, used by the dashboard to update without polling
- Auth: v1.0.0 ships with a single static API token (`PORTER_API_TOKEN` env var) checked via bearer header. Multi-user auth is out of scope for v1 (see BUILD.md, Section 3 (Roadmap & Scope)).

#### 1.2.2 VM Manager (`backend/internal/vmmanager`)
A thin wrapper around a `containerd` client (`github.com/containerd/containerd`), talking to a containerd instance configured with the `firecracker-containerd` runtime shim. Responsibilities:
- Build the OCI runtime spec / task options for each VM (vcpu/mem sizing → `firecracker-containerd`'s `proto.FirecrackerMachineConfiguration`, kernel path, network config passed as kernel boot args to the shim)
- Boot lifecycle: `client.Pull()` (if not cached) → `container.NewTask()` → `task.Start()` → track task/container IDs in the store → graceful stop (`task.Kill(SIGTERM)`) → hard stop (`task.Kill(SIGKILL)`) fallback → `task.Delete()`/`container.Delete()` teardown
- Subscribes to containerd's task exit events (`client.Subscribe()` on the events service) to detect unexpected exits and flip state to `stopped`/`failed` in the store
- **Not** responsible for pulling or flattening images, running an in-VM init, or wiring the jailer — `firecracker-containerd` owns all of that. VM Manager only talks to containerd's task API.

#### 1.2.3 Compose Mapper (`backend/internal/compose`)
- Parses a (deliberately constrained) subset of Compose v3 syntax
- Translates each `services.<name>` entry into exactly one `ComposeService` → one containerd task → one VM
- Resolves `depends_on` into a boot order via topological sort; refuses (with a clear error) on circular dependencies
- Full mapping rules: Section 3 (Compose Mapping Rules)

#### 1.2.4 Network Manager (`backend/internal/netmgr`)
- Allocates one Linux `tap` device per VM. `firecracker-containerd`'s CNI-driven network setup (via its `tc-redirect-tap` plugin) attaches the tap to the microVM; Porter's job is to provide the CNI network config per project, not to hand-roll tap creation itself
- Allocates one `/24` bridge subnet per **project** (not per VM) from a configurable base range (default `10.42.0.0/16`, sliced into per-project `/24`s), so services within one compose project share an L2 segment and can reach each other by IP
- Cross-project isolation: each project's bridge is a separate Linux bridge with no route between them unless explicitly peered (not in v1 scope)
- Deterministic MAC generation from VM ID, so the same VM ID always gets the same MAC across restarts

#### 1.2.5 Porter Gateway (`backend/internal/gateway`)
The single HTTP front door. One process, three jobs, because all three are naturally the same layer — every request already passes through here:

- **Routing:** by `Host` header (subdomain or custom domain) or path prefix, resolved against the live VM→IP mapping in the store, so a VM restart (new IP) never requires manual route updates
- **Domains:** owns the Vercel-style subdomain model — stable + preview subdomains under the operator's wildcard, plus verified custom domains. Full detail: Section 4 (Domains & Traffic Log) §4.1
- **Traffic log:** records a bounded in-memory ring buffer of recent requests per VM (method, path, status, latency, remote IP — metadata only, never bodies), surfaced live in the dashboard via SSE. Full detail: Section 4 (Domains & Traffic Log) §4.2
- **Identity:** injects `X-Porter-VM-ID` / `X-Porter-Project-ID` headers on every proxied request, so backend services can trust routing identity without re-implementing auth

#### 1.2.6 L7 Service Pool — built-in load balancing (`backend/internal/gateway`)
The Gateway (§1.2.5) doesn't map a hostname to a single VM anymore — it maps a hostname to a **pool** of VM IDs, one pool per compose service (or per standalone deploy, which is just a pool of size 1). Concretely:
- Each service's stable subdomain resolves to its pool rather than one fixed VM; on each incoming request, the Gateway picks a target from the pool via round-robin and proxies to that VM's IP
- Pool membership updates atomically as replicas boot, crash, or get drained for health reasons (§1.2.7) — readers never see a half-updated pool, the swap is a single pointer/slice replace under the existing gateway lock
- Every proxied request is stamped with `X-Porter-Replica-ID` (in addition to the existing `X-Porter-VM-ID` / `X-Porter-Project-ID` headers) so backend services and the traffic log can distinguish which specific replica served a given request
- No external load balancer, Envoy, or Nginx involved — this is a few hundred lines of Go inside the Gateway process, same design posture as the rest of Porter ("pure Go, one binary")

#### 1.2.7 Health Checker (`backend/internal/health`)
A pure-Go goroutine per VM, started the moment a task reaches `running` and stopped when the VM is deleted. Responsibilities:
- Runs the probe declared by the service's `healthcheck:` (HTTP GET or TCP dial, see Section 3 (Compose Mapping Rules)) on the configured `interval`, with `timeout` per attempt and `retries` before flipping state
- Tracks each VM's health as `healthy` / `unhealthy` / `checking` (the last covering the `start_period` grace window before the first real verdict counts)
- On `unhealthy`: immediately drains that VM out of the Gateway's L7 pool (§1.2.6) so no new traffic routes to it, without touching already-in-flight requests
- On `unhealthy` **and** the service's `restart:` policy is `always` or `on-failure`: triggers the VM Manager to kill and replace that specific replica (same boot path as initial creation, reusing the replica's index/identity — see §1.2.8's `<service>-<n>` naming), then re-adds it to the pool once it reports `healthy` again
- Health state is exposed per-VM via the API (`GET /vms/{id}` → `health_status`) and rolled up per-service in project responses (`service_pools[service].healthy_count`)

#### 1.2.8 Embedded DNS — service discovery (`backend/internal/dns`)
A tiny authoritative DNS server (built on `github.com/miekg/dns`, still pure Go, no external binary) bound to each project's bridge gateway IP, answering only for that project's `.local` zone:
- `<service>.<project>.local` resolves to the **healthy** replica IPs for that service, round-robin, sourced from the same pool the Gateway maintains (§1.2.6/§1.2.7) — so DNS and the L7 Gateway never disagree about which replicas are up
- `<service>-<n>.<project>.local` resolves to one specific replica by index, for cases where a caller needs a pinned target rather than the load-balanced pool (mirrors the `<service>-<n>` naming used for SSH and replica identity elsewhere)
- Every VM in a project gets this DNS server injected as its resolver (passed the same way other per-VM network config reaches the guest — via the CNI/kernel-boot-args path already used for IP assignment, see §1.2.4), so standard `getaddrinfo`-based resolution just works inside the guest with no extra guest-side configuration
- This replaces the earlier `${PORTER_<SERVICE>_IP}` environment-variable workaround entirely — see Section 3 (Compose Mapping Rules) §3.4 for the updated networking story
- Scope stays intentionally small: one authoritative zone per project, no recursion, no external DNS forwarding — services needing to resolve the public internet use the guest's normal upstream resolver for anything outside `.local`, this server only ever answers for its own project's zone

#### 1.2.9 SSH access layer
`firecracker-containerd` boots a minimal in-VM agent (its own `agent` binary, statically linked, running as the guest's init) that speaks a gRPC-over-vsock protocol back to the shim for task lifecycle (start/stop/exec/IO) — this is what replaces a custom guest-init entirely. Porter builds interactive SSH on top of that instead of a hand-rolled network sshd:
- **Exec-based sessions (default):** the SSH Gateway authenticates the operator, then calls containerd's `task.Exec()` over the existing vsock control channel — with a shell (`/bin/sh` or the image's shell) — and streams stdio back over SSH. No SSH server runs inside the guest at all, no extra port, no baked-in dropbear.
- Because this rides the same vsock channel `firecracker-containerd` already uses for task control, there's no dependency on the guest's network stack being up or on any SSH daemon shipping inside the source image — it works for a bare `redis:7` with zero image customization, same guarantee as before, achieved differently.
- **With replicas (§1.2.6):** `porter ssh api` targets replica `0` of the `api` service pool by default; a specific replica is addressable as `porter ssh api-2`, matching the `<service>-<n>` naming used for DNS (§1.2.8) and health/restart identity (§1.2.7)
- Full detail: Section 5 (SSH Access)

#### 1.2.10 SSH Gateway (`backend/internal/sshgw`)
- Single SSH server the operator connects to (`ssh <vm-name>@gateway.<base-domain>` or via `porter ssh <name>` which wraps this)
- Terminates the operator's SSH session normally (cert-based or static-key auth, same as before — see Section 5 (SSH Access)), but instead of proxying TCP to an in-guest sshd, it translates the session into a containerd `task.Exec()` call against that VM's task and pipes stdio both ways
- On connect, the gateway looks up the target VM's containerd task ID from the store; the operator never sees or needs an internal IP, because this path never touches the guest's network at all
- No inbound SSH ports are exposed on VM bridges; there's nothing listening there to expose
- Full detail: Section 5 (SSH Access)

#### 1.2.11 State Store (`backend/internal/store`)
- In-memory maps guarded by a `sync.RWMutex`, persisted to a single JSON file on every write (`state.json`)
- Holds `VM` (now including containerd `ContainerID`/`TaskID`, `HealthStatus`, and `ReplicaIndex`), `Project` (now including per-service desired replica counts), and `Domain` records (see `pkg/types`)
- v1.0.0 explicitly does not use a real database — acceptable for single-host, single-operator scale; flagged in BUILD.md, Section 3 (Roadmap & Scope) as the first thing to swap for multi-host v2

#### 1.2.12 Dashboard (`frontend/`)
Next.js + React, Vercel-inspired IA. Full detail in BUILD.md, Section 1 (Dashboard UI Spec). Talks only to the Control API (REST + SSE), never touches the host, containerd, or Firecracker directly.

### 1.3 Data flow: booting a single image

```
User (CLI/UI)
  → POST /vms {image: "redis:7", name: "cache"}
  → API creates VM record (state=pending), returns 202 immediately
  → API kicks off async: VM Manager.CreateAndStart()
       1. containerd client.Pull("redis:7")
            - already present in containerd's content store? → skip pull
            - snapshotter (devmapper, block-device backed) prepares the
              VM's root block device from the image layers — this is
              firecracker-containerd's job, not Porter's
       2. Network Manager provisions the project's CNI network config,
          Network Manager assigns/tracks the IP for this VM
       3. container.NewTask(...) with FirecrackerMachineConfiguration
          (vcpu/mem, kernel path, jailer config) → task.Start()
            - firecracker-containerd's shim invokes the jailer, boots
              the microVM, starts its own in-VM agent as guest init
       4. VM state → running, containerd task/container IDs + IP recorded
          in store
       5. Gateway auto-registers cache.<base-domain> (stable subdomain)
  → Dashboard/CLI observes state via SSE / poll, shows "running" + URL + SSH command
```

### 1.4 Data flow: booting a compose project

```
User uploads docker-compose.yml
  → POST /projects/compose {name, compose_yaml}
  → Compose Mapper.Parse() → ordered []ComposeService (topo-sorted on depends_on)
  → API creates Project record, returns 202
  → API kicks off async, sequential per service (respecting boot order):
       for each service in order:
         1. Build N VM records, N = deploy.replicas (default 1), each
            tagged with its ReplicaIndex (0..N-1)
         2. VM Manager.CreateAndStart() per replica — same containerd task
            path as single-image boot, env vars passed as task Process.Env
         3. Health Checker starts a probe goroutine per replica once running
         4. Gateway adds each healthy replica to the service's L7 pool,
            registers stable + preview subdomains pointing at the pool
         5. Embedded DNS registers <service>.<project>.local (pool) and
            <service>-<n>.<project>.local (per replica)
         6. Register SSH gateway entries: "<project>-<service>-<n>" → VM ID
  → All services share one project bridge subnet; service A reaches
    service B via the embedded DNS name db.<project>.local (round-robin
    across healthy replicas) or db-0.<project>.local for a pinned replica
    — see Section 3 (Compose Mapping Rules) §3.4
```

### 1.5 Failure handling

| Failure | Behavior |
|---|---|
| Image pull fails (containerd `client.Pull()` error) | VM → `failed`, containerd's error surfaced verbatim in the UI; no partial task left running |
| `task.Start()` fails (jailer/shim error, e.g. cgroup or chroot setup failure) | VM → `failed` before any VMM process exists; error surfaced from the shim's task-create response |
| Firecracker process / task dies unexpectedly | Containerd event subscription (`TaskExit`) fires, flips state to `stopped`, dashboard shows "crashed" badge if it wasn't a user-initiated stop |
| Compose service fails mid-boot | Boot sequence halts; already-booted services in that project stay running; project shows partial state (`degraded`) so the user can retry just the failed service |
| Tap/CNI network setup fails | VM → `failed` before `task.Start()` is called, no leftover task or VMM process |
| Host reboot | v1.0.0 does **not** auto-resume VMs on host boot; this is explicitly deferred (see BUILD.md, Section 3 (Roadmap & Scope)) — all VMs simply show as `stopped` on next dashboard load, matching containerd's actual task state after a restart |
| Replica fails its health check | Health Checker (§1.2.7) drains it from the Gateway pool and embedded DNS immediately; if `restart: always`/`on-failure` is set, it's killed and replaced automatically, otherwise it's left `unhealthy` and visible in the dashboard for manual intervention |

### 1.6 Security notes (v1.0.0 scope)

- Single static API token — treat the Control API as trusted-network-only; do not expose port 8080 to the public internet without a reverse proxy adding real auth
- Porter has no multi-tenant isolation model and never will (see BUILD.md, Section 4 (OSS & Future SaaS Strategy)) — it assumes everyone who can reach the API token is fully trusted with everything on the host. Do not run Porter in any setup where that assumption doesn't hold.
- SSH gateway is the *only* SSH-facing thing meant to be internet-facing; since v1.0.0's SSH path is `task.Exec()` over vsock (§1.2.10), there is no guest-side network SSH daemon to expose or misconfigure at all
- Guest images run whatever the source image runs — no image scanning/policy engine in v1
- The Firecracker jailer (`chroot` + cgroups + seccomp) is configured and invoked by `firecracker-containerd`'s shim itself — this is the documented, recommended `firecracker-containerd` deployment mode, and Porter's runtime config simply turns it on rather than wiring it by hand

### 1.7 Why not just use plain containers?

This project deliberately trades container-level density for **VM-grade isolation with near-container boot times**, because Firecracker's whole reason to exist is exactly that trade-off (it's what AWS Lambda/Fargate run on). If the goal were maximum density and you trusted all workloads equally, plain containerd with the default `runc` shim would be the simpler answer. Porter configures containerd with the `firecracker-containerd` shim specifically for cases where per-service kernel isolation actually matters — while still getting containerd's mature image pull, snapshot, and task-lifecycle machinery for free instead of rebuilding it.

### 1.8 What this replaced, and why it's a real improvement

An earlier draft of this document specified a fully custom pipeline: a hand-rolled OCI image puller/flattener, a from-scratch `guest-init` PID-1 binary (mount filesystems, bring up networking, reap zombies, run an embedded sshd), and manual jailer wiring. All three were flagged as the riskiest unbuilt parts of the whole plan — `guest-init` in particular is genuine, unforgiving systems programming with no partial-credit failure mode (get PID 1 wrong and nothing boots).

Standardizing on `firecracker-containerd` deletes all three at once:
- The custom OCI puller/flattener → containerd's existing pull + snapshotter
- The custom `guest-init` agent → `firecracker-containerd`'s own in-VM agent
- Manual jailer wiring → the shim's own jailer integration, which is its documented default mode

What's left for Porter to actually build is genuinely smaller: a control API, a compose mapper, a gateway, an SSH gateway that rides the existing vsock task-exec channel, and a thin client over containerd's task API. The remaining open question worth resourcing early is the **kernel image** — `firecracker-containerd` still needs a `vmlinux` built/configured for Firecracker guest boot, shared across all VMs; that's still a real one-time setup step and worth a smoke-tested "does a task actually boot and reach `running`" checklist before layering on domains/SSH/compose.
## 2. API Reference

Base URL: `http://<host>:8080`
Auth: `Authorization: Bearer <PORTER_API_TOKEN>` on every request except `/health`.
All bodies/responses are JSON.

---

### Health

#### `GET /health`
No auth required.

```json
{"status": "ok"}
```

---

### VMs

#### `GET /vms`
List all VMs across all projects (including standalone).

**200**
```json
[
  {
    "id": "5b2e...",
    "name": "cache",
    "project_id": "",
    "service_name": "",
    "state": "running",
    "health_status": "healthy",
    "replica_index": 0,
    "image": "redis:7",
    "vcpus": 1,
    "mem_mib": 256,
    "ip_address": "10.42.0.5",
    "ports": [],
    "created_at": "2026-08-04T10:00:00Z",
    "started_at": "2026-08-04T10:00:02Z"
  }
]
```

`health_status` is `"healthy"` | `"unhealthy"` | `"checking"` — the last meaning the service's `start_period` grace window hasn't elapsed yet, or the service declares no `healthcheck:` at all (in which case it stays `"checking"` indefinitely and is never drained from its pool for health reasons — see Section 3 (Compose Mapping Rules)). `replica_index` starts at 0; standalone single-image VMs are always index 0 of a pool of size 1.

#### `POST /vms`
Create + boot a single VM from an image.

**Request**
```json
{
  "name": "cache",
  "image": "redis:7",
  "vcpus": 1,
  "mem_mib": 256,
  "env": {"REDIS_PASSWORD": "..."},
  "ports": [{"container_port": 6379, "protocol": "tcp"}]
}
```

**202 Accepted** — returns the VM record immediately in `pending` state; boot continues async. Poll `GET /vms/{id}` or subscribe to `GET /events` for state transitions.

#### `GET /vms/{id}`
Fetch one VM's current state.

**200** — same shape as list item.
**404** — `{"error": "vm not found"}`

#### `POST /vms/{id}/stop`
Graceful stop (ACPI shutdown attempt, then hard stop after 5s timeout). Tears down the tap device.

**200**
```json
{"status": "stopped"}
```

#### `POST /vms/{id}/start`
Re-boot a stopped VM (re-runs the full create+start path with the same image/config).

**202 Accepted** — VM record, state transitions to `booting`.

#### `DELETE /vms/{id}`
Stop (if running) and permanently remove the VM record. Also removes any domains and SSH gateway entries pointing at it.

**200**
```json
{"status": "deleted"}
```

---

### Projects (compose)

#### `GET /projects`
List all projects.

**200**
```json
[
  {
    "id": "9a1c...",
    "name": "my-app",
    "created_at": "2026-08-04T10:00:00Z",
    "network": "10.42.3.0/24",
    "vm_ids": ["...", "..."],
    "source": "compose"
  }
]
```

#### `POST /projects/compose`
Create a project from a `docker-compose.yml`. Each service becomes one VM, booted in dependency order.

**Request**
```json
{
  "name": "my-app",
  "compose_yaml": "version: '3'\nservices:\n  api:\n    image: myapp/api:latest\n    ports:\n      - \"3000:3000\"\n  worker:\n    image: myapp/worker:latest\n    depends_on:\n      - api\n"
}
```

**202 Accepted** — returns the Project record plus a `service_pools` map showing each service's replica state as it fills in:
```json
{
  "id": "9a1c...",
  "name": "my-app",
  "vm_ids": ["...", "..."],
  "service_pools": {
    "api": {"desired": 3, "healthy": 2, "vms": ["5b2e...", "7c1a...", "9f0d..."]},
    "worker": {"desired": 1, "healthy": 1, "vms": ["3d4e..."]}
  }
}
```
`desired` comes from each service's `deploy.replicas` (default 1). `healthy` lags `desired` while replicas are still booting or passing their `start_period`. Subscribe to `GET /events` for progress.

**400** — parse errors are returned verbatim, e.g.:
```json
{"error": "compose parse error: service \"worker\": only image-based services are supported (no `build:`)"}
```

#### `GET /projects/{id}`
Fetch one project, including its current `vm_ids` and `service_pools`.

#### `GET /projects/{id}?expand=vms`
Same as above but embeds full VM objects instead of just IDs, so the dashboard can render a project page in one call.

#### `PATCH /projects/{id}/services/{service}/scale`
Change a service's replica count.

**Request**
```json
{"replicas": 5}
```

**202 Accepted**
```json
{"service": "api", "desired": 5, "current": 3, "vms": ["5b2e...", "7c1a...", "9f0d..."]}
```
Scaling up boots new replicas (next available indices) asynchronously; scaling down stops and removes the highest-indexed replicas first, waiting for in-flight requests to drain from the Gateway pool before killing each one. `current` reflects the count at request time; poll `GET /projects/{id}?expand=vms` or subscribe to `GET /events` to watch it converge on `desired`.

#### `DELETE /projects/{id}/services/{service}`
Scale a service to zero — stops and deletes all of its replicas, removes it from the Gateway pool and embedded DNS, but leaves the rest of the project (and the project record itself) intact.

**200**
```json
{"status": "removed"}
```

#### `DELETE /projects/{id}`
Stops and deletes every VM in the project, removes all associated domains and SSH gateway entries, then deletes the project record.

**200**
```json
{"status": "deleted"}
```

---

### Domains

#### `GET /vms/{id}/domains`
List all domains (stable, preview, custom) currently routed to a VM.

**200**
```json
[
  {"domain": "api.example.com", "type": "stable", "status": "verified"},
  {"domain": "api-a1b2c3.example.com", "type": "preview", "status": "verified"},
  {"domain": "shop.mybrand.com", "type": "custom", "status": "pending"}
]
```

#### `POST /vms/{id}/domains`
Attach a custom domain to a VM.

**Request**
```json
{"domain": "shop.mybrand.com"}
```

**202 Accepted**
```json
{
  "domain": "shop.mybrand.com",
  "type": "custom",
  "status": "pending",
  "required_record": {"type": "CNAME", "name": "shop.mybrand.com", "value": "gateway.example.com"}
}
```
Status flips to `verified` once the gateway confirms the CNAME resolves correctly (polled every `PORTER_DOMAIN_VERIFY_INTERVAL`, default 30s). No traffic is routed to the custom domain until `verified`.

#### `DELETE /vms/{id}/domains/{domain}`
Detach a custom domain. Stable and preview subdomains cannot be deleted individually — they're managed automatically by the deploy lifecycle (see Section 4 (Domains & Traffic Log), §4.1).

**200**
```json
{"status": "removed"}
```

---

### Traffic log

#### `GET /vms/{id}/traffic?limit=200`
Returns the most recent requests from the in-memory ring buffer for that VM (metadata only, no bodies). See Section 4 (Domains & Traffic Log) §4.2.

**200**
```json
[
  {"timestamp": "2026-08-04T12:03:41.221Z", "method": "GET", "host": "api.example.com", "path": "/v1/orders", "status": 200, "duration_ms": 42, "remote_ip": "203.0.113.4"}
]
```

---

### Events (SSE)

#### `GET /events`
Server-Sent Events stream of state transitions, used by the dashboard instead of polling.

```
event: vm.state
data: {"vm_id": "5b2e...", "state": "running", "ip_address": "10.42.0.5"}

event: vm.state
data: {"vm_id": "5b2e...", "state": "failed", "error": "mkfs.ext4 failed: ..."}

event: project.progress
data: {"project_id": "9a1c...", "booted": 2, "total": 4}

event: traffic.request
data: {"vm_id": "5b2e...", "method": "GET", "path": "/v1/orders", "status": 200, "duration_ms": 42}

event: domain.status
data: {"vm_id": "5b2e...", "domain": "shop.mybrand.com", "status": "verified"}

event: replica.health
data: {"vm_id": "5b2e...", "service": "api", "health_status": "unhealthy"}

event: pool.updated
data: {"project_id": "9a1c...", "service": "api", "desired": 5, "healthy": 4}
```

---

### SSH

#### `GET /vms/{id}/ssh-info`
Returns what the CLI needs to construct an SSH command through the gateway (no raw key material in the response).

**200**
```json
{
  "gateway_host": "gateway.example.com",
  "gateway_port": 2222,
  "target_name": "my-app-api-0",
  "command": "ssh my-app-api-0@gateway.example.com -p 2222"
}
```
For a service running multiple replicas, `porter ssh <service>` targets replica `0` by default; request `/vms/{id}/ssh-info` for a specific replica's VM ID to get its own `<service>-<n>` target name.

#### `POST /vms/{id}/ssh-cert`
Requests a short-lived (default 10 min) SSH certificate for interactive login, signed by the gateway CA. Used internally by `porter ssh`; exposed for programmatic/CI use.

**Request**
```json
{"public_key": "ssh-ed25519 AAAA..."}
```

**200**
```json
{
  "certificate": "ssh-ed25519-cert-v01@openssh.com AAAA...",
  "expires_at": "2026-08-04T10:15:00Z"
}
```

---

### Logs

#### `GET /vms/{id}/logs?tail=200&follow=true`
Streams the task's stdout/stderr, captured via containerd's normal task IO (piped over the vsock control channel `firecracker-containerd` already uses, same path as `ctr task attach`). `follow=true` keeps the connection open (chunked transfer) and streams new lines as they arrive.

---

### Error format

All non-2xx responses:
```json
{"error": "human-readable message"}
```

### Status codes used

| Code | Meaning |
|---|---|
| 200 | Success (sync operation) |
| 202 | Accepted (async operation started, poll or subscribe for completion) |
| 400 | Bad request (validation, parse errors) |
| 401 | Missing/invalid API token |
| 404 | Resource not found |
| 409 | Conflict (e.g. delete requested while already deleting) |
| 500 | Internal error (host-level failure, logged server-side with full detail) |
## 3. Compose Mapping Rules

Core rule: **one `services.<name>` entry = one or more identical microVMs** (one per replica, `deploy.replicas` default 1). There is no "multiple containers sharing one VM" mode — replicas are always separate VMs, never one VM running multiple copies of a process.

### 3.1 Supported top-level keys

| Compose key | Supported | Notes |
|---|---|---|
| `version` | ✅ (ignored) | Parsed but not enforced |
| `services` | ✅ | Required |
| `networks` | ❌ (v1) | All services in a project share one flat bridge subnet automatically; custom network topologies are ignored with a warning |
| `volumes` | ❌ (v1) | Named/bind volumes are not mounted into the VM in v1 — flagged loudly at parse time if present. Roadmap: virtio-fs pass-through |
| `secrets` | ❌ (v1) | Not supported; use `environment` for now |

### 3.2 Supported service keys

| Service key | Supported | Mapping |
|---|---|---|
| `image` | ✅ **required** | Pulled directly via OCI registry client, flattened to rootfs. `build:` is explicitly **not supported** — services must reference a pre-built, pushed image. Presence of `build:` without `image:` is a hard parse error. |
| `environment` (map or list form) | ✅ | Both `KEY: value` map syntax and `- KEY=value` list syntax accepted. Injected into the guest via a boot-time env file guest-init reads before exec'ing the entrypoint. |
| `ports` | ✅ | `"host:container"`, `"host:container/proto"`, and bare `"container"` forms all parsed. Each becomes a Porter Gateway route (stable + preview subdomain, see Section 4 (Domains & Traffic Log), §4.1) *and* is noted for direct TCP/UDP forwarding (roadmap item for non-HTTP protocols — v1.0.0 gateway is HTTP/1.1 + HTTP/2 only). |
| `depends_on` (list or map form) | ✅ | Used for **boot ordering** (topological sort) — a dependency is considered "ready to proceed" the instant its first replica reaches `running`, not necessarily `healthy`. Note this is boot-order sequencing, distinct from the `healthcheck`/`restart` support above: a dependency having a passing healthcheck does not currently gate when its dependents start booting (that stronger form of readiness-gating is still a roadmap item — see BUILD.md, Section 3 (Roadmap & Scope)). |
| `command` | ✅ | Overrides the image's default `Cmd` (but not `Entrypoint`) — same override semantics as `docker run`. |
| `deploy.resources.limits.cpus` / `.memory` | ✅ | Maps to `vcpus` / `mem_mib`. `cpus: "0.5"` rounds up to 1 vCPU (Firecracker doesn't do fractional vCPUs). `memory: "512m"` / `"1g"` both parsed. |
| `deploy.replicas` | ✅ **Supported** | Boots N identical VMs for the service (default 1 if omitted). Each gets a `ReplicaIndex` (0..N-1), its own `<service>-<n>` SSH target, and membership in the service's Gateway load-balancing pool. See Section 1 (Architecture) §1.2.6. |
| `healthcheck` | ✅ **Supported** | `test: ["CMD", "curl", "-f", "http://localhost:PORT/path"]` is treated as an HTTP probe against that path; `test: ["CMD", "nc", "-z", "localhost", "PORT"]` (or any `nc -z` form) is treated as a TCP dial probe. `interval` (default `30s`), `timeout` (default `5s`), `retries` (default `3`), and `start_period` (default `0s`) are all read and applied. Full behavior: Section 1 (Architecture) §1.2.7. |
| `restart` | ✅ **Supported** | `"always"` and `"on-failure"` both enable auto-replacement: an unhealthy replica is killed and re-booted in place by the Health Checker. `"no"` (or omitted) leaves failed replicas as-is for manual intervention, matching v1.0.0's original conservative default. |
| `build` | ❌ (hard error if present without `image`) | |
| `volumes` (service-level) | ❌ (v1) | Ignored with a warning in the parse response |
| `networks` (service-level) | ❌ (v1) | Ignored — all services in one project land on the same subnet automatically |

### 3.3 Boot ordering

Services are topologically sorted on `depends_on` before boot. Example:

```yaml
services:
  db:
    image: postgres:16
  api:
    image: myapp/api:latest
    depends_on: [db]
  worker:
    image: myapp/worker:latest
    depends_on: [api, db]
```

Boot order: `db` → `api` → `worker`. If a cycle is detected (`a depends_on b`, `b depends_on a`), the whole project creation is rejected at parse time with a clear error naming the cycle — nothing is booted.

### 3.4 Networking between services — embedded DNS

All services within one project share a single `/24` bridge subnet, and Porter runs a small authoritative DNS server on that bridge so services can reach each other **by name**, not by IP:

- `db.<project>.local` resolves to the **pool** of `db`'s healthy replica IPs, round-robin — this is the normal way to reach a dependency, and it stays correct automatically as replicas scale, restart, or fail health checks
- `db-0.<project>.local` resolves to one **specific** replica by index, for the less-common case where a caller needs a pinned target instead of the load-balanced pool
- Every VM in the project gets this DNS server as its resolver automatically — no guest-side configuration needed, standard name resolution just works

```yaml
services:
  db:
    image: postgres:16
    deploy:
      replicas: 2
  api:
    image: myapp/api:latest
    depends_on: [db]
    environment:
      DATABASE_URL: "postgres://user:pass@db.my-app.local:5432/app"
```

Full detail on how the DNS server is scoped and injected: Section 1 (Architecture) §1.2.8. This replaces the `${PORTER_<SERVICE>_IP}` environment-injection approach from earlier drafts — DNS names are now the primary, supported mechanism, since they keep working correctly as replica counts and pool membership change, which a one-time env-var substitution could not.

### 3.5 Domains for compose services

Given:
```yaml
services:
  api:
    image: myapp/api:latest
    ports:
      - "3000:3000"
```

Porter Gateway auto-registers, at boot:
- `api.<project-name>.<base-domain>` — the **stable** URL for this service, load-balanced (round-robin) across all of the service's currently-healthy replicas — not a single fixed VM
- `api-<deploy-id>.<project-name>.<base-domain>` — a **preview** URL unique to this specific deploy, also load-balanced across that deploy's replicas if `deploy.replicas > 1`, useful for testing before promoting

Full domain model: Section 4 (Domains & Traffic Log) §4.1.

### 3.6 What happens to unsupported keys

Parsing is **permissive but loud**: unknown/unsupported top-level or service keys don't hard-fail the whole file — they're collected into a `warnings` array returned alongside the `202` project-creation response, e.g.:

```json
{
  "id": "9a1c...",
  "name": "my-app",
  "warnings": [
    "top-level `volumes` ignored in v1.0.0",
    "service \"worker\": `networks` (service-level) ignored — all services in one project share the project's subnet"
  ]
}
```

The only **hard errors** (project creation fully rejected) are:
- A service with `build:` and no `image:`
- A `depends_on` cycle
- A `depends_on` reference to a service name that doesn't exist in the file
- Empty `services:` block
## 4. Domains & Traffic Log

Both features live inside the **Porter Gateway** (`backend/internal/gateway`) — the single HTTP front door already handling routing. Domain assignment and request-level traffic visibility are both naturally proxy-layer concerns, so no new component is needed.

---

### 4.1 Domains — the Vercel model

#### 4.1.1 One-time setup: point your wildcard at Porter

You own a domain, e.g. `example.com`. Once, at install time, you create:

```
*.example.com        A       <host-public-ip>
```

Then tell Porter:
```bash
porter domain set-base example.com
```

From this point forward, **every deploy is reachable with zero further DNS work** — that's the entire point of the wildcard.

#### 4.1.2 Two auto subdomains per service, always

Every service that declares a port gets **two** subdomains the instant it boots, matching the Vercel pattern of a stable production URL plus a unique preview URL per deploy:

| Type | Pattern | Points at |
|---|---|---|
| **Stable** | `<service>.<project>.example.com` | Whichever VM for that service is currently the *live* one — stays constant across restarts/redeploys of the same service |
| **Preview** | `<service>-<deploy-id>.<project>.example.com` | This *specific* deploy's VM, forever (or until that VM is deleted) — lets you test a new version at its own URL before it becomes the stable one |

Standalone (non-compose) VMs follow the same pattern without the project segment:
```
cache.example.com                 (stable)
cache-a1b2c3.example.com          (preview, this specific deploy)
```

**Promoting a preview to stable:** when you redeploy a service (`porter up` again with the same name), the new VM boots, gets its own fresh preview URL immediately, and — once it reaches `running` — Porter Gateway atomically repoints the **stable** subdomain at the new VM. The previous VM keeps its own preview URL working (still reachable directly) until you delete it, so nothing breaks mid-cutover and you always have a rollback target one command away (`porter rollback <service>`).

#### 4.1.3 Custom domains (bring-your-own, CNAME)

Attach any fully-owned domain to a service whenever you're ready:

```bash
porter domains add shop.mybrand.com --service my-app-api
```

or via the dashboard's Domains tab.

**Setup flow:**
1. Operator adds the domain → Control API records it, returns the exact DNS record to create
2. Operator creates a CNAME at their DNS provider:
   ```
   shop.mybrand.com   CNAME   gateway.example.com
   ```
3. Gateway verifies the CNAME resolves correctly (polls every `PORTER_DOMAIN_VERIFY_INTERVAL`, default 30s) before routing live traffic to it — prevents accidentally serving someone else's misdirected traffic
4. Once verified, requests to `shop.mybrand.com` route the same as a stable subdomain would, including the same `X-Porter-VM-ID` / `X-Porter-Project-ID` identity headers

A custom domain always points at a service's **stable** slot (tracks whichever VM is currently live for that service) — it is not attached to one specific deploy's preview VM, matching how production custom domains behave on Vercel.

#### 4.1.4 TLS

- v1.0.0 does **not** ship automatic Let's Encrypt provisioning for either wildcard subdomains or custom domains. Operators front Porter with their own TLS termination (Cloudflare, a load balancer, etc.), or wait for the roadmap item.
- **Deferred to v1.1:** automatic wildcard + per-custom-domain TLS via ACME (HTTP-01/DNS-01), matching how Vercel/Netlify auto-provision certs on domain add.

#### 4.1.5 Config

| Env var | Default | Purpose |
|---|---|---|
| `PORTER_BASE_DOMAIN` | *(required, no default)* | The wildcard base domain, e.g. `example.com` — set via `porter domain set-base` |
| `PORTER_DOMAIN_VERIFY_INTERVAL` | `30s` | How often the gateway re-checks pending custom-domain CNAMEs |

#### 4.1.6 Domain → route resolution order

1. Exact match on a verified custom domain (`Host` header)
2. Exact match on a stable subdomain
3. Exact match on a preview subdomain
4. Path-prefix match (`/<project>/<service>/...`) — fallback/manual-testing path
5. No match → `502` with a clear "no route for `<host><path>`" body

#### 4.1.7 What happens to domains when a VM is deleted

- Deleting a **non-live** (older preview) VM removes only its own preview subdomain
- Deleting the **currently-live** VM for a service removes its stable subdomain too — the service shows as unreachable in the dashboard until redeployed, rather than silently falling back to an older VM
- Custom domains are **never** silently reassigned. If their target service's live VM is deleted, the custom domain is shown as "unassigned" in the dashboard until reattached to something

---

### 4.2 Traffic log

#### 4.2.1 Scope

In-memory ring buffer per gateway process. No disk persistence, no external log drain in v1.0.0. Restarting the gateway process clears the log — this is a live/recent-activity view, not an audit trail.

#### 4.2.2 What's captured per request

```json
{
  "timestamp": "2026-08-04T12:03:41.221Z",
  "method": "GET",
  "host": "api.my-app.example.com",
  "path": "/v1/orders",
  "status": 200,
  "duration_ms": 42,
  "vm_id": "5b2e...",
  "project_id": "9a1c...",
  "service_name": "api",
  "remote_ip": "203.0.113.4",
  "bytes_out": 1834
}
```

Ring buffer default size: last **2,000 requests per gateway process** (configurable, `PORTER_TRAFFIC_LOG_SIZE`), oldest entries drop as new ones come in.

#### 4.2.3 Where it shows up

- **Project/VM detail page** — a "Traffic" tab, live-updating table (via the same SSE mechanism already used for state events — a `traffic.request` event type), most recent first
- Filterable client-side by status code range (2xx/4xx/5xx), method, and path substring — filters the buffer already sent to the browser, no new query round-trip
- A small live requests/sec sparkline at the top, computed client-side from the buffer's timestamps

#### 4.2.4 API surface

Covered in Section 2 (API Reference) — `GET /vms/{id}/traffic` and the `traffic.request` SSE event. No new endpoints beyond those two.

#### 4.2.5 Explicitly deferred (not v1.0.0)

- Persisted/rotated log files on disk
- Log drains / export to external systems
- Per-project or global aggregate traffic dashboards (v1.0.0 ships per-VM/per-service only)
- Long-window analytics (daily/weekly traffic trends) — the ring buffer is a "what's happening right now" view
- Request/response body capture — headers/metadata only, never bodies, in v1.0.0
## 5. SSH Access

### Goal

`porter ssh <vm-name>` should just work, from anywhere the operator has network access to the gateway — no hunting for internal bridge IPs, no per-VM port forwarding, no manually-managed `known_hosts` chaos as VMs come and go.

**With replicas:** `porter ssh api` connects to replica `0` of the `api` service's pool by default. To reach a specific replica, target it directly by name — `porter ssh api-2` — matching the same `<service>-<n>` naming used for the embedded DNS names and health/restart identity (see Section 1 (Architecture) §§1.2.6–1.2.8). This is unrelated to which replica is currently serving traffic through the load balancer; SSH always goes to the exact replica you name.

### Model: single gateway, certificate-based, routes by name — backed by `task.Exec()`, not a guest sshd

```
 operator
    │  ssh my-app-api@gateway.example.com -p 2222
    ▼
┌─────────────────────────┐
│   SSH Gateway (sshgw)     │   ← only SSH-facing thing exposed to the operator
│  - authenticates operator│
│  - issues short-lived    │
│    cert OR checks it     │
│  - looks up VM's          │
│    containerd task ID     │
│  - calls task.Exec(shell)│
│    and pipes stdio        │
└──────────┬───────────────┘
           │  containerd task API, over the same vsock
           │  channel firecracker-containerd's in-VM
           │  agent already uses for task control
           ▼
  ┌─────────────────────┐
  │  in-VM agent           │  ← ships with firecracker-containerd,
  │  (guest init, runs     │     not built or baked in by Porter
  │  the exec'd shell)     │
  └─────────────────────┘
```

There is no SSH server running inside the guest at all in v1.0.0. The SSH Gateway terminates a real SSH session with the operator, then internally turns it into a `task.Exec()` call against that VM's containerd task and streams stdio both ways — the same mechanism `ctr task exec` uses.

### Why this instead of exposing (or baking in) a guest sshd

- One place to manage auth, audit, and revocation instead of N — same rationale as a bastion, but there's no second hop to a real network daemon after the gateway
- Nothing to expose on VM bridges at all: the exec path never touches the guest's network stack, so there's no in-guest port to firewall, no host key to manage per VM, and a VM with no network configured yet is still SSH-reachable
- Renaming/restarting a VM never breaks the operator's muscle memory (`ssh my-app-api` always works) — the gateway just resolves the name to a current task ID instead of a current IP
- No per-image dependency: a bare `redis:7` with zero customization is exec-able the moment its task is `running`, because the exec path rides infrastructure `firecracker-containerd` already provides for every task, not something Porter has to inject into the image

### Auth flow (two options, v1.0.0 ships both)

#### Option A — Certificate flow (recommended, default for `porter ssh`)
1. Operator runs `porter ssh my-app-api`
2. CLI generates (or reuses) a local ephemeral ed25519 keypair if none cached
3. CLI calls `POST /vms/{id}/ssh-cert` with the public key, authenticated via the same API token used for the dashboard
4. Gateway's CA signs a certificate valid for 10 minutes (configurable), scoped to that one VM's principal name
5. CLI opens the SSH connection using the signed cert; gateway validates the cert against its own CA, extracts the target VM name from the cert's principal, looks up the VM's current containerd task ID in the store, and calls `task.Exec()` to start a shell, piping the SSH session's stdio to/from it
6. Certificate expires after 10 minutes — no long-lived SSH keys sitting around

#### Option B — Static authorized key (simple mode, opt-in)
For operators who just want `~/.ssh/config` to work without the CLI wrapper:
1. Operator adds their public key once via `porter auth add-key ~/.ssh/id_ed25519.pub`
2. Key is trusted by the gateway indefinitely (until revoked)
3. Plain `ssh <vm-name>@gateway.example.com -p 2222` works directly, e.g. from any terminal or IDE's SSH integration — no CLI required for the connection itself, only for the one-time key registration

Static keys are logged and can be listed/revoked via `porter auth list-keys` / `porter auth revoke-key <fingerprint>`.

### Guest-side setup: none required

Unlike a network-sshd design, there's no per-image baking step at all. `firecracker-containerd`'s in-VM agent is what runs as guest init on every VM regardless of source image, and `task.Exec()` is a capability it provides out of the box — Porter doesn't inject a binary, a CA key, or a host key into any image.

This means **every VM is exec-reachable by default**, with zero per-image configuration required — even a bare `redis:7` image booted with no customization gets a working shell the moment its task reaches `running`.

One tradeoff worth naming: because there's no real network sshd in the guest, an operator can't bypass the gateway and `ssh` directly into a VM's bridge IP for local debugging the way they could with a baked-in dropbear. Everything goes through `task.Exec()`, which means everything also goes through the gateway's auth and logging — a deliberate constraint, not an oversight.

### What you get once connected

A shell (`/bin/sh`, or the image's configured shell) as `root` inside the guest's minimal environment — whatever the source image's filesystem provides, nothing injected into it. Since there's no in-guest Docker daemon, this is a direct look at exactly what the service process sees.

### Session logging (v1.0.0 scope)

- The gateway logs connection metadata (who, which VM, when, cert fingerprint or key fingerprint used, duration) — not full session content/keystrokes in v1
- Full session recording (asciinema-style) is a roadmap item, not v1.0.0

### Limitations in v1.0.0

- No SSH access to a VM that's `stopped` — the gateway refuses with a clear error rather than hang, since there's no IP to route to
- No port-forwarding (`-L`/`-R`) support through the gateway yet — direct interactive sessions only
- No SFTP/SCP explicitly tested/supported yet (dropbear technically supports it, but it's not part of the v1.0.0 test matrix — treat as best-effort)
- Gateway itself is a single process/single point of failure in v1 — acceptable for the single-host target, flagged for HA work in BUILD.md, Section 3 (Roadmap & Scope)
