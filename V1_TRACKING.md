# Porter v1.0.0 — Stage-by-Stage Tracking

> **Purpose:** Track every stage from current state → v1.0.0 release. Each stage = one or more commits, verifiable at build+test time. Status tags: **[DONE]** **[IN PROGRESS]** **[PLANNED]**.

---

## Current State (verified 2026-08)

- **Routes:** 255 registered, 243 REAL, 12 wired in commit `e0271ec`
- **Deploy pipeline:** 100% real (OCI → VM, compose → stack → per-service pools, git → build → VM)
- **Analytics:** real traffic ring + durable logs, paths, status codes, timeseries, global
- **Domain verify:** real DNS probe (not stub)
- **DB:** PostgreSQL only, migrations consolidated to 3 phases, `go build/vet/test` green

---

## Stage 1: Deploy Pipeline Hardening — [DONE]

| Commit | What | Status |
|--------|------|--------|
| `e0271ec` | Wire 12 stub/partial handlers (groups, envs, drain, cache path, host/kernel, builds) | **[DONE]** |

**Result:** OCI→VM, compose→stack, git→build→VM, deployments promote/rollback — all real.

---

## Stage 2: Domains + DNS + TLS — [PLANNED]

### 2a. Authoritative DNS server (stdlib, no new deps)
- [ ] Create `internal/dnsserver/` package: minimal UDP authoritative resolver
  - Parse RFC1035 DNS queries
  - Answer A records for `*.porter.<baseDomain>` → replica IPs from store
  - Answer explicit domain entries from `domains` table
- [ ] Wire into `cmd/porter main.go` when `[dns] enabled`
- [ ] Config: `[dns] address = "0.0.0.0:53"` (UDP), `[dns] enabled = true`

### 2b. Preview/prod domain auto-assignment
- [ ] On deploy (git/compose), auto-assign `<project>-<deployment>.porter.<base>`
- [ ] Store in `domains` table with `status = "active"`
- [ ] `handleVerifyDomain` — real DNS check (already done) + store status

### 2c. TLS via ACME
- [ ] `golang.org/x/crypto/acme/autocert` — cache certs in `internal/cache/`
- [ ] Gateway TLS listener on `:443` when cert available
- [ ] Auto-provision on first domain attachment
- [ ] Fallback: HTTP-only when no cert

**Dependencies:** DNS server must land before TLS/ACME (needs TXT challenge response).

---

## Stage 3: Analytics + Traffic — [PARTIAL]

### 3a. Real byte-level bandwidth
- [ ] Add `Bytes int64` field to `types.TrafficEntry`
- [ ] Gateway records response bytes at proxy (update `addTraffic` call)
- [ ] Store `traffic_logs` — add `bytes` column (migration 0021)
- [ ] Analytics handlers aggregate `e.Bytes` instead of `DurationMS` proxy

### 3b. Web-vitals refinement
- [ ] LCP: p95 latency as proxy (already partially implemented)
- [ ] INP: derive from request duration distribution
- [ ] CLS: keep 299 synthetic beacon mechanism (already wired)
- [ ] FID: derive from first-byte-to-interactive delta (if measured)

### 3c. Account-level aggregates
- [ ] `GET /analytics/account/usage` — total requests/bandwidth across all projects
- [ ] `GET /analytics/account/projects` — per-project breakdown
- [ ] `GET /analytics/account/top` — top 10 projects by requests

---

## Stage 4: Image Catalog — [PLANNED]

### 4a. Multiple base vmlinux images
- [ ] Extend `0016_seed_golden_images` — add Debian, Alpine, Ubuntu base images
- [ ] `GET /images/bases` — list available base images
- [ ] User can select base at project create

### 4b. Custom image upload
- [ ] `POST /images/custom` — upload rootfs+vmlinux zip
- [ ] Unpack to `customImagesDir`, register in golden_images table
- [ ] Boot via direct-Firecracker path

---

## Stage 5: Storage (Volumes) — [PLANNED]

### 5a. Real persistent volumes
- [ ] `POST /volumes` — create real host directory + DB row
- [ ] `GET /volumes` — list with real size/usage
- [ ] `DELETE /volumes/{id}` — remove directory + DB row
- [ ] Boot: bind-mount `volume.path` → `volume.mount_path` in VM spec

### 5b. Snapshots
- [ ] `POST /volumes/{id}/snapshot` — rsync/copy to snapshot dir
- [ ] `POST /volumes/{id}/restore` — restore from snapshot

---

## Stage 6: RBAC & Access Control — [PLANNED]

### 6a. Per-user roles
- [ ] Extend `users` table: `role` column (admin/member/viewer)
- [ ] `requireRole` middleware: enforce per-org/per-project access
- [ ] `POST /users/{id}/invite` — email invite flow (or config-based)
- [ ] `DELETE /users/{id}` — remove user (not bootstrap admin)

### 6b. API key scoping
- [ ] `api_keys` table: add `scope` column (projects/read, projects/write, admin)
- [ ] Enforce scope in middleware

---

## Stage 7: Cron Jobs & Alerts — [STUBBED]

### 7a. Cron runner
- [ ] Background goroutine: parse 5-field cron expressions
- [ ] `POST /crons` — create job (image, schedule, command)
- [ ] Boot job image as short-lived VM at scheduled times

### 7b. Alert evaluator
- [ ] Ticker: compare `metrics_samples` thresholds
- [ ] Insert `HealthEvent` + SSE broadcast on threshold breach
- [ ] `POST /alerts` — create alert rule (metric, threshold, action)

---

## Stage 8: Firewall & WAF — [STUBBED]

- [ ] `POST /firewall/rules` — create rule (allow/deny, port, CIDR)
- [ ] Apply rules to per-project veth/tap via `ip`/`tc` (best-effort)
- [ ] `GET /firewall/stats` — return real iptables counters

---

## Stage 9: Git Deploy Pipeline — [DONE, needs polish]

Already real: `handleDeployGit` → `runGitBuild` → clone + Dockerfile detect + build + boot.
Polish:
- [ ] Build logs streaming (SSE) during clone/build
- [ ] SHA-tagged images for rollback
- [ ] Build cache (layer reuse)

---

## Stage 10: Observability Hardening — [PARTIAL]

### 10a. Metrics subsystem
- [ ] `internal/metrics` package: collect per-VM CPU/RAM via cgroups
- [ ] `GET /projects/{id}/metrics/cpu` — timeseries
- [ ] `GET /projects/{id}/metrics/ram` — timeseries

### 10b. Audit log persistence
- [ ] `daemon_logs` table (0017) — already exists
- [ ] Ensure all mutations logged (currently best-effort via `AppendDaemonLog`)

---

## Stage 11: Dashboard Polish — [IN PROGRESS]

- [ ] Wire remaining UI views to real API data
- [ ] Add loading states for async operations (deploy, build)
- [ ] Mobile-responsive layout

---

## Stage 12: Hardening & Release — [PLANNED]

- [ ] Startup sanity check for jailer/shim config
- [ ] `porter kernel set` validation (check file exists, is vmlinux)
- [ ] Graceful shutdown (drain connections, stop VMs cleanly)
- [ ] Health check endpoint for load balancer (`/healthz`)
- [ ] Rate limiting per-client (already wired: `rateLimit` config)
- [ ] CSRF protection on all write routes (already wired)
- [ ] SQL injection prevention (parameterized queries — already done)

---

## Commit Log (append as stages land)

| Stage | Commit | Description |
|-------|--------|-------------|
| 1 | `e64d330` | docs: Porter PaaS expert skill |
| 1 | `73f46a0` | chore: move deploy tooling to root |
| 1 | `10d1085` | chore: remove stray backend.zip |
| 1 | `946c065` | refactor: consolidate 40→3 migrations |
| 1 | `e0271ec` | feat: wire 12 stub/partial handlers |
| 1 | `d8e4849` | docs: update skill stub list |
| 2–8,10a | `274836e` | feat: DNS/TLS, bandwidth, web-vitals, volumes, RBAC, cron, firewall, metrics |
| 2a | `wip` | feat: stdlib authoritative DNS server (miekg/dns) |
| 2b | `wip` | feat: preview/prod domain auto-assign |
| 2c | `wip` | feat: TLS via ACME (Let's Encrypt) |
| 3a | `wip` | feat: real byte-level bandwidth (bytes_in/bytes_out) |
| 3b | `wip` | feat: web-vitals beacon + p75/rating aggregation |
| 4a | `done` | feat: multiple base vmlinux (per-VM kernel already honored at runtime) |
| 5a | `wip` | feat: real persistent volumes (dir + sparse image) |
| 6a | `wip` | feat: per-user RBAC via project_members/org_members (PG) |
| 7a | `wip` | feat: cron runner (5-field schedule scheduler) |
| 8 | `wip` | feat: firewall/WAF enforcement in gateway |
| 10a | `wip` | feat: metrics collector (CPU/mem samples) |
| 12 | — | TBD: hardening & release prep |

---

## Status (updated after each commit)

- **Current stage:** Stages 2–8, 10a coded — pending commit + verify
- **Last commit:** `d8e4849`
- **Build:** ✅ `go build/vet/test` green (verified 2026-08)
- **Branch:** `main`, ahead of origin (delayed push)
