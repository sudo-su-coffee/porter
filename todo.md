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

## Fresh v1.0.0-beta readiness audit

- [x] Reconcile the final stacked branch and all open PR heads with the local checkout.
- [x] Generate a method-aware report for all 311 `api.go` registrations and match each to a literal Vue caller, route schema action, tab, stream, or documented exception.
- [x] Verify dynamic parameter substitution and request payloads for every state-changing endpoint, including legacy aliases and compatibility routes.
- [x] Verify every declared frontend route is reachable from the dashboard shell, project workspace, account/access flow, or an intentional detail/action control.
- [x] Re-run backend tests/vet, Vue build, wrapper/chat scans, shell checks, route/RBAC scans, and release ZIP integrity checks.
- [x] Update the MVP readiness conclusion based on the fresh report; no product endpoint remains unmapped by the source audit.

## One-PR consolidation pass

- [x] Repair the method-aware audit script and generate its endpoint-to-source report.
- [x] Review every unmatched product route and fix any missing Vue caller, method, payload, or navigation path.
- [x] Create one clean consolidated branch from `feat/direct-firecracker-beta-dev` containing the complete validated stack.
- [x] Open one final PR against `feat/direct-firecracker-beta-dev` with a thorough merge checklist and no automatic merge.
- [x] Rerun all backend, frontend, route, RBAC, source-structure, shell, and ZIP integrity checks on the consolidated branch.

## Linux v1.0.0-beta release packaging

- [ ] Reconcile PR #11 and create the Linux release branch from the approved backend baseline.
- [ ] Verify the Vue dashboard is embedded into the Go binary through the backend web asset pipeline.
- [ ] Add or verify a systemd service and installer that compile/install the Go daemon with least-privilege filesystem ownership.
- [ ] Add or verify official GitHub-only Firecracker installation with version pinning and SHA-256 verification.
- [ ] Verify real `vmlinux` and `rootfs.ext4` artifact configuration without OCI/containerd boot claims.
- [ ] Verify database-seeded super-admin bootstrap without committing or printing a reusable default password.
- [ ] Run Linux installer dry-runs, Go tests/vet, embedded-asset checks, and release archive integrity checks.
- [ ] Create one GitHub release PR and document the dashboard URL, first-run credential setup, and remaining operator-host checks.

## API, configuration, and PostgreSQL deployment review

- [ ] Re-run the 311-route API-to-handler/RBAC/Vue coverage audit after the interrupted Linux release edits.
- [ ] Decide and document the canonical editable TOML path under `/var/lib/porter`, while keeping secrets in a protected environment file.
- [ ] Make the installer explicitly choose local host PostgreSQL, an operator-managed remote PostgreSQL URL, or a documented external database setup; do not use Docker in the Linux installer.
- [ ] Add non-Docker PostgreSQL checks/installation guidance without pretending the sandbox can prove a real production database connection.
- [ ] Verify systemd, embedded dashboard, direct Firecracker paths, real guest artifacts, and data-directory ownership against the chosen configuration layout.
