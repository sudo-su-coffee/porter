# Porter stacked API-to-Vue coverage plan

## Contract freeze

- [ ] Extract every route registration from `backend/internal/api/api.go` into a method-aware inventory.
- [ ] Classify each route as product page, tab, stream, resource action, authentication/account flow, or intentional transport exception.
- [ ] Cross-check every product route against its RBAC permission and the frontend caller, including dynamic path parameters and request payloads.
- [ ] Record the final route, view, component, and permission counts in the audit documents.

## Stacked PR slices

- [ ] PR 1 — frontend contract foundation: router metadata, API client behavior, endpoint/action schema, and route reachability checks.
- [ ] PR 2 — project lifecycle: project creation/edit/delete, deployments, deployment checks/rollouts, builds, Git source, Compose/runtime, environments, variables, and settings.
- [ ] PR 3 — runtime operations: replicas/VMs, lifecycle actions, live logs, health/metrics/traffic streams, services, networks, volumes, firewall, domains, redirects, hooks, drains, alerts, and cron history.
- [ ] PR 4 — platform operations: images and real Firecracker artifacts, servers, host readiness, daemon logs, global traffic/analytics, cache, feedback, and system events.
- [ ] PR 5 — access and account: organizations, groups, members, users, roles, permissions, API keys, session, login/logout, signup, password recovery, profile, and account deletion.
- [ ] PR 6 — full validation and documentation: contract coverage report, no-wrapper/no-chat scans, payload checks, build/test/vet, release ZIPs, and PR summaries.

## MVP quality gates

- [ ] Every product endpoint has a reachable real Vue caller; no endpoint is represented only by a dead route alias.
- [ ] No one-line wrapper view files, demo records, fabricated reviews, or WhatsApp/chat-specific UI remain.
- [ ] All state-changing calls use the shared bearer, organization, CSRF, and error-handling path.
- [ ] Direct Firecracker limitations remain explicit: validated `vmlinux` plus `rootfs.ext4`, no OCI/containerd boot claim, and no unsupported guest SSH claim.
- [ ] `go test ./...`, `go vet ./...`, `npm run build`, shell syntax checks, and route/permission scans pass.
- [ ] Backend changes are not merged or pushed beyond the approved PR workflow without explicit user approval.
