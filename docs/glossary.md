# Glossary — one term, one meaning (Porter)

> Normative vocabulary. Use exactly these terms in code, docs, API, and UI.

| Term | Meaning |
|---|---|
| Desired state (`spec`) | What the user asked for. Persisted in PG. Controllers converge toward it. |
| Observed state (`status`) | What the infrastructure last reported. Never written by users. |
| Generation / observedGeneration | Monotonic desired revision vs last-reconciled revision. Equal = converged. |
| Condition | Typed state detail (e.g. `Ready`, `Provisioning`, `Degraded`) with reason + timestamp. Never a bare "failed". |
| Reconciliation | The loop `Desired → Observe → Diff → Plan → Act → Verify → Status → Repeat`. |
| Draft | A pre-confirmation reservation (name + capacity held) kept until the runtime confirms or 404s. |
| Placement | Scheduler decision (node for a replica), persisted before provisioning. |
| Capacity sample | One host capacity report at a moment; placement subtracts newer reservations. |
| Rollout weight | Traffic share 0→100 for a deployment during health-gated promotion. |
| Artifact | Immutable build output pinned by digest (OCI image + rootfs + kernel + config). |
| Guest transform | OCI layers → rootfs + `/sbin/porter-init` + kernel (explicit, never direct boot). |
| Warm artifact | Host-local disk+memory+FC state for one exact image+shape (boot skip, never uploaded). |
| System image | Shared base boot artifact for all tenants (pinned, versioned). |
| Machine image | Tenant-isolated image captured from a VM disk. |
| Snapshot staging | Temporary host data for transfer to object storage. |
| Egress (`uplink/mesh/none`) | Internet+mesh / mesh-only / nothing. Independent of mesh reachability. |
| Privileged tenant (0) | System tenant whose VMs may cross tenants via mesh whitelist. |
| Capability | Atomic permission `namespace.action` (vocabulary, rarely changes). |
| Role | Named set of capabilities (may be team-custom, never mints new strings). |
| Scope | Tree node a grant attaches to (`reseller/org/team/project/environment`); inherits down. |
| `AuthContext` | Verified caller `{principal_type, principal_id, scope, claims}` per request. |
| Task / operation | Durable long-running work (QUEUED/…/NEEDS_ATTENTION) with progress + logs + retry. |
| Event | Versioned fact `{name,version,payload,scope,occurred_at}` + ids/correlation/actor; feeds audit+meters. |
| Entitlement | Plan allowance gating admission/quota (e.g. `max_vcpu`, `custom_domains`). |
| Disruption budget | `minAvailable` replicas enforced during drain/upgrade/reschedule. |
| Preview | Ephemeral per-PR environment (own domain/vars), TTL + auto-destroy. |
| Ephemeral VM | `ephemeral:true` MicroVM: disk discarded on stop, scale-to-zero allowed. |
