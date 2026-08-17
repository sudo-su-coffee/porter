# Porter: Coolify-Class Firecracker Platform Roadmap

## Product position

Porter should be positioned as a **microVM-native self-hosted application platform**: the developer experience should approach Coolify/Vercel, while every application workload runs in a Firecracker microVM rather than a shared container runtime. Coolify’s official feature surface includes Git and image deployments, preview environments, custom domains, automatic TLS, backups, webhooks, APIs, collaboration, monitoring, notifications, and rolling updates.[^1] [^2] Porter should match that workflow while making isolation, immutable artifacts, fast startup, snapshots, and per-deployment VM pools its differentiators.

The correct product principle is:

> **Coolify-like control plane; Firecracker-native data plane.**

Porter should not reproduce Docker internally. Docker/OCI is an input and distribution format. BuildKit produces the application artifact, Porter combines it with a validated guest base and guest agent, and Firecracker runs the resulting kernel/rootfs pair.

## Capability map

| Coolify-class capability | Porter implementation | Priority |
|---|---|---:|
| Git, Dockerfile, and OCI image deployment | Git webhook/API → BuildKit solve → OCI artifact → guest-base conversion → immutable `vmlinux`/`rootfs.ext4` | P0 |
| Application versions | User tag, immutable deployment ID, artifact digest, guest-base digest, environment, VM pool | P0 |
| Preview environments | PR/tag deployment with isolated VM pool, scoped secrets, preview domain, automatic cleanup | P0 |
| Production environments | Stable deployment pointer, readiness gate, gradual traffic shift, rollback pointer | P0 |
| Rolling updates | Start new VM pool, health-check it, shift traffic, drain old VMs, preserve rollback pool | P0 |
| A/B testing | Weighted deployment routing plus stable signed cookie and optional header targeting | P1 |
| Custom domains | Durable domain-to-project/environment/deployment mapping | P0 |
| Automatic TLS | Go ACME certificate manager, renewal state, certificate storage, HTTP-01/DNS-01 strategy | P0 |
| Environment variables and secrets | Environment-scoped encrypted secrets; never copy production secrets into preview builds | P0 |
| Persistent storage | Named volumes, attachment declarations, snapshots/backups, opt-in per deployment | P0 |
| Databases and services | Curated Firecracker service images and volume policies; no arbitrary one-click claim until images are validated | P1 |
| Health checks | Guest-agent readiness plus HTTP/TCP checks; five-failure replacement/rollback policy | P0 |
| Logs and terminal | Guest-agent log stream and controlled exec protocol; no host shell exposure | P1 |
| Backups | Scheduled database/volume/control-plane backups to S3-compatible storage | P1 |
| Monitoring and alerts | VM state, CPU, memory, disk, network, health, build, backup, and certificate metrics | P1 |
| Team collaboration | Organizations, projects, roles, scoped API keys, audit events, deployment approvals | P1 |
| Webhooks and automation | Signed Git webhooks, deployment webhooks, outbound notifications, idempotency keys | P0 |
| Multi-server support | Single-host scheduler first; host-agent and placement abstraction for later cluster mode | P2 |

## Target architecture

Porter has four explicit planes.

| Plane | Responsibility | Required components |
|---|---|---|
| Control plane | API, authentication, organizations, projects, deployments, domains, secrets, policies | Go API, PostgreSQL, migration runner, RBAC, audit log |
| Build plane | Convert source or OCI input into immutable guest artifacts | BuildKit, OCI resolver, layer unpacker, guest-base merger, artifact verifier, build cache |
| Runtime plane | Run and recover isolated workloads | Firecracker manager, Unix-socket client, guest-agent protocol, TAP/network manager, volume manager |
| Edge plane | Route public traffic to healthy VM replicas | Go gateway, TLS/ACME, domain resolver, weighted deployment selector, access logs |

The single-host implementation should keep these boundaries even when all processes run on one machine. Later, a cluster scheduler can move the build plane and runtime plane to host agents without changing the deployment API.

## Product tiers

### Tier 1: single-host production

The first production milestone supports one Linux host, multiple organizations, multiple projects, and concurrent deployment pools. PostgreSQL is the source of truth. Redis is optional for cache and queues. Firecracker requires KVM and TAP privileges. Every deployment receives configurable vCPUs, memory, disk, replicas, health checks, and optional named persistent volumes.

### Tier 2: resilient single host

The second milestone adds process supervision, crash recovery, durable snapshots, build queues, disk-pressure protection, backup verification, certificate renewal, deployment locks, and safe admission control. The goal is to ensure that one host can operate many tenants without noisy-neighbor failures.

### Tier 3: multi-host cluster

The third milestone introduces a host-agent protocol, placement constraints, artifact replication, volume placement, host heartbeats, rescheduling, and a scheduler. PostgreSQL remains the control-plane source of truth; each host runs a constrained Firecracker runtime agent. Kubernetes is not required for the first multi-host version.

## P0 implementation sequence

### Milestone 1: immutable image pipeline

Porter must support a BuildKit Unix-socket worker without Docker. The worker accepts a Git context, Dockerfile, or OCI reference and emits an OCI layout plus metadata. A conversion worker then selects Alpine by default or Debian/Ubuntu when requested, merges the application filesystem with the base rootfs, installs the Porter guest agent, preserves entrypoint/CMD/environment metadata, creates an ext4 rootfs, copies the matching kernel, calculates digests, and rejects incomplete artifacts.

A deployment must not become `ready` until all of the following are true:

| Check | Requirement |
|---|---|
| Build | BuildKit solve completed successfully |
| Base | Managed/custom base exists and is verified |
| Guest contract | Init/guest-agent binary and startup contract are present |
| Artifact | Kernel and rootfs exist, are regular files, and pass digest/size checks |
| Runtime | Firecracker configuration can be rendered without conflicts |
| Security | Image and source policy checks pass |

### Milestone 2: guest-agent contract

The guest agent is mandatory for managed bases. It must report boot ID, readiness, application health, exposed ports, CPU/memory/disk metrics, graceful shutdown state, and log cursor. The control plane communicates over vsock or a narrowly scoped guest channel. It must not expose arbitrary host commands.

### Milestone 3: deployment engine

A deployment is immutable and owns a VM pool. The deployment engine supports create, build, verify, preview, promote, rollout, pause, rollback, drain, and delete. Promotion changes a database pointer and edge routing state; it does not rename or mutate old deployment artifacts. Deployment locks and idempotency keys prevent duplicate webhook deliveries from creating duplicate pools.

### Milestone 4: storage and networking

Ephemeral rootfs is the default. Named volumes are opt-in and attached explicitly. The volume manager enforces ownership, size, mount path, backup policy, and deletion protection. Each VM receives an isolated TAP interface, private address, egress policy, and optional public domain mapping. The gateway never routes to a VM that is not healthy and authorized for the requested project/environment.

### Milestone 5: safe production rollout

The default rollout is gradual. Porter starts the new pool, waits for readiness and health checks, shifts a configured percentage, observes the failure budget, and continues only when the error rate remains within policy. Five consecutive failed health checks trigger replacement or rollback according to the deployment policy. The previous production pool remains warm until the new deployment is stable or the retention policy removes it.

## P1 implementation sequence

Porter should then add PR preview deployments, preview-secret isolation, automatic cleanup after merge/close, deployment comments/webhooks, stable cookie-based A/B experiments, scheduled backups to S3-compatible storage, backup restore drills, deployment logs, guest logs, metrics, alert rules, email/Slack/Discord/webhook notifications, team invitations, approvals, and an audit timeline. Coolify documents the value of preview cleanup, scoped secrets, automated PR comments, scheduled backups, monitoring, and notifications; Porter should implement these with deployment and VM semantics rather than container semantics.[^3] [^4]

## Security requirements

The platform must enforce organization and project boundaries in every API, gateway lookup, volume operation, artifact operation, and log stream. Secrets must be environment-scoped and encrypted at rest. Preview deployments must never inherit production secrets by default. Git webhooks must be signed and idempotent. Build jobs must run with bounded network, CPU, memory, disk, and timeout policies. Firecracker processes must run with least privilege and validated kernel/rootfs artifacts. Certificate private keys must be protected separately from deployment metadata.

## Acceptance test strategy

| Test level | Acceptance test |
|---|---|
| Unit | Guest-base selection, OCI manifest parsing, layer whiteouts, artifact digest validation, rollout state machine, weighted routing, cookie affinity, rollback threshold |
| Contract | Every API route maps to a handler, permission, store operation, and documented request/response schema |
| Database | Apply all migrations from empty PostgreSQL, upgrade from existing data, rollback development migrations, verify tenant isolation and deployment pointers |
| Build | Build a minimal Dockerfile through BuildKit, convert it using Alpine/Debian/Ubuntu, boot each result, and verify entrypoint/env/ports |
| Runtime | Start, health-check, stop, restart, snapshot, restore, crash-recover, and replace replicas using real Firecracker and KVM |
| Routing | Preview domain, production domain, gradual rollout, cookie-stable A/B, failed health checks, automatic rollback, and old-pool draining |
| Persistence | Attach a named volume, restart the VM, restore the volume, back it up, verify checksum, and test deletion protection |
| Security | Organization isolation, preview secret isolation, signed webhook validation, RBAC, CSRF, scoped API keys, and audit events |
| Resilience | Fill disk threshold, kill a Firecracker process, interrupt a build, restart Porter, lose Redis, expire a certificate, and recover safely |
| Performance | Measure build time, artifact unpack time, Firecracker launch, readiness, restore p50/p95, route latency, and concurrent replica capacity |

## Immediate next sprint

The next sprint should not add more dashboard pages. It should finish the production data plane in this order: **guest-agent protocol, real BuildKit-to-base conversion, artifact verification, deployment state machine, health-driven rollout, volume attachment, and a real KVM acceptance harness**. Only after that should Porter advertise Dockerfile/OCI deployments as fully supported.

The success criterion is one complete curl-verifiable flow:

```text
create project
→ submit Dockerfile/OCI deployment
→ BuildKit build
→ select Alpine/Debian/Ubuntu base
→ produce and verify vmlinux/rootfs.ext4
→ start two Firecracker replicas
→ pass guest readiness and HTTP health checks
→ attach optional volume
→ expose preview domain with TLS
→ shift production traffic gradually
→ promote or automatically rollback
→ retain or delete the tagged deployment
```

## Strategic differentiation

Porter should not compete with Coolify by copying every container feature immediately. It should win on **stronger workload isolation, immutable VM-level deployments, fast snapshot restore, consistent rollback, explicit tenant boundaries, and a simple operator-controlled single-host path**. Coolify should remain the feature benchmark for developer experience and operations; Firecracker should remain the reason to choose Porter.

## References

[^1]: [Coolify homepage](https://coolify.io/) — application, database, service, Git, SSL, backup, API, collaboration, monitoring, and notification capabilities.
[^2]: [Coolify applications documentation](https://coolify.io/docs/applications) — Git/Dockerfile/Compose/image deployment, environment variables, storage, health checks, rollbacks, resource limits, and previews.
[^3]: [Coolify GitHub preview deployments](https://coolify.io/docs/applications/ci-cd/github/preview-deploy) — preview URLs, cleanup, scoped deployments, scoped secrets, and PR comments.
[^4]: [Coolify rolling updates](https://coolify.io/docs/knowledge-base/rolling-updates) — health-gated replacement of the old running workload by a new workload.
