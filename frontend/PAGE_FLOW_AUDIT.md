# Porter Vue page-flow audit — v1.0.0-beta-dev

This is the current inventory of the Porter Vue dashboard after the full Whatomate-inspired non-WhatsApp workspace pass. The dashboard now exposes **57 route entries, 17 Vue view components, and 113 live API call sites**. Resource routes use real backend endpoints and explicit loading/error/empty states; they do not seed demo records.

## Existing top-level routes and nested workspaces

| Route | Current page | Current coverage |
|---|---|---|
| `/` | Deployments | Project list, search/filter, list/grid layout, start/stop/restart/delete, host overview, direct-runtime command deck |
| `/projects/:id` | Project workspace | Overview, deployment history, promote/rollback, replica pool actions, traffic, logs, domains, environment, secrets, analytics, crons, firewall, settings |
| `/vms/:id` | Replica detail | State, start/stop/restart/delete, health, metrics, logs, traffic, SSH info, copyable SSH command |
| `/images` | Image library | Direct image search/filter, kernel/rootfs metadata, image reference copy, deploy handoff |
| `/analytics` | Platform analytics | Period usage, global requests, global timeseries, request/bandwidth charts, project breakdown |
| `/traffic` | Gateway traffic | Request table, success/error filters, clear action |
| `/domains` | Domain inventory | Domain listing and project-domain verification/removal flows |
| `/teams` and `/access/*` | Access and RBAC workspace | Organization context selection, persisted organization members, groups, users, scoped API keys, roles, permissions, and organization audit data |
| `/settings` | Host operations | Host overview, kernel, direct Firecracker prerequisites, runtime configuration, mapped ports, daemon logs |
| `/servers` | Host/server inventory | Server registration, heartbeat, SSH metadata, removal, persistent volumes |
| `/logs` | Daemon logs | Global daemon log viewer and refresh |
| `/login` | Authentication | Database-backed login gate with CSRF bootstrap and persisted organization context handled by the API client |

The nested project workspace now also exposes dedicated routes for deployments, builds, services, networks, environments, hooks, cron jobs and history, alerts, drains, redirects, firewall rules, members, volumes, analytics, metrics, events, replica-pool status, application-log streaming, build settings, Git settings, Functions settings, security, and networking. Replica routes expose live logs, health, metrics, traffic, and SSH information. Host routes expose overview, prerequisites, runtime configuration, ports, and daemon logs.

## Product-flow coverage

The current dashboard has the core self-hosted PaaS loop: **create project → select a verified direct image or Git source → preflight the host → boot a replica pool → inspect health → operate lifecycle → inspect live application/build logs → inspect traffic → manage domains and access**. Project and replica log streams use authenticated SSE fetch streams backed by persisted build logs and real VM log rings. The API client sends the selected `X-Porter-Org-Id` context so organization membership and project RBAC can resolve against PostgreSQL rows.

The direct Firecracker limitation is intentionally visible. The dashboard can expose SSH information and copy a connection command, but a real interactive SSH or exec session still requires the planned guest-vsock agent. The frontend must not imply that a container task shell exists.

## High-value gaps to audit next

| Surface | Why it matters | Current status / next boundary |
|---|---|---|
| Project onboarding | Vercel/Coolify-style first deploy needs a guided path from source or image to a healthy release | Existing modal and project workspace are wired; the remaining work is preflight gating before the first privileged boot |
| BuildKit source builds | Dockerfile/Compose source builds require BuildKit plus a separate OCI/filesystem-to-guest conversion step | The current Git flow truthfully accepts only repositories containing `vmlinux` and `rootfs.ext4`; BuildKit-to-guest conversion is not yet a ready runtime path |
| Replica pool | Operators need pool status, batch actions, drain state, and liveness in one place | Dedicated pool/resource route exists; batch controls remain a deeper UX refinement |
| Runtime preflight | Direct Firecracker needs KVM, binary, socket directory, kernel, rootfs, and TAP readiness before deploy | Host readiness is exposed at `/host/prerequisites` and `/images/base/readiness`; deployment gating should be the next safety pass |
| Serverless-style workloads | Functions, cron, usage, and route analytics need distinct execution contracts | Functions settings and cron views are exposed only as live backend resources; no fictitious invocation behavior is shown |
| Project settings | The backend exposes extensive build, rollout, git, OIDC, protection, cache, volume, and notification settings | Grouped resource routes now exist; dedicated editable forms should be added only where handler semantics are fully real |

This audit is a working artifact. It is not a claim that every backend route should become a top-level page; many routes are nested resource actions and should remain contextual to a project or replica workspace.
