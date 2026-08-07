#!/usr/bin/env bash
# ============================================================================
#  Porter PRODUCTION installer — run once as root on a Linux + KVM host.
#
#   sudo bash backend/deploy/install.sh
#
#  Self-contained: installs PostgreSQL, containerd + devmapper, the
#  aws.firecracker shim, the firecracker VMM, CNI, builds the binary, and
#  installs a systemd unit. PostgreSQL runs on the host — NOT in a microVM
#  (see backend/deploy/README.md for why).
#
#  Env overrides:  STATE_DIR, POOL_SIZE, KERNEL, FIRECRACKER_VERSION,
#                  PORTER_API_TOKEN, PORTER_ADMIN_PASSWORD, PORTER_DATABASE_URL.
#
#  After install:  porter kernel set <path|URL>   then   systemctl start porter
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
POOL_SIZE=${POOL_SIZE:-10G}
KERNEL=${KERNEL:-$STATE_DIR/vmlinux}
FIRECRACKER_VERSION=${FIRECRACKER_VERSION:-v1.7.0}
BACKEND_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

c()    { printf '\n\033[1;34m==> %s\033[0m\n' "$1"; }
ok()   { printf '\033[0;32m    ok: %s\033[0m\n' "$1"; }
warn() { printf '\033[0;33m    warn: %s\033[0m\n' "$1"; }
die()  { printf '\033[0;31m    FAIL: %s\033[0m\n' "$1" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || die "run as root:  sudo bash backend/deploy/install.sh"
[ -e /dev/kvm ] || warn "/dev/kvm missing — enable nested/AMD-V/Intel VT + modprobe kvm; microVMs will not boot."

DB_USER=${PORTER_DB_USER:-porter}
DB_PASSWORD=${PORTER_DB_PASSWORD:-porter}
DB_NAME=${PORTER_DB_NAME:-porter}
DB_URL=${PORTER_DATABASE_URL:-"postgres://$DB_USER:$DB_PASSWORD@localhost:5432/$DB_NAME?sslmode=disable"}

# ---------------------------------------------------------------------------
c "1/9  PostgreSQL (control-plane state — on the host, not in a microVM)"
if command -v psql >/dev/null 2>&1 && pg_isready -q 2>/dev/null; then
  sudo -u postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='$DB_USER'" | grep -q 1 || \
    sudo -u postgres psql -c "CREATE ROLE $DB_USER LOGIN PASSWORD '$DB_PASSWORD';"
  sudo -u postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='$DB_NAME'" | grep -q 1 || \
    sudo -u postgres createdb -O "$DB_USER" "$DB_NAME"
  ok "role '$DB_USER' + db '$DB_NAME' ready"
else
  warn "postgres not running — install/start it or set PORTER_DATABASE_URL to a managed instance."
fi

# ---------------------------------------------------------------------------
c "2/9  containerd + devmapper snapshotter + aws.firecracker runtime"
command -v containerd >/dev/null 2>&1 || { echo "ERROR: containerd not found — apt install containerd" >&2; exit 1; }
command -v dmsetup >/dev/null 2>&1 || { echo "ERROR: dmsetup not found — install device-mapper tools" >&2; exit 1; }
command -v losetup >/dev/null 2>&1 || { echo "ERROR: losetup not found — install util-linux" >&2; exit 1; }
mkdir -p /etc/containerd /var/lib/containerd /var/lib/containerd-devmapper "$CNI_BIN_DIR" "$CNI_CONF_DIR" "$STATE_DIR"
if [ ! -f "$STATE_DIR/thin-data" ]; then
  truncate -s "$POOL_SIZE" "$STATE_DIR/thin-data"
  truncate -s 512M          "$STATE_DIR/thin-meta"
fi
LOOP_DATA=$(losetup --find --show "$STATE_DIR/thin-data")
LOOP_META=$(losetup --find --show "$STATE_DIR/thin-meta")
dmsetup remove porter-pool 2>/dev/null || true
dmsetup create porter-pool --table "0 $(blockdev --getsz "$LOOP_DATA") thin-pool $LOOP_META $LOOP_DATA 128 32768 1 skip_block_zeroing"
cat > /etc/containerd/config.toml <<'EOF'
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
systemctl enable containerd 2>/dev/null || true
systemctl restart containerd
ok "containerd + porter-pool + aws.firecracker runtime registered"

# ---------------------------------------------------------------------------
c "3/9  firecracker VMM binary"
if ! command -v firecracker >/dev/null 2>&1; then
  ARCH=$(uname -m); case "$ARCH" in x86_64) FA=x86_64;; aarch64) FA=aarch64;; *) die "unsupported arch $ARCH";; esac
  TMP=$(mktemp -d)
  curl -fsSL "https://github.com/firecracker-microvm/firecracker/releases/download/${FIRECRACKER_VERSION}/firecracker-${FIRECRACKER_VERSION}-${FA}.tgz" -o "$TMP/fc.tgz"
  tar -xzf "$TMP/fc.tgz" -C "$TMP"
  install -m 0755 "$(find "$TMP" -name firecracker -type f | head -1)" /usr/local/bin/firecracker
  install -m 0755 "$(find "$TMP" -name jailer -type f | head -1)" /usr/local/bin/jailer 2>/dev/null || true
  rm -rf "$TMP"
fi
ok "firecracker: $(command -v firecracker || echo MISSING — install it manually)"

# ---------------------------------------------------------------------------
c "4/9  shim runtime config + shared kernel"
FIRECRACKER_BIN=$(command -v firecracker || echo /usr/bin/firecracker)
cat > /etc/containerd/firecracker-runtime.json <<EOF
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
systemctl restart containerd
ok "wrote /etc/containerd/firecracker-runtime.json (kernel=$KERNEL)"

# ---------------------------------------------------------------------------
c "5/9  CNI bridge + NAT (microVM networking)"
if [ ! -x "$CNI_BIN_DIR/bridge" ] || [ ! -x "$CNI_BIN_DIR/host-local" ]; then
  ARCH=$(uname -m); case "$ARCH" in x86_64) CA=amd64;; aarch64) CA=arm64;; *) die "unsupported arch";; esac
  TMP=$(mktemp -d)
  curl -fsSL "https://github.com/containernetworking/plugins/releases/download/v1.4.0/cni-plugins-linux-$CA-v1.4.0.tgz" -o "$TMP/cni.tgz"
  tar -xzf "$TMP/cni.tgz" -C "$CNI_BIN_DIR"
  rm -rf "$TMP"
fi
ip link add porter0 type bridge 2>/dev/null || true
ip addr add 172.20.0.0/16 dev porter0 2>/dev/null || true
ip link set porter0 up || true
INTERNET_IF=$(ip route | awk '/default/ {print $5; exit}')
[ -n "$INTERNET_IF" ] && { iptables -t nat -C POSTROUTING -s 172.20.0.0/16 -o "$INTERNET_IF" -j MASQUERADE 2>/dev/null || iptables -t nat -A POSTROUTING -s 172.20.0.0/16 -o "$INTERNET_IF" -j MASQUERADE; }
sysctl -w net.ipv4.ip_forward=1 >/dev/null || true
cat > "$CNI_CONF_DIR/porter.conflist" <<'EOF'
{ "cniVersion": "0.4.0", "name": "porter", "plugins": [ { "type": "tc-redirect-tap", "bridge": "porter0", "ipam": { "type": "host-local", "subnet": "172.20.0.0/16" } } ] }
EOF
ok "CNI ready ($CNI_BIN_DIR + $CNI_CONF_DIR); tc-redirect-tap must be installed separately"

# ---------------------------------------------------------------------------
c "6/9  Build Porter binary"
command -v go >/dev/null 2>&1 || die "go not found — install Go 1.25+ (https://go.dev/dl)"
( cd "$BACKEND_DIR" && go build -trimpath -ldflags "-X main.Version=$(git describe --tags --always 2>/dev/null || echo v1.0.0)" -o /usr/local/bin/porter ./cmd/porter )
ok "installed /usr/local/bin/porter"

# ---------------------------------------------------------------------------
c "7/9  Config at $PORTER_DIR"
mkdir -p "$PORTER_DIR"
if [ ! -f "$PORTER_DIR/porter.toml" ]; then
  sed -e "s|__DB_URL__|$DB_URL|g" \
      -e "s|__API_TOKEN__|${PORTER_API_TOKEN:-$(openssl rand -hex 32 2>/dev/null || echo change-me)}|g" \
      -e "s|__ADMIN_PASSWORD__|${PORTER_ADMIN_PASSWORD:-change-me}|g" \
      "$BACKEND_DIR/deploy/porter.toml" > "$PORTER_DIR/porter.toml"
  chmod 600 "$PORTER_DIR/porter.toml"
  ok "wrote $PORTER_DIR/porter.toml"
else
  ok "$PORTER_DIR/porter.toml exists — leaving as-is"
fi

# ---------------------------------------------------------------------------
c "8/9  systemd unit"
cat > /etc/systemd/system/porter.service <<'EOF'
[Unit]
Description=Porter control plane (Firecracker microVM PaaS)
After=network-online.target postgresql.service containerd.service
Wants=network-online.target

[Service]
Type=simple
User=porter
Group=porter
WorkingDirectory=/etc/porter
EnvironmentFile=-/etc/porter/porter.env
Environment=PORTER_AUTO_MIGRATE=true
ExecStart=/usr/local/bin/porter -config /etc/porter/porter.toml
Restart=on-failure
RestartSec=3
TimeoutStopSec=30
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=full
ReadWritePaths=/var/lib/porter /var/log/porter

[Install]
WantedBy=multi-user.target
EOF
useradd -r -s /usr/sbin/nologin porter 2>/dev/null || true
systemctl daemon-reload
systemctl enable porter
ok "installed + enabled porter.service"

# ---------------------------------------------------------------------------
c "9/9  Summary"
echo "  KVM        : $([ -e /dev/kvm ] && echo OK || echo MISSING)"
echo "  containerd : $(systemctl is-active containerd 2>/dev/null || echo stopped)  ($( [ -S /run/containerd/containerd.sock ] && echo socket-ok || echo no-socket ))"
echo "  kernel     : $([ -f "$KERNEL" ] && echo present || echo "MISSING -> run: /usr/local/bin/porter kernel set")"
echo "  porter     : /usr/local/bin/porter"
echo "  config     : $PORTER_DIR/porter.toml"
echo
echo "Next:"
echo "  1) edit $PORTER_DIR/porter.toml (api_token + admin password)"
echo "  2) /usr/local/bin/porter kernel set <path|URL>"
echo "  3) systemctl start porter   ->   dashboard http://localhost:8080"
echo "Log: $LOG"
