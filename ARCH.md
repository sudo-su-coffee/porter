# Porter — Architecture & Flow

> How a deploy becomes running VMs, how traffic reaches them, and where every
> piece of a project lives. Read this top-to-bottom for the mental model, or
> jump to a section. All of this matches `backend/internal` as it actually
> works today (2026-08, `go build/vet/test` green).

---

## 1. The one-line model

```
A PROJECT is an app. Each app runs as a POOL OF VM REPLICAS.
Every replica is one Firecracker microVM. Traffic is routed to the pool.
```

**User → Org → Project → Services → VM Replicas**

```
┌─────────────────────────────────────────────────────────────────┐
│ User (admin / member / viewer — per-user token + RBAC roles)    │
└──────────────────────────────┬──────────────────────────────────┘
                               │ creates
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│ ORG   (owner / member) — org_members table                      │
└──────────────────────────────┬──────────────────────────────────┘
                               │ owns
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│ PROJECT  = one deployable app                                   │
│   • source: image | compose | git                               │
│   • image: OCI ref (e.g. nginx:1.27)                            │
│   • replicas_desired (pool size)                                │
│   • env / secrets, ports, networks, healthcheck                 │
│   • domains (preview + production)                              │
│   • autoscale policy (min/max replicas, CPU target)             │
│   • service_pools (one per compose service)                     │
│   • vm_ids → the replica pool                                   │
└──────────────┬───────────────────────────┬──────────────────────┘
               │                            │
               ▼                            ▼
┌─────────────────────────────┐   ┌─────────────────────────────┐
│ VM REPLICA 0  (microVM)     │   │ VM REPLICA 1  (microVM)     │
│   • project_id, service     │   │   • same image, ports, env  │
│   • replica_index 0         │   │   • replica_index 1         │
│   • static IP + MAC + tap   │   │   • own IP on project /24   │
│   • state/health per-VM     │   │   • state/health per-VM     │
│   • own logs + metrics      │   │   • own logs + metrics      │
└─────────────────────────────┘   └─────────────────────────────┘
```

Every replica is identical (same image, env, ports) — that's what makes the pool
scalable and replaceable. If one goes unhealthy, the health checker replaces it
with a fresh identical VM.

---

## 2. What a project actually contains (the full inventory)

| Piece | Type | Where it lives | Notes |
|---|---|---|---|
| Org | `orgs` + `org_members` | Postgres | owner/member roles |
| Project row | `projects` | Postgres | name, source, image, env, networks |
| Replica pool | `vm_ids` + `replicas` | Postgres | one VM row per replica |
| Service pools | `service_pools` | Postgres | compose multi-service: one pool per service |
| Domains | `domains` | Postgres | preview + production + custom |
| Deployments | `deployments` | Postgres | build status, preview/promote/rollback |
| Env & secrets | `envs` + `secrets` | Postgres | secrets AES-GCM encrypted at rest |
| Builds (git) | `builds` | Postgres | git URL, branch, build log, status |
| Traffic | `traffic_logs` + in-memory ring | Postgres + RAM | per-request: status, latency, bytes |
| Web vitals | in-memory ring | RAM | LCP/CLS/INP/TTFB beacons |
| Metrics | `metrics_samples` | Postgres | CPU%, memory per VM (30s samples) |
| Logs | `daemon_logs` + per-VM ring | Postgres + RAM | live tail + durable copies |
| Health events | `health_events` | Postgres | transitions healthy/unhealthy |
| Firewall rules | `firewall_rules` | Postgres | deny rules enforced by gateway |
| Cron jobs | `crons` | Postgres | 5-field schedule → job microVM |
| Volumes | `volumes` | Postgres + disk | real dir + sparse `data.img` |
| Autoscale policy | `autoscale` on project | Postgres | min/max replicas, CPU target |
| Members | `project_members` | Postgres | per-project roles (owner/member/viewer) |

---

## 3. Deploy flow — how an app goes from "pressed deploy" to "running"

Three source paths converge on the **same** runtime: `internal/runtime` boots an
OCI image as a Firecracker microVM.

```
          ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
 Deploy   │   image      │     │  compose     │     │  git         │
          │  (OCI ref)   │     │ (compose.yml)│     │  (repo)      │
          └──────┬───────┘     └──────┬───────┘     └──────┬───────┘
                 │                    │                    │
                 ▼                    ▼                    ▼
        ┌─────────────────────────────────────────────────────────┐
        │ build image (compose expands; git → clone + Dockerfile) │
        │ → OCI image in containerd content store (namespace:     │
        │   "porter")                                              │
        └──────────────────────────┬──────────────────────────────┘
                                   │
                                   ▼
        ┌─────────────────────────────────────────────────────────┐
        │ internal/runtime (containerd + aws.firecracker shim)    │
        │ boots N replicas from that image                        │
        └───────────────┬───────────────────────┬─────────────────┘
                        │                       │
                        ▼                       ▼
              ┌────────────────┐      ┌────────────────┐
              │ replica 0      │      │ replica 1      │
              │ ip 10.42.0.2   │      │ ip 10.42.0.3   │
              │ boot→running   │      │ boot→running   │
              └────────────────┘      └────────────────┘
```

- **Image deploy:** `POST /projects` with an image ref → pool boots.
- **Compose deploy:** `POST /projects/compose` → each service becomes a project
  with its own `service_pool` → replicas per service.
- **Git deploy:** `POST /projects/{id}/deployments/git` → clone → detect
  Dockerfile → build (BuildKit) → OCI → boot. Preview → promote to production.

### Preview → promote → rollback (release versions)

```
       preview domain                        production domain
  myapp.preview.porter.test             myapp.porter.test
            │                                    │
            ▼                                    ▼
   ┌──────────────────┐                 ┌──────────────────┐
   │ deployment v2    │                 │ deployment v1    │
   │ (new build/ver)  │   promote ───▶  │ (current live)   │
   └──────────────────┘                 └──────────────────┘
          ▲                                    │
          │         rollback (reverts to v1    │
          └─────────────── v0 ─────────────────┘
```

Each deployment is a tagged image (by git SHA or build id). **Promote** moves a
preview deployment to the production domain; **rollback** points the production
pool back at an earlier image. The replica pool is re-booted against the target
image, one replica at a time (rolling), so there's no full downtime.

---

## 4. Request flow — how a user's browser reaches a replica

```
 Browser → https://myapp.porter.test
                │  (TLS: internal/tls — Let's Encrypt cert for *.porter.test)
                ▼
   ┌─────────────────────────────────────────────────────────────┐
   │ GATEWAY (internal/gateway, port :80/:443)                   │
   │  1. read Host header → myapp.porter.test                    │
   │  2. DNS resolve if *.local (internal/dns resolver)          │
   │  3. firewall check: deny rules by source IP/CIDR → 403      │
   │  4. find the project's replica pool                         │
   │  5. round-robin → pick a healthy replica                    │
   │  6. reverse-proxy to replica IP:port                        │
   │  7. record traffic: status, latency, bytes_in/bytes_out     │
   └───────────────────────┬─────────────────────────────────────┘
                           │
                           ▼
                 ┌──────────────────────┐
                 │ replica (microVM)    │
                 │ 10.42.0.2:80         │
                 │ your app code runs   │
                 └──────────────────────┘
```

Observability is collected on every hop:
- **Traffic** (gateway) → ring + `traffic_logs` → dashboard Traffic / analytics.
- **Logs** (runtime per-VM) → ring + `daemon_logs` → dashboard Logs.
- **Metrics** (collector, 30s) → `metrics_samples` → dashboard Metrics.
- **Web vitals** (browser beacon) → ring → dashboard Web Vitals.
- **Health** (checker) → `health_events` → dashboard status + auto-replace.

---

## 5. Data flow into observability

```
 ┌─────────────┐  ┌──────────────┐  ┌───────────────┐  ┌────────────┐
 │ gateway     │  │ runtime      │  │ metrics       │  │ browser    │
 │ (traffic)   │  │ (per-VM logs)│  │ (cpu/mem)     │  │ (vitals)   │
 └──────┬──────┘  └──────┬───────┘  └───────┬───────┘  └─────┬──────┘
        │                 │                  │                │
        ▼                 ▼                  ▼                ▼
   in-memory ring    in-memory ring    Postgres         in-memory ring
   traffic_logs      daemon_logs       metrics_samples  (web vitals)
        │                 │                  │                │
        └─────────────────┴────────┬─────────┴────────────────┘
                                   ▼
                    ┌──────────────────────────────┐
                    │ DASHBOARD + API              │
                    │ /projects/{id}/traffic       │
                    │ /projects/{id}/logs          │
                    │ /projects/{id}/metrics       │
                    │ /projects/{id}/web-vitals    │
                    │ /overview (global)           │
                    └──────────────────────────────┘
```

---

## 6. VM replica lifecycle

```
        ┌────────┐  scale up  ┌────────┐  boot ok  ┌─────────┐
  ----▶ │pending │──────────▶ │booting │──────────▶│ running │
        └────────┘            └────────┘           └────┬────┘
           ▲                                             │ health check
           │ failed / replaced                           ▼
        ┌────────┐                                 ┌────────────┐
        │ failed │◀──────────── unhealthy ─────────│ unhealthy  │
        └────────┘                                 └────────────┘
                                                     (health checker
                                                      boots a replacement)
```

States: `pending → booting → running → stopping → stopped` (+ `failed`).
The health checker probes each running VM; a bad replica is stopped and a fresh
one boots to keep the pool at `replicas_desired`. The autoscaler adjusts
`replicas_desired` between `min`/`max` based on average CPU.

---

## 7. Domain / DNS / TLS flow

```
 project created ──▶ internal/dns/domains.go auto-assigns:
                        preview:   <slug>.preview.<base_domain>
                        production:<slug>.<base_domain>

 *.base_domain queries ──▶ internal/dns/server.go (UDP/TCP :53, miekg/dns)
                              A/AAAA → gateway IP
                              PTR    → reverse lookup
                              NS/SOA → authoritative answers

 https://<slug>.base_domain ──▶ internal/tls (Let's Encrypt ACME)
                                    cert cached on disk (autocert.DirCache)
                                    auto-renew, HTTP-01 challenge
```

---

## 8. Package map (where each piece lives)

| Responsibility | Package |
|---|---|
| Boot/stop/restart VMs (containerd + firecracker) | `internal/runtime` |
| Per-project subnet + IP + MAC allocation | `internal/net`, `internal/netmgr` |
| Compose parse → services | `internal/compose` |
| Reverse proxy + traffic + firewall | `internal/gateway` |
| `.local` + `*.base_domain` DNS resolution | `internal/dns` |
| Let's Encrypt certs | `internal/tls` |
| Healthcheck + auto-replace | `internal/health` |
| Per-user auth + project RBAC | `internal/api` (`requireProjectRole`) |
| Cron scheduler | `internal/cron` |
| Horizontal autoscaler | `internal/autoscale` |
| Metrics collector | `internal/metrics` |
| Real volumes | `internal/volumes` |
| PostgreSQL persistence | `internal/store` |
| Pre-flight sanity checks | `internal/startup` |
| SSH bridge into OCI VMs | `internal/sshgw` |

---

## 9. Multi-service (compose) shape

A compose app with 3 services becomes 3 projects sharing one stack:

```
 PROJECT "app" (StackID = stack-123)
 ├── service "web"   → ServicePool{desired:2, healthy:2} → replicas web-0, web-1
 ├── service "api"   → ServicePool{desired:2, healthy:2} → replicas api-0, api-1
 └── service "db"    → ServicePool{desired:1, healthy:1} → replica  db-0

 Each replica: <service>-<n>.<project>.local  → resolves to its pool
```

Services talk to each other over the project bridge network by service name
(`web` → `api:8080`), exactly like Docker Compose, but each service runs on
kernel-isolated microVMs.
