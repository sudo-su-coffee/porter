# Testing & acceptance — matrix, e2e harness, completion rule

> Gate: `cd backend && go build ./... && go test ./...` + `go vet ./...` green after every task.

## 1. Per-subsystem matrix (every feature)

Unit + integration + failure-path + idempotency + authZ. Runtime/net/scheduler add Linux
integration on supported hosts (KVM/Firecracker/TAP/bridge). Existing suites to extend:
`api_test|coverage|rbac_test.go`, `store_test|migration_test.go`, `fc_test|mode_test|socket_test.go`.

## 2. E2e harness (T11, API-driven, ARCH §36 path)

Install → register node → KVM/FC preflight → create project → connect GitHub → build →
artifact → schedule → provision MicroVM → net → start → health → gateway → domain → TLS →
live traffic → observe → rollback. Every step asserts resource/task/event state (store-backed;
mock host ops where KVM absent). Security/tenancy acceptance alongside: quota enforcement,
secret isolation (values never in logs), audit generation, RBAC inheritance + deny.

## 3. Critical 18 (SRS §62 — all must pass)

Enrollment, preflight, VM create, restart/recovery, net attach, Git build, artifact prep,
deployment, health, gateway, domain/TLS, rollback, node failure, restart-reconcile, quota,
secret isolation, audit, backup/restore.

## 4. Completion rule (ARCH §37 — per feature, truthful label)

API contract + authZ + persistence + controller/workflow + real infra op + status + events +
audit + failure handling + retry/idempotency + tests (+ UI/CLI where applicable).
Labels: `IMPLEMENTED / PARTIAL / EXPERIMENTAL / PLANNED` (ARCH §38) — never present PLANNED as shipped.
T12 audits every MVP feature against this list.
