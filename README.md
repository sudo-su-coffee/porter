<p align="center">
  <img width="80" height="80" alt="Porter logo" src="/assets/porterlogo.png" />
</p>

# Porter

> **The self-hosted PaaS for direct Firecracker microVMs.**

Porter is a Go control plane and Vue dashboard for running applications as
kernel-isolated Firecracker microVM replicas on a Linux host. It provides the
operator workflow around projects, releases, replicas, domains, logs, traffic,
images, volumes, host readiness, organizations, and database-backed RBAC.

Porter v1.0.0-beta-dev intentionally uses **official Firecracker over Unix-domain
HTTP API sockets**. It does not use containerd, firecracker-containerd, a Docker
daemon, an OCI runtime, or a Docker socket as its VM control path.

## Status and scope

The repository contains a working direct-runtime backend, a Vue 3 dashboard,
database migrations for seeded RBAC and image artifacts, checksum-pinned
Firecracker release helpers, and a GitHub Releases-only distribution contract.
The dashboard has 57 route entries spanning the non-WhatsApp PaaS workspace and
includes authenticated application, replica, and build log streams.

The current Git source-build path is deliberately honest: it clones a source
repository and accepts a verified `vmlinux` plus `rootfs.ext4` bundle. A
Dockerfile or Compose file is **not** automatically a Firecracker guest. A
future BuildKit solve still requires a separate reviewed guest-conversion step
before its result can be booted.

## Runtime architecture

```text
Vue dashboard / REST client
            │ bearer auth + X-CSRF-Token + X-Porter-Org-Id
            ▼
       Porter Go API
            │ PostgreSQL-backed projects, builds, RBAC, images, audit data
            ▼
     direct runtime manager
            │ one process and one Unix socket per replica
            ▼
       Firecracker
            │ vmlinux + rootfs.ext4 + TAP interface
            ▼
        guest microVM
```

The host network boundary uses TAP interfaces, private per-project address
ranges, deterministic guest MAC/IP allocation, and gateway traffic logging.
The runtime validates the kernel, rootfs, Firecracker binary, KVM, TAP, and
socket prerequisites before a replica is considered bootable.

## Quickstart

Porter requires a Linux host with KVM, PostgreSQL, `ip tuntap` capability, and
the permissions needed to create TAP devices and Firecracker Unix sockets.

```bash
# 1. Configure secrets for the database-backed control plane.
export PORTER_DATABASE_URL='postgres://porter:porter@localhost:5432/porter?sslmode=disable'
export PORTER_BOOTSTRAP_ADMIN_PASSWORD='replace-with-a-long-unique-password'
export PORTER_SECRET_KEY='replace-with-at-least-32-random-bytes'

# 2. Install the pinned official Firecracker release and provision the base
#    guest bundle. The installer prefers local files, then GitHub Releases.
sudo -E bash deploy/install.sh

# 3. Start Porter after installation.
sudo systemctl enable --now porter

# 4. Open the dashboard and sign in with the migration-seeded admin account.
#    The first startup consumes PORTER_BOOTSTRAP_ADMIN_PASSWORD once; it is not
#    used as an authorization bypass.
```

The canonical TOML template is `backend/porter.toml.example`. Runtime secrets
should be injected through the environment or a protected systemd environment
file, not committed to the repository.

## Firecracker and base-image artifacts

Firecracker stays a host prerequisite. `release/firecracker-versions.json`
pins the stable and fallback official GitHub releases and their SHA-256
digests. `install-firecracker.sh` downloads only the configured official
GitHub asset, verifies it before extraction, and fails closed when the digest
does not match.

Porter guest artifacts are separate from the Firecracker binary. A real base
bundle contains:

```text
vmlinux       # compatible Linux guest kernel
rootfs.ext4   # bootable ext4 guest filesystem
```

The operator guide is [`FIRECRACKER_ARTIFACTS.md`](FIRECRACKER_ARTIFACTS.md).
The release builder accepts a local verified bundle and creates separate
daemon and base-image packages for upload to a Porter GitHub Release. No AWS
bucket or arbitrary artifact mirror is part of the supported flow.

## Authentication and RBAC

Porter uses persisted users, organizations, memberships, roles, permissions,
role-permission rows, sessions, and scoped API keys. There is no hardcoded admin
username, TOML API token, or configuration-admin bypass.

Migration `0007_rbac` creates the role and permission catalog. Migration
`0012_seed_rbac_admin` creates the initial admin identity and default
organization. Migration `0015_seed_super_admin` creates the persisted
`super_admin` role, grants it every permission, promotes the seeded admin, and
links the account to its default organization as owner. The bootstrap password
is initialized once from `PORTER_BOOTSTRAP_ADMIN_PASSWORD`.

The Vue Teams & Access workspace can create custom roles, inspect the permission
catalog, assign and revoke custom-role permissions, select an organization,
manage organization members, create users, and create or revoke user-scoped
API keys. Migration-managed system roles are protected from destructive edits.

## Dashboard coverage

The Vue dashboard in `frontend/` is a Whatomate-inspired workspace adapted for
Porter, not a WhatsApp application. It includes:

| Area | Examples |
|---|---|
| Projects | Projects, deployments, build history, source provenance, environments, settings, and rollout actions |
| Runtime | Replicas, start/stop/restart/delete, health, metrics, traffic, SSH information, host readiness, and base-image readiness |
| PaaS resources | Domains, services, networks, hooks, cron jobs, alerts, drains, redirects, firewall, volumes, secrets, and project members |
| Observability | Project logs, replica logs, build logs, live SSE streams, analytics, events, traffic, and daemon logs |
| Access | Organizations, groups, users, organization members, roles, permissions, audit data, and API keys |

Read-oriented resource routes use real endpoint data and explicit loading,
error, and empty states. They do not seed fake projects, builds, reviews,
traffic, or user records.

## API and stream contracts

The API is registered in `backend/internal/api/api.go` and implemented in
`backend/internal/api/handlers_impl.go`. Authenticated state-changing requests
require the CSRF token from `GET /csrf`.

Important stream routes are:

```text
GET /projects/{projectId}/logs/stream
GET /vms/{replicaId}/logs/stream
GET /projects/{projectId}/builds/{buildId}/logs/stream
```

Build logs are persisted with an optional `build_id` by migration `0014`, so
historical project-level lines remain valid while live build streams can be
scoped precisely.

## Source builds and BuildKit boundary

The current direct-only source flow performs Git source retrieval and validates
real guest artifacts. It does not claim that an OCI image or Dockerfile is
bootable as a microVM. BuildKit integration is a future subsystem with this
required boundary:

```text
GitHub source → BuildKit solve → filesystem/image result
             → reviewed kernel/rootfs guest conversion
             → SHA-256 validation → registered golden image
             → direct Firecracker boot
```

Until guest conversion is implemented and tested on a privileged host, the
dashboard reports direct-artifact readiness rather than showing a misleading
Docker build-to-VM success state.

## Development and validation

```bash
cd backend
go test ./...
go vet ./...

cd ../frontend
npm ci
npm run build
```

The dashboard build is copied to `backend/web/dist/` and embedded by the Go
binary. The release archives and SHA-256 manifest are generated locally by the
release scripts and are not automatically uploaded or merged into GitHub.

## Repository map

| Path | Purpose |
|---|---|
| `backend/` | Go API, direct Firecracker runtime, PostgreSQL store, migrations, and embedded dashboard |
| `frontend/` | Vue 3 operator dashboard |
| `deploy/` | Linux installer, development launcher, and deployment guidance |
| `release/` | GitHub artifact manifest and release-package builder |
| `FIRECRACKER_ARTIFACTS.md` | Base-image and Firecracker artifact operations guide |
| `PLAN.md` | Current beta-dev roadmap and release acceptance criteria |

## License

Porter is released under the MIT License. See [`LICENSE`](LICENSE).
