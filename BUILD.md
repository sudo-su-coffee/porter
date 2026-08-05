# Porter — Build Guide (v1.0.0)

Dashboard UI spec, host deployment guide, roadmap/scope, and the OSS/future-SaaS strategy. For system design and API reference, see [`ARCHITECTURE.md`](./ARCHITECTURE.md). For the short pitch and quickstart, see [`README.md`](./README.md).

## Contents

1. Dashboard UI Spec
2. Deployment Guide
3. Roadmap & Scope
4. OSS & Future SaaS Strategy

---

## 1. Dashboard UI Spec

Design language: Vercel-like. Minimal chrome, dark-mode-first, monospace for anything technical (IPs, IDs, commands, URLs), generous whitespace. Two nouns only: **Projects** and **Deployments** (a Deployment is a VM).

### 1.1 Screens

#### 1.1.1 Projects list (`/`)
- Grid of project cards, most-recently-active first
- Each card: project name, source badge (`compose` or `single-image`), status summary (`3/3 running` / `2/3 running, 1 failed` / `stopped`), a rolled-up health summary across all services' replicas (e.g. `7/8 healthy`), stable-domain link shown directly on the card (click to open the live URL)
- Top-right: **New Project** button → opens the create flow (§1.1.3, New Project flow)
- Empty state: "Deploy your first image or compose file" CTA

#### 1.1.2 Project detail (`/projects/[id]`)
- Header: project name, source, created date, **Delete Project** (destructive, confirm modal), **Redeploy**
- Service list (card-per-service):
  - Service name, image ref, **replica count** (e.g. `3 replicas`) with a **health summary** (e.g. `2/3 healthy`, amber if any replica is `unhealthy` or `checking`), stable URL (monospace, click-to-copy, click-to-open), preview URL for the current deploy shown smaller underneath
  - Per-card actions: **Scale** (opens a modal to set a new replica count, calls `PATCH .../scale`), **Stop**, **Restart**, **Delete**, **SSH** (copies the `porter ssh <name>` command, targets replica 0 by default), **Domains**, **Traffic**, **Logs**
  - Expanding a service card lists its individual replicas (`api-0`, `api-1`, `api-2`), each with its own state badge, health badge, and per-replica SSH/logs actions — useful for spotting one bad replica in an otherwise-healthy pool
  - If any replica is `failed`: error message shown inline in red, expandable, scoped to that specific replica
- Network panel (collapsed by default): project's bridge subnet + a simple reachability diagram derived from `depends_on` and the embedded DNS names (`db.my-app.local`, etc.) each service resolves to
- Compose source (collapsed): read-only viewer of the original `compose_yaml`, with parse **warnings** shown inline

#### 1.1.3 New Project flow (`/projects/new`)

**Tab A — Single Image**
- Fields: Name, Image ref, vCPUs (stepper, default 1), Memory (stepper in MiB, default 256), Environment variables (repeatable rows), Ports (repeatable rows)
- **Deploy** button → `POST /vms`, redirects to the new VM's detail view with live boot progress

**Tab B — docker-compose.yml**
- Drag-and-drop or paste-in YAML editor (syntax highlighted)
- Live client-side pre-validation as you type
- **Deploy** button → `POST /projects/compose`, redirects to the new project detail page
- On `400`: exact error shown inline, editor highlights the offending line where identifiable
- On `202` with warnings: redirect happens, dismissible banner lists warnings once

#### 1.1.4 VM (single deployment) detail (`/vms/[id]`)
Same layout as a service card, full-page — used for standalone (non-compose) VMs. Includes Stop/Restart/Delete/SSH/Domains/Traffic/Logs, plus resource stats (vCPU/mem allocation, uptime).

#### 1.1.5 Domains tab
- List of all domains for the service: **Stable** (badge: "Live"), **Preview** (badge: "This deploy"), and any **Custom** domains, each with a status pill (`pending` amber / `verified` green)
- A small note under Stable and Custom domains clarifies they point at the service's **load-balanced pool**, not a single VM — e.g. "Routes to 3 healthy replicas (round-robin)" — so it's clear scaling a service doesn't require any domain reconfiguration
- **Add domain** button opens a form (just the domain string); on submit, shows the exact CNAME record to create, copy-to-clipboard
- Pending domains show a "waiting for DNS..." state that flips live via the `domain.status` SSE event
- **Rollback** button appears here when a service has more than one deploy: lets the operator repoint the stable subdomain back to a previous (still-running) deploy's pool in one click

#### 1.1.6 Traffic tab
- Live-updating request table fed by `traffic.request` SSE events plus an initial fetch on open
- Columns: time, method, path, status (2xx green / 4xx amber / 5xx red), duration, remote IP
- Small requests/sec sparkline above the table, computed client-side
- Filter row: status-range chips (2xx/4xx/5xx/all), method dropdown, path substring search — all client-side against the loaded buffer

#### 1.1.7 Logs drawer
- Slides in from the right, doesn't navigate away
- Streams via `GET /vms/{id}/logs?follow=true`, auto-scrolls unless the user has scrolled up
- Simple text search/filter box, client-side only

#### 1.1.8 SSH modal
Not a terminal-in-browser in v1.0.0 (roadmap item) — instead:
- Shows the exact `porter ssh <name>` command with a copy button
- Shows the raw `ssh <name>@gateway... -p 2222` form too, for Option B static-key auth
- If the VM isn't `running`, explains why SSH isn't available instead of showing a dead command

### 1.2 Status badge colors (consistent everywhere)

| State | Color | Label |
|---|---|---|
| `pending` | gray, pulsing dot | Pending |
| `booting` | amber, pulsing dot | Booting |
| `running` | green, solid dot | Running |
| `stopping` | amber, pulsing dot | Stopping |
| `stopped` | gray, solid dot | Stopped |
| `failed` | red, solid dot | Failed |

### 1.3 Real-time updates

- Dashboard subscribes to `GET /events` (SSE) on mount; state badges, domain status, replica health, and traffic all update live with no manual refresh
- Project creation flow shows a step-by-step progress list while booting a compose project (`db ✓ → api ⋯ → worker ⋯`), driven by `project.progress` events
- Redeploys show a live cutover indicator on the Domains tab: "new deploy running → verifying → stable URL repointed" — makes the promotion moment visible instead of silent
- Scaling a service shows a live progress indicator on its card ("scaling 3 → 5... 4/5 healthy"), driven by `pool.updated` events, until `healthy` catches up to `desired`
- A replica flipping `unhealthy` updates its badge immediately via `replica.health`; if auto-healing kicks in, the card briefly shows a "replacing api-2..." state until the new replica reports healthy

### 1.4 Non-goals for v1.0.0 UI

- No drag-and-drop visual network topology editor
- No in-browser terminal/SSH (copy-command only, §1.1.8, SSH modal)
- No multi-user roles/permissions UI
- No billing/usage/cost estimation views
- No dark/light theme toggle — dark-mode only for v1

### 1.5 Component inventory

- `ProjectCard`, `ServiceCard` (shared status-badge + action-row component)
- `StatusBadge`
- `NewProjectTabs` (Single Image / Compose)
- `ComposeEditor` (YAML syntax highlight + inline error annotation)
- `DomainsPanel` (stable/preview/custom list + add-domain form + status pills + rollback control)
- `ScaleModal` (replica count input, calls `PATCH .../scale`)
- `ReplicaList` (per-replica state/health badges, expandable under a `ServiceCard`)
- `TrafficTable` (live-updating, client-side filterable, requests/sec sparkline)
- `LogsDrawer`
- `SSHModal`
- `NetworkPanel` (simple reachability diagram)
- `EventStream` (SSE hook, `usePorterEvents()`)
## 2. Deployment Guide

### Host requirements

| Requirement | Why |
|---|---|
| Linux with KVM (`/dev/kvm` present, readable/writable) | Firecracker requires KVM — no software-emulation fallback |
| `containerd` ≥ 1.7, with the [`firecracker-containerd`](https://github.com/firecracker-microvm/firecracker-containerd) runtime shim installed and registered (`runtime-type: "aws.firecracker"`) | This is Porter's execution engine — VM Manager talks to it over containerd's task API instead of driving Firecracker directly |
| `devmapper` snapshotter configured on containerd (thin-pool set up ahead of time) | `firecracker-containerd` needs a block-device-backed snapshotter to hand each VM a root block device; the default overlayfs snapshotter doesn't work here |
| `iproute2` (`ip` command) + a CNI plugin set including `tc-redirect-tap` | Tap device wiring between the host bridge and each microVM — installed as part of `firecracker-containerd`'s own setup |
| Root or equivalent capabilities (`CAP_NET_ADMIN`, `CAP_SYS_ADMIN`) for the `containerd`/shim processes | Tap/bridge creation, jailer chroot + cgroup setup — the shim does this, Porter's Control API process itself does not need these capabilities |
| A Linux kernel image built/configured for Firecracker guest boot (`vmlinux`, uncompressed) | Shared across all VMs — one kernel, referenced in `firecracker-containerd`'s runtime config |
| Outbound network access to your OCI registry (Docker Hub, GHCR, etc.) | `containerd` pulls images directly via the registry HTTP API |
| A domain you control, with the ability to add a wildcard DNS record | Needed for the auto stable/preview subdomain model — see ARCHITECTURE.md, Section 4 (Domains & Traffic Log) |
| Disk space for containerd's content store + devmapper thin-pool | Each unique image gets its own cached snapshot; no Porter-level cap in v1.0.0 (see Section 3 (Roadmap & Scope)) |

### Embedded services — nothing else to install

Porter's load balancer and DNS server are both built into the main `porter` binary, in pure Go — there's no separate CoreDNS, Envoy, Nginx, or HAProxy to install, configure, or keep in sync. The L7 load-balancing pool lives inside the Porter Gateway process (`PORTER_GATEWAY_ADDR`), and the embedded DNS server binds to each project's bridge gateway IP on port 53 automatically as projects are created — no extra port or config needed beyond what's already listed above.

### Recommended host sizing (starting point, not a hard rule)

- Reserve host resources separately from what you hand out to VMs — e.g. on a 16 vCPU / 32GB host, budget ~2 vCPU / 4GB for the Control API + gateway + SSH gateway + `containerd`/shim overhead, leaving the rest for VM allocation
- Firecracker's own overhead per microVM is small, so density is mostly bound by however much vCPU/mem you assign per VM

### Install (target v1.0.0 flow)

```bash
# 0. install & configure containerd + the firecracker-containerd shim first
#    (devmapper thin-pool, runtime registration) — see firecracker-containerd's
#    own getting-started docs; Porter assumes this is already working
#    (`ctr run --runtime aws.firecracker ...` should boot a test VM)

# one binary bundles: Control API, gateway, SSH gateway
curl -fsSL https://get.porter.dev | sh

# verify KVM access
ls -l /dev/kvm

# set the shared kernel image Porter should tell firecracker-containerd to use (one-time)
porter kernel set /path/to/vmlinux

# point your domain's wildcard DNS at this host, then tell Porter:
#   *.example.com  A  <this-host-ip>
porter domain set-base example.com

# set required env / config
export PORTER_API_TOKEN=$(openssl rand -hex 32)
export PORTER_DATA_DIR=/var/lib/porter     # state.json, rootfs cache, run sockets
export PORTER_API_ADDR=:8080
export PORTER_GATEWAY_ADDR=:8081
export PORTER_SSH_GATEWAY_ADDR=:2222

# run as a systemd service (unit shipped under deploy/systemd/)
sudo systemctl enable --now porter
```

### Config reference

| Env var | Default | Purpose |
|---|---|---|
| `PORTER_API_TOKEN` | *(required, no default)* | Bearer token for all Control API + dashboard + CLI auth |
| `PORTER_DATA_DIR` | `/var/lib/porter` | Root for state file, rootfs cache, VM run sockets |
| `PORTER_KERNEL_PATH` | `$PORTER_DATA_DIR/vmlinux` | Shared kernel image path |
| `PORTER_API_ADDR` | `:8080` | Control API listen address |
| `PORTER_GATEWAY_ADDR` | `:8081` | HTTP gateway (routing + domains + traffic) listen address |
| `PORTER_SSH_GATEWAY_ADDR` | `:2222` | SSH gateway listen address |
| `PORTER_BRIDGE_BASE` | `10.42.0.0/16` | Base range sliced into per-project `/24` subnets |
| `PORTER_SSH_CERT_TTL` | `10m` | Lifetime of gateway-issued SSH certificates |
| `PORTER_BASE_DOMAIN` | *(required, no default)* | Wildcard base domain for stable/preview subdomains — set via `porter domain set-base` |
| `PORTER_DOMAIN_VERIFY_INTERVAL` | `30s` | How often pending custom-domain CNAMEs are re-checked |
| `PORTER_TRAFFIC_LOG_SIZE` | `2000` | Max requests kept per VM in the in-memory traffic ring buffer |
| `PORTER_HEALTHCHECK_DEFAULT_INTERVAL` | `30s` | Used when a service's `healthcheck:` omits `interval` |
| `PORTER_SCALE_DOWN_DRAIN_TIMEOUT` | `10s` | How long the Gateway waits for in-flight requests to finish on a replica being removed (via scale-down or health-triggered replacement) before force-killing its task |

### Firewall / exposure guidance

- Expose only what you mean to expose publicly:
  - `PORTER_GATEWAY_ADDR` (8081) — fine to expose publicly, it's the HTTP front door
  - `PORTER_SSH_GATEWAY_ADDR` (2222) — expose if you want remote SSH access; otherwise keep to trusted networks/VPN
  - `PORTER_API_ADDR` (8080) — **do not expose publicly** without adding a real auth layer in front — v1.0.0's single static token is not designed to withstand public internet exposure on its own

### Upgrading

v1.0.0 has no migration story yet since it's the first version — `state.json` schema stability across versions will be documented starting at v1.1.0.

### Uninstall / cleanup

```bash
sudo systemctl disable --now porter
# stop all VMs first via the API/CLI, or:
sudo pkill firecracker
sudo rm -rf /var/lib/porter
# remove any leftover tap devices
ip -o link show | grep tap- | awk -F': ' '{print $2}' | xargs -n1 sudo ip link del
```
## 3. Roadmap & Scope

Porter is self-hosted, open source (MIT), and single-tenant by permanent design — see Section 4 (OSS & Future SaaS Strategy) for the reasoning. Everything below is roadmap for the OSS core itself, not a hosted product.

### v1.0.0 scope (this doc set describes exactly this)

- Single host, single kernel image shared across all VMs
- One microVM per replica, one or more replicas per compose service (no in-guest Docker)
- Image pull, snapshotting, and VM boot handled by `containerd` + the `firecracker-containerd` shim (no custom OCI puller or guest-init)
- **Horizontal scaling** — `deploy.replicas: N`, plus a `PATCH .../scale` endpoint to change replica count live
- **Built-in L7 load balancing** — the Gateway round-robins each service's stable/preview domains across its healthy replicas, no external LB needed
- **Health checks** — HTTP and TCP probes from `healthcheck:`, with `interval`/`timeout`/`retries`/`start_period`
- **Auto-healing** — `restart: always`/`on-failure` kills and replaces unhealthy replicas automatically; unhealthy replicas are drained from the load-balancer pool immediately regardless of restart policy
- **Embedded DNS** — `<service>.<project>.local` (pool, round-robin) and `<service>-<n>.<project>.local` (pinned replica), no external DNS server needed
- Vercel-style dashboard: Projects → Deployments
- Vercel-style domain model: wildcard base domain, stable + preview subdomains per deploy, custom domain attach via CNAME
- In-memory, dashboard-only traffic log per VM
- SSH gateway, cert-based (10-min ephemeral) + static-key auth, replica-addressable (`porter ssh api-2`)
- JSON-file state store
- Single static API token auth
- CLI (`porter`) as a thin REST client mirroring the dashboard 1:1

### Explicitly deferred (not v1.0.0)

| Feature | Why deferred | Target |
|---|---|---|
| Multi-host scheduling | v1 is single-host by design; multi-host needs a real scheduler + distributed state store. Note: multi-host ≠ multi-tenant — this would still serve one trusted operator across several machines, not multiple customers. | v2 (OSS, if pursued) |
| Health-check-gated `depends_on` (a dependency must be `healthy`, not just `running`, before dependents boot) | v1.0.0 ships healthchecks themselves and boot-order sequencing, but doesn't yet wire the two together — `depends_on` still only waits for `running`. This stronger form is a v1.1 refinement, not a v1.0.0 gap in the underlying mechanism (which now exists). | v1.1 |
| Volume / bind-mount support (virtio-fs) | Needs a real design for persistence across VM restarts and host paths | v1.1–v2 |
| Auto-scaling (HPA-style, based on load/CPU/traffic) | v1.0.0 ships manual scaling (`PATCH .../scale`) only — no metric-driven automatic replica adjustment | v1.2 |
| Multi-region deployment (Fly.io-style) | Out of scope while Porter remains single-host; would follow multi-host scheduling | v2+ |
| Non-HTTP port forwarding through the gateway (raw TCP/UDP) | v1.0.0 gateway is HTTP-only | v1.1 |
| In-browser terminal (SSH-in-UI) | Copy-command UX ships in v1.0.0; xterm.js + WebSocket bridge is more surface area | v1.1 |
| Multi-user auth / RBAC | **Not planned, ever, for the OSS core** — Porter is single-tenant by permanent design, not by v1 limitation. See Section 4 (OSS & Future SaaS Strategy). A future hosted product would handle multi-tenancy in a separate closed-source layer, without this core changing. | Not on the OSS roadmap |
| Host-reboot VM auto-resume | Needs persisted boot-on-start config + ordering | v1.1 |
| Image scanning / policy enforcement | No vulnerability/policy gate on pulled images in v1 | v2 |
| SSH session recording | Connection metadata logging only in v1.0.0 | v1.1+ |
| Cross-project networking / peering | Projects are isolated bridges by design in v1 | v2 |
| Automatic TLS (Let's Encrypt) for wildcard + custom domains | v1.0.0 ships domain routing but not cert provisioning | v1.1 |
| Persisted / exportable traffic logs (log drains) | v1.0.0 traffic log is an in-memory ring buffer, dashboard-only, cleared on gateway restart | v1.1 |
| Aggregate/historical traffic analytics (daily/weekly trends) | v1.0.0 traffic view is "what's happening right now," per-VM only | v1.2+ |
| `build:` support in compose (building from Dockerfile) | Would require a build pipeline separate from containerd's pull-only path | v2, possibly never |
| High-availability gateway / SSH gateway | Both are single processes in v1 | v2 |

### Known limitations to document clearly at launch

1. containerd's snapshotter (devmapper, block-device backed — required for `firecracker-containerd`) needs its thin-pool sized and configured correctly on the host, or task creation fails; call this out prominently in Section 2 (Deployment Guide)'s host requirements.
2. Snapshot/content-store cache has no Porter-level size cap / eviction policy in v1.0.0 beyond whatever `containerd`'s own garbage collection does — monitor disk usage manually.
3. `depends_on` readiness is "VM/task is running," not "healthcheck passing" — even though healthchecks exist in v1.0.0, they aren't yet wired into boot-order gating (see the deferred table above), so services must still handle their own dependency-not-ready-yet retry logic.
4. No image content scanning — only pull images you trust, same caveat as running any `docker pull` today.
5. No TLS termination built in — front Porter with your own cert layer until v1.1.

### Suggested version sequence

- **v1.0.0** — everything in this doc set, including replicas, L7 load balancing, health checks/auto-healing, and embedded DNS
- **v1.1.0** — health-check-gated boot ordering, in-browser terminal, raw TCP/UDP forwarding, automatic TLS, persisted/exportable traffic logs, host-reboot VM auto-resume
- **v1.2.0** — virtio-fs volumes, auto-scaling (HPA-style), aggregate traffic analytics
- **v2.0.0** — multi-host scheduling, multi-region, real database backend, multi-user auth/RBAC, image policy engine
## 4. OSS & Future SaaS Strategy

### Where this stands today

Porter v1.0.0 ships as **self-hosted, open source, MIT licensed.** This is not a "SaaS with a self-hosted mode bolted on" — it's the other way around. The self-hosted core is the actual product being built and shipped first. A hosted/SaaS offering is a possibility to keep the door open for, not a plan being executed in parallel.

This doc exists so that architectural decisions made now don't accidentally foreclose a hosted option later, without letting that future possibility complicate or slow down v1.0.0 itself.

### License: MIT

Chosen for maximum adoption with the fewest questions asked. No copyleft obligations for self-hosters, no chilling effect on companies evaluating it internally, no "will my legal team allow this" friction. This is the standard choice for infra tools that want to be trusted defaults (Traefik-style, not a research project).

**What MIT means practically here:** anyone can fork Porter and run a competing hosted version of it. That's an accepted tradeoff of going MIT over AGPL. The bet is that adoption and ecosystem trust from a fully permissive license outweigh that risk at this stage — same bet projects like Supabase, PostHog (core), and Traefik made.

### Single-tenant by design, not by accident

The self-hosted OSS core stays single-tenant **forever** — this is a permanent architectural stance, not a v1.0.0 shortcut to be lifted later. Concretely, this means:

- One operator, one API token, one trust boundary — this does not change across versions
- No per-tenant resource quotas, billing hooks, or isolation-between-customers logic will be added to the OSS core
- The JSON-file state store, single static token auth, and every other "good enough for one operator" decision documented across ARCHITECTURE.md, Section 1 (Architecture) and Section 2 (Deployment Guide) are permanent characteristics of the self-hosted product, not placeholders

**Why lock this in now instead of leaving it open:** multi-tenancy is not a feature you add — it's a set of assumptions (auth model, data isolation, resource accounting, billing) that has to be designed in from the start or retrofitted at real cost. Deciding now that OSS stays single-tenant means every future OSS contribution and every doc in this set can be written with a clear, stable audience in mind: one team, one host, full trust. That clarity is worth more to the OSS project's health than keeping the door open to a multi-tenant core nobody's actually building yet.

### If a hosted product happens later

It would be a **separate, closed-source product** built *around* the OSS core — not a fork or a mode-flag inside it. Roughly:

```
┌─────────────────────────────────────────┐
│   Hosted control plane (closed source)    │
│   - multi-tenant auth, billing, quotas    │
│   - orchestrates many isolated Porter      │
│     instances, one per customer            │
└───────────────┬───────────────────────┘
                │  (each customer gets their own)
┌───────────────▼───────────────────────┐
│  Porter core (OSS, MIT, single-tenant)    │
│  — exactly what's in this doc set          │
└───────────────────────────────────────────┘
```

Each customer effectively gets their own isolated single-tenant Porter instance underneath, managed by a hosted control plane that itself stays proprietary. This means:

- The OSS core never needs multi-tenant auth, billing, or quota code — that complexity lives entirely in the hosted layer, which can iterate fast without dragging the OSS project's stability along with it
- Self-hosters and hosted customers run literally the same core, so bugs/improvements benefit both, and self-hosters are never running a "lesser" version
- If the hosted product never happens, nothing about the OSS project needs to be unwound or apologized for — it was never contorted to prepare for it

### What NOT to build into v1.0.0 because of this

To keep the OSS core honest to "single-tenant forever," explicitly avoid, even as "just in case" scaffolding:
- Tenant IDs on any data model
- Per-resource ownership/ACL fields beyond the single API token
- Billing/usage-metering hooks of any kind
- Config flags like `PORTER_MULTI_TENANT=true` that gesture at a mode that doesn't exist

If any of these show up in a PR or a future doc revision, that's a signal the SaaS conversation has become real enough to warrant its own separate closed-source repo — not a reason to compromise the OSS core's simplicity.

### What TO keep in mind, without building it

A short list of things worth *designing around* (costs nothing now, saves pain later) versus *building* (which we're explicitly not doing):

| Consideration | OSS v1.0.0 stance |
|---|---|
| Could the state store be swapped for Postgres later? | Yes — `store.Store` is already a small interface (see ARCHITECTURE.md, Section 1 (Architecture) §1.2.11); swapping the backend doesn't require touching the API layer. This helps self-hosters who outgrow JSON-file storage just as much as it would help a future hosted layer — not SaaS-specific. |
| Could the Control API run behind a different auth layer? | Yes — it already expects a bearer token and doesn't assume *how* that token was issued. A hosted control plane could issue tokens differently without the core caring. Not a change, just a property that already falls out of the design. |
| Should VM/Project IDs be globally unique (UUIDs) rather than sequential? | Already true (`uuid.NewString()` throughout) — happens to help multi-instance-anything later, but was chosen for ordinary good-engineering reasons, not SaaS prep. |

None of these are changes to make — they're just existing v1.0.0 decisions worth naming so it's clear the door isn't accidentally welded shut, even though nothing is being built toward it.

### Bottom line

Ship Porter v1.0.0 as a genuinely good, permissively-licensed, single-tenant self-hosted tool. Judge it on whether people self-host it and like it. A hosted product is a question to revisit *if and when* that happens — not a constraint on how v1.0.0 gets built today.
