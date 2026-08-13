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
import ProjectDeployments from "./views/ProjectDeployments.vue";
import ProjectServices from "./views/ProjectServices.vue";
import ProjectEnvironments from "./views/ProjectEnvironments.vue";
import ProjectDomains from "./views/ProjectDomains.vue";
import ProjectSecretsView from "./views/ProjectSecretsView.vue";
import ProjectVolumes from "./views/ProjectVolumes.vue";
import ProjectNetworks from "./views/ProjectNetworks.vue";
import ProjectHooks from "./views/ProjectHooks.vue";
import ProjectCrons from "./views/ProjectCrons.vue";
import ProjectCronHistory from "./views/ProjectCronHistory.vue";
import ProjectAlerts from "./views/ProjectAlerts.vue";
import ProjectDrains from "./views/ProjectDrains.vue";
import ProjectRedirects from "./views/ProjectRedirects.vue";
import ProjectFirewall from "./views/ProjectFirewall.vue";
import ProjectAnalytics from "./views/ProjectAnalytics.vue";
import ProjectMetrics from "./views/ProjectMetrics.vue";
import ProjectEvents from "./views/ProjectEvents.vue";
import ProjectPool from "./views/ProjectPool.vue";
import ProjectMembers from "./views/ProjectMembers.vue";
import ProjectBuildSettings from "./views/ProjectBuildSettings.vue";
import ProjectGitSettings from "./views/ProjectGitSettings.vue";
import ProjectFunctionsSettings from "./views/ProjectFunctionsSettings.vue";
import ProjectSecuritySettings from "./views/ProjectSecuritySettings.vue";
import ProjectNetworkingSettings from "./views/ProjectNetworkingSettings.vue";
import ReplicaLogs from "./views/ReplicaLogs.vue";
import ReplicaSSH from "./views/ReplicaSSH.vue";
import ReplicaHealth from "./views/ReplicaHealth.vue";
import ReplicaMetrics from "./views/ReplicaMetrics.vue";
import ReplicaTraffic from "./views/ReplicaTraffic.vue";
import Replicas from "./views/Replicas.vue";
import HostOverview from "./views/HostOverview.vue";
import HostPrerequisites from "./views/HostPrerequisites.vue";
import HostRuntime from "./views/HostRuntime.vue";
import HostPorts from "./views/HostPorts.vue";
import DaemonLogs from "./views/DaemonLogs.vue";
import Organizations from "./views/Organizations.vue";
import OrgMembers from "./views/OrgMembers.vue";
import Users from "./views/Users.vue";
import Roles from "./views/Roles.vue";
import Permissions from "./views/Permissions.vue";
import AuditLog from "./views/AuditLog.vue";
import ApiKeys from "./views/ApiKeys.vue";

const routes = [
  { path: "/", name: "list", component: DeploymentsList },
  { path: "/analytics", name: "analytics", component: Analytics },
  { path: "/projects/:id", name: "project", component: ProjectDetail, props: true },
  { path: "/projects/:projectId/deployments/:deploymentId", name: "deployment", component: DeploymentDetail },
  { path: "/projects/:projectId/builds/:buildId/logs", name: "build-logs", component: LiveLogStream, meta: { streamTitle: "Build logs", streamEndpoint: "/projects/:projectId/builds/:buildId/logs/stream", back: "/projects/:projectId/builds" } },
  { path: "/projects/:projectId/builds", name: "project-builds", component: Builds },
  { path: "/projects/:projectId/deployments", name: "project-deployments", component: ProjectDeployments, meta: { resource: { title: "Deployments", description: "Deployment history and rollout state from the control plane.", endpoint: "/projects/:projectId/deployments", back: true } } },
  { path: "/projects/:projectId/services", name: "project-services", component: ProjectServices, meta: { resource: { title: "Services", description: "Service and replica topology for this project.", endpoint: "/projects/:projectId/services", back: true } } },
  { path: "/projects/:projectId/domains", name: "project-domains", component: ProjectDomains, meta: { resource: { title: "Domains", description: "Project domains and verification state.", endpoint: "/projects/:projectId/domains", back: true } } },
  { path: "/projects/:projectId/secrets", name: "project-secrets", component: ProjectSecretsView, meta: { resource: { title: "Secrets", description: "Opaque project secret metadata.", endpoint: "/projects/:projectId/secrets", back: true } } },
  { path: "/projects/:projectId/networks", name: "project-networks", component: ProjectNetworks, meta: { resource: { title: "Networks", description: "Project network allocations and direct TAP boundary metadata.", endpoint: "/projects/:projectId/networks", back: true } } },
  { path: "/projects/:projectId/environments", name: "project-environments", component: ProjectEnvironments, meta: { resource: { title: "Environments", description: "Environment records and deployment branches.", endpoint: "/projects/:projectId/environments", back: true } } },
  { path: "/projects/:projectId/hooks", name: "project-hooks", component: ProjectHooks, meta: { resource: { title: "Webhooks", description: "Project hooks and delivery configuration.", endpoint: "/projects/:projectId/hooks", back: true } } },
  { path: "/projects/:projectId/crons", name: "project-crons", component: ProjectCrons, meta: { resource: { title: "Cron jobs", description: "Scheduled project jobs and execution state.", endpoint: "/projects/:projectId/crons", back: true } } },
  { path: "/projects/:projectId/crons/history", name: "project-cron-history", component: ProjectCronHistory, meta: { resource: { title: "Cron history", description: "Recorded cron execution history.", endpoint: "/projects/:projectId/crons/history", back: true } } },
  { path: "/projects/:projectId/alerts", name: "project-alerts", component: ProjectAlerts, meta: { resource: { title: "Alerts", description: "Project health and operational alerts.", endpoint: "/projects/:projectId/alerts", back: true } } },
  { path: "/projects/:projectId/drains", name: "project-drains", component: ProjectDrains, meta: { resource: { title: "Log drains", description: "External log drain configuration.", endpoint: "/projects/:projectId/drains", back: true } } },
  { path: "/projects/:projectId/redirects", name: "project-redirects", component: ProjectRedirects, meta: { resource: { title: "Redirects", description: "HTTP redirect rules for this project.", endpoint: "/projects/:projectId/redirects", back: true } } },
  { path: "/projects/:projectId/firewall", name: "project-firewall", component: ProjectFirewall, meta: { resource: { title: "Firewall", description: "Firewall rules and current policy state.", endpoint: "/projects/:projectId/firewall/rules", back: true } } },
  { path: "/projects/:projectId/members", name: "project-members", component: ProjectMembers, meta: { resource: { title: "Project members", description: "Project-scoped memberships and roles.", endpoint: "/projects/:projectId/members", back: true } } },
  { path: "/projects/:projectId/volumes", name: "project-volumes", component: ProjectVolumes, meta: { resource: { title: "Volumes", description: "Persistent volume records and host backing state.", endpoint: "/projects/:projectId/volumes", back: true } } },
  { path: "/projects/:projectId/analytics", name: "project-analytics", component: ProjectAnalytics, meta: { resource: { title: "Project analytics", description: "Durable analytics data for this project.", endpoint: "/projects/:projectId/analytics/usage", back: true } } },
  { path: "/projects/:projectId/metrics", name: "project-metrics", component: ProjectMetrics, meta: { resource: { title: "Project metrics", description: "Replica metric samples and health signals.", endpoint: "/projects/:projectId/metrics", back: true } } },
  { path: "/projects/:projectId/events", name: "project-events", component: ProjectEvents, meta: { resource: { title: "Project events", description: "Recorded deployment and health events.", endpoint: "/projects/:projectId/events", back: true } } },
  { path: "/projects/:projectId/pool", name: "project-pool", component: ProjectPool, meta: { resource: { title: "Replica pool", description: "Desired/current replica pool state.", endpoint: "/projects/:projectId/pool", back: true } } },
  { path: "/projects/:projectId/logs", name: "project-logs", component: LiveLogStream, meta: { streamTitle: "Application logs", streamEndpoint: "/projects/:projectId/logs/stream", back: "/projects/:projectId" } },
  { path: "/projects/:projectId/settings/build", name: "project-build-settings", component: ProjectBuildSettings, meta: { resource: { title: "Build settings", description: "Build command, machine, and deployment settings.", endpoint: "/projects/:projectId/settings/build", back: true } } },
  { path: "/projects/:projectId/settings/git", name: "project-git-settings", component: ProjectGitSettings, meta: { resource: { title: "Git settings", description: "Repository, branch, sync, and LFS configuration.", endpoint: "/projects/:projectId/settings/git", back: true } } },
  { path: "/projects/:projectId/settings/functions", name: "project-functions", component: ProjectFunctionsSettings, meta: { resource: { title: "Functions", description: "Serverless function settings for the project.", endpoint: "/projects/:projectId/settings/functions", back: true } } },
  { path: "/projects/:projectId/settings/security", name: "project-security", component: ProjectSecuritySettings, meta: { resource: { title: "Security settings", description: "Project security policy and controls.", endpoint: "/projects/:projectId/settings/security", back: true } } },
  { path: "/projects/:projectId/settings/networking", name: "project-networking-settings", component: ProjectNetworkingSettings, meta: { resource: { title: "Networking settings", description: "Networking and TAP-related project configuration.", endpoint: "/projects/:projectId/settings/networking", back: true } } },
  { path: "/vms/:id", name: "vm", component: VmDetail, props: true },
  { path: "/vms/:id/logs", name: "vm-logs", component: ReplicaLogs, meta: { streamTitle: "Replica logs", streamEndpoint: "/vms/:id/logs/stream", back: "/vms/:id" } },
  { path: "/vms/:id/health", name: "vm-health", component: ReplicaHealth, meta: { stream: "health" } },
  { path: "/vms/:id/metrics", name: "vm-metrics", component: ReplicaMetrics, meta: { stream: "metrics" } },
  { path: "/vms/:id/traffic", name: "vm-traffic", component: ReplicaTraffic, meta: { stream: "traffic" } },
  { path: "/vms/:id/ssh", name: "vm-ssh", component: ReplicaSSH, meta: { resource: { title: "SSH information", description: "Replica SSH metadata and guest-channel readiness.", endpoint: "/vms/:id/ssh-info", back: true } } },
  { path: "/traffic", name: "traffic", component: Traffic },
  { path: "/domains", name: "domains", component: Domains },
  { path: "/teams", name: "teams", component: Teams },
  { path: "/settings", name: "settings", component: Settings },
  { path: "/images", name: "images", component: Images },
  { path: "/servers", name: "servers", component: Servers },
  { path: "/logs", name: "logs", component: Logs },
  { path: "/replicas", name: "replicas", component: Replicas, meta: { resource: { title: "All replicas", description: "Global replica inventory across projects.", endpoint: "/replicas" } } },
  { path: "/host/overview", name: "host-overview", component: HostOverview, meta: { resource: { title: "Host overview", description: "Host capacity and runtime state.", endpoint: "/host/overview" } } },
  { path: "/host/prerequisites", name: "host-prerequisites", component: HostPrerequisites, meta: { resource: { title: "Host readiness", description: "KVM, TAP, Firecracker, and artifact readiness checks.", endpoint: "/host/prerequisites" } } },
  { path: "/host/runtime", name: "host-runtime", component: HostRuntime, meta: { resource: { title: "Runtime configuration", description: "Direct Firecracker runtime configuration and state paths.", endpoint: "/host/runtime" } } },
  { path: "/host/ports", name: "host-ports", component: HostPorts, meta: { resource: { title: "Host ports", description: "Host-to-guest port forwarding state.", endpoint: "/host/ports" } } },
  { path: "/daemon-logs", name: "daemon-logs", component: DaemonLogs, meta: { resource: { title: "Daemon logs", description: "Control-plane operational log history.", endpoint: "/logs" } } },
  { path: "/access", name: "access", component: Teams },
  { path: "/access/organizations", name: "access-organizations", component: Organizations, meta: { resource: { title: "Organizations", description: "Organizations available to the current user.", endpoint: "/orgs" } } },
  { path: "/access/members", name: "access-members", component: OrgMembers, meta: { resource: { title: "Organization members", description: "Organization membership and role assignments.", endpoint: "/orgs/members" } } },
  { path: "/access/users", name: "access-users", component: Users, meta: { resource: { title: "Users", description: "Persisted platform users and global roles.", endpoint: "/users" } } },
  { path: "/access/roles", name: "access-roles", component: Roles, meta: { resource: { title: "Roles", description: "Database-backed roles available for assignment.", endpoint: "/roles" } } },
  { path: "/access/permissions", name: "access-permissions", component: Permissions, meta: { resource: { title: "Permissions", description: "Fine-grained permission catalog from PostgreSQL.", endpoint: "/permissions" } } },
  { path: "/access/audit", name: "access-audit", component: AuditLog, meta: { resource: { title: "Organization audit log", description: "Persisted organization security and membership events.", endpoint: "/orgs/audit" } } },
  { path: "/access/api-keys", name: "access-api-keys", component: ApiKeys, meta: { resource: { title: "API keys", description: "Scoped keys belonging to the current user.", endpoint: "/users/me/api-keys" } } },
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
