# Deployment — Porter v1.0.0

## Host requirements

| Requirement | Why |
|---|---|
| Linux with KVM (`/dev/kvm` present, readable/writable) | Firecracker requires KVM — no software-emulation fallback |
| `containerd` ≥ 1.7, with the [`firecracker-containerd`](https://github.com/firecracker-microvm/firecracker-containerd) runtime shim installed and registered (`runtime-type: "aws.firecracker"`) | This is Porter's execution engine — VM Manager talks to it over containerd's task API instead of driving Firecracker directly |
| `devmapper` snapshotter configured on containerd (thin-pool set up ahead of time) | `firecracker-containerd` needs a block-device-backed snapshotter to hand each VM a root block device; the default overlayfs snapshotter doesn't work here |
| `iproute2` (`ip` command) + a CNI plugin set including `tc-redirect-tap` | Tap device wiring between the host bridge and each microVM — installed as part of `firecracker-containerd`'s own setup |
| Root or equivalent capabilities (`CAP_NET_ADMIN`, `CAP_SYS_ADMIN`) for the `containerd`/shim processes | Tap/bridge creation, jailer chroot + cgroup setup — the shim does this, Porter's Control API process itself does not need these capabilities |
| A Linux kernel image built/configured for Firecracker guest boot (`vmlinux`, uncompressed) | Shared across all VMs — one kernel, referenced in `firecracker-containerd`'s runtime config |
| Outbound network access to your OCI registry (Docker Hub, GHCR, etc.) | `containerd` pulls images directly via the registry HTTP API |
| A domain you control, with the ability to add a wildcard DNS record | Needed for the auto stable/preview subdomain model — see `DOMAINS_AND_TRAFFIC.md` |
| Disk space for containerd's content store + devmapper thin-pool | Each unique image gets its own cached snapshot; no Porter-level cap in v1.0.0 (see `ROADMAP.md`) |

## Recommended host sizing (starting point, not a hard rule)

- Reserve host resources separately from what you hand out to VMs — e.g. on a 16 vCPU / 32GB host, budget ~2 vCPU / 4GB for the Control API + gateway + SSH gateway + `containerd`/shim overhead, leaving the rest for VM allocation
- Firecracker's own overhead per microVM is small, so density is mostly bound by however much vCPU/mem you assign per VM

## Install (target v1.0.0 flow)

```bash
# 0. install & configure containerd + the firecracker-containerd shim first
#    (devmapper thin-pool, runtime registration) — see firecracker-containerd's
#    own getting-started docs; Porter assumes this is already working
#    (`ctr run --runtime aws.firecracker ...` should boot a test VM)

# one binary bundles: Control API, gateway, SSH gateway
curl -fsSL https://get.porter.dev | sh

# verify KVM access
ls -l /dev/kvm

# set the shared kernel image Porter should tell firecracker-containerd to use (one-time)
porter kernel set /path/to/vmlinux

# point your domain's wildcard DNS at this host, then tell Porter:
#   *.example.com  A  <this-host-ip>
porter domain set-base example.com

# set required env / config
export PORTER_API_TOKEN=$(openssl rand -hex 32)
export PORTER_DATA_DIR=/var/lib/porter     # state.json, rootfs cache, run sockets
export PORTER_API_ADDR=:8080
export PORTER_GATEWAY_ADDR=:8081
export PORTER_SSH_GATEWAY_ADDR=:2222

# run as a systemd service (unit shipped under deploy/systemd/)
sudo systemctl enable --now porter
```

## Config reference

| Env var | Default | Purpose |
|---|---|---|
| `PORTER_API_TOKEN` | *(required, no default)* | Bearer token for all Control API + dashboard + CLI auth |
| `PORTER_DATA_DIR` | `/var/lib/porter` | Root for state file, rootfs cache, VM run sockets |
| `PORTER_KERNEL_PATH` | `$PORTER_DATA_DIR/vmlinux` | Shared kernel image path |
| `PORTER_API_ADDR` | `:8080` | Control API listen address |
| `PORTER_GATEWAY_ADDR` | `:8081` | HTTP gateway (routing + domains + traffic) listen address |
| `PORTER_SSH_GATEWAY_ADDR` | `:2222` | SSH gateway listen address |
| `PORTER_BRIDGE_BASE` | `10.42.0.0/16` | Base range sliced into per-project `/24` subnets |
| `PORTER_SSH_CERT_TTL` | `10m` | Lifetime of gateway-issued SSH certificates |
| `PORTER_BASE_DOMAIN` | *(required, no default)* | Wildcard base domain for stable/preview subdomains — set via `porter domain set-base` |
| `PORTER_DOMAIN_VERIFY_INTERVAL` | `30s` | How often pending custom-domain CNAMEs are re-checked |
| `PORTER_TRAFFIC_LOG_SIZE` | `2000` | Max requests kept per VM in the in-memory traffic ring buffer |

## Firewall / exposure guidance

- Expose only what you mean to expose publicly:
  - `PORTER_GATEWAY_ADDR` (8081) — fine to expose publicly, it's the HTTP front door
  - `PORTER_SSH_GATEWAY_ADDR` (2222) — expose if you want remote SSH access; otherwise keep to trusted networks/VPN
  - `PORTER_API_ADDR` (8080) — **do not expose publicly** without adding a real auth layer in front — v1.0.0's single static token is not designed to withstand public internet exposure on its own

## Upgrading

v1.0.0 has no migration story yet since it's the first version — `state.json` schema stability across versions will be documented starting at v1.1.0.

## Uninstall / cleanup

```bash
sudo systemctl disable --now porter
# stop all VMs first via the API/CLI, or:
sudo pkill firecracker
sudo rm -rf /var/lib/porter
# remove any leftover tap devices
ip -o link show | grep tap- | awk -F': ' '{print $2}' | xargs -n1 sudo ip link del
```
