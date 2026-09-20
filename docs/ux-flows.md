# UX flows — customer + admin journeys (screen → API → guard)

> Wireframes live in `ui/` + `docs/screens-audit.md`. This doc connects them:
> each step = screen + API call + capability + empty/error states. One primary action per screen.

## Customer journeys

### C1. Signup → team → project → deploy → live (first vertical slice)

1. Signup/login (`LoginPage/SignupPage`, `POST /auth/signup|/login`) → personal org+team seeded.
2. Create project (`POST /projects`, `project.create`) → guided review (runtime/scale/domain/resources) → Deploy.
3. Connect GitHub (`PUT …/settings/git`, webhook) → pick repo/branch → detection shows plan.
4. Build (`POST …/builds` → stream `…/builds/{id}/logs/stream`) → artifact pinned (digest + SBOM/sign/scan badges).
5. Guest prep → provision (`POST …/deployments`, `deployment.create`) → stages shown (never spinner): schedule → net → start → health → gateway → domain → TLS → Healthy.
6. Domain (`POST …/domains` → verify flow → TLS auto) → traffic live. Empty: no projects → template gallery. Errors: build fail (logs pointer), quota (limits + upgrade), health fail (checks + rollback offer).

### C2. Preview per PR

PR opened → ephemeral env (`{{pr_id}}.{{domain}}`, own vars, fork guard) → status comment → per-preview logs → close/merge → auto-destroy (TTL). Uses non-prod creds; data outlives container (warned).

### C3. Operate: scale → rollback → secrets → storage

Scale (`PATCH …/scale`, `replica.scale`, impact preview → apply) → health-gated rollout weight 0→100.
Rollback (retained digests → redeploy with current config; state via volume restore — two explicit steps).
Secrets (create → encrypted, scoped inject, rotate; never displayed). Storage (volume create → attach `/dev/vdb` → snapshot/backup schedules → restore-to-new test).

### C4. Observe + pay (deferred billing UI, meters now)

Logs/metrics/traces per replica, alerts → incidents, usage meters per team, forecast. Billing pages show plan/invoices when subscriptions ship.

## Admin journeys

### A1. Node lifecycle

Register (token → `POST /servers` → preflight checklist from `doctor` → keypair verify → caps → READY) → capacity view → cordon/drain (budget-checked) → upgrade/reboot → recover/remove. Every step: check/status/observed/expected/cause/action/evidence.

### A2. Customer-360 → support

Customers → org → team → projects → services → usage/billing + infra links (deployments/nodes/VMs/incidents). Support ticket → request impersonation (policy→consent→30-min banner→audit→expiry) → act → timeline.

### A3. Incident → remediation

Alert (threshold/anomaly/absence/rate/SLO) → incident (severity/timeline/affected/remediation/postmortem) → workflow/AI plan (intent/diff/risk/cost/perms/approval/steps/verify/rollback) → execute → verify → audit.

### A4. Suspend/resume (commerce deferred, path live)

Past-due → grace (notify/retry) → suspend (stop/restrict compute, preserve storage) → resume on payment → audit. Billing emits actions; provisioning executes.

## Cross-cutting UX rules

~3 interactions per common op; explicit operation states everywhere; global search/command palette (⌘K); timelines on every resource; dense tables + keyboard nav; UI latency vs infra latency distinguished; Automatic→Explain→Override for every automation.
