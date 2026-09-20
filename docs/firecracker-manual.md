# Firecracker manual for Porter (features · how it works · handling)

> Distilled 2026-09-18 from the lost `firecracker-docs.md` (full upstream read, FC v1.16–1.17)
> + Porter `backend/internal/runtime/*.go` (`FCClient`, `Manager`, `VMManager`, `fc.go`,
> `network.go`, `mode.go`). Reconstructs the reference for later use.
> Rule: Firecracker = isolation boundary; Porter = control plane; Linux = primitives.

## 1. What Firecracker is

- One **VMM process per MicroVM**, KVM-backed, Rust, Apache-2.0 (AWS Lambda/Fargate
  lineage; Kata/Flintlock integrate it). One tenant per process. Minimalist: one
  implementation per capability; security on by default, customer cannot disable.
- Threads: **API / VMM / vCPU(s)**. API thread never in fast path; vCPUs run `KVM_RUN`
  + sync PIO/MMIO emulation. Mutation throughput ~**5 boots/transitions per host-core/sec**
  (~180 VMs/sec on 36 cores, minimal kernel + 1 vCPU + 128 MiB).
- Default machine **1 vCPU / 128 MiB** (up to 32 vCPUs design). Devices: VirtIO
  Net/Block/Vsock, entropy (virtio-rng), pmem, virtio-mem hotplug, serial console,
  i8042 (reboot only). Oversubscription ON (demand-fault paging + CPU). Per-thread seccomp.
- Releases ~2–3 mo, SemVer; API version == binary version; client X.Y.Z works on X.V.W≥Y.
- Status labels: **STABLE** (prod) / **DEV-PREVIEW** (may change, no prod) /
  **DEPRECATED** (works, removed next major) / **EOL**.

## 2. SLA / perf (SPECIFICATION.md — M5D/M6G metal, HT off)

- VMM start to API socket **≤8 CPU ms** (wall 6–60, ~12 typical). Boot to `/sbin/init`
  **≤125 ms** (`InstanceStart`, serial off, minimal kernel/rootfs). VMM overhead **≤5 MiB**.
  Guest CPU >95% bare metal (pending test). Net **14.5 Gbps** @≤80% core, **25 Gbps** @100%,
  **+0.06 ms** (single emulation thread — spread NICs across VMs to scale). Storage up to
  **1 GiB/s** @≤70% core. Never crashes/halting once started (enforced). Perf headers
  partly `[integration test pending]` — treat as aspiration.

## 3. Security floor (MUST for multi-tenant)

- **Jailer** (primary): `--id` (≤64 alnum+hyphen), `--exec-file`, `--uid/--gid` (dedicated
  non-root per instance); chroot `<base>/<bin>/<id>/root` (binary copied, no shared mem);
  mount-ns + bind + pivot_root + chroot; mknod `/dev/net/tun` + `/dev/kvm` in-jail;
  cgroups v1 (default) / v2 (`--cgroup-version 2`); `--new-pid-ns` (child = pseudo-init;
  without it killing jailer does NOT kill child); `--netns`; same-version static musl only.
  Operator places disks/kernels/pipes in jail + cleans cgroups.
- **Seccomp** default-on (release): per-thread filters (VMM/API/vCPU) from
  `resources/seccomp`; `--seccomp-filter` overrides (BPF, misconfig kills proc);
  `--no-seccomp` prototyping only. Debug/GNU builds = no filters.
- Host: **serial console OFF** (`8250.nr_uarts=0`, guest stdout = unbounded host mem;
  aarch64 restore 3→8.5 ms with serial), `quiet loglevel=1`; log/metric pipes bounded
  (journald/logrotate); external watchdog (SIGSEGV/SIGSYS handlers not async-safe);
  `kvm-pit` kthread into VM cgroup; **SMT off, KSM off, no swap**, ECC+TRR (Rowhammer);
  **guest egress filtered on host, drop IMDS** (`FORWARD -i tap+ -d 169.254.169.254 -j DROP`);
  patch microcode+kernel (FC can't fix HW vulns); ARM `CNTPCT` needs host ≥6.4;
  cgroupsv2 `favordynmods` or `kvm nx_huge_pages=never` for the 6.1 cgroup regression;
  jailer limits (`fsize`, `no-file` 4096, `cpu.shares`, `cfs_period/quota`).
- Guest drivers are paravirt best-effort (compromised driver breaks balloon/mem guarantees
  but PFNs validated — memory never leaks in/out; `MAP_PRIVATE|MAP_ANONYMOUS`).

## 4. Devices & memory (API matrix)

- **Block** (`/drives/{id}`, `/dev/vd<x>`, first `is_root_device` → `/dev/vda`; `PATCH`
  path/rate-limiter live): `cache_type` Unsafe (default, no FLUSH) vs Writeback (FLUSH→fsync);
  `io_engine` Sync (default) vs **Async (DEV-PREVIEW**, io_uring, host ≥5.10.51, ~110 ms
  create, 1.5–3× reads / 30× IOPS NVMe); `discard` off (writable+Sync only); `blk_size`
  512 (default)…4096. Snapshot of vhost-user VMs unsupported. PARTUUID boot supported.
  **vhost-user block = DEV-PREVIEW** (backend UDS, `memfd+MAP_SHARED`, no snapshot, backend
  rate-limits). Hotplug block/pmem/net on running VM = DEV-PREVIEW (`--enable-pci`,
  guest `echo 1 > /sys/bus/pci/rescan`, unplug `…/devices/<BDF>/remove` first).
- **Balloon** (install-only amount, `PATCH` target+polling; `deflate_on_oom`,
  `stats_polling_interval_s`; `Out of puff!`+0.2 s retry; stats incl. Linux ≥6.12
  OOM/stall/scan/reclaim): `free_page_reporting` stable, **`free_page_hinting` DEV-PREVIEW**
  (spec race). Traditional balloon = 4K reports (can't reclaim hugepage RSS, can cap).
- **virtio-mem hotplug** (x86 ≥5.16, ARM ≥5.18, `CONFIG_VIRTIO_MEM=y`; pre-boot
  `total/block(≥2M)/slot(≥128M)`; `PATCH requested` async; `memhp_default_state=
  online_movable` for removal; 64 B per 4K boot mem w/o `memmap_on_memory`; sparse holes
  in snapshots; NOT on restored VMs).
- **PMEM** (`/dev/pmem<N>`, any-size file padded to 2 MB anon; DAX bypasses page cache;
  flush = `msync(MS_SYNC)` whole region — bandwidth/ops buckets correlated; never share
  one file across VMs (side channel); read-only writes crash ARM / warn x86).
- **virtio-rng**: max one per VM (`/entropy`, `aws-lc-rs`, guest `CONFIG_HW_RANDOM_VIRTIO=y`,
  rate-limitable — strongly recommended, early-boot entropy starves without it).
- **Hugepages** (`/machine-config`): None (4K) / Transparent (THP `madvise`, mult of 2 MB) /
  2M (hugetlbfs, ~50% faster boot, needs UFFD for resume; file-backed restore errors;
  dirty-tracking forces 4K → negates benefit).
- **initrd** (`/boot-source initrd_path`, `CONFIG_BLK_DEV_INITRD=y`, `switch_root` not
  `pivot_root`, no `is_root_device` drive with initrd). **PVH** (x86, `CONFIG_PVH=y`,
  FreeBSD ≥14 default). **GDB** (`--features gdb`, blocking stub, not prod). **Fuzzing**
  builds never deploy (hardcoded ISNs, changed balloon). **Kani** model-checks guest-data
  paths (MMDS stack, rate limiter) per PR.
- Device gates: each endpoint family requires its device or `400` (e.g. net/MMDs→virtio-net,
  `entropy`→virtio-rng, `pmem`→virtio-pmem, `serial`→console, `vsock`→vsock,
  `SendCtrlAltDel`→keyboard).

## 5. CPU templates

- Static **DEPRECATED since v1.5** (C3/T2/T2A/T2CL/T2S/V1N1, removal planned) → **custom
  `/cpu-config`** (`kvm_capabilities`, `vcpu_features` ARM, `cpuid/msr_modifiers` x86,
  `reg_modifiers` ARM). Not a security boundary (ISA masking ≠ execution block; KVM owns
  MSR/reg perms). Last-set wins. **Normalization** (irreducible, post-template): vendor ID,
  CLFLUSH, APIC ID, no PDCM, TSC_DEADLINE+HYPERVISOR, topology leaves. `cpu-template-helper`
  `template dump|strip|verify`, `fingerprint dump|compare` across kernel/microcode updates.

## 6. Networking (the Porter-critical section)

- Backend **TUN/TAP only, single virtqueue pair** (no MQ, no MACVTAP). Topologies: NAT
  (not LAN-exposed) / Bridge (exposed) / Namespaced NAT (clones). Smallest IPv4 link = **/30**.
  **TAP IP ≠ host subnet.** `ip_forward=1`; nft `masquerade` + forward accepts (iptables-nft
  deprecated). Guest static IP + `ip link up` + default via TAP, or cmdline
  `ip=G::T:GM::GI:off`. Scales to thousands of VMs/host (`172.16.[…]` /30 math).
  `guest_mac` optional (random else); `iface_id` management-only. IPv6 manual.
- Perf: 25 Gbps @1460 B → 18 @96 B both ways; 18 bidirectional all sizes; +0.06 ms.
- Rate limiting: per-interface `rx/tx_rate_limiter` token buckets (2 buckets each:
  ops + bandwidth: `size/one_time_burst/refill_time`), **`PATCH /network-interfaces/{id}`
  post-boot tunable**, 0-size disables; `MTU` 68–65535 (`VIRTIO_NET_F_MTU`).
- **vsock** (host↔guest, no vhost): host `CID 2`, guest `≥3`; host→guest `CONNECT PORT`
  / `OK PORT` over `uds_path`, guest→host forwards to `uds_path_<port>`; host
  `CONFIG_VHOST_VSOCK=m`, guest `CONFIG_VIRTIO_VSOCKETS=y`; **pre-boot only** (`PUT /vsock`);
  restore re-points via `vsock_override.uds_path`; `vsock_id` deprecated.
- Porter mapping (`runtime/network.go`, `vm_manager.go`, `netmgr`): deterministic
  `AllocateVMNetwork(projectID, replicaIndex, vmID)` (FNV-32 third octet, guest `.2` /
  gateway `.1`, `02:xx:…` locally-administered MAC from FNV-64, `tap<hex>` within IFNAMSIZ);
  per-project `/24` via `netmgr` (`VMManager.NetSpec`); **DIVERGENCE TO FIX**: `netmgr`
  `10.42/16` vs `runtime.NetworkManager` `172.16/16` + MAC conventions + `/24` boot-mask
  vs reference `/30` (task T6b). Boot args in `fc_client.go` (`console=ttyS0 reboot=k
  panic=1 pci=off nomodules ro root=/dev/vda` + `ip=<guest>::<gw>:255.255.255.0::eth0:off`).

## 7. Snapshots / clones / migration

- Files: **guest-mem + VM-state (+ 0…N disks, your job)**. Load = `MAP_PRIVATE` (COW,
  on-demand); mem file immutable for VM life; state file has version + CRC64 (bad CRC =
  terminate). **Full** (resumable) vs **Diff** (sparse dirtied pages, needs base;
  `track_dirty_pages` KVM log — NOT saved, re-set on load — or `mincore`, swap off).
- `PUT /snapshot/load`: `backend_type` File|Uffd, `mem_backend` (legacy `mem_file_path`
  deprecated), `track_dirty_pages/resume_vm/huge_pages(Snapshot|None|Transparent|2M,
  2M needs Uffd)`. Order: Pause/Resume/Create only after boot; Load only before boot
  (logger+metrics first ok). Resume needs **identical sys/HW + resource paths** (TAP,
  block, vsock UDS); **unstable across host kernels**; no Intel↔AMD/cross-model; ARM
  GICv2↔vGICv3 no; early-boot snapshots may crash; cgroups v1 slow restore (prefer v2).
  MMDS/metrics/log config NOT persisted.
- **Never resume one state twice** (dup IDs/RNG/tokens): VMGenID (16 B, x86+ARM, ≥5.18
  ACPI / 6.10 DT) reseeds kernel PRNG; userspace de-dups itself; `clock_realtime:true`
  (x86, host ≥5.16) advances clock. Clones share RNG — reseed (`RNDRESEEDCRNG` ≤5.18 w/
  CAP_SYS_ADMIN; VMGenID ≥5.18 with race window); delete systemd random-seed.
- `snapshot-editor`: `edit-memory rebase` (merge diff→base), `edit-vmstate remove-regs`
  (ARM), `info-vmstate`. Clones: netns per clone (`jailer --netns`) + veth + NAT/DNAT +
  ARP flush (or `NetworkOverride iface→host_dev` non-jailer). UFFD: kernel per-page IO vs
  userspace `UFFDIO_COPY` handler (5.10 syscall / 6.1 `/dev/userfaultfd` ACL, jailer
  exposes in-jail); serve balloon `UFFD_EVENT_REMOVE` as zeroes; handler crash hangs FC
  by design (needs recycle).
- Porter (`fc_client.go`, `fc.go`, `manager.go`): `CreateSnapshot` (`PUT /snapshot/create`
  Full → `<id>.snap/.mem`); `LoadSnapshot` **NOT IMPLEMENTED** (T8/G3 work);
  `StopVM` = `SendCtrlAltDel` → wait 5 s → `Kill`; `Manager.Boot/Stop/Restart/Delete`
  (resource.Replica) vs `VMManager.Boot/Stop/Snapshot/Restore` (types.VM + `netmgr.BootSpec`);
  `Exec` = explicit error → guestagent-over-vsock path.

## 8. MMDS / observability / API

- **MMDS V2** (IMDSv2-like, V1 deprecated): mutable JSON at `169.254.169.254`
  (overridable) via in-VMM Dumbo TCP/IPv4; pre-boot attach NIC → `PUT /mmds/config`;
  guest route to MMDS IP; `PUT` replace / `PATCH` merge (RFC 7396); host `GET`; token via
  `/latest/api/token` + `X-metadata-token-ttl-seconds` (1–21600 s); JSON or IMDS (`Accept`,
  `imds_compat`); NOT persisted across snapshots; cap 51200 B. Never use MMDS stack to
  filter host IMDS — host firewall does that. Porter: guest platform metadata
  (hostname/mesh-IP/per-VM keys) via MMDS.
- **Logger**: single, once (`PUT /logger`: `log_path/level/show_level/show_log_origin`;
  CLI `--log-path/--level Warning/--show-level/--show-log-origin`); human lines to file
  or named pipe; immutable; prod builds disable serial (leak).
- **Metrics**: JSON lines, auto-flush **60 s** + `FlushMetrics`; NOT restored; families
  `api_server/balloon/block/deprecated_api/entropy/get|patch|put_api_requests/i8042/
  latencies_us/logger/mmds/net/rtc/seccomp/signals/uart/vcpu/vmm/vsock/vhost_user_block`;
  per-device `block_<id>/net_<iface>`; units in key (`_bytes/_ms/_us` else count);
  optional `emit_id/properties`. Porter: consume stream + own PG event/audit truth.
- **Tracing** (`--features tracing`): entry/exit logs, release-absent, >10× slowdown —
  never prod. **Actions**: `InstanceStart` (once), `FlushMetrics`, `SendCtrlAltDel`.
- **Kernel policy (AUTHORITATIVE)**: host+guest **only 6.18** in support (FC ≥v1.16/1.16.1,
  EOL 2028-05/06); 5.10 + 6.1 EOL. Guest min configs (initrd `VIRTIO_MMIO` ARM /
  `KVM_GUEST` x86 + `BLK_DEV_INITRD`; block `VIRTIO_BLK` + x86 `ACPI/PCI`; net
  `VIRTIO_NET/VSOCKETS`). Default cmdline FC passes:
  `reboot=k panic=1 nomodule 8250.nr_uarts=0 i8042.noaux i8042.nomux i8042.dumbkbd
  swiotlb=noforce` (+ `pci=off` iff no PCI). ACPI replaces MPTable/cmdline-VirtIO
  (legacy deprecated). Stale docs listing 5.10/6.1/4.20 = untrusted; trust kernel-policy.
- Porter handling today: `FCClient` (unix-socket `PUT`, 10 s timeout, ≥400 = error;
  `bin --api-sock <sock> --config-file /dev/null`, 10 s socket wait, then
  `/boot-source → /drives/rootfs → /network-interfaces/eth0 → /machine-config →
  /actions InstanceStart`); `Mode` = `direct` only (`ParseMode` validates at boot);
  `Close()` kills tracked procs. Gaps (do not fake): snapshot load, exec (vsock agent),
  pause/resume/reboot endpoints, balloon/memdial/pmem/vsock wiring, jailer/seccomp/cgroup
  confinement, metrics/log consumption, MMDS config.

## 9. Porter runbook (handling)

- Preflight (`porter doctor`): OS/arch, kernel = 6.18, `/dev/kvm` RW, cgroups v2,
  FC binary + jailer, BuildKit, TAP/bridge + forwarding + NAT, IPAM, gateway/DNS/TLS,
  storage, PG — each `check/status/severity/observed/expected/cause/action/evidence`.
- Boot: enroll node → allocate net (fix T6b first) → kernel+rootfs (digest-pinned) →
  jail (uid/gid/cgroups/netns) → drives → NIC → vsock → metadata → `InstanceStart` →
  health → gateway route → status/events/audit. Privilege failures = explicit conditions.
- Operate: `stop` (graceful → kill 5 s), `snapshot` (full; diff later), `restore` (same
  host/shape/paths; reseed RNG; de-dup), `pause/resume` (T8), rate-limit `PATCH` live,
  vsock exec/logs (never SSH), MMDS metadata, metrics poll + PG events.
- Fail: any step → condition → retry-if-retryable → reconcile partial → idempotent
  cleanup → Failed. Controller crash = restart → list pending → reconcile (no mem state).
  Node loss = Unhealthy → stop placement → disruption-budget → recreate → net/volumes →
  health → traffic → audit.
