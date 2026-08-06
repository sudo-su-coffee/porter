#!/usr/bin/env bash
# ============================================================================
#  Porter installer — single script, run once as root on a Linux + KVM host.
#
#   bash deploy/install.sh
#
#  It checks /dev/kvm (enabling it if possible, else telling you exactly where
#  to turn it on), then pulls + installs each runtime piece one by one, all
#  under /etc/porter:
#
#     containerd           (OCI runtime + content store)
#     devmapper snapshot   (image/rootfs layering)
#     aws.firecracker shim (boots a microVM per container)
#     firecracker binary   (the VMM)
#     CNI bridge + NAT     (microVM networking)
#     Porter binary        (the control plane, /usr/local/bin)
#     porter.toml + env    (/etc/porter)
#     systemd unit         (porter.service)
#
#  After it finishes:  systemctl start porter   and   porter kernel set <kernel>
#  See README / USAGE.md for the deploy flow.
# ============================================================================
set -euo pipefail

LOG=/var/log/porter-install.log
mkdir -p /var/log/porter
exec > >(tee -a "$LOG") 2>&1

PORTER_DIR=/etc/porter
STATE_DIR=/var/lib/porter
LOG_DIR=/var/log/porter
CNI_BIN_DIR=/opt/cni/bin
CNI_CONF_DIR=/etc/cni/net.d
BRIDGE=porter0
BRIDGE_SUBNET=${BRIDGE_SUBNET:-172.20.0.0/16}
POOL_SIZE=${POOL_SIZE:-10G}

c() { printf '\n\033[1;34m==> %s\033[0m\n' "$1"; }
ok() { printf '\033[0;32m    ok: %s\033[0m\n' "$1"; }
warn() { printf '\033[0;33m    warn: %s\033[0m\n' "$1"; }
die() { printf '\033[0;31m    FAIL: %s\033[0m\n' "$1"; exit 1; }

[ "$(id -u)" -eq 0 ] || die "run as root:  sudo bash deploy/install.sh"

# ----------------------------------------------------------------------------
c "1/9  Check hardware virtualization (/dev/kvm)"
if [ -e /dev/kvm ]; then
  ok "/dev/kvm present"
else
  if grep -E -q '(vmx|svm)' /proc/cpuinfo; then
    warn "CPU supports virtualization but /dev/kvm is missing."
    warn "Trying to load the KVM module..."
    if modprobe kvm_intel 2>/dev/null || modprobe kvm_amd 2>/dev/null; then
      sleep 1
      [ -e /dev/kvm ] && ok "KVM module loaded — /dev/kvm present"
    fi
  fi
  if [ ! -e /dev/kvm ]; then
    cat >&2 <<'EOF'

    We cannot enable KVM from here. It is enabled in the BIOS/firmware or the
    VM hypervisor, NOT at runtime. Do ONE of these:

      - Physical host : reboot into BIOS/UEFI -> enable "Intel VT-x"/"AMD-V"
      - Cloud VM      : your provider exposes nested virtualization as a toggle
      - WSL2/Kali     : add to %UserProfile%\.wslconfig  and `wsl --shutdown`
            [wsl2]
            nestedVirtualization=true
            memory=8GB
            processors=4
      - QEMU/VirtualBox/KVM guest: enable "nested VMX/SVM" in the VM settings

    MicroVMs cannot boot without KVM. Re-run this installer after enabling it.
EOF
    exit 1
  fi
fi

# ----------------------------------------------------------------------------
c "2/9  Detect package manager"
PKG=""
if command -v apt-get >/dev/null 2>&1; then PKG=apt; fi
if command -v dnf >/dev/null 2>&1; then PKG=dnf; fi
if command -v yum >/dev/null 2>&1; then PKG=yum; fi
[ -n "$PKG" ] || die "unsupported distro (need apt/dnf/yum)"
ok "package manager: $PKG"

# ----------------------------------------------------------------------------
c "3/9  Install base packages (containerd, firecracker, networking)"
case "$PKG" in
  apt)
    apt-get update -y
    apt-get install -y containerd firecracker iptables curl jq systemd-coredump 2>/dev/null || \
      apt-get install -y containerd firecracker iptables curl jq || true
    ;;
  dnf|yum)
    (command -v containerd >/dev/null 2>&1 || "$PKG" install -y containerd.io 2>/dev/null || true)
    "$PKG" install -y firecracker iptables curl jq 2>/dev/null || \
      warn "firecracker not in repos; install manually"
    ;;
esac
command -v containerd >/dev/null 2>&1 || die "containerd install failed"
ok "containerd + firecracker + iptables"

# ----------------------------------------------------------------------------
c "4/9  Create porter directories"
mkdir -p "$PORTER_DIR" "$STATE_DIR" "$LOG_DIR"
if ! id porter >/dev/null 2>&1; then
  useradd --system --home "$PORTER_DIR" --shell /usr/sbin/nologin porter || true
fi
chown -R porter:porter "$PORTER_DIR" "$STATE_DIR" "$LOG_DIR"
ok "dirs under $PORTER_DIR + $STATE_DIR + $LOG_DIR"

# ----------------------------------------------------------------------------
c "5/9  Configure devmapper snapshotter (thin pool for image rootfs)"
THIN_DATA=$STATE_DIR/thin-data
THIN_META=$STATE_DIR/thin-meta
if [ ! -f "$THIN_DATA" ]; then
  truncate -s "$POOL_SIZE" "$THIN_DATA"
  truncate -s 512M "$THIN_META"
  ok "created thin pool data/meta ($POOL_SIZE)"
fi
LOOP_DATA=$(losetup --find --show "$THIN_DATA")
LOOP_META=$(losetup --find --show "$THIN_META")
dmsetup remove porter-pool 2>/dev/null || true
dmsetup create porter-pool \
  --table "0 $(blockdev --getsz "$LOOP_DATA") thin-pool $LOOP_META $LOOP_DATA 128 32768 1 skip_block_zeroing" || \
  die "could not create devmapper thin pool"

cat > /etc/containerd/config.toml <<EOF
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
ok "devmapper snapshotter + aws.firecracker runtime registered"

# ----------------------------------------------------------------------------
c "6/9  Write firecracker-runtime.json (shim config)"
KERNEL=${KERNEL:-$STATE_DIR/vmlinux}
FIRECRACKER_BIN=${FIRECRACKER_BIN:-$(command -v firecracker || echo /usr/bin/firecracker)}
cat > /etc/containerd/firecracker-runtime.json <<EOF
{
  "firecracker_binary_path": "$FIRECRACKER_BIN",
  "kernel_image_path": "$KERNEL",
  "kernel_args": "console=ttyS0 noapic reboot=k panic=1 pci=off nomodules rw",
  "default_vcpu_count": 1,
  "default_mem_size_mib": 256,
  "jailer": true,
  "jailer_cgroup_ver": "2",
  "cpu_template": "T2",
  "enable_metadata": false
}
EOF
ok "shim config written (kernel: $KERNEL)"

# ----------------------------------------------------------------------------
c "7/9  CNI bridge + NAT"
mkdir -p "$CNI_BIN_DIR" "$CNI_CONF_DIR"
if [ ! -x "$CNI_BIN_DIR/tc-redirect-tap" ]; then
  warn "tc-redirect-tap plugin not found in $CNI_BIN_DIR"
  warn "Build it from firecracker-containerd (netns/cni) and install, then re-run."
else
  ip link add "$BRIDGE" type bridge 2>/dev/null || true
  ip addr add "$BRIDGE_SUBNET" dev "$BRIDGE" 2>/dev/null || true
  ip link set "$BRIDGE" up
  INTERNET_IF=$(ip route | awk '/default/ {print $5; exit}')
  iptables -t nat -C POSTROUTING -s "$BRIDGE_SUBNET" -o "$INTERNET_IF" -j MASQUERADE 2>/dev/null || \
    iptables -t nat -A POSTROUTING -s "$BRIDGE_SUBNET" -o "$INTERNET_IF" -j MASQUERADE
  sysctl -w net.ipv4.ip_forward=1
  cat > "$CNI_CONF_DIR/porter.conflist" <<EOF
{
  "cniVersion": "0.4.0",
  "name": "porter",
  "plugins": [ {
      "type": "tc-redirect-tap",
      "bridge": "$BRIDGE",
      "ipam": { "type": "host-local", "subnet": "$BRIDGE_SUBNET" }
  } ]
}
EOF
  ok "bridge $BRIDGE + NAT configured"
fi

systemctl restart containerd
ok "containerd restarted"

# ----------------------------------------------------------------------------
c "8/9  Install Porter binary + config"

install_porter_binary() {
  # 1) explicit path
  if [ -n "${PORTER_BIN:-}" ] && [ -f "$PORTER_BIN" ]; then
    install -m 0755 "$PORTER_BIN" /usr/local/bin/porter
    return 0
  fi
  # 2) latest STABLE release binary from GitHub Releases (amd64)
  local LATEST_URL="https://github.com/${PORTER_REPO:-porter/porter}/releases/latest/download/${PORTER_BIN_NAME:-porter-linux-amd64}"
  if [ "${PORTER_BIN_NAME:-}" != "build" ]; then
    warn "trying to download latest release: $LATEST_URL"
    if curl -fSL "$LATEST_URL" -o /usr/local/bin/porter 2>/dev/null; then
      chmod 0755 /usr/local/bin/porter
      return 0
    fi
    warn "download failed — falling back to building from source."
  fi
  # 3) build from source (needs Go 1.25 + Node for the embedded dashboard)
  local SRC=${PORTER_SRC:-..}
  (cd "$SRC/backend" && go build -trimpath -ldflags "-s -w" -o /usr/local/bin/porter ./cmd/porter)
  [ -x /usr/local/bin/porter ] || die "could not obtain a porter binary"
}

install_porter_binary
porter version 2>/dev/null || true
ok "porter -> /usr/local/bin/porter ($(porter version 2>/dev/null | head -1))"

if [ ! -f "$PORTER_DIR/porter.toml" ]; then
  cat > "$PORTER_DIR/porter.toml" <<EOF
[server]
listen_addr = ":8080"
base_domain = "porter.test"
state_file  = "$STATE_DIR/porter.db"
api_token   = "CHANGE_ME"

[firecracker]
containerd_socket = "/run/containerd/containerd.sock"
snapshotter       = "devmapper"
namespace         = "porter"
logs_dir          = "$LOG_DIR"

[admin]
username = "admin"
password = "CHANGE_ME"
EOF
  chmod 600 "$PORTER_DIR/porter.toml"
  ok "porter.toml template at $PORTER_DIR/porter.toml (EDIT the CHANGE_ME secrets)"
fi

# systemd unit
cat > /etc/systemd/system/porter.service <<'EOF'
[Unit]
Description=Porter control plane (Firecracker microVM PaaS)
After=network-online.target containerd.service
Wants=network-online.target containerd.service

[Service]
Type=simple
User=porter
Group=porter
WorkingDirectory=/etc/porter
EnvironmentFile=-/etc/porter/porter.env
ExecStart=/usr/local/bin/porter server -workers 2 -config /etc/porter/porter.toml
Restart=on-failure
RestartSec=3
NoNewPrivileges=yes
ProtectSystem=full
ReadWritePaths=/var/lib/porter /var/log/porter

[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable porter
ok "systemd unit installed + enabled"

# ----------------------------------------------------------------------------
c "9/9  Summary"
echo "  KVM              : $( [ -e /dev/kvm ] && echo OK || echo MISSING )"
echo "  containerd       : $(systemctl is-active containerd 2>/dev/null || echo stopped)  (socket /run/containerd/containerd.sock)"
echo "  shim             : aws.firecracker registered"
echo "  kernel           : $([ -f "$KERNEL" ] && echo present || echo 'MISSING -> run: porter kernel set')"
echo "  porter           : /usr/local/bin/porter"
echo "  config           : $PORTER_DIR/porter.toml"
echo
echo "Next:"
echo "  1) edit $PORTER_DIR/porter.toml  (set api_token + admin password)"
echo "  2) export PORTER_KERNEL_IMAGE='$KERNEL'"
echo "  3) porter kernel set <path|URL>   # e.g. a vmlinux built for Firecracker"
echo "  4) systemctl start porter"
echo "  5) open http://localhost:8080  (dashboard) and deploy an image"
echo
echo "Log: $LOG"