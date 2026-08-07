#!/usr/bin/env bash
# ============================================================================
#  Porter host provisioning — part 3: CNI plugins + network config.
#  Idempotent; safe to re-run.
#
#   sudo bash deploy/host/03-cni.sh
#
#  Ensures the CNI plugin directory (/opt/cni/bin) holds the standard plugins
#  (bridge, host-local) and the firecracker-containerd `tc-redirect-tap` NIC
#  plugin, and that /etc/cni/net.d has a porter bridge + NAT conflist. It also
#  brings up the bridge and NAT so microVMs can talk to each other and out.
#
#  tc-redirect-tap is NOT published as a standalone release; if it is missing
#  we print how to build it from firecracker-containerd (netns/cni) and set
#  CONTAINERD's task cni path accordingly.
#
#  Env overrides:
#    CNI_BIN_DIR   (default /opt/cni/bin)
#    CNI_CONF_DIR  (default /etc/cni/net.d)
#    BRIDGE        (default porter0)
#    BRIDGE_SUBNET (default 172.20.0.0/16)
# ============================================================================
set -euo pipefail

[ "$(id -u)" -eq 0 ] || { echo "ERROR: run as root: sudo bash deploy/host/03-cni.sh" >&2; exit 1; }

CNI_BIN_DIR=${CNI_BIN_DIR:-/opt/cni/bin}
CNI_CONF_DIR=${CNI_CONF_DIR:-/etc/cni/net.d}
BRIDGE=${BRIDGE:-porter0}
BRIDGE_SUBNET=${BRIDGE_SUBNET:-172.20.0.0/16}
CNI_PLUGINS_VERSION=${CNI_PLUGINS_VERSION:-v1.4.0}

mkdir -p "$CNI_BIN_DIR" "$CNI_CONF_DIR"

# --- Standard CNI plugins (bridge + host-local for the conflist's IPAM) ------
if [ ! -x "$CNI_BIN_DIR/bridge" ] || [ ! -x "$CNI_BIN_DIR/host-local" ]; then
  ARCH=$(uname -m)
  case "$ARCH" in
    x86_64)  CNI_ARCH=amd64 ;;
    aarch64) CNI_ARCH=arm64 ;;
    *)       echo "ERROR: unsupported arch $ARCH" >&2; exit 1 ;;
  esac
  TMPL=$(mktemp -d)
  URL="https://github.com/containernetworking/plugins/releases/download/$CNI_PLUGINS_VERSION/cni-plugins-linux-$CNI_ARCH-$CNI_PLUGINS_VERSION.tgz"
  echo "downloading CNI plugins: $URL"
  curl -fsSL "$URL" -o "$TMPL/cni.tgz" || { echo "ERROR: CNI plugin download failed" >&2; rm -rf "$TMPL"; exit 1; }
  tar -xzf "$TMPL/cni.tgz" -C "$CNI_BIN_DIR"
  rm -rf "$TMPL"
  echo "installed standard CNI plugins into $CNI_BIN_DIR"
fi

# --- tc-redirect-tap (the microVM NIC plugin from firecracker-containerd) ----
if [ ! -x "$CNI_BIN_DIR/tc-redirect-tap" ]; then
  cat >&2 <<'EOF'
WARN: tc-redirect-tap not found in /opt/cni/bin.
  Build it from firecracker-microvm/firecracker-containerd:
    git clone https://github.com/firecracker-microvm/firecracker-containerd
    cd firecracker-containerd && make cni-plugins
    install -m 0755 bin/tc-redirect-tap /opt/cni/bin/
  then re-run this script. Until then the shim cannot attach NICs.
EOF
fi

# --- Bridge + NAT so microVMs share the host's uplink ------------------------
ip link add "$BRIDGE" type bridge 2>/dev/null || true
ip addr add "$BRIDGE_SUBNET" dev "$BRIDGE" 2>/dev/null || true
ip link set "$BRIDGE" up || true
INTERNET_IF=$(ip route | awk '/default/ {print $5; exit}')
if [ -n "$INTERNET_IF" ]; then
  iptables -t nat -C POSTROUTING -s "$BRIDGE_SUBNET" -o "$INTERNET_IF" -j MASQUERADE 2>/dev/null || \
    iptables -t nat -A POSTROUTING -s "$BRIDGE_SUBNET" -o "$INTERNET_IF" -j MASQUERADE
fi
sysctl -w net.ipv4.ip_forward=1 || true

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
echo "wrote $CNI_CONF_DIR/porter.conflist (bridge=$BRIDGE, subnet=$BRIDGE_SUBNET)"
echo "CNI ready: $CNI_BIN_DIR + $CNI_CONF_DIR"
