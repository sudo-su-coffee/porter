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

## Guest bundle URL verification

- [ ] Search official Firecracker and related GitHub sources for a compatible guest kernel/rootfs pair.
- [ ] Verify that any candidate URL contains both `vmlinux` and `rootfs.ext4`, with architecture and checksums.
- [ ] Reject hypervisor-only release URLs, arbitrary mirrors, and unverified demo artifacts.
- [ ] Give the user a precise workflow input URL only if it is safe and compatible; otherwise explain how to create or host the bundle.

## Separate release artifact inputs

- [x] Keep the official Firecracker VMM download/checksum separate from guest `vmlinux` and `rootfs.ext4` inputs.
- [x] Allow manual GitHub Release upload of `vmlinux` and `rootfs.ext4` as separate assets or a verified bundle.
- [x] Add separate kernel/rootfs asset and folder inputs to the release workflow, with architecture selection and automatic checksum calculation.
- [x] Document the exact GitHub web steps for uploading guest files and starting the workflow.
- [ ] Validate that the resulting Porter release archive and installer use the uploaded guest files without OCI/containerd runtime dependencies.

## Live guest-artifact upload check

- [ ] Inspect `main` for the uploaded `vmlinux` and `rootfs.ext4` paths, names, sizes, and latest commit.
- [ ] Confirm whether the current workflow’s architecture folder or fallback Release-asset path will consume them.

## Release workflow checksum fix

- [x] Verify the release builder’s relative checksum filenames against the workflow working directory.
- [x] Fix checksum verification for both daemon and base-image archives.
- [x] Re-run local archive/checksum validation and push the focused workflow fix for an Actions rerun.

## Cached release installer and PostgreSQL prompt

- [x] Inspect installer download paths and PostgreSQL mode input handling.
- [x] Reuse a locally cached release archive when its checksum matches.
- [x] Redownload incomplete or corrupt cached files safely.
- [x] Reuse verified Firecracker and guest artifacts without repeated downloads where supported.
- [x] Read interactive PostgreSQL selection from `/dev/tty` when the installer is piped through curl/sudo.
- [x] Validate cache-hit, cache-miss, corrupt-cache, and installer syntax paths.
- [x] Ask for local or remote PostgreSQL mode at the GitHub bootstrap entrypoint and forward the choice into the extracted installer.
- [x] Ask for a remote PostgreSQL URL interactively and document the correct `sudo VAR=value bash` syntax for WSL Bash.
- [x] Validate the interactive PostgreSQL prompt through a pseudo-terminal.

## Automatic local PostgreSQL installation follow-up

- [x] Install the current stable upstream PostgreSQL server/client automatically from the official PGDG repository when local mode is selected.
- [x] Keep local PostgreSQL bound to the host loopback and apply only lightweight Porter-compatible settings.
- [x] Create and use one dedicated `porter` application role and `porter` database; do not grant the app superuser privileges.
- [x] Make local setup idempotent and safe to rerun without replacing the Porter password unexpectedly.
- [x] Validate the automatic local install path with shell tests and update the release PR.

## Single-installer consolidation and backend audit

- [ ] Confirm the canonical public installer entrypoint and remove or demote duplicate user-facing install paths.
- [ ] Inspect open PRs and verify the candidate branch contains all installer, PostgreSQL, cache, and documentation fixes.
- [ ] Audit backend release packaging, embedded dashboard, migrations, direct Firecracker runtime, and systemd installer boundaries for related blockers.
- [ ] Run backend tests/vet, frontend build, shell tests, package/archive checks, and route/RBAC validation.
- [ ] Merge the validated installer PR into `main` only after all checks pass, then verify the merged main branch and release workflow inputs.
- [ ] Run every installer and validation script syntax check from the consolidated branch.
- [ ] Check for a usable local PostgreSQL service and run a controlled backend database smoke path when available.
- [ ] Start the Go backend and Vue preview in the sandbox and exercise reachable health, login, and dashboard requests.
- [ ] Record sandbox limitations for KVM, TAP, systemd, and real Firecracker microVM boot.

## Two-script installer surface

- [ ] Define one production installer entrypoint and keep `dev.sh` as the only development entrypoint.
- [ ] Route source-checkout and GitHub-release production installation through the production entrypoint without duplicate user-facing commands.
- [ ] Keep helper implementation files internal to the production flow and exclude them from the public script list/documentation.
- [ ] Update release packaging, README, deployment docs, and tests for the two-script model.
- [ ] Validate the two-script flow and publish the consolidated changes without automatically merging unrelated work.
- [ ] Remove the obsolete install-from-github.sh user path and ensure main documents only install.sh and dev.sh.
- [ ] Ensure the release package embeds the corrected stdin-based PostgreSQL role SQL and install.sh.
- [ ] Add release checksum refresh behavior so an updated package under the same tag replaces stale cached archives.
- [ ] Verify the final retry command does not use the old raw URL or stale cached archive.
- [ ] Confirm the corrected PostgreSQL helper and API fixes are present in the working tree before commit.
- [ ] Run the real PostgreSQL migration, backend login/session/API-key, Vue, Go, shell, and release archive tests.
- [ ] Commit the consolidated changes and open a review-only PR against main.
- [ ] Provide the user a fresh `install.sh` command and explain that the old cached release must be replaced by a rebuilt release asset.
- [ ] Commit the current consolidated installer and backend fixes to ci/release-workflow.
- [ ] Open a review-only PR against main and run the committed branch tests.
