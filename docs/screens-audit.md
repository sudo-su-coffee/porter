# Screens audit (alone) — Atlas + Central + Porter UI

> Audited 2026-09-18. Sources: `references/atlas/atlas/workspace|www|number_card`,
> `references/central/central/workspace*`, `references/central/dashboard/src/router+pages`,
> `porter1/ui/*.html`. Screens are READ-ONLY references — Porter reimplements as thin
> REST clients (no Desk logic, no Filament/Livewire, no copied components).

## A. Atlas Desk (Frappe Desk, `/app/*`)

### A1. Workspace Atlas (`/app/atlas`, all Desk users)

- 6 number-cards (tenant/status-filtered counts, `charts=[]`):
  `virtual_machines`, `metal_servers`, `proxy_servers`, `ip_addresses_available`,
  `ip_addresses_in_use`, `virtual_machine_images`.
- Verdict: keep the 6 counts → Porter `GET /metrics/summary` cards (RBAC-filtered).

### A2. Sidebar Atlas

- Sections: Home → Atlas workspace | VM: Virtual Machine, Virtual Machine Image, Virtual Machine State | Service: Proxy Server, Cargo Server | Metal: Metal Server, Metal Server IP Address, Metal Server Size, Metal Server Image, Metal Server Usage | Core: SSH Task, Atlas Settings.
- Verdict: sidebar groups → Porter nav groups (Workspace / Compute / Infrastructure / Network & storage / Operations / Commerce), each item = one REST list route.

### A3. `www/vm_console` (`/vm_console?vm=<id>`)

- Guest-allowed page; one-time 48-char 60s console token passed in URL `#fragment` (never hits server logs/`Referer`); xterm + socket.io + fit addon; events `atlas_console_open/input(resize)/disconnect`; token consumed via Redis `GETDEL`; slow viewers dropped, guest never blocked.
- Verdict: → Porter `POST /vms/:id/console` (vsock, `vm.console` cap) + streamed WS; same fragment-token trick for share links.

## B. Central Desk (Frappe Desk)

### B1. Workspace Central (`/app/central`, operators)

- Shortcuts: Team, Team Member rows, Team Invitation, Team Role, Capability, IAM Permission Probe.
- Verdict: → Porter Admin → Identity section (users/roles/teams/invitations/probe as REST lists).

### B2. Workspace Billing (`/app/billing`, System Manager / billing_operator)

- Number-cards: Open Invoices, Overdue Invoices, Active Subscriptions, Failed Payments (24h).
- Shortcuts: Launch plan/add-on (→ Plan Configurator), Update prices, Retire.
- Sidebar (`billing.json`): Catalog | Customers | Subscriptions | Revenue | Operations | Setup + ~20 reports (Invoice Register, Collection Outlook, AR Aging, MRR/YTD, Atlas/Services/Cluster Revenue, Failed Payments, Dunning Recovery, Gateway Success Ratio, Payment Method Mix, Refunds, Churn, Credit Movements, Team Balances, New Signups, Webhook Lag, Invariant Violations, Billing Projection).
- Verdict: → Porter Admin → Billing section, deferred past MVP; reports become `GET /billing/reports/*` (metered aggregates, same RBAC).

## C. Central Vue SPA (`/dashboard/`, session + `activeTeam` switcher, no team-in-URL)

| # | Route | File | Guard | Shows |
|---|---|---|---|---|
| 1 | `/login` | `auth/LoginPage.vue` | guest | Email/password + provider OAuth buttons + forgot link |
| 2 | `/signup` | `auth/SignupPage.vue` | guest | Name/email/password + OTP trigger |
| 3 | `/signup/verify` | `auth/VerifyEmailPage.vue` | guest+pending | OTP code entry → verified user + personal team + billing profile |
| 4 | `/forgot-password` | `auth/ForgotPasswordPage.vue` | guest | Reset request |
| 5 | `/onboarding/site` | `onboarding/SiteNamePage.vue` | logged-in, `onboarding_complete=false` | Subdomain picker + region |
| 6 | `/onboarding/provisioning/:name` | `onboarding/SiteReadyPage.vue` | same | Provision poll → ready redirect |
| 7 | `/` → redirect `/servers` | AppShell | session | Shell (nav + team switcher + notifications) |
| 8 | `/home` | `home/HomePage.vue` | member | Team overview, spend, servers summary |
| 9 | `/servers` | `servers/ServersPage.vue` | `server:view` | Fleet list (`ServerMap.vue` 912 LOC: map + `useFleetRows/useServers/useServerMapData`) |
| 10 | `/servers/new` | `servers/NewServerPage.vue` | `server:create` | Wizard (plan/region/version → composed config) |
| 11 | `/addons` | `addons/Page.vue` | entitled | Generic add-on host |
| 12 | `/addons/ai` | `addons/AIInference.vue` | entitled + `llm` flag | Model offers, keys, token usage |
| 13 | `/addons/object-storage` | `addons/ObjectStorage.vue` | entitled + `storage` flag | Buckets, keys, endpoints |
| 14 | `/billing` | `billing/BillingOverviewPage.vue` | `billing:*`/Owner/Admin/Billing | Forecast, credit balance, schedule |
| 15 | `/billing/invoices` | `billing/BillingInvoicesPage.vue` | same | Invoice list + detail + pay + tax |
| 16 | `/billing/reports` | `billing/BillingReportsPage.vue` | same | Cost explorer, breakdowns |
| 17 | `/billing/limits` | `billing/SpendingLimitsPage.vue` | same | Trust-tier caps, project limits |
| 18 | `/settings` | `settings/SettingsPage.vue` | member | Workspace settings index |
| 19 | `/settings/:tab` | `settings/SettingsDetailPage.vue` | tab-gated | Profile/team/billing tabs (deep-linkable) |
| 20 | `/team` → redirect `/team/members` (+`team/roles`, `team/settings` aliases) | `team/AccessPage.vue` | member (mutates need `team:manage_members`) | Members + roles + rename/transfer/delete |
| 21 | `/team/invitations` | `team/InvitationsPage.vue` | member | Sent invites (resend/revoke) |
| 22 | `/invitations` | same | member | Received inbox (accept/decline) |
| 23 | `/invitations/:name` | same | member | Single invite detail |

- Supporting: ~40 composables (`useSession/Auth/Team*/Capabilities/Billing*/Servers/Plans/Projects/Regions/Services/Search/Notifications/...`), lib (`frappeCall/toast/status/serverMap/roles/...`), types (Team/Member/Role/Capability/Invitation/Asset/AtlasInstance/...).
- Verdict: routes 1–7 → Porter auth/onboarding (OTP + `porter doctor` + enrollment); 8–10 → Porter customer Compute; 11–13 → Porter marketplace/services (flag-gated); 14–17 → Porter billing (deferred); 18–23 → Porter team settings. Guards become `HasCapability` middleware; team switcher becomes `X-Tenant-ID` + scope selector.

## D. Porter `ui/` mockups (reference only, never ship)

- `apple.html` (74KB): single-pane Apple mock (SF Pro, `#0071E3`, vibrancy sidebar 264px). Sidebar: Deployments (active), MicroVM (Firecracker), Git Deploy, Deploy Container (OCI), Deployment Logs, phpMyAdmin + infra/net/storage/observability/billing. Main: Deployments page + right info-panel (MicroVM/Kubernetes Load, Current Deployment, runtime = MicroVM Firecracker).
- `final desing only .html` (112KB, UI/UX Master Reference v5, canonical): SPA mock — light/dark, topbar (Customer/Admin switch, ⌘K, notifications), collapsible sidebar, card/list toggle, modals, palette, toasts, login. Admin 39 areas + Customer 22 areas (see §C-equivalent groups); Infrastructure-360 graph (`Provider→…→Deployment/Network`); resource modal (generation 42/42, controller, reconcile 18s, audit timeline); support (30-min audited impersonation).
- `image/` (9 AI PNGs ~11.6MB): density/moodboard only.
- Verdict: take nav groups, page-head + explicit states (never spinner), resource-page sections (Header/Summary/Activity/Config/Observability/Deps/Infra/Audit/Danger), Customer-360/Infra-360 traversal. Never port markup/logic.

## E. Gaps vs Porter (what has NO screen yet)

1. Node enrollment wizard (token → preflight → READY) — no Atlas/Central equivalent screen; design from `porter doctor` output.
2. Build/artifact view (BuildKit task → digest → SBOM/sign/scan → rootfs+kernel) — Central has none (billing-only); design from G2.
3. Reconcile inspector (desired vs observed + generation + op ID + retry) — Atlas exposes virtual fields only; Porter needs a first-class screen.
4. Impersonation banner + consent flow UI (spec'd, Central Desk lacks it) — build with audit trail visible.
5. Per-resource RBAC matrix screen (who has which cap at which scope) — neither Atlas nor Central renders `resolve_user_grants`; Porter Admin needs it.

## F. Web access matrix — who sees what (role × screen)

Legend: ✅ full · 🔍 read-only · ❌ denied · 👑 bypass. Atlas roles: `System Manager` (Frappe built-in, 👑 everywhere), `Atlas Admin` (tenant operator). Central team roles: Owner / Admin / Developer / Viewer / Billing (+ `Central User` portal baseline). Guest = unauthenticated.

### F1. Atlas Desk + tenant API views

| Screen / view | Guest | Tenant user (`Atlas Admin`, tenant=N) | Central caller (`tenant=*` + `X-Tenant-ID`) | System Manager |
|---|---|---|---|---|
| Desk workspace `/app/atlas` + 6 cards | ❌ | ✅ own-tenant counts | ✅ any-tenant counts (header selects) | 👑 all |
| DocType lists (VM/Image/IP/Migration via `frappe.get_list`) | ❌ | ✅ `tenant_id=N` rows only (+ shared System images for read) | ✅ per-header-tenant rows | 👑 all |
| DocType form (single VM/Image/IP) | ❌ | ✅ own doc (`get_owned_document`, else 404) | ✅ header-tenant doc | 👑 all |
| `GET /atlas/jwks.json`, `/api/atlas/docs`, `/files/*` (binaries/artifacts) | ✅ | ✅ | ✅ | ✅ |
| Tenant API `POST /virtual-machines` (privileged flag) | ❌ | ✅, privileged only if tenant=0 | ✅, privileged only if header=0 | 👑 |
| Tenant API power/snapshot/console-token/compute/disk/network/ssh/metadata/IP | ❌ | ✅ own-tenant docs | ✅ header-tenant docs | 👑 |
| Tenant API images download (24h signed URL, `no-store`) | ❌ | ✅ own + System (Site-File refused) | ✅ same per header | 👑 |
| `www/vm_console` (fragment token) | ✅ with valid 60s single-use token | ✅ (token minted from own VM) | ✅ (token minted per header) | 👑 |
| Metal Server provision ops / provider creds / bare-metal | ❌ | ❌ | ❌ | 👑 only |
| Realtime handshake paths (`/socket.io`, `get_user_info`, `has_permission`) | ✅ (handshake only) | ✅ | ✅ | ✅ |

### F2. Central Desk views

| Screen / view | Guest | Team role | System Manager |
|---|---|---|---|
| `Workspace Central` (Team/Invitation/Role/Capability/Probe lists) | ❌ | ✅ membership rows (`team_query_conditions`); mutates per `team:edit/manage_members/delete` | 👑 all |
| `Workspace Billing` + 20 reports | ❌ | ✅ own teams (`billing:view` for reads; `billing:manage` for methods/credits/checkout) | 👑 all |
| Invitation rows | ❌ | ✅ own-email OR `team:manage_members` | 👑 all |
| IAM Probe rows | ❌ | ✅ own-user rows only | 👑 all |
| Asset/Site/Resource-Action rows | ❌ | ✅ teams with `server:view` (open needs mutating cap) | 👑 all |

### F3. Central Vue SPA (per-route guards)

| Route | Guest | Viewer | Billing | Developer | Admin | Owner |
|---|---|---|---|---|---|---|
| `/login|/signup|/verify|/forgot-password` | ✅ | — | — | — | — | — |
| `/onboarding/*` | ❌ (redirect login) | ✅ own | ✅ own | ✅ own | ✅ own | ✅ own |
| `/home` | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `/servers` | ❌ | 🔍 (`server:view`) | 🔍 | ✅ operate | ✅ | ✅ |
| `/servers/new` | ❌ | ❌ | ❌ | ✅ (`server:create`) | ✅ | ✅ |
| `/addons*` | ❌ | 🔍 | 🔍 | ✅ | ✅ | ✅ (flag + entitlement gated) |
| `/billing*` | ❌ | ❌ | ✅ (`billing:*`) | ❌ | ✅ | ✅ |
| `/settings[/:tab]` | ❌ | ✅ own tabs | ✅ own tabs | ✅ own tabs | ✅ | ✅ |
| `/team/members` (+aliases) | ❌ | 🔍 | 🔍 | 🔍 | ✅ mutates | ✅ all incl. transfer/delete |
| `/team/invitations`, `/invitations[/:name]` | ❌ | receive-only | receive-only | receive-only | ✅ send/manage | ✅ |

### F4. Porter `ui/` mockup modes (reference)

- Customer mode (`Acme Systems / Platform / Production`): projects/apps/deployments/envs/domains/variables/secrets/storage/logs/metrics/networking/cron/backups/team/usage/billing/settings/support/dev-tools/activity — all read-operate, no host internals.
- Admin mode (platform scope): + customers/orgs/users/roles/resellers/nodes/pools/capacity/scheduler/firecracker/providers/regions/gateways/DNS/certs/incidents/security/audit/products/meters/invoices/automation/webhooks/tasks/AI-agents/integrations/API/marketplace/system/maintenance.
- Mode switch itself is the RBAC demo: same components, different scope + guards.

## G. Per-screen detail (widgets + actions + API + guard)

### G1. Atlas VM list → detail (the core loop)

- List widgets: status cards (counts from `Virtual Machine State`), table (name/server/image/vcpus/mem/state/`synced_at`), tenant filter implicit. Actions: Create (opens `CreateVirtualMachinePayload` form), Terminate (202 + keeps record). API: `GET /virtual-machines?offset&limit`, `POST /virtual-machines` (201), `DELETE …/{id}` (202). Guard: query conditions + `get_owned_document`.
- Detail widgets: desired vs current panels (compute/disk/network/guest from live Metal GET), error panel (never fakes state), activity (SSH Task + job logs), danger actions (power/restart/snapshot/console-token/compute/disk/network/ssh/metadata/IP attach-detach, privilege grant/revoke, idle-shutdown, egress change). API: `GET …/{id}`, `POST …/actions/*`, `PATCH …/{compute|disk|network}`, `PUT …/{ssh-keys|metadata|ip-address}`, `DELETE …/ip-address`. Guard: same-doc tenant check per action; privileged actions tenant-0 only.

### G2. Atlas Images + IPs

- Images: filter `image_type=system|machine`, rows (title/arch/version/status/enabled/`cache_image`/`memory_snapshot`/sizes/sha), detail (artifact locations, transfer progress, source server/snapshot), actions (download 24h URL, retire→disable, delete→Deleting job, `Retry Transfer`, `Migrate to Object Storage`). API: `GET /images[?type]`, `GET …/{id}`, `GET …/{id}/download?artifact=`, `DELETE …/{id}`.
- IPs: pool vs provider tabs, rows (address/server/VM/status/intent-version/tenant), actions (reserve pool/provider, attach w/ uplink enforcement, detach, release, `Reset Tenant` admin-only + audit comment). API: `POST /ip-addresses{source}`, `GET …`, `DELETE …/{id}`.

### G3. Metal Server detail (operators only)

- Widgets: lifecycle phase tracker (create→wait→addrs→install→WG→complete), capacity sample table (from `Metal Server Usage`), disk inventory, catalog mirrors (sizes/images), sync state + faults (transport vs response), metald version + rollback control, WG IP + peers.
- Actions: `provision/setup_server/ping/sync_disks/sync_state/install_metald/upgrade_metald/reboot/poweroff/poweron/archive`. Guard: System Manager only.

### G4. Central Servers + New Server

- `ServersPage`: fleet map (`ServerMap.vue`: region/cluster pins + `useFleetRows` status rows), filters (team/region/status), row actions (open console via `server:open` signed SSO → bench, power/resize/terminate via Resource Action + Atlas RPC), `refresh_assets` reconcile pull.
- `NewServerPage`: wizard (plan ladder → region (`cluster:view`) → version → composed includes → price resolve → provision → asset_id). Guards: `server:create` + size clamp + trial gate.

### G5. Central Billing (4 pages)

- Overview: profile wizard (currency lock), trust tier, forecast, credit balance/ledger, top-up options, payment schedule, collection mode (₹115k gate).
- Invoices: list + detail (lines with project tags, tax, attempts) + pay (checkout/confirm, fallback offer, mandate flows).
- Reports: cost explorer (team/cluster/project cuts), aging, MRR/YTD, gateway mix, dunning recovery, webhook lag, invariants.
- Limits: tier ladder, per-currency thresholds, project `spending_limit` + headroom enforcement display.
- All mutating buttons declare `methods=["POST"]` (Frappe rolls back writes on GET).

### G6. Central Team (Access + Invitations)

- AccessPage: member table (user/role/status), role matrix editor (vocabulary-subset custom roles), invite box (email+role, no-Owner), actions (set roles/status, remove, rename team, transfer ownership, delete team) — each gated (`team:manage_members` / `team:edit` / `team:delete` / Owner-only transfer).
- Invitations: sent (resend/revoke) vs received (accept/decline) tabs; expired sweep daily.

### G7. Porter screen plan (from gaps)

1. Enrollment wizard: token entry → `doctor` checklist (each `check/status/severity/observed/expected/cause/action/evidence`) → keypair verify → capabilities → READY + mesh status.
2. Build/artifact view: BuildKit task (logs stream) → digest + SBOM/provenance/sign/scan badges → rootfs+kernel sizes/sha → boot-test button → promote/rollback.
3. Reconcile inspector: desired vs observed diff + generation/observedGeneration + op ID + last error + retry button + event timeline (every controller).
4. Impersonation: request → policy → consent → 30-min banner + countdown → full audit link → auto-expiry display.
5. RBAC matrix: principal × scope tree × capability grid (allow/deny source role shown), effective-perms preview (`resolve_user_grants` equivalent), test-as-principal probe.
