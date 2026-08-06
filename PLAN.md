
# The Idea

Porter is a **self-hosted PaaS** — the Vercel/Fly.io model running on your own
metal — built on **Firecracker microVMs**.

Most "serverless" platforms give you two bad choices:

- **Containers** — fast and cheap, but every workload shares the host kernel.
  Weak isolation for anything multi-tenant or untrusted.
- **Classic VMs** — strong isolation, but slow boots, hundreds of MB of
  baseline overhead, and a huge emulated device surface.

Firecracker is the third option: a real, hardware-isolated microVM that boots in
milliseconds, uses a few MB per instance, and exposes only the minimal devices a
Linux workload needs. It powers AWS Lambda + Fargate and Fly.io's Fly Machines.

**Porter is the self-hosted control plane for that engine.** Instead of spinning
raw Firecracker processes yourself, you deploy **Docker/OCI images** and Porter
boots each one as a kernel-isolated microVM through containerd + the
`aws.firecracker` shim — image pull, snapshots, jailer, networking, and the
in-VM agent are all handled for you.

## Why that matters

| | Containers | Classic VM | Porter (microVM) |
|---|---|---|---|
| Isolation | shared kernel | full VM | **real kernel isolation** |
| Cold start | ms | seconds | **sub-second** |
| Per-instance overhead | tiny | hundreds of MB | **few MB** |
| Multi-tenant safe | ❌ | ✅ | ✅ |
| Runs Docker images | ✅ | ❌ | ✅ |

## The fly.io model, self-hosted

Fly Machines = Firecracker microVMs you control with an API, a fast proxy, and
optional volumes. Porter is that model you run yourself:

- **Deploy** a Docker/OCI image or a `compose.yml` → each service becomes a microVM.
- **Run** fast, isolated, high-density — many workloads per host.
- **Manage** — create / stop / restart / delete, live logs, traffic, overview,
  all from a clean Vercel-style dashboard or the REST API.
- **One binary** — a single pure-Go control plane. No Docker daemon, no
  orchestrator of your own to babysit.

## What it is not

- Not a container orchestrator (K8s/Mesos).
- Not a UI that just spawns `firecracker` processes for you.
- Not "like Docker." The engine is a microVM, not a container runtime;
  containers are only the **packaging** format.

Porter is the point, the microVM is the engine, and your goal is to get a real
app running in a kernel-isolated microVM from a stock Docker image, through a
dashboard that feels like the PaaS you already wish you had.

# Using Porter

Porter is a self-hosted PaaS (Vercel/Fly.io-style) that runs **Docker/OCI images
as Firecracker microVMs**. This guide covers the v0.1.0 flow end to end.

- `idea.md` — what Porter is and why.
- `README.md` — architecture, config reference, API, compose rules.

---

## 1. What you need (Linux host)

A Linux host with KVM (`/dev/kvm`) is required — microVMs cannot boot without
hardware virtualization. Porter itself is a single Go binary.

Install the runtime stack once (see `deploy/host/*.sh`):

```
containerd               # container runtime + content store
aws.firecracker shim     # the runtime that boots a microVM per container
devmapper snapshotter    # image/rootfs layering
firecracker binary       # the VMM
vmlinux kernel           # shared guest kernel (provision with `porter kernel set`)
```

Run the deploy scripts in order:

```bash
bash deploy/host/01-containerd.sh
bash deploy/host/02-shim.sh
bash deploy/host/03-cni.sh
```

Then provision a kernel (local file or remote URL):

```bash
porter kernel set ./vmlinux                 # local
porter kernel set https://…/vmlinux         # remote
```

## 2. Configure

```toml
[server]
listen_addr = ":8080"
base_domain = "porter.test"
state_file  = "porter.db"
api_token   = "change-me"

[firecracker]
containerd_socket = "/run/containerd/containerd.sock"
snapshotter       = "devmapper"
namespace         = "porter"
logs_dir          = "/var/log/porter"

[admin]
username = "admin"
password = "change-me"
```

Every setting has a `PORTER_*` env override (`PORTER_API_TOKEN`,
`PORTER_CONTAINERD_SOCKET`, …). Porter refuses to start without an API token
and admin password.

## 3. Start

```bash
porter server -workers 2 -config porter.toml
# Dashboard: http://localhost:8080  (login with the [admin] credentials)
```

## 4. Deploy an image

Via the dashboard, or the API:

```bash
curl -X POST localhost:8080/vms \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"cache","image":"redis:7-alpine","vcpus":1,"mem_mib":256,
       "ports":[{"container_port":6379,"protocol":"tcp"}]}'
```

Porter pulls the image through containerd and boots it as a microVM
(`pending → booting → running`). Watch the state stream at `GET /events`.

## 5. Deploy a Compose app

The canonical input is a `compose.yml` referencing images — one microVM per
service:

```bash
curl -X POST localhost:8080/projects/compose \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"myapp","compose_yaml":"services:\n  api:\n    image: nginx\n    ports:\n      - 8080:80\n"}'
```

## 6. Volumes (v0.1.0 fold-in)

Persistent storage for VM services, pulled forward from the **Phase 7 (Storage)**
roadmap as a scaffolded `/volumes` API:

```bash
# Create a persistent volume
curl -X POST localhost:8080/volumes \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"data","size_mib":2048}'

GET    /volumes          # list volumes
DELETE /volumes/{id}     # delete a volume
```

Attach a volume to a VM at create time (`POST /vms`, section 4) by referencing
its name; the volume is mounted before the microVM boots.

## 7. Networking / port mapping (v0.1.0 fold-in)

A VM's `ports` now map **host → guest** (`host_port : container_port`), so the
service is reachable on the host. In the `POST /vms` body (pulled forward from
the **Phase 6 (Networking)** roadmap):

```bash
-d '{"name":"cache","image":"redis:7","vcpus":1,"mem_mib":256,
     "ports":[{"container_port":6379,"host_port":16379,"protocol":"tcp"}]}'
# → Redis is now reachable from the host at localhost:16379
```

If `host_port` is omitted it defaults to `container_port`.

## 8. Manage

```bash
POST   /vms/{id}/stop        # graceful stop
POST   /vms/{id}/start       # re-boot
POST   /vms/{id}/restart     # stop + start
DELETE /vms/{id}             # stop + remove
GET    /vms/{id}/logs?tail=200
GET    /vms/{id}/traffic?limit=100
GET    /overview             # host + VM counts
GET    /images               # image catalog
```

## 9. Day-2 operations

- Logs land in `PORTER_LOGS_DIR` and stream live in the dashboard.
- Traffic is recorded per-VM in an in-memory ring and served by the API.
- Stop the process → Porter gracefully stops all tracked VMs.

## Troubleshooting

- `containerd socket not found at /run/containerd/containerd.sock` — containerd
  isn't running. `systemctl start containerd` (or run `deploy/host/01-containerd.sh`).
- `runtime aws.firecracker not registered` — run `deploy/host/02-shim.sh`.
- `pull image ... snapshotter devmapper` — the devmapper snapshotter isn't
  configured; see `deploy/host/02-shim.sh`.
- `kernel set` errors — point `porter kernel set` at a real vmlinux for the
  shim's `/etc/containerd/firecracker-runtime.json`.
  
# Porter Roadmap

Porter is a self-hosted PaaS (Vercel/Fly.io-style) for Firecracker microVMs. This
roadmap is ordered toward a single destination: **v1.0.0, the stable release.**
The `0.x` series is the foundation — each version layers on the last until the
feature set freezes and ships as **v1.0.0**. After that, `1.1.x` becomes the
cloud platform.

```
v0.1.0 … v0.10.0-rc1  →  v1.0.0 (The Release)  →  v1.1.x+ (Cloud Platform)
```

---

# v1.0.0 — The Stable Release

## Theme

**Production-ready.** Everything advertised in the 0.x series works, the API and
deployment model are stable, and it is safe to build real workloads on Porter.

## Complete Features

**Deploy from**
- OCI Image
- Docker Compose
- Git Repository (auto-build)
- Golden MicroVM Image

**Manage**
- Projects
- Services
- MicroVMs
- Domains
- Volumes
- Networks
- Secrets

**Observe**
- Logs
- Metrics
- Traffic
- Events

**Operate**
- Browser Console
- Optional SSH
- REST API
- CLI
- Dashboard

**Developer Experience**
- One-click deployment
- Templates
- Image library
- Build pipeline
- Automatic TLS

The API and deployment model are considered stable at this version.

---

# v0.1.0-beta

## Theme

**Core Runtime & First Deployment** — the first public preview that launches
production-grade Firecracker microVMs from OCI images. This is the foundation
every later version builds on.

## Runtime

- Firecracker integration
- firecracker-containerd integration (`aws.firecracker` shim)
- VM lifecycle management (create / stop / restart / delete)
- Boot Linux kernels (shared `vmlinux`, provisioned with `porter kernel set`)
- RootFS support
- OCI **and Docker** image support (the in-VM agent unpacks any OCI rootfs)
- Snapshot support (basic, devmapper)
- Per-project private subnets + static IP allocation

## Deployments

- Deploy Docker image
- Deploy OCI image
- Deploy Docker Compose (**the canonical input** — a `compose.yml` referencing
  images is parsed into one microVM per service)
- Deploy MicroVM image (image catalog)

## Dashboard

- Login
- Dashboard
- Project list
- VM list
- Deployment history
- Server overview

## VM Management

- Create VM
- Stop VM
- Restart VM
- Delete VM

SSH remains disabled by default.

## Logs

- VM logs (live ring + file tail)
- Application logs

## Networking

- Private bridge
- Static IP
- Port mapping

## Storage

- RootFS
- Volume mount (folded in from Phase 7 — see note below; a `/volumes` API, see
  `POST /volumes` / `GET /volumes` / `DELETE /volumes/{id}`)

## API

- REST API

> **GitHub / GitLab auto-build** is deliberately deferred to **v0.2.0** (builds,
> build cache, and a build pipeline belong to the Developer-Experience release).
> v0.1.0 is OCI-native: you bring an image, Porter runs it.

### Fold-in for v0.1.0 (Phases 6 / 7 / 8)

The **Phase 6 (Networking)**, **Phase 7 (Storage)**, and **Phase 8 (Multi-Host)**
workstreams below keep their own releases (`v0.6.0-beta`, `v0.7.0-beta`,
`v0.8.0-beta`), but three v0.1.0-relevant items are pulled forward into **v0.1.0**
as missing pieces:

- **Port mapping** (from Networking / v0.6) — real **host-port → guest-port**
  binding so a VM service is reachable on the host. A VM's `ports` now carry an
  optional `host_port` (defaults to the container port).
- **Volume mount** (from Storage / v0.7) — made concrete via a `/volumes` API:
  `POST /volumes {name, size_mib}` creates a persistent volume, `GET /volumes`
  lists them, `DELETE /volumes/{id}` removes one; a volume can be attached to a
  VM at create time.
- **Multi-Host scaffold** (Phase 8) — v0.1.0 only scaffolds this with a
  placeholder `POST /servers` register endpoint. The scheduler, resource
  allocation, and VM migration stay deferred to v0.8.0.

---

# v0.2.0-beta

## Theme

**Developer Experience** — everything becomes easier.

### New Features

- GitHub deployment
- GitLab deployment
- Gitea deployment
- Forgejo deployment
- Automatic builds
- Dockerfile builds
- Build cache
- Environment variables
- Secrets
- Configuration management

### UI

- Better dashboard
- Live deployment progress
- Build logs
- Search
- Filtering

---

# v0.3.0-beta

## Theme

**Application Platform** — Porter begins managing applications instead of
individual VMs.

Projects → Deployment → Services → MicroVMs

Features
- Project grouping
- Service grouping
- Multiple deployments
- Version history
- Rollback
- Clone deployment

---

# v0.4.0-beta

## Theme

**Golden Images** — reusable VM templates.

Images: Ubuntu, Debian, Alpine, Builder images, Company images, AI runtime,
Database templates.

Features
- Image library
- Import/export
- Snapshot manager
- Clone image
- Image versioning

---

# v0.5.0-beta

## Theme

**Service Discovery** — Porter starts behaving like Kubernetes.

Features
- ReplicaSets
- Scaling
- Health checks
- Restart policy
- Internal DNS
- Service discovery
- Dependency ordering

```yaml
# compose.yml
api:
  replicas: 3
```
↓ Three MicroVMs.

---

# v0.6.0-beta

## Theme

**Networking** — production traffic paths.

Features
- Overlay networking
- Load balancer
- Firewall
- Domains
- HTTPS
- Let's Encrypt
- Wildcard domains
- Reverse proxy
- Traffic dashboard

---

# v0.7.0-beta

## Theme

**Storage**

Features
- Persistent volumes
- Snapshots
- Scheduled backups
- Restore
- Object storage support
- Shared volumes

---

# v0.8.0-beta

## Theme

**Multi-Host**

Features
- Porter Agent
- Register server
- Cluster view
- Scheduler
- Resource allocation
- VM migration (future)
- Node labels

---

# v0.9.0-beta

## Theme

**Observability**

Features
- Metrics
- Events
- Alerts
- Live traffic
- Live resource graphs
- Audit log
- Notifications

---

# v0.10.0-rc1

## Theme

**Feature Freeze** — no new features. Only bug fixes, performance, memory
optimization, security review, documentation, API/CLI freeze, UI polish,
testing, benchmarking.

Goal: everything advertised works, ready for v1.0.0.

---

# v1.1.0

## Theme

**Cloud Platform** — Porter moves beyond a self-hosted deployment tool.

### High Availability
- Multi-node scheduling
- Automatic failover
- Self-healing
- Rolling updates
- Canary deployments
- Blue/Green deployments

### Scaling
- Autoscaling
- Resource quotas
- Cluster balancing
- Scheduling policies

### Marketplace
One-click deployments: WordPress, Ghost, Grafana, Prometheus, PostgreSQL,
Redis, N8N, Supabase.

### Enterprise
- RBAC
- Organizations
- Teams
- SSO
- Audit logging
- API keys

### Platform
- Plugin SDK
- Extensions
- Webhooks
- Terraform provider
- GitOps
- CI/CD integrations

---

# Long-Term Vision (v2.x)

Porter evolves into a complete MicroVM-native cloud platform.

- Distributed control plane
- Global multi-region deployments
- Edge node support
- ARM and x86 scheduling
- GPU workloads
- AI/ML runtime templates
- Live migration (if supported by the runtime)
- Built-in object storage
- Managed databases
- Service mesh
- eBPF observability
- WASM workload support
- Policy engine
- Marketplace ecosystem
- Hosted Porter Cloud

## Guiding Principles

Every release should reinforce these goals:

- **MicroVM-first:** Firecracker is the execution engine, with containers used
  only as an application packaging format.
- **OCI-native:** Use OCI images and registries rather than inventing a new
  image ecosystem.
- **Developer-friendly:** Support Docker Compose, Git repositories, and
  browser-based management to reduce operational complexity.
- **Pure Go control plane:** Keep Porter itself as a Go-native platform, with
  Firecracker and `firecracker-containerd` as the primary runtime dependencies.
- **Single binary experience:** Installation, upgrades, and operation should
  remain as simple as possible while expanding capabilities over time.
