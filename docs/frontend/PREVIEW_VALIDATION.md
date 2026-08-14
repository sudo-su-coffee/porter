# Development preview validation

Validated the Vue development preview at `http://127.0.0.1:5173/` with the Express mock API at `http://127.0.0.1:8787/` on 2026-08-14.

The projects route shows the `DEVELOPMENT PREVIEW` badge, grouped sidebar sections (`Operate`, `Observe`, `Manage`, and `Access`), separate Projects/Deployments/Replicas navigation, project search and status filters, host/runtime context, and synthetic project records from the mock API.

The project detail route for `proj-api` preserves the project URL and presents grouped tabs: `Operate` contains Overview, Deployments, and Builds; `Observe` contains Analytics, Traffic, and Logs; and `Configure` contains Domains, Environment, Secrets, Cron jobs, Firewall, and Settings. The page also exposes replica health and direct Firecracker runtime context. No production authentication or RBAC behavior was bypassed; the badge identifies the synthetic preview session.

Representative browser captures are stored by the sandbox at:

- `/home/ubuntu/screenshots/127_0_0_1_2026-08-14_01-47-57_8886.webp` — projects route.
- `/home/ubuntu/screenshots/127_0_0_1_2026-08-14_01-48-11_1169.webp` — project detail route.

After adding the opt-in Sentry bootstrap, the same routes were rechecked successfully. With no `VITE_SENTRY_ENABLED=true` and no DSN, the UI remains in the labeled synthetic preview mode and renders without external error-tracking initialization. The latest captures are `/home/ubuntu/screenshots/127_0_0_1_2026-08-14_02-22-52_7878.webp` for Projects and `/home/ubuntu/screenshots/127_0_0_1_2026-08-14_02-23-06_2369.webp` for Project Detail.
