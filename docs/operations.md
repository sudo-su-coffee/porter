# Operations runbook — install, doctor, nodes, backup, incidents

> Practical companion to `docs/firecracker-manual.md` §9. Commands are `porter`-centric;
> adapt flags to the built binary (`backend/Makefile`: `make build|run|dev|test|vet`).

## 1. Install (single host)

1. Provision Linux (6.18, KVM, cgroups v2) → 2. install `porter` binary + Postgres →
3. `DB_URL=… ./bin/porter server` (runs `store.Migrate`) → 4. bootstrap admin (first user) →
5. `porter doctor` all-green → 6. enroll self as node → 7. deploy first project (C1 flow).

## 2. `porter doctor` (deterministic, AI-optional)

Checks: OS/arch/kernel/KVM/cgroups/FC+jailer/BuildKit/PG/agent/TAP/bridge/forwarding/NAT/IPAM/
gateway/DNS/TLS/storage/scheduler/capacity. Each returns
`check/status/severity/observed/expected/cause/action/evidence`. P1 extension: safe auto-repair
(agent restart, config re-push, mesh re-register, route re-attach) with audit.

## 3. Node lifecycle

Enroll: token → `POST /servers` → OS/arch → KVM → FC → BuildKit → net → storage → keypair →
verify → capabilities → READY. Operate: `sync_state` per node; cordon (stop placement) → drain
(budget-checked recreate + volume reattach + health + traffic) → upgrade/reboot → resume.
Failure: heartbeat lost → Unhealthy → replacements → net/volumes → health → traffic → audit.

## 4. Deploy / rollback / backup

Deploy: build → digest artifact → guest prep → schedule → provision → net → start → health →
gateway → domain → TLS → healthy (stages visible; failed build never replaces running).
Rollback: prior digest + current config (+ volume restore as separate explicit step).
Backup: engine-aware dumps + volume/snapshot schedules (cron/named) → local + S3 copies,
independent retention; **restore-to-staging test required before trusted**; restore-to-new supported.

## 5. Incidents

Alert → incident → timeline → plan → approval → execute → verify → postmortem. Keep: runbooks for
proxy-quorum, DNS, Atlas→Metal-equivalent (API→runtime) faults, generation mismatch, unknown/failed
VMs, stuck servers, stale capacity, pending IPs, stuck images/uploads. Console via vsock (recorded).

## 6. Maintenance & danger zone

Windows (announce → cordon → drain → work → verify → resume), upgrades, drains, recovery.
Destructive ops require: confirm + RBAC + audit + dep-check + impact preview + backup option +
rollback plan (+ window). Bulk ops use the same policy/auth/rate-limit/audit as single ops.
