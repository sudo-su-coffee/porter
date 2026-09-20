# Atlas + Central RBAC & flows → Porter (bench excluded)

> Distilled 2026-09-18 from `D:\github\atlas` + `D:\github\central` (both AGPL — patterns only).
> Scope: RBAC + control/runtime flows. Bench/pilot deliberately excluded.
> Porter reimplements all of this natively in Go + Postgres + Firecracker.

## 1. Atlas API auth (`atlas/auth/`)

- `token.py TokenValidator.claims()`: `kid` → issuer via `region_id`; `alg == EdDSA` only;
  JWKS `trusted_keys().key(kid)`; `jwt.decode(audience=admin_audience_id, issuer,
  require=[iss,sub,aud,scope,tenant,iat,exp])`. `_has_atlas_authority`: `scope=="*"`
  + `constraints=={}` + int `iat/exp` (+ optional int `nbf`). Else `None` (fail-closed).
- `identity.py AtlasIdentity{subject,issuer,tenant,scope}` (frozen slots). `tenant=="*"`
  = central; else uint32 `parse_tenant_id`. `get_current_tenant_id()`: central callers
  MUST send `X-Tenant-ID`; scoped callers' header must equal token tenant.
- `overrides.py`: `TENANT_DOCUMENT_TYPES = {Metal Server IP Address, Virtual Machine,
  Virtual Machine Image}`. `get_permission_query_conditions`: no tenant → `""` if no
  identity + System Manager else `1=0`; Migration → subselect on VM tenant; Image →
  `(tenant=X OR type='system')`; else `tenant_id=X`. `has_permission` mirrors it.
  `roles.py`: `has_role` only; `System Manager` = sole bypass.
- Porter: `AuthContext{principal_type,principal_id,scope,claims}`; EdDSA/JWKS + opaque
  keys; `X-Tenant-ID`; `HasCapability()` (never role strings in handlers); scoped list
  filtering; `platform_admin` sole bypass; Postgres RLS second layer.

## 2. Central IAM (`central/iam.py`, `permissions.py`, `CAPABILITIES.md`)

- Vocabulary (13, `resource:action`, versioned `CAPABILITY_VERSION=4`): central(5)
  `billing:view/manage, team:edit/manage_members/delete`; atlas(8) `cluster:view,
  server:view/create/power/resize/snapshot/terminate/open`; bench(0, deferred).
  Custom roles recombine vocabulary only — never mint new strings.
- Implications (`expand_capabilities`, closed before assert/eval):
  `open→view`, `create→view+cluster:view`, `power/resize/snapshot/terminate→view`.
- Resolution: 4-table join `Team Member→Team Role→Role Capability→Capability`
  (request-cached `resolve_user_grants`), one role per member per team, no per-user
  overrides, personal team per user, Owner non-transferable except via transfer.
  Roles: Owner 13, Admin 12, Developer 8, Viewer 2, Billing 4.
- `can(user,team,cap)`; `get_fc_teams_claim` → `fc_teams` OIDC claim
  (`{team:[{role,source,scope:"*",caps}]}`). `permissions.py`: `team_query_conditions`
  (`Team.name in (teams)` / `1=0`), `_team_field_query_conditions` (`tabX.team in
  (teams-with-cap)`), `has_permission` splits read-caps vs `MUTATING_PERMISSION_TYPES`.
- Porter: `capabilities` table (versioned, implication-closed, **deny overrides allow**);
  `roles` + `role_assignments(principal,role,scope)` inherit-down; per-grant `scope`
  deferred `"*"` (same as Central) for future server-level scoping.

## 3. Atlas VM + placement flows (`atlas/vm/SPEC.md`, `metal_server/SPEC.md`)

- Create: `validate → lock Metal Server (capacity sample − reservations − drafts) →
  insert draft + COMMIT (reserves name/capacity) → PUT /v1/vms/{name} (idempotent,
  caller ID + first-request fingerprint) → clear draft`. Lost response = keep draft;
  only Metal `404` deletes it. Lock released at commit (slow host never holds row).
  Stale sample → sync fault, never guess.
- Reads: property = one Metal call/request (cached); draft=`pending`, absent=`unknown`,
  other errors raise. List = `last_known_state` from `Virtual Machine State` refreshed
  by `POST /v1/sync` per host (`state_synced_at`).
- Reconcile: 2 jobs (stale drafts, terminating VMs), one settle path, Metal authoritative.
  Terminate = `DELETE`, detach IPv4 (keep tenant reservation), keep record until absent.
- Metal Server: phased provision (create host w/ stable identity → wait → addresses →
  install Metal → WG → complete), resumable, retry reuses identity, compensation deletes
  only self-created host. IPv4 = intent + version + tenant; shared pool with locked claim.
- Porter: same draft→PUT→reconcile→sync shape over PG
  (`services→deployments→replicas→micro_vms`); agent heartbeat + capacity sample
  replaces `/v1/sync`; node enrollment replaces provision phases.

## 4. Metal contract (`docs/metal-v1-contract.md`, `docs/architecture.md`)

- JSON, bearer, complete field names, `mib/mibps/iops`, reject unknown fields/trailing
  data, persist-before-respond, never leak URLs/userdata/cmd output/paths/PIDs.
- `PUT /v1/vms/{id} 202` (create), `GET 200`, `power/restart/compute/disk/network 202`,
  `ssh-keys/metadata 200|202`, `DELETE 202`, `snapshots 201`, `upload 202`, `console` WS.
- Desired `{generation,restart_generation,state,compute,disk,
  image{ref,arch,rootfs/kernel sha256,cache_image,memory_snapshot},
  network{egress,public_ipv4,mesh_ipv6,throughputs},guest{hostname,ssh_keys,metadata}}`
  vs observed `{generation,state,phase,op_id,used_mib,mac,error}}`. Mutations replace
  complete objects (shape change needs stopped; disk never shrinks).
- Sync replaces peer/image/priv-addr sets + returns capacity + VM statuses + JWT rotation
  (fixed issuer/receiver, `409` on change).
- Images: `object → cache → disk`; `disk → staging → object → Machine image` (multipart,
  2 GiB parts, 24 h URLs, SHA-256 verified, 48 h staging TTL). System images shared;
  Machine images tenant-isolated. Warm snapshots host-local only, exact-shape only.
- Network: `uplink` (mesh+internet, IP allowed) / `mesh` (peers only) / `none`;
  throughput `mibps` (`0` = remove); one IPv4 per VM; MMDS for hostname/mesh-IP/metadata.
- Proxy: 1–5 regional VMs, IPv6, local route maps, merged JWKS, issuer-bound keys,
  scopes `site:*/domain:*/*` + name constraints; `tenant`-claim tokens refused.
- Tunnel/SSO: WG hub `10.88.0.0/16` dials spokes + auto-revert firewall + scoped machine
  users + 8-step Register → Active (Desk goes dark); SSO RS256+JWKS, `aud` =
  bench credential, TTL 5m/30m/7d, fail-closed.
- Porter: internal reconcile replaces Metal HTTP; gateway replaces proxy scopes with
  `route.manage`; enrollment replaces tunnel register; `AuthContext` replaces SSO mint.
