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

> **The missing control plane for Firecracker.** Deploy Docker images as isolated microVMs with automatic DNS, instant SSH, and Vercel-style preview deployments.

Built on [`firecracker-containerd`](https://github.com/firecracker-microvm/firecracker-containerd) — the same runtime AWS itself runs Fargate and Lambda workloads on. Porter is a control-plane UI/API sitting in front of it, not a from-scratch VMM orchestrator.

MIT licensed. Self-hosted. Single-tenant by permanent design.

---

## 🚀 What Porter Does

Porter brings the pieces of Fargate, Kubernetes, Vercel, and Fly.io that actually matter for a single self-hosted box, into one pure-Go binary:

- **Scale like Fargate** — `deploy.replicas: 3` in your compose file, or `porter scale api 5` any time, and Porter boots identical, isolated microVMs to match
- **Heal like Kubernetes** — declare a `healthcheck:`, set `restart: on-failure`, and Porter probes every replica, drains unhealthy ones out of traffic immediately, and replaces them automatically
- **Discover like Kubernetes DNS** — every service gets a real name (`db.my-app.local`) that resolves to its healthy replicas, no manual IP wiring
- **Deploy like Vercel** — wildcard domains, a stable URL for the live version, a unique preview URL for every deploy, real-time traffic view, all with zero DNS busywork after the first wildcard record
- **Run like Fly.io** — single binary, single host, `porter up` and you're live
- **Speak Compose natively** — bring your existing `docker-compose.yml`, each service becomes one or more real, kernel-isolated microVMs instead of containers
- **Pure Go, no Docker required** — no Docker daemon anywhere in the loop, host or guest

**Not** a Docker-in-VM system, **not** a full multi-host Kubernetes replacement, and **not** multi-tenant (see `BUILD.md` for why that last one is permanent, not a v1 limitation).

---

## ⚡ Quickstart

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

# 5. Deploy a docker-compose.yml (with replicas, healthchecks, restart policy)
porter up -f docker-compose.yml --name my-app
# → each service live at <service>.my-app.example.com, load-balanced
#   across its healthy replicas
# → services reach each other via db.my-app.local, api.my-app.local, etc.

# 6. Scale a service up or down, any time
porter scale api 5

# 7. SSH into any service by name (or a specific replica: my-app-api-2)
porter ssh my-app-api

# 8. Attach your own domain to a service, any time
porter domains add shop.mybrand.com --service my-app-api

# 9. Open the dashboard
porter dashboard   # http://localhost:3000
```

The CLI and dashboard talk to the same Control API — anything done in one shows up in the other in real time.

---

## 📚 Full Documentation

This README is intentionally short. The complete spec lives in two files:

| File | Covers |
|---|---|
| [`ARCHITECTURE.md`](./ARCHITECTURE.md) | System design, every component, data flow, the REST API reference, `docker-compose.yml` mapping rules, the domain/traffic model, and SSH access |
| [`BUILD.md`](./BUILD.md) | Dashboard UI spec, host deployment guide, v1.0.0 roadmap/scope, and the OSS + future-SaaS strategy |

---

## 📄 License

MIT — see `BUILD.md` for why, and for how Porter stays a genuinely open, single-tenant self-hosted project even if a hosted product exists someday.
