# Data model (living) — Postgres schema, migrations, RLS, seeds

> Living doc (replaces archived `db-rbac-model.md` as the working contract).
> Conventions: `uuid/text/timestamptz`, pgcrypto secrets, FKs, RLS on tenancy columns.

## 1. Migration history (`backend/migrations/`, up/down pairs, `migrations.go` runner)

| Migration | Tables |
|---|---|
| 0001 v1 core | projects, services, vms, domains, volumes, ssh_keys, servers, users, secrets, deployments, networks, golden_images, metrics_samples, health_events |
| 0003 v3 system | orgs, org_members, groups, group_projects, project_settings, project_members, builds, stacks, api_keys, alerts, hooks, crons, drains, redirects, firewall_rules, environments (+ re-affirmed dns_records, build_logs, golden_images, metrics_samples, health_events, servers, deployments, secrets, networks, volumes) |
| 0005 full paas | daemon_logs, traffic_logs, vm_logs, analytics_daily, daemon_logs_history |
| 0007 rbac | roles, permissions, role_permissions |
| 0008 deployment_checks | (checks contract) |
| 0009 feedback | feedback |
| 0010 server_cluster | (cluster) |
| 0011 user_email | (email) |
| 0012 seed_rbac_admin | admin seed |
| 0013 image_artifact_contract | (image/artifact) |
| 0014 build_log_stream_contract | (log streaming) |
| 0015 seed_super_admin | super-admin seed |
| 0016 image_upload_permission + versioned_deployments | (upload perm + versioned deploys) |
| **0017 PLANNED (T1)** | `capabilities, role_capabilities, role_assignments, resellers, teams, ip_allocations, micro_vms` — NO billing tables |

Total today: 41 tables, PARTIAL vs SRS §60 minimum.

## 2. Tenancy tree (authoritative)

```
Platform → Reseller? → Organization (orgs) → Team (groups/teams) → Project (projects)
  → Environment (environments) → Services → Deployments → Replicas → MicroVMs (vms/micro_vms)
```

Infra dimension (separate, cross-linked): Provider → Region → Zone → NodePool → Node (servers) → MicroVM.
Every row carries its scope node. Grants inherit downward; **deny overrides allow**.
`memberships`: org_members + project_members (+ team links via groups).

## 3. RBAC tables (0017 shape)

`capabilities(key UNIQUE e.g. workload.create, category, description)` →
`role_capabilities(role_id, capability_id, effect allow|deny)` →
`role_assignments(principal_type user|service_account|agent, principal_id, scope_type, scope_id, role_id, granted_by)`.
Resolution: `HasCapability` = union of caps at scope + ancestors, deny wins.
Seed catalog (compute/network/identity/billing/ops per `atlas-central-rbac-flows.md` §2)
+ system roles (`platform_admin, infra_operator, billing_operator, support_operator,
reseller_admin, org_admin, org_developer, org_reader, service_account, ai_agent, extension`)
+ bootstrap `platform_admin` (first-user-is-admin). No role strings in handlers.

## 4. Compute / network / storage / build rows

- Compute: services (environment_id, kind, image_ref, desired_replicas, spec desired) →
  deployments (revision, status, build_id, spec) → replicas (desired/observed node,
  observed_state, healthy) → micro_vms (replica_id, node_id, state, cpu/mem, target_state).
- Network: networks (environment_id, cidr, ipam) + `ip_allocations` (network_id, ip, micro_vm_id) + domains + dns_records + certificates + firewall_rules + redirects + drains.
- Storage: volumes (environment_id, size_gb, class, state — independent of VM life) + snapshots + backups + object config.
- Build: builds (service_id, status, oci_ref) → artifacts (oci_ref, digest immutable) + build_logs (stream contract).
- Ops: events (name, version, payload, scope, occurred_at — durable spine) + tasks + alerts + incidents + audit_events + daemon/vm/traffic/analytics logs + feedback.

## 5. RLS (defense in depth under API RBAC)

Enable RLS on all tenancy-scoped tables keyed to a session `scope_org` set from the validated
token (`AuthContext`). API enforces first; RLS re-enforces at query layer (500-tenant case).
`platform_admin` bypass only. Tests must assert cross-tenant reads return 0 rows at SQL level.

## 6. Deferred (do NOT create until the feature ships)

Billing (`products/plans/prices/entitlements/subscriptions/usage_events/meters/charges/invoices/payments/credits/refunds`), per honest-status rule. Meters may be WRITTEN (`usage_events` insert-only) before rating exists.
