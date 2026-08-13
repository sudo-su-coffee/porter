# API endpoint to Vue source coverage report

Generated from backend/internal/api/api.go and frontend/src on 2026-08-13T11:40:17.091Z.

| Measure | Count |
|---|---:|
| Registered api.go routes | 311 |
| Routes with normalized Vue source evidence | 311 |
| Routes without literal source evidence | 0 |
| Unmatched documented transport/auth prefixes | 0 |
| Unmatched product routes requiring review | 0 |

> This is a deterministic path-evidence report, not a substitute for the method/payload review. Resource schemas and shared components are included because their endpoint strings are declared in the router or component source.

## Product routes requiring review

| Method | Backend path | Normalized path | Source evidence |
|---|---|---|---|
| — | — | — | No unmatched product route paths detected by the normalized source scan. |

## Covered route evidence

| Method | Backend path | Vue source files |
|---|---|---|
| GET | /csrf | frontend/src/api/client.js |
| GET | /health | frontend/src/views/ProjectSourceRuntime.vue, frontend/src/views/SystemStatus.vue |
| GET | /healthz | frontend/src/views/SystemStatus.vue |
| GET | /version | frontend/src/views/SystemStatus.vue |
| POST | /feedback | frontend/src/App.vue, frontend/src/router.js, frontend/src/views/Feedback.vue |
| GET | /feedback | frontend/src/App.vue, frontend/src/router.js, frontend/src/views/Feedback.vue |
| POST | /auth/login | frontend/src/api/client.js |
| POST | /login | frontend/src/api/client.js, frontend/src/router.js |
| POST | /auth/logout | frontend/src/views/Account.vue |
| POST | /logout | frontend/src/views/Account.vue |
| POST | /auth/signup | frontend/src/views/AuthRecovery.vue |
| POST | /auth/password/forgot | frontend/src/views/AuthRecovery.vue |
| POST | /auth/password/reset | frontend/src/views/AuthRecovery.vue |
| GET | /auth/session | frontend/src/views/Account.vue |
| GET | /users/me | frontend/src/views/Account.vue |
| PATCH | /users/me | frontend/src/views/Account.vue |
| DELETE | /users/me | frontend/src/views/Account.vue |
| GET | /users/me/api-keys | frontend/src/views/Account.vue, frontend/src/views/Teams.vue |
| POST | /users/me/api-keys | frontend/src/views/Account.vue, frontend/src/views/Teams.vue |
| DELETE | /users/me/api-keys/{keyId} | frontend/src/views/Account.vue, frontend/src/views/Teams.vue |
| GET | /orgs | frontend/src/views/Teams.vue |
| GET | /orgs/default | frontend/src/views/Teams.vue |
| POST | /orgs | frontend/src/views/Teams.vue |
| GET | /org | frontend/src/views/Teams.vue |
| PATCH | /org | frontend/src/views/Teams.vue |
| GET | /orgs/current | frontend/src/views/Teams.vue |
| PATCH | /orgs/current | frontend/src/views/Teams.vue |
| DELETE | /orgs/current | frontend/src/views/Teams.vue |
| GET | /orgs/members | frontend/src/views/Teams.vue |
| POST | /orgs/members | frontend/src/views/Teams.vue |
| PATCH | /orgs/members/{username} | frontend/src/views/Teams.vue |
| DELETE | /orgs/members/{username} | frontend/src/views/Teams.vue |
| GET | /orgs/audit | frontend/src/views/Teams.vue |
| POST | /orgs/transfer | frontend/src/views/Teams.vue |
| GET | /groups | frontend/src/views/Teams.vue |
| POST | /groups | frontend/src/views/Teams.vue |
| GET | /groups/{groupId} | frontend/src/views/Teams.vue |
| PATCH | /groups/{groupId} | frontend/src/views/Teams.vue |
| DELETE | /groups/{groupId} | frontend/src/views/Teams.vue |
| GET | /groups/{groupId}/projects | frontend/src/views/Teams.vue |
| POST | /groups/{groupId}/projects/{projectId} | frontend/src/views/Teams.vue |
| DELETE | /groups/{groupId}/projects/{projectId} | frontend/src/views/Teams.vue |
| GET | /projects | frontend/src/App.vue, frontend/src/components/NewProjectModal.vue, frontend/src/views/DeploymentsList.vue, frontend/src/views/Domains.vue, frontend/src/views/NewProject.vue, frontend/src/views/Teams.vue |
| POST | /projects | frontend/src/App.vue, frontend/src/components/NewProjectModal.vue, frontend/src/views/DeploymentsList.vue, frontend/src/views/Domains.vue, frontend/src/views/NewProject.vue, frontend/src/views/Teams.vue |
| POST | /projects/compose | frontend/src/components/NewProjectModal.vue, frontend/src/views/NewProject.vue |
| GET | /projects/{projectId} | frontend/src/router.js, frontend/src/views/DeploymentsList.vue, frontend/src/views/ProjectDetail.vue |
| PATCH | /projects/{projectId} | frontend/src/router.js, frontend/src/views/DeploymentsList.vue, frontend/src/views/ProjectDetail.vue |
| DELETE | /projects/{projectId} | frontend/src/router.js, frontend/src/views/DeploymentsList.vue, frontend/src/views/ProjectDetail.vue |
| POST | /projects/{projectId}/redeploy | frontend/src/views/ProjectDetail.vue |
| GET | /projects/{projectId}/scale | frontend/src/components/ScaleModal.vue, frontend/src/components/ServiceCard.vue, frontend/src/views/ProjectSourceRuntime.vue |
| PATCH | /projects/{projectId}/scale | frontend/src/components/ScaleModal.vue, frontend/src/components/ServiceCard.vue, frontend/src/views/ProjectSourceRuntime.vue |
| GET | /projects/{projectId}/healthcheck | frontend/src/views/ProjectSourceRuntime.vue |
| PUT | /projects/{projectId}/healthcheck | frontend/src/views/ProjectSourceRuntime.vue |
| GET | /projects/{projectId}/autoscale | frontend/src/views/ProjectSourceRuntime.vue |
| PUT | /projects/{projectId}/autoscale | frontend/src/views/ProjectSourceRuntime.vue |
| POST | /projects/{projectId}/restart | frontend/src/views/DeploymentsList.vue, frontend/src/views/ProjectDetail.vue |
| GET | /projects/{projectId}/env | frontend/src/router.js, frontend/src/views/ProjectDetail.vue, frontend/src/views/ProjectEnvVars.vue |
| POST | /projects/{projectId}/env | frontend/src/router.js, frontend/src/views/ProjectDetail.vue, frontend/src/views/ProjectEnvVars.vue |
| POST | /projects/{projectId}/env/bulk | frontend/src/views/ProjectEnvVars.vue |
| PATCH | /projects/{projectId}/env/{envId} | frontend/src/views/ProjectDetail.vue, frontend/src/views/ProjectEnvVars.vue |
| DELETE | /projects/{projectId}/env/{envId} | frontend/src/views/ProjectDetail.vue, frontend/src/views/ProjectEnvVars.vue |
| GET | /projects/{projectId}/secrets | frontend/src/components/ProjectSecrets.vue, frontend/src/router.js |
| POST | /projects/{projectId}/secrets | frontend/src/components/ProjectSecrets.vue, frontend/src/router.js |
| DELETE | /projects/{projectId}/secrets/{secretId} | frontend/src/components/ProjectSecrets.vue |
| GET | /projects/{projectId}/domains | frontend/src/components/AddDomainModal.vue, frontend/src/router.js, frontend/src/views/Domains.vue, frontend/src/views/ProjectDetail.vue, frontend/src/views/ProjectDomains.vue |
| POST | /projects/{projectId}/domains | frontend/src/components/AddDomainModal.vue, frontend/src/router.js, frontend/src/views/Domains.vue, frontend/src/views/ProjectDetail.vue, frontend/src/views/ProjectDomains.vue |
| GET | /projects/{projectId}/domains/records | frontend/src/views/ProjectDomains.vue |
| GET | /projects/{projectId}/domains/{domainId} | frontend/src/router.js, frontend/src/views/Domains.vue, frontend/src/views/ProjectDetail.vue, frontend/src/views/ProjectDomains.vue |
| DELETE | /projects/{projectId}/domains/{domainId} | frontend/src/router.js, frontend/src/views/Domains.vue, frontend/src/views/ProjectDetail.vue, frontend/src/views/ProjectDomains.vue |
| POST | /projects/{projectId}/domains/{domainId}/verify | frontend/src/views/Domains.vue, frontend/src/views/ProjectDetail.vue, frontend/src/views/ProjectDomains.vue |
| POST | /projects/{projectId}/domains/{domainId}/reverify | frontend/src/views/ProjectDomains.vue |
| GET | /projects/{projectId}/dns | frontend/src/router.js, frontend/src/views/ProjectDomains.vue |
| GET | /projects/{projectId}/dns/records | frontend/src/router.js, frontend/src/views/ProjectDomains.vue |
| GET | /projects/{projectId}/compose | frontend/src/router.js, frontend/src/views/ProjectSourceRuntime.vue |
| PUT | /projects/{projectId}/compose | frontend/src/router.js, frontend/src/views/ProjectSourceRuntime.vue |
| POST | /projects/{projectId}/compose/validate | frontend/src/views/ProjectSourceRuntime.vue |
| GET | /projects/{projectId}/compose/preview | frontend/src/views/ProjectSourceRuntime.vue |
| GET | /projects/{projectId}/logs | frontend/src/router.js, frontend/src/views/ProjectDetail.vue |
| GET | /projects/{projectId}/logs/stream | frontend/src/router.js |
| GET | /projects/{projectId}/metrics | frontend/src/router.js |
| GET | /projects/{projectId}/traffic | frontend/src/views/ProjectDetail.vue |
| GET | /projects/{projectId}/events | frontend/src/router.js |
| GET | /projects/{projectId}/pool | frontend/src/router.js |
| POST | /projects/{projectId}/pool/drain | frontend/src/router.js |
| GET | /projects/{projectId}/status | frontend/src/router.js |
| GET | /projects/{projectId}/liveness | frontend/src/router.js |
| GET | /projects/{projectId}/replicas | frontend/src/router.js |
| POST | /projects/{projectId}/replicas/batch/start | frontend/src/views/DeploymentsList.vue |
| POST | /projects/{projectId}/replicas/batch/stop | frontend/src/views/DeploymentsList.vue |
| GET | /projects/{projectId}/replicas/{n} | frontend/src/router.js, frontend/src/views/VmDetail.vue |
| POST | /projects/{projectId}/replicas/{n}/start | frontend/src/router.js, frontend/src/views/VmDetail.vue |
| POST | /projects/{projectId}/replicas/{n}/stop | frontend/src/router.js, frontend/src/views/VmDetail.vue |
| POST | /projects/{projectId}/replicas/{n}/restart | frontend/src/router.js, frontend/src/views/VmDetail.vue |
| DELETE | /projects/{projectId}/replicas/{n} | frontend/src/router.js, frontend/src/views/VmDetail.vue |
| GET | /projects/{projectId}/replicas/{n}/logs | frontend/src/router.js, frontend/src/views/VmDetail.vue |
| GET | /projects/{projectId}/replicas/{n}/metrics | frontend/src/views/VmDetail.vue |
| GET | /projects/{projectId}/replicas/{n}/traffic | frontend/src/views/VmDetail.vue |
| GET | /projects/{projectId}/replicas/{n}/health | frontend/src/views/VmDetail.vue |
| GET | /projects/{projectId}/replicas/{n}/ssh-info | frontend/src/views/VmDetail.vue |
| POST | /projects/{projectId}/replicas/{n}/ssh-cert | frontend/src/views/VmDetail.vue |
| POST | /projects/{projectId}/replicas/{n}/exec | frontend/src/views/VmDetail.vue |
| GET | /projects/{projectId}/replicas/{n}/console | frontend/src/views/VmDetail.vue |
| GET | /projects/{projectId}/deployments | frontend/src/router.js, frontend/src/views/NewDeployment.vue, frontend/src/views/ProjectDetail.vue |
| POST | /projects/{projectId}/deployments | frontend/src/router.js, frontend/src/views/NewDeployment.vue, frontend/src/views/ProjectDetail.vue |
| GET | /projects/{projectId}/deployments/upload | frontend/src/views/DeploymentDetail.vue |
| GET | /projects/{projectId}/deployments/{deployId} | frontend/src/router.js, frontend/src/views/DeploymentDetail.vue |
| GET | /projects/{projectId}/deployments/{deployId}/checks | frontend/src/router.js, frontend/src/views/DeploymentDetail.vue |
| PUT | /projects/{projectId}/deployments/{deployId}/checks | frontend/src/router.js, frontend/src/views/DeploymentDetail.vue |
| PATCH | /projects/{projectId}/deployments/{deployId}/checks/{checkName} | frontend/src/views/DeploymentDetail.vue |
| PUT | /projects/{projectId}/deployments/{deployId}/rollout | frontend/src/views/DeploymentDetail.vue |
| GET | /projects/{projectId}/deployments/{deployId}/logs | frontend/src/router.js, frontend/src/views/DeploymentDetail.vue |
| POST | /projects/{projectId}/deployments/{deployId}/promote | frontend/src/views/ProjectDetail.vue |
| POST | /projects/{projectId}/deployments/{deployId}/rollback | frontend/src/views/ProjectDetail.vue |
| GET | /projects/{projectId}/deployments/{deployId}/source | frontend/src/router.js, frontend/src/views/DeploymentDetail.vue |
| GET | /projects/{projectId}/deployments/{deployId}/og | frontend/src/router.js |
| GET | /projects/{projectId}/settings/general | frontend/src/router.js |
| PATCH | /projects/{projectId}/settings/general | frontend/src/router.js |
| POST | /projects/{projectId}/avatar | frontend/src/router.js |
| POST | /projects/{projectId}/transfer | frontend/src/router.js |
| GET | /projects/{projectId}/settings/build | frontend/src/router.js |
| PUT | /projects/{projectId}/settings/build | frontend/src/router.js |
| GET | /projects/{projectId}/settings/checks | frontend/src/router.js |
| POST | /projects/{projectId}/settings/checks | frontend/src/router.js |
| GET | /projects/{projectId}/settings/rollout | frontend/src/router.js |
| PUT | /projects/{projectId}/settings/rollout | frontend/src/router.js |
| GET | /projects/{projectId}/settings/build-machine | frontend/src/router.js |
| PUT | /projects/{projectId}/settings/build-machine | frontend/src/router.js |
| POST | /projects/{projectId}/settings/ignore-command | frontend/src/router.js |
| GET | /projects/{projectId}/settings/framework | frontend/src/router.js |
| GET | /projects/{projectId}/environments | frontend/src/router.js, frontend/src/views/ProjectEnvironment.vue |
| POST | /projects/{projectId}/environments | frontend/src/router.js, frontend/src/views/ProjectEnvironment.vue |
| GET | /projects/{projectId}/environments/available | frontend/src/views/ProjectEnvironment.vue |
| GET | /projects/{projectId}/environments/{envId} | frontend/src/views/ProjectEnvironment.vue |
| PATCH | /projects/{projectId}/environments/{envId} | frontend/src/views/ProjectEnvironment.vue |
| DELETE | /projects/{projectId}/environments/{envId} | frontend/src/views/ProjectEnvironment.vue |
| POST | /projects/{projectId}/environments/{envId}/branch | frontend/src/views/ProjectEnvironment.vue |
| POST | /projects/{projectId}/environments/{envId}/domain | frontend/src/views/ProjectEnvironment.vue |
| GET | /projects/{projectId}/environments/{envId}/range | frontend/src/views/ProjectEnvironment.vue |
| GET | /projects/{projectId}/settings/git | frontend/src/router.js |
| PUT | /projects/{projectId}/settings/git | frontend/src/router.js |
| POST | /projects/{projectId}/settings/git/sync | frontend/src/router.js |
| PATCH | /projects/{projectId}/settings/git/toggles | frontend/src/router.js |
| GET | /projects/{projectId}/settings/git/lfs | frontend/src/router.js |
| PUT | /projects/{projectId}/settings/git/lfs | frontend/src/router.js |
| GET | /projects/{projectId}/hooks | frontend/src/router.js |
| POST | /projects/{projectId}/hooks | frontend/src/router.js |
| DELETE | /projects/{projectId}/hooks/{hookId} | frontend/src/router.js |
| POST | /projects/{projectId}/hooks/{hookId}/trigger | frontend/src/router.js |
| GET | /projects/{projectId}/settings/deployment-protection | frontend/src/router.js |
| PUT | /projects/{projectId}/settings/deployment-protection | frontend/src/router.js |
| GET | /projects/{projectId}/settings/oidc | frontend/src/router.js |
| PUT | /projects/{projectId}/settings/oidc | frontend/src/router.js |
| GET | /projects/{projectId}/settings/functions | frontend/src/router.js |
| PUT | /projects/{projectId}/settings/functions | frontend/src/router.js |
| GET | /projects/{projectId}/crons | frontend/src/components/ProjectCron.vue, frontend/src/router.js |
| POST | /projects/{projectId}/crons | frontend/src/components/ProjectCron.vue, frontend/src/router.js |
| GET | /projects/{projectId}/crons/history | frontend/src/router.js |
| GET | /projects/{projectId}/crons/{cronId} | frontend/src/components/ProjectCron.vue |
| PATCH | /projects/{projectId}/crons/{cronId} | frontend/src/components/ProjectCron.vue |
| DELETE | /projects/{projectId}/crons/{cronId} | frontend/src/components/ProjectCron.vue |
| POST | /projects/{projectId}/crons/{cronId}/run | frontend/src/components/ProjectCron.vue |
| GET | /projects/{projectId}/members | frontend/src/router.js, frontend/src/views/ProjectMembers.vue |
| POST | /projects/{projectId}/members | frontend/src/router.js, frontend/src/views/ProjectMembers.vue |
| GET | /projects/{projectId}/members/{username} | frontend/src/views/ProjectMembers.vue |
| PATCH | /projects/{projectId}/members/{username} | frontend/src/views/ProjectMembers.vue |
| DELETE | /projects/{projectId}/members/{username} | frontend/src/views/ProjectMembers.vue |
| POST | /projects/{projectId}/members/invite | frontend/src/views/ProjectMembers.vue |
| GET | /projects/{projectId}/drains | frontend/src/router.js |
| POST | /projects/{projectId}/drains | frontend/src/router.js |
| DELETE | /projects/{projectId}/drains/{drainId} | frontend/src/router.js |
| POST | /projects/{projectId}/drains/{drainId}/test | frontend/src/router.js |
| GET | /projects/{projectId}/alerts | frontend/src/router.js |
| POST | /projects/{projectId}/alerts | frontend/src/router.js |
| GET | /projects/{projectId}/alerts/{alertId} | frontend/src/router.js |
| PATCH | /projects/{projectId}/alerts/{alertId} | frontend/src/router.js |
| DELETE | /projects/{projectId}/alerts/{alertId} | frontend/src/router.js |
| POST | /projects/{projectId}/alerts/{alertId}/silence | frontend/src/router.js |
| POST | /projects/{projectId}/alerts/{alertId}/unsilence | frontend/src/router.js |
| GET | /projects/{projectId}/settings/security | frontend/src/router.js |
| PUT | /projects/{projectId}/settings/security | frontend/src/router.js |
| GET | /projects/{projectId}/settings/retention | frontend/src/router.js |
| PUT | /projects/{projectId}/settings/retention | frontend/src/router.js |
| GET | /projects/{projectId}/settings/networking | frontend/src/router.js |
| PUT | /projects/{projectId}/settings/networking | frontend/src/router.js |
| GET | /projects/{projectId}/settings/advanced | frontend/src/router.js |
| PUT | /projects/{projectId}/settings/advanced | frontend/src/router.js |
| GET | /projects/{projectId}/settings/passport | frontend/src/router.js |
| PUT | /projects/{projectId}/settings/passport | frontend/src/router.js |
| GET | /projects/{projectId}/settings/microfrontends | frontend/src/router.js |
| PUT | /projects/{projectId}/settings/microfrontends | frontend/src/router.js |
| GET | /projects/{projectId}/redirects | frontend/src/router.js |
| POST | /projects/{projectId}/redirects | frontend/src/router.js |
| DELETE | /projects/{projectId}/redirects/{redirectId} | frontend/src/router.js |
| PUT | /projects/{projectId}/redirects/bulk | frontend/src/router.js |
| GET | /projects/{projectId}/analytics/usage | frontend/src/components/ProjectAnalytics.vue |
| GET | /projects/{projectId}/analytics/usage/timeseries | frontend/src/components/ProjectAnalytics.vue |
| GET | /projects/{projectId}/analytics/paths | frontend/src/components/ProjectAnalytics.vue |
| GET | /projects/{projectId}/analytics/status-codes | frontend/src/components/ProjectAnalytics.vue |
| GET | /projects/{projectId}/analytics/bandwidth | frontend/src/components/ProjectAnalytics.vue |
| GET | /projects/{projectId}/analytics/requests | frontend/src/components/ProjectAnalytics.vue |
| GET | /projects/{projectId}/analytics/invocations | frontend/src/components/ProjectAnalytics.vue |
| GET | /projects/{projectId}/observability/web-vitals | frontend/src/components/ProjectAnalytics.vue |
| POST | /projects/{projectId}/observability/web-vitals/beacon | frontend/src/components/ProjectAnalytics.vue |
| GET | /projects/{projectId}/observability/web-vitals/timeseries | frontend/src/components/ProjectAnalytics.vue |
| GET | /projects/{projectId}/observability/lcp | frontend/src/components/ProjectAnalytics.vue |
| GET | /projects/{projectId}/observability/cls | frontend/src/components/ProjectAnalytics.vue |
| GET | /projects/{projectId}/observability/fid | frontend/src/components/ProjectAnalytics.vue |
| GET | /global/analytics | frontend/src/views/Analytics.vue |
| GET | /global/analytics/timeseries | frontend/src/views/Analytics.vue |
| GET | /usage | frontend/src/views/Analytics.vue |
| GET | /usage/bandwidth | frontend/src/views/Analytics.vue |
| GET | /usage/requests | frontend/src/views/Analytics.vue |
| GET | /usage/timeseries | frontend/src/views/Analytics.vue |
| GET | /replicas | frontend/src/App.vue, frontend/src/router.js |
| GET | /replicas/{replicaId} | frontend/src/router.js |
| GET | /projects/{projectId}/firewall/rules | frontend/src/components/ProjectFirewall.vue |
| POST | /projects/{projectId}/firewall/rules | frontend/src/components/ProjectFirewall.vue |
| GET | /projects/{projectId}/firewall/rules/{ruleId} | frontend/src/components/ProjectFirewall.vue |
| DELETE | /projects/{projectId}/firewall/rules/{ruleId} | frontend/src/components/ProjectFirewall.vue |
| PATCH | /projects/{projectId}/firewall/rules/{ruleId} | frontend/src/components/ProjectFirewall.vue |
| GET | /projects/{projectId}/firewall/events | frontend/src/router.js |
| GET | /projects/{projectId}/firewall/stats | frontend/src/router.js |
| POST | /projects/{projectId}/firewall/whitelist | frontend/src/router.js |
| GET | /projects/{projectId}/cache/stats | frontend/src/views/ProjectCache.vue |
| POST | /projects/{projectId}/cache/purge | frontend/src/views/ProjectCache.vue |
| POST | /projects/{projectId}/cache/purge/path | frontend/src/views/ProjectCache.vue |
| GET | /volumes | frontend/src/router.js, frontend/src/views/Servers.vue |
| POST | /volumes | frontend/src/router.js, frontend/src/views/Servers.vue |
| GET | /volumes/{volumeId} | frontend/src/router.js, frontend/src/views/Servers.vue |
| DELETE | /volumes/{volumeId} | frontend/src/router.js, frontend/src/views/Servers.vue |
| POST | /volumes/{volumeId}/resize | frontend/src/router.js, frontend/src/views/Servers.vue |
| GET | /volumes/{volumeId}/usage | frontend/src/router.js, frontend/src/views/Servers.vue |
| GET | /projects/{projectId}/volumes | frontend/src/router.js |
| POST | /projects/{projectId}/volumes | frontend/src/router.js |
| GET | /projects/{projectId}/volumes/{volumeId} | frontend/src/router.js |
| DELETE | /projects/{projectId}/volumes/{volumeId} | frontend/src/router.js |
| POST | /projects/{projectId}/volumes/{volumeId}/resize | frontend/src/router.js |
| GET | /projects/{projectId}/volumes/{volumeId}/usage | frontend/src/router.js |
| GET | /images | frontend/src/App.vue, frontend/src/components/NewProjectModal.vue, frontend/src/router.js, frontend/src/views/Images.vue |
| GET | /images/base | frontend/src/router.js, frontend/src/views/Images.vue |
| GET | /images/base/readiness | frontend/src/router.js, frontend/src/views/Images.vue |
| POST | /images/custom | frontend/src/api/client.js |
| GET | /images/search | frontend/src/router.js, frontend/src/views/Images.vue |
| GET | /images/{reference} | frontend/src/router.js, frontend/src/views/Images.vue |
| DELETE | /images/{reference} | frontend/src/router.js, frontend/src/views/Images.vue |
| POST | /images/prune | frontend/src/views/Images.vue |
| GET | /images/stats | frontend/src/router.js, frontend/src/views/Images.vue |
| GET | /orgs/events | frontend/src/views/Teams.vue |
| GET | /overview | frontend/src/views/DeploymentsList.vue, frontend/src/views/Servers.vue |
| GET | /vms | frontend/src/App.vue, frontend/src/router.js, frontend/src/views/DeploymentsList.vue |
| GET | /vms/{replicaId} | frontend/src/router.js, frontend/src/views/ReplicaStream.vue, frontend/src/views/VmDetail.vue |
| POST | /vms/{replicaId}/start | frontend/src/views/VmDetail.vue |
| POST | /vms/{replicaId}/stop | frontend/src/views/VmDetail.vue |
| POST | /vms/{replicaId}/restart | frontend/src/views/VmDetail.vue |
| DELETE | /vms/{replicaId} | frontend/src/router.js, frontend/src/views/ReplicaStream.vue, frontend/src/views/VmDetail.vue |
| GET | /vms/{replicaId}/domains | frontend/src/router.js, frontend/src/views/VmDetail.vue |
| GET | /vms/{replicaId}/logs | frontend/src/router.js, frontend/src/views/VmDetail.vue |
| GET | /vms/{replicaId}/logs/stream | frontend/src/router.js |
| GET | /vms/{replicaId}/metrics | frontend/src/router.js, frontend/src/views/VmDetail.vue |
| GET | /vms/{replicaId}/traffic | frontend/src/router.js, frontend/src/views/VmDetail.vue |
| GET | /vms/{replicaId}/health | frontend/src/router.js, frontend/src/views/VmDetail.vue |
| GET | /vms/{replicaId}/ssh-info | frontend/src/router.js, frontend/src/views/VmDetail.vue |
| POST | /vms/{replicaId}/ssh-cert | frontend/src/views/VmDetail.vue |
| POST | /vms/{replicaId}/exec | frontend/src/views/VmDetail.vue |
| GET | /vms/{replicaId}/console | frontend/src/views/VmDetail.vue |
| GET | /host/overview | frontend/src/App.vue, frontend/src/router.js, frontend/src/views/Settings.vue |
| GET | /logs | frontend/src/App.vue, frontend/src/router.js, frontend/src/views/Logs.vue, frontend/src/views/Settings.vue |
| GET | /host/ports | frontend/src/router.js, frontend/src/views/Settings.vue |
| GET | /host/kernel | frontend/src/router.js, frontend/src/views/Settings.vue |
| GET | /host/prerequisites | frontend/src/App.vue, frontend/src/router.js, frontend/src/views/Settings.vue |
| GET | /host/runtime | frontend/src/App.vue, frontend/src/router.js, frontend/src/views/Settings.vue |
| GET | /traffic | frontend/src/App.vue, frontend/src/router.js, frontend/src/views/Traffic.vue |
| DELETE | /traffic | frontend/src/App.vue, frontend/src/router.js, frontend/src/views/Traffic.vue |
| GET | /traffic/search | frontend/src/views/Traffic.vue |
| GET | /servers | frontend/src/App.vue, frontend/src/router.js, frontend/src/views/Servers.vue |
| POST | /servers | frontend/src/App.vue, frontend/src/router.js, frontend/src/views/Servers.vue |
| GET | /servers/{id} | frontend/src/router.js, frontend/src/views/Servers.vue |
| POST | /servers/{id}/heartbeat | frontend/src/views/Servers.vue |
| GET | /servers/{id}/ssh | frontend/src/views/Servers.vue |
| DELETE | /servers/{id} | frontend/src/router.js, frontend/src/views/Servers.vue |
| GET | /users | frontend/src/views/Teams.vue |
| POST | /users | frontend/src/views/Teams.vue |
| DELETE | /users/{username} | frontend/src/views/Teams.vue |
| GET | /roles | frontend/src/views/Teams.vue |
| POST | /roles | frontend/src/views/Teams.vue |
| GET | /roles/{roleId} | frontend/src/views/Teams.vue |
| PATCH | /roles/{roleId} | frontend/src/views/Teams.vue |
| DELETE | /roles/{roleId} | frontend/src/views/Teams.vue |
| GET | /permissions | frontend/src/views/Teams.vue |
| GET | /roles/{roleId}/permissions | frontend/src/views/Teams.vue |
| PUT | /roles/{roleId}/permissions | frontend/src/views/Teams.vue |
| POST | /roles/{roleId}/permissions/{permissionId} | frontend/src/views/Teams.vue |
| DELETE | /roles/{roleId}/permissions/{permissionId} | frontend/src/views/Teams.vue |
| POST | /projects/{projectId}/export | frontend/src/router.js |
| POST | /projects/{projectId}/import | frontend/src/router.js |
| PUT | /projects/{projectId}/ssh | frontend/src/router.js |
| POST | /projects/{projectId}/git/import | frontend/src/views/Builds.vue |
| POST | /projects/{projectId}/deployments/git | frontend/src/views/Builds.vue |
| GET | /projects/{projectId}/builds | frontend/src/router.js, frontend/src/views/Builds.vue |
| POST | /projects/{projectId}/builds | frontend/src/router.js, frontend/src/views/Builds.vue |
| POST | /projects/{projectId}/builds/run | frontend/src/views/Builds.vue |
| GET | /projects/{projectId}/builds/{buildId}/logs | frontend/src/router.js |
| GET | /projects/{projectId}/builds/{buildId}/logs/stream | frontend/src/router.js |
| GET | /projects/{projectId}/git/branches | frontend/src/views/Builds.vue |
| GET | /projects/{projectId}/rollouts | frontend/src/router.js |
| GET | /projects/{projectId}/services | frontend/src/router.js |
| GET | /projects/{projectId}/services/{serviceName} | frontend/src/router.js |
| POST | /projects/{projectId}/services/{serviceName}/scale | frontend/src/router.js |
| GET | /projects/{projectId}/networks | frontend/src/router.js |
| POST | /projects/{projectId}/networks | frontend/src/router.js |
| GET | /images/ml | frontend/src/router.js |
