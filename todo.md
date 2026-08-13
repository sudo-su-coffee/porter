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

- [x] Re-run the 311-route API-to-handler/RBAC/Vue coverage audit after the interrupted Linux release edits.
- [x] Decide and document the canonical editable TOML path under `/var/lib/porter`, while keeping secrets in a protected environment file.
- [x] Make the installer explicitly choose local host PostgreSQL, an operator-managed remote PostgreSQL URL, or a documented external database setup; do not use Docker in the Linux installer.
- [x] Add non-Docker PostgreSQL checks/installation guidance without pretending the sandbox can prove a real production database connection.
- [x] Verify systemd, embedded dashboard, direct Firecracker paths, real guest artifacts, and data-directory ownership against the chosen configuration layout.

## One-command install and backend readiness pass

- [x] Reconcile PR #11 and keep one canonical user-facing install command.
- [x] Make the installer support a noninteractive mode suitable for scripted Linux installation while retaining interactive PostgreSQL selection.
- [x] Add a final installer status/health check that clearly distinguishes API availability, database readiness, KVM/TAP readiness, Firecracker checksum readiness, and guest-artifact readiness.
- [x] Re-run the complete API-to-handler/RBAC/Vue audit, backend tests/vet, Vue build, embedded Go build, shell checks, and no-Docker installer scan.
- [x] Verify the one-command documentation, dashboard URL, config path, credential behavior, and operator-host limitations.

## GitHub branch cleanup review

- [x] Inspect PR #11 state, merge state, default branch, and branch protection.
- [x] Inventory local and remote branches with their last commits and merged/open PR association.
- [x] Classify branches as protected/current, open-review, merged/superseded, or safe-to-delete.
- [x] Report a deletion list and preserve the branch needed for rollback until the user confirms cleanup.

## Final candidate PR into main

- [x] Verify the final candidate commit and current `main` tip before creating the PR.
- [x] Compare the complete candidate diff against `main` and confirm the expected release files are included.
- [x] Open one final PR from the candidate branch into `main` with validation evidence and explicit operator-host limitations.
- [x] Keep the candidate branch until the user reviews and merges the PR.

## Post-merge main and PostgreSQL handoff

- [ ] Verify PR #12 is merged and the current remote `main` contains the final candidate commit/files.
- [ ] Verify local and remote branch cleanup after the user’s deletion actions.
- [ ] Re-run API coverage, backend tests/vet, Vue build, and installer-file checks from `main`.
- [ ] Explain the exact PostgreSQL choices, required operator inputs, data stored in PostgreSQL, and local `/var/porter` state.

## Merged-main database and image-pipeline review

- [ ] Verify migrations and the `porter migrate` command against a real or controlled PostgreSQL connection path.
- [ ] Verify frontend calls reach the backend contracts from merged `main`, beyond source-only route evidence.
- [ ] Identify a PostgreSQL path that does not require the user to run Docker on the target server.
- [ ] Decide whether OCI images are only build inputs and define the conversion boundary to real guest artifacts.
- [ ] Document the direct Firecracker boot sequence through per-VM Unix sockets and prohibit direct OCI boot claims.

## GitHub one-command installer 404 review

- [ ] Inspect live GitHub release tags and architecture-specific assets for the installer’s default tag.
- [ ] Confirm whether the 404 is from the raw script URL or the missing daemon/checksum release asset.
- [ ] Correct the user-facing release/install path without fabricating a package or guest artifacts.
- [ ] Validate the corrected command and document the exact missing prerequisite if a release cannot yet be published.

## Automated GitHub release build

- [ ] Inspect existing `.github/workflows` and release-builder compatibility.
- [ ] Define tag-push and manual-dispatch triggers for the Linux release workflow.
- [ ] Require a real `vmlinux`/`rootfs.ext4` bundle or a secure operator-provided artifact URL/input.
- [ ] Build the Vue dashboard and Go daemon, verify Firecracker checksums, create daemon/base-image archives and sidecars, and publish a GitHub Release.
- [ ] Validate the published asset names against `install-from-github.sh` before telling the user to rerun the one-command installer.
