# API specification (living) — routes, conventions, capability map

> Living contract extracted 2026-09-18 from `backend/internal/api/api.go` (`API.Routes`,
> all mutating reads behind `a.auth`). Conventions first, then grouped inventory.
> Gaps marked PLANNED. UI/CLI/SDK/Terraform/AI all bind to this — no hidden routes.

## Conventions (normative for new routes)

- Versioned base, JSON only. Reads `GET`; mutations `POST/PUT/PATCH/DELETE`; actions `POST …/verb`.
- Auth: session/JWT/opaque key → `AuthContext`. Public only: `GET /health|/healthz|/version`, `POST /auth/*`, `GET /images/ml` alias (catalog filter).
- Standard envelopes: resource + opID for async ops; task ID for long-running; cursor pagination on lists; `ETag`/`If-Match` on mutating writes (T10); idempotency keys on writes (T10a); `X-Request-ID` echo; structured errors `{code,message,fields,request_id}`.
- Capability map (planned T4a — every route gets one; table below shows intended caps).

## Route inventory (as implemented)

- System: `GET /health|/healthz|/version`, `GET /csrf`, `GET /overview`, `GET /host/overview|/ports|/kernel|/prerequisites|/runtime`, `GET /logs`, `GET|DELETE /traffic[/search]`.
- Auth/session: `POST /auth/login|/logout|/signup|/password/forgot|/password/reset` (+`/login|/logout` aliases), `GET /auth/session`, `GET|PATCH|DELETE /users/me`, API keys `GET|POST /users/me/api-keys`, `DELETE …/{keyId}`.
- Identity/RBAC: `GET|POST|DELETE /users[/{username}]`, `GET|POST|GET|PATCH|DELETE /roles[/{roleId}]`, `GET /permissions`, `GET|PUT|POST|DELETE /roles/{roleId}/permissions[/{permissionId}]` → migrate to `capabilities/role_capabilities/role_assignments` (0017).
- Orgs/tenancy: `GET|POST /orgs`, `GET /orgs/default`, `GET|PATCH /org`, `GET|PATCH|DELETE /orgs/current`, members `GET|POST|PATCH|DELETE /orgs/members[/{username}]`, `POST /orgs/transfer`, `GET /orgs/audit|/events`.
- Groups/teams: `GET|POST /groups`, `GET|PATCH|DELETE /groups/{groupId}`, `GET /groups/{groupId}/projects`, `POST|DELETE …/projects/{projectId}`.
- Projects (core): `GET|POST /projects`, `POST /projects/compose`, `GET|PATCH|DELETE /projects/{projectId}`, `POST …/redeploy|/restart`, scale `GET|PATCH …/scale`, healthcheck `GET|PUT`, autoscale `GET|PUT`, env `GET|POST|POST bulk|PATCH|DELETE …/env[/{envId}]`, secrets `GET|POST|DELETE …/secrets[/{secretId}]`, transfer/avatar/settings/protection/oidc/functions/passport/microfrontends/networking/advanced/retention/security.
- Domains/DNS: `GET|POST /projects/{id}/domains`, `GET …/domains/records`, `GET|DELETE …/domains/{domainId}`, `POST …/verify|/reverify`, `GET …/dns[/records]`, redirects `GET|POST|DELETE|PUT bulk`, firewall `GET|POST|GET|DELETE|PATCH …/firewall/rules[/{ruleId}]`, `GET …/firewall/events|/stats`, `POST …/firewall/whitelist`.
- Compose/git/builds: `GET|PUT|POST validate|GET preview …/compose`, git `GET|PUT …/settings/git*`, `POST …/git/import|/sync`, `PATCH toggles`, branches; builds `GET|POST|POST run …/builds`, `GET …/builds/{id}/logs[/stream]`; deploy-git `POST …/deployments/git`.
- Deployments/rollouts: `GET|POST /projects/{id}/deployments`, upload/source/og/logs, checks `GET|PUT|PATCH`, rollout `PUT|GET`, `POST …/promote|/rollback`, `DELETE`, `GET /projects/{id}/rollouts`, services `GET …/services[/{name}]`, `POST …/scale`.
- Replicas/VMs: `GET /projects/{id}/replicas`, batch start/stop, `GET|POST start|stop|restart|snapshot|restore|recover|DELETE …/replicas/{n}`, `GET …/logs|/metrics|/traffic|/health|/ssh-info`, `POST …/ssh-cert|/exec`, `GET …/console`; compat mirror `GET /vms|/vms/{id}` + same verbs by ID (+`logs/stream`); pool `GET …/pool`, `POST …/pool/drain`; status/liveness/traffic/events.
- Environments: `GET|POST …/environments`, `GET …/environments/available`, `GET|PATCH|DELETE …/environments/{envId}`, `POST …/branch|/domain`, `GET …/range`.
- Hooks/crons/alerts/drains: hooks `GET|POST|DELETE|POST trigger`; crons `GET|POST|GET history|GET|PATCH|DELETE|POST run`; alerts `GET|POST|GET|PATCH|DELETE|POST silence|unsilence`; drains `GET|POST|DELETE|POST test`.
- Analytics/usage/observability: project analytics (usage/timeseries/paths/status-codes/bandwidth/requests/invocations), web-vitals (+beacon/timeseries, lcp/cls/fid aliases), `GET /global/analytics[/timeseries]`, `GET /usage[/bandwidth|/requests|/timeseries]`, `GET /replicas[/{id}]` global.
- Storage/images: volumes global + per-project (`GET|POST|GET|DELETE|POST resize|GET usage`), `GET /guest-bases`, images `GET /images|/base|/base/readiness|/search|/{reference}`, `POST /images/custom|/prune`, `DELETE /images/{reference}`, `GET /images/stats`.
- Servers/nodes: `GET|POST /servers`, `GET|POST heartbeat|GET ssh|DELETE /servers/{id}` → node enrollment hardening (T-series).
- Import/export: `POST /projects/{id}/export|/import`, `PUT …/ssh`, cache `GET stats|POST purge|POST purge/path`.

## Gaps (PLANNED — add with the feature)

Idempotency keys, cursor pagination, ETags, field selection, SSE streams (`/events/stream`, build/log streams beyond existing), token scopes/constraints, team/org switcher endpoints, preview-env lifecycle, marketplace/extension endpoints, billing endpoints (deferred).
