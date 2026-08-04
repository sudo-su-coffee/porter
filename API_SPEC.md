# API Spec — Porter Control API v1.0.0

Base URL: `http://<host>:8080`
Auth: `Authorization: Bearer <PORTER_API_TOKEN>` on every request except `/health`.
All bodies/responses are JSON.

---

## Health

### `GET /health`
No auth required.

```json
{"status": "ok"}
```

---

## VMs

### `GET /vms`
List all VMs across all projects (including standalone).

**200**
```json
[
  {
    "id": "5b2e...",
    "name": "cache",
    "project_id": "",
    "service_name": "",
    "state": "running",
    "image": "redis:7",
    "vcpus": 1,
    "mem_mib": 256,
    "ip_address": "10.42.0.5",
    "ports": [],
    "created_at": "2026-08-04T10:00:00Z",
    "started_at": "2026-08-04T10:00:02Z"
  }
]
```

### `POST /vms`
Create + boot a single VM from an image.

**Request**
```json
{
  "name": "cache",
  "image": "redis:7",
  "vcpus": 1,
  "mem_mib": 256,
  "env": {"REDIS_PASSWORD": "..."},
  "ports": [{"container_port": 6379, "protocol": "tcp"}]
}
```

**202 Accepted** — returns the VM record immediately in `pending` state; boot continues async. Poll `GET /vms/{id}` or subscribe to `GET /events` for state transitions.

### `GET /vms/{id}`
Fetch one VM's current state.

**200** — same shape as list item.
**404** — `{"error": "vm not found"}`

### `POST /vms/{id}/stop`
Graceful stop (ACPI shutdown attempt, then hard stop after 5s timeout). Tears down the tap device.

**200**
```json
{"status": "stopped"}
```

### `POST /vms/{id}/start`
Re-boot a stopped VM (re-runs the full create+start path with the same image/config).

**202 Accepted** — VM record, state transitions to `booting`.

### `DELETE /vms/{id}`
Stop (if running) and permanently remove the VM record. Also removes any domains and SSH gateway entries pointing at it.

**200**
```json
{"status": "deleted"}
```

---

## Projects (compose)

### `GET /projects`
List all projects.

**200**
```json
[
  {
    "id": "9a1c...",
    "name": "my-app",
    "created_at": "2026-08-04T10:00:00Z",
    "network": "10.42.3.0/24",
    "vm_ids": ["...", "..."],
    "source": "compose"
  }
]
```

### `POST /projects/compose`
Create a project from a `docker-compose.yml`. Each service becomes one VM, booted in dependency order.

**Request**
```json
{
  "name": "my-app",
  "compose_yaml": "version: '3'\nservices:\n  api:\n    image: myapp/api:latest\n    ports:\n      - \"3000:3000\"\n  worker:\n    image: myapp/worker:latest\n    depends_on:\n      - api\n"
}
```

**202 Accepted** — returns the Project record; `vm_ids` fills in as each service boots. Subscribe to `GET /events` for progress.

**400** — parse errors are returned verbatim, e.g.:
```json
{"error": "compose parse error: service \"worker\": only image-based services are supported (no `build:`)"}
```

### `GET /projects/{id}`
Fetch one project, including its current `vm_ids`.

### `GET /projects/{id}?expand=vms`
Same as above but embeds full VM objects instead of just IDs, so the dashboard can render a project page in one call.

### `DELETE /projects/{id}`
Stops and deletes every VM in the project, removes all associated domains and SSH gateway entries, then deletes the project record.

**200**
```json
{"status": "deleted"}
```

---

## Domains

### `GET /vms/{id}/domains`
List all domains (stable, preview, custom) currently routed to a VM.

**200**
```json
[
  {"domain": "api.example.com", "type": "stable", "status": "verified"},
  {"domain": "api-a1b2c3.example.com", "type": "preview", "status": "verified"},
  {"domain": "shop.mybrand.com", "type": "custom", "status": "pending"}
]
```

### `POST /vms/{id}/domains`
Attach a custom domain to a VM.

**Request**
```json
{"domain": "shop.mybrand.com"}
```

**202 Accepted**
```json
{
  "domain": "shop.mybrand.com",
  "type": "custom",
  "status": "pending",
  "required_record": {"type": "CNAME", "name": "shop.mybrand.com", "value": "gateway.example.com"}
}
```
Status flips to `verified` once the gateway confirms the CNAME resolves correctly (polled every `PORTER_DOMAIN_VERIFY_INTERVAL`, default 30s). No traffic is routed to the custom domain until `verified`.

### `DELETE /vms/{id}/domains/{domain}`
Detach a custom domain. Stable and preview subdomains cannot be deleted individually — they're managed automatically by the deploy lifecycle (see `DOMAINS_AND_TRAFFIC.md`).

**200**
```json
{"status": "removed"}
```

---

## Traffic log

### `GET /vms/{id}/traffic?limit=200`
Returns the most recent requests from the in-memory ring buffer for that VM (metadata only, no bodies). See `DOMAINS_AND_TRAFFIC.md` §2.

**200**
```json
[
  {"timestamp": "2026-08-04T12:03:41.221Z", "method": "GET", "host": "api.example.com", "path": "/v1/orders", "status": 200, "duration_ms": 42, "remote_ip": "203.0.113.4"}
]
```

---

## Events (SSE)

### `GET /events`
Server-Sent Events stream of state transitions, used by the dashboard instead of polling.

```
event: vm.state
data: {"vm_id": "5b2e...", "state": "running", "ip_address": "10.42.0.5"}

event: vm.state
data: {"vm_id": "5b2e...", "state": "failed", "error": "mkfs.ext4 failed: ..."}

event: project.progress
data: {"project_id": "9a1c...", "booted": 2, "total": 4}

event: traffic.request
data: {"vm_id": "5b2e...", "method": "GET", "path": "/v1/orders", "status": 200, "duration_ms": 42}

event: domain.status
data: {"vm_id": "5b2e...", "domain": "shop.mybrand.com", "status": "verified"}
```

---

## SSH

### `GET /vms/{id}/ssh-info`
Returns what the CLI needs to construct an SSH command through the gateway (no raw key material in the response).

**200**
```json
{
  "gateway_host": "gateway.example.com",
  "gateway_port": 2222,
  "target_name": "my-app-api",
  "command": "ssh my-app-api@gateway.example.com -p 2222"
}
```

### `POST /vms/{id}/ssh-cert`
Requests a short-lived (default 10 min) SSH certificate for interactive login, signed by the gateway CA. Used internally by `porter ssh`; exposed for programmatic/CI use.

**Request**
```json
{"public_key": "ssh-ed25519 AAAA..."}
```

**200**
```json
{
  "certificate": "ssh-ed25519-cert-v01@openssh.com AAAA...",
  "expires_at": "2026-08-04T10:15:00Z"
}
```

---

## Logs

### `GET /vms/{id}/logs?tail=200&follow=true`
Streams the task's stdout/stderr, captured via containerd's normal task IO (piped over the vsock control channel `firecracker-containerd` already uses, same path as `ctr task attach`). `follow=true` keeps the connection open (chunked transfer) and streams new lines as they arrive.

---

## Error format

All non-2xx responses:
```json
{"error": "human-readable message"}
```

## Status codes used

| Code | Meaning |
|---|---|
| 200 | Success (sync operation) |
| 202 | Accepted (async operation started, poll or subscribe for completion) |
| 400 | Bad request (validation, parse errors) |
| 401 | Missing/invalid API token |
| 404 | Resource not found |
| 409 | Conflict (e.g. delete requested while already deleting) |
| 500 | Internal error (host-level failure, logged server-side with full detail) |
