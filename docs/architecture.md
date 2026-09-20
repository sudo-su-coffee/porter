# Architecture (living) — Porter control plane

> Living doc (replaces archived `ARCHITECTURE_FLOW.md` as the working contract).
> Normative companion to `SRS.md`. Update in the same PR as any structural change.

## 1. Topology

```
User / Admin / CLI / Terraform / Webhook / AI agent
                    ↓  (one RBAC-ed REST, no privileged side channel)
               Porter API  (backend/internal/api — ~300 routes, all behind a.auth;
                            public only: /health, /healthz, /version, /auth/*, /images/ml alias)
                    ↓
     AuthN (session/JWT/opaque key) → AuthZ (HasCapability) → RateLimit
                    ↓
     Quota/Entitlement → Admission/Policy → Validate → Defaults
                    ↓
     Persist desired state (PostgreSQL — sole truth)
                    ↓
     Task/Event (durable op + versioned event)
                    ↓  return resource + opID (async for infra mutations)
     Controllers (Deployment/Replica/Node — reconcile loops over store)
                    ↓
     Scheduler (filter → score → persist placement)
                    ↓
     Runtime (FCClient per-VM unix socket) → Agent (privileged host ops)
                    ↓
     Linux / KVM / Firecracker (jailer/seccomp, TAP/vsock) → MicroVM → workload
```

One compiled Go binary (`cmd/porter`: server|worker|kernel|version) + thin CLI
(`cmd/porter-cli`). No daemon zoo: a new process only for a real security/privilege/
reliability/lifecycle boundary. Redis (optional) = cache/queue/coordination only.

## 2. Component ownership (`backend/internal/*`, 43 pkgs)

| Package | Owns | Must NOT own |
|---|---|---|
| `api` | versioning, validation, task creation, serialization | business logic beyond admission |
| `auth` | sessions, JWT/opaque validation, `AuthContext` | permission strings (delegates to `rbac`) |
| `rbac` | `HasCapability(principal, cap, scope)`, inherit + deny | route wiring (done in `api`) |
| `resource` | canonical domain types + `spec/status` contract | persistence (that's `store`) |
| `types` | API/store projections (`VM`, `Project`, 41 types) | canonical truth (that's `resource`; unify per T7b) |
| `store` | Postgres access + `Migrate` on boot | orchestration |
| `controller` | desired→actual convergence, retry, recovery | in-memory progress (must re-list on restart) |
| `scheduler` | capacity, placement, constraints, replicas | execution (runtime/agent do that) |
| `runtime` | `Manager` (Replica) + `VMManager` (VM) over shared `FCClient`; `NetworkManager` allocator | `netmgr`'s divergent allocator (unify per T6b) |
| `firecracker` | (empty today — FC logic lives in `runtime`; keep or collapse deliberately) | — |
| `agent` | authenticated host bridge: FC lifecycle, net/storage/build exec, telemetry | policy decisions |
| `network` + `netmgr` | abstraction + IPAM/TAP/bridge/NAT/policy (UNIFY — two allocators today: `10.42/16` vs `172.16/16`) | — |
| `gateway/dns/tls/certificate` | ingress, records, ACME, cert lifecycle | edge-provider lock-in (Cloudflare optional) |
| `storage/volumes` | classes, block attach/detach/resize, snapshots | workload liveness |
| `build/buildkit` | build execution → immutable digest artifacts | running customer workloads |
| `deployment/imagecatalog/compose` | rollout records, image catalog, stacks | runtime |
| `health` | checks consumed BY reconcile (not beside it) | — |
| `event` | versioned hub + durable store | secrets in payloads |
| `observability/metrics/logging` | FC metrics/logs consumption, traces | durable truth (PG event store is truth) |
| `policy/billing` | admission, entitlements/quotas; billing emits actions, never kills VMs | direct host ops |
| `workflow/cron/deployment` hooks etc. | durable tasks, schedules, triggers | — |
| `ai/marketplace` | typed tools + approval plans; out-of-process extensions | superuser paths |

## 3. Request lifecycle (normative)

```
Client → API → Authenticate → Authorize → RateLimit → Quota/Entitlement →
Admission/Policy → Validate → Defaults → Persist desired → Task/Event →
return resource + opID → async controller → status/events → stream/poll
```

Sync reads return directly. Infra mutations are async. Controllers crash-safe:
desired survives in PG, actual stays observable, restart re-lists and converges.

## 4. Reconcile contract (every controller)

`Desired → Observe → Diff → Plan → Act → Verify → Status → Repeat`.
Actions idempotent (duplicate/delayed events, agent reconnects, process crashes safe).
`spec`/`generation` (desired) vs `status`/`observedGeneration` (observed); equal =
converged. Partial failure → condition + retry-if-retryable + idempotent cleanup.

## 5. Failure handling (normative)

Controller crash → restart → list pending → reconcile. Node heartbeat lost → Unhealthy →
stop placement → disruption-budget → replacement capacity → recreate → net/volumes →
health → traffic → audit. Privilege failures → explicit conditions (never generic
"failed"). Billing never executes host ops (emits policy actions through provisioning).

## 6. State ownership (invariant)

```
PostgreSQL → durable truth | Redis → optional accel | Porter → desired/orchestration
Agent → host bridge | Firecracker → VM runtime | Linux → kernel primitives | BuildKit → builds
```

No component owns another's state. Secrets encrypted at rest, never in logs/traces/events/audit.
