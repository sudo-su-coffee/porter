# Porter Vue view matrix

Porter keeps the Whatomate-inspired workspace structure while replacing chat-specific surfaces with PaaS operations. Each view below is a dedicated Vue file; list/detail data comes from the backend endpoint shown in route metadata.

| Area | Dedicated views |
|---|---|
| Core | DeploymentsList, ProjectDetail, DeploymentDetail, Builds, Images, Domains, Servers |
| Runtime | VmDetail, ReplicaStream, LiveLogStream, ReplicaHealth, ReplicaMetrics, ReplicaTraffic, ReplicaSSH |
| Project operations | ProjectDeployments, ProjectEnvironments, ProjectDomains, ProjectSecretsView, ProjectVolumes, ProjectNetworks, ProjectHooks, ProjectCrons, ProjectCronHistory |
| Project observability | ProjectAlerts, ProjectDrains, ProjectRedirects, ProjectFirewall, ProjectAnalytics, ProjectMetrics, ProjectEvents, ProjectPool |
| Project settings/access | ProjectMembers, ProjectBuildSettings, ProjectGitSettings, ProjectFunctionsSettings, ProjectSecuritySettings, ProjectNetworkingSettings |
| Platform operations | Analytics, Traffic, Logs, DaemonLogs, HostOverview, HostPrerequisites, HostRuntime, HostPorts |
| Organization/RBAC | Organizations, OrgMembers, Users, Roles, Permissions, AuditLog, ApiKeys, Teams |

All dedicated views preserve real loading, error, and empty states. No view fabricates project, user, role, log, image, or runtime records.
