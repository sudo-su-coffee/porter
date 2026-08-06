# Using Porter

Porter is a self-hosted PaaS (Vercel/Fly.io-style) that runs **Docker/OCI images
as Firecracker microVMs**. This guide covers the v0.1.0 flow end to end.

- `idea.md` — what Porter is and why.
- `README.md` — architecture, config reference, API, compose rules.

---

## 1. What you need (Linux host)

A Linux host with KVM (`/dev/kvm`) is required — microVMs cannot boot without
hardware virtualization. Porter itself is a single Go binary.

Install the runtime stack once (see `deploy/host/*.sh`):

```
containerd               # container runtime + content store
aws.firecracker shim     # the runtime that boots a microVM per container
devmapper snapshotter    # image/rootfs layering
firecracker binary       # the VMM
vmlinux kernel           # shared guest kernel (provision with `porter kernel set`)
```

Run the deploy scripts in order:

```bash
bash deploy/host/01-containerd.sh
bash deploy/host/02-shim.sh
bash deploy/host/03-cni.sh
```

Then provision a kernel (local file or remote URL):

```bash
porter kernel set ./vmlinux                 # local
porter kernel set https://…/vmlinux         # remote
```

## 2. Configure

```toml
[server]
listen_addr = ":8080"
base_domain = "porter.test"
state_file  = "porter.db"
api_token   = "change-me"

[firecracker]
containerd_socket = "/run/containerd/containerd.sock"
snapshotter       = "devmapper"
namespace         = "porter"
logs_dir          = "/var/log/porter"

[admin]
username = "admin"
password = "change-me"
```

Every setting has a `PORTER_*` env override (`PORTER_API_TOKEN`,
`PORTER_CONTAINERD_SOCKET`, …). Porter refuses to start without an API token
and admin password.

## 3. Start

```bash
porter server -workers 2 -config porter.toml
# Dashboard: http://localhost:8080  (login with the [admin] credentials)
```

## 4. Deploy an image

Via the dashboard, or the API:

```bash
curl -X POST localhost:8080/vms \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"cache","image":"redis:7-alpine","vcpus":1,"mem_mib":256,
       "ports":[{"container_port":6379,"protocol":"tcp"}]}'
```

Porter pulls the image through containerd and boots it as a microVM
(`pending → booting → running`). Watch the state stream at `GET /events`.

## 5. Deploy a Compose app

The canonical input is a `compose.yml` referencing images — one microVM per
service:

```bash
curl -X POST localhost:8080/projects/compose \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"myapp","compose_yaml":"services:\n  api:\n    image: nginx\n    ports:\n      - 8080:80\n"}'
```

## 6. Manage

```bash
POST   /vms/{id}/stop        # graceful stop
POST   /vms/{id}/start       # re-boot
POST   /vms/{id}/restart     # stop + start
DELETE /vms/{id}             # stop + remove
GET    /vms/{id}/logs?tail=200
GET    /vms/{id}/traffic?limit=100
GET    /overview             # host + VM counts
GET    /images               # image catalog
```

## 7. Day-2 operations

- Logs land in `PORTER_LOGS_DIR` and stream live in the dashboard.
- Traffic is recorded per-VM in an in-memory ring and served by the API.
- Stop the process → Porter gracefully stops all tracked VMs.

## Troubleshooting

- `containerd socket not found at /run/containerd/containerd.sock` — containerd
  isn't running. `systemctl start containerd` (or run `deploy/host/01-containerd.sh`).
- `runtime aws.firecracker not registered` — run `deploy/host/02-shim.sh`.
- `pull image ... snapshotter devmapper` — the devmapper snapshotter isn't
  configured; see `deploy/host/02-shim.sh`.
- `kernel set` errors — point `porter kernel set` at a real vmlinux for the
  shim's `/etc/containerd/firecracker-runtime.json`.