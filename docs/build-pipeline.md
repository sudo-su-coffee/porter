# Build pipeline: Dokploy / Coolify (BuildKit) → Porter MicroVM

> Distilled 2026-09-18 from Dokploy docs (build server, deployment flow, going-production,
> auto-deploy) + Coolify docs v4 (builds overview, Nixpacks/Railpack/Static/Dockerfile/
> Compose) + Nixpacks how-it-works + Docker BuildKit/Buildx docs.
> Rule: same build front, different runtime back. BuildKit builds; Firecracker runs.

## 1. What BuildKit is (both use it)

- Buildx (client) sends Dockerfile + args + cache/export opts to BuildKit (daemon).
  BuildKit solves the build graph concurrently: skips unused stages/files, parallelizes
  independent stages, incrementally transfers only changed context, pulls secrets/SSH/
  registry tokens on demand. Output → registry or client.
- `docker build` = wrapper around `docker buildx build` (subtle flag differences).

## 2. Dokploy (take the separation + triggers)

- App services (single container): clone → build (Nixpacks / Railpack / Heroku
  Buildpacks / Dockerfile) → image → container. Compose services: multi-container
  Compose/Stack definitions.
- **Build Server** (key pattern): remote SSH host that ONLY builds (Nixpacks, Docker,
  Railpack, Buildpacks installed; runs zero containers) → pushes to registry (Hub/GHCR)
  → deploy servers pull + run. Solves build-RAM/CPU freezing prod.
  Requires registry select (Cluster Settings → registry); `docker image prune` jobs.
- Going-production: in-repo builds OOM/freeze hosts → recommended CI/CD
  (GitHub Actions `build+push`, then `dokploy/dokploy-action` or
  `POST /api/application.deploy {applicationId}` w/ `x-api-key`, tag-matched).
  Webhook auto-deploy (branch-matched) or API trigger; zero-downtime deploys.

## 3. Coolify (take the pack matrix + cache discipline)

- Build Packs (per app): Nixpacks (auto-detect: plan → generated Dockerfile → BuildKit
  OCI image; `nixpacks.toml`, `NIXPACKS_*`, engines/`.nvmrc` pinning) / Railpack beta /
  Static (publish dir → Nginx image) / Dockerfile (exact control: base, packages, stages,
  user, CMD; build-args injection toggleable; BuildKit secrets for sensitive values) /
  Compose (per-service build-or-pull) / Docker Image (skip build).
- Builds on deploy server by default; build server isolates CPU/mem/disk (can't run
  resources, needs registry path). Build vars (new build on Dockerfile/commit/build-cmd/
  build-var change) vs runtime vars (reuse image). `SOURCE_COMMIT` excluded from build
  by default (cache reuse); `Include Source Commit` / `Force deploy (without cache)` /
  `Disable Build Cache` for exceptions. Pre-deploy runs in OLD container (fail stops
  deploy; skipped on first deploy); post-deploy runs in NEW (fail logged, deploy stays
  success). Health + domain verify after start.

## 4. Porter MicroVM mapping (what to build)

```
Git/webhook|API
  → detect (Nixpacks/Railpack/Dockerfile/Static/Compose — same as above)
  → BuildKit (local worker or remote build server via SSH; layer cache;
     secrets, SOURCE_COMMIT excluded; no-cache/force flags)
  → OCI image → registry/artifact store (immutable digest + SBOM/provenance/sign/scan)
  → GUEST TRANSFORM (the step Docker skips):
     unpack layers → rootfs.ext4 + vyoma-config.json init (CMD/ENTRYPOINT/ENV/WORKDIR)
     + kernel → vm image record (arch + rootfs/kernel sha256, exact bytes)
  → scheduler (Atlas placement: capacity sample − reservations − drafts, lock+recheck)
  → TAP//30 + limiters + vsock + volumes/secrets → Firecracker create/boot
  → health-gated rollout (Dokploy zero-downtime + Coolify pre-blocking/post-logged
     as reconcilePromoting 0→100 weight + rollback to prior digest)
  → gateway/DNS/TLS → Healthy → usage/meter/audit
```

- Keep: remote builders, registry handoff, layer caching, CI-built images + webhook/API
  triggers (branch/tag-matched), build-vs-runtime var split, pre (blocking) / post
  (non-blocking) hooks, health + domain verify.
- Change only the last mile: `docker run` → guest-prep + MicroVM boot. BuildKit never
  runs customer workloads (SRS invariant).
- Keep: remote builders, registry handoff, layer caching, CI-built images + webhook/API
  triggers (branch/tag-matched), build-vs-runtime var split, pre (blocking) / post
  (non-blocking) hooks, health + domain verify.
- Change only the last mile: `docker run` → guest-prep + MicroVM boot. BuildKit never
  runs customer workloads (SRS invariant).
- Task order: G2 (BuildKit durable task → digest artifact) → G2b (rootfs+kernel→boot) →
  T9 (health-gated rollout/rollback) → G4 (secrets/volumes at boot).

## 5. Build engines (all feed the same OCI output)

| Engine | How it works | Porter take |
|---|---|---|
| Nixpacks (Railway OSS) | Plan phase (providers match source → nix packages + install/build/start cmds, overridable, `nixpacks.toml`) → Build phase (BuildKit OCI image: setup/install/build phases in topological order, Nix/Apt pkgs, shell cmds, assets, default CMD) | Primary auto-detect. Keep plan JSON (reproducible rebuilds) + command overrides + `NIXPACKS_*` vars |
| Railpack (Railway successor, beta) | Same shape as Nixpacks, newer detector (`railpack.json`) | Second auto-detect alongside Nixpacks |
| Heroku Buildpacks | Detect → build in layers (each pack contributes layers, cached) → runnable image | Third fallback (Dokploy parity); layer cache = fast rebuilds |
| Dockerfile | Exact control (base, packages, stages, user, CMD); build-args injection (toggleable); BuildKit secrets for sensitive values; `SOURCE_COMMIT` excluded by default | Power-user path; staged builds encouraged (small runtime stage → small rootfs) |
| Static | Publish dir (e.g. `/dist`) served via Nginx image | Static sites → Nginx rootfs variant (same pipeline, fixed CMD) |
| Compose | Per-service build-or-pull; Coolify runs `docker compose up` (service reconcile), Swarm via `stack deploy` start-first | Porter compose (vyoma `compose` pkg): multi-MicroVM stacks, `depends_on`, per-service scale, shared networks |

## 6. OCI → MicroVM conversion (the core Porter pipeline, step-by-step)

```
1. Resolve: pull manifest by tag → pin digest (immutable from here on)
2. Config: parse image config → OciImageConfig{entrypoint,cmd,env,workdir,exposed_ports,user}
3. Layers: fetch + verify (sha256) each layer blob → unpack in order onto staging dir
4. Init: generate /sbin/porter-init (export ENV…; cd WORKDIR; exec ENTRYPOINT+CMD or /bin/sh)
5. Rootfs: mkfs.ext4 image → copy staging + init → record exact bytes + sha256
6. Kernel: select compatible kernel (arch + virtio config, 6.18; MMIO vs PCI per --enable-pci)
7. Record: vm image row {arch, rootfs{loc,sha256,bytes}, kernel{…}, config JSON} + immutable_reference
8. Cache: content-addressed store (same digest → skip download); optional warm snapshot for exact shape
9. Boot: drives (root + /dev/vdb volumes) → NIC (TAP/MAC/IP) → vsock → MMDS (hostname/mesh-IP/keys/metadata) → boot_args (init=/sbin/porter-init) → InstanceStart → workload = init → app
```

Failure handling per step: retryable (fetch/unpack/mkfs) vs fatal (bad digest/config) vs reconcile (boot). Every step emits task progress + events (Coolify-style deployment log: revision → build/pull → config → actions).

## 7. Plain MicroVMs from base images (a real VM, not just containers)

- Base images (Atlas `System image` equivalent): pinned Ubuntu 22.04/24.04 (+minimal) rootfs+kernel, versioned (bump only on artifact change), shared to all tenants, built by `build-ubuntu-base-image`-style publisher (cloud-init datasource + `AuthorizedKeysCommand` MMDS keys + `porter-metadata.service` for hostname/mesh-IP).
- `porter run ubuntu:24.04 --vcpu 2 --memory 1024` → resolve base digest → skip build → straight to §6 steps 5–9 → real VM with shell (`exec` over vsock, no SSH) + `logs -f` + `snapshot/restore` + `stop/start/restart` + `pause/resume`.
- Custom images: `porter build -t my-app:v1 .` (Vyomafile `FROM/RUN/COPY` + `CMD/ENTRYPOINT/ENV/WORKDIR/EXPOSE`) then same path. `commit/save/load` + `export/import` round-trips included.

## 8. Missed Coolify features → Porter PaaS checklist

- Preview deployments (PR/MR → isolated env `{{pr_id}}.{{domain}}`, own vars/domains, auto-cleanup on close/merge, fork guard, GitHub status comments) → Porter ephemeral environments (TTL + auto-destroy).
- Rolling updates (start replacement → health → stop old; overlap-safe releases; graceful shutdown within grace period; detect no-rolling cases and say why) → Porter `reconcilePromoting` weight shift.
- Rollbacks (retained image list → redeploy older with current runtime config; retention count; state restored separately) → Porter rollback to prior digest (code) + volume snapshot restore (state), two explicit steps.
- Deployment queue + history (queued/in-progress/success/failed/cancelled, per-op logs, first-failure pointer; failed build never replaces running) → Porter durable tasks (QUEUED/…/NEEDS_ATTENTION) + deploy history.
- Health checks + resource limits (CPU/mem caps, readiness gates) + persistent storage + env var groups (production vs preview creds separated) → Porter health in reconcile + cgroups + volumes + scoped secrets.
- Monitoring (disk usage + auto-cleanup, stopped/restarted containers, backup status) + notifications (Email/Telegram/Discord/Slack/Mattermost/Pushover/webhooks for deploy/backup/alerts) → Porter alerts/incidents + webhook/SSE spine.
- Web terminal (xterm.js + WS, per container/server) → Porter console over vsock (audited, recorded).
- Scheduled tasks + resource operations + migrate-app + danger zone (confirm/RBAC/audit/dep-check/backup-option) → Porter workflows/cron + operational-safety checklist (SRS §55).
- Multi-server + templates (one-click Plausible/Pocketbase/Calcom-style) + Docker management view → Porter multi-node scheduler + marketplace templates (compose stacks).

## 9. Missed Dokploy features → Porter PaaS checklist

- Databases (Postgres/MySQL/MariaDB/MongoDB/Redis + libsql): one-click create, env wiring, live CPU/mem/disk/net graphs, real-time logs, S3 automated backups + one-click restore (autocomplete from bucket), custom image swap, in-container run-command, persistent volumes, resource controls, danger-zone wipe → Porter native services (DB as MicroVM service + volume + backup/restore + scoped creds, same primitives as apps).
- Compose native (multi-container orchestration in place) + Swarm multi-node (cluster scale) → Porter compose stacks + multi-node scheduler (own, Nomad executor only in north-star).
- Templates (open-source one-clicks) → Porter marketplace (signed manifests, permission model, workflow packs).
- Traefik integration (routing/LB auto-wired) → Porter gateway (routes, LB, health-aware/weighted/canary, TLS/ACME).
- Real-time monitoring per resource + CLI/API parity + notifications (Slack/Discord/Telegram/Email) + multi-server remote deploy → Porter observability + thin CLI + same-API rule + event spine.

## 10. Next-gen serverless PaaS (beyond both)

- **Scale-to-zero / sleep-wake:** `sleep_after_idle_seconds` (Metal pattern: desired stays `running`, observed `stopped`) + traffic-triggered restore (host TCP sniff → wake; first packet may drop → client retries) + saved-state per VM. Idle VMs cost disk only. → Porter functions + idle services.
- **Cold-start budget:** warm snapshots for exact image+shape (host-local, never uploaded) + squashfs-compressed rootfs + minimal kernels → boot ≤125 ms path; cold boot always available as fallback.
- **Functions + cron + jobs as first-class workloads:** same runtime/net/storage/secrets/health/logs/metrics/events/scaling/lifecycle/RBAC/billing primitives (SRS §20) — a function is a service with `sleep_after_idle_seconds=60` + gateway route + per-invocation meter.
- **Per-request metering:** invocation count + GB-s (vcpu×time, mem×time) + egress bytes → `usage_events → meters → charges` (already in commerce model); dashboard shows cost per deploy/preview/function.
- **Edge posture:** gateway at region edge (Cloudflare optional), preview domains with auto-TLS, private services never exposed (egress `none/mesh` modes).
- **Run finish (phased):** v0.1 runtime+sleep/wake → v0.2 build+OCI→MicroVM → v0.3 gateway/DNS/TLS+previews → v0.4 multi-node+compose → v0.5 services (DBs/volumes/secrets/autoscale/strategies/rollback) → v0.6 observability+notifications → v0.7 commerce (meters→invoices, serverless billing) → v0.8 marketplace/templates → v0.9 hibernate/teleport/disruption budgets → v1.0 hardening.

## 11. Domains / DNS / edge (preview + custom + own DNS + Cloudflare)

- **Domain options (Coolify model, Porter same):** own custom domain (prod; A record → server IP, FQDN `https://app.example.com[,port][/path]`, multi-URL comma-separated, lowercase+dedupe) | wildcard-generated (`*.example.com` + wildcard A record → per-app subdomains, best for many apps/previews) | `sslip.io`-style zero-DNS testing (IP-encoded hostname, temp only, never prod). Databases never get domains (use public ports instead).
- **Setup flow:** point DNS → enter FQDN in Porter → verify (A record matches server IP or recognized Cloudflare proxy range; ports 80/443 open; proxy running; app listening on target port) → redeploy → route live. Punycode first for non-ASCII names (record + saved value must resolve identically).
- **Preview domains:** template `{{pr_id}}.{{domain}}` (+`{{random}}` variant), per-preview vars/domains separate from production (non-prod creds for preview DBs/storage — preview data outlives preview containers), fork-PR guard (block unless owner/member/collaborator), PR status comments (needs GitHub App perms), auto-cleanup on close/merge, manual Load-PR + rebuild-without-pull + per-preview logs.
- **Routing:** hostname+path match → container/MicroVM port (default 80, explicit `:port` override); path-based sharing (`https://d.com/a`, `https://d.com:3000/b`); www→apex redirects; `HostRegexp` multitenancy (Traefik-style labels); catch-all documented as no-cert.
- **TLS:** `https://` = automatic (proxy requests + installs + renews Let's Encrypt 90-day; Traefik `dnsChallenge` for wildcards with provider env vars; custom/self-signed supported). Origin options: Cloudflare Origin cert (15y, free, Cloudflare-always-in-front) vs public CA (needs DNS challenge). Renewal seamless; expiry monitored.
- **Own DNS system (Porter-native, like Atlas images/docs):** provider-neutral `dns_zones/dns_records` (A/AAAA/CNAME/TXT/MX/NS/SRV) + per-team zones + verification (TXT/propagation checks) + private DNS/SD + ACME DNS-01 + preview-domain automation. Cloudflare = first-class adapter (DNS/edge/WAF/CDN/tunnel/full-TLS modes) never a hard dep; Route53-style adapters follow the same interface.
- **Egress modes per workload (Atlas matrix):** `uplink` (mesh + internet, public IP allowed) / `mesh` (peers only, IP rejected) / `none` (nothing) — preview DBs stay `mesh`/`none`, public apps `uplink`.

## 12. Mail (built-in SMTP now, full system later) + monitoring + backups/schedules

- **Built-in SMTP (now):** system-wide transactional mail (welcome, invites, invoice, alerts) via configurable provider: hosted email / SMTP host-port-creds / Resend API key. Team notification channels (Coolify model, per-team settings, per-channel event selection + test-send): Email, Discord (webhook URL), Telegram (bot token + chat ID), Slack/Mattermost (incoming webhook), Pushover (user+app keys), generic JSON-POST Webhook. Events: deploy success/fail, backup success/fail+warning, scheduled-task result, container/VM stopped/restarted, server/disk alerts.
- **Later mail system:** per-service outbound (app mail via scoped SMTP creds, rate-limited, audited), inbound routing (domain → webhook/handler), per-team sender identities + SPF/DKIM guidance, queue + retry + bounce tracking. Queued behind the same workflow/task spine.
- **Monitoring (Coolify model → Porter):** disk usage (+auto-cleanup at threshold), stopped/restarted runtimes, backup status/success-warning/failed, per-resource live CPU/mem/disk/net graphs, real-time logs, terminal access (xterm over WS → Porter vsock console). Log drains (Fluent Bit custom config) for external ships.
- **Backups/schedules (Coolify model → Porter):** engine-aware DB dumps (pg_dump custom/all gzip, mysqldump, mariadb-dump, mongodump gzip, ClickHouse archive; Redis/KeyDB excluded) + file-level `.tar.gz` of volume/directory mounts; cron or named (`every_minute/hourly/daily/weekly/monthly/yearly`) schedules + timeout (≥60s, default 3600); local (`/data/.../backups`, inspect via Executions) + S3 copy (R2/S3/Supabase-compatible; `Disable Local` keeps remote-only; S3-fail keeps local + `Success (S3 Warning)`); independent retention (count + days + GB, each side; most-recent-first delete); restore = separate explicit flow (never from the schedule page); full CLI parity (`backup create/trigger/executions/update/delete`). Porter: same shape over volumes + MicroVM snapshots, **restore verified before trusted**, restore-to-new supported.

## 13. K8s features adapted to our own PaaS (no K8s itself)

| K8s concept | Porter adaptation (MicroVM-native) |
|---|---|
| Deployment + ReplicaSet | Service `desired_replicas` → replica rows → MicroVMs; rolling knobs (maxUnavailable/maxSurge as counts) drive `reconcilePromoting` weight shifts |
| HPA (CPU/mem/custom metrics) | Autoscaler controller: metrics (CPU/mem/req-latency/queue-depth) → desired replicas within `min/max_replicas`; scale-up fast, scale-down with stabilization delay; events + audit per scaling decision |
| Namespaces + labels/selectors | Tenancy groups: org → team → project → environment (+ labels/annotations on every resource); selectors become Porter label queries; `prod/dev` = environments with protection rules (approvals, freeze windows, resource quotas) |
| Services (ClusterIP/NodePort/LB) | Internal DNS/SD names per service (mesh-only) + gateway routes (public); private stays `mesh`/`none`, public via `uplink` + route |
| Ingress + cert-manager | Gateway routes (host/path/port, redirects, headers, rate limits) + certificate lifecycle (request→validate→issue→deploy→renew, ACME http-01/dns-01, expiry monitoring) |
| PersistentVolumeClaims | Volumes independent of VM lifecycle (create/attach/detach/resize/snapshot/backup/restore/clone, storage classes); ephemeral root/scratch dies with VM by design |
| Jobs + CronJobs | One-shot workloads + scheduled workloads (cron syntax + named schedules), history + retry + output capture, same primitives as services |
| ConfigMaps + Secrets | Variables (non-secret, env-grouped prod-vs-preview) + secrets (encrypted, scoped injection, rotation/versioning, never in logs) |
| Taints/tolerations + affinity/anti-affinity + topology spread | Scheduler filters (arch/capacity/health/labels/taints/storage/net/placement) + scoring (binpack/topology/cost/failure-domain) + disruption budgets |
| PodDisruptionBudget | Workload disruption budget (`minAvailable`) enforced on drain/upgrade/reschedule |
| Node cordon/drain | Same verbs on Porter nodes (cordon → drain with budget → workloads recreate → volumes reattach → health → traffic) |
| kubectl exec/logs/port-forward | `vm exec` (vsock, `vm.console` cap, recorded) + `logs -f` + console WS; no SSH, no host ports |
| Ephemeral (non-persistent) present VMs | First-class: `ephemeral: true` services (preview/sleep-wake/functions) — root/scratch discarded on stop, persistent data only via attached volumes/backups; scale-to-zero allowed; present-but-stopped VMs keep disk only |
| StatefulSet (stable identity + storage) | Stateful services: stable replica index → stable hostname + stable volume binding + ordered rollout; DBs run this way |
