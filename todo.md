# Porter stacked API-to-Vue coverage plan

## Contract freeze

- [x] Extract every route registration from `backend/internal/api/api.go` into a method-aware inventory.
- [x] Classify each route as product page, tab, stream, resource action, authentication/account flow, or intentional transport exception.
- [x] Cross-check every product route against its RBAC permission and the frontend caller, including dynamic path parameters and request payloads.
- [x] Record the final route, view, component, and permission counts in the audit documents.

## Stacked PR slices

- [x] PR 1 — frontend contract foundation: router metadata, API client behavior, endpoint/action schema, and route reachability checks.
- [x] PR 2 — project lifecycle: project creation/edit/delete, deployments, deployment checks/rollouts, builds, Git source, Compose/runtime, environments, variables, settings.
- [x] PR 3 — runtime operations: replicas/VMs, lifecycle actions, live logs, health/metrics/traffic streams, services, networks, storage, firewall, domains, redirects, hooks, drains, alerts, and cron history.
- [x] PR 4 — platform operations: images and real Firecracker artifacts, servers, host readiness, daemon logs, global traffic/analytics, cache, feedback, and system events.
- [x] PR 5 — access and account: organizations, groups, members, users, roles, permissions, API keys, session, login/logout, signup, password recovery, profile, and account deletion.
- [x] PR 6 — full validation and documentation: contract coverage report, no-wrapper/no-chat scans, payload checks, build/test/vet, release ZIPs, and PR summaries.

## MVP quality gates

- [x] Every product endpoint has a reachable real Vue caller; no endpoint is represented only by a dead route alias.
- [x] No one-line wrapper view files, demo records, fabricated reviews, or WhatsApp/chat-specific UI remain.
- [x] All state-changing calls use the shared bearer, organization, CSRF, and error-handling path.
- [x] Direct Firecracker limitations remain explicit: validated `vmlinux` plus `rootfs.ext4`, no OCI/containerd boot claim, and no unsupported guest SSH claim.
- [x] `go test ./...`, `go vet ./...`, `npm run build`, shell syntax checks, and route/permission scans pass.
- [x] Backend changes are not merged; all work is in the approved stacked PR workflow and remains awaiting user approval.

## Remaining operator-host checks

- [ ] Run privileged KVM/TAP Firecracker smoke tests on an operator host.
- [ ] Run live PostgreSQL migration validation on an operator host.
- [ ] Review and implement any separate BuildKit Dockerfile/Compose-to-guest conversion worker before claiming that capability.
