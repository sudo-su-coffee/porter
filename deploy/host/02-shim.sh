#!/usr/bin/env bash
# ============================================================================
#  Porter host provisioning — part 2: firecracker-containerd shim.
#  Idempotent; safe to re-run.
#
#   sudo bash deploy/host/02-shim.sh
#
#  Writes /etc/containerd/firecracker-runtime.json — the shim's JSON config:
#  firecracker binary path, the shared kernel (vmlinux), jailer/cgroup
#  settings, CVU count, and memory. The `aws.firecracker` runtime is attached
#  to containerd by 01-containerd.sh (config.toml); this script ensures the
#  config file it points at exists and reloads containerd so the runtime is
#  live.
#
#  Env overrides:
#    STATE_DIR          root dir (default /var/lib/porter)
#    KERNEL             path to the shared vmlinux (default $STATE_DIR/vmlinux)
#    FIRECRACKER_BIN    path to the firecracker VMM binary
# ============================================================================
set -euo pipefail

[ "$(id -u)" -eq 0 ] || { echo "ERROR: run as root: sudo bash deploy/host/02-shim.sh" >&2; exit 1; }

STATE_DIR=${STATE_DIR:-/var/lib/porter}
KERNEL=${KERNEL:-$STATE_DIR/vmlinux}
FIRECRACKER_BIN=${FIRECRACKER_BIN:-$(command -v firecracker || echo /usr/bin/firecracker)}
CONFIG=/etc/containerd/firecracker-runtime.json

mkdir -p /etc/containerd "$STATE_DIR"

# --- Sanity: the VMM binary + jailer must be present before we advertise the
#     runtime; otherwise containerd's runtime-v2 plugin will fail at boot time.
command -v firecracker >/dev/null 2>&1 || {
  echo "WARN: 'firecracker' binary not in PATH ($FIRECRACKER_BIN not found)."
  echo "      Install the Firecracker VMM (distro package or GitHub release) or"
  echo "      set FIRECRACKER_BIN=/path/to/firecracker, then re-run."
} >&2
command -v jailer >/dev/null 2>&1 || echo "WARN: 'jailer' not in PATH — the aws.firecracker shim usually requires it too." >&2

# --- Write the shim runtime config -------------------------------------------
cat > "$CONFIG" <<EOF
{
  "firecracker_binary_path": "$FIRECRACKER_BIN",
  "kernel_image_path": "$KERNEL",
  "kernel_args": "console=ttyS0 noapic reboot=k panic=1 pci=off nomodules rw",
  "default_vcpu_count": 1,
  "default_mem_size_mib": 256,
  "snapshotter": "devmapper",
  "jailer": true,
  "jailer_cgroup_ver": "2",
  "cpu_template": "T2",
  "enable_metadata": false
}
EOF
echo "wrote $CONFIG (snapshotter=devmapper, kernel=$KERNEL)"

# The aws.firecracker runtime is registered in /etc/containerd/config.toml by
# 01-containerd.sh; make sure that registration is present, then reload so the
# runtime goes live.
if ! grep -q 'aws.firecracker' /etc/containerd/config.toml 2>/dev/null; then
  echo "WARN: 'aws.firecracker' runtime not yet registered in /etc/containerd/config.toml."
  echo "      Run 01-containerd.sh first (it registers [plugins.\"io.containerd.runtime.v2.task\"].aws.firecracker)."
fi

systemctl restart containerd
echo "containerd restarted with aws.firecracker runtime"
echo
echo "Provision the kernel before booting VMs:"
echo "  porter kernel set <local-path|https://url>   # writes $KERNEL"
echo
echo "Next: sudo bash deploy/host/03-cni.sh"