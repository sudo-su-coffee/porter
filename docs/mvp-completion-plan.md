# MVP completion plan — full feature checklist (2026-09-19)

> Written after reading every `backend/internal/*` package file-by-file.
> MVP = Identity + RBAC + Tenancy + Services/Replicas/MicroVM + Networks/Domains
> (+ full API surface). NO billing tables. Gate after every item:
> `go build ./... && go test ./...`, `go vet ./...` green.

## 0. Baseline truth (read, not assumed)

- `api/`: ~300 routes, all behind `a.auth` (bearer→principal→rate-limit→CSRF→`routePerms`→`granted()` project/org/global). Public: health/version/auth + images/ml alias.
- `store/`: PG pool + `Migrate` (embedded `NNNN_*.up.sql` + version tracking) + seeds; mixed direct-SQL + cached maps; `HasPermission/HasProjectPermission/HasOrgPermission`, `OrgRoleForUser/ProjectRoleForUser`.
- `controller/`: Manager (ticker/reconciler) + Deployment (building/rolling/promoting/failed/rollback) + Replica (pending→booting→running→stopping→stopped/failed→delete, snapshot recovery, restart-policy) + Node (Register→…→Ready, heartbeat, Unhealthy) — **implemented but NEVER started**; `main.go` drives VMs imperatively via `vmEngine` (VMManager+netmgr). **T7a decision: register controllers as the loop, keep vmEngine as the executor they call (ReplicaController already takes `*runtime.VMManager`).**
- `runtime/`: `FCClient` (unix-socket PUT: boot-source→drives→net→machine-config→InstanceStart; stop/snapshot; load NOT implemented; exec returns "use guestagent" error) + `Manager`/`VMManager` + deterministic allocators.
- Net split: `netmgr` (10.42/16, TAP-creating `AllocateVMNetwork`) vs `runtime.NetworkManager` (172.16/16, pure `AllocateVMNetwork`) + divergent MACs — unify in T6b (one allocator, one base, `/30` target).
- 15 empty pkgs: agent/ai/billing/build/certificate/deployment/firecracker/incident/marketplace/network/policy/rbac(done)/scheduler/storage/workflow.
- DONE P0–P5 + T6b/T7/T8/T9 + G1(exec)/G2(digest)/G3/G4(volumes)/G5/G6(seeds) + T10a/T10b(request IDs) + T4b filtering + RLS migration + T11 harness (2026-09-19 run; gate green — see docs/completion-audit.md).

## 1. Phase checklist (all MVP features)

### T5 Tenancy wiring (DONE 2026-09-19)
- [x] Scope consts + `ScopeChain` ordering (`resource`/`rbac`)
- [x] `store/tenancy.go`: `ScopeChainForProject` (project→groups→org), `OrgForGroup`, `ProjectForEnvironment`, `GroupRoleForUser` (via org membership until team-member rows exist in 0018), `UserOrgIDs`, `UserScopes` (orgs+groups+projects for T4b filtering)
- [x] Wire `X-Tenant-ID` → scope in `granted()` for org routes (project already done; team routes inherit via org membership until team-member rows exist)

### T6 Networking (DONE 2026-09-19)
- [x] `ip_allocations` allocator in store (`store/network.go`: allocate/release/list) backed by 0017 table
- [x] T6b UNIFY: single math (base 10.42, gateway .1, guests .10+idx, `DeterministicMAC` 06:) shared by `netmgr` + `runtime.NetworkManager` + convergence test (`/30` layout stays follow-up: needs guest-image + Linux e2e cover)
- [ ] TAP + NAT + token-bucket limiter host ops behind agent/runtime boundary; vsock for guestagent (host CID 2)

### T7 Register controllers (DONE 2026-09-19)
- [x] `main.go runServer`: `controller.NewManager` + Deployment/Replica/Node controllers + Start/Stop on shutdown; ReplicaController drives `*runtime.VMManager` executor (vmEngine stays as API imperative escape hatch)
- [x] T7b: VM state + health constants unified (`types` aliases `resource`; single source of truth)

### T8 MicroVM lifecycle chain (DONE 2026-09-19)
- [x] `micro_vms` rows persisted in ReplicaController boot/delete/stop paths (`store/microvms.go`)
- [x] Durable lifecycle events (`vm.create.started/created/failed`, `vm.deleted`) via `store.AppendEvent` + SSE hub fan-out

### T9 Real rollback + health-gated rollout (DONE 2026-09-19)
- [x] Rollback restores prior digest (existed) + health-gated promotion (weight steps 0→100 on healthy count)
- [x] `reconcileRolling` uses `replicas_desired` (falls back to `vm_ids` approximation)

### T10 API surface (PARTIAL 2026-09-19)
- [x] T10a idempotency keys on mutating writes (`idempotency_keys` table + `Idempotency-Key` middleware with scoped replay + `X-Idempotent-Replay` marker)
- [x] Request IDs on every route (`X-Request-ID` generate/echo + context) + replay tests
- [ ] Cursor pagination + ETag/`If-Match` + field select (envelope exists via request IDs; `writeError` still unstructured)

### G1 Interactivity (PARTIAL 2026-09-19)
- [x] Real exec chain: protocol `exec` action + `VMManager.Exec` vsock dial (explicit "agent not connected" without guest agent) + argv forwarded (was dropped) + `api.Execer`/`sshgw.Execer` unified signature + 2 protocol tests
- [ ] Interactive `/console` (non-interactive placeholder today); `GET /vms/:id/logs` persisted (`vm_logs` exists — wire runtime → store)

### G2 Build pipeline (PARTIAL 2026-09-19)
- [x] Digest pinning wired: `RootfsSHA256/KernelSHA256/ValidatedAt/Status` filled at build completion + logged + broadcast (SBOM/sign/scan stay open)
- [ ] `buildkit` builder → OCI transform → guest prep chain (builder exists, OCI→rootfs helper exists, unwired end-to-end)

### G3 Health in loop (DONE 2026-09-19)
- [x] Health probe in replica reconcile (unhealthy → failed → restart-policy replace); snapshot crash recovery in `reconcileFailed` (pre-existing; idempotency asserted by phase design)

### G4 Secrets + volumes at runtime (PARTIAL 2026-09-19)
- [ ] Scoped encrypted secrets → guest config at boot (at rest encrypted + project-scoped today; injection needs guest-agent binary in images)
- [x] Volumes independent of VM life, attach `/dev/vdb` at boot (`FCDataDrive` + `VMManager.Boot` lookup)

### G5 Events + audit spine (DONE store-side 2026-09-19)
- [x] Durable tables (migration 0018) + store spine (`AppendEvent/AppendAudit/CreateTask/UpdateTaskStatus`) + lifecycle feed in ReplicaController
- [ ] `event.read`/`audit.read` scoping on read paths

### G6 Per-resource RBAC (PARTIAL 2026-09-19)
- [x] Missing cap seeds added (migration 0018: replica.snapshot/restore, network.*, certificate.*, event/audit.read) + org-scope resolution in `granted()`
- [ ] Full per-route cap audit pass

### T11 E2e + T12 audit (PARTIAL 2026-09-19)
- [x] PG-gated harness `backend/tests/e2e_test.go` (5 tests: RBAC round-trip, deny-wins, event/audit/task spine, idempotency replay, network/compute rows; skips without `PORTER_TEST_DATABASE_URL`)
- [ ] Full-path run on KVM host + PG; security/tenancy acceptance beyond unit scope
- [ ] Per-feature ARCH §37 audit + truthful labels (see docs/completion-audit.md §8)

## 2. Decisions (binding, from the read)

1. vmEngine stays as executor; controllers own the loop (T7a resolved — no rewrite).
2. 0007 bridged, never replaced (plan §2).
3. Controllers stay on `types.*`; canonical new types in `resource.*` (T7b = constant unification only).
4. One allocator after T6b; `netmgr` owns host TAP creation, `runtime` consumes BootSpec.
5. `firecracker/` empty dir: DELETE when T7 lands (runtime is home).
6. Billing pkgs stay empty until subscriptions ship; meters may be write-only earlier.
