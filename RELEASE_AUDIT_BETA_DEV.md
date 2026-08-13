# Porter v1.0.0-beta-dev Coverage Audit

## Executive result

The Porter dashboard now has a substantially broader Whatomate-inspired operator workspace, while excluding WhatsApp/chat-specific behavior. The current Vue router contains **57 route entries**, the source tree contains **17 Vue view components**, and the dashboard has approximately **113 live API call sites**. The additional route entries intentionally reuse the existing real-data `ResourceView` for read-oriented resources and dedicated components for builds, project workspaces, access management, and authenticated log streams.

The backend and dashboard builds pass. The release is not yet an honest claim of complete GitHub-Dockerfile-to-microVM automation: the current Git build path clones a repository and requires real `vmlinux` and `rootfs.ext4` artifacts. A BuildKit solve plus OCI/filesystem-to-Firecracker guest conversion remains a separate host subsystem that must be implemented before Dockerfile or Compose input can be presented as deployable.

## Whatomate-derived dashboard coverage

| Surface | Current Porter implementation |
|---|---|
| Workspace shell | Persistent sidebar, grouped navigation, mobile shell, connection state, collapse state, and primary New Project action. |
| Projects and deployments | Project list, project workspace, replica lifecycle, deployment detail, promote/rollback, source provenance, project settings, and direct image contract. |
| Builds and sources | Dedicated project Builds view, Git URL/branch submission, branch discovery, build history, and live build-log route. Current source build accepts only a repository-provided direct artifact pair. |
| Runtime | Global replicas, VM detail, start/stop/restart/delete, health, metrics, traffic, SSH information, host overview, host prerequisites, host runtime, host ports, and image/base-image readiness. |
| Observability | Global/project/replica logs, authenticated SSE application-log streams, build-log streams, traffic, analytics, project events, daemon logs, and resource-level empty/error/loading states. |
| PaaS resources | Domains, services, networks, environments, hooks, cron jobs/history, alerts, drains, redirects, firewall, volumes, project members, build settings, Git settings, Functions settings, security, and networking resource pages. |
| Access and RBAC | Organization selector, organizations, groups, persisted org members, users, scoped API keys, roles, permissions, role-permission toggles, revocation/removal actions, and organization audit view. |

The dashboard contains no WhatsApp-specific routes, chat stores, chatbot components, contacts, campaigns, templates, or conversation navigation in the Porter frontend source. Whatomate’s useful workspace patterns were retained; its product-specific messaging domain was not.

## Database-seeded RBAC

The migration order is now:

1. `0007_rbac.up.sql` creates `roles`, `permissions`, and `role_permissions`, then seeds the standard `owner`, `admin`, `member`, and `viewer` roles plus the fine-grained permission catalog.
2. `0012_seed_rbac_admin.up.sql` creates the migration-owned `admin` user identity with an empty password hash and creates the default organization. The password is initialized once from `PORTER_BOOTSTRAP_ADMIN_PASSWORD`; no plaintext password or token is stored in SQL.
3. `0015_seed_super_admin.up.sql` creates a persisted `super_admin` role, grants it every row in `permissions`, promotes the migration-created `admin` identity to `super_admin`, and inserts the admin as `owner` in the default organization.

Authorization remains database-backed. The Go code does not special-case the username `admin`, does not accept a TOML admin password or API token, and does not bypass the permission resolver for `super_admin`. System roles are migration-managed and protected from destructive UI edits; custom roles can be created, edited, assigned permissions, and deleted through the existing RBAC endpoints.

The organization membership bug found during the audit is corrected. Member list/add/update/remove operations now use `org_members` rows, membership roles are organization-scoped, and the resolver translates authenticated usernames to their UUID before querying `org_members` or `project_members`. Organization membership changes no longer mutate a user’s global role or delete the entire account.

## Live log transport

The following authenticated SSE routes are now present and mapped to `log.read`:

| Endpoint | Data source |
|---|---|
| `GET /projects/{projectId}/logs/stream` | Project VM log rings plus project build history. |
| `GET /vms/{replicaId}/logs/stream` | The selected replica’s real VM log ring. |
| `GET /projects/{projectId}/builds/{buildId}/logs/stream` | Build-scoped persisted `build_logs` rows and build status. |

Migration `0014_build_log_stream_contract.up.sql` adds a nullable `build_id` to `build_logs`, preserving historical project-level log lines while enabling build-scoped streams. The Vue client uses authenticated fetch streaming because the browser `EventSource` API cannot attach Porter’s bearer header.

## Direct Firecracker and TAP boundary

The canonical runtime still launches the official Firecracker binary and communicates over per-VM Unix sockets. The host-side network manager creates TAP devices, assigns gateway addresses, brings interfaces up, and records the VM MAC/IP identity used by the Firecracker boot request. Readiness endpoints expose KVM, Firecracker, artifact, and host prerequisites before a deployment can be considered viable.

The dashboard does not claim that an OCI image reference is itself a Firecracker guest. A deployable artifact must resolve to a compatible kernel and ext4 rootfs, with validation and SHA-256 metadata.

## BuildKit boundary and remaining limitation

The official BuildKit Go client can submit a `dockerfile.v0` solve to a separately managed `buildkitd`, stream `SolveStatus`, and export an OCI image or filesystem result.[^1] That result is not automatically a Firecracker kernel/rootfs pair. Porter therefore still needs a reviewed guest-conversion worker before a Dockerfile or Compose repository can be reported as bootable. Until that worker exists, the API and Vue Builds page deliberately report the current behavior: Git clone plus direct-artifact validation only.

This limitation is intentional rather than hidden. The system must not label a BuildKit OCI result as a ready microVM image, and it must not reintroduce containerd or an OCI runtime into the direct Firecracker boot path.

## Validation performed

The following checks passed after the RBAC, live-stream, Vue route, and workspace changes:

```text
go test ./...
go vet ./...
npm run build             # Vue dashboard, output embedded into backend/web/dist
```

The marketing project remains separately published at `https://portermktg-epn9z6ry.manus.space`; its latest checkpoint is `manus-webdev://f13052c6`.

## References

[^1]: [Moby BuildKit official repository](https://github.com/moby/buildkit)
[^2]: [Whatomate public repository](https://github.com/shridarpatil/whatomate)
