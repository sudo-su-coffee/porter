# Porter API-to-Vue Gap Audit

## Result

The refreshed audit is run against the stacked API-to-Vue coverage branch. The backend registers **311 route patterns** in `backend/internal/api/api.go`, with **296 central route-permission entries**. The remaining 15 registered routes are deliberate authentication, session, CSRF, health, version, compatibility, and event-transport exceptions rather than administrator bypasses. The Vue dashboard now contains **31 genuine view components**, **116 route declarations**, and **40 explicitly schema-driven resource routes**.

The count of Vue files is deliberately not equal to the number of product surfaces. A product surface may be a dedicated component, a real tabbed workspace, a live SSE stream, or an explicitly schema-driven `ResourceManager` route. One-line wrapper files remain prohibited.

## Newly found and corrected gaps

| Backend capability | Previous state | Current frontend coverage |
|---|---|---|
| `POST /projects`, `POST /projects/compose` | No reachable New Project form | `NewProject.vue` with direct image/Git and Compose creation, validation, and truthful artifact boundary copy |
| Project Compose GET/PUT/validate/preview | No dedicated source page | `ProjectSourceRuntime.vue` |
| Project scale, healthcheck, autoscale | Scale modal only; health/autoscale routes not surfaced | `ProjectSourceRuntime.vue` with persisted forms and exact PATCH/PUT contracts |
| Git import and deploy-from-Git | Build page only queued a generic build | `Builds.vue` now exposes build, import, and deploy-from-Git actions |
| Git settings sync, ignore-command, toggles, framework, Git LFS | Several backend settings contracts were not reachable | `ProjectSettingsPage.vue` routes/actions |
| VM SSH certificate, exec, console capability | VM page showed only SSH metadata | `VmDetail.vue` exposes certificate readiness, non-interactive exec, and console capability with unsupported status handling |
| Server heartbeat and SSH metadata | Servers page only registered/removed hosts | `Servers.vue` now exposes heartbeat and SSH metadata |
| Global volume usage and resize | Servers page only created/deleted volumes | `Servers.vue` and project volume actions expose usage and resize |
| Global feedback | Backend endpoint had no dashboard page | `Feedback.vue` |
| Organization events | Teams page did not load `/orgs/events` | Events tab in `Teams.vue` plus `/access/events` |
| Host kernel | Backend endpoint was loaded by Settings but had no route | `/host/kernel` ResourceManager route |
| Global traffic search | Traffic page only listed and cleared | `Traffic.vue` now calls `/traffic/search?q=` |
| Server registration payload | Vue sent `name`, backend accepts `hostname` | Corrected to `{ hostname, address }` |
| Organization aliases and default/current organization | Default org, legacy `/org`, and current-org deletion were not all reachable | `Teams.vue` loads `/orgs/default`, `/org`, `/orgs/current`, exposes both PATCH contracts, and provides current-org deletion |
| Group detail and project membership | Group detail was only inferred from list data | `Teams.vue` exposes GET/PATCH/DELETE group controls plus project add/remove and detail inspection |
| Deployment check replacement | Per-check PATCH existed without full-list replacement | `DeploymentDetail.vue` exposes PUT `/checks` with JSON-array validation |
| Historical logs behind streams | Stream routes did not consistently load the persisted GET log endpoint | `LiveLogStream.vue` loads historical lines before the corresponding SSE stream for build, project, replica, and VM routes |
| Compatibility VM inventory and aliases | `/vms` global inventory and legacy auth aliases were not explicit | `/vms` resource screen plus canonical-first `/auth/login` and `/auth/logout` fallback behavior |
| Detail endpoints for alerts, cron, firewall rules, volumes, and servers | Several detail handlers were reachable only by URL knowledge | Visible Details controls now call each registered detail endpoint from its resource table |
| Observability metric aliases | LCP, CLS, and FID aliases were not loaded by the project analytics page | `ProjectAnalytics.vue` loads aggregate, timeseries, beacon, LCP, CLS, and FID contracts |

## Deliberate non-page exceptions

`GET /events` is both the authenticated event-hub transport used for dashboard refreshes and a dedicated `GlobalEvents.vue` live stream page. Authentication, session, password, CSRF, health, version, and current-user self-service endpoints are explicit account or transport flows rather than missing PaaS pages. Compatibility aliases are covered by canonical-first calls or visible compatibility inventory routes. SSH and exec controls surface the backend’s explicit unsupported status when a guest-vsock agent is absent; they do not claim interactive guest SSH. BuildKit Dockerfile/Compose-to-guest conversion remains outside the current beta-dev contract.

## Validation

The fresh consolidated audit generated `docs/frontend/API_ENDPOINT_VIEW_COVERAGE.md` and found **311 of 311 registered route paths with Vue source evidence**, with **zero unmatched product routes** and zero unmatched documented transport prefixes. The result is method-aware at the route/action review level: direct GET callers, declared mutation methods, router resource actions, stream history calls, and compatibility aliases were inspected alongside the normalized path scan. The current implementation pass is green for `go test ./...`, `go vet ./...`, `npm run build`, `git diff --check`, shell syntax checks, no-wrapper scans, and WhatsApp/chat-specific scans. The source/build pages continue to state that direct Git builds accept repositories containing validated `vmlinux` and `rootfs.ext4` artifacts and do not claim arbitrary OCI images can boot as Firecracker guests. The remaining operator-host validation requirements are privileged KVM/TAP Firecracker smoke testing, live PostgreSQL migration testing, and any separately reviewed guest conversion worker.
