# Plan additions — what Atlas/Central leave out (Porter planning, 2026-09-18)

> Status: PLANNING, not committed. Source: gaps found while distilling Atlas + Central
> (their accepted-risks + deferred-scope sections) + Dokploy/Coolify DX + Porter needs.
> Rule: small fields/checks now beat painful migrations later. Phased: MVP → G-groups → post-v0.1.

## P0 — fold into MVP slice (cheap now, painful later)

1. **Quotas at admission.** Atlas: none by design (cost control "belongs to Central",
   Central defers enforcement). Porter: per-scope hard ceilings
   (`max_vcpu/mem/storage/vms/ips/volumes/domains/projects`) checked in request lifecycle
   (AuthN → AuthZ → Quota/Entitlement → Policy → persist). Tables: `quotas(scope, limits)`.
2. **Scoped tokens from day one.** Atlas API is all `scope=*` (their future-work note).
   Porter: token = routes + resource + name constraint (proxy already proves the model).
   `api_keys(scope_type/scope_id)` + JWT `scope` claim enforced per route.
3. **Suspend/resume path (minimal).** Billing tables deferred, but the workflow shape is not:
   `ACTIVE → GRACE → SUSPENDED → (resume|terminate)`, storage preserved, compute stopped/
   restricted, every transition audited + policy-driven (billing emits actions, never kills VMs).
4. **Audit spine from the start.** Versioned durable events
   (`name,version,payload,scope,occurred_at` + id/correlation/idempotency/actor/source/tenant/
   resource/timestamp) feeding audit reads (`audit.read` cap). Every MVP task already owes it (ARCH §37).

## P1 — G-groups layer (already in task.md, sharpened)

5. **Verified backups (G4/G5).** Atlas snapshots = transfer artifacts, no rollback op, warm state host-local.
   Porter: snapshot-tree (parent-linked, delta-only) + TimeMachine point-in-time + auto-snapshot policy +
   scheduled restore-to-staging tests. Backup untrusted until restore verified; restore targets new resource.
6. **Deployment strategies (T9/G3).** Canary/blue-green/weighted + health-gated promotion 0→100 +
   auto-rollback to prior digest + pre-deploy (blocking, in old) / post-deploy (logged, in new) hooks.
   Dokploy zero-downtime + Coolify hook semantics.
7. **Supply-chain policy (G2).** Image signing (Ed25519) + SBOM/provenance/scan + admission policy gate
   (Atlas verifies hashes only). Digest-pinned artifacts; mutable tags never boot.
8. **Noisy-neighbor enforcement (G3).** Metal skips cgroup-IO; Porter adds cgroups v2 CPU/mem/IO per MicroVM
   + OOM-kill counts into health/scheduler signals, alongside FC token-bucket limiters (live PATCH).

## P2 — post-v0.1 (reserve fields now, build later)

9. **Per-VM ACLs + resource groups + reseller pools** (Central deferred). Reserve `scope` on grants
   (deferred `"*"` like Central) + `resellers` table now; enforce later. No migration if fields exist.
10. **Session grant refresh.** Central grants are session-stuck. Porter: short-lived caps + refresh +
    `cap_version` drift detection (bench/Central pattern) from the start.
11. **SLOs + synthetics + disruption budgets.** Neither ships them. Per-service SLOs (availability/latency/error),
    probes (DNS/TLS/HTTP/TCP/API), maintenance windows, drain-aware budgets for node ops.
12. **Exec recording + secret rotation.** Console tokens exist; add session transcripts (audited) + secret
    versioning/rotation with zero log leakage (values never in logs/traces/events/audit).
13. **Doctor-that-fixes.** Both have checks-only. Porter `doctor` returns
    `check/status/severity/observed/expected/cause/action/evidence` AND offers safe audited auto-repair
    (agent restart, config re-push, mesh re-register, route re-attach).
14. **GitOps + Terraform + preview envs.** Declarative manifests + provider (same RBAC/audit/idempotency as UI);
    `SOURCE_COMMIT` cache discipline; ephemeral preview environments per PR with TTL + auto-destroy.
15. **Cost visibility per team.** Even before billing ships: per-scope usage meters + forecast + breakdown
    (the invoice grain is `team|period`; meters exist from day one, rating later).

## Missed-field checklist (add to 0017+ migrations when touched)

- `quotas`, token `scopes[]/constraints`, subscription-lifecycle `service_status` on services,
  `snapshot_tree` (parent links), `restore_tests`, `rollout_weight/history`, `sbom/provenance/signature`,
  `cgroup_limits`, `reseller_id` on orgs, `grant_scope` on assignments, `cap_version`,
  `slos/synthetic_checks`, `exec_sessions`, `secret_versions`, `doctor_runs`, `previews`,
  `usage_events` (write-only from day one, rating deferred).

## What NOT to add (guardrails)

- No K8s-as-plane, no Docker-as-runtime, no Redis-as-truth, no daemon zoo, no hidden AI superuser,
  no unscoped extension PG, no mutable artifacts, no non-idempotent controllers, no silent escalation.
