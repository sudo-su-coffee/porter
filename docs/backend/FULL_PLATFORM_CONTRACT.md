# Porter Full Platform Contract

Porter is a single-host, multi-tenant deployment platform built on Firecracker microVMs. It provides Vercel-style versioned deployments while using a separate microVM replica pool for every deployment version.

## Confirmed decisions

| Area | Decision |
|---|---|
| Runtime isolation | Official Firecracker Unix-socket API; no firecracker-containerd and no Docker runtime dependency. |
| Image input | Docker/OCI image selected by the user, built with BuildKit, then converted into a bootable Firecracker artifact. |
| Guest bases | Porter-managed Alpine, Debian, and Ubuntu bases; Alpine is the default. Advanced users may select a validated custom base. |
| Guest contract | Every managed base includes a Porter init/guest-agent contract for networking, readiness, health, graceful shutdown, and recovery. |
| Deployment identity | User-provided tag is preferred, such as `v1.4.0`, `demo`, or `canary`; an automatic tag is generated when omitted. |
| Deployment isolation | Every deployment owns its own versioned VM replica pool. Old versions remain available until the user removes the deployment tag. |
| Tenancy | Multiple organizations share one Linux host with database-backed isolation and per-project network/resource limits. |
| Availability | Configurable replica count, health replacement, snapshot recovery, and single-host HA. Multi-host scheduling is a later extension. |
| Traffic | Preview domains, production domains, cookie-stable A/B routing, gradual percentage rollout, promotion, and rollback. |
| Rollback | Five consecutive failed health checks trigger automatic rollback to the previous stable deployment. |
| Persistence | Ephemeral by default. Named persistent volumes may be attached for databases, Redis, files, or application storage. |
| Networking and TLS | Go-managed domain routing, DNS integration, and TLS certificate lifecycle. |
| Cleanup | Deployments are retained while tagged; removing a deployment tag stops its replica pool and releases its artifacts according to retention policy. |
| Production safety | Only validated stable deployments may receive production traffic. |

## Image conversion boundary

BuildKit is the build engine and may run as a local Unix-socket service or inside an isolated build microVM. Docker is not required to run application workloads. The conversion pipeline must produce a kernel-compatible ext4 root filesystem, preserve the image entrypoint and environment metadata, install the Porter guest init/agent, and record immutable artifact digests with the deployment.

## Cold-start target

The product target is a cold start below 150 ms on the supported host profile. This is a benchmark target, not a guaranteed value; the final p50/p95 must be measured with the selected kernel, guest base, artifact size, storage device, and host KVM configuration.
