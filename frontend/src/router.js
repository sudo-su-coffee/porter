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
import ResourceView from "./views/ResourceView.vue";
import ReplicaStream from "./views/ReplicaStream.vue";
import LiveLogStream from "./views/LiveLogStream.vue";
import Builds from "./views/Builds.vue";
import ResourceManager from "./views/ResourceManager.vue";

const routes = [
  { path: "/", name: "list", component: DeploymentsList },
  { path: "/analytics", name: "analytics", component: Analytics },
  { path: "/projects/:id", name: "project", component: ProjectDetail, props: true },
  { path: "/projects/:projectId/deployments/:deploymentId", name: "deployment", component: DeploymentDetail },
  { path: "/projects/:projectId/builds/:buildId/logs", name: "build-logs", component: LiveLogStream, meta: { streamTitle: "Build logs", streamEndpoint: "/projects/:projectId/builds/:buildId/logs/stream", back: "/projects/:projectId/builds" } },
  { path: "/projects/:projectId/builds", name: "project-builds", component: Builds },
  { path: "/projects/:projectId/deployments", name: "project-deployments", component: ResourceView, meta: { resource: { title: "Deployments", description: "Deployment history and rollout state from the control plane.", endpoint: "/projects/:projectId/deployments", back: true } } },
  { path: "/projects/:projectId/services", name: "project-services", component: ResourceView, meta: { resource: { title: "Services", description: "Service and replica topology for this project.", endpoint: "/projects/:projectId/services", back: true } } },
  { path: "/projects/:projectId/networks", name: "project-networks", component: ResourceView, meta: { resource: { title: "Networks", description: "Project network allocations and direct TAP boundary metadata.", endpoint: "/projects/:projectId/networks", back: true } } },
  { path: "/projects/:projectId/environments", name: "project-environments", component: ResourceManager, meta: { resource: { title: "Environments", description: "Environment records and deployment branches.", endpoint: "/projects/:projectId/environments", back: true, createLabel: "Add environment", createFields: [{ key: "name", label: "Name", placeholder: "preview" }, { key: "branch", label: "Branch", placeholder: "main", required: false }, { key: "url", label: "URL", placeholder: "https://preview.example.com", required: false }, { key: "env_domain", label: "Environment domain", placeholder: "preview.example.com", required: false }], rowActions: [{ label: "Update", method: "PATCH", endpoint: "/projects/:projectId/environments/:id", body: (row) => ({ branch: row.branch || "", url: row.url || "", env_domain: row.env_domain || "" }), success: "Environment updated" }, { label: "Delete", method: "DELETE", endpoint: "/projects/:projectId/environments/:id", danger: true, confirm: "Delete {name}?", success: "Environment deleted" }] } } },
  { path: "/projects/:projectId/hooks", name: "project-hooks", component: ResourceManager, meta: { resource: { title: "Webhooks", description: "Project hooks and delivery configuration.", endpoint: "/projects/:projectId/hooks", back: true, createLabel: "Add webhook", createFields: [{ key: "name", label: "Name", placeholder: "Deploy hook", required: false }, { key: "url", label: "Endpoint URL", placeholder: "https://example.com/porter-hook" }, { key: "events", label: "Events", placeholder: "deployment.ready,replica.health", type: "csv", required: false }], rowActions: [{ label: "Trigger", endpoint: "/projects/:projectId/hooks/:id/trigger", success: "Webhook triggered" }, { label: "Delete", method: "DELETE", endpoint: "/projects/:projectId/hooks/:id", danger: true, confirm: "Delete {name}?", success: "Webhook deleted" }] } } },
  { path: "/projects/:projectId/crons", name: "project-crons", component: ResourceManager, meta: { resource: { title: "Cron jobs", description: "Scheduled project jobs and execution state.", endpoint: "/projects/:projectId/crons", back: true, createLabel: "Add cron job", createFields: [{ key: "name", label: "Name", placeholder: "Nightly task" }, { key: "schedule", label: "Schedule", placeholder: "0 * * * *" }, { key: "job_image", label: "Direct image reference", placeholder: "base://default" }], rowActions: [{ label: "Run now", endpoint: "/projects/:projectId/crons/:id/run", confirm: "Run {name} now?", success: "Cron job queued" }, { label: "Delete", method: "DELETE", endpoint: "/projects/:projectId/crons/:id", danger: true, confirm: "Delete {name}?", success: "Cron deleted" }] } } },
  { path: "/projects/:projectId/crons/history", name: "project-cron-history", component: ResourceView, meta: { resource: { title: "Cron history", description: "Recorded cron execution history.", endpoint: "/projects/:projectId/crons/history", back: true } } },
  { path: "/projects/:projectId/alerts", name: "project-alerts", component: ResourceManager, meta: { resource: { title: "Alerts", description: "Project health and operational alerts.", endpoint: "/projects/:projectId/alerts", back: true, createLabel: "Create alert", createFields: [{ key: "name", label: "Name", placeholder: "High error rate" }, { key: "metric", label: "Metric", placeholder: "5xx_rate" }, { key: "threshold", label: "Threshold", type: "number", default: 1 }, { key: "op", label: "Operator", type: "select", options: [">", ">=", "<", "<="], default: ">=" }, { key: "cooldown_s", label: "Cooldown seconds", type: "number", default: 300 }], rowActions: [{ label: "Silence", endpoint: "/projects/:projectId/alerts/:id/silence", when: { key: "silenced", equals: false }, success: "Alert silenced" }, { label: "Unsilence", endpoint: "/projects/:projectId/alerts/:id/unsilence", when: { key: "silenced", equals: true }, success: "Alert unsilenced" }, { label: "Delete", method: "DELETE", endpoint: "/projects/:projectId/alerts/:id", danger: true, confirm: "Delete {name}?", success: "Alert deleted" }] } } },
  { path: "/projects/:projectId/drains", name: "project-drains", component: ResourceManager, meta: { resource: { title: "Log drains", description: "External log drain configuration.", endpoint: "/projects/:projectId/drains", back: true, createLabel: "Add drain", createFields: [{ key: "name", label: "Name", placeholder: "Operations sink", required: false }, { key: "endpoint", label: "Endpoint", placeholder: "https://logs.example.com/ingest" }, { key: "kind", label: "Kind", placeholder: "webhook", required: false }], rowActions: [{ label: "Test", endpoint: "/projects/:projectId/drains/:id/test", success: "Drain test sent" }, { label: "Delete", method: "DELETE", endpoint: "/projects/:projectId/drains/:id", danger: true, confirm: "Delete {name}?", success: "Drain deleted" }] } } },
  { path: "/projects/:projectId/redirects", name: "project-redirects", component: ResourceManager, meta: { resource: { title: "Redirects", description: "HTTP redirect rules for this project.", endpoint: "/projects/:projectId/redirects", back: true, createLabel: "Add redirect", createFields: [{ key: "source", label: "Source path", placeholder: "/old-path" }, { key: "target", label: "Target URL", placeholder: "/new-path" }, { key: "permanent", label: "Permanent (308)", type: "checkbox", default: true }], rowActions: [{ label: "Delete", method: "DELETE", endpoint: "/projects/:projectId/redirects/:id", danger: true, confirm: "Delete redirect {name}?", success: "Redirect deleted" }] } } },
  { path: "/projects/:projectId/firewall", name: "project-firewall", component: ResourceManager, meta: { resource: { title: "Firewall", description: "Firewall rules and current policy state.", endpoint: "/projects/:projectId/firewall/rules", back: true, createLabel: "Add firewall rule", createFields: [{ key: "direction", label: "Direction", type: "select", options: ["inbound", "outbound"], default: "inbound" }, { key: "action", label: "Action", type: "select", options: ["allow", "deny"], default: "deny" }, { key: "proto", label: "Protocol", type: "select", options: ["tcp", "udp", "all"], default: "tcp" }, { key: "ports", label: "Ports", placeholder: "80,443", required: false }, { key: "source", label: "Source CIDR", placeholder: "0.0.0.0/0", required: false }, { key: "priority", label: "Priority", type: "number", default: 100 }], rowActions: [{ label: "Delete", method: "DELETE", endpoint: "/projects/:projectId/firewall/rules/:id", danger: true, confirm: "Delete firewall rule {name}?", success: "Firewall rule deleted" }] } } },
  { path: "/projects/:projectId/members", name: "project-members", component: ResourceView, meta: { resource: { title: "Project members", description: "Project-scoped memberships and roles.", endpoint: "/projects/:projectId/members", back: true } } },
  { path: "/projects/:projectId/volumes", name: "project-volumes", component: ResourceManager, meta: { resource: { title: "Volumes", description: "Persistent volume records and host backing state.", endpoint: "/projects/:projectId/volumes", back: true, createLabel: "Create volume", createFields: [{ key: "name", label: "Name", placeholder: "data" }, { key: "size_mib", label: "Size (MiB)", type: "number", default: 1024 }, { key: "mount_path", label: "Mount path", placeholder: "/data", required: false }], rowActions: [{ label: "Delete", method: "DELETE", endpoint: "/projects/:projectId/volumes/:id", danger: true, confirm: "Delete volume {name}?", success: "Volume deleted" }] } } },
  { path: "/projects/:projectId/analytics", name: "project-analytics", component: ResourceView, meta: { resource: { title: "Project analytics", description: "Durable analytics data for this project.", endpoint: "/projects/:projectId/analytics/usage", back: true } } },
  { path: "/projects/:projectId/metrics", name: "project-metrics", component: ResourceView, meta: { resource: { title: "Project metrics", description: "Replica metric samples and health signals.", endpoint: "/projects/:projectId/metrics", back: true } } },
  { path: "/projects/:projectId/events", name: "project-events", component: ResourceView, meta: { resource: { title: "Project events", description: "Recorded deployment and health events.", endpoint: "/projects/:projectId/events", back: true } } },
  { path: "/projects/:projectId/pool", name: "project-pool", component: ResourceView, meta: { resource: { title: "Replica pool", description: "Desired/current replica pool state.", endpoint: "/projects/:projectId/pool", back: true } } },
  { path: "/projects/:projectId/logs", name: "project-logs", component: LiveLogStream, meta: { streamTitle: "Application logs", streamEndpoint: "/projects/:projectId/logs/stream", back: "/projects/:projectId" } },
  { path: "/projects/:projectId/settings/build", name: "project-build-settings", component: ResourceView, meta: { resource: { title: "Build settings", description: "Build command, machine, and deployment settings.", endpoint: "/projects/:projectId/settings/build", back: true } } },
  { path: "/projects/:projectId/settings/git", name: "project-git-settings", component: ResourceView, meta: { resource: { title: "Git settings", description: "Repository, branch, sync, and LFS configuration.", endpoint: "/projects/:projectId/settings/git", back: true } } },
  { path: "/projects/:projectId/settings/functions", name: "project-functions", component: ResourceView, meta: { resource: { title: "Functions", description: "Serverless function settings for the project.", endpoint: "/projects/:projectId/settings/functions", back: true } } },
  { path: "/projects/:projectId/settings/security", name: "project-security", component: ResourceView, meta: { resource: { title: "Security settings", description: "Project security policy and controls.", endpoint: "/projects/:projectId/settings/security", back: true } } },
  { path: "/projects/:projectId/settings/networking", name: "project-networking-settings", component: ResourceView, meta: { resource: { title: "Networking settings", description: "Networking and TAP-related project configuration.", endpoint: "/projects/:projectId/settings/networking", back: true } } },
  { path: "/vms/:id", name: "vm", component: VmDetail, props: true },
  { path: "/vms/:id/logs", name: "vm-logs", component: LiveLogStream, meta: { streamTitle: "Replica logs", streamEndpoint: "/vms/:id/logs/stream", back: "/vms/:id" } },
  { path: "/vms/:id/health", name: "vm-health", component: ReplicaStream, meta: { stream: "health" } },
  { path: "/vms/:id/metrics", name: "vm-metrics", component: ReplicaStream, meta: { stream: "metrics" } },
  { path: "/vms/:id/traffic", name: "vm-traffic", component: ReplicaStream, meta: { stream: "traffic" } },
  { path: "/vms/:id/ssh", name: "vm-ssh", component: ResourceView, meta: { resource: { title: "SSH information", description: "Replica SSH metadata and guest-channel readiness.", endpoint: "/vms/:id/ssh-info", back: true } } },
  { path: "/traffic", name: "traffic", component: Traffic },
  { path: "/domains", name: "domains", component: Domains },
  { path: "/teams", name: "teams", component: Teams },
  { path: "/settings", name: "settings", component: Settings },
  { path: "/images", name: "images", component: Images },
  { path: "/servers", name: "servers", component: Servers },
  { path: "/logs", name: "logs", component: Logs },
  { path: "/replicas", name: "replicas", component: ResourceView, meta: { resource: { title: "All replicas", description: "Global replica inventory across projects.", endpoint: "/replicas" } } },
  { path: "/host/overview", name: "host-overview", component: ResourceView, meta: { resource: { title: "Host overview", description: "Host capacity and runtime state.", endpoint: "/host/overview" } } },
  { path: "/host/prerequisites", name: "host-prerequisites", component: ResourceView, meta: { resource: { title: "Host readiness", description: "KVM, TAP, Firecracker, and artifact readiness checks.", endpoint: "/host/prerequisites" } } },
  { path: "/host/runtime", name: "host-runtime", component: ResourceView, meta: { resource: { title: "Runtime configuration", description: "Direct Firecracker runtime configuration and state paths.", endpoint: "/host/runtime" } } },
  { path: "/host/ports", name: "host-ports", component: ResourceView, meta: { resource: { title: "Host ports", description: "Host-to-guest port forwarding state.", endpoint: "/host/ports" } } },
  { path: "/daemon-logs", name: "daemon-logs", component: ResourceView, meta: { resource: { title: "Daemon logs", description: "Control-plane operational log history.", endpoint: "/logs" } } },
  { path: "/access", name: "access", component: Teams },
  { path: "/access/organizations", name: "access-organizations", component: ResourceView, meta: { resource: { title: "Organizations", description: "Organizations available to the current user.", endpoint: "/orgs" } } },
  { path: "/access/members", name: "access-members", component: ResourceView, meta: { resource: { title: "Organization members", description: "Organization membership and role assignments.", endpoint: "/orgs/members" } } },
  { path: "/access/users", name: "access-users", component: ResourceView, meta: { resource: { title: "Users", description: "Persisted platform users and global roles.", endpoint: "/users" } } },
  { path: "/access/roles", name: "access-roles", component: ResourceView, meta: { resource: { title: "Roles", description: "Database-backed roles available for assignment.", endpoint: "/roles" } } },
  { path: "/access/permissions", name: "access-permissions", component: ResourceView, meta: { resource: { title: "Permissions", description: "Fine-grained permission catalog from PostgreSQL.", endpoint: "/permissions" } } },
  { path: "/access/audit", name: "access-audit", component: ResourceView, meta: { resource: { title: "Organization audit log", description: "Persisted organization security and membership events.", endpoint: "/orgs/audit" } } },
  { path: "/access/api-keys", name: "access-api-keys", component: ResourceView, meta: { resource: { title: "API keys", description: "Scoped keys belonging to the current user.", endpoint: "/users/me/api-keys" } } },
  { path: "/login", name: "login", component: Login },
];

const router = createRouter({
  // Hash history keeps this a static-file-friendly single binary with
  // no server-side route handling required beyond serving index.html.
  history: createWebHashHistory(),
  routes,
});

router.beforeEach((to) => {
  if (to.name !== "login" && !getToken()) {
    return { name: "login" };
  }
});

export default router;
