# Coolify feature research for Porter roadmap

Research date: 2026-08-17.

## Sources

1. Coolify homepage: https://coolify.io/
2. Coolify applications: https://coolify.io/docs/applications
3. Coolify GitHub preview deployments: https://coolify.io/docs/applications/ci-cd/github/preview-deploy
4. Coolify rolling updates: https://coolify.io/docs/knowledge-base/rolling-updates

## Findings

Coolify positions itself as a self-hostable alternative to Vercel, Heroku, Netlify, and Railway. Its official homepage highlights application, database, and service deployment; Git integration; free Let's Encrypt SSL; automatic S3-compatible database backups; webhooks; API automation; real-time terminal; team collaboration and permissions; pull-request deployments; server automation; monitoring; and notifications.

Coolify's application documentation describes deployment from public/private Git repositories, Dockerfiles, Docker Compose, and pre-built Docker images. It supports build packs, environment variables, persistent storage, health checks, rollbacks, resource limits, preview deployments, and custom commands/ports. Coolify can use a dedicated build server for resource-intensive builds.

Coolify's preview deployment documentation describes unique preview URLs, automatic deployment for pull requests, cleanup after a pull request is merged or closed, scoped deployment permissions, separate preview secrets, and automated pull-request comments. Preview DNS requires a wildcard record pointing to the server.

Coolify's rolling-update documentation describes starting a new container while the old one continues serving, waiting for health checks to pass, then stopping the old container. The same pattern maps directly to Porter as a new Firecracker replica pool becoming healthy before traffic moves and the old VM pool is drained.

## Firecracker-native translation

Coolify container -> Porter Firecracker microVM.
Docker/Compose image -> BuildKit OCI result plus a validated Porter guest base and ext4 rootfs.
Container health check -> guest-agent readiness plus HTTP/TCP health checks.
Rolling container update -> parallel Firecracker deployment pool, weighted routing, readiness gate, and graceful VM drain.
Preview container -> isolated tagged VM deployment with scoped preview environment variables and a preview domain.
Docker volume -> named Firecracker data volume attached to selected replicas.
Docker host/server -> single-host Firecracker scheduler initially, later a multi-host scheduler.
Traefik proxy -> Go gateway with domain lookup, TLS, cookie-stable experiment routing, and weighted deployment selection.
Coolify automatic cleanup -> deployment tag/preview lifecycle controller that stops VMs and releases artifacts when a tag or preview is removed.

## Additional findings

The official Coolify backup documentation describes scheduled cron-based database backups for PostgreSQL and other databases, custom-format PostgreSQL restore, and S3-compatible backup storage.

The official monitoring documentation describes monitoring disk usage, stopped/restarted containers, and backup status, with automatic cleanup and notifications. It also describes multi-channel notifications including email, Telegram, Discord, Slack, Mattermost, Pushover, and webhooks.

The official Coolify pages describe team collaboration and permissions, real-time terminal access, Git integrations, automatic SSL, webhooks, APIs, monitoring, and support for single-server, multi-server, and Docker Swarm deployments.

Sources:

5. https://coolify.io/docs/databases/backups
6. https://coolify.io/docs/knowledge-base/monitoring
7. https://coolify.io/docs/enterprise/teams
8. https://coolify.io/docs/servers
