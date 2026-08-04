<p align="center">
  <img width="80" height="80" alt="Porter Logo" src="/assets/porterlogo.png" />
</p>
# Porter
```
██████╗  ██████╗ ██████╗ ████████╗███████╗██████╗ 
██╔══██╗██╔═══██╗██╔══██╗╚══██╔══╝██╔════╝██╔══██╗
██████╔╝██║   ██║██████╔╝   ██║   █████╗  ██████╔╝
██╔═══╝ ██║   ██║██╔══██╗   ██║   ██╔══╝  ██╔══██╗
██║     ╚██████╔╝██║  ██║   ██║   ███████╗██║  ██║
╚═╝      ╚═════╝ ╚═╝  ╚═╝   ╚═╝   ╚══════╝╚═╝  ╚═╝
  ```                                                                                                         
                                                                       
                                                                       
<p align="left">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/badge/version-v1.0.0-181717.svg?style=flat&logo=vercel&logoColor=white&mode=dark" />
    <img alt="Version 1.0.0" src="https://shieldcn.dev/badge/version-v1.0.0-181717.svg?style=flat&logo=vercel&logoColor=black&mode=light" />
  </picture>
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/badge/Go.svg?variant=secondary&size=sm&logo=go&logoColor=00ADD8&mode=dark" />
    <img alt="Go" src="https://shieldcn.dev/badge/Go.svg?variant=secondary&size=sm&logo=go&logoColor=00ADD8&mode=light" />
  </picture>
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/badge/Firecracker.svg?variant=secondary&size=sm&logo=amazonaws&logoColor=FF9900&mode=dark" />
    <img alt="Firecracker" src="https://shieldcn.dev/badge/Firecracker.svg?variant=secondary&size=sm&logo=amazonaws&logoColor=FF9900&mode=light" />
  </picture>
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/badge/SQLite.svg?variant=secondary&size=sm&logo=sqlite&logoColor=003B57&mode=dark" />
    <img alt="SQLite" src="https://shieldcn.dev/badge/SQLite.svg?variant=secondary&size=sm&logo=sqlite&logoColor=003B57&mode=light" />
  </picture>
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/badge/gRPC.svg?variant=secondary&size=sm&logo=grpc&logoColor=6C4A9B&mode=dark" />
    <img alt="gRPC" src="https://shieldcn.dev/badge/gRPC.svg?variant=secondary&size=sm&logo=grpc&logoColor=6C4A9B&mode=light" />
  </picture>
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/badge/Linux.svg?variant=secondary&size=sm&logo=linux&logoColor=FCC624&mode=dark" />
    <img alt="Linux" src="https://shieldcn.dev/badge/Linux.svg?variant=secondary&size=sm&logo=linux&logoColor=FCC624&mode=light" />
  </picture>
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/badge/MIT.svg?variant=secondary&size=sm&logo=opensourceinitiative&logoColor=3DA639&mode=dark" />
    <img alt="MIT License" src="https://shieldcn.dev/badge/MIT.svg?variant=secondary&size=sm&logo=opensourceinitiative&logoColor=3DA639&mode=light" />
  </picture>
</p>

> **The missing control plane for Firecracker.** Deploy Docker images as isolated microVMs with automatic DNS, instant SSH, and Vercel-style preview deployments.

**Spin up Firecracker microVMs from any Docker image or `docker-compose.yml`.** Get an instant live URL, native SSH access, and real-time traffic visibility—right from your terminal or dashboard. 

It's the Vercel experience, but every deploy is a real, kernel-isolated microVM instead of a shared container.

Built on [`firecracker-containerd`](https://github.com/firecracker-microvm/firecracker-containerd) — the same runtime AWS itself runs Fargate and Lambda workloads on. Porter is a control-plane UI/API sitting in front of it, not a from-scratch VMM orchestrator.

---

## 🚀 What Porter Does

- **Deploy a single image** → `porter up --image redis:7` → running microVM in seconds, reachable at an auto-assigned URL
- **Deploy a `docker-compose.yml`** → each service becomes its own microVM, booted in dependency order, all reachable on the same private network
- **Domains, the Vercel way** — point one wildcard DNS record at Porter once. Every deploy gets its own subdomain automatically: a stable one for the current live version, plus a unique preview subdomain per deploy. Attach your own fully-owned domain to any service anytime via a CNAME
- **SSH into any VM by name** through a single gateway — no IP hunting, no per-VM key management
- **Live traffic view** per service — method, path, status, latency — right in the dashboard, no log pipeline to wire up
- **Dashboard** — Projects → Deployments, status at a glance, one-click stop/restart/delete/SSH/logs

---

## 🚫 What Porter Is Not

- **Not** a Docker-in-VM system. The guest never runs a Docker daemon — `containerd`'s `firecracker-containerd` shim snapshots each compose service's image straight onto a microVM's root block device. One service, one VM, one kernel boundary.
- **Not** a Kubernetes replacement. No scheduler, no multi-host bin-packing in v1.0.0. One host, one shared kernel image, as many microVMs as it can hold.
- **Not** multi-tenant. v1.0.0 assumes one trusted operator per deployment.

---

## 🏗️ Architecture at a Glance

```
┌───────────────────────────────────────────────────────────────┐
│  Dashboard (Next.js/React)                                     │
│  Projects → Deployments → Domains / Traffic / Logs / SSH        │
└───────────────┬─────────────────────────────────────────────┘
                │ REST + SSE
┌───────────────▼─────────────────────────────────────────────┐
│  Control API (Go)                                                │
│  project/VM CRUD, compose parsing, domain records, state store   │
└───────┬─────────────────┬─────────────────┬───────────────────┘
        │                 │                 │
┌───────▼──────┐  ┌────────▼────────┐  ┌──────▼───────────┐
│ VM Manager   │  │ containerd +    │  │ SSH Gateway        │
│ (containerd  │  │ firecracker-    │  │ (bastion, routes   │
│  task client)│  │ containerd shim │  │  via task.Exec)     │
└───────┬──────┘  └────────┬────────┘  └──────┬───────────┘
        │                  │                  │
┌───────▼──────────────────▼──────────────────▼───────────────────┐
│  Host: containerd, jailer + Firecracker processes (shim-managed)  │
│  Edge: Porter Gateway — routing, domains, TLS-ready, traffic log  │
└───────────────────────────────────────────────────────────────┘
```

For the full deep dive, see [`ARCHITECTURE.md`](./ARCHITECTURE.md).

---

## ⚡ Quickstart (Target UX for v1.0.0)

```bash
# 1. Install the Porter agent (one binary, needs KVM + root)
curl -fsSL https://get.porter.dev | sh

# 2. Point Porter at a kernel image (one-time)
porter kernel set ./vmlinux-5.10

# 3. Point your domain's wildcard DNS at this host (one-time)
#    *.example.com  A  <this-host-ip>
porter domain set-base example.com

# 4. Deploy a single image
porter up --image redis:7 --name cache
# → live at cache.example.com

# 5. Deploy a docker-compose.yml
porter up -f docker-compose.yml --name my-app
# → each service live at <service>.my-app.example.com
# → this specific deploy also live at <service>-<deploy-id>.my-app.example.com

# 6. SSH into any service by name
porter ssh my-app-api

# 7. Attach your own domain to a service, any time
porter domains add shop.mybrand.com --service my-app-api

# 8. Open the dashboard
porter dashboard   # http://localhost:3000
```

The CLI and dashboard talk to the same Control API — anything done in one shows up in the other in real time.

---

## 📚 Documents in This Set

| File | Purpose |
|------|---------|
| `README.md` | This file — overview, quickstart, directory layout |
| `ARCHITECTURE.md` | Full system design: components, data flow, networking, SSH gateway, edge gateway |
| `API_SPEC.md` | REST API reference for the Control API |
| `COMPOSE_MAPPING.md` | Exact rules for translating `docker-compose.yml` → microVMs |
| `SSH_ACCESS.md` | How the SSH gateway works, key management, guest-side setup |
| `DOMAINS_AND_TRAFFIC.md` | Wildcard domain model, preview vs. production subdomains, custom domains, live traffic log |
| `UI_SPEC.md` | Dashboard screens, components, states, and interaction flows |
| `ROADMAP.md` | v1.0.0 scope, what's deferred, known limitations |
| `DEPLOYMENT.md` | Host requirements, install steps, config reference |
| `OSS_AND_SAAS_STRATEGY.md` | Why MIT, why single-tenant-forever, and how a future hosted product would relate to this repo without changing it |

---

## 📁 Directory Layout (Target Repo Structure)

```
porter/
├── backend/
│   ├── cmd/
│   │   ├── server/          # Control API + gateway + SSH gateway daemon
│   │   └── cli/             # porter CLI (thin client over REST API)
│   ├── internal/
│   │   ├── api/             # HTTP handlers
│   │   ├── vmmanager/       # containerd task client + firecracker-containerd config
│   │   ├── compose/         # docker-compose.yml parser/mapper
│   │   ├── gateway/         # routing, domains, traffic log
│   │   ├── sshgw/           # SSH gateway (proxies to containerd task.Exec)
│   │   ├── store/           # state persistence
│   │   └── netmgr/          # CNI network config, bridges, IP allocation
│   └── pkg/types/           # shared domain types
├── frontend/                 # Next.js dashboard
├── docs/                     # this document set
└── deploy/                   # systemd units, install script, kernel build notes,
                               # containerd + firecracker-containerd config
```

---

## 📄 License

MIT — see [`OSS_AND_SAAS_STRATEGY.md`](./OSS_AND_SAAS_STRATEGY.md) for why, and for how Porter stays a genuinely open, single-tenant self-hosted project even if a hosted product exists someday.
