#!/usr/bin/env bash
# ============================================================================
#  Porter host provisioning — part 1: containerd + devmapper snapshotter + CNI
#  directories. Idempotent; safe to re-run.
#
#   sudo bash deploy/host/01-containerd.sh
#
#  Installs/confirms containerd is present, builds a device-mapper thin pool
#  (porter-pool) for image/rootfs layering, writes /etc/containerd/config.toml
#  with the devmapper snapshotter + a registered `aws.firecracker` task
#  runtime, and creates the CNI plugin/conf directories.
#
#  Env overrides:
#    STATE_DIR   root dir that holds the thin-pool files   (default /var/lib/porter)
#    POOL_SIZE   size of the thin data pool                (default 10G)
# ============================================================================
set -euo pipefail

[ "$(id -u)" -eq 0 ] || { echo "ERROR: run as root: sudo bash deploy/host/01-containerd.sh" >&2; exit 1; }

STATE_DIR=${STATE_DIR:-/var/lib/porter}
POOL_SIZE=${POOL_SIZE:-10G}
CONTAINERD_CONF=/etc/containerd/config.toml
CNI_BIN_DIR=/opt/cni/bin
CNI_CONF_DIR=/etc/cni/net.d

command -v containerd >/dev/null 2>&1 || { echo "ERROR: containerd not found — install it (apt install containerd / dnf install containerd.io)." >&2; exit 1; }
command -v dmsetup   >/dev/null 2>&1 || { echo "ERROR: dmsetup not found — install device-mapper tools." >&2; exit 1; }
command -v losetup   >/dev/null 2>&1 || { echo "ERROR: losetup not found — install util-linux." >&2; exit 1; }

mkdir -p /etc/containerd /var/lib/containerd /var/lib/containerd-devmapper \
         "$CNI_BIN_DIR" "$CNI_CONF_DIR" "$STATE_DIR"

# --- Thin-pool backing files (create once, reuse on re-run) ------------------
THIN_DATA="$STATE_DIR/thin-data"
THIN_META="$STATE_DIR/thin-meta"
if [ ! -f "$THIN_DATA" ]; then
  truncate -s "$POOL_SIZE" "$THIN_DATA"
  truncate -s 512M          "$THIN_META"
  echo "created thin pool backing files ($POOL_SIZE data + 512M meta)"
else
  echo "thin pool backing files already exist — reusing"
fi

# --- (Re)attach loop devices and (re)create the device-mapper pool ----------
LOOP_DATA=$(losetup --find --show "$THIN_DATA")
LOOP_META=$(losetup --find --show "$THIN_META")
# Drop any stale pool from a previous install (idempotent).
dmsetup remove porter-pool 2>/dev/null || true
dmsetup create porter-pool \
  --table "0 $(blockdev --getsz "$LOOP_DATA") thin-pool $LOOP_META $LOOP_DATA 128 32768 1 skip_block_zeroing"
echo "device-mapper thin pool 'porter-pool' ready"

# --- containerd config: devmapper snapshotter + aws.firecracker runtime -----
cat > "$CONTAINERD_CONF" <<'EOF'
version = 2
root = "/var/lib/containerd"
state = "/run/containerd"

[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    disable_snapshot_annotations = true
  [plugins."io.containerd.snapshotter.v1.devmapper"]
    root_path = "/var/lib/containerd-devmapper"
    pool_name = "porter-pool"
    base_image_size = "8GB"
    async_remove = true
  [plugins."io.containerd.runtime.v2.task"]
    [plugins."io.containerd.runtime.v2.task"."aws.firecracker"]
      config_path = "/etc/containerd/firecracker-runtime.json"
EOF
echo "wrote $CONTAINERD_CONF (devmapper snapshotter + aws.firecracker runtime registered)"

systemctl enable containerd 2>/dev/null || true
systemctl restart containerd
echo "containerd restarted (socket: /run/containerd/containerd.sock)"
echo
echo "Next: sudo bash deploy/host/02-shim.sh"