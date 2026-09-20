# Vyoma → Porter feature distill (for Go/Firecracker reimplementation)

> Source: `references/vyoma/` (removed 2026-09-18, Rust + Cloud Hypervisor, Apache-2.0, v2.9.0).
> Purpose: keep only what Porter needs. Reimplement natively in Go against Firecracker — never vendor Rust.
> Full spec was `vyoma-technical-spec.md` (~2000+ lines); CLI in `COMMANDS.md`; crates in `Cargo.toml`.

## 1. OCI → MicroVM (highest value — adopt first)

- `pull <image>`: manifest → config blob (`OciImageConfig`: entrypoint/cmd/env/workdir/exposed_ports/user) → layers → unpack → `rootfs.ext4` + `vyoma-config.json` + manifest in image cache.
- `build -t <tag> .`: Vyomafile `FROM`/`RUN`/`COPY` (working) + MUST add `CMD`/`ENTRYPOINT`/`ENV`/`WORKDIR`/`EXPOSE` (were silently dropped in vyoma v1.1 — VMs booted to `/bin/sh`, most Hub images broken).
- `run <image>`: inject `/sbin/vyoma-init` (env exports + cd + exec ENTRYPOINT+CMD) via debugfs into COW layer, boot kernel with `init=/sbin/vyoma-init`.
- Porter mapping: `internal/imagecatalog` + `internal/build/buildkit` + guest rootfs prep → FC drives. Artifacts immutable by digest. Add image signing (Ed25519) + VMIF format (kernel+rootfs+metadata) + Hub bridge (Docker Hub → VMIF cache).

## 2. Lifecycle UX (Porter CLI parity)

`run/stop/start/restart/ps/logs -f/exec/rm` + `pull/build` + `network create/ls/rm` + `up -d/down/scale` + `swarm init/join/ls` + `snapshot/restore/export/import` + `commit/save/load` + `doctor/stats/inspect`.
Porter: same verbs over REST (`POST /vms/:id/exec`, `/console`, `/logs`), thin `porter` CLI as API client.

## 3. Storage (COW + snapshots)

- COW: sparse file + loop + device-mapper snapshot (origin read-only base + COW delta). Replace `dmsetup/losetup` subprocess with native libs where Go allows.
- Snapshot tree (replace git-hack): `SnapshotNode{id, vm_id, parent_id, created_at, label/tag, memory/snapshot/cow_delta paths}` in sled/PG; ops `history/branch/diff`; delta-only backups.
- Ops: `snapshot/restore`, `export/import` (tarball), `commit <vm> <tag>` (pause → merge COW → new image → resume), `save/load` (image tar + config).
- Porter: volumes independent of VM lifecycle (`/dev/vdb`), auto-snapshot before deploy, TimeMachine point-in-time restore, snapshot diffs for cheap backup.

## 4. Networking

- Per-VM TAP + Linux bridge (`vyoma0`) + NAT; CNI bridge/host-local IPAM; deterministic subnet leases per node (e.g. `10.42.X.0/24`); internal DNS on gateway (`.1:53`, service-name → IP).
- Swarm overlay: VXLAN skeleton → MUST add WireGuard (boringtun model, UDP 51820, keypair per node, `add_peer` on join) — vyoma shipped plaintext, insecure for multi-tenant.
- Porter: TAP + `/30` NAT + token-bucket rate limiters + vsock (guestagent, no SSH) + optional slirp user-mode NAT isolation mode + per-stack IPAM subnets.

## 5. Orchestration

- Compose: `vyoma-compose.yml` `up -d/down/scale web=3`. MUST support Docker Compose v3 schema (`services/networks/volumes`, `depends_on`, `environment` map/list, `deploy.replicas`, `vm:` extension block for kernel/vcpu/mem/snapshot policy) — vyoma v1.0 schema incompatible.
- Swarm: `init/join/ls`; seed-node SPOF → replace with Raft for Porter multi-node scheduler.

## 6. Guest agent (no SSH)

- `exec <id> <cmd>` + `logs -f` over secure agent HTTP channel (vsock in Porter). Agent also reports `ProcessInfo` + `VmMetrics` (CPU/mem/oom) feeding scheduler/autoscaling/health.
- Porter: guestagent over vsock (host CID 2, guest ≥3); exec/console/logs endpoints RBAC-gated (`vm.console`, `log.read`).

## 7. Hardening + ops

- privdrop: dedicated `vyoma` user, caps `CAP_NET_ADMIN/CAP_SYS_ADMIN/CAP_NET_RAW/CAP_SETUID/CAP_SETGID` only, socket `0660`, `/dev/kvm` via `kvm` group. Porter: same + jailer + seccomp + SMT/KSM/swap off + egress/IMDS filtering + serial off.
- `doctor`: KVM/TAP/daemon/virtiofsd/caps checks → Porter `porter doctor` (OS/arch/kernel/KVM/cgroups/FC/BuildKit/PG/agent/TAP/bridge/routing/firewall/IPAM/gateway/DNS/TLS/storage/scheduler/capacity).
- WAL crash recovery: sled WAL (`VmCreating/Started/Stopping/Stopped/Destroyed`, `SnapshotCreated`, `VolumeAttached`, `NetworkCreated/Deleted`) + `recover_from_wal` (adopt alive, cleanup orphaned/half-created) + chaos tests. Porter: PG is truth + idempotent reconcile (same guarantee, different store).
- Metrics: Prometheus exporter + `stats <id>`; cgroups v2 CPU/mem/IO enforcement + `get_cpu_usage/get_memory/oom_kill_count`.

## 8. Migration / boot trust

- Teleport live migration (`send_migration/receive_migration`, dirty-page tracking) + hibernate (suspend to disk) for node drain/cost.
- Boot assets: kernel/initramfs/UEFI-OVMF management; vTPM (`set_tpm`, PCR read) + unified attestation (`Tpm/SevSnp/Tdx`) + measured/secure boot policy; confidential computing (SEV-SNP/TDX) as differentiator.
- Porter phases: snapshot/teleport + hibernate after basic lifecycle; attestation/confidential as Phase 2.

## 9. What NOT to take

- Cloud Hypervisor specifics (Porter = Firecracker), Rust crate structure, git-based snapshots, plaintext VXLAN, root daemon, v1.0-only compose, TypeScript dashboard, vk8s CRI shim (optional/lowest).
