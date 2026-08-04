# Architecture — Porter v1.0.0

## 1. Design principles

1. **One microVM per unit of work.** A compose service, or a single `--image` deploy, always maps to exactly one Firecracker microVM. This keeps the guest attack surface tiny, boot times fast (~125ms typical Firecracker boot), and the mental model simple: *VM = service*.
2. **Don't reinvent what `firecracker-containerd` already solved.** Porter runs [`firecracker-containerd`](https://github.com/firecracker-microvm/firecracker-containerd) — the same runtime AWS itself uses (Fargate/Lambda lineage) — as its execution engine. `containerd` runs on the host as normal; a custom runtime shim (`runtime-type: "aws.firecracker"`) intercepts the standard containerd task lifecycle and boots one microVM per task instead of a namespaced process. Image pull, snapshotting, the in-VM agent, and jailer wiring are all `firecracker-containerd`'s job, not Porter's. Porter is a control-plane UI/API in front of it.
3. **Single front door.** Users never need to know or track per-VM IPs. HTTP goes through the gateway; SSH goes through the SSH gateway. Both route by name.
4. **Domains work like Vercel.** Point one wildcard DNS record at Porter, once. Every deploy is then reachable immediately with zero further DNS work — a stable subdomain for the live version, a unique one per deploy for previews. Fully-owned custom domains attach on top, whenever you want, via CNAME.
5. **Boring persistence.** State is a JSON-file-backed store in v1 (see `store` package) — no external DB dependency for a single-host deployment. Swappable for Postgres later without touching the API surface.
6. **UX target: Vercel, not vSphere.** Two top-level concepts only — **Projects** (a compose file or a single image) and **Deployments** (the resulting VM(s)). No cluster/node/pool concepts in v1.
7. **Single-tenant, permanently.** Porter is one operator's tool for their own host(s) — not a foundation for serving multiple untrusted customers from one instance. This isn't a v1.0.0 shortcut to be revisited; it's a fixed design stance. See `OSS_AND_SAAS_STRATEGY.md`.

## 2. Components

### 2.1 Control API (`backend/internal/api`)
Stateless HTTP server (Go, `chi` router). Owns:
- Project/VM CRUD endpoints (see `API_SPEC.md`)
- Compose file upload + parse trigger
- Domain attach/verify endpoints
- Server-Sent Events (SSE) stream for live VM state, domain status, and traffic events, used by the dashboard to update without polling
- Auth: v1.0.0 ships with a single static API token (`PORTER_API_TOKEN` env var) checked via bearer header. Multi-user auth is out of scope for v1 (see `ROADMAP.md`).

### 2.2 VM Manager (`backend/internal/vmmanager`)
A thin wrapper around a `containerd` client (`github.com/containerd/containerd`), talking to a containerd instance configured with the `firecracker-containerd` runtime shim. Responsibilities:
- Build the OCI runtime spec / task options for each VM (vcpu/mem sizing → `firecracker-containerd`'s `proto.FirecrackerMachineConfiguration`, kernel path, network config passed as kernel boot args to the shim)
- Boot lifecycle: `client.Pull()` (if not cached) → `container.NewTask()` → `task.Start()` → track task/container IDs in the store → graceful stop (`task.Kill(SIGTERM)`) → hard stop (`task.Kill(SIGKILL)`) fallback → `task.Delete()`/`container.Delete()` teardown
- Subscribes to containerd's task exit events (`client.Subscribe()` on the events service) to detect unexpected exits and flip state to `stopped`/`failed` in the store
- **Not** responsible for pulling or flattening images, running an in-VM init, or wiring the jailer — `firecracker-containerd` owns all of that. VM Manager only talks to containerd's task API.

### 2.3 Compose Mapper (`backend/internal/compose`)
- Parses a (deliberately constrained) subset of Compose v3 syntax
- Translates each `services.<name>` entry into exactly one `ComposeService` → one containerd task → one VM
- Resolves `depends_on` into a boot order via topological sort; refuses (with a clear error) on circular dependencies
- Full mapping rules: `COMPOSE_MAPPING.md`

### 2.4 Network Manager (`backend/internal/netmgr`)
- Allocates one Linux `tap` device per VM. `firecracker-containerd`'s CNI-driven network setup (via its `tc-redirect-tap` plugin) attaches the tap to the microVM; Porter's job is to provide the CNI network config per project, not to hand-roll tap creation itself
- Allocates one `/24` bridge subnet per **project** (not per VM) from a configurable base range (default `10.42.0.0/16`, sliced into per-project `/24`s), so services within one compose project share an L2 segment and can reach each other by IP
- Cross-project isolation: each project's bridge is a separate Linux bridge with no route between them unless explicitly peered (not in v1 scope)
- Deterministic MAC generation from VM ID, so the same VM ID always gets the same MAC across restarts

### 2.5 Porter Gateway (`backend/internal/gateway`)
The single HTTP front door. One process, three jobs, because all three are naturally the same layer — every request already passes through here:

- **Routing:** by `Host` header (subdomain or custom domain) or path prefix, resolved against the live VM→IP mapping in the store, so a VM restart (new IP) never requires manual route updates
- **Domains:** owns the Vercel-style subdomain model — stable + preview subdomains under the operator's wildcard, plus verified custom domains. Full detail: `DOMAINS_AND_TRAFFIC.md` §1
- **Traffic log:** records a bounded in-memory ring buffer of recent requests per VM (method, path, status, latency, remote IP — metadata only, never bodies), surfaced live in the dashboard via SSE. Full detail: `DOMAINS_AND_TRAFFIC.md` §2
- **Identity:** injects `X-Porter-VM-ID` / `X-Porter-Project-ID` headers on every proxied request, so backend services can trust routing identity without re-implementing auth

### 2.6 SSH access layer
`firecracker-containerd` boots a minimal in-VM agent (its own `agent` binary, statically linked, running as the guest's init) that speaks a gRPC-over-vsock protocol back to the shim for task lifecycle (start/stop/exec/IO) — this is what replaces a custom guest-init entirely. Porter builds interactive SSH on top of that instead of a hand-rolled network sshd:
- **Exec-based sessions (default):** the SSH Gateway authenticates the operator, then calls containerd's `task.Exec()` over the existing vsock control channel — with a shell (`/bin/sh` or the image's shell) — and streams stdio back over SSH. No SSH server runs inside the guest at all, no extra port, no baked-in dropbear.
- Because this rides the same vsock channel `firecracker-containerd` already uses for task control, there's no dependency on the guest's network stack being up or on any SSH daemon shipping inside the source image — it works for a bare `redis:7` with zero image customization, same guarantee as before, achieved differently.
- Full detail: `SSH_ACCESS.md`

### 2.7 SSH Gateway (`backend/internal/sshgw`)
- Single SSH server the operator connects to (`ssh <vm-name>@gateway.<base-domain>` or via `porter ssh <name>` which wraps this)
- Terminates the operator's SSH session normally (cert-based or static-key auth, same as before — see `SSH_ACCESS.md`), but instead of proxying TCP to an in-guest sshd, it translates the session into a containerd `task.Exec()` call against that VM's task and pipes stdio both ways
- On connect, the gateway looks up the target VM's containerd task ID from the store; the operator never sees or needs an internal IP, because this path never touches the guest's network at all
- No inbound SSH ports are exposed on VM bridges; there's nothing listening there to expose
- Full detail: `SSH_ACCESS.md`

### 2.8 State Store (`backend/internal/store`)
- In-memory maps guarded by a `sync.RWMutex`, persisted to a single JSON file on every write (`state.json`)
- Holds `VM` (now including containerd `ContainerID`/`TaskID`), `Project`, and `Domain` records (see `pkg/types`)
- v1.0.0 explicitly does not use a real database — acceptable for single-host, single-operator scale; flagged in `ROADMAP.md` as the first thing to swap for multi-host v2

### 2.9 Dashboard (`frontend/`)
Next.js + React, Vercel-inspired IA. Full detail in `UI_SPEC.md`. Talks only to the Control API (REST + SSE), never touches the host, containerd, or Firecracker directly.

## 3. Data flow: booting a single image

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

## 4. Data flow: booting a compose project

```
User uploads docker-compose.yml
  → POST /projects/compose {name, compose_yaml}
  → Compose Mapper.Parse() → ordered []ComposeService (topo-sorted on depends_on)
  → API creates Project record, returns 202
  → API kicks off async, sequential per service (respecting boot order):
       for each service in order:
         1. Build VM record (image, env, resources from service)
         2. VM Manager.CreateAndStart() — same containerd task path as
            single-image boot, env vars passed as task Process.Env
         3. Gateway registers stable + preview subdomains for any declared ports
         4. Register SSH gateway entry: "<project>-<service>" → VM ID (→ task ID)
  → All services share one project bridge subnet, so service A can reach
    service B by its VM's assigned IP or via ${PORTER_<SERVICE>_IP} env
    injection — see COMPOSE_MAPPING.md §5
```

## 5. Failure handling

| Failure | Behavior |
|---|---|
| Image pull fails (containerd `client.Pull()` error) | VM → `failed`, containerd's error surfaced verbatim in the UI; no partial task left running |
| `task.Start()` fails (jailer/shim error, e.g. cgroup or chroot setup failure) | VM → `failed` before any VMM process exists; error surfaced from the shim's task-create response |
| Firecracker process / task dies unexpectedly | Containerd event subscription (`TaskExit`) fires, flips state to `stopped`, dashboard shows "crashed" badge if it wasn't a user-initiated stop |
| Compose service fails mid-boot | Boot sequence halts; already-booted services in that project stay running; project shows partial state (`degraded`) so the user can retry just the failed service |
| Tap/CNI network setup fails | VM → `failed` before `task.Start()` is called, no leftover task or VMM process |
| Host reboot | v1.0.0 does **not** auto-resume VMs on host boot; this is explicitly deferred (see `ROADMAP.md`) — all VMs simply show as `stopped` on next dashboard load, matching containerd's actual task state after a restart |

## 6. Security notes (v1.0.0 scope)

- Single static API token — treat the Control API as trusted-network-only; do not expose port 8080 to the public internet without a reverse proxy adding real auth
- Porter has no multi-tenant isolation model and never will (see `OSS_AND_SAAS_STRATEGY.md`) — it assumes everyone who can reach the API token is fully trusted with everything on the host. Do not run Porter in any setup where that assumption doesn't hold.
- SSH gateway is the *only* SSH-facing thing meant to be internet-facing; since v1.0.0's SSH path is `task.Exec()` over vsock (§2.6), there is no guest-side network SSH daemon to expose or misconfigure at all
- Guest images run whatever the source image runs — no image scanning/policy engine in v1
- The Firecracker jailer (`chroot` + cgroups + seccomp) is configured and invoked by `firecracker-containerd`'s shim itself — this is the documented, recommended `firecracker-containerd` deployment mode, and Porter's runtime config simply turns it on rather than wiring it by hand

## 7. Why not just use plain containers?

This project deliberately trades container-level density for **VM-grade isolation with near-container boot times**, because Firecracker's whole reason to exist is exactly that trade-off (it's what AWS Lambda/Fargate run on). If the goal were maximum density and you trusted all workloads equally, plain containerd with the default `runc` shim would be the simpler answer. Porter configures containerd with the `firecracker-containerd` shim specifically for cases where per-service kernel isolation actually matters — while still getting containerd's mature image pull, snapshot, and task-lifecycle machinery for free instead of rebuilding it.

## 8. What this replaced, and why it's a real improvement

An earlier draft of this document specified a fully custom pipeline: a hand-rolled OCI image puller/flattener, a from-scratch `guest-init` PID-1 binary (mount filesystems, bring up networking, reap zombies, run an embedded sshd), and manual jailer wiring. All three were flagged as the riskiest unbuilt parts of the whole plan — `guest-init` in particular is genuine, unforgiving systems programming with no partial-credit failure mode (get PID 1 wrong and nothing boots).

Standardizing on `firecracker-containerd` deletes all three at once:
- The custom OCI puller/flattener → containerd's existing pull + snapshotter
- The custom `guest-init` agent → `firecracker-containerd`'s own in-VM agent
- Manual jailer wiring → the shim's own jailer integration, which is its documented default mode

What's left for Porter to actually build is genuinely smaller: a control API, a compose mapper, a gateway, an SSH gateway that rides the existing vsock task-exec channel, and a thin client over containerd's task API. The remaining open question worth resourcing early is the **kernel image** — `firecracker-containerd` still needs a `vmlinux` built/configured for Firecracker guest boot, shared across all VMs; that's still a real one-time setup step and worth a smoke-tested "does a task actually boot and reach `running`" checklist before layering on domains/SSH/compose.
