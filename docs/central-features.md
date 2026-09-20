# Central — full feature inventory (distilled before deletion, 2026-09-18)

> Source: `references/central/` copy (AGPL — patterns only, deleted after extraction).
> Central = global control plane: IAM + billing + add-ons + SSO + WG hub. Frappe/Python 3.14,
> Node 24 console, MariaDB 11.8, Redis 6+. Atlas clusters consume via OAuth, enforce locally.

## 0. Bootstrap: README / CAPABILITIES / spec index

- Install: `bench get-app/new-site/install-app central`, `developer_mode 1`, `bench build/start`; seed `developer_setup.setup_local([register_atlas])`; Atlas second site + `atlas.atlas.demo.run(reset)` demo fleet; console `yarn install/dev/build`; tests `run-tests --app central`.
- Capability taxonomy: `resource:action`, planes central/atlas/bench(deferred), server-atomic v3, 13–15 strings, 5 roles (Owner/Admin/Developer/Viewer/Billing), implication closure, change via fixtures + `CAPABILITY_VERSION` bump.
- Spec index: IAM + ExecutionPlan (identity/permissions/OAuth/enforcement) + Tunnel (hub/Register/service-user/host-exec) + Billing (Atlas Integration workflow).

## 1. IAM engine (`iam.py`, `permissions.py`, `oauth.py`, `users.py`, guards)

- `iam.py`: `System Manager` sole bypass; `CAPABILITY_VERSION=4`; `CAP_IMPLICATIONS` closed via `expand_capabilities`; `resolve_user_grants` (4-table join, request-cached; operator fan-out; custom-role scoping); `clear_grants_cache`; `get_fc_teams_claim` (`{team:[{role,source,scope:"*",caps}]}`); `can()`; `get_effective_permissions`; team resolvers.
- `permissions.py`: team/role/invitation/asset/site/probe conditions + `has_permission` (read vs `MUTATING_PERMISSION_TYPES`); `_team_field_*` team-field scoping; `_can_any`.
- `oauth.py`: stamp `fc_teams` into OIDC userinfo (patched `get_userinfo` + `openid_profile`).
- `users.py`: User-insert bootstrap (Central User role → personal Team + Owner → accept email-matched invitations).
- Guards (`utils/guards.py`, `inputs.py`): `resource_action` envelope + `require_team_member/require_capability/require_self_or_operator/require_text/require_secret`.
- Supporting: `geo.py` (country from IP), `errors.py` (envelope), `mirror.py` (LWW upsert + freshness), `host_task.py` (Atlas-compatible `--kebab`/ `ATLAS_RESULT=` runner + audit + 30d prune), `scripts_catalog.py` (hub-script allowlist), `www/dashboard.py` (SPA boot: csrf/user/features/provider-logins/onboarding).

## 2. DocTypes (19): Team/Member/Role/Capability/Invitation/Probe/Asset/Site/Atlas Instance/Tunnel Settings/Pilot Credential/SSO Settings/Settings/Region/Resource Action/Atlas Event/Host Task/Cargo Instance

- Team graph: `Team` (members, Owner, staging-trial) + invite/roles/status/remove/transfer/accept + guards; `Team Role` (system vs own-team custom, vocabulary-subset); `Capability` vocabulary; `Team Invitation` (email/role/scope, accept/revoke/resend/decline, daily expiry, no-Owner); `IAM Probe` (evaluate `can()` matrix).
- Mirrors: `Asset` (resource_id/team/cluster/specs/IPs/URLs/resize/last-synced; LWW mirror + drift fix + subscription sync); `Site` (name/subdomain/team/cluster/pilot-credential/login URL); `Atlas Event` (HMAC-verified inbound log → apply).
- Atlas link: `Atlas Instance` (region/base_url, admin creds, `skip_tunnel`, tunnel_ip/url, peer key/endpoint, service_user, status; `register()` handshake, `remove_tunnel()`, `test_connection()`); `Central Tunnel Settings` single (key path/pubkey/endpoint/port/CIDR/status, `initialize_hub()`, next-free-IP).
- Machine identity: `Pilot Credential` (pcid/team/asset/audience, SHA256 hash store, once-plaintext, reserve/issue/mint/verify/rotate/revoke/link); `Central SSO Settings` single (issuer/kid/keypair, RS256 authority); `Cargo Instance` (region/URLs, bootstrap + dual access tokens, region binding).
- Ops: `Resource Action` (type/team/resource/correlation, `open_action()/succeed()/record_mirror_status()`, 15m `sweep_stale` 5m-cron); `Host Task` audit; `Region`; `Central Settings` (retention + feature flags).

## 3. API (`api/` 12 surfaces)

- `atlas.py`: `event` (guest HMAC ingest → mirror), `register` (legacy anchor), `sizes/images` (cluster catalog), `ping` (health/auth).
- `auth.py`: `sign_up/resend/verify_signup` (OTP + rate limit → verified user + team + billing), `change_password` (history).
- `identity.py`: `my_capabilities/my_teams/my_invitations/my_profile/update_profile`.
- `teams.py`: list members/roles/capabilities/invitations; create/rename/transfer/delete team; invite/resend/revoke/accept/decline; set roles/status/remove; create/delete custom roles.
- `servers.py`: catalog (`frappe_versions/registry`), fleet (`server_overview/list_instances`), `refresh_assets`, `create_server` (size clamp + trial gate), `create_composed_server`, `start/stop/terminate` (via Resource Action + Atlas RPC).
- `sites.py`: `create_site` (billing gate), `get_site` (+ fresh login URL), `check_subdomain`, `site_domain`, `terminate_site`.
- `sso.py` / `site_login.py`: `get_bench_link` (needs `server:open` + Running + gateway + enrolled); site SID relay (cached 5m).
- `pilot.py` (`X-Pilot-Token`): `heartbeat/config/metrics_token/log_token/enroll` (Redis single-use jti).
- `cargo.py`: `garage_tokens/register_storage_cluster/register_telemetry_service/request_control_credentials`.
- `jwks.py`: `get_jwks` public. `developer_setup.py`: `setup_local` seed.

## 4. Integrations (`integrations/`)

- `atlas.py` (878 LOC): `AtlasClient` (ping/provision_tunnel/confirm/lifecycle/`run_doc_method`, tunnel-vs-base URL, admin auth); `ingest/apply_event`; `verify_atlas_webhook` HMAC; `_on_vm/_on_vm_deleted/_on_site` mirrors; `reconcile` 10m backstop; `register_atlas` (+ local variant, service user/role/creds rotation, tunnel verify, peer endpoint, rollback; errors `AtlasError/ResourceGone/TunnelRegistrationError`); `remove_tunnel`.
- `pilot.py`: bench monitoring fetch + cached telemetry + site-login relay.
- `cargo.py`: Cargo JWT gates + single-flight enroll + region binding.

## 5. Billing (postpaid in-arrears, ~36 DocTypes, 87 endpoints)

- Layers: dashboard/admin APIs → catalog (product/pricing/subscriptions/trust/signing) → revenue (draft→open→collect, metering, credits, tax, commitments, dunning, pricelock, erpnext) → payments (charges/collection/₹115k mode/mandates/emandate/instruments/methods/profile/reconciliation/refunds/settlement/webhooks/declines) → gateways (Stripe/Razorpay/PayPal adapters + registry) → platform (notifications/sync/alerts/invariants/constraints/metrics) → projection (what-if simulator + outlook).
- Money = float major units (minor only at gateway). One invoice per team per period (`period_key=team|start|end`). Projects = cost tags + run-rate guardrail (no credit/method scoping).
- Monthly run: 1st draft tick (page jobs, per-team commit, series-lock aware) + daily collect sweep (credits→card waterfall → Paid → ERPNext sync); worker-count = rate limit; contention retried; late runs defer dunning (never cost grace). Dunning Day1/3/7→past_due→suspend→terminate (token keeps asset to expiry). Meters idempotent (seq/version). Wallet `(team,currency)` locked. Trust ladder + Ed25519 entitlement tokens.
- DocTypes: Plan/Category/SubCategory/ResourceType/Includes/Configurator/Rates/Subscription(+Change)/Project/Invoice(+Lines)/Attempts/Methods/Gateways/Customers/Wallets/Ledger/Commitments/Rollups/Refunds/Webhooks/Profiles/Tax/Tiers/Tokens/Scenarios/Batches. Reports (20+): MRR, cluster/atlas, metered, aging, outlook, dunning, gateway mix, churn, credits, signups, webhook lag, invariants, projections. Demo seeder + ~80 tests + Playwright e2e (real gateways).

## 6. Services / notifications / fixtures / patches / tests / scripts / console / e2e / jobs

- Managed services: catalogue/entitlement/credentials/billing vs executor runtime; LLM (Grove sync/pull, plan→tiers, token reconcile), Garage S3 (cluster tokens, buckets, creds), provisioning options, `service:view/manage` gates; 7 DocTypes.
- Notifications: `dispatch(team,event,context,ref)` templated fan-out, dedupe, per-user prefs.
- Fixtures: 15 capabilities, 5 roles (Owner 15/Admin 14/Dev 8+/Viewer 2/Billing 4), Central User role, t0–t3 tiers, event catalogue.
- Patches (53): taxonomy, Add-on→Plan, PriceLock→ledger, Team→Link, rates standalone, gateways/currencies, wallet rekey, capability strip/rename, Project rename, trust-tier, collection modes, trial/SSO/provision seeds, credential merge, indexes/currency, ladders, workspace, roles.
- Tests (30+): IAM/teams/scoped-perms, auth/profile, Atlas register/sync/errors/webhook, enroll/SSO/bench/pilot, servers/fleet/geo/hooks/setup.
- Scripts: `hub-up/peer-add/peer-remove` + wireguard lib + sudoers pins + mail theme.
- Console (Vue3, 121 components): auth/onboarding/provisioning funnels; Home/Servers(+New/Map)/AI/ObjectStorage/Addons/Billing(+Invoices/Reports/Limits)/Settings/Team/Invitations; ~40 composables (`useSession/Auth/Team*/Capabilities/Billing*/Servers/Plans/Projects/Regions/Services/Search/Notifications/...`); lib + types.
- e2e (Playwright, real gateways): bench `:8011`, seed/teardown isolation; billing (onboarding/topup×2/invoices/settlement/mandate/emandate/dunning/refunds/generation) + signup.
- Scheduler: Atlas reconcile */10, ResourceAction sweep */5, invitations/host-tasks/dunning/prune/emandate/backfill/credits/LLM-sync/invariants daily, ERPNext/LLM-pull/reconciliation/alerts hourly, billing 1st + methods monthly; bench CLI (`setup_local`, demo seed, catalog/invoicing/dunning, tests, `export-fixtures`).
