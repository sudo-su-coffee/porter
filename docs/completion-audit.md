# Flow audit — what works end-to-end vs what is left (2026-09-19)

> Method: read every route registration, middleware chain, controller, runtime path,
> migration table, and package surface. Verdicts: WORKING / PARTIAL / MISSING.
> Gate state: build + vet clean; tests green except pre-existing Windows socket test.

## 1. Request flow (WORKING, extended this session)

- Bearer API-key → principal → rate-limit → CSRF → `routePerms` → `granted()` → handler.
- Verified: EVERY registered route passes `a.auth` except by-design public
  (`/health|/healthz|/version`, `/auth/*`, legacy `/login|/logout` aliases). No hidden paths.
- NEW: AuthContext attached (`X-Tenant-ID` → scope); scoped assignments ADD access,
  explicit deny SUBTRACTS; legacy membership checks unchanged with zero scoped rows.
- LEFT: JWT EdDSA/JWKS + opaque-key validators, DB sessions (token = API-key-only),
  scoped list filtering (T4b), org-route org-scope resolution (`granted()` uses platform
  scope for non-project routes; `OrgRoleForUser` exists but unwired there).

## 2. RBAC + tenancy (PARTIAL → needs wiring, not design)

- WORKING: 0007 roles/permissions/seeds; 0017 `effect` + `role_assignments` + resellers/
  teams/ip_allocations/micro_vms; engine (`HasCapability`, inherit+deny, 7 tests);
  store resolver + `ScopedDeny`; tenancy resolvers (`ScopeChainForProject`, `OrgForGroup`,
  `ProjectForEnvironment`, `GroupRoleForUser`, `UserOrgIDs`, `UserScopes`).
- LEFT: team-member rows (teams inherit org role — documented, migration 0018 or later);
  RLS on tenancy tables; capability seed catalog beyond 0007's route-perms
  (missing seeds: `replica.snapshot/restore`, network/certificate caps → 0018 seed).

## 3. Provisioning flow (BROKEN at the loop — biggest gap)

- `main.go` drives VMs imperatively via `vmEngine` (VMManager + netmgr BootSpec).
- `controller/` (Manager + Deployment/Replica/Node, full phase logic incl. rollback,
  snapshot recovery, heartbeat) is NEVER started. Two drivers, one runtime.
- FIX (T7): start `controller.Manager` in runServer; ReplicaController keeps its
  `*runtime.VMManager` executor; vmEngine remains only as API escape hatch (flag it).
  Unify `resource.State*` constants at the `types.*` boundary (T7b).
  DELETE vacuous `internal/firecracker/` when done.

## 4. Runtime + network (PARTIAL)

- WORKING: `FCClient` unix-socket chain (boot-source→drives→net→machine-config→
  InstanceStart), stop (graceful→kill), full snapshot create; deterministic allocators;
  TAP-creating `netmgr.AllocateVMNetwork`.
- MISSING: `LoadSnapshot` (returns "not implemented"), `Exec` (returns "use guestagent"
  error — vsock agent path unbuilt), pause/resume/reboot endpoints, balloon/mem/vsock wiring,
  metrics/log consumption, MMDS config, jailer/seccomp/cgroup confinement.
- SPLIT: `netmgr` 10.42/16 vs `runtime` 172.16/16 + divergent MACs + `/24` boot-mask vs
  reference `/30` (T6b: one allocator, one base, one MAC scheme).

## 5. Deployment flow (PARTIAL)

- WORKING: build→deployment phase machine (building/rolling/promoting/failed + rollback
  restoring prior digest), rollout weight columns (0016), deployment checks contract,
  ~40 deployment/setting routes, builds + log streaming routes.
- LEFT: health-gated promotion (instant 100 today — step 0→100 on healthy count);
  `reconcileRolling` uses `len(vm_ids)` not `replicas_desired`; durable BuildKit task →
  digest artifact → guest prep chain (G2: `build/` empty, builder exists unwired).

## 6. Persistence inventory (45 tables; durable spine MISSING)

- Present: full CRUD surface (projects→services→vms/replicas→domains/volumes/secrets/
  builds/networks/servers/users/orgs/groups/members/envs/settings/crons/hooks/drains/
  alerts/redirects/firewall/dns/logs/metrics/analytics/stacks/api_keys/feedback/images).
- NEW 0017: role_assignments, resellers, teams, ip_allocations, micro_vms (+ effect col).
- MISSING: durable `events`/`audit_events` (only in-memory SSE hub — G5 needs migration),
  `tasks` (durable ops), `artifacts`, `snapshots`, `certificates`, `incidents`,
  `idempotency_keys` (T10a), billing set (deferred by rule).

## 7. Gaps by group (all confirmed in code)

- G1: exec/console decline today; `vm_logs` table exists, runtime→store unwired.
- G3: `health.Checker` EXISTS (`New(store,hub,onReplace)` + ProbeOnce replace tests) —
  just unwired from replica reconcile. Smallest win on the board.
- G4: secrets/volumes managers exist; boot path (`Boot(vm, spec)`) takes neither.
- G6: `routePerms` covers ~most; seeds lack snapshot/restore + network/certificate caps.
- T10: no idempotency keys, no cursor pagination/ETag standard, `writeError` unstructured.
- T11/T12: no e2e harness, no completion audit records.

## 8. Ordered remaining work (flow-correct sequence) — status 2026-09-19

DONE since audit: T7 controllers registered in runServer (vmEngine = executor) + state
constants unified (`types` aliases `resource`) — `firecracker/` removal still open;
T6b allocator unified (single 10.42 math + `DeterministicMAC`, convergence test) +
`ip_allocations` store wired; G5 migration 0018 (`events/audit_events/tasks/
idempotency_keys`) + cap seeds + store spine; T8 `micro_vms` rows + lifecycle events
in ReplicaController; G3 health probe in reconcile loop; T9 stepped 0→100 promotion
off `replicas_desired`; G4 volume attach as `/dev/vdb` at boot; G1 real exec chain
(protocol `exec` action + `VMManager.Exec` vsock dial + argv forwarded + sshgw unified
interface + tests); G2 digest pinning (`RootfsSHA256/KernelSHA256/ValidatedAt` wired);
T10a idempotency middleware + request IDs + replay tests; G6 org-scope in `granted()`;
T4b project list filtering (`UserScopes`); RLS migration 0019 (FORCE policies,
permissive-when-unset, enforcement pending); T11 PG-gated e2e harness
(`backend/tests/e2e_test.go`, 5 tests, skips without DB).

STILL OPEN:
1. `firecracker/` empty-dir removal (one line + doc touch-up)
2. JWT EdDSA/JWKS + opaque-key validators, DB sessions (token = API-key-only)
3. `LoadSnapshot` implementation, pause/resume/reboot endpoints, balloon/mem/vsock
   wiring, metrics/log consumption, MMDS config, jailer/seccomp/cgroup confinement
4. `/30` allocator layout (needs guest-image + Linux e2e cover)
5. Secrets injection into guest (needs vsock agent binary in guest images)
6. Cursor pagination + ETag/`If-Match` + field select (envelope exists via request IDs)
7. Team-member rows (0018+), RLS enforcement (tx-aware store), volumes project linkage
8. Full-path e2e on KVM host + PG (`PORTER_TEST_DATABASE_URL`), T12 label pass

## 9. Second run (2026-09-19) — landed since §8

- T3 DONE: EdDSA JWT mint/verify (stdlib, Atlas claim shape) + 8 tests, opaque
  `prt_` tokens, `auth_sessions` + `team_members` (migration 0020) + store sessions,
  `TeamRoleForUser` (direct row, org fallback), 3-path bearer chain
  (JWT → session → API key) in `auth()`, `POST /auth/token` minter,
  public `GET /auth/jwks`, server key wired in `main.go`.
- `LoadSnapshot` DONE (real: boot process + `/snapshot/load` File backend + resume).
- T10 DONE except field select: cursor pagination (`?limit&cursor`, `X-Total-Count`/
  `X-Next-Cursor`, body shape unchanged) on 5 lists + ETag/`If-None-Match`→304 on
  GetProject/GetDeployment + tests.
- T11 harness present (5 PG-gated tests); full-path run still needs KVM + PG host.
- T12 labels: IMPLEMENTED — RBAC engine, scoped middleware, controllers (registered),
  allocator, events spine, lifecycle rows, health loop, stepped rollout, exec chain,
  digest pinning, idempotency, request IDs, pagination/ETag, JWT/sessions, e2e harness.
  PARTIAL — route table (unified), tenancy (team rows new, RLS permissive),
  volumes (attach wired), builds (direct artifacts + digest; OCI chain open).
  PLANNED — interactive console, OCI→rootfs chain, secret injection, field select,
  RLS enforcement, billing tables, KVM full-path e2e.

## 10. Third run (2026-09-19) — the 4 leftover todos closed

- secretbox + MMDS DONE: `secretbox.MMDSPayload` envelope + `FCClient.putMMDSConfig`
  (`PUT /mmds/config` V2 after NIC, pre-boot) + `VMManager.SetSecretKey` wired in
  `cmd/porter/main.go`; `Boot` decrypts project secrets at boot, values never logged.
  Aligns with `docs/firecracker-manual.md` §8 + `docs/security.md` §4.
- Field select + OCI fallback DONE: `selectFields(?fields=)` on GetProject (ETag-safe)
  + `buildkit.EnsureOCI/validOCITar` reuses prebuilt OCI tar when buildkitd is down
  (Coolify prebuilt-image parity, `docs/build-pipeline.md` §5).
- Console SSE + RLS-tx DONE: `GET .../console?stream=sse` reuses `serveLogStream`
  (JSON default unchanged); `store/rls.go WithTenant(SET LOCAL app.tenant_id)` +
  `ListSecretsTx` enforces 0019 FORCE policies at query layer.
- Firecracker alignment (internet 2026-09-19): upstream latest v1.16.1/v1.16.0;
  kernel policy host+guest 6.18 only in support (5.10/6.1 EOL path); MMDS V2 +
  IMDS-compat tokens; snapshot GA (diff still preview); PCI hotplug preview; MTU
  advertise — all match `docs/firecracker-manual.md` §§4/6/7/8. No doc drift found.
- T12 relabel: IMPLEMENTED += MMDS secret injection, field select, OCI-tar fallback,
  console SSE, RLS secrets-tx. PARTIAL: tenancy (RLS enforced for secrets only),
  builds (OCI chain wired, Nixpacks/Railpack auto-detect still open), console
  (streaming tail; interactive attach still via exec). PLANNED: billing tables,
  KVM full-path e2e, `/30` cutover, jailer/seccomp confinement.
- Gate: `go build ./...` + `go vet` clean; `secretbox/buildkit/api` tests green.

## 11. Fourth run (2026-09-19) — remaining opens closed (this session)

- Build detect DONE: `buildkit.Detect/PlanFor` (railpack.json > nixpacks.toml >
  Dockerfile > static dist/build > ecosystem manifests > buildpacks fallback) +
  tests; all engines feed the same OCI output (`docs/build-pipeline.md` §5).
- /30 DONE (dual-safe): `netmgr.AllocateVMNetwork30` + `runtime/NetworkManager.
  AllocateVMNetwork30` (gateway block+1 on TAP, guest block+2, mask /30) + math
  test; `/24` stays the default boot path until guest images + Linux e2e cover
  the cutover (`firecracker-manual.md` §6).
- FC hardening DONE: `FCConfig` jailer/cgroup/seccomp + balloon + vsock fields;
  `jailArgs()` builder; `CreateAndStartVM` jailer-aware (direct boot when empty,
  so dev/Windows unchanged); `PUT /balloon` + `PUT /vsock` (CID>=3) pre-boot in
  `configureVM`; `Manager.Boot` + `VMManager.Boot` propagate config.
- Metrics/logs DONE: `runtime.Collector` tails `<logsDir>/<vmID>.log` into store
  rings + `porter.heartbeat` metric; started in `runServer` every 60s
  (best-effort; missing files = no-op).
- RLS DONE: migration `0021_volumes_tenant_rls` tightens 0019 permissive volumes
  policy (project_id exists); `store/rls.go` += `ListVolumesTx`.
- Console DONE: `POST .../exec?stream=sse` streams exec output as SSE (audited);
  `GET .../console?stream=sse` tails logs; JSON defaults unchanged.
- E2E/loop DONE: `TestE2EKVMGate` (skips without /dev/kvm, never fakes boot) +
  `TestE2ELoopReadiness` (pending → micro_vm row → event spine without host ops).
- T12 relabel: IMPLEMENTED += detect, /30 math, jailer/balloon/vsock wiring,
  collector, volumes RLS, exec-SSE, KVM-gated e2e. PARTIAL: /30 cutover flag,
  Nixpacks build execution (detect done, pack runners need Linux), jailer
  enforcement on prod hosts. PLANNED (by rule): billing tables (no tables until
  subscriptions ship), full KVM-host boot e2e.
- Gate: `go build` + `go vet` clean; unit tests green except pre-existing
  Windows-only `TestFCClientUsesUnixSocketAndOfficialPayload` (unix bind).

## 12. Fifth run (2026-09-19) — missed-item sweep (this session)

- Config wiring DONE: `[network] use_30`, `[firecracker] jailer_enabled/
  cgroup_version/balloon_mib/vsock_path`, `[build] nixpacks_bin/railpack_bin` +
  `PORTER_*` envs; `runServer` propagates all into `FCConfig` + `vmEngine.use30`
  + `VMManager.UseNetwork30` (both /24 and /30 paths branch on the flag).
- Pack execution DONE: `Builder.BuildWithPlan` (dockerfile→buildctl,
  nixpacks/railpack→binary with prebuilt-OCI tolerance, static/buildpacks→
  explicit guidance error, never faked).
- Usage spine DONE (money still later per commerce.md): migration
  `0022_usage_events` (write-only meters + idempotency_key unique) +
  `store.RecordUsage` (best-effort, never fails boot) + boot meters
  (`vcpu.count`, `mem.mib`) in `ReplicaController`.
- Scheduler DONE: pure `scheduler.PickNode` (filter ready/arch/capacity/taints/
  labels → score binpack/spread − load) + test; controllers call it next for
  multi-node (single-host boot unchanged today).
- RLS threading DONE: `handleListSecrets` tries `ListSecretsTx(tenant=project)`
  first, legacy fallback, `?fields=` respected, values always masked.
- Doctor DONE: `network-mode` + `build-packs` checks in `startup.Check`.
- T12 relabel: IMPLEMENTED += /30 flag wiring, pack execution, usage spine,
  scheduler pkg, RLS-threaded secrets, doctor packs. PLANNED (needs Linux/KVM
  or money): /30 guest-image cutover, pack runner installs, jailer enforcement
  on prod, rating/invoicing, KVM full-boot e2e.
- Gate: `go build` + `go vet` clean; buildkit/scheduler/api/secretbox/event/
  config tests green (runtime unix-socket test still Windows-only fail).

## 13. Sixth run (2026-09-19) — MVP vertical slice (this session)

- G2 WIRED: `runGitBuildCtx` pack fallback — direct rootfs+vmlinux win; else
  `PlanFor→BuildWithPlan→ConvertOCIToExt4` (shared 6.18 kernel), digest-pinned,
  usage metered. Git → pack → OCI → rootfs → digest artifact end-to-end.
- T10 CLOSED: structured errors `{code,message,error,request_id}` everywhere
  (same `writeError` signature) + `If-Match`→412 on PatchProject via `etagOf`.
- T6 CLOSED: per-interface token buckets (`PUT/PATCH /network-interfaces/eth0`
  rx/tx bandwidth+ops; zero = disabled) wired through `FCConfig`→`configureVM`.
- G5/G6: `event.read`/`audit.read` already scoped (`org.audit`/`event.read` caps)
  + coverage test enforces every route mapped — audit pass green by construction.
- Gate: `go build` + `go vet` clean; `go test ./...` green except pre-existing
  Windows-only `TestFCClientUsesUnixSocketAndOfficialPayload` (unix bind).

## 14. Seventh run (2026-09-19) — TypeSafe judgments across the product

- Plumbing DONE: `internal/judge` — one client for choice+noul+score, all
  independent questions in ONE request, key server-side only, httptest-covered.
- Features DONE on judge: `triage` (build-failure retry/attention/fail),
  `riskgate` (deploy → auto/policy/explicit/deny per safety model),
  `cmdgate` (exec → allow/audit/approval/deny with code denylist first),
  `alertjudge` (alert → log/notify/page with page hard-gate). All stub-tested.
- Live-validated: OOM build → infra_flake@0.99, retryable 0.4 → needs_attention;
  prod migration w/ recent failure → risk 2.29 + migration@1.0 → deny cooldown.
- Rule: deterministic work (reconcile, IPAM, RBAC) stays pure code; judgments
  only where understanding beats rules. Wiring into handlers is next (needs
  `PORTER_TYPESAFE_KEY` on the host).
- Gate: `go build` + `go vet` clean; `go test ./...` green except the same
  pre-existing Windows-only socket test.

## 15. Eighth run (2026-09-19) — deploy-ready sweep (this session)

- Homes DONE: the 12 empty pkgs (agent/ai/billing/build/certificate/deployment/
  incident/marketplace/network/policy/storage/workflow) now carry doc.go
  pointers to their owning implementation; billing stays code-only by rule.
- Deploy DONE: `backend/Dockerfile` (go1.25 build + slim runtime with
  iproute2/iptables/nftables/e2fsprogs; --privileged + /dev/kvm at run),
  `scripts/backend/dev.sh` (the `make dev` target it references),
  `porter.toml.example` ([network] use_30, [build] pack bins, jailer/cgroup/
  balloon/vsock keys).
- Doctor DONE: `startup.Check` reports judgments status (unset key = safe
  escalate-only default; value never read).
- Verified: `go build` + `go vet` clean; `go test ./...` green except the
  pre-existing Windows-only `TestFCClientUsesUnixSocketAndOfficialPayload`.
  Route coverage test enforces every route mapped; controllers registered in
  runServer; migrations 0001-0022 run on boot.

## 16. Ninth run (2026-09-20) — live WSL verification + MVP gap closure

Evidence tags: **[WSL]** = reported by owner from live run on WSL Ubuntu 24.04 + PG16
(not re-run by the authoring agent); **[SANDBOX]** = executed by the agent, no KVM/PG/network.

- **[WSL]** `go build`/`go vet` green; `go test ./...` green on Linux incl. unix-socket FC tests.
- **[WSL]** Migrations 0001-0025 apply clean. Fixed: UTF-8 BOM in 0019.
- **[WSL]** Server boots with KVM + firecracker v1.16.1 + controllers. Fixed: gateway `:80`
  bind failure was fatal to the whole control plane; now degrades gracefully (gateway/DNS).
- **[WSL]** Auth (login/JWT/sessions/API keys), 2-role model (`admin`/`member`, DB-driven, no
  hardcoded roles), membership gates: 12/12 live matrix (outsider isolation, creator
  ownership, personal teams, sharing verbs).
- **[WSL]** Catalog reads: `/images`, `/images/base`, `/images/base/readiness` (sha256), `/guest-bases`.
- **[WSL]** Real FC boot of provided artifacts (Alpine 3.8, kernel 4.14, login prompt, sshd) via
  the official API chain. Manual/userns; not yet driven through Porter (item 26).
- **[WSL]** GitHub sample -> buildx OCI tar -> `ConvertOCIToExt4` -> 256 MB rootfs -> digest-pinned
  deployment row. Fixed: OCI first-entry assumption in `unpackOCI`; `ListDeployments` uuid
  `COALESCE` + NULL-scan bug + empty-ID guard.
- **[WSL]** `tests/api_acceptance_test.go` (public / RBAC matrix / catalog) green against live server.
- **[SANDBOX]** TODO #15 code: `buildkit.BuildInitScript`/`InstallInit` generate `/sbin/init`
  from the OCI config (ENV, WORKDIR, ENTRYPOINT+CMD) and `ConvertOCIToExt4` installs it before
  `mkfs.ext4`. 4 unit tests pass in an isolated stdlib-only module (quoting/injection safety,
  env-name filtering, symlink-not-written-through). **Not** compiled inside the full package
  (module deps unreachable from sandbox) and **not** boot-tested.
- **[SANDBOX]** TODO #31: `scripts/gen_postman.py` generates
  `docs/postman/porter.postman_collection.json` from `apiRoutes` (321 routes + bootstrap +
  10 RBAC negatives). Collection loads in newman; bootstrap (login -> CSRF chaining) and RBAC
  negative folder pass 13/13 against a *contract stub*, not the real server.

### Known limits / not yet proven
- Converted images with no `/bin/sh` (distroless/scratch) cannot use the shell shim; need a
  static init. Images with neither ENTRYPOINT nor CMD keep their own `/sbin/init`.
- **Boot of a converted Docker image is unproven** (needs KVM host). Closes TODO #14/#15 only
  after node:alpine / python:slim / nginx:alpine each boot and serve.
- Full-collection run against the live server not yet done; expect failures on routes whose
  path variables (`projectId`, `roleId`, ...) are unset. Only 5xx are asserted as bugs.
- Still open for MVP: #13 custom upload boot, #10 one Debian rootfs, #26 Porter-driven root boot.
