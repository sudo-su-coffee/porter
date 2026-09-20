# ADRs — decisions already made (do not re-litigate without a new ADR)

## ADR-1: Go for the control plane (not Rust, not PHP)

Polyglot author (Go/Python/Rust/Java/PHP); Laravel → Go → Rust → Go path. Go chosen for
flexibility + deeper Firecracker control (direct FC API, jailer/seccomp, TAP/vsock,
snapshot/MMDS) without fighting Rust VMM internals. Vyoma (Rust) = inspiration, reimplemented.
SiteKit (Laravel) = UX reference only. Status: ACCEPTED.

## ADR-2: Firecracker = isolation boundary (never Docker-as-runtime, never K8s-as-plane)

OCI images are artifacts; explicit guest transform required. BuildKit builds, never runs
customer workloads. K8s concepts adapted (HPA/namespaces/PDB/cordon), K8s itself never adopted.
Status: ACCEPTED (SRS §67).

## ADR-3: PostgreSQL = sole durable truth; Redis = optional acceleration

Controllers/schedulers read+write PG; critical state recoverable from PG + observation alone.
No component owns another's state. Status: ACCEPTED.

## ADR-4: One small Go binary, not a daemon zoo

`porter` (+ thin CLI) unless a split has concrete security/reliability/scale/lifecycle
justification. Boundaries > directories. Status: ACCEPTED.

## ADR-5: One RBAC-ed REST for UI/CLI/Terraform/AI; no superuser paths

`HasCapability` + deny-overrides-allow + RLS. AI = governed operator (typed tools, plans,
approvals, audit). Extensions out-of-process, no direct PG. Status: ACCEPTED.

## ADR-6: AGPL references stay patterns-only

Atlas/Central/press/pilot/torii = separate process or reimplementation; Nomad MPL = executor
only; Vyoma = Go reimplementation. Status: ACCEPTED.

## ADR-7: Honest status + completion rule

Every feature labeled IMPLEMENTED/PARTIAL/EXPERIMENTAL/PLANNED; complete = contract + authZ +
persistence + controller + infra op + status + events + audit + failure + retry + tests.
Status: ACCEPTED.
