# Porter Vue dashboard

This directory contains Porter’s standalone Vue 3 dashboard. It adapts the workspace form of the Whatomate frontend—persistent grouped navigation, responsive sidebar, operator tables, filters, settings panels, and reconnecting event status—without importing WhatsApp-specific stores, types, screens, or service code.

## Product surface

| Area | Route | Purpose |
|---|---|---|
| Deployments | `/` | Search, filter, start, stop, restart, and inspect project replica pools |
| Project workspace | `/projects/:id` | Deployment history, replica health, traffic, logs, domains, environment, secrets, analytics, firewall, crons, and settings |
| Replica detail | `/vms/:id` | State, health, metrics, logs, traffic, SSH information, and lifecycle actions |
| Analytics | `/analytics` | Period-based usage plus global request and timeseries signals |
| Images | `/images` | Direct Firecracker image library and kernel/rootfs artifact metadata |
| Domains and traffic | `/domains`, `/traffic` | Host routing and gateway request inspection |
| Access | `/teams` | Organizations, groups, members, users, API keys, roles, and permissions |
| Host operations | `/servers`, `/settings`, `/logs` | Host inventory, direct runtime readiness, mapped ports, and daemon logs |

## Runtime contract

The dashboard sends requests to the same origin as the Porter control API. It authenticates with the bearer token returned by `/login`, fetches `/csrf` for state-changing requests, and reconnects to `/events` with bounded exponential backoff.

Porter’s runtime is direct Firecracker. A deployable image is expected to resolve to a direct image reference and a compatible `vmlinux` plus `rootfs.ext4` artifact. The custom-image flow accepts a ZIP containing those two files. The dashboard does not assume a Docker daemon, containerd, OCI task, registry pull, or in-guest SSH daemon.

## Local development

```bash
npm install
npm run dev
```

The Vite development server expects the Porter API to be available at the same origin or through the host configuration used by the enclosing Porter server.

## Production build

```bash
npm run build
```

The current build emits the embedded dashboard into `../backend/web/dist/`, which is the directory served by Porter’s Go control plane. The standalone frontend ZIP intentionally excludes `node_modules` and generated dependencies; reinstall them from `package.json` and `package-lock.json` or the repository lockfile when developing locally.

## Design direction

The UI keeps the workspace shell quiet and makes the operational path explicit: **deploy → observe → adjust**. The deployment command deck, direct-runtime status cards, host prerequisite checks, and replica health/metrics tabs are the primary additions over the original minimal dashboard. Colors and motion remain restrained so failed boots, unhealthy replicas, and pending rollouts carry the visual hierarchy.
