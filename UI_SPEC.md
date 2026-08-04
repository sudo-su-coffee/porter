# UI Spec — Porter Dashboard v1.0.0

Design language: Vercel-like. Minimal chrome, dark-mode-first, monospace for anything technical (IPs, IDs, commands, URLs), generous whitespace. Two nouns only: **Projects** and **Deployments** (a Deployment is a VM).

## 1. Screens

### 1.1 Projects list (`/`)
- Grid of project cards, most-recently-active first
- Each card: project name, source badge (`compose` or `single-image`), status summary (`3/3 running` / `2/3 running, 1 failed` / `stopped`), stable-domain link shown directly on the card (click to open the live URL)
- Top-right: **New Project** button → opens the create flow (§1.3)
- Empty state: "Deploy your first image or compose file" CTA

### 1.2 Project detail (`/projects/[id]`)
- Header: project name, source, created date, **Delete Project** (destructive, confirm modal), **Redeploy**
- Service list (card-per-service):
  - Service name, image ref, state badge, stable URL (monospace, click-to-copy, click-to-open), preview URL for the current deploy shown smaller underneath
  - Per-card actions: **Stop**, **Restart**, **Delete**, **SSH** (copies the `porter ssh <name>` command), **Domains**, **Traffic**, **Logs**
  - If `state == failed`: error message shown inline in red, expandable
- Network panel (collapsed by default): project's bridge subnet + a simple reachability diagram derived from `depends_on` + `${PORTER_<SERVICE>_IP}` resolution
- Compose source (collapsed): read-only viewer of the original `compose_yaml`, with parse **warnings** shown inline

### 1.3 New Project flow (`/projects/new`)

**Tab A — Single Image**
- Fields: Name, Image ref, vCPUs (stepper, default 1), Memory (stepper in MiB, default 256), Environment variables (repeatable rows), Ports (repeatable rows)
- **Deploy** button → `POST /vms`, redirects to the new VM's detail view with live boot progress

**Tab B — docker-compose.yml**
- Drag-and-drop or paste-in YAML editor (syntax highlighted)
- Live client-side pre-validation as you type
- **Deploy** button → `POST /projects/compose`, redirects to the new project detail page
- On `400`: exact error shown inline, editor highlights the offending line where identifiable
- On `202` with warnings: redirect happens, dismissible banner lists warnings once

### 1.4 VM (single deployment) detail (`/vms/[id]`)
Same layout as a service card, full-page — used for standalone (non-compose) VMs. Includes Stop/Restart/Delete/SSH/Domains/Traffic/Logs, plus resource stats (vCPU/mem allocation, uptime).

### 1.5 Domains tab
- List of all domains for the service: **Stable** (badge: "Live"), **Preview** (badge: "This deploy"), and any **Custom** domains, each with a status pill (`pending` amber / `verified` green)
- **Add domain** button opens a form (just the domain string); on submit, shows the exact CNAME record to create, copy-to-clipboard
- Pending domains show a "waiting for DNS..." state that flips live via the `domain.status` SSE event
- **Rollback** button appears here when a service has more than one deploy: lets the operator repoint the stable subdomain back to a previous (still-running) deploy's VM in one click

### 1.6 Traffic tab
- Live-updating request table fed by `traffic.request` SSE events plus an initial fetch on open
- Columns: time, method, path, status (2xx green / 4xx amber / 5xx red), duration, remote IP
- Small requests/sec sparkline above the table, computed client-side
- Filter row: status-range chips (2xx/4xx/5xx/all), method dropdown, path substring search — all client-side against the loaded buffer

### 1.7 Logs drawer
- Slides in from the right, doesn't navigate away
- Streams via `GET /vms/{id}/logs?follow=true`, auto-scrolls unless the user has scrolled up
- Simple text search/filter box, client-side only

### 1.8 SSH modal
Not a terminal-in-browser in v1.0.0 (roadmap item) — instead:
- Shows the exact `porter ssh <name>` command with a copy button
- Shows the raw `ssh <name>@gateway... -p 2222` form too, for Option B static-key auth
- If the VM isn't `running`, explains why SSH isn't available instead of showing a dead command

## 2. Status badge colors (consistent everywhere)

| State | Color | Label |
|---|---|---|
| `pending` | gray, pulsing dot | Pending |
| `booting` | amber, pulsing dot | Booting |
| `running` | green, solid dot | Running |
| `stopping` | amber, pulsing dot | Stopping |
| `stopped` | gray, solid dot | Stopped |
| `failed` | red, solid dot | Failed |

## 3. Real-time updates

- Dashboard subscribes to `GET /events` (SSE) on mount; state badges, domain status, and traffic all update live with no manual refresh
- Project creation flow shows a step-by-step progress list while booting a compose project (`db ✓ → api ⋯ → worker ⋯`), driven by `project.progress` events
- Redeploys show a live cutover indicator on the Domains tab: "new deploy running → verifying → stable URL repointed" — makes the promotion moment visible instead of silent

## 4. Non-goals for v1.0.0 UI

- No drag-and-drop visual network topology editor
- No in-browser terminal/SSH (copy-command only, §1.8)
- No multi-user roles/permissions UI
- No billing/usage/cost estimation views
- No dark/light theme toggle — dark-mode only for v1

## 5. Component inventory

- `ProjectCard`, `ServiceCard` (shared status-badge + action-row component)
- `StatusBadge`
- `NewProjectTabs` (Single Image / Compose)
- `ComposeEditor` (YAML syntax highlight + inline error annotation)
- `DomainsPanel` (stable/preview/custom list + add-domain form + status pills + rollback control)
- `TrafficTable` (live-updating, client-side filterable, requests/sec sparkline)
- `LogsDrawer`
- `SSHModal`
- `NetworkPanel` (simple reachability diagram)
- `EventStream` (SSE hook, `usePorterEvents()`)
