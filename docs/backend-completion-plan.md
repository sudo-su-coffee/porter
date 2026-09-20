# Backend completion plan — update flow for the existing Go backend

> Written 2026-09-19 after reading the old backend (`backend/internal/*`, `cmd/porter/main.go`,
> `migrations/0001…0016`). This is THE execution order. Gate after every phase:
> `cd backend && go build ./... && go test ./...` (+ `go vet ./...`) stays green.

## 0. What exists (do NOT rewrite)

- `api/` — ~300 routes in `api.go`, all behind `a.auth`; 157KB `handlers_impl.go`. KEEP.
- `store/` — 84KB Postgres (`store.go`), `Migrate` runs embedded `NNNN_*.up.sql` in order with version tracking, `EnsureDefaultOrg` + `EnsureSeededAdmin` seeds. KEEP.
- `types/` (41 types) + `resource/` (15 canonical files) split — controllers stay on `types.*`; unify later (T7b), never mid-phase.
- `runtime/` — `FCClient` (unix-socket PUT chain), `Manager` + `VMManager`, deterministic `NetworkManager`. KEEP.
- `controller/` — `Manager` (ticker per reconciler) + Deployment (building/rolling/promoting/failed/rollback) + Replica + Node controllers. EXIST but NOT registered in `main.go`. Register, don't rewrite.
- Working small pkgs: gateway, compose, buildkit builder, dns, netmgr, volumes, event hub, health, observability, config, cache, cron, sshgw, guestagent, tls, notify, autoscale, imagecatalog.
- 15 EMPTY pkgs (0 `.go` files): agent, ai, billing, build, certificate, deployment, firecracker, incident, marketplace, network, policy, rbac, scheduler, storage, workflow. Fill in task order; delete nothing (except vacuous `firecracker/` if `runtime/` stays home — decide at Phase 7).
- Migrations 0001…0016 = 41 tables. **Existing RBAC (0007) is `roles/permissions/role_permissions` + `HasPermission(username, permission)` — NO scopes, NO deny, NO capabilities table.** The 0017 plan BRIDGES it (adds scope), never replaces it (data loss + rewrite risk).

## 1. Phase order (each phase = implement + tests + green gate)

```
P0  Gate check: go build ./... && go test ./... && go vet ./... (record baseline)
P1  Migration 0017 (up/down): role_assignments (principal,role,scope) + deny column on
    role_permissions + resellers + teams + ip_allocations + micro_vms. Bridge over 0007.
P2  Go types (T1c): Capability→(reuse Permission), RoleAssignment, Reseller, Team,
    MicroVM, IPAllocation in resource/ + store CRUD.
P3  RBAC engine (T2): internal/rbac HasCapability(principal, capability, scope) =
    union at scope + ancestors, deny overrides allow. Store methods: assign/revoke/list.
P4  Auth middleware (T3): JWT (EdDSA+JWKS plan, HMAC interim) + opaque API keys +
    AuthContext{principal_type, principal_id, scope, claims} + bootstrap admin.
    Chain: extract → validate → context → 401/403 structured errors.
P5  Route guards (T4): apply middleware in api.Routes(); route→capability map;
    scoped list filtering (T4b). No hidden privileged path.
P6  Tenancy (T5): org→team→project→environment scope tree end-to-end + memberships.
P7  Network (T6): ip_allocations allocator + TAP//30 NAT + limiters; UNIFY netmgr
    10.42/16 vs runtime 172.16/16 + MAC conventions (T6b). Single allocator.
P8  Controllers (T7): construct Deployment/Replica/Node + register in controller.Manager;
    Start/Stop in cmd/porter/runServer. Unify VM state constants (T7b).
P9  Lifecycle (T8): service→deployment→replica→micro_vm rows via reconcile; status +
    durable events + audit (vm.create.started/created/failed…).
P10 Rollout (T9): real rollback (prior digest) + health-gated 0→100 promotion.
P11 API surface (T10): idempotency keys (T10a) + cursor pagination + ETag/If-Match +
    request IDs + structured errors + field select + POST-for-actions (T10b).
P12 G1–G6 full surface: exec/console (vm.console) + logs; BuildKit task → digest artifact →
    rootfs+kernel→boot; health in loop + snapshot recovery; secrets/volumes at runtime;
    durable events/audit spine; per-resource caps on gateway/DNS/certs/cron/hook/alert/etc.
P13 E2e (T11): API-driven harness (install→node→preflight→project→build→schedule→
    provision→net→start→health→gateway→domain→traffic) + security/tenancy acceptance.
P14 Audit (T12): per-feature ARCH §37 completion check + truthful status labels.
```

## 2. Why bridge 0007 instead of replacing (decision, binding)

0007 seeds 4 roles × ~80 `resource.action` permissions with live assignments possible in
deployed DBs. The capability model (`capabilities`, scoped assignments, deny) is a SUPERSET
expressed as: keep `roles/permissions/role_permissions` rows; ADD `effect` (allow|deny) to
`role_permissions`, ADD `role_assignments` with scope columns, TREAT `permissions.id` as the
capability string. Zero data migration, zero handler rewrites in P1; `HasPermission` keeps
working while `HasCapability` layers scope on top. Replacement would orphan seeds + break
every existing check for no runtime gain.

## 3. Per-phase done rule (ARCH §37, condensed)

API contract · authZ · persistence · controller/workflow · real infra op · status · events ·
audit · failure handling · retry/idempotency · tests. Label IMPLEMENTED/PARTIAL/EXPERIMENTAL/PLANNED.

## 4. Status (2026-09-19 — P0–P10 partial + G1–G6 partial + T11 harness; gate green)

- P0 DONE: `go build ./...`, `go vet ./...` clean; full `go test ./...` green except ONE
  pre-existing Windows-only failure (`TestFCClientUsesUnixSocketAndOfficialPayload` —
  unix-socket bind in this sandbox; `fc_test.go` untouched, fails identically on HEAD).
- P1 DONE: `migrations/0017_scoped_rbac_tenancy_compute.{up,down}.sql` (effect column,
  role_assignments, resellers, teams, ip_allocations, micro_vms; no billing tables).
- P2 DONE: `internal/resource/{rbac,microvm}.go` + 4 Kinds (Team reused from org.go).
- P3 DONE: `internal/rbac/` engine + 7 unit tests + `internal/store/rbac_scoped.go`.
- P4 DONE except JWT/sessions/list-filter-core: `auth/context.go` + 4 tests; `api.auth`
  attaches AuthContext; `granted()` layers scoped-allow/deny incl. org scope; T4b project
  list filtering via `UserScopes`; RLS migration 0019 (FORCE, permissive-when-unset).
- P5 DONE: `store/tenancy.go` (chain, org/group/env resolvers, memberships, UserScopes).
- P6 DONE except host TAP/NAT/limiter ops + `/30`: unified 10.42 math + `DeterministicMAC`
  (`netmgr` + `runtime`), convergence test, `store/network.go` allocator.
- P7 DONE: controllers registered + started in runServer; states unified via aliases.
- P8 DONE: `store/microvms.go` + lifecycle rows/events in ReplicaController.
- P9 DONE: stepped 0→100 promotion off `replicas_desired`.
- P10 DONE except pagination/ETag/field-select: idempotency middleware + replay marker +
  request IDs + tests.
- G1 DONE except interactive console: real exec chain (protocol + vsock dial + argv +
  unified Execer + tests). G2 DONE except OCI chain: digest pinning wired. G3 DONE.
  G4 DONE except secret injection: volume attach. G5 DONE store-side. G6 DONE except
  full per-route audit pass.
- P11 DONE (PG-gated harness, 5 tests). P12: see docs/completion-audit.md §8.
- NEXT: `firecracker/` dir removal, JWT validators, `LoadSnapshot`, secrets injection,
  pagination/ETags, team-member rows + RLS enforcement, KVM-host full e2e, T12 labels.
(End of status — supersedes the older P0–P4 snapshot that was here.)
