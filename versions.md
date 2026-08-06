# Porter Roadmap

* **Phase 1:** Build the Foundation (v0.1.0 → v0.10.0)
* **Phase 2:** Production Release (v1.0.0)
* **Phase 3:** Cloud Platform (v1.1.x+)


## Phase I — Foundation (v0.x)

The goal of the 0.x series is to transform Porter from a Firecracker wrapper into a complete MicroVM application platform.

At the end of this phase, Porter should allow users to deploy applications from Docker images, Docker Compose files, Git repositories, or reusable MicroVM images through a modern web interface.

---

# v0.1.0-beta

## Theme

**Core Runtime & First Deployment**

## Objectives

Deliver the first public preview capable of launching production-grade Firecracker MicroVMs from OCI images.

## Runtime

* Firecracker integration
* firecracker-containerd integration
* VM lifecycle management
* Boot Linux kernels
* RootFS support
* OCI image support
* Snapshot support (basic)

## Deployments

* Deploy Docker image
* Deploy OCI image
* Deploy Docker Compose
* Deploy MicroVM image

## Dashboard

* Login
* Dashboard
* Project list
* VM list
* Deployment history
* Server overview

## VM Management

* Create VM
* Stop VM
* Restart VM
* Delete VM
* Console
* Browser terminal

SSH remains disabled by default.

## Logs

* VM logs
* Application logs

## Networking

* Private bridge
* Static IP
* Port mapping

## Storage

* RootFS
* Volume mount

## API

* REST API
* CLI

---

# v0.2.0-beta

## Theme

**Developer Experience**

Everything should become easier.

### New Features

* GitHub deployment
* GitLab deployment
* Gitea deployment
* Forgejo deployment
* Automatic builds
* Dockerfile builds
* Build cache
* Environment variables
* Secrets
* Configuration management

### UI

* Better dashboard
* Live deployment progress
* Build logs
* Search
* Filtering

---

# v0.3.0-beta

## Theme

**Application Platform**

Porter begins managing applications instead of individual VMs.

### Projects

Projects

↓

Deployment

↓

Services

↓

MicroVMs

Features

* Project grouping
* Service grouping
* Multiple deployments
* Version history
* Rollback
* Clone deployment

---

# v0.4.0-beta

## Theme

**Golden Images**

Support reusable VM templates.

### Images

* Ubuntu
* Debian
* Alpine
* Builder images
* Company images
* AI runtime
* Database templates

Features

* Image library
* Import/export
* Snapshot manager
* Clone image
* Image versioning

---

# v0.5.0-beta

## Theme

**Service Discovery**

Porter starts behaving like Kubernetes.

Features

* ReplicaSets
* Scaling
* Health checks
* Restart policy
* Internal DNS
* Service discovery
* Dependency ordering

Compose

```yaml
api:
  replicas: 3
```

↓

Three MicroVMs.

---

# v0.6.0-beta

## Theme

**Networking**

Features

* Overlay networking
* Load balancer
* Firewall
* Domains
* HTTPS
* Let's Encrypt
* Wildcard domains
* Reverse proxy
* Traffic dashboard

---

# v0.7.0-beta

## Theme

**Storage**

Features

* Persistent volumes
* Snapshots
* Scheduled backups
* Restore
* Object storage support
* Shared volumes

---

# v0.8.0-beta

## Theme

**Multi-Host**

Features

* Porter Agent
* Register server
* Cluster view
* Scheduler
* Resource allocation
* VM migration (future)
* Node labels

Dashboard

```
Cluster

Server A

Server B

Server C
```

---

# v0.9.0-beta

## Theme

**Observability**

Features

* Metrics
* Events
* Alerts
* Live traffic
* Live resource graphs
* Audit log
* Notifications

---

# v0.10.0-rc1

## Theme

**Feature Freeze**

No new features.

Only

* Bug fixes
* Performance
* Memory optimization
* Security review
* Documentation
* API freeze
* CLI freeze
* UI polish
* Testing
* Benchmarking

Goal

Everything advertised works.

---

# Phase II

# v1.0.0

## Stable Release

Production-ready.

### Complete Features

Deploy from

* OCI Image
* Docker Compose
* Git Repository
* Golden MicroVM Image

Manage

* Projects
* Services
* MicroVMs
* Domains
* Volumes
* Networks
* Secrets

Observe

* Logs
* Metrics
* Traffic
* Events

Operate

* Browser Console
* Optional SSH
* REST API
* CLI
* Dashboard

Developer Experience

* One-click deployment
* Templates
* Image library
* Build pipeline
* Automatic TLS

The API and deployment model are considered stable.

---

# Phase III

# v1.1.0

## Theme

**Cloud Platform**

This is where Porter moves beyond a self-hosted deployment tool.

### High Availability

* Multi-node scheduling
* Automatic failover
* Self-healing
* Rolling updates
* Canary deployments
* Blue/Green deployments

### Scaling

* Autoscaling
* Resource quotas
* Cluster balancing
* Scheduling policies

### Marketplace

One-click deployments

* WordPress
* Ghost
* Grafana
* Prometheus
* PostgreSQL
* Redis
* N8N
* Supabase

### Enterprise

* RBAC
* Organizations
* Teams
* SSO
* Audit logging
* API keys

### Platform

* Plugin SDK
* Extensions
* Webhooks
* Terraform provider
* GitOps
* CI/CD integrations

---

# Long-Term Vision (v2.x)

Porter evolves into a complete MicroVM-native cloud platform.

* Distributed control plane
* Global multi-region deployments
* Edge node support
* ARM and x86 scheduling
* GPU workloads
* AI/ML runtime templates
* Live migration (if supported by the runtime)
* Built-in object storage
* Managed databases
* Service mesh
* eBPF observability
* WASM workload support
* Policy engine
* Marketplace ecosystem
* Hosted Porter Cloud

## Guiding Principles

Every release should reinforce these goals:

* **MicroVM-first:** Firecracker is the execution engine, with containers used only as an application packaging format.
* **OCI-native:** Use OCI images and registries rather than inventing a new image ecosystem.
* **Developer-friendly:** Support Docker Compose, Git repositories, and browser-based management to reduce operational complexity.
* **Pure Go control plane:** Keep Porter itself as a Go-native platform, with Firecracker and `firecracker-containerd` as the primary runtime dependencies.
* **Single binary experience:** Installation, upgrades, and operation should remain as simple as possible while expanding capabilities over time.

This roadmap provides a coherent path from an initial single-host MicroVM platform to a mature orchestration system without attempting to replicate the full complexity of Kubernetes from the outset.
