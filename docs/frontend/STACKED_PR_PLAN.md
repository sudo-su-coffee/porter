# Porter Vue API coverage: stacked PR plan

This document is the execution contract for completing the Porter dashboard against `backend/internal/api/api.go`. The work is organized as a stacked series rather than one unreviewable frontend change. Each child branch is based on the previous branch, so reviewers can inspect a focused diff while the final branch contains the complete MVP dashboard.

## Stacking model

| Order | Branch | Proposed PR title | Depends on | Primary responsibility | Exit condition |
|---:|---|---|---|---|---|
| 1 | `feat/stack-vue-contract-foundation` | `feat: add method-aware Vue API coverage foundation` | `feat/direct-firecracker-beta-dev` | Shared API client guarantees, route metadata, `ResourceManager` actions, stream history loading, and route reachability conventions | Every shared component can represent a real GET, mutation, stream, or object action without a wrapper-only view |
| 2 | `feat/stack-vue-project-workflows` | `feat: complete project and deployment API workflows` | PR 1 | Projects, deployments, checks, rollout, builds, Git, Compose/runtime, environments, variables, secrets, domains, hooks, cron, alerts, drains, redirects, analytics, and settings | Every project/deployment/settings route in `api.go` has a reachable control with a truthful payload and limitation state |
| 3 | `feat/stack-vue-runtime-ops` | `feat: complete replica runtime and storage operations` | PR 2 | Replica/VM lifecycle, batch operations, historical and live logs, health, metrics, traffic, services, networks, volumes, firewall, cache, and direct host boundaries | Every runtime, stream, storage, network, firewall, and cache endpoint has a visible tab, action, or resource screen |
| 4 | `feat/stack-vue-platform-ops` | `feat: complete platform, host, artifact, and observability surfaces` | PR 3 | Real Firecracker image catalog/upload/readiness, servers, host readiness, daemon logs, global traffic, usage, analytics, feedback, system status, and event streams | Platform operators can reach every non-account endpoint and see persisted/host-backed state without fabricated records |
| 5 | `feat/stack-vue-access-account` | `feat: complete organization, RBAC, authentication, and account surfaces` | PR 4 | Organizations, legacy org aliases, groups, members, users, roles, permissions, API keys, login/logout, signup, recovery, sessions, profile, and account deletion | Every access/account registration is reachable from Teams, Login, Auth Recovery, Account, or a shared client fallback |
| 6 | `feat/stack-vue-validation-docs` | `chore: validate and document complete Vue API coverage` | PR 5 | Method-aware route audit, payload checks, no-wrapper/no-chat scans, backend tests, Vue build, shell checks, audit docs, release ZIPs, and PR summaries | The full branch is green, documented, packaged, and ready for user approval without merging |

## Task contracts

The foundation slice owns the shared behavior rather than product-specific screens. It must keep bearer authentication, CSRF, organization headers, 401 handling, multipart image upload, dynamic route parameter substitution, object actions, row actions, and historical-plus-SSE log loading in one auditable path. It must not create generic placeholder pages.

The project slice owns all project-scoped mutations and reads, including the less visible contracts such as deployment upload metadata, full checks replacement, environment bulk updates, branch/domain/range actions, settings section PUT/PATCH behavior, redirect bulk replacement, alert detail, cron detail, firewall rule detail, and Web Vitals aliases. Build and Compose views must retain the direct microVM boundary: a validated `vmlinux` plus `rootfs.ext4` artifact is required, and an OCI reference is not presented as a bootable guest.

The runtime slice owns direct operational controls. It must represent project and compatibility VM aliases, batch replica operations, service detail and scale, historical logs before streams, volume detail/usage/resize, firewall rule detail, cache purge, and the explicit unsupported state for guest SSH or exec when no guest-vsock agent is available.

The platform slice owns host and global surfaces. It must preserve real artifact readiness and checksum semantics, server detail/heartbeat/SSH metadata, host overview/prerequisite/kernel/runtime routes, global analytics and usage detail, daemon logs, traffic search/clear, feedback, system health, and EventSource streams. No dashboard state may be seeded with demo records.

The access slice owns the database-seeded RBAC model. It must expose canonical and legacy organization aliases, default/current organization state, destructive organization deletion, group detail and project membership, role detail and permission replacement, user and membership lifecycle, API keys, account self-service, authentication recovery, and logout compatibility. It must not introduce hardcoded administrator fallback behavior.

## Review and merge policy

Each PR should be opened against the immediately preceding branch and left unmerged. The final branch may be pushed to GitHub and packaged only after the complete validation pass succeeds. No branch should claim support for BuildKit Dockerfile/Compose-to-guest conversion, privileged KVM/TAP smoke tests, live PostgreSQL migration testing, or interactive guest SSH unless those capabilities are separately implemented and verified on an operator host.

## MVP definition

The Porter Vue MVP is complete when every product route registration has one of the following evidence types: a direct Vue API caller, a visible tab that calls the endpoint, a schema-driven resource screen with declared method-aware actions, a historical/live stream pair, or an intentional authentication/transport exception documented in the audit. A route declaration by itself is insufficient, and a one-line wrapper component is not evidence of coverage.
