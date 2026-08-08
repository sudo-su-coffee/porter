---
name: porter-paas-expert
description: Porter PaaS expert — simple, phase-wise truth. What the code does NOW vs what's needed LATER. PostgreSQL-only storage + Firecracker runtime + optional Redis cache + Vue frontend. Includes user-record schema, real-vs-stub list, contribution rules, status refresh protocol.
---

# Skill: Porter PaaS Expert

## What Porter is (plain)
A self-hosted PaaS, one codebase. You give it a Docker/OCI image (or `docker-compose.yml`), it boots that app as **Firecracker microVMs** and gives you a dashboard + REST API to deploy, scale, see logs/traffic, and health-replace bad replicas. Like Fly.io / Vercel on your own machine. Single operator, single tenant.

## The 4 golden rules (never break)
1. **PostgreSQL-only storage.** No MySQL, no SQLite, no on-disk JSON. Every durable write goes through `internal/store` (pgx). `go.mod` has no sqlite/MySQL driver.
2. **Firecracker is the only runtime.** MicroVMs. `simulate` is gone. Real boots only.
3. **Redis = cache/queue only** (new, [NEXT]). Postgres stays the source of truth; Redis is for cache, sessions, SSE fan-out, rate-limit counters, build queue. Never the truth store.
4. **Runtime = containerd + `aws.firecracker` shim; control plane = Go.** containerd/firecracker boots each OCI image (pulled from a Docker/OCI image store) as a microVM. One **base microVM image** (rootfs+vmlinux) is the default base for "run whatever", and users can upload their own custom image (direct-Firecracker). All DNS, networking, gateway, health, scheduling, auth logic is Go.
5. **Proven Go libraries — don't hand-code.** DNS → `miekg/dns`; SSH host-side bridge → `golang.org/x/crypto/ssh`; traffic capture/log streaming → `gopacket`/`pcap`; queue → Redis/`asynq`; RBAC → existing packages. Own code = control-plane glue, never reimplementing these.
6. **UI-only product.** End users get the dashboard behind the code — **no user-facing CLI** to manage apps. `cmd/porter` subcommands (`server`, `kernel`, `version`, workers) are installer/ops internal only.

---

## PHASE NOW — what the code already does (verified 2026-08, re-check before trusting)
- **Persistence:** PostgreSQL via `pgx/v5`. `store.Migrate` runs `backend/migrations/*.sql` at startup (up to `0020`). Refuses to start without `PORTER_DATABASE_URL`.
- **Runtime:** containerd + `aws.firecracker` shim for OCI images; direct-Firecracker for uploaded rootfs/vmlinux. Entrypoint `backend/cmd/porter`.
- **Real features:** project CRUD, compose import+boot, replica scale/start/stop/restart/logs, gateway proxy, traffic ring, health auto-replace, `.local` DNS, SSH gateway (`[ssh] enabled`), deployments/preview/promote/rollback, secret AES-GCM injection, honest git clone+build (marks `failed` with real error, never fake "building"), image catalog + golden seeds (redis/postgresql/mysql), overview/host stats, analytics aggregations, audit log (`daemon_logs`), org/team membership RBAC CRUD.
- **Frontend:** Vue 3 dashboard (`frontend/`), Vercel shell. Talks to `/api/v1` routes. Needs CSRF token for writes.
- **Two in-memory by design:** traffic ring + per-VM log tail (fast path); durable copies in `daemon_logs`, metrics tables.

## PHASE NOW — known stubs / partials (don't claim these work; check the handler first)
- Real wire-level DNS server — `.local` resolver only today.
- TLS / ACME / Let's Encrypt — not yet; `handleVerifyDomain` still reports "verified" as a stub.
- Real host-port binding — compose `parsePort` drops host port ("8080:80" keeps only 80).
- Wired 2026-08 to real store/runtime: cache purge/path, env branch/domain, environments/available, pool/drain, host/kernel probe, groups PATCH/DELETE, password forgot/reset (single-tenant policy), builds GET/POST split. Remaining gaps: wire-level DNS server, TLS/ACME, real host-port binding, real volumes, per-user RBAC.
- Real persistent volumes — DB row only (nothing boots a block device).

## PHASE NEXT — what's needed (maintainer directives + PLAN.md; mark [planned] until seen in source)
1. **Redis wiring** [NEW]: apply `internal/cache/` optional Redis (cache, sessions, SSE fan-out, rate limits, build queue). Off by default — still works without it.
2. **Headroom / capacity guard** [NEW]: scheduler tracks host CPU/RAM vs replica reservations; refuse over-commit or keep a configurable `headroom` buffer.
3. **Base microVM image** [core, mostly done]: one rootfs+vmlinux base image as the default base for "run whatever" + user-upload custom images (direct-Firecracker path); OCI images keep going through containerd + `aws.firecracker` shim from the Docker/OCI store.
4. **TLS via ACME** + authoritative DNS server (self-answer TXT).
5. **Git deploys** E2E (clone → Dockerfile detect → build → boot).
6. **Real volumes** (create/attach/mount), not DB rows only.
7. **Networking consolidation** — one allocator (`internal/net` vs `netmgr`). Delete dead `internal/vmmanager`.
8. **Per-user RBAC** beyond single bearer token + admin.

## PHASE LATER — v1.0.0 workstreams (PLAN.md; all [planned] until source confirms)
Deploy-from (image/compose/git) → Manage & Scale → K8s-parity orchestration → multi-node cluster (Fly parity) → Vercel-parity dev-experience → observe/measure → operate (backups, quotas, upgrades).

---

## Storage & credentials — exact schema + queries (PostgreSQL, not MySQL/SQLite)
> "MySQL" in code = only the *seeded golden image name* in `0016_seed_golden_images`. SQLite = only a stale comment at `types.go:132`. Real store is PostgreSQL.

**`users` table** (migrations `0010`, originally `0002`):
```sql
CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username      TEXT NOT NULL UNIQUE,
    role          TEXT NOT NULL DEFAULT 'admin',
    password_hash TEXT NOT NULL,               -- hex(sha256(salt + password))
    salt          TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
```
**Queries** (`internal/store/store.go`, "--- Users ---"):
- `PutUser` — `INSERT INTO users (id, username, role, password_hash, salt, created_at) VALUES ($1..$5, now()) ON CONFLICT (username) DO UPDATE SET role=EXCLUDED.role, password_hash=EXCLUDED.password_hash, salt=EXCLUDED.salt`
- `ListUsers` — `SELECT id, username, role FROM users ORDER BY created_at`
- `GetUserByUsername` — `SELECT id, username, role, password_hash, salt FROM users WHERE username = $1`
- `DeleteUser` — `DELETE FROM users WHERE id = $1`

**`api_keys` table** (`0011`): `id, user_id, name, token_hash (sha256 hex of raw key), created_at, last_used_at`.

**Hashing** (`handlers_impl.go:183`): `passwordHash = hex(sha256(salt + password))` — **SHA-256 + per-user salt, NOT bcrypt**. Create user: `{username, password, role}` → `salt = store.NewID()` → `PutUser`. Login: `constantTimeEqual(passwordHash(input, user.Salt), user.PasswordHash)` (`api.go:198`).
**Bootstrap admin is NOT a row** — lives in `porter.toml` `[admin] password` / `api_token`.

---

## Frontend needs (when adding a feature, wire these too)
- Writes send `Authorization: Bearer <token>` and `X-CSRF-Token` (get first from `GET /csrf`).
- Refresh on SSE (`GET /events`): events `vm.state`, `replica.health`, `domain.status`.
- Used routes: `/projects`, `/projects/{id}/replicas/{n}`, `/projects/{id}/logs`, `/images`, `/overview`, `/host/*`, `/volumes`, `/users`, `/traffic`.
- Design system at `frontend/src/style.css` (tokens/tables/tags/tabs/status/terminal/modals/image grid) — reuse.

## Status refresh (before answering "is X shipped?" — mandatory)
1. `git log --oneline -n 15` — recent commits change everything.
2. PLAN.md status table / README § Planned — README wins over PLAN.
3. Open the handler in `handlers_impl.go` — empty JSON = stub.
4. Migration ceiling (`backend/migrations` max number) — new table = real feature.
5. To record status → update the per-project memory files + `MEMORY.md` index with the date.

## Contribution rules (maintainer's workflow)
- Windows: use Read/Glob/Grep (not bash) for exploring code.
- Write code in one pass; **no compile in the loop**; single `go vet ./...` + `go test ./...` at the end. Commit only when asked.
- Postgres only; images by URL (never local paths — user may be offline); app must work offline.
- Honest status: never fake `200` + empty JSON as a working feature.

## Troubleshooting cheat sheet
- Boots fail → `/dev/kvm`? containerd `devmapper`? `aws.firecracker` runtime registered? `porter kernel set` done?
- Compose import fails → `build:` rejected, no volumes, `depends_on` acyclic, every service needs `image:`.
- Multi-service → only `POST /projects/compose` (YAML) today; UI form [planned].
- Storage → PG container (`deploy/dev.sh`) or installer-provisioned PG; data in `users`, `api_keys`, `orgs`, `projects`, `replicas`, `domains`, `deployments`, `secrets`, `golden_images`, `daemon_logs`, `metrics_samples`. Never SQLite.

---

Purpose: best answer, least input. Correct premise → honest NOW vs LATER → exact schema/queries → file:line.
