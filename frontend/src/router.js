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

const resource = (title, description, endpoint, extra = {}) => ({ resource: { title, description, endpoint, back: true, ...extra } });

const routes = [
  { path: "/", name: "list", component: DeploymentsList },
  { path: "/analytics", name: "analytics", component: Analytics },
  { path: "/projects/:id", name: "project", component: ProjectDetail, props: true },
  { path: "/projects/:projectId/deployments/:deploymentId", name: "deployment", component: DeploymentDetail },
  { path: "/projects/:projectId/builds/:buildId/logs", name: "build-logs", component: LiveLogStream, meta: { streamTitle: "Build logs", streamEndpoint: "/projects/:projectId/builds/:buildId/logs/stream", back: "/projects/:projectId/builds" } },
  { path: "/projects/:projectId/builds", name: "project-builds", component: Builds },
  { path: "/projects/:projectId/deployments", name: "project-deployments", component: ResourceManager, meta: resource("Deployments", "Deployment history and rollout state from the control plane.", "/projects/:projectId/deployments") },
  { path: "/projects/:projectId/services", name: "project-services", component: ResourceManager, meta: resource("Services", "Service and replica topology for this project.", "/projects/:projectId/services") },
  { path: "/projects/:projectId/domains", name: "project-domains", component: ResourceManager, meta: resource("Domains", "Project domains and verification state.", "/projects/:projectId/domains") },
  { path: "/projects/:projectId/secrets", name: "project-secrets", component: ResourceManager, meta: resource("Secrets", "Opaque project secret metadata.", "/projects/:projectId/secrets") },
  { path: "/projects/:projectId/networks", name: "project-networks", component: ResourceManager, meta: resource("Networks", "Project network allocations and direct TAP boundary metadata.", "/projects/:projectId/networks") },
  { path: "/projects/:projectId/environments", name: "project-environments", component: ResourceManager, meta: resource("Environments", "Environment records and deployment branches.", "/projects/:projectId/environments", { createLabel: "Add environment", createFields: [{ key: "name", label: "Name", placeholder: "preview" }, { key: "branch", label: "Branch", placeholder: "main", required: false }, { key: "url", label: "URL", placeholder: "https://preview.example.com", required: false }, { key: "env_domain", label: "Environment domain", placeholder: "preview.example.com", required: false }], rowActions: [{ label: "Update", method: "PATCH", endpoint: "/projects/:projectId/environments/:id", body: (row) => ({ branch: row.branch || "", url: row.url || "", env_domain: row.env_domain || "" }), success: "Environment updated" }, { label: "Delete", method: "DELETE", endpoint: "/projects/:projectId/environments/:id", danger: true, confirm: "Delete {name}?", success: "Environment deleted" }] }) },
  { path: "/projects/:projectId/hooks", name: "project-hooks", component: ResourceManager, meta: resource("Webhooks", "Project hooks and delivery configuration.", "/projects/:projectId/hooks", { createLabel: "Add webhook", createFields: [{ key: "name", label: "Name", placeholder: "Deploy hook", required: false }, { key: "url", label: "Endpoint URL", placeholder: "https://example.com/porter-hook" }, { key: "events", label: "Events", placeholder: "deployment.ready,replica.health", type: "csv", required: false }], rowActions: [{ label: "Trigger", endpoint: "/projects/:projectId/hooks/:id/trigger", success: "Webhook triggered" }, { label: "Delete", method: "DELETE", endpoint: "/projects/:projectId/hooks/:id", danger: true, confirm: "Delete {name}?", success: "Webhook deleted" }] }) },
  { path: "/projects/:projectId/crons", name: "project-crons", component: ResourceManager, meta: resource("Cron jobs", "Scheduled project jobs and execution state.", "/projects/:projectId/crons", { createLabel: "Add cron job", createFields: [{ key: "name", label: "Name", placeholder: "Nightly task" }, { key: "schedule", label: "Schedule", placeholder: "0 * * * *" }, { key: "job_image", label: "Direct image reference", placeholder: "base://default" }], rowActions: [{ label: "Run now", endpoint: "/projects/:projectId/crons/:id/run", confirm: "Run {name} now?", success: "Cron job queued" }, { label: "Delete", method: "DELETE", endpoint: "/projects/:projectId/crons/:id", danger: true, confirm: "Delete {name}?", success: "Cron deleted" }] }) },
  { path: "/projects/:projectId/crons/history", name: "project-cron-history", component: ResourceManager, meta: resource("Cron history", "Recorded cron execution history.", "/projects/:projectId/crons/history") },
  { path: "/projects/:projectId/alerts", name: "project-alerts", component: ResourceManager, meta: resource("Alerts", "Project health and operational alerts.", "/projects/:projectId/alerts", { createLabel: "Create alert", createFields: [{ key: "name", label: "Name", placeholder: "High error rate" }, { key: "metric", label: "Metric", placeholder: "5xx_rate" }, { key: "threshold", label: "Threshold", type: "number", default: 1 }, { key: "op", label: "Operator", type: "select", options: [">", ">=", "<", "<="], default: ">=" }, { key: "cooldown_s", label: "Cooldown seconds", type: "number", default: 300 }], rowActions: [{ label: "Silence", endpoint: "/projects/:projectId/alerts/:id/silence", when: { key: "silenced", equals: false }, success: "Alert silenced" }, { label: "Unsilence", endpoint: "/projects/:projectId/alerts/:id/unsilence", when: { key: "silenced", equals: true }, success: "Alert unsilenced" }, { label: "Delete", method: "DELETE", endpoint: "/projects/:projectId/alerts/:id", danger: true, confirm: "Delete {name}?", success: "Alert deleted" }] }) },
  { path: "/projects/:projectId/drains", name: "project-drains", component: ResourceManager, meta: resource("Log drains", "External log drain configuration.", "/projects/:projectId/drains", { createLabel: "Add drain", createFields: [{ key: "name", label: "Name", placeholder: "Operations sink", required: false }, { key: "endpoint", label: "Endpoint", placeholder: "https://logs.example.com/ingest" }, { key: "kind", label: "Kind", placeholder: "webhook", required: false }], rowActions: [{ label: "Test", endpoint: "/projects/:projectId/drains/:id/test", success: "Drain test sent" }, { label: "Delete", method: "DELETE", endpoint: "/projects/:projectId/drains/:id", danger: true, confirm: "Delete {name}?", success: "Drain deleted" }] }) },
  { path: "/projects/:projectId/redirects", name: "project-redirects", component: ResourceManager, meta: resource("Redirects", "HTTP redirect rules for this project.", "/projects/:projectId/redirects", { createLabel: "Add redirect", createFields: [{ key: "source", label: "Source path", placeholder: "/old-path" }, { key: "target", label: "Target URL", placeholder: "/new-path" }, { key: "permanent", label: "Permanent (308)", type: "checkbox", default: true }], rowActions: [{ label: "Delete", method: "DELETE", endpoint: "/projects/:projectId/redirects/:id", danger: true, confirm: "Delete redirect {name}?", success: "Redirect deleted" }] }) },
  { path: "/projects/:projectId/firewall", name: "project-firewall", component: ResourceManager, meta: resource("Firewall", "Firewall rules and current policy state.", "/projects/:projectId/firewall/rules", { createLabel: "Add firewall rule", createFields: [{ key: "direction", label: "Direction", type: "select", options: ["inbound", "outbound"], default: "inbound" }, { key: "action", label: "Action", type: "select", options: ["allow", "deny"], default: "deny" }, { key: "proto", label: "Protocol", type: "select", options: ["tcp", "udp", "all"], default: "tcp" }, { key: "ports", label: "Ports", placeholder: "80,443", required: false }, { key: "source", label: "Source CIDR", placeholder: "0.0.0.0/0", required: false }, { key: "priority", label: "Priority", type: "number", default: 100 }], rowActions: [{ label: "Delete", method: "DELETE", endpoint: "/projects/:projectId/firewall/rules/:id", danger: true, confirm: "Delete firewall rule {name}?", success: "Firewall rule deleted" }] }) },
  { path: "/projects/:projectId/members", name: "project-members", component: ResourceManager, meta: resource("Project members", "Project-scoped memberships and roles.", "/projects/:projectId/members") },
  { path: "/projects/:projectId/volumes", name: "project-volumes", component: ResourceManager, meta: resource("Volumes", "Persistent volume records and host backing state.", "/projects/:projectId/volumes", { createLabel: "Create volume", createFields: [{ key: "name", label: "Name", placeholder: "data" }, { key: "size_mib", label: "Size (MiB)", type: "number", default: 1024 }, { key: "mount_path", label: "Mount path", placeholder: "/data", required: false }], rowActions: [{ label: "Delete", method: "DELETE", endpoint: "/projects/:projectId/volumes/:id", danger: true, confirm: "Delete volume {name}?", success: "Volume deleted" }] }) },
  { path: "/projects/:projectId/analytics", name: "project-analytics", component: ResourceManager, meta: resource("Project analytics", "Durable analytics data for this project.", "/projects/:projectId/analytics/usage") },
  { path: "/projects/:projectId/metrics", name: "project-metrics", component: ResourceManager, meta: resource("Project metrics", "Replica metric samples and health signals.", "/projects/:projectId/metrics") },
  { path: "/projects/:projectId/events", name: "project-events", component: ResourceManager, meta: resource("Project events", "Recorded deployment and health events.", "/projects/:projectId/events") },
  { path: "/projects/:projectId/pool", name: "project-pool", component: ResourceManager, meta: resource("Replica pool", "Desired/current replica pool state.", "/projects/:projectId/pool") },
  { path: "/projects/:projectId/logs", name: "project-logs", component: LiveLogStream, meta: { streamTitle: "Application logs", streamEndpoint: "/projects/:projectId/logs/stream", back: "/projects/:projectId" } },
  { path: "/projects/:projectId/settings/build", name: "project-build-settings", component: ResourceManager, meta: resource("Build settings", "Build command, machine, and deployment settings.", "/projects/:projectId/settings/build") },
  { path: "/projects/:projectId/settings/git", name: "project-git-settings", component: ResourceManager, meta: resource("Git settings", "Repository, branch, sync, and LFS configuration.", "/projects/:projectId/settings/git") },
  { path: "/projects/:projectId/settings/functions", name: "project-functions", component: ResourceManager, meta: resource("Functions", "Serverless function settings for the project.", "/projects/:projectId/settings/functions") },
  { path: "/projects/:projectId/settings/security", name: "project-security", component: ResourceManager, meta: resource("Security settings", "Project security policy and controls.", "/projects/:projectId/settings/security") },
  { path: "/projects/:projectId/settings/networking", name: "project-networking-settings", component: ResourceManager, meta: resource("Networking settings", "Networking and TAP-related project configuration.", "/projects/:projectId/settings/networking") },
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
  { path: "/access", name: "access", component: Teams },
  { path: "/access/organizations", name: "access-organizations", component: ResourceManager, meta: resource("Organizations", "Organizations available to the current user.", "/orgs", { back: false }) },
  { path: "/access/members", name: "access-members", component: ResourceManager, meta: resource("Organization members", "Organization membership and role assignments.", "/orgs/members", { back: false }) },
  { path: "/access/users", name: "access-users", component: ResourceManager, meta: resource("Users", "Persisted platform users and global roles.", "/users", { back: false }) },
  { path: "/access/roles", name: "access-roles", component: ResourceManager, meta: resource("Roles", "Database-backed roles available for assignment.", "/roles", { back: false }) },
  { path: "/access/permissions", name: "access-permissions", component: ResourceManager, meta: resource("Permissions", "Fine-grained permission catalog from PostgreSQL.", "/permissions", { back: false }) },
  { path: "/access/audit", name: "access-audit", component: ResourceManager, meta: resource("Organization audit log", "Persisted organization security and membership events.", "/orgs/audit", { back: false }) },
  { path: "/access/api-keys", name: "access-api-keys", component: ResourceManager, meta: resource("API keys", "Scoped keys belonging to the current user.", "/users/me/api-keys", { back: false }) },
  { path: "/login", name: "login", component: Login },
];

const router = createRouter({ history: createWebHashHistory(), routes });

router.beforeEach((to) => {
  if (to.name !== "login" && !getToken()) return { name: "login" };
});

export default router;
