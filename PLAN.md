# Porter — Idea, Roadmap & Plan

> **Honesty note (matches README.md):** this file is the *target state* and
> phased plan, not a claim about what's built today. Every feature below is
> tagged **[DONE]**, **[PARTIAL]**, or **[PLANNED]** against the current
> checked-in code, verified against `README.md`'s audited status. If this
> file and `README.md` ever disagree, `README.md` — which is re-verified
> against source on every revision — wins. Read both; don't scope work off
> this file alone.

---

## The Idea

Porter is a **self-hosted PaaS** — the Vercel/Fly.io model running on your own
metal — built on **Firecracker microVMs**.

Most "serverless" platforms give you two bad choices:

- **Containers** — fast and cheap, but every workload shares the host kernel.
  Weak isolation for anything multi-tenant or untrusted.
- **Classic VMs** — strong isolation, but slow boots, hundreds of MB of
  baseline overhead, and a large emulated device surface.

Firecracker is the third option: a real, hardware-isolated microVM that boots
in milliseconds, uses a few MB per instance, and exposes only the minimal
devices a Linux workload needs. It powers AWS Lambda + Fargate and Fly.io's
Fly Machines.

**Porter is the self-hosted control plane for that engine.** Instead of
spinning raw Firecracker processes yourself, you deploy **Docker/OCI images**
and Porter boots each one as a kernel-isolated microVM through containerd +
the `aws.firecracker` shim — image pull, snapshots, jailer, networking, and
the in-VM agent are all handled for you, for OCI-image boots. ("Golden" /
bare rootfs images are the one exception — see `README.md` § Architecture.)

### Why that matters

| | Containers | Classic VM | Porter (microVM) |
|---|---|---|---|
| Isolation | shared kernel | full VM | **real kernel isolation** |
| Cold start | ms | seconds | **sub-second** |
| Per-instance overhead | tiny | hundreds of MB | **few MB** |
| Multi-tenant-*trusted* workloads safe | ❌ | ✅ | ✅ |
| Runs Docker images | ✅ | ❌ | ✅ (OCI path only) |

> **Multi-tenancy caveat:** "multi-tenant safe" above refers to isolating
> *workloads* from each other, not running Porter itself as a multi-tenant
> SaaS control plane. Porter's control plane is single-tenant by permanent
> design — see § Security & Trust Model below and `README.md` § OSS &
> Future SaaS Strategy.

### The Fly.io model, self-hosted

Fly Machines = Firecracker microVMs you control with an API, a fast proxy,
and optional volumes. Porter is that model, self-hosted:

- **Deploy** a Docker/OCI image, or a Multi-Service App, as one or more
  microVMs. **[DONE, compose-YAML only]** A UI-native dashboard form for
  defining services without hand-writing YAML is **[PLANNED]** — see
  `README.md` § Planned / In Progress. Compose import will remain supported
  as a convenience path once the form ships, not be replaced by it.
- **Run** fast, isolated, high-density — many workloads per host. **[DONE]**
- **Manage** — create / stop / restart / delete, live logs, traffic,
  overview, from the dashboard or REST API. **[DONE]**
- **One binary control plane.** A single Go binary (`cmd/porter`) that embeds
  the built frontend and serves the API. **[DONE]** — but see § Runtime
  Dependencies below: "single binary" describes Porter's own code, not the
  full deployment (containerd, PostgreSQL, and optionally BuildKit run as
  separate processes/daemons Porter drives, the same way it already depends
  on containerd and `firecracker`).

---

## Component Status (against `internal/*`, verified in README.md)

| Capability | Backing package | Status |
|---|---|---|
| MicroVM lifecycle (OCI via containerd + `aws.firecracker`) | `internal/runtime` | **[DONE]** |
| MicroVM lifecycle (bare rootfs, direct Firecracker API socket) | `internal/runtime` | **[DONE]** |
| Compose-file multi-service parsing | `internal/compose` | **[DONE, constrained subset — no `build:`]** |
| UI-native multi-service definition (no YAML) | `frontend/` + API | **[PLANNED]** |
| Reverse proxy / traffic gateway (Host-header routing) | `internal/gateway` | **[DONE, HTTP only — no TLS termination]** |
| Health checks + auto-replace on failure | `internal/health` | **[DONE]** |
| `.local` service discovery | `internal/dns` | **[DONE]** |
| Authoritative DNS server for preview/production domains | `internal/dns/server.go` | **[DONE]** — UDP/TCP authoritative server (miekg/dns) for `*.baseDomain` |
| Automatic TLS via ACME | `internal/tls/autocert.go` | **[DONE]** — Let's Encrypt with on-disk cache + HTTP-01 |
| Persistent volumes (real device) | `internal/volumes` | **[DONE]** — real dir + sparse data.img; delete/usage hit disk (block-device mount not yet wired into VM boot) |
| Real host-reachable port mapping | `internal/gateway`, `types.Port.HostPort` | **[PLANNED]** — field exists, nothing binds it; compose parser also currently drops the host-side port (tracked bug, see README) |
| Git repository deploy (clone → Dockerfile → build → boot) | `internal/api` git build handlers | **[PARTIAL]** — real clone + Dockerfile detect + honest logs; BuildKit image build is the remaining step |
| SSH access via containerd `task.Exec` | `internal/sshgw` | **[DONE, OCI-image VMs only]** — disabled by default |
| Network allocation | `internal/net` (active) + `internal/netmgr` (unused, cosmetic) | **[DONE, but duplicated — consolidation planned]** |
| Analytics, web-vitals, redirects, crons, firewall, cache-purge routes | `internal/api` | **[DONE]** — real traffic/bandwidth/vitals rings, cron scheduler, gateway firewall enforcement |
| Per-user RBAC | `internal/api` (`requireProjectRole`) | **[DONE]** — per-user tokens + project_members/org_members PG roles |
| Metrics | `internal/metrics` collector | **[DONE]** — CPU/mem samples into metrics_samples table |
| `internal/vmmanager` (second VM lifecycle implementation) | — | **dead code, not imported, scheduled for removal** |

This table exists specifically so nobody scopes a sprint against a feature
that's actually a stub. When in doubt, grep the handler in
`backend/internal/api/handlers_impl.go` before promising a date.

---

## Security & Trust Model

Porter's security posture is intentionally simple, and that simplicity is
the whole point — read this section before deploying to anything but a
private lab box.

- **Single-tenant, permanently.** Anyone holding the API bearer token or
  admin credentials is fully trusted with the entire host. This is a fixed
  design stance (see `README.md` § OSS & Future SaaS Strategy), not a
  v1 shortcut to be "fixed" later. Do not point Porter's Control API at the
  public internet without a real auth/proxy layer in front of it — the
  single static bearer token is not designed to survive that exposure.
- **Workload isolation vs. control-plane isolation are different claims.**
  Firecracker + jailer gives each *workload* (replica) real kernel isolation
  from other workloads. It does not give the *operator* isolation from their
  own workloads, and it does not make the Porter control plane itself
  multi-tenant-safe.
- **Jailer configuration is the shim's responsibility, not Porter's.** The
  `aws.firecracker` containerd shim invokes the jailer per its own
  `/etc/containerd/firecracker-runtime.json`; Porter does not configure,
  audit, or verify jailer flags (seccomp profile, chroot, cgroup limits) at
  runtime. If you change that file, re-verify the jailer is actually being
  invoked as expected — Porter will not warn you if it silently isn't.
  **[PLANNED]**: a startup sanity check that fails loudly if the shim/jailer
  path looks misconfigured, rather than only failing on first VM boot.
- **No TLS termination on the gateway yet.** `internal/gateway` proxies HTTP
  only. Until ACME/DNS-01 ships (see table above), put Porter's gateway
  behind your own TLS-terminating reverse proxy (or a WireGuard tunnel) if
  it needs to face anything but a trusted LAN.
- **Domain verification is currently a no-op that always succeeds.**
  `handleVerifyDomain` returns `"verified"` unconditionally. Do not treat a
  "verified" domain status as proof of DNS ownership until the real
  authoritative DNS server and DNS-01 challenge flow ship — right now it's
  purely cosmetic.
- **SSH gateway has no port-forwarding/SFTP, and only reaches OCI-image
  VMs.** It's a `task.Exec` bridge with its own self-generated CA — treat
  the `[ssh]` listener as an admin/debug channel, not a general-purpose
  bastion, until it's hardened further.
- **State store durability.** PostgreSQL (`pgx`) is the source of truth for
  everything except two explicitly ephemeral in-memory rings (traffic log,
  per-VM log tail) — those are lost on restart by design, since they're
  high-write, low-durability-need data. Back up the Postgres instance; do
  not expect the log rings to survive a restart.
- **Secrets at rest.** `porter.toml` refuses to start without an explicit
  `api_token` and `admin.password` — there is no default that could leak via
  a stale example config. Secrets stored for compose `environment:` values
  and future Git-deploy credentials should be reviewed for at-rest
  encryption before this pipeline (below) ships; that is an open item, not
  a solved one.

---

## Git-to-VM Pipeline — Scoping Notes

This is the largest unbuilt piece of the platform and deserves its own
section rather than a single roadmap bullet.

```
Git push / webhook
  → clone repo (deploy key or GitHub App/PAT token, stored as a secret)
  → detect Dockerfile at repo root (or configurable path)
  → build via a standalone BuildKit daemon (buildkitd, its own gRPC API —
    not the Docker daemon; keeps the "no Docker daemon" stance intact)
  → export image straight into containerd's content store
    (namespace: porter) via BuildKit's containerd exporter — no registry
    round-trip required
  → boot through the existing, already-working internal/runtime OCI path
```

Why this is scoped separately: the boot step is *already solved* —
`internal/runtime` boots any OCI image today. This feature is really "add a
new image producer in front of a boot path that already works," which
narrows the real engineering surface to:

1. **Repo fetch + auth.** Shelling out to the `git` binary via `os/exec` is
   consistent with how Porter already spawns `firecracker` and `ip tuntap` —
   simpler than vendoring a partial Git implementation.
2. **Webhook receipt + signature verification** (`net/http` +
   `crypto/hmac`, no new dependency).
3. **BuildKit client integration** (`github.com/moby/buildkit/client`) —
   the one new significant dependency this feature needs.
4. **Build queueing** — a single Go channel-backed worker (or small pool)
   per host is sufficient at Porter's target scale; no external job queue.
5. **Build log streaming into the existing SSE/ring-buffer log
   infrastructure** — BuildKit's client streams progress natively.
6. **Image tagging by git SHA**, so one-click rollback (already a v1.0.0
   target) has a stable reference to roll back to.
7. **Failure states that need real handling, not stubs:** bad/missing
   Dockerfile, build timeout, disk pressure from cached layers, private-repo
   auth failure. Each needs a distinct, surfaced error — "build failed" with
   no detail is not acceptable given how failure handling works everywhere
   else in the runtime (see README.md § Failure handling for the bar this
   should meet).

**Sequencing recommendation:** ship the authoritative DNS server + preview
domains *before* Git-to-VM deploy. Git-triggered deploys are only genuinely
useful once there's a real place to route the resulting preview build —
otherwise you ship builds with nowhere good to land.

---

## v1.0.0 — The Stable Release

### Theme

**Production-ready core runtime, UI-native management for what's already
wired.** The Firecracker/containerd execution path is solid enough to build
real workloads on today. Several dashboard-facing and infra-facing features
below are still **[PLANNED]** — see the Component Status table above for the
authoritative per-feature status. Everything is UI-first (no CLI, by
design — see `README.md` for why).

### Deploy from

- OCI Image — **[DONE]**
- Docker Image — **[DONE]** (same OCI path)
- Multi-Service Apps — **[DONE via compose YAML]** · UI-native form — **[PLANNED]**
- Git Repository (auto-build via standalone BuildKit) — **[PLANNED]**, see scoping notes above
- Golden MicroVM Image — **[DONE]**

### Manage & Scale

- Projects, Services, and MicroVMs — **[DONE]**
- Environments (Production, Preview, Development) — **[PARTIAL]** — project/env modeling exists; preview-domain wiring depends on the DNS server (planned)
- Environment Variables (scoped per environment) — **[DONE]**
- Domains — **[PARTIAL]** — CRUD exists; verification is a stub (see § Security above); real authoritative DNS server is planned
- Persistent Volumes — **[PLANNED]** — DB row only today, see table above
- Networks — **[PARTIAL]** — functional but duplicated across two allocators pending consolidation
- Secrets — **[DONE]**, review at-rest encryption before Git-deploy credentials are added
- Export / Import of VMs — **[DONE]**

### Kubernetes-Parity Orchestration

- Horizontal Autoscaling (CPU/RAM/Traffic) — **[PLANNED]**
- Automatic Failover & Self-healing — **[DONE]** (healthcheck-driven replace)
- Zero-Downtime Rolling Updates — **[PLANNED]**
- Multi-node Scheduling & Cluster Balancing — **[PLANNED]**
- Service Mesh (built-in mTLS between services) — **[PLANNED]**
- Strict RBAC & Fine-grained Access Control — **[PLANNED]** — today is a single bearer token + single admin login, not per-user RBAC
- Sidecar VMs (init tasks, log shippers) — **[PLANNED]**

### Multi-Node & Cluster Infrastructure (Fly.io Parity)

- Multi-Server Clustering — **[PLANNED]**
- Distributed Networking — **[PLANNED]**
- WireGuard Private Networks (VM-to-VM backplane) — **[PLANNED]**
- Automatic Scale-to-Zero — **[PLANNED]**
- Distributed Persistent Volumes — **[PLANNED]**

### Vercel-Parity Developer Experience

- Instant Preview Deployments — **[PLANNED]** (depends on DNS server)
- Framework Auto-detection — **[PLANNED]**
- Streaming ISR / Partial Prerendering / Microfrontends — **[STUBBED]** (routes return empty JSON)
- Monorepo Support & Deployment Skew Protection — **[PLANNED]**
- Edge Caching & Purge API — **[STUBBED]**
- Firewall & WAF Rules — **[STUBBED]** (route exists, no subsystem)
- Cron Jobs / Scheduled Tasks — **[STUBBED]**
- Webhooks & Integrations — **[PLANNED]** (Git webhook is the first real one, see above)
- One-click Rollbacks and Instant Reverts — **[PLANNED]**, blocked on Git deploy's SHA-tagged images
- Automatic TLS (Let's Encrypt, DNS-01) — **[PLANNED]**, blocked on the DNS server

### Observe & Measure

- Logs (live ring + file tail) — **[DONE]**
- Analytics (Web Vitals, Usage, Bandwidth, Path tracking) — **[STUBBED]**
- Metrics (cold/hot start, boot time tracking) — **[PARTIAL]** — basic state transitions are logged; dedicated metrics subsystem not built
- Traffic and Events — **[DONE]** (SSE `/events`, traffic ring + store)

### Operate

- Browser Console & SSH (`task.Exec` bridge) — **[DONE]**, OCI-image VMs only, disabled by default
- REST API — **[DONE]**, 268 routes as of this revision — see README.md for the stubbed-vs-real split
- Dashboard — **[DONE]** for implemented capabilities; UI for stubbed backend features exists cosmetically in some views and should not be assumed functional

The API surface and deployment model for the **[DONE]** capabilities above
are considered stable. Anything marked **[PLANNED]** or **[STUBBED]** is not
covered by that stability claim and may change shape before it ships.

---

## Guiding Principles

- **MicroVM-first:** Firecracker is the execution engine; containers are an
  application-packaging format, not the isolation boundary.
- **OCI-native:** use OCI images and registries rather than inventing a new
  image ecosystem.
- **Developer-friendly:** UI-native forms and dashboard-first management to
  reduce operational complexity — and where the UI can't do something yet,
  say so in the UI rather than showing a form that quietly does nothing.
- **Pure Go control plane:** Porter's own code stays Go-native. External
  daemons it drives (containerd, `firecracker`, and — once Git deploy
  ships — `buildkitd`) are dependencies of the runtime, not violations of
  this principle, the same way containerd already is.
- **Single binary experience:** installation, upgrades, and operation of
  Porter's own code stay as simple as possible; this does not imply zero
  runtime dependencies (containerd + KVM + PostgreSQL are required today).
- **Say what's real.** Every status claim in this file and in `README.md`
  should be checkable against the source. A stubbed route is not a shipped
  feature. This principle is what the status tags above exist to enforce.