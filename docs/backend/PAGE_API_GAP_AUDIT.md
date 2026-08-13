# Porter API-to-Vue Gap Audit

## Result

The fresh audit was run against the current PR branch after the previous 39-surface implementation. The backend registers **312 route patterns** in `backend/internal/api/api.go`, with **296 central route-permission entries**. The remaining 16 registered routes are deliberate authentication, session, CSRF, health, version, and event-transport exceptions rather than administrator bypasses. The Vue dashboard now contains **25 genuine view components** and **78 route declarations**.

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

## Deliberate non-page exceptions

`GET /events` is the authenticated event-hub transport used for dashboard refreshes, not a standalone table page. Authentication, session, password, CSRF, health, version, and current-user self-service endpoints are transport or account flows rather than missing PaaS pages. SSH and exec controls surface the backend’s explicit unsupported status when a guest-vsock agent is absent; they do not claim interactive guest SSH. BuildKit Dockerfile/Compose-to-guest conversion remains outside the current beta-dev contract.

## Validation

The final pass must remain green for `go test ./...`, `go vet ./...`, `npm run build`, `gofmt`, route/permission scans, no-wrapper scans, legacy `ResourceView` scans, and WhatsApp/chat-specific scans. The source/build pages continue to state that direct Git builds accept repositories containing validated `vmlinux` and `rootfs.ext4` artifacts and do not claim arbitrary OCI images can boot as Firecracker guests.
