# Porter

> **The self-hosted PaaS for direct Firecracker microVMs.**

Porter is a Go control plane and Vue 3 operator dashboard for running applications as kernel-isolated Firecracker microVM replicas on a Linux host. The repository is organized around two workstreams: **Backend** and **Frontend**.

Porter v1.0.0-beta-dev uses the official Firecracker HTTP API over one Unix-domain socket per replica. It does **not** use containerd, firecracker-containerd, a Docker daemon, an OCI runtime, or a Docker socket as its VM control path.

## Workstreams

| Workstream | Location | Responsibility | Primary guide |
|---|---|---|---|
| Backend | `backend/` | Go API, PostgreSQL store, migrations, direct Firecracker runtime, TAP networking, embedded dashboard output | [`docs/backend/README.md`](docs/backend/README.md) |
| Frontend | `frontend/` | Vue 3 dashboard, router, real API forms, resource workflows, SSE views, responsive workspace | [`docs/frontend/README.md`](docs/frontend/README.md) |

The repository also keeps shared automation under `scripts/backend/` and `scripts/frontend/`. The root `Makefile` is the short entrypoint for common build and validation operations.

## Current beta-dev scope

The dashboard exposes **39 canonical PaaS product surfaces**, implemented through **31 genuine Vue view components**, **116 route declarations**, and **40 schema-driven resource routes**. The count is intentionally not 39 files: detail pages, live streams, settings sections, and schema-driven resources use real dedicated or shared implementations rather than one-line wrappers.

The dashboard covers projects, deployments, builds, direct source and Compose boundaries, replicas, health, metrics, logs, traffic, images, domains, volumes, networks, hooks, cron, alerts, firewall, settings, analytics, servers, host readiness, organizations, teams, users, roles, permissions, audit/events, and API keys. The API-to-Vue audit is recorded in [`docs/backend/PAGE_API_MATRIX.md`](docs/backend/PAGE_API_MATRIX.md), with the latest gap closure in [`docs/backend/PAGE_API_GAP_AUDIT.md`](docs/backend/PAGE_API_GAP_AUDIT.md).

## Runtime truth

The supported boot artifact is a real direct microVM manifest containing:

```text
vmlinux       # compatible Linux guest kernel
rootfs.ext4   # bootable ext4 guest filesystem
```

Docker or OCI image references are not bootable Firecracker guests. The current Git flow clones a repository and accepts it only when it contains validated `vmlinux` and `rootfs.ext4` artifacts. Dockerfile/Compose-to-guest BuildKit conversion remains a separate reviewed worker and is not represented as complete in the dashboard.

Firecracker binaries are pinned to official GitHub releases and verified by SHA-256. Guest artifacts are separate from the Firecracker binary and are never fetched from AWS or an arbitrary mirror. Read the operational artifact policy in [`docs/backend/FIRECRACKER_ARTIFACTS.md`](docs/backend/FIRECRACKER_ARTIFACTS.md) and the distribution policy in [`docs/backend/GITHUB_ARTIFACTS.md`](docs/backend/GITHUB_ARTIFACTS.md).

## Quickstart

Porter requires a Linux host with Go 1.25+, PostgreSQL, KVM, `iproute2` with TAP capability, and permission to create Firecracker Unix sockets. For a host-independent dashboard build, KVM and TAP are not required, but direct microVM boot will remain unavailable.

```bash
# 1. Install frontend dependencies and build the embedded dashboard.
make frontend

# 2. Run backend checks.
make test

# 3. Run backend development with Docker-backed PostgreSQL only for local development.
bash scripts/backend/dev.sh up

# 4. Install a Linux host for direct runtime operation; the installer asks
#    whether PostgreSQL should be local on this host or operator-managed remote.
sudo -E bash scripts/backend/install.sh
```

After a GitHub Release is published, users can install without cloning:

```bash
curl -fsSL https://raw.githubusercontent.com/sudo-su-coffee/porter/main/scripts/backend/install-from-github.sh | sudo bash
```

The Linux installer does not use Docker. It offers local host PostgreSQL (with
optional Debian/Ubuntu package installation using `PORTER_INSTALL_SYSTEM_DEPS=1`)
or a verified remote `PORTER_DATABASE_URL`. The editable non-secret runtime
configuration is `/var/porter/porter.toml`; database credentials and key material
are kept in `/var/porter/porter.env`. Authorization is database-seeded RBAC only;
the bootstrap password initializes the persisted account and is not an
authorization bypass.

## Validation

```bash
make validate       # frontend production build and shell syntax checks
make test           # go test ./...
cd backend && go vet ./...
```

The Vue build is emitted to `backend/web/dist/` and embedded by the Go binary. The release package requires a real base image and is built with:

```bash
PORTER_BASE_IMAGE_DIR=/path/to/base-image \
  bash scripts/backend/build-release.sh v1.0.0-beta-dev x86_64
```

The release helper refuses to package a release without non-empty `vmlinux` and `rootfs.ext4` artifacts.

## Documentation map

### Backend

The [Backend documentation index](docs/backend/README.md) links the API reference, runtime boundary, artifact operations, BuildKit limitation, deployment guide, release audit, route/page matrices, and release distribution policy.

### Frontend

The [Frontend documentation index](docs/frontend/README.md) links the dashboard contract, page-flow audit, view architecture, route philosophy, build output, and no-wrapper policy.

### Project decisions

The [beta-dev plan](docs/backend/PLAN.md) records the migration intent and acceptance criteria. Historical audit documents are retained under the relevant workstream instead of being duplicated at the repository root.

## License

Porter is released under the MIT License. See [`LICENSE`](LICENSE).
