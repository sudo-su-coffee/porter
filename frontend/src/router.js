// Porter Vue router: core workflows use dedicated components; operational
// resource screens use one real ResourceManager with route-specific API schema.
import { createRouter, createWebHashHistory } from "vue-router";
import { getToken } from "./api/client";

import DeploymentsList from "./views/DeploymentsList.vue";
import Analytics from "./views/Analytics.vue";
import ProjectDetail from "./views/ProjectDetail.vue";
import DeploymentDetail from "./views/DeploymentDetail.vue";
import VmDetail from "./views/VmDetail.vue";
import Images from "./views/Images.vue";
import Servers from "./views/Servers.vue";
import Logs from "./views/Logs.vue";
import Traffic from "./views/Traffic.vue";
import Domains from "./views/Domains.vue";
import Teams from "./views/Teams.vue";
import Settings from "./views/Settings.vue";
import Login from "./views/Login.vue";
import ResourceManager from "./views/ResourceManager.vue";
import ReplicaStream from "./views/ReplicaStream.vue";
import LiveLogStream from "./views/LiveLogStream.vue";
import Builds from "./views/Builds.vue";
import ProjectEnvironment from "./views/ProjectEnvironment.vue";
import ProjectEnvVars from "./views/ProjectEnvVars.vue";
import ProjectMembers from "./views/ProjectMembers.vue";
import ProjectSettingsPage from "./views/ProjectSettingsPage.vue";
import ProjectDomains from "./views/ProjectDomains.vue";
import ProjectAnalytics from "./components/ProjectAnalytics.vue";
import ProjectCron from "./components/ProjectCron.vue";
import ProjectFirewall from "./components/ProjectFirewall.vue";
import ProjectSecrets from "./components/ProjectSecrets.vue";

const resource = (title, description, endpoint, extra = {}) => ({ resource: { title, description, endpoint, back: true, ...extra } });

const routes = [
  { path: "/", name: "list", component: DeploymentsList },
  { path: "/analytics", name: "analytics", component: Analytics },
  { path: "/projects/:id", name: "project", component: ProjectDetail, props: true },
  { path: "/projects/:projectId/deployments/:deploymentId", name: "deployment", component: DeploymentDetail },
  { path: "/projects/:projectId/builds/:buildId/logs", name: "build-logs", component: LiveLogStream, meta: { streamTitle: "Build logs", streamEndpoint: "/projects/:projectId/builds/:buildId/logs/stream", back: "/projects/:projectId/builds" } },
  { path: "/projects/:projectId/builds", name: "project-builds", component: Builds },
  { path: "/projects/:projectId/deployments", name: "project-deployments", component: ResourceManager, meta: resource("Deployments", "Deployment history and rollout state from the control plane.", "/projects/:projectId/deployments") },
  { path: "/projects/:projectId/services", name: "project-services", component: ResourceManager, meta: resource("Services", "Service and replica topology for this project.", "/projects/:projectId/services", { rowActions: [{ label: "Scale", endpoint: "/projects/:projectId/services/:name/scale", prompt: "Desired replica count", promptDefault: "1", body: (_row, value) => ({ replicas: Number(value) }), success: "Service scale requested" }] }) },
  { path: "/projects/:projectId/domains", name: "project-domains", component: ProjectDomains },
  { path: "/projects/:projectId/secrets", name: "project-secrets", component: ProjectSecrets, props: (route) => ({ projectId: route.params.projectId }) },
  { path: "/projects/:projectId/networks", name: "project-networks", component: ResourceManager, meta: resource("Networks", "Project network allocations and direct TAP boundary metadata.", "/projects/:projectId/networks", { createLabel: "Create network", createFields: [{ key: "name", label: "Name", placeholder: "private" }, { key: "cidr", label: "CIDR", placeholder: "10.42.0.0/24", required: false }, { key: "driver", label: "Driver", placeholder: "bridge", required: false }] }) },
  { path: "/projects/:projectId/environments", name: "project-environments", component: ProjectEnvironment },
  { path: "/projects/:projectId/env", name: "project-env-vars", component: ProjectEnvVars },
  { path: "/projects/:projectId/hooks", name: "project-hooks", component: ResourceManager, meta: resource("Webhooks", "Project hooks and delivery configuration.", "/projects/:projectId/hooks", { createLabel: "Add webhook", createFields: [{ key: "name", label: "Name", placeholder: "Deploy hook", required: false }, { key: "url", label: "Endpoint URL", placeholder: "https://example.com/porter-hook" }, { key: "events", label: "Events", placeholder: "deployment.ready,replica.health", type: "csv", required: false }], rowActions: [{ label: "Trigger", endpoint: "/projects/:projectId/hooks/:id/trigger", success: "Webhook triggered" }, { label: "Delete", method: "DELETE", endpoint: "/projects/:projectId/hooks/:id", danger: true, confirm: "Delete {name}?", success: "Webhook deleted" }] }) },
  { path: "/projects/:projectId/crons", name: "project-crons", component: ProjectCron, props: (route) => ({ projectId: route.params.projectId }) },
  { path: "/projects/:projectId/crons/history", name: "project-cron-history", component: ResourceManager, meta: resource("Cron history", "Recorded cron execution history.", "/projects/:projectId/crons/history") },
  { path: "/projects/:projectId/alerts", name: "project-alerts", component: ResourceManager, meta: resource("Alerts", "Project health and operational alerts.", "/projects/:projectId/alerts", { createLabel: "Create alert", createFields: [{ key: "name", label: "Name", placeholder: "High error rate" }, { key: "metric", label: "Metric", placeholder: "5xx_rate" }, { key: "threshold", label: "Threshold", type: "number", default: 1 }, { key: "op", label: "Operator", type: "select", options: [">", ">=", "<", "<="], default: ">=" }, { key: "cooldown_s", label: "Cooldown seconds", type: "number", default: 300 }], rowActions: [{ label: "Update", method: "PATCH", endpoint: "/projects/:projectId/alerts/:id", body: (row) => ({ threshold: Number(row.threshold || 0), op: row.op || ">=", silenced: Boolean(row.silenced) }), success: "Alert updated" }, { label: "Silence", endpoint: "/projects/:projectId/alerts/:id/silence", when: { key: "silenced", equals: false }, success: "Alert silenced" }, { label: "Unsilence", endpoint: "/projects/:projectId/alerts/:id/unsilence", when: { key: "silenced", equals: true }, success: "Alert unsilenced" }, { label: "Delete", method: "DELETE", endpoint: "/projects/:projectId/alerts/:id", danger: true, confirm: "Delete {name}?", success: "Alert deleted" }] }) },
  { path: "/projects/:projectId/drains", name: "project-drains", component: ResourceManager, meta: resource("Log drains", "External log drain configuration.", "/projects/:projectId/drains", { createLabel: "Add drain", createFields: [{ key: "name", label: "Name", placeholder: "Operations sink", required: false }, { key: "endpoint", label: "Endpoint", placeholder: "https://logs.example.com/ingest" }, { key: "kind", label: "Kind", placeholder: "webhook", required: false }], rowActions: [{ label: "Test", endpoint: "/projects/:projectId/drains/:id/test", success: "Drain test sent" }, { label: "Delete", method: "DELETE", endpoint: "/projects/:projectId/drains/:id", danger: true, confirm: "Delete {name}?", success: "Drain deleted" }] }) },
  { path: "/projects/:projectId/redirects", name: "project-redirects", component: ResourceManager, meta: resource("Redirects", "HTTP redirect rules for this project.", "/projects/:projectId/redirects", { createLabel: "Add redirect", createFields: [{ key: "source", label: "Source path", placeholder: "/old-path" }, { key: "target", label: "Target URL", placeholder: "/new-path" }, { key: "permanent", label: "Permanent (308)", type: "checkbox", default: true }], rowActions: [{ label: "Delete", method: "DELETE", endpoint: "/projects/:projectId/redirects/:id", danger: true, confirm: "Delete redirect {name}?", success: "Redirect deleted" }] }) },
  { path: "/projects/:projectId/firewall", name: "project-firewall", component: ProjectFirewall, props: (route) => ({ projectId: route.params.projectId }) },
  { path: "/projects/:projectId/members", name: "project-members", component: ProjectMembers },
  { path: "/projects/:projectId/volumes", name: "project-volumes", component: ResourceManager, meta: resource("Volumes", "Persistent volume records and host backing state.", "/projects/:projectId/volumes", { createLabel: "Create volume", createFields: [{ key: "name", label: "Name", placeholder: "data" }, { key: "size_mib", label: "Size (MiB)", type: "number", default: 1024 }, { key: "mount_path", label: "Mount path", placeholder: "/data", required: false }], rowActions: [{ label: "Resize", endpoint: "/projects/:projectId/volumes/:id/resize", prompt: "New size in MiB", promptDefault: "1024", body: (_row, value) => ({ size_mib: Number(value) }), success: "Volume resize requested" }, { label: "Usage", method: "GET", endpoint: "/projects/:projectId/volumes/:id/usage", success: "Volume usage refreshed" }, { label: "Delete", method: "DELETE", endpoint: "/projects/:projectId/volumes/:id", danger: true, confirm: "Delete volume {name}?", success: "Volume deleted" }] }) },
  { path: "/projects/:projectId/analytics", name: "project-analytics", component: ProjectAnalytics, props: (route) => ({ projectId: route.params.projectId }) },
  { path: "/projects/:projectId/metrics", name: "project-metrics", component: ResourceManager, meta: resource("Project metrics", "Replica metric samples and health signals.", "/projects/:projectId/metrics") },
  { path: "/projects/:projectId/events", name: "project-events", component: ResourceManager, meta: resource("Project events", "Recorded deployment and health events.", "/projects/:projectId/events") },
  { path: "/projects/:projectId/pool", name: "project-pool", component: ResourceManager, meta: resource("Replica pool", "Desired/current replica pool state.", "/projects/:projectId/pool", { objectActions: [{ label: "Drain pool", endpoint: "/projects/:projectId/pool/drain", danger: true, confirm: "Drain all project replicas?", success: "Replica pool draining" }] }) },
  { path: "/projects/:projectId/logs", name: "project-logs", component: LiveLogStream, meta: { streamTitle: "Application logs", streamEndpoint: "/projects/:projectId/logs/stream", back: "/projects/:projectId" } },
  { path: "/projects/:projectId/settings/general", name: "project-general-settings", component: ProjectSettingsPage, meta: { settingsSection: "general", settingsTitle: "General project settings" } },
  { path: "/projects/:projectId/settings/build", name: "project-build-settings", component: ProjectSettingsPage, meta: { settingsSection: "build", settingsTitle: "Build settings" } },
  { path: "/projects/:projectId/settings/git", name: "project-git-settings", component: ProjectSettingsPage, meta: { settingsSection: "git", settingsTitle: "Git settings" } },
  { path: "/projects/:projectId/settings/functions", name: "project-functions", component: ProjectSettingsPage, meta: { settingsSection: "functions", settingsTitle: "Functions settings" } },
  { path: "/projects/:projectId/settings/security", name: "project-security", component: ProjectSettingsPage, meta: { settingsSection: "security", settingsTitle: "Security settings" } },
  { path: "/projects/:projectId/settings/networking", name: "project-networking-settings", component: ProjectSettingsPage, meta: { settingsSection: "networking", settingsTitle: "Networking settings" } },
  { path: "/projects/:projectId/settings/checks", name: "project-check-settings", component: ProjectSettingsPage, meta: { settingsSection: "checks", settingsTitle: "Deployment checks" } },
  { path: "/projects/:projectId/settings/rollout", name: "project-rollout-settings", component: ProjectSettingsPage, meta: { settingsSection: "rollout", settingsTitle: "Rollout settings" } },
  { path: "/projects/:projectId/settings/build-machine", name: "project-build-machine", component: ProjectSettingsPage, meta: { settingsSection: "build-machine", settingsTitle: "Build machine" } },
  { path: "/projects/:projectId/settings/deployment-protection", name: "project-deployment-protection", component: ProjectSettingsPage, meta: { settingsSection: "deployment-protection", settingsTitle: "Deployment protection" } },
  { path: "/projects/:projectId/settings/oidc", name: "project-oidc", component: ProjectSettingsPage, meta: { settingsSection: "oidc", settingsTitle: "OIDC settings" } },
  { path: "/projects/:projectId/settings/retention", name: "project-retention", component: ProjectSettingsPage, meta: { settingsSection: "retention", settingsTitle: "Retention settings" } },
  { path: "/projects/:projectId/settings/advanced", name: "project-advanced-settings", component: ProjectSettingsPage, meta: { settingsSection: "advanced", settingsTitle: "Advanced settings" } },
  { path: "/projects/:projectId/settings/passport", name: "project-passport", component: ProjectSettingsPage, meta: { settingsSection: "passport", settingsTitle: "Passport settings" } },
  { path: "/projects/:projectId/settings/microfrontends", name: "project-microfrontends", component: ProjectSettingsPage, meta: { settingsSection: "microfrontends", settingsTitle: "Microfrontends settings" } },
  { path: "/vms/:id", name: "vm", component: VmDetail, props: true },
  { path: "/vms/:id/logs", name: "vm-logs", component: LiveLogStream, meta: { streamTitle: "Replica logs", streamEndpoint: "/vms/:id/logs/stream", back: "/vms/:id" } },
  { path: "/vms/:id/health", name: "vm-health", component: ReplicaStream, meta: { stream: "health" } },
  { path: "/vms/:id/metrics", name: "vm-metrics", component: ReplicaStream, meta: { stream: "metrics" } },
  { path: "/vms/:id/traffic", name: "vm-traffic", component: ReplicaStream, meta: { stream: "traffic" } },
  { path: "/vms/:id/ssh", name: "vm-ssh", component: ResourceManager, meta: resource("SSH information", "Replica SSH metadata and guest-channel readiness.", "/vms/:id/ssh-info") },
  { path: "/traffic", name: "traffic", component: Traffic },
  { path: "/domains", name: "domains", component: Domains },
  { path: "/teams", name: "teams", component: Teams },
  { path: "/settings", name: "settings", component: Settings },
  { path: "/images", name: "images", component: Images },
  { path: "/servers", name: "servers", component: Servers },
  { path: "/logs", name: "logs", component: Logs },
  { path: "/replicas", name: "replicas", component: ResourceManager, meta: resource("All replicas", "Global replica inventory across projects.", "/replicas", { back: false }) },
  { path: "/host/overview", name: "host-overview", component: ResourceManager, meta: resource("Host overview", "Host capacity and runtime state.", "/host/overview", { back: false }) },
  { path: "/host/prerequisites", name: "host-prerequisites", component: ResourceManager, meta: resource("Host readiness", "KVM, TAP, Firecracker, and artifact readiness checks.", "/host/prerequisites", { back: false }) },
  { path: "/host/runtime", name: "host-runtime", component: ResourceManager, meta: resource("Runtime configuration", "Direct Firecracker runtime configuration and state paths.", "/host/runtime", { back: false }) },
  { path: "/host/ports", name: "host-ports", component: ResourceManager, meta: resource("Host ports", "Host-to-guest port forwarding state.", "/host/ports", { back: false }) },
  { path: "/daemon-logs", name: "daemon-logs", component: ResourceManager, meta: resource("Daemon logs", "Control-plane operational log history.", "/logs", { back: false }) },
  { path: "/access", name: "access", component: Teams, meta: { accessTab: "orgs" } },
  { path: "/access/organizations", name: "access-organizations", component: Teams, meta: { accessTab: "orgs" } },
  { path: "/access/members", name: "access-members", component: Teams, meta: { accessTab: "members" } },
  { path: "/access/users", name: "access-users", component: Teams, meta: { accessTab: "users" } },
  { path: "/access/roles", name: "access-roles", component: Teams, meta: { accessTab: "roles" } },
  { path: "/access/permissions", name: "access-permissions", component: Teams, meta: { accessTab: "roles" } },
  { path: "/access/audit", name: "access-audit", component: Teams, meta: { accessTab: "orgs" } },
  { path: "/access/api-keys", name: "access-api-keys", component: Teams, meta: { accessTab: "apikeys" } },
  { path: "/login", name: "login", component: Login },
];

const router = createRouter({ history: createWebHashHistory(), routes });

router.beforeEach((to) => {
  if (to.name !== "login" && !getToken()) return { name: "login" };
});

export default router;
