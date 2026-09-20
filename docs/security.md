# Security model — boundaries, auth, isolation, audit

> One place for what today is scattered across 4 docs. Read before touching auth,
> permissions, tenant boundaries, secrets, network, console, or extensions.

## 1. Trust boundaries

Porter trusts: control-plane code, enrolled agents (scoped tokens), Postgres, object storage.
Porter does NOT trust: API callers, guest workloads, proxy viewers, extensions.
`/api/*` (single REST) is the only tenant surface. Nodes/creds/bare-metal stay operator-only.

## 2. Authentication

Password + sessions, API keys (hash-stored, scoped, revocable), service accounts, machine/node
identities (cryptographic enrollment, never IP/HW-derived), agent identities. Future: OIDC/SAML.
Planned: JWT EdDSA+JWKS + opaque keys (`AuthContext{principal_type, principal_id, scope, claims}`),
`X-Tenant-ID` (`"*"` = central), first-user-is-admin seed. No private creds in binaries.

## 3. Authorization (single model, all surfaces)

UI, CLI, Terraform, webhooks, workflows, AI, extensions — all resolve via
`HasCapability(principal, capability, scope)`: union at scope + ancestors, **deny overrides allow**.
Capabilities (`namespace.action`) seeded per §2 of `atlas-central-rbac-flows.md`; roles are named
sets; assignments are the only edge. No hardcoded role strings. System roles seeded; custom roles
recombine vocabulary only. Defense in depth: API RBAC + Postgres RLS.

## 4. Workload isolation

Firecracker MicroVM per tenant (jailer + per-instance uid/gid + cgroups v2 + seccomp) +
host egress/IMDS filtering + serial off + SMT/KSM/swap off. Workloads NEVER receive: PG access,
host/FC control sockets, agent creds, other-tenant volumes, unrestricted host net. Egress modes
`uplink/mesh/none` per workload. Secrets encrypted at rest, scoped injection, rotation/versioning,
values NEVER in logs/traces/events/audit (sensitive ops emit audit without values).

## 5. Console / exec / network policy

Exec/console over vsock guestagent (`vm.console` cap), recorded transcripts (audited), no SSH.
Network policy/SGs/firewall per scope; rate limits per principal + per-interface token buckets.
TLS everywhere on the API; ACME for customer domains (http-01/dns-01); expiry monitored.

## 6. Supply chain + extensions + AI

Builds: SBOM/provenance/signing/scan + admission policy; digest-pinned artifacts only.
Extensions: no direct PG, out-of-process, scoped caps, signed manifests. AI: same API/RBAC/audit,
typed tools only; arbitrary shell = explicit privileged cap; non-trivial changes = plan +
risk/cost/perms/approval/verify/rollback. Safety: READ auto / LOW-RISK policy / PROD approval /
DESTRUCTIVE explicit approval. Impersonation: request→policy→consent→time-box→banner→audit→expiry.

## 7. Accepted risks (explicit, revisit before prod)

Bearer tokens replayable until expiry (keep TTLs short: login 5m, enroll 30m, metrics 7d).
Signed URLs are bearer caps (24h image URLs, 60s console tokens). Quotas enforced at admission
(P0 addition — upstream Atlas has none). Billing enforcement deferred (suspend path shaped, tables later).
