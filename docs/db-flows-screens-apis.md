# DB + flows + screens + APIs: Atlas/Central → Porter

> Distilled 2026-09-18 from `D:\github\atlas` + `D:\github\central` (AGPL, patterns only).
> Companion to `atlas-features.md`, `central-features.md`, `atlas-central-rbac-flows.md`.
> Frappe/MariaDB (`tab<DocType>`, `name` PK) → Porter Go/Postgres (uuid/text/timestamptz,
> pgcrypto secrets, FKs + RLS, no Frappe tables).

## 1. DB — DocTypes / tables

### Atlas (14)

- `Atlas Settings` (Single): region/wildcard-domain/object-storage/proxy-password/signing-key/JWKS URL (`region_id`, `private_network_cidr 10.1.0.0/20`, scaleway creds).
- `SSH Task`: host-exec audit (`target,status,port,ssh_user,script,output,exit_code,started/ended_at`).
- `Metal Server`: provider host + lifecycle (`status,is_provisioning_completed,provider_server_id,metald_api_token,size/image,public/private_ipv4`).
- `Metal Server Size`: SKU (`provider_type,enabled,cpu_count,memory_mib,disk_gib,pricing_usd_cents`).
- `Metal Server Image`: catalog image (`provider_type,enabled,image,provider_metadata`).
- `Metal Server IP Address`: one IPv4 + intent (`address,server,virtual_machine,status,intent_version,tenant_id`).
- `Metal Server Disk` (child): block inventory (`device,uuid,mount_point,size_gb`).
- `Metal Server Usage`: capacity sample from `POST /v1/sync` (cpu/vm/mem/storage availables).
- `Virtual Machine`: reservation+request, name = Metal ID (`is_draft,is_terminating,active_migration,current/desired_state,server,image,vcpus/memory/disk/throughput`).
- `Virtual Machine Image`: boot artifact (`image_type System|Machine,enabled,status,title,platform,artifact_storage,rootfs/kernel+sha256+sizes,cache_image,memory_snapshot`).
- `Virtual Machine State`: last host status, name = VM name (`virtual_machine,status,synced_at`).
- `Virtual Machine Migration`: live move (`virtual_machine,status,abort_requested,source/target_server,progress`).
- `Proxy Server`: cluster member (`status,virtual_machine,provisioning_completed,dns_health,config_hashes,tls_expires_on`).
- `Cargo Server` (Single): regional S3 service (`status,virtual_machine,installation_task,failure_message`).

### Central (19 + ~43 billing + 7 services)

- Team graph: `Team` (name/owner/status/trial) + `Team Member` (user+role+scope+status) + `Team Role` (system vs own-team custom) + `Role Capability` (capability) + `Capability` (`capability,plane,resource,description`) + `Team Invitation` (email/role/scope/status/expiry) + `IAM Probe` (user/team/cap/allowed/grants debug).
- Mirrors: `Site` (site/subdomain/team/cluster/pilot-credential/login URL), `Asset` (resource_id/team/cluster/plan/specs/IPs/URLs/resize/synced), `Atlas Event` (cluster/event/status/occurred/signature/raw).
- Link: `Atlas Instance` (region/base_url/status/creds/tunnel ip+url+status/peer key/service user/webhook secret), `Central Tunnel Settings` single (hub key path/pubkey/endpoint/port/CIDR/status), `Pilot Credential` (pcid/team/asset/audience/token_hash/status/expiry), `Central SSO Settings` single (issuer/kid/keypair RS256), `Cargo Instance` (region/URLs/tokens/telemetry).
- Ops: `Resource Action` (type/team/resource/correlation/status/atlas_task/error/retriable), `Host Task` (script audit), `Region`, `Central Settings` (retention + feature flags).
- Billing (~43): catalog (Plan/Category/SubCategory/Includes/ResourceType/CatalogRate/Configurator+rates/Scenario), customer (BillingProfile/TaxProfile/Project+spending_limit/Methods/Gateway(+Currency)/GatewayCustomer), subscription/entitlement (Subscription/Change/Commitment/EntitlementToken/UsageRollup), revenue (BillingRun/Event/ProjectionBatch+Summary/Invoice+Lines/Attempts/Refunds/Webhooks/NotificationLog/Wallet/Ledger/WelcomeCredits/Tiers/Thresholds/Settings/ReratingRun).
- Services (7): `Add On Service/Service Backend/Managed Service/Service Credential/Site Service Credential/LLM Model/Plan Tier/Plan Policy`.

## 2. Flows (step chains)

- VM create: validate → placement (sample − drafts) → lock → draft+COMMIT → `PUT /v1/vms/{name}` → clear draft (uncertain→keep, 404→delete).
- Placement: sample → subtract post-sample + drafts → freshness → lock+recheck → arch match.
- Read: detail = live Metal per request (draft→pending, absent→unknown); list = `Virtual Machine State` only + `state_synced_at`.
- Reconcile: cron stale drafts + terminating → GET Metal → absent→delete else log+retry; per-server `sync_state`; `POST /v1/sync` refreshes all.
- Terminate: `DELETE` → detach IPv4 (keep reservation) → keep record → poll absent → delete doc.
- Machine image: `POST snapshots` → UUIDv7 staging → multipart (2GiB/24h) → verify → complete → delete staging (48h). Delete: mark Deleting+disable → refuse in-use → abort multiparts → delete objects + staging → doc (30s retry). Site-File→object migration on Settings change + 15m sweep.
- Metal provision: stable-identity host → wait → addresses → install Metal → WG → complete (resume-at-failed-phase).
- IPv4: pool-lock unowned OR provider reserve → tenant; attach owned-or-unowned→claim (else refuse, uplink required); release clears tenant, keeps provider.
- Proxy: VM+IP → node/regional/wildcard DNS → SSH package (digest-skipped) → secret config push → peers → readiness → Active (archive reverses).
- Cargo: needs proxy+image+IP → tenant-0 VM → `install-cargo.sh` → PATCH sites → ping→pong → Active → bucket `atlas-<region>` → Settings.
- Migration: `PUT migrations` → source snapshot/stop/start/destroy (mesh, no cred) → finish|abort → delete source record.
- Tunnel Register: ping public → hub `wg0` + `/32` alloc → scoped service user + webhook secret → `provision_tunnel` (auto-revert armed) → peer-add → ping-over-WG → confirm → Active (`skip_tunnel` = identity-only dev).
- SSO: RS256 mint (`bench|site` 5m, `enroll` 30m, `datum`/logs 7d, cargo 1y; `aud` = pcid) → offline JWKS verify (aud+scope+jti) → bench `?sid=` exchange vs site Central→bench relay → `desk?sid=`.
- Billing: period → rollups → Draft → Open → (Paid|Overdue|Cancelled|Waived); attempts Initiated→Authorised→Captured→(Failed|Refunded); webhooks settle; dunning PastDue→Suspended→Current. Metering idempotent (seq/version); wallet `(team,currency)` locked; trust ladder + Ed25519 tokens.
- Pilot enroll/heartbeat: bootstrap token → `POST enroll` (guest, Redis jti) → long-lived bearer (hash stored) → `heartbeat/config/metrics_token/log_token`.
- Cargo enroll: bootstrapping token → control creds → dual access tokens → garage/telemetry register (region-bound, idempotent).

## 3. Screens / pages + user views

- Atlas Desk (`/app/atlas`, all Desk users): workspace + 6 number-cards (VMs, Metal Servers, Proxy Servers, IPs in-use/available, Images); sidebar (Home | VM: Machine/Image/State | Service: Proxy/Cargo | Metal: Server/IP/Size/Image/Usage | Core: SSH Task/Settings). `www/vm_console (/vm_console?vm=)` guest + one-time fragment token → xterm via SocketIO.
- Central Desk: `Workspace Central` (Team/Invitation/Role/Capability/Probe); `Workspace Billing` (+`billing.json` sidebar: Catalog/Customers/Subscriptions/Revenue/Operations/Setup, ~20 reports: Invoice Register, Collection Outlook, AR Aging, MRR/YTD, Atlas/Services/Cluster Revenue, Failed Payments, Dunning Recovery…).
- Central Vue SPA (`/dashboard/`, team via switcher not URL): `/login|/signup|/signup/verify|/forgot-password` (guest, OTP); `/onboarding/site|/provisioning/:name` (new users); `/home` (overview); `/servers|/servers/new` (`server:view/create`); `/addons|/addons/ai|/addons/object-storage` (entitled); `/billing|/invoices|/reports|/limits` (`billing:*`); `/settings[/:tab]`; `/team/members` (+aliases), `/team|/invitations[/:name]` (`team:manage_members` for mutates).
- Porter mapping: Desk workspaces → REST + thin UI (no Desk logic); team routes `/dashboard/t/<team>/…` → scoped `WHERE team IN (…)` + RLS; number-cards → `GET /metrics/summary` + RBAC.

## 4. API routes (auth per route)

- Atlas tenant (`/atlas/*`: regional Bearer `tenant=N` + matching `X-Tenant-ID`, or central `tenant=*` naming any tenant; `jwks.json` public): `POST/GET list/GET detail/DELETE /virtual-machines[/{id}]`, `POST …/actions/{start|stop|pause|resume|restart|snapshot|console-token}`, `PATCH …/{compute|disk|network}`, `PUT …/{ssh-keys|metadata|ip-address}`, `DELETE …/ip-address`, `POST/GET list/GET/DELETE /ip-addresses[/{id}]`, `GET /images[/{id}][/download]`, `DELETE /images/{id}` →202.
- Metal `/v1` (static Bearer; migration-source mesh routes unauth): `GET /health|/docs|/docs/swagger.json`; `POST /v1/sync`; `GET /v1/vms`, `PUT|GET|DELETE /v1/vms/:id`, `PUT …/power`, `POST …/restart`, `PUT …/{compute|disk|network|ssh-keys|metadata}`, `POST …/snapshots`, `GET …/console` (WS); `POST|GET|DELETE /v1/snapshots/:id[/upload]`; `PUT|GET|POST {abort|finish}|DELETE /v1/migrations/:id` + source mesh routes.
- Proxy control (`:9000`: regional password + JWT `site:*|domain:*` name-constrained; internal `X-Atlas-Cluster-Password`): `GET /healthz|/readyz`, `GET|PUT /v1/sites|/domains`, `PATCH|DELETE /v1/sites/{name}|/domains/{domain}` (+generation header), `GET /v1/cluster/status`, `/internal/cluster/{status|snapshot|vote|heartbeat|replicate|mutate}`.
- Central (`frappe.whitelist`, session + `can()` unless noted): `atlas.py` (`POST event` guest+HMAC, `POST register` admin deprecated, `GET sizes|images|ping` service-user); `auth.py` (`POST sign_up|resend|verify_signup` guest, `POST change_password`); `teams.py` + `identity.py` (session, `team:*` gates); `servers.py`/`sites.py` (`server:*`, session+team); `sso.py` `GET get_bench_link` (session); `jwks.py` `GET` guest; `developer_setup.setup_local` (dev only); `pilot.py` (guest+pilot Bearer: `GET heartbeat|config|metrics_token|log_token`, `POST enroll`); `cargo.py` (guest+Cargo JWT); services dashboard/pilot split (session+`service:*` vs guest+pilot Bearer).
- Billing (~87, session+team; admin = System Manager/billing_operator): `billing_api.py` (gateways, plans, subscriptions, usage, credits, profiles, methods, topups, checkouts), dashboard account/catalog/invoices/methods/outlook/projects/reports/services/spend, admin catalog/gateways/projection/rerating/revenue/services/teams.
- Porter mapping: same verbs over one RBAC-ed REST (`HasCapability` per route, idempotency keys, cursor pagination, ETag/`If-Match`, request IDs, structured errors, POST-for-actions); Metal/proxy/pilot/cargo splits collapse into `internal/{runtime,firecracker,gateway,agent,build}` behind the API.

## 5. Roles data (exact seed contents — checked 2026-09-18)

- Atlas `fixtures/role.json`: ONE role — `Atlas Admin` (`desk_access=0`, not custom). `fixtures/role_profile.json`: ONE profile `Atlas Admin → [Atlas Admin]`. Tenant users (`tenant-<id>@atlas.local`, `tenant-all@atlas.local`) get this role on first use (`ensure_tenant_user`). `System Manager` is Frappe-built-in (not seeded here) = sole bypass.
- Central `fixtures/role.json`: ONE portal role — `Central User` (`is_custom=1`, `desk_access=0`). Every signup gets it; personal Team bootstrap depends on it.
- Central `fixtures/capability.json`: 15 strings — `billing:view/manage`, `team:edit/manage_members/delete`, `cluster:view`, `server:view/create/power/resize/snapshot/terminate/open`, `service:view/manage` (+plane/resource/description each).
- Central `fixtures/team_role.json` (5 system roles, `is_system=1`, `team=null`): Owner 15 caps (all + `team:delete`), Admin 14 (all − `team:delete`), Developer 10 (`cluster:view`, all 7 server ops, `service:view/manage`), Viewer 3 (`cluster:view`, `server:view`, `service:view`), Billing 6 (`billing:view/manage`, `cluster:view`, `server:view`, `service:view/manage`). Ladder: Viewer → Billing → Developer → Admin → Owner. Custom team roles recombine these strings only.
- Porter seed (migration 0017 per task T1a): mirror this shape — `capabilities` catalog + `roles` (`platform_admin, infra_operator, billing_operator, support_operator, reseller_admin, org_admin, org_developer, org_reader, service_account, ai_agent, extension`) + `role_capabilities` (allow/deny) + bootstrap `platform_admin` user. Never hardcode role strings in handlers.

## 6. DB migrations (checked 2026-09-18)

- Atlas `patches.txt`: EMPTY (both `[pre_model_sync]` + `[post_model_sync]` have zero entries — schema evolves via DocType JSON sync only, no data migrations).
- Central `patches.txt` + `patches/v0_0/` (53 patches, pre-1.0 series): pre-sync (rename patch-log rows, snapshot legacy rates, `team` Data→Link migration, vCPU dropdown widen, one-gateway-per-adapter, Billing-Group→Project rename); post-sync (rate children standalone, backfills: priority/region/wallet/gateway-customers/provisioning, guards: team-links, drops: billing-roles/trust-tier/project-scoping/unused-meta, renames: capabilities/team-role-autoname/subscription-project/invoice-consolidation/collection-modes, taxonomy: catalog/product-families/configurator-alignment/memory-ratio/provision-target/compose-bounds/vcpu-ladder, retires: addon→plan/price-lock, fixes: terminated-subs/cap-strip/region/welcome-credits/autoname/credentials/INR-ceiling/predebit-notice/trial-flags/SSO-issuer). Pattern: rename-before-sync (table exists to rename from), backfill-after-sync, drop-last.
- Porter `backend/migrations/` (0001…0016 + `migrations.go` runner, Postgres `IF NOT EXISTS`, up/down pairs): 0001 core (projects, services, vms, domains, volumes, ssh_keys, servers, users, secrets, deployments, networks, golden_images, metrics_samples, health_events) → 0003 system (orgs, org_members, groups, group_projects, project_settings, project_members, builds, stacks, api_keys, alerts, hooks, crons, drains, redirects, firewall_rules, environments + dns_records/build_logs re-affirmed) → 0005 paas (daemon_logs, traffic_logs, vm_logs, analytics_daily, history) → 0007 RBAC (roles, permissions, role_permissions) → 0008 deployment checks → 0009 feedback → 0010 server_cluster → 0011 user_email → 0012 seed rbac_admin → 0013 image_artifact_contract → 0014 build_log_stream_contract → 0015 seed_super_admin → 0016 image_upload_permission + versioned_deployments. Gap (task T1): 0017 `capabilities, role_capabilities, role_assignments` + `resellers, teams, ip_allocations, micro_vms` still missing — NO billing tables until subscriptions ship.

## 7. Screens / pages / view route list (checked 2026-09-18)

- Atlas www (2 files): `www/vm_console.py/.html` (`/vm_console?vm=<id>`, guest allowed, one-time 48-char 60s token in URL `#fragment` so it never hits server logs/`Referer`; xterm + socket.io + fit addon; SocketIO `atlas_console_open/input/resize/disconnect`).
- Atlas Desk workspace (`atlas/atlas/workspace/atlas/atlas.json` + `sidebar` + 6 `number_card/*.json`): cards `virtual_machines, metal_servers, proxy_servers, ip_addresses_available/in_use, virtual_machine_images` (tenant/status-filtered counts, charts=[]); sidebar sections Home | VM (Machine/Image/State) | Service (Proxy/Cargo) | Metal (Server/IP/Size/Image/Usage) | Core (SSH Task/Settings).
- Central `dashboard/src/router/index.ts` (exact paths): public `/login, /signup, /signup/verify, /forgot-password`; onboarding `/onboarding/site, /onboarding/provisioning/:name`; app shell `/` → redirect `/servers`, children: `home`, `servers`, `servers/new`, `addons/ai`, `addons/object-storage`, `addons` (generic `Page.vue`), `billing`, `billing/invoices`, `billing/reports`, `billing/limits`, `settings`, `settings/:tab`, `team`→redirect `/team/members`, `team/members` (+`team/roles`, `team/settings` aliases → same `AccessPage`), `team/invitations`, `invitations`, `invitations/:name`; fallback `/login`. Guards: public (guest), onboarding (`onboarding_complete=false`), app (session + `activeTeam`; per-page `server:view/create`, `billing:*`, `team:manage_members` for mutates; `addons/*` behind `Central Settings` feature flags).
- Central `dashboard/src/pages/` (23 files): `auth/` Login/Signup/VerifyEmail/ForgotPassword; `onboarding/` SiteName/SiteReady; `home/` HomePage; `servers/` Servers/NewServer (+`signing-in.html`); `addons/` AIInference/ObjectStorage/Page; `billing/` Overview/Invoices/Reports/SpendingLimits; `settings/` Settings/SettingsDetail; `team/` Access/Invitations. Composables (~40 `use*`), lib + types mirror the API contract.
- Central Desk billing workspace: number-cards (Open/Overdue Invoices, Active Subscriptions, Failed Payments 24h) + shortcuts (Launch plan/add-on, Update prices, Retire) + sidebar Catalog/Customers/Subscriptions/Revenue/Operations/Setup + ~20 reports.
- Porter mapping: every page above = thin client over the same RBAC-ed REST (no Desk logic); route guards become `HasCapability` middleware + scoped list filtering; number-cards become `GET /metrics/summary`; onboarding becomes `porter doctor` + node-enrollment flow; console page becomes `POST /vms/:id/console` (vsock, `vm.console` cap).
