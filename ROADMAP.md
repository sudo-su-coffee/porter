# Roadmap & Scope — Porter v1.0.0

Porter is self-hosted, open source (MIT), and single-tenant by permanent design — see `OSS_AND_SAAS_STRATEGY.md` for the reasoning. Everything below is roadmap for the OSS core itself, not a hosted product.

## v1.0.0 scope (this doc set describes exactly this)

- Single host, single kernel image shared across all VMs
- One microVM per compose service (no in-guest Docker)
- Image pull, snapshotting, and VM boot handled by `containerd` + the `firecracker-containerd` shim (no custom OCI puller or guest-init)
- Vercel-style dashboard: Projects → Deployments
- Vercel-style domain model: wildcard base domain, stable + preview subdomains per deploy, custom domain attach via CNAME
- In-memory, dashboard-only traffic log per VM
- SSH gateway, cert-based (10-min ephemeral) + static-key auth
- JSON-file state store
- Single static API token auth
- CLI (`porter`) as a thin REST client mirroring the dashboard 1:1

## Explicitly deferred (not v1.0.0)

| Feature | Why deferred | Target |
|---|---|---|
| Multi-host scheduling | v1 is single-host by design; multi-host needs a real scheduler + distributed state store. Note: multi-host ≠ multi-tenant — this would still serve one trusted operator across several machines, not multiple customers. | v2 (OSS, if pursued) |
| Auto-restart / restart policies | Needs supervisor logic + backoff design; v1 leaves crashed VMs visible for manual intervention | v1.1 |
| Health-check-gated `depends_on` | Needs a defined health-check protocol between the Control API and the task running inside each VM (over the existing vsock channel) | v1.1 |
| In-project DNS resolution (service name → IP) | Workaround (`PORTER_<SERVICE>_IP` env injection) ships in v1.0.0; a real embedded DNS stub is a v1.1 addition | v1.1 |
| Volume / bind-mount support (virtio-fs) | Needs a real design for persistence across VM restarts and host paths | v1.1–v2 |
| Non-HTTP port forwarding through the gateway (raw TCP/UDP) | v1.0.0 gateway is HTTP-only | v1.1 |
| In-browser terminal (SSH-in-UI) | Copy-command UX ships in v1.0.0; xterm.js + WebSocket bridge is more surface area | v1.1 |
| Multi-user auth / RBAC | **Not planned, ever, for the OSS core** — Porter is single-tenant by permanent design, not by v1 limitation. See `OSS_AND_SAAS_STRATEGY.md`. A future hosted product would handle multi-tenancy in a separate closed-source layer, without this core changing. | Not on the OSS roadmap |
| Host-reboot VM auto-resume | Needs persisted boot-on-start config + ordering | v1.1 |
| Image scanning / policy enforcement | No vulnerability/policy gate on pulled images in v1 | v2 |
| SSH session recording | Connection metadata logging only in v1.0.0 | v1.1+ |
| Cross-project networking / peering | Projects are isolated bridges by design in v1 | v2 |
| Automatic TLS (Let's Encrypt) for wildcard + custom domains | v1.0.0 ships domain routing but not cert provisioning | v1.1 |
| Persisted / exportable traffic logs (log drains) | v1.0.0 traffic log is an in-memory ring buffer, dashboard-only, cleared on gateway restart | v1.1 |
| Aggregate/historical traffic analytics (daily/weekly trends) | v1.0.0 traffic view is "what's happening right now," per-VM only | v1.2+ |
| `build:` support in compose (building from Dockerfile) | Would require a build pipeline separate from containerd's pull-only path | v2, possibly never |
| High-availability gateway / SSH gateway | Both are single processes in v1 | v2 |

## Known limitations to document clearly at launch

1. containerd's snapshotter (devmapper, block-device backed — required for `firecracker-containerd`) needs its thin-pool sized and configured correctly on the host, or task creation fails; call this out prominently in `DEPLOYMENT.md`'s host requirements.
2. Snapshot/content-store cache has no Porter-level size cap / eviction policy in v1.0.0 beyond whatever `containerd`'s own garbage collection does — monitor disk usage manually.
3. `depends_on` readiness is "VM/task is running," not "app is accepting connections" — services must handle their own dependency-not-ready-yet retry logic.
4. No image content scanning — only pull images you trust, same caveat as running any `docker pull` today.
5. No TLS termination built in — front Porter with your own cert layer until v1.1.

## Suggested version sequence

- **v1.0.0** — everything in this doc set
- **v1.1.0** — DNS stub, restart policies, health-check-gated boot ordering, in-browser terminal, raw TCP/UDP forwarding, automatic TLS, persisted/exportable traffic logs
- **v1.2.0** — virtio-fs volumes, host-reboot VM resume, aggregate traffic analytics
- **v2.0.0** — multi-host scheduling, real database backend, multi-user auth/RBAC, image policy engine
