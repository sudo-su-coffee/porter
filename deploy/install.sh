#!/usr/bin/env bash
# ============================================================================
#  Porter DEV installer — local dev/testing variant of backend/deploy/install.sh
#
#   bash backend/deploy/dev-install.sh
#
#  Differences from the production installer:
#   - PostgreSQL runs in Docker (named container, persistent volume), not on
#     the host.
#   - Go binary is built and run from a project-local ./.dev directory, not
#     /usr/local/bin — no system-wide install of the porter binary itself.
#   - firecracker / containerd / CNI still install to real system paths,
#     because that's the thing under test — but every step checks state
#     first and skips if already satisfied. Nothing is blindly reinstalled.
#   - No systemd unit. You run porter in the foreground (or via `dev-install.sh
#     run`) so you can see logs / iterate quickly.
#   - Root is only required for the containerd/firecracker/CNI/KVM steps.
#     Those are skipped (with a clear warning) if not run as root, so you can
#     still use this script to just stand up Postgres + build the binary.
#
#  Usage:
#    bash backend/deploy/dev-install.sh          # install / check everything
#    bash backend/deploy/dev-install.sh run       # build (if needed) + run porter
#    bash backend/deploy/dev-install.sh status    # print state of every component
#    bash backend/deploy/dev-install.sh nuke      # tear down dev state (asks first)
#
#  Env overrides: PG_PORT, PG_PASSWORD, POOL_SIZE, FIRECRACKER_VERSION,
#                 KERNEL_PATH, PORTER_API_TOKEN
# ============================================================================
set -euo pipefail

# ---------------------------------------------------------------------------
# Paths — everything dev-related lives under the repo, not system dirs.
# ---------------------------------------------------------------------------
BACKEND_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEV_DIR="$BACKEND_DIR/.dev"
BIN_DIR="$DEV_DIR/bin"
STATE_DIR="$DEV_DIR/state"          # thin-data / thin-meta / kernel live here
LOG_DIR="$DEV_DIR/logs"
CONF_DIR="$DEV_DIR/conf"
CNI_BIN_DIR="/opt/cni/bin"          # still a system path — CNI plugins expect it
CNI_CONF_DIR="/etc/cni/net.d"       # containerd/CNI don't support relocating this cleanly

PG_CONTAINER=porter-dev-postgres
PG_VOLUME=porter-dev-pgdata
PG_PORT=${PG_PORT:-5432}
PG_USER=porter
PG_PASSWORD=${PG_PASSWORD:-porter}
PG_DB=porter
DB_URL="postgres://$PG_USER:$PG_PASSWORD@localhost:$PG_PORT/$PG_DB?sslmode=disable"

POOL_SIZE=${POOL_SIZE:-10G}
FIRECRACKER_VERSION=${FIRECRACKER_VERSION:-v1.16.1}
KERNEL_PATH=${KERNEL_PATH:-$STATE_DIR/vmlinux}
PORTER_API_TOKEN=${PORTER_API_TOKEN:-dev-token-$(openssl rand -hex 8 2>/dev/null || echo insecure)}

mkdir -p "$DEV_DIR" "$BIN_DIR" "$STATE_DIR" "$LOG_DIR" "$CONF_DIR"

c()    { printf '\n\033[1;34m==> %s\033[0m\n' "$1"; }
ok()   { printf '\033[0;32m    ok: %s\033[0m\n' "$1"; }
skip() { printf '\033[0;36m    skip (already present): %s\033[0m\n' "$1"; }
warn() { printf '\033[0;33m    warn: %s\033[0m\n' "$1"; }
die()  { printf '\033[0;31m    FAIL: %s\033[0m\n' "$1" >&2; exit 1; }

IS_ROOT=0
[ "$(id -u)" -eq 0 ] && IS_ROOT=1

IS_WSL=0
grep -qi microsoft /proc/version 2>/dev/null && IS_WSL=1

# Running from /mnt/<drive>/... under WSL2 means the repo sits on DrvFs
# (9p over the Windows filesystem), which has caused AccessDenied /
# cache-rename failures on Go builds and can behave oddly under dmsetup/
# loop devices too. Not fatal, just a heads-up.
if [ "$IS_WSL" -eq 1 ] && [[ "$BACKEND_DIR" == /mnt/* ]]; then
  warn "repo is on a DrvFs mount ($BACKEND_DIR) — Go build caches and loop/dm"
  warn "  operations are known to be flaky here. If you hit odd AccessDenied"
  warn "  or rename failures, move the repo to native ext4, e.g. ~/porter-main,"
  warn "  and re-clone/copy from there instead of /mnt/d/..."
fi

# ---------------------------------------------------------------------------
# 0. KVM / WSL2 check — informational, never fatal, so Postgres+build still work.
# ---------------------------------------------------------------------------
check_kvm() {
  c "0/8  KVM availability"
  if [ -e /dev/kvm ]; then
    if [ -r /dev/kvm ] && [ -w /dev/kvm ]; then
      ok "/dev/kvm present and accessible as $(whoami)"
    else
      warn "/dev/kvm exists but isn't read/write for $(whoami)."
      warn "  fix: sudo usermod -aG kvm \$USER   (then log out/in, or: newgrp kvm)"
    fi
  else
    warn "/dev/kvm not found — microVMs will not boot."
    if [ "$IS_WSL" -eq 1 ]; then
      cat <<'EOF'
    You're in WSL2. To get /dev/kvm:
      1. On Windows: enable nested virtualization for the WSL2 VM. In an
         elevated PowerShell:
           Set-VMProcessor -VMName <WSLDistroVM> -ExposeVirtualizationExtensions $true
         (or, simpler on Win11 23H2+: nested virt is on by default if your
         host CPU/BIOS has VT-x/AMD-V + "Virtualization" enabled in BIOS and
         Hyper-V/Core Isolation isn't blocking it).
      2. Inside WSL2:
           sudo modprobe kvm
           sudo modprobe kvm_intel   # or kvm_amd on AMD hosts
           ls -l /dev/kvm
      3. If /dev/kvm still doesn't appear, your WSL2 kernel may need
         CONFIG_KVM built in — check with: zcat /proc/config.gz | grep CONFIG_KVM
         (Microsoft's stock WSL2 kernel ships this enabled on recent builds;
         a custom kernel may not.)
EOF
    else
      warn "  enable virtualization in BIOS, then: sudo modprobe kvm kvm_intel  (or kvm_amd)"
    fi
  fi
}

# ---------------------------------------------------------------------------
# 1. PostgreSQL in Docker — idempotent: reuse existing container/volume.
# ---------------------------------------------------------------------------
setup_postgres() {
  c "1/8  PostgreSQL (Docker, dev-local)"
  command -v docker >/dev/null 2>&1 || die "docker not found — install Docker Desktop (with WSL2 integration) or docker-ce."

  if ! docker volume inspect "$PG_VOLUME" >/dev/null 2>&1; then
    docker volume create "$PG_VOLUME" >/dev/null
    ok "created volume $PG_VOLUME"
  else
    skip "volume $PG_VOLUME"
  fi

  if docker ps --format '{{.Names}}' | grep -qx "$PG_CONTAINER"; then
    skip "container $PG_CONTAINER (already running)"
  elif docker ps -a --format '{{.Names}}' | grep -qx "$PG_CONTAINER"; then
    docker start "$PG_CONTAINER" >/dev/null
    ok "started existing container $PG_CONTAINER"
  else
    docker run -d \
      --name "$PG_CONTAINER" \
      -e POSTGRES_USER="$PG_USER" \
      -e POSTGRES_PASSWORD="$PG_PASSWORD" \
      -e POSTGRES_DB="$PG_DB" \
      -p "127.0.0.1:${PG_PORT}:5432" \
      -v "$PG_VOLUME:/var/lib/postgresql/data" \
      --restart unless-stopped \
      postgres:15-alpine >/dev/null
    ok "created + started container $PG_CONTAINER on 127.0.0.1:$PG_PORT"
  fi

  printf '    waiting for postgres to accept connections'
  for i in $(seq 1 30); do
    if docker exec "$PG_CONTAINER" pg_isready -U "$PG_USER" >/dev/null 2>&1; then
      echo; ok "postgres ready ($DB_URL)"
      return 0
    fi
    printf '.'; sleep 1
  done
  echo
  die "postgres in $PG_CONTAINER did not become ready in 30s — check: docker logs $PG_CONTAINER"
}

# ---------------------------------------------------------------------------
# 2. containerd + devmapper + aws.firecracker shim  (needs root)
#    BUGFIX vs prod script: reuse existing loop devices instead of creating
#    duplicates on every run (previously broke `dmsetup create` on re-runs).
# ---------------------------------------------------------------------------
setup_containerd() {
  c "2/8  containerd + devmapper snapshotter + aws.firecracker runtime"
  if [ "$IS_ROOT" -ne 1 ]; then
    warn "not running as root — skipping containerd/devmapper/firecracker/CNI setup."
    warn "  re-run with: sudo bash backend/deploy/dev-install.sh"
    return 0
  fi

  command -v containerd >/dev/null 2>&1 || die "containerd not found — apt install containerd"
  command -v dmsetup    >/dev/null 2>&1 || die "dmsetup not found — apt install thin-provisioning-tools"
  command -v losetup    >/dev/null 2>&1 || die "losetup not found — apt install util-linux"

  mkdir -p /var/lib/containerd-devmapper "$CNI_BIN_DIR" "$CNI_CONF_DIR"

  if [ ! -f "$STATE_DIR/thin-data" ]; then
    truncate -s "$POOL_SIZE" "$STATE_DIR/thin-data"
    truncate -s 512M         "$STATE_DIR/thin-meta"
    ok "created thin-pool backing files ($POOL_SIZE data / 512M meta) under $STATE_DIR"
  else
    skip "thin-pool backing files ($STATE_DIR/thin-data)"
  fi

  # BUGFIX: reuse an already-attached loop device for this file instead of
  # always attaching a fresh one (losetup -j finds it if it exists).
  LOOP_DATA=$(losetup -j "$STATE_DIR/thin-data" | cut -d: -f1 | head -1)
  [ -z "$LOOP_DATA" ] && LOOP_DATA=$(losetup --find --show "$STATE_DIR/thin-data")
  LOOP_META=$(losetup -j "$STATE_DIR/thin-meta" | cut -d: -f1 | head -1)
  [ -z "$LOOP_META" ] && LOOP_META=$(losetup --find --show "$STATE_DIR/thin-meta")
  [ -n "$LOOP_DATA" ] && [ -n "$LOOP_META" ] || die "losetup failed to attach thin-pool files — out of loop devices?"

  if dmsetup info porter-pool >/dev/null 2>&1; then
    skip "dm thin-pool 'porter-pool' (already active)"
  else
    dmsetup create porter-pool --table \
      "0 $(blockdev --getsz "$LOOP_DATA") thin-pool $LOOP_META $LOOP_DATA 128 32768 1 skip_block_zeroing"
    ok "created dm thin-pool 'porter-pool'"
  fi

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
      config_path = "$CONF_DIR/firecracker-runtime.json"
EOF
  systemctl enable containerd >/dev/null 2>&1 || true
  ok "wrote /etc/containerd/config.toml (config regenerated every run — cheap, not a reinstall)"
}

# ---------------------------------------------------------------------------
# 3. firecracker VMM binary  (needs root for /usr/local/bin, but check first)
# ---------------------------------------------------------------------------
setup_firecracker() {
  c "3/8  firecracker VMM binary"
  if command -v firecracker >/dev/null 2>&1; then
    skip "firecracker ($(command -v firecracker))"
    return 0
  fi
  if [ "$IS_ROOT" -ne 1 ]; then
    warn "firecracker missing and not root — can't install to /usr/local/bin. Skipping."
    return 0
  fi
  ARCH=$(uname -m); case "$ARCH" in x86_64) FA=x86_64;; aarch64) FA=aarch64;; *) die "unsupported arch $ARCH";; esac
  TMP=$(mktemp -d)
  FC_URL="https://github.com/firecracker-microvm/firecracker/releases/download/${FIRECRACKER_VERSION}/firecracker-${FIRECRACKER_VERSION}-${FA}.tgz"
  echo "    downloading $FC_URL"
  curl --connect-timeout 15 --max-time 300 --retry 3 -fL --progress-bar -o "$TMP/fc.tgz" "$FC_URL" \
    || die "download failed (network/DNS/proxy issue, or $FIRECRACKER_VERSION/$FA has no release asset — check $FC_URL in a browser)"
  tar -xzf "$TMP/fc.tgz" -C "$TMP"
  # BUGFIX: release assets are named firecracker-vX.Y.Z-ARCH (+ a .debug
  # symbols sibling with the same prefix), not a bare "firecracker" file.
  # `find -name firecracker` never matches anything, so $(...) expands to ""
  # and `install -m 0755 "" /usr/local/bin/firecracker` fails with
  # "install: cannot stat '': No such file or directory".
  FC_BIN=$(find "$TMP" -type f -name "firecracker-${FIRECRACKER_VERSION}-${FA}" ! -name "*.debug" | head -1)
  JAILER_BIN=$(find "$TMP" -type f -name "jailer-${FIRECRACKER_VERSION}-${FA}" ! -name "*.debug" | head -1)
  [ -n "$FC_BIN" ] || die "firecracker binary not found in downloaded archive — asset layout may have changed, check: tar -tzf $TMP/fc.tgz"
  install -m 0755 "$FC_BIN" /usr/local/bin/firecracker
  [ -n "$JAILER_BIN" ] && install -m 0755 "$JAILER_BIN" /usr/local/bin/jailer
  rm -rf "$TMP"
  ok "installed firecracker $FIRECRACKER_VERSION"
}

# ---------------------------------------------------------------------------
# 4. shim runtime config
#    BUGFIX vs prod script: "jailer": false for dev — matches the actual
#    demo-first / no-jailer-for-solo-use approach (GoCloud precedent), and
#    avoids a hard failure when jailer isn't installed. Flip to true + install
#    jailer if you specifically want to test the jailed path.
# ---------------------------------------------------------------------------
setup_shim_config() {
  c "4/8  shim runtime config + kernel path"
  FIRECRACKER_BIN=$(command -v firecracker || echo /usr/local/bin/firecracker)
  cat > "$CONF_DIR/firecracker-runtime.json" <<EOF
{
  "firecracker_binary_path": "$FIRECRACKER_BIN",
  "kernel_image_path": "$KERNEL_PATH",
  "kernel_args": "console=ttyS0 noapic reboot=k panic=1 pci=off nomodules rw",
  "default_vcpu_count": 1,
  "default_mem_size_mib": 256,
  "snapshotter": "devmapper",
  "jailer": false,
  "cpu_template": "T2",
  "enable_metadata": false
}
EOF
  ok "wrote $CONF_DIR/firecracker-runtime.json (jailer=false for dev)"
  if [ -f "$KERNEL_PATH" ]; then
    ok "kernel present at $KERNEL_PATH"
  else
    warn "no kernel at $KERNEL_PATH yet — run: porter kernel set <path|URL> --kernel-dir=$STATE_DIR"
  fi
  [ "$IS_ROOT" -eq 1 ] && command -v containerd >/dev/null 2>&1 && systemctl restart containerd 2>/dev/null || true
}

# ---------------------------------------------------------------------------
# 5. CNI bridge + NAT
#    BUGFIX vs prod script: bridge gets a real host IP (172.20.0.1/16), not
#    the bare network address. Also actually fetches tc-redirect-tap instead
#    of silently depending on it being present.
# ---------------------------------------------------------------------------
setup_cni() {
  c "5/8  CNI bridge + NAT (microVM networking)"
  if [ "$IS_ROOT" -ne 1 ]; then
    warn "not root — skipping CNI/bridge/NAT setup."
    return 0
  fi

  if [ -x "$CNI_BIN_DIR/bridge" ] && [ -x "$CNI_BIN_DIR/host-local" ]; then
    skip "base CNI plugins ($CNI_BIN_DIR)"
  else
    ARCH=$(uname -m); case "$ARCH" in x86_64) CA=amd64;; aarch64) CA=arm64;; *) die "unsupported arch";; esac
    TMP=$(mktemp -d)
    CNI_URL="https://github.com/containernetworking/plugins/releases/download/v1.4.0/cni-plugins-linux-$CA-v1.4.0.tgz"
    echo "    downloading $CNI_URL"
    curl --connect-timeout 15 --max-time 300 --retry 3 -fL --progress-bar -o "$TMP/cni.tgz" "$CNI_URL" \
      || die "download failed (network/DNS/proxy issue) — check $CNI_URL in a browser"
    mkdir -p "$CNI_BIN_DIR"
    tar -xzf "$TMP/cni.tgz" -C "$CNI_BIN_DIR"
    rm -rf "$TMP"
    ok "installed base CNI plugins"
  fi

  # BUGFIX: tc-redirect-tap is NOT in the standard plugins release — it's
  # firecracker-go-sdk's own binary, and AWS has no official prebuilt release
  # for it (see awslabs/tc-redirect-tap#6). Previously this ran
  # `go install .../tc-redirect-tap@latest`, which is out of place here: a
  # floating version pulled into the shared Go module cache, unlike every
  # other tool in this script (a plain binary download into a system dir).
  # Fixed to match that pattern: a curl'd prebuilt binary first, falling
  # back to an isolated source build that never touches your project's
  # go.mod or ~/go module cache.
  if [ -x "$CNI_BIN_DIR/tc-redirect-tap" ]; then
    skip "tc-redirect-tap"
  else
    ARCH=$(uname -m); case "$ARCH" in x86_64) TA="";; aarch64) TA="-arm64";; *) TA="";; esac
    TCR_RELEASE=2022-04-01-1337
    TCR_URL="https://github.com/alexellis/tc-tap-redirect-builder/releases/download/${TCR_RELEASE}/tc-redirect-tap${TA}"
    echo "    downloading prebuilt tc-redirect-tap: $TCR_URL"
    if curl --connect-timeout 15 --max-time 60 --retry 2 -fL -o "$CNI_BIN_DIR/tc-redirect-tap" "$TCR_URL" 2>/tmp/tc-redirect-tap-build.log; then
      chmod 0755 "$CNI_BIN_DIR/tc-redirect-tap"
      ok "installed prebuilt tc-redirect-tap"
    elif command -v go >/dev/null 2>&1; then
      warn "prebuilt binary unavailable — falling back to an isolated source build (not touching your go.mod)"
      TMP=$(mktemp -d)
      if git clone --depth 1 https://github.com/firecracker-microvm/firecracker-go-sdk.git "$TMP/src" >>/tmp/tc-redirect-tap-build.log 2>&1 \
        && ( cd "$TMP/src/cni/cmd/tc-redirect-tap" \
             && GOPATH="$TMP/gopath" GOCACHE="$TMP/gocache" GOFLAGS=-mod=mod \
                go build -o "$TMP/tc-redirect-tap" . ) >>/tmp/tc-redirect-tap-build.log 2>&1
      then
        install -m 0755 "$TMP/tc-redirect-tap" "$CNI_BIN_DIR/tc-redirect-tap"
        ok "built + installed tc-redirect-tap (isolated build — did not touch $BACKEND_DIR/go.mod or your module cache)"
      else
        warn "isolated source build also failed (see /tmp/tc-redirect-tap-build.log) — install manually into $CNI_BIN_DIR"
      fi
      rm -rf "$TMP"
    else
      warn "prebuilt binary download failed and go not found — install tc-redirect-tap manually into $CNI_BIN_DIR"
    fi
  fi

  ip link add porter0 type bridge 2>/dev/null || true
  # BUGFIX: assign a usable host address inside the subnet, not the network
  # address itself.
  ip addr show dev porter0 | grep -q '172.20.0.1/16' || ip addr add 172.20.0.1/16 dev porter0 2>/dev/null || true
  ip link set porter0 up || true

  INTERNET_IF=$(ip route | awk '/default/ {print $5; exit}')
  if [ -n "$INTERNET_IF" ]; then
    iptables -t nat -C POSTROUTING -s 172.20.0.0/16 -o "$INTERNET_IF" -j MASQUERADE 2>/dev/null \
      || iptables -t nat -A POSTROUTING -s 172.20.0.0/16 -o "$INTERNET_IF" -j MASQUERADE
  else
    warn "no default route found — skipping NAT rule (fine if this host has no internet egress requirement)"
  fi
  sysctl -w net.ipv4.ip_forward=1 >/dev/null || true

  cat > "$CNI_CONF_DIR/porter.conflist" <<'EOF'
{ "cniVersion": "0.4.0", "name": "porter", "plugins": [ { "type": "tc-redirect-tap", "bridge": "porter0", "ipam": { "type": "host-local", "subnet": "172.20.0.0/16" } } ] }
EOF
  ok "CNI ready — bridge porter0 @ 172.20.0.1/16"
}

# ---------------------------------------------------------------------------
# 6. Build Porter binary — LOCAL only, no /usr/local/bin install.
# ---------------------------------------------------------------------------
build_porter() {
  c "6/8  Build Porter binary (local)"
  command -v go >/dev/null 2>&1 || die "go not found — install Go 1.25+ (https://go.dev/dl)"

  SRC_HASH=$(find "$BACKEND_DIR" -name '*.go' -newer "$BIN_DIR/porter" 2>/dev/null | head -1)
  if [ -x "$BIN_DIR/porter" ] && [ -z "$SRC_HASH" ]; then
    skip "$BIN_DIR/porter (no .go files newer than existing binary)"
    return 0
  fi

  ( cd "$BACKEND_DIR" && go build -trimpath \
      -ldflags "-X main.Version=$(git -C "$BACKEND_DIR" describe --tags --always 2>/dev/null || echo dev-$(date +%Y%m%d))" \
      -o "$BIN_DIR/porter" ./cmd/porter )
  ok "built $BIN_DIR/porter"
}

# ---------------------------------------------------------------------------
# 7. Config
# ---------------------------------------------------------------------------
write_config() {
  c "7/8  Config"
  cat > "$CONF_DIR/porter.toml" <<EOF
# generated by dev-install.sh — regenerated on every run, don't hand-edit
[database]
url = "$DB_URL"

[server]
listen = "127.0.0.1:8080"
api_token = "$PORTER_API_TOKEN"

[firecracker]
runtime_config = "$CONF_DIR/firecracker-runtime.json"
kernel_path = "$KERNEL_PATH"
state_dir = "$STATE_DIR"

[logging]
dir = "$LOG_DIR"
level = "debug"
EOF
  ok "wrote $CONF_DIR/porter.toml (api_token=$PORTER_API_TOKEN)"
}

# ---------------------------------------------------------------------------
# 8. Summary
# ---------------------------------------------------------------------------
print_status() {
  c "8/8  Status"
  echo "  KVM         : $([ -e /dev/kvm ] && echo OK || echo MISSING)"
  echo "  docker pg   : $(docker inspect -f '{{.State.Status}}' "$PG_CONTAINER" 2>/dev/null || echo "not created")"
  echo "  containerd  : $(systemctl is-active containerd 2>/dev/null || echo "n/a (needs root)")"
  echo "  firecracker : $(command -v firecracker || echo MISSING)"
  echo "  tc-redirect : $([ -x "$CNI_BIN_DIR/tc-redirect-tap" ] && echo OK || echo MISSING)"
  echo "  kernel      : $([ -f "$KERNEL_PATH" ] && echo present || echo "MISSING -> $BIN_DIR/porter kernel set")"
  echo "  binary      : $([ -x "$BIN_DIR/porter" ] && echo "$BIN_DIR/porter" || echo "not built")"
  echo "  config      : $CONF_DIR/porter.toml"
  echo "  db url      : $DB_URL"
  echo
  echo "Run it:   bash $0 run"
  echo "Logs:     $LOG_DIR/"
}

nuke() {
  read -r -p "This deletes .dev/ (local state, kernel, config) and the Docker postgres volume. Continue? [y/N] " ans
  [ "$ans" = "y" ] || [ "$ans" = "Y" ] || { echo "aborted"; exit 0; }
  docker rm -f "$PG_CONTAINER" >/dev/null 2>&1 || true
  docker volume rm "$PG_VOLUME" >/dev/null 2>&1 || true
  [ "$IS_ROOT" -eq 1 ] && { dmsetup remove porter-pool 2>/dev/null || true; }
  rm -rf "$DEV_DIR"
  ok "nuked dev state. System-level containerd/CNI config left in place — rerun as root to reset those too."
}

run_porter() {
  build_porter
  write_config
  [ -x "$BIN_DIR/porter" ] || die "binary not built"
  exec "$BIN_DIR/porter" -config "$CONF_DIR/porter.toml"
}

main() {
  case "${1:-install}" in
    run)    run_porter ;;
    status) print_status ;;
    nuke)   nuke ;;
    install|"")
      check_kvm
      setup_postgres
      setup_containerd
      setup_firecracker
      setup_shim_config
      setup_cni
      build_porter
      write_config
      print_status
      ;;
    *) die "unknown command: $1 (use: install | run | status | nuke)" ;;
  esac
}

main "$@"