# docker-compose.yml → microVM mapping rules — Porter v1.0.0

Core rule: **one `services.<name>` entry = one microVM.** No exceptions in v1. There is no "multiple containers sharing one VM" mode.

## 1. Supported top-level keys

| Compose key | Supported | Notes |
|---|---|---|
| `version` | ✅ (ignored) | Parsed but not enforced |
| `services` | ✅ | Required |
| `networks` | ❌ (v1) | All services in a project share one flat bridge subnet automatically; custom network topologies are ignored with a warning |
| `volumes` | ❌ (v1) | Named/bind volumes are not mounted into the VM in v1 — flagged loudly at parse time if present. Roadmap: virtio-fs pass-through |
| `secrets` | ❌ (v1) | Not supported; use `environment` for now |

## 2. Supported service keys

| Service key | Supported | Mapping |
|---|---|---|
| `image` | ✅ **required** | Pulled directly via OCI registry client, flattened to rootfs. `build:` is explicitly **not supported** — services must reference a pre-built, pushed image. Presence of `build:` without `image:` is a hard parse error. |
| `environment` (map or list form) | ✅ | Both `KEY: value` map syntax and `- KEY=value` list syntax accepted. Injected into the guest via a boot-time env file guest-init reads before exec'ing the entrypoint. |
| `ports` | ✅ | `"host:container"`, `"host:container/proto"`, and bare `"container"` forms all parsed. Each becomes a Porter Gateway route (stable + preview subdomain, see `DOMAINS_AND_TRAFFIC.md`) *and* is noted for direct TCP/UDP forwarding (roadmap item for non-HTTP protocols — v1.0.0 gateway is HTTP/1.1 + HTTP/2 only). |
| `depends_on` (list or map form) | ✅ | Used purely for **boot ordering** (topological sort). v1 does **not** implement health-check-based readiness gating — a dependency is considered "ready" the instant its VM reaches `running`, not when the app inside is actually accepting connections. |
| `command` | ✅ | Overrides the image's default `Cmd` (but not `Entrypoint`) — same override semantics as `docker run`. |
| `deploy.resources.limits.cpus` / `.memory` | ✅ | Maps to `vcpus` / `mem_mib`. `cpus: "0.5"` rounds up to 1 vCPU (Firecracker doesn't do fractional vCPUs). `memory: "512m"` / `"1g"` both parsed. |
| `restart` | ❌ (v1) | All VMs restart-on-failure is a roadmap toggle; v1 always leaves a crashed VM in `stopped`/`failed` state for the operator to see and manually restart |
| `healthcheck` | ❌ (v1) | Parsed and ignored; noted above under `depends_on` |
| `build` | ❌ (hard error if present without `image`) | |
| `volumes` (service-level) | ❌ (v1) | Ignored with a warning in the parse response |
| `networks` (service-level) | ❌ (v1) | Ignored — all services in one project land on the same subnet automatically |

## 3. Boot ordering

Services are topologically sorted on `depends_on` before boot. Example:

```yaml
services:
  db:
    image: postgres:16
  api:
    image: myapp/api:latest
    depends_on: [db]
  worker:
    image: myapp/worker:latest
    depends_on: [api, db]
```

Boot order: `db` → `api` → `worker`. If a cycle is detected (`a depends_on b`, `b depends_on a`), the whole project creation is rejected at parse time with a clear error naming the cycle — nothing is booted.

## 4. Networking between services

All services within one project share a single `/24` bridge subnet. Concretely:

- Service `api` can reach service `db` at `db`'s assigned VM IP (e.g. `10.42.3.4:5432`)
- v1.0.0 does **not** ship a DNS resolver inside the bridge — services must be configured to reach each other **by IP**, not by service name, unless you inject the IP via `environment` yourself
- **Roadmap (v1.1):** an embedded DNS stub so `db`, `api`, etc. resolve automatically within a project's subnet

### Workaround for name-based lookup today
The Control API resolves `${PORTER_<SERVICE>_IP}` placeholders in `environment` values automatically, after the dependency has been assigned an IP but before the dependent VM boots:

```yaml
services:
  db:
    image: postgres:16
  api:
    image: myapp/api:latest
    depends_on: [db]
    environment:
      DATABASE_URL: "postgres://user:pass@${PORTER_DB_IP}:5432/app"
```

## 5. Domains for compose services

Given:
```yaml
services:
  api:
    image: myapp/api:latest
    ports:
      - "3000:3000"
```

Porter Gateway auto-registers, at boot:
- `api.<project-name>.<base-domain>` — the **stable** URL for this service, always points at whatever the current live VM for that service is
- `api-<deploy-id>.<project-name>.<base-domain>` — a **preview** URL unique to this specific deploy, useful for testing before promoting

Full domain model: `DOMAINS_AND_TRAFFIC.md` §1.

## 6. What happens to unsupported keys

Parsing is **permissive but loud**: unknown/unsupported top-level or service keys don't hard-fail the whole file — they're collected into a `warnings` array returned alongside the `202` project-creation response, e.g.:

```json
{
  "id": "9a1c...",
  "name": "my-app",
  "warnings": [
    "top-level `volumes` ignored in v1.0.0",
    "service \"api\": `healthcheck` parsed but not enforced (see depends_on readiness note)",
    "service \"worker\": `restart` policy ignored, VMs do not auto-restart in v1.0.0"
  ]
}
```

The only **hard errors** (project creation fully rejected) are:
- A service with `build:` and no `image:`
- A `depends_on` cycle
- A `depends_on` reference to a service name that doesn't exist in the file
- Empty `services:` block
