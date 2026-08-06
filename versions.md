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

## API

- REST API

> **GitHub / GitLab auto-build** is deliberately deferred to **v0.2.0** (builds,
> build cache, and a build pipeline belong to the Developer-Experience release).
> v0.1.0 is OCI-native: you bring an image, Porter runs it.

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
