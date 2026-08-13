# Porter v1.0.0-beta-dev plan

This document is the current release plan. It is intentionally synchronized
with the checked-in direct-Firecracker implementation rather than the retired
containerd/shim prototype. Status labels describe the repository as it exists:
**DONE**, **PARTIAL**, or **PLANNED**.

## Product definition

Porter is a self-hosted PaaS control plane for running application replicas as
direct Firecracker microVMs. A project resolves to a verified kernel/rootfs
artifact, a replica receives a TAP interface and private address, and the
runtime configures one Firecracker process through one Unix-domain API socket.

Containers, Dockerfiles, Compose files, and OCI images may be source or packaging
inputs in future workflows, but they are never the isolation or boot boundary.

## Current status

| Capability | Status | Evidence / boundary |
|---|---|---|
| Direct Firecracker lifecycle | **DONE** | `backend/internal/runtime`; one process and Unix socket per VM |
| TAP networking | **DONE** | `backend/internal/netmgr`; host-side TAP, gateway, MAC/IP allocation |
| Base kernel/rootfs lifecycle | **DONE** | validated `vmlinux` and `rootfs.ext4`, golden-image metadata, readiness API |
| Official Firecracker distribution | **DONE** | pinned GitHub releases, SHA-256 verification, local-first installer |
| GitHub-only Porter artifacts | **DONE** | daemon and guest bundle release contract; no AWS dependency |
| PostgreSQL persistence | **DONE** | embedded ordered migrations and store-backed resource state |
| Seeded RBAC catalog | **DONE** | roles, permissions, role permissions, memberships, API-key scope |
| Seeded `super_admin` | **DONE** | migration `0015_seed_super_admin`; all catalog permissions and owner membership |
| Project and replica control | **DONE** | create, scale, start, stop, restart, delete, health, rollout, rollback |
| Domains, traffic, analytics, and observability | **DONE** | real handlers, persisted metadata, rings, metrics, and SSE streams |
| Vue operator workspace | **DONE** | 31 real view components, 116 route declarations, projects, PaaS resources, host views, logs, and access management |
| Authenticated build-log streaming | **DONE** | project, replica, and build-scoped SSE routes |
| Persistent volume guest attachment | **PARTIAL** | host directory and sparse image lifecycle exist; boot attachment needs final host smoke test |
| Interactive guest SSH/exec | **PARTIAL** | SSH information and contracts exist; direct guest-vsock agent is not yet enabled |
| Dockerfile/Compose source build to guest | **PLANNED** | requires BuildKit solve plus reviewed kernel/rootfs conversion worker |
| Multi-node scheduling and cluster balancing | **PLANNED** | current release is host-local |

## Release architecture

```text
GitHub Release assets
  ├── porter daemon package
  ├── official Firecracker archive (pinned digest)
  └── Porter base bundle (vmlinux + rootfs.ext4, pinned digest)
                │
                ▼
Linux host installer → /var/porter + /run/porter/firecracker
                │
                ▼
Porter Go API → PostgreSQL + direct runtime → Firecracker Unix sockets
```

The installer must prefer operator-provided local assets, then use only the
configured GitHub Release URLs. Every archive is verified before extraction.
The runtime must never download a kernel, rootfs, Firecracker binary, or image
while a VM is booting.

## RBAC and organization acceptance criteria

The seeded database must contain a default organization, a migration-created
admin identity, the standard roles, the permission catalog, role-permission
rows, and an owner membership. The `super_admin` role must be a persisted role
with every permission row rather than a Go conditional.

The dashboard must permit an authorized user to select an organization, list
and manage its members, create users, create custom roles, inspect permissions,
assign and revoke custom-role permissions, and create/revoke scoped API keys.
System roles are migration-managed and cannot be deleted or stripped through
the dashboard.

## Source-build roadmap

### Completed direct-artifact path

1. Retrieve the Git source and record URL/branch provenance.
2. Detect a real `vmlinux` and `rootfs.ext4` bundle at the accepted locations.
3. Validate regular files, reject symlinks and incomplete bundles, calculate
   SHA-256 digests, and persist a golden-image manifest.
4. Register the image and boot through the direct Firecracker runtime.
5. Stream build and replica logs through authenticated routes.

### Planned BuildKit path

1. Detect Dockerfile or Compose build input and persist the source contract.
2. Submit a `dockerfile.v0` solve to an operator-managed BuildKit service.
3. Stream solve status and build logs into the build record.
4. Convert the result into a reviewed Linux guest kernel/rootfs artifact.
5. Validate and digest the resulting `vmlinux` and `rootfs.ext4`.
6. Register the artifact as a golden image and deploy through direct
   Firecracker only.

No step may label a raw OCI result as a ready microVM, invoke containerd, or
boot through an OCI runtime.

## Validation gates

Every beta-dev PR must pass:

```text
go test ./...
go vet ./...
npm run build
shell syntax checks for installers and release scripts
JSON manifest parsing and checksum guard checks
route-to-handler and route-to-permission coverage tests
ZIP integrity and release manifest verification
```

An operator-host smoke test remains required before production use. It must
exercise PostgreSQL migrations, KVM availability, TAP creation, a pinned
Firecracker binary, a real guest bundle, Unix-socket boot, health reporting,
and log streaming.

## Explicit non-goals for this release

Porter v1.0.0-beta-dev does not claim multi-tenant SaaS isolation, multi-node
scheduling, a public artifact registry, AWS object storage, a Docker daemon,
containerd, an OCI runtime, automatic Dockerfile-to-microVM conversion, or an
interactive guest shell without a guest agent. These are boundaries, not hidden
fallbacks.

## Working documents

* [`README.md`](../../README.md) is the user-facing implementation guide.
* [`FIRECRACKER_ARTIFACTS.md`](FIRECRACKER_ARTIFACTS.md) documents artifact
  storage and offline installation.
* [`API_COVERAGE_AUDIT.md`](API_COVERAGE_AUDIT.md) records the
  backend route audit.
* [`../frontend/PAGE_FLOW_AUDIT.md`](../frontend/PAGE_FLOW_AUDIT.md) records the Vue
  workspace coverage.
* [`RELEASE_AUDIT_BETA_DEV.md`](RELEASE_AUDIT_BETA_DEV.md) records the current
  end-to-end audit and remaining BuildKit boundary.
