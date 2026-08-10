# Porter Control API — v1 Reference

> Complete REST API reference for the Porter self-hosted PaaS (Firecracker
> microVMs). Every route below is **real** — registered in
> `backend/internal/api/api.go` and backed by a handler in
> `handlers_impl.go`. Use with **Hoppscotch** (or Postman): base URL
> `http://localhost:8080`.

---

## Auth & Setup (do this first)

1. **Login** — get a bearer token:
   ```http
   POST /login
   Content-Type: application/json

   { "username": "admin", "password": "<porter.toml [admin] password>" }
   ```
   → `{ "token": "<64-hex>", "user": { "username": "admin", "role": "admin" } }`

2. **CSRF** — every non-GET request must send `X-CSRF-Token`. Fetch it once:
   ```http
   GET /csrf
   Authorization: Bearer <token>
   ```
   → `{ "csrf_token": "..." }`

3. **Set the headers on every authenticated call** (except `GET /health`, `GET /csrf`, and the auth endpoints):
   ```
   Authorization: Bearer <token>
   X-CSRF-Token: <csrf_token>      # only required on writes (POST/PATCH/PUT/DELETE)
   X-Porter-Org-Id: <org-id>       # only for org-scoped routes
   ```

> **Hoppscotch tip:** store `token` and `csrf` as environment variables after
> login and reference them in headers as `Authorization: Bearer {{token}}`,
> `X-CSRF-Token: {{csrf}}`. All examples below assume these two.

---

## 1. Health & System

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/health` | none | Liveness. `{ "status": "ok", "version": "..." }` |
| GET | `/healthz` | none | Readiness — pings Postgres; `{ "status", "version", "db" }` (503 when DB down) |
| GET | `/version` | none | Version + runtime identity `{ "version", "engine", "storage", "api_prefix" }` |
| GET | `/csrf` | token | CSRF token for write requests |
| POST | `/feedback` | token | Submit feedback `{ "message", "subject", "category", "project_id" }` → `201` |
| GET | `/feedback` | token | List recent feedback (`?limit=N`) |
| GET | `/overview` | token | Global counts: host, version, vm_running/total, projects, images |
| GET | `/host/overview` | token | Host stats: hostname, os, arch, cpu, mem_total_mb, started_at |
| GET | `/host/kernel` | token | Provisioned guest kernel path |
| GET | `/host/ports` | token | Mapped host→container port table |
| GET | `/logs` | token | Daemon / audit log (`?tail=N`) |
| GET | `/vms` | token | All VMs across all projects |
| GET | `/servers` | token | Registered cluster nodes (live status from heartbeats) |
| POST | `/servers` | token | Register a node `{ "hostname", "address" }` |
| GET | `/servers/{id}` | token | Node details (cpu/mem/os/arch/version/projects/vms/last_seen) |
| POST | `/servers/{id}/heartbeat` | token | Node status report `{ "id", "status", "vcpus", "mem_mib", ... }` |
| GET | `/servers/{id}/ssh` | token | SSH connection info for a node |
| DELETE | `/servers/{id}` | token | Unregister a node |
| GET | `/traffic` | token | All proxied traffic (ring) |
| DELETE | `/traffic` | token | Clear traffic history |
| GET | `/traffic/search` | token | Search traffic `?q=` |
| GET | `/events` | token | SSE live-event stream (`vm.state`, `replica.health`, …) |
| GET | `/usage` | token | Usage metering overview |
| GET | `/usage/bandwidth` | token | Global bandwidth usage |
| GET | `/usage/requests` | token | Global request counts |
| GET | `/usage/timeseries` | token | Global timeseries |
| GET | `/global/analytics` | token | Global analytics aggregates |
| GET | `/global/analytics/timeseries` | token | Global analytics timeseries |

---

## 2. Auth & Account

| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/login` | none | Login → bearer token (alias of `/auth/login`) |
| POST | `/auth/login` | none | Canonical login endpoint |
| POST | `/auth/logout` | none | Logout (no-op; tokens are stateless) |
| POST | `/auth/signup` | none | Single-tenant notice (returns 201 + note) |
| POST | `/auth/password/forgot` | none | Reports reset path for config-admin (no email backend) |
| POST | `/auth/password/reset` | none | Same — reset via `porter.toml [admin] password` |
| GET | `/auth/session` | token | Session check `{ "authenticated": true, "username": "..." }` |
| GET | `/users/me` | token | Current user profile |
| PATCH | `/users/me` | token | Update profile (note: config-admin not persisted) |
| DELETE | `/users/me` | token | Deletion blocked for bootstrap admin |
| GET | `/users/me/api-keys` | token | List my API keys |
| POST | `/users/me/api-keys` | token | Create API key `{ "name" }` → returns `{ "token" }` once |
| DELETE | `/users/me/api-keys/{keyId}` | token | Revoke an API key |

**Login request/response:**
```http
POST /auth/login
{ "username": "admin", "password": "secret" }
```
```json
{ "token": "abc...", "user": { "username": "admin", "role": "admin" } }
```

**Per-user tokens:** additional users created via `POST /users` each get their
own API token on login, resolving to their RBAC role (admin/member/viewer).

---

## 3. Organizations

Org scope is set with the `X-Porter-Org-Id` header.

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/orgs` | token | List orgs |
| GET | `/orgs/default` | token | The default org |
| POST | `/orgs` | token | Create org `{ "name" }` |
| GET | `/orgs/current` | token | Current org (header-scoped) |
| PATCH | `/orgs/current` | token | Update current org |
| DELETE | `/orgs/current` | token | Delete current org (default org cannot be deleted) |
| GET | `/orgs/members` | token | List org members |
| POST | `/orgs/members` | token | Add member `{ "username", "role" }` |
| PATCH | `/orgs/members/{username}` | token | Change member role |
| DELETE | `/orgs/members/{username}` | token | Remove member |
| GET | `/orgs/audit` | token | Org audit trail |
| POST | `/orgs/transfer` | token | Transfer org ownership |
| GET | `/orgs/events` | token | Org-scoped SSE events |

---

## 4. Groups (project collections)

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/groups` | token | List groups |
| POST | `/groups` | token | Create group `{ "name" }` |
| GET | `/groups/{groupId}` | token | Get group |
| PATCH | `/groups/{groupId}` | token | Update group |
| DELETE | `/groups/{groupId}` | token | Delete group |
| GET | `/groups/{groupId}/projects` | token | Projects in group |
| POST | `/groups/{groupId}/projects/{projectId}` | token | Add project to group |
| DELETE | `/groups/{groupId}/projects/{projectId}` | token | Remove project from group |

---

## 5. Projects

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/projects` | token | List projects |
| POST | `/projects` | token | Create project (single image) |
| POST | `/projects/compose` | token | Create project from compose YAML |
| GET | `/projects/{projectId}` | token | Get project (`?expand=vms` adds replicas) |
| PATCH | `/projects/{projectId}` | token | Update project |
| DELETE | `/projects/{projectId}` | token | Delete project + all replicas |
| POST | `/projects/{projectId}/redeploy` | token | Redeploy current image |
| POST | `/projects/{projectId}/restart` | token | Restart project pool |
| GET | `/projects/{projectId}/status` | token | Aggregate status |
| GET | `/projects/{projectId}/liveness` | token | Liveness probe |
| POST | `/projects/{projectId}/export` | token | Export project manifest |
| POST | `/projects/{projectId}/import` | token | Import project manifest |
| POST | `/projects/{projectId}/transfer` | token | Transfer project |
| POST | `/projects/{projectId}/avatar` | token | Set avatar |

**Create single-image project:**
```http
POST /projects
{ "name": "my-app", "image": "nginx:latest", "replicas": 1,
  "env": { "FOO": "bar" }, "ports": [{ "container_port": 80 }] }
```
→ `201 { "id": "...", "state": "pending", ... }`

**Compose import** — `POST /projects/compose` with `{ "compose_yaml": "..." }`
(image-based services only; `build:` is rejected). Boots a per-service replica pool.

---

### 5a. Scale & policies

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/projects/{id}/scale` | token | Current scale |
| PATCH | `/projects/{id}/scale` | token | Scale `{ "replicas": N }` |
| GET | `/projects/{id}/healthcheck` | token | Healthcheck config |
| PUT | `/projects/{id}/healthcheck` | token | Set healthcheck `{ "path", "interval_s", "timeout_s" }` |
| GET | `/projects/{id}/autoscale` | token | Autoscale policy |
| PUT | `/projects/{id}/autoscale` | token | Set autoscale policy |

### 5b. Environment variables & secrets

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/projects/{id}/env` | token | List env vars |
| POST | `/projects/{id}/env` | token | Set env var `{ "key", "value" }` |
| POST | `/projects/{id}/env/bulk` | token | Bulk set `{ "env": {k:v} }` |
| PATCH | `/projects/{id}/env/{envId}` | token | Update env var |
| DELETE | `/projects/{id}/env/{envId}` | token | Delete env var |
| GET | `/projects/{id}/secrets` | token | List secrets (names only) |
| POST | `/projects/{id}/secrets` | token | Create secret `{ "key", "value" }` (AES-GCM at rest) |
| DELETE | `/projects/{id}/secrets/{secretId}` | token | Delete secret |

### 5c. Domains & DNS

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/projects/{id}/domains` | token | List domains |
| POST | `/projects/{id}/domains` | token | Add domain `{ "domain" }` |
| GET | `/projects/{id}/domains/records` | token | DNS records for domains |
| GET | `/projects/{id}/domains/{domainId}` | token | Get domain |
| DELETE | `/projects/{id}/domains/{domainId}` | token | Remove domain |
| POST | `/projects/{id}/domains/{domainId}/verify` | token | Verify (real DNS probe) |
| POST | `/projects/{id}/domains/{domainId}/reverify` | token | Re-verify |
| GET | `/projects/{id}/dns` | token | Project DNS overview |
| GET | `/projects/{id}/dns/records` | token | DNS records (alias) |

### 5d. Compose

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/projects/{id}/compose` | token | Get compose YAML |
| PUT | `/projects/{id}/compose` | token | Replace compose YAML |
| POST | `/projects/{id}/compose/validate` | token | Validate compose file |
| GET | `/projects/{id}/compose/preview` | token | Parsed service preview |

### 5e. Logs, metrics, traffic, events

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/projects/{id}/logs` | token | Project logs (`?tail=N`) |
| GET | `/projects/{id}/metrics` | token | Project metric series |
| GET | `/projects/{id}/traffic` | token | Project traffic |
| GET | `/projects/{id}/events` | token | Project events |
| GET | `/projects/{id}/pool` | token | Replica pool status |
| POST | `/projects/{id}/pool/drain` | token | Drain pool (roll replacement) |

### 5f. Replicas (microVMs)

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/projects/{id}/replicas` | token | List replicas |
| POST | `/projects/{id}/replicas/batch/start` | token | Start all stopped replicas |
| POST | `/projects/{id}/replicas/batch/stop` | token | Stop all replicas |
| GET | `/projects/{id}/replicas/{n}` | token | Get replica by index |
| POST | `/projects/{id}/replicas/{n}/start` | token | Start replica |
| POST | `/projects/{id}/replicas/{n}/stop` | token | Stop replica |
| POST | `/projects/{id}/replicas/{n}/restart` | token | Restart replica |
| DELETE | `/projects/{id}/replicas/{n}` | token | Delete replica |
| GET | `/projects/{id}/replicas/{n}/logs` | token | Replica logs (`?tail=N`) |
| GET | `/projects/{id}/replicas/{n}/metrics` | token | Replica metric series |
| GET | `/projects/{id}/replicas/{n}/traffic` | token | Replica traffic |
| GET | `/projects/{id}/replicas/{n}/health` | token | Replica health status |
| GET | `/projects/{id}/replicas/{n}/ssh-info` | token | SSH endpoint info `{ "user", "host", "port" }` |
| POST | `/projects/{id}/replicas/{n}/ssh-cert` | token | Issue SSH cert |
| POST | `/projects/{id}/replicas/{n}/exec` | token | `task.Exec` bridge `{ "command", "args" }` |
| GET | `/projects/{id}/replicas/{n}/console` | token | Web console (WS) |

**Replica log example:**
```http
GET /projects/p1/replicas/0/logs?tail=200
```
```json
{ "logs": ["INFO listening on :80", ...] }
```

### 5g. Deployments (promote / rollback / checks / rollout)

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/projects/{id}/deployments` | token | List deployments |
| POST | `/projects/{id}/deployments` | token | Create deployment |
| GET | `/projects/{id}/deployments/upload` | token | Upload deployment payload |
| GET | `/projects/{id}/deployments/{d}` | token | Get deployment |
| GET | `/projects/{id}/deployments/{d}/checks` | token | Deployment checks |
| PUT | `/projects/{id}/deployments/{d}/checks` | token | Upsert checks |
| PATCH | `/projects/{id}/deployments/{d}/checks/{name}` | token | Set one check |
| PUT | `/projects/{id}/deployments/{d}/rollout` | token | Set rollout weight `{ "weight": 50 }` |
| GET | `/projects/{id}/deployments/{d}/logs` | token | Deployment logs |
| POST | `/projects/{id}/deployments/{d}/promote` | token | Promote to production |
| POST | `/projects/{id}/deployments/{d}/rollback` | token | Rollback to this deployment |
| GET | `/projects/{id}/deployments/{d}/source` | token | Deployment source info |
| GET | `/projects/{id}/deployments/{d}/og` | token | Deployment OG/asset |

### 5h. Project settings (Vercel-style)

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/projects/{id}/settings/framework` | token | Auto-detected framework `{ "framework", "install/build/start_command", "detected" }` |
| GET/PATCH | `/projects/{id}/settings/general` | token | General settings |
| GET/PUT | `/projects/{id}/settings/build` | token | Build settings |
| GET/POST | `/projects/{id}/settings/checks` | token | Deployment checks |
| GET/PUT | `/projects/{id}/settings/rollout` | token | Rollout settings |
| GET/PUT | `/projects/{id}/settings/build-machine` | token | Build machine size |
| POST | `/projects/{id}/settings/ignore-command` | token | Set ignore command |
| GET | `/projects/{id}/settings/framework` | token | Detected framework |
| GET/PUT | `/projects/{id}/settings/git` | token | Git repo settings |
| POST | `/projects/{id}/settings/git/sync` | token | Sync git repo |
| PATCH | `/projects/{id}/settings/git/toggles` | token | Git feature toggles |
| GET/PUT | `/projects/{id}/settings/git/lfs` | token | Git LFS settings |
| GET/PUT | `/projects/{id}/settings/security` | token | Security settings |
| GET/PUT | `/projects/{id}/settings/retention` | token | Log retention |
| GET/PUT | `/projects/{id}/settings/networking` | token | Networking settings |
| GET/PUT | `/projects/{id}/settings/advanced` | token | Advanced settings |
| GET/PUT | `/projects/{id}/settings/passport` | token | Passport settings |
| GET/PUT | `/projects/{id}/settings/microfrontends` | token | Microfrontend settings |
| GET/PUT | `/projects/{id}/settings/deployment-protection` | token | Deployment protection |
| GET/PUT | `/projects/{id}/settings/oidc` | token | OIDC settings |
| GET/PUT | `/projects/{id}/settings/functions` | token | Functions settings |

### 5i. Environments

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/projects/{id}/environments` | token | List environments |
| POST | `/projects/{id}/environments` | token | Create environment |
| GET | `/projects/{id}/environments/available` | token | Available env presets |
| GET | `/projects/{id}/environments/{e}` | token | Get environment |
| PATCH | `/projects/{id}/environments/{e}` | token | Update environment |
| DELETE | `/projects/{id}/environments/{e}` | token | Delete environment |
| POST | `/projects/{id}/environments/{e}/branch` | token | Set git branch |
| POST | `/projects/{id}/environments/{e}/domain` | token | Set domain |

### 5j. Hooks (webhooks)

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/projects/{id}/hooks` | token | List webhooks |
| POST | `/projects/{id}/hooks` | token | Create webhook |
| DELETE | `/projects/{id}/hooks/{hookId}` | token | Delete webhook |
| POST | `/projects/{id}/hooks/{hookId}/trigger` | token | Trigger webhook manually |

### 5k. Crons (scheduled jobs)

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/projects/{id}/crons` | token | List crons |
| POST | `/projects/{id}/crons` | token | Create cron `{ "name", "schedule", "image", "command" }` |
| GET | `/projects/{id}/crons/history` | token | Cron run history |
| GET | `/projects/{id}/crons/{cronId}` | token | Get cron |
| PATCH | `/projects/{id}/crons/{cronId}` | token | Update cron |
| DELETE | `/projects/{id}/crons/{cronId}` | token | Delete cron |
| POST | `/projects/{id}/crons/{cronId}/run` | token | Run now |

### 5l. Members & roles (RBAC)

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/projects/{id}/members` | token | List project members |
| POST | `/projects/{id}/members` | token | Add member `{ "username", "role" }` |
| GET | `/projects/{id}/members/{username}` | token | Get member |
| PATCH | `/projects/{id}/members/{username}` | token | Update member role |
| DELETE | `/projects/{id}/members/{username}` | token | Remove member |
| POST | `/projects/{id}/members/invite` | token | Invite member |

### 5m. Log drains

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/projects/{id}/drains` | token | List log drains |
| POST | `/projects/{id}/drains` | token | Create drain |
| DELETE | `/projects/{id}/drains/{drainId}` | token | Delete drain |
| POST | `/projects/{id}/drains/{drainId}/test` | token | Test drain delivery |

### 5n. Alerts

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/projects/{id}/alerts` | token | List alerts |
| POST | `/projects/{id}/alerts` | token | Create alert `{ "name", "metric", "threshold", "op" }` |
| GET | `/projects/{id}/alerts/{alertId}` | token | Get alert |
| PATCH | `/projects/{id}/alerts/{alertId}` | token | Update alert |
| DELETE | `/projects/{id}/alerts/{alertId}` | token | Delete alert |
| POST | `/projects/{id}/alerts/{alertId}/silence` | token | Silence alert |
| POST | `/projects/{id}/alerts/{alertId}/unsilence` | token | Unsilence alert |

### 5o. Redirects

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/projects/{id}/redirects` | token | List redirects |
| POST | `/projects/{id}/redirects` | token | Create redirect |
| DELETE | `/projects/{id}/redirects/{redirectId}` | token | Delete redirect |
| PUT | `/projects/{id}/redirects/bulk` | token | Bulk upsert redirects |

### 5p. Analytics & observability

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/projects/{id}/analytics/usage` | token | Requests/bandwidth totals |
| GET | `/projects/{id}/analytics/usage/timeseries` | token | Usage timeseries |
| GET | `/projects/{id}/analytics/paths` | token | Top paths |
| GET | `/projects/{id}/analytics/status-codes` | token | Status-code distribution |
| GET | `/projects/{id}/analytics/bandwidth` | token | Bandwidth series |
| GET | `/projects/{id}/analytics/requests` | token | Request series |
| GET | `/projects/{id}/analytics/invocations` | token | Invocation series |
| GET | `/projects/{id}/observability/web-vitals` | token | Web-vitals aggregates |
| POST | `/projects/{id}/observability/web-vitals/beacon` | token | Ingest web-vitals beacon |
| GET | `/projects/{id}/observability/web-vitals/timeseries` | token | Web-vitals timeseries |
| GET | `/projects/{id}/observability/lcp` | token | LCP series |
| GET | `/projects/{id}/observability/cls` | token | CLS series |
| GET | `/projects/{id}/observability/fid` | token | FID series |

### 5q. Firewall

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/projects/{id}/firewall/rules` | token | List rules |
| POST | `/projects/{id}/firewall/rules` | token | Create rule `{ "action": "deny", "src_ip", "src_cidr", "port" }` |
| GET | `/projects/{id}/firewall/rules/{ruleId}` | token | Get rule |
| DELETE | `/projects/{id}/firewall/rules/{ruleId}` | token | Delete rule |
| PATCH | `/projects/{id}/firewall/rules/{ruleId}` | token | Update rule |
| GET | `/projects/{id}/firewall/events` | token | Firewall event log |
| GET | `/projects/{id}/firewall/stats` | token | Rule counters |
| POST | `/projects/{id}/firewall/whitelist` | token | Add whitelist entry |

### 5r. Cache

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/projects/{id}/cache/stats` | token | Cache hit-rate stats |
| POST | `/projects/{id}/cache/purge` | token | Purge entire cache |
| POST | `/projects/{id}/cache/purge/path` | token | Purge by path `{ "path" }` |

### 5s. Git & builds

| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/projects/{id}/git/import` | token | Import git repo `{ "repo_url", "branch" }` |
| POST | `/projects/{id}/deployments/git` | token | Trigger git deploy (clone→build→boot) |
| GET | `/projects/{id}/builds` | token | List builds |
| POST | `/projects/{id}/builds` | token | Create build |
| POST | `/projects/{id}/builds/run` | token | Run build |
| GET | `/projects/{id}/builds/{buildId}/logs` | token | Build log stream |
| GET | `/projects/{id}/git/branches` | token | List repo branches |
| GET | `/projects/{id}/rollouts` | token | List rollouts |
| GET | `/projects/{id}/services` | token | List compose services |
| GET | `/projects/{id}/services/{serviceName}` | token | Get service |
| POST | `/projects/{id}/services/{serviceName}/scale` | token | Scale service `{ "replicas": N }` |
| GET | `/projects/{id}/networks` | token | List project networks |
| POST | `/projects/{id}/networks` | token | Create network |
| PUT | `/projects/{id}/ssh` | token | Toggle SSH gateway for project |

---

## 6. Volumes

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/volumes` | token | List volumes |
| POST | `/volumes` | token | Create volume `{ "name", "size_mib" }` (real dir + sparse data.img) |
| GET | `/volumes/{volumeId}` | token | Get volume |
| DELETE | `/volumes/{volumeId}` | token | Delete volume + data |
| POST | `/volumes/{volumeId}/resize` | token | Resize `{ "size_mib" }` |
| GET | `/volumes/{volumeId}/usage` | token | Real disk usage |

---

## 7. Images

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/images` | token | List catalog + custom images |
| GET | `/images/search` | token | Search catalog (`?q=`) |
| GET | `/images/ml` | token | ML image catalog filter |
| GET | `/images/{reference}` | token | Get image by ref |
| DELETE | `/images/{reference}` | token | Delete image |
| POST | `/images/prune` | token | Prune unused images |
| GET | `/images/stats` | token | Image store stats |
| POST | `/images/custom` | token | Upload custom microVM `.zip` (rootfs.ext4 + vmlinux) — multipart |

---

## 8. Users, Roles & Permissions

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/users` | token | List users |
| POST | `/users` | token | Create user `{ "username", "password", "role" }` |
| DELETE | `/users/{username}` | token | Delete user |
| GET | `/roles` | token | List roles |
| POST | `/roles` | token | Create role |
| GET | `/roles/{roleId}` | token | Get role |
| PATCH | `/roles/{roleId}` | token | Update role |
| DELETE | `/roles/{roleId}` | token | Delete role |
| GET | `/permissions` | token | List all permission codes |
| GET | `/roles/{roleId}/permissions` | token | Role's permissions |
| PUT | `/roles/{roleId}/permissions` | token | Replace role permissions |
| POST | `/roles/{roleId}/permissions/{permissionId}` | token | Grant permission |
| DELETE | `/roles/{roleId}/permissions/{permissionId}` | token | Revoke permission |

**Create user:**
```http
POST /users
{ "username": "dev", "password": "s3cret", "role": "member" }
```

---

## 9. Compat / convenience (VM-centric)

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/vms` | token | List all VMs |
| GET | `/vms/{replicaId}` | token | Get VM by replica ID |
| POST | `/vms/{replicaId}/start` | token | Start VM |
| POST | `/vms/{replicaId}/stop` | token | Stop VM |
| POST | `/vms/{replicaId}/restart` | token | Restart VM |
| DELETE | `/vms/{replicaId}` | token | Delete VM |

---

## Status codes

| Code | Meaning |
|---|---|
| `200` | OK |
| `201` | Created |
| `202` | Accepted (async boot/deploy started) |
| `204` | No content |
| `400` | Bad request (missing/invalid body) |
| `401` | Unauthorized (bad/missing token) |
| `403` | Forbidden (missing permission or CSRF token) |
| `404` | Not found |
| `409` | Conflict |
| `429` | Rate limited |
| `500` | Internal error |

---

## SSE events (`GET /events`)

Named events the dashboard subscribes to:

| Event | Payload hint |
|---|---|
| `vm.state` | `{ "vm_id", "state" }` |
| `replica.health` | `{ "vm_id", "health" }` |
| `project.progress` | `{ "project_id", "progress", "stage" }` |
| `pool.updated` | `{ "project_id", "desired", "running" }` |
| `domain.status` | `{ "domain_id", "status" }` |
| `traffic.request` | `{ "project_id", "path", "status", "duration_ms" }` |

---

## Hoppscotch quick test sequence

```text
1.  GET  /health                → liveness + version
2.  GET  /healthz               → readiness (db up/down)
3.  GET  /version               → version + runtime identity
4.  POST /auth/login            → copy token
5.  GET  /csrf                  → copy csrf
6.  Set env: token, csrf
7.  POST /feedback              → submit feedback {message}
8.  GET  /feedback              → list feedback
9.  GET  /overview              → system counters
10. POST /projects              → create app (image deploy)
11. GET  /projects              → list
12. GET  /projects/{id}/replicas/0/logs → real logs
13. POST /projects/{id}/replicas/0/stop  → stop a replica
14. POST /projects/{id}/replicas/0/start → start it again
15. GET  /vms/{id}/ssh-info     → SSH endpoint
16. GET  /traffic               → proxied requests
17. GET  /logs                  → daemon log
```

> **Version note:** this API surface is `v0.1.0-beta-dev` until the maintainer
> approves a release bump to v1.0.0. Routes are stable for the v1.0.0-rc surface.
