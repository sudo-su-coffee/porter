# Porter — NEXT SESSION PLAN (handoff → v1.0.0)

> Write this so the next session can start with zero context loss. Everything
> below is current truth (verified 2026-08, `go build/vet/test` green as of the
> last commit).

## 1. Where we are (DONE — committed)

Backend is **feature-complete for v1.0.0-rc**. 40 tables across 5 migrations,
~270 routes, all guarded by fine-grained RBAC permissions. Commits (chronological):

| Commit | What |
|--------|------|
| `274836e` | Stages 2–8 & 10a: DNS/TLS server, bandwidth, web-vitals, volumes, RBAC, cron, firewall, metrics |
| `89b35a0` | startup sanity checks, horizontal autoscaler, removed dead `internal/vmmanager` |
| `156a2fd` | RBAC v2: roles/permissions/role_permissions + routePerms map + Vercel usage metering |
| `55d3913` | rolling updates + deployment checks + Redis read-through cache |
| `eabd1c8` | dedupe routes + complete routePerms coverage (two-way test) |
| `ec75f1a` | git→build bridge (docker/buildctl → containerd import) + config.example + cache config |
| `7749904` | docs: tracking |

### Task list state
**COMPLETED (14/18):** #8 RBAC CRUD, #9 route coverage, #10 git build bridge,
#11 rolling checks, #12 error audit, #13 migrations verified, #15 Redis cache.
**REMAINING:** #14 Frontend (big), #16 release docs, #17 networking dedup,
#18 completion verify.

## 2. WORK IN PROGRESS (not committed — finish FIRST)

**Networking consolidation (#17)** — mid-edit in the working tree:
- `internal/netmgr/netmgr.go` ⚠️ now has `BootSpec`, `AllocateProjectSubnet`, `AllocateVMNetwork`,
  `bootMAC`, `bootShortID` added (moved from the old `internal/net`).
- `internal/runtime/manager.go` ⚠️ import changed `netmgr "porter/internal/net"` → `"porter/internal/netmgr"`.
- `cmd/porter/main.go` ⚠️ removed `fcnet` import; `vmEngine.net` is now `*netmgr.NetManager`;
  `newVMEngine` uses `netmgr.NewNetManager()`; `netMgr` (API) already netmgr.
- `internal/net/` ⚠️ still on disk, now **dead** (no imports; grep `"porter/internal/net"` = none).

**Finish this first:**
```bash
cd backend
rm -rf internal/net        # dead package
go build ./...             # must be green
go test ./...              # must be green
git add -A && git commit -m "refactor(net): consolidate internal/net into netmgr"
```
> The tree currently builds even without the `rm` (dead package compiles), so if
> anything above surprises you, just `rm -rf internal/net` and rebuild.

## 3. REMAINING BACKEND (small, v1.0.0-rc)

1. **Real host-port binding** — `types.Port.HostPort` is parsed but no host
   listener/DNAT binds it. Add a small host-port forwarder (bind HostPort →
   proxy to VM container port) or document domain-routing as the replacement.
   Files: `internal/api/handlers_impl.go` (compose ports), `internal/gateway`.
2. **Release docs (#16)** — update `README.md` "Current Code State" + `PLAN.md`
   component table (most rows now DONE), bump `VERSION` only when the user approves
   (currently keep `v0.1.0-beta-dev` — user explicitly wants beta-dev until fully
   working). Update `V1_TRACKING.md` status.

## 4. FRONTEND (#14) — the remaining big track

Goal: Vercel/Apple-quality Vue 3 dashboard. Reference: `frontendsample/` (excluded
from git via `.gitignore`). Existing shell: `frontend/` (Vue 3 + Vite, dev proxy → :8080).
Build: `make frontend` (npm install + vite build → `backend/web/dist`, embedded via go:embed).

Views to build against the ALREADY-WORKING API:
- Dashboard / overview (`GET /overview`, `GET /usage`)
- Project detail: Deployments, Domains, Logs, Traffic/Analytics, Firewall, Volumes
- Project settings (General/Build/Git/Env/Secrets/Security/Networking — all have real endpoints)
- Members + Roles/Permissions UI (`GET /roles`, `GET /permissions`, `PUT /roles/{id}/permissions`)
- Login/auth (per-user token) + CSRF header on writes

Do NOT rebuild the backend for the frontend — every route already exists.

## 5. Rules to respect

- **Version stays `v0.1.0-beta-dev`** until the user approves a release bump.
- Frontend is NOT part of the backend repo commit concerns (`frontendsample/` ignored).
- Postgres-only; no new DB driver. Add tables via `migrations/NNNN_*.{up,down}.sql`.
- Every new route MUST get a `routePerms` entry + coverage test still passes.
- Honest status only: never fake `200`+empty JSON as a "working" feature.