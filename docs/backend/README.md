# Porter backend
# Porter Backend

This directory contains the Go control plane, PostgreSQL store, direct
Firecracker runtime, embedded migrations, and the built Vue dashboard.

Porter v1.0.0-beta-dev uses the official Firecracker binary directly. The
runtime starts one Firecracker process per replica, creates one Unix-domain HTTP
API socket per process, and configures the VM through that socket. The canonical
runtime does not use containerd, firecracker-containerd, a Docker daemon, an
OCI runtime, a Docker socket, or CNI plugins.

## Packages

| Package | Responsibility |
|---|---|
| `internal/api` | REST routes, authentication, RBAC permission checks, resource handlers, and SSE streams |
| `internal/runtime` | Firecracker process lifecycle, Unix-socket API client, boot/stop/restart/delete, and metrics hooks |
| `internal/netmgr` | TAP interface, gateway, CIDR, guest MAC/IP, and host-network allocation |
| `internal/store` | PostgreSQL persistence, migrations, users, organizations, roles, permissions, builds, artifacts, logs, and audit state |
| `internal/imagecatalog` | Direct image manifests and verified kernel/rootfs artifact validation |
| `internal/compose` | Constrained Compose mapping into Porter direct-image services; `build:` is rejected until guest conversion exists |
| `internal/health` | Guest health checks and replacement actions |
| `internal/gateway` | Host-header HTTP routing, traffic logging, firewall enforcement, and port forwarding boundaries |
| `internal/sshgw` | Optional host-side SSH/debug contract; direct guest exec remains unavailable without a reviewed guest channel |
| `migrations` | Ordered PostgreSQL schema and seed migrations embedded into the binary |

The Go entrypoint is `cmd/porter/main.go`. The embedded frontend is built into
`web/dist` before the backend binary is compiled.

## Documentation index

| Document | Purpose |
|---|---|
| [`API_REFERENCE.md`](API_REFERENCE.md) | REST, authentication, resources, RBAC, and stream contracts |
| [`DIRECT_FIRECRACKER.md`](DIRECT_FIRECRACKER.md) | Direct Firecracker and Unix-socket runtime boundary |
| [`FIRECRACKER_ARTIFACTS.md`](FIRECRACKER_ARTIFACTS.md) | Kernel/rootfs artifacts, checksums, storage, and installation |
| [`PAGE_API_MATRIX.md`](PAGE_API_MATRIX.md) | Canonical 39-surface API/page matrix |
| [`PAGE_API_GAP_AUDIT.md`](PAGE_API_GAP_AUDIT.md) | Latest missing-page and endpoint audit |
| [`DEPLOYMENT.md`](DEPLOYMENT.md) | Development and host setup guidance |
| [`RELEASE_AUDIT_BETA_DEV.md`](RELEASE_AUDIT_BETA_DEV.md) | Beta-dev readiness and known limitations |
| [`HOST_ACCEPTANCE.md`](HOST_ACCEPTANCE.md) | Read-only host readiness, API smoke, Firecracker, and observability acceptance |
| [`BACKUP_RESTORE.md`](BACKUP_RESTORE.md) | PostgreSQL backup, restore, checksum, and retention runbook |

## Database-seeded RBAC

The database is the only authorization source. There is no hardcoded admin
username, TOML admin password, API token, or privileged configuration bypass.

| Migration | Contract |
|---|---|
| `0007_rbac` | Creates roles, permissions, and role-permission rows for the standard access model |
| `0012_seed_rbac_admin` | Creates the migration-owned `admin` identity and default organization with an empty password hash |
| `0015_seed_super_admin` | Creates the persisted `super_admin` role, grants every permission, promotes the seeded admin, and creates owner membership |

`PORTER_BOOTSTRAP_ADMIN_PASSWORD` initializes the seeded account once. It is
never used as a runtime authorization fallback. `PORTER_SECRET_KEY` protects
encrypted project secret material. Users, organizations, members, roles,
permissions, sessions, and scoped API keys are persisted in PostgreSQL.

## Direct image contract

A bootable image is a verified manifest resolving to two real host artifacts:

```text
vmlinux       # compatible Linux guest kernel
rootfs.ext4   # bootable ext4 guest filesystem
```

`imagecatalog.ValidateArtifacts` rejects missing, empty, symlinked, or digest-
mismatched files. A base image is registered at startup from the configured
state paths and exposed through `/images/base` and `/images/base/readiness`.

The large guest artifacts do not belong in Git source history. Operators store
them in the Porter state directory or upload the separate base-image package to
a Porter GitHub Release. See [`FIRECRACKER_ARTIFACTS.md`](FIRECRACKER_ARTIFACTS.md)
and [`GITHUB_ARTIFACTS.md`](GITHUB_ARTIFACTS.md).

## Firecracker distribution

`../../release/firecracker-versions.json` pins official Firecracker releases and
SHA-256 values. `../../scripts/backend/install-firecracker.sh` is local-first,
GitHub-only for remote downloads, verifies every archive before extraction, and
fails closed on missing or mismatched digests. `../../scripts/backend/build-release.sh`
creates a compiled Porter package and a separate verified base-image package.

The installer never downloads an artifact during VM boot. The runtime uses only
local paths and refuses to boot when host prerequisites or artifact validation
fail.

## API and streaming

Routes are registered in `internal/api/api.go` and implemented in
`internal/api/handlers_impl.go`. The dashboard calls the API with a bearer
session or scoped API key, the CSRF token from `GET /csrf`, and the selected
`X-Porter-Org-Id` organization context.

Important stream routes are:

```text
GET /projects/{projectId}/logs/stream
GET /vms/{replicaId}/logs/stream
GET /projects/{projectId}/builds/{buildId}/logs/stream
```

Migration `0014_build_log_stream_contract` adds a nullable `build_id` to
`build_logs`, preserving historical project-level lines while supporting exact
build-scoped live logs.

## Git source and BuildKit boundary

The current Git source path records repository provenance and accepts a real
`vmlinux`/`rootfs.ext4` bundle from the repository’s accepted artifact paths.
It does not claim that a Dockerfile, Compose `build:`, or OCI image reference
is a bootable Firecracker guest.

A future BuildKit integration must follow this boundary:

```text
GitHub source → BuildKit solve → filesystem/image result
             → reviewed guest kernel/rootfs conversion
             → digest validation → golden image → direct Firecracker boot
```

No BuildKit result may bypass kernel/rootfs validation or reintroduce
containerd/OCI booting.

## Development

```bash
cd ../../backend
go test ./...
go vet ./...
go build ./cmd/porter
cd ..

# From the repository root:
bash scripts/backend/dev.sh up
bash scripts/backend/api-smoke.sh
```

Build the dashboard before embedding it:

```bash
cd ../../frontend
npm ci
npm run build
cd ../backend
go build ./cmd/porter
```

Integration smoke tests require PostgreSQL, Linux KVM, TAP privileges, the
official Firecracker binary, and a real base bundle. Unit tests do not fabricate
guest artifacts or customer data.
