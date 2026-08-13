#!/usr/bin/env bash
# Porter v1.0.0-beta-dev installer: direct Firecracker + PostgreSQL.
#
# This script intentionally does not install or configure containerd, an OCI
# runtime, a Firecracker shim, or CNI plugins. Porter owns one TAP device per
# VM and configures the official Firecracker HTTP API through per-VM Unix
# sockets. PostgreSQL is kept in a local Docker container for development only.
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BACKEND_DIR="$REPO_DIR/backend"
DEV_DIR="$REPO_DIR/.dev"
BIN_DIR="$DEV_DIR/bin"
STATE_DIR="$DEV_DIR/state"
LOG_DIR="$DEV_DIR/logs"
CONF_DIR="$DEV_DIR/conf"
PG_CONTAINER="${PG_CONTAINER:-porter-dev-postgres}"
PG_VOLUME="${PG_VOLUME:-porter-dev-pgdata}"
PG_PORT="${PG_PORT:-5432}"
PG_USER="${PG_USER:-porter}"
PG_PASSWORD="${PG_PASSWORD:-porter}"
PG_DB="${PG_DB:-porter}"
DB_URL="postgres://${PG_USER}:${PG_PASSWORD}@localhost:${PG_PORT}/${PG_DB}?sslmode=disable"
FIRECRACKER_VERSION="${FIRECRACKER_VERSION:-v1.16.1}"
FIRECRACKER_DIR="${PORTER_FIRECRACKER_DIR:-$BIN_DIR}"
FIRECRACKER_BIN="${PORTER_FIRECRACKER_BIN:-$FIRECRACKER_DIR/firecracker}"
BASE_IMAGE_DIR="${PORTER_BASE_IMAGE_DIR:-$STATE_DIR/base-images/default}"
KERNEL_PATH="${PORTER_KERNEL_PATH:-${PORTER_KERNEL_IMAGE:-$BASE_IMAGE_DIR/vmlinux}}"
ROOTFS_PATH="${PORTER_ROOTFS_PATH:-${PORTER_ROOTFS_IMAGE:-$BASE_IMAGE_DIR/rootfs.ext4}}"
BASE_IMAGE_URL="${PORTER_BASE_IMAGE_URL:-}"
BASE_IMAGE_SHA256="${PORTER_BASE_IMAGE_SHA256:-}"
GITHUB_REPOSITORY="${PORTER_GITHUB_REPOSITORY:-sudo-su-coffee/porter}"
RELEASE_TAG="${PORTER_RELEASE_TAG:-v1.0.0-beta-dev}"
BASE_IMAGE_ASSET="${PORTER_BASE_IMAGE_ASSET:-}"
SOCKET_DIR="${FIRECRACKER_SOCKET_DIR:-$STATE_DIR/sockets}"
PORTER_BOOTSTRAP_ADMIN_PASSWORD="${PORTER_BOOTSTRAP_ADMIN_PASSWORD:-}"
PORTER_SECRET_KEY="${PORTER_SECRET_KEY:-}"

mkdir -p "$BIN_DIR" "$FIRECRACKER_DIR" "$BASE_IMAGE_DIR" "$STATE_DIR/images" "$STATE_DIR/custom" "$SOCKET_DIR" "$LOG_DIR" "$CONF_DIR"

if [ -z "$BASE_IMAGE_URL" ] && [ -n "$BASE_IMAGE_ASSET" ]; then
  BASE_IMAGE_URL="https://github.com/${GITHUB_REPOSITORY}/releases/download/${RELEASE_TAG}/${BASE_IMAGE_ASSET}"
fi
if [ -n "$BASE_IMAGE_URL" ]; then
  case "$BASE_IMAGE_URL" in
    https://github.com/*/releases/download/*) ;;
    *) printf '\033[0;31m    FAIL: base image source must be a GitHub Release URL; AWS and arbitrary mirrors are not supported\033[0m\n' >&2; exit 1 ;;
  esac
fi

c() { printf '\n\033[1;34m==> %s\033[0m\n' "$1"; }
ok() { printf '\033[0;32m    ok: %s\033[0m\n' "$1"; }
skip() { printf '\033[0;36m    skip: %s\033[0m\n' "$1"; }
warn() { printf '\033[0;33m    warn: %s\033[0m\n' "$1"; }
die() { printf '\033[0;31m    FAIL: %s\033[0m\n' "$1" >&2; exit 1; }

IS_ROOT=0
[ "$(id -u)" -eq 0 ] && IS_ROOT=1

check_kvm() {
  c "1/7  KVM availability"
  if [ -e /dev/kvm ] && [ -r /dev/kvm ] && [ -w /dev/kvm ]; then
    ok "/dev/kvm is readable and writable"
  elif [ -e /dev/kvm ]; then
    warn "/dev/kvm exists but is not readable/writable for $(whoami)"
  else
    warn "/dev/kvm is missing; the API can run, but direct microVMs cannot boot"
  fi
}

setup_postgres() {
  c "2/7  PostgreSQL development database"
  command -v docker >/dev/null 2>&1 || die "docker is required for the dev database; set up PostgreSQL separately for production"
  if ! docker volume inspect "$PG_VOLUME" >/dev/null 2>&1; then
    docker volume create "$PG_VOLUME" >/dev/null
    ok "created volume $PG_VOLUME"
  else
    skip "volume $PG_VOLUME"
  fi
  if docker ps --format '{{.Names}}' | grep -qx "$PG_CONTAINER"; then
    skip "container $PG_CONTAINER"
  elif docker ps -a --format '{{.Names}}' | grep -qx "$PG_CONTAINER"; then
    docker start "$PG_CONTAINER" >/dev/null
    ok "started container $PG_CONTAINER"
  else
    docker run -d --name "$PG_CONTAINER" \
      -e POSTGRES_USER="$PG_USER" -e POSTGRES_PASSWORD="$PG_PASSWORD" -e POSTGRES_DB="$PG_DB" \
      -p "127.0.0.1:${PG_PORT}:5432" -v "$PG_VOLUME:/var/lib/postgresql/data" \
      --restart unless-stopped postgres:15-alpine >/dev/null
    ok "created PostgreSQL container on 127.0.0.1:$PG_PORT"
  fi
  for _ in $(seq 1 30); do
    if docker exec "$PG_CONTAINER" pg_isready -U "$PG_USER" >/dev/null 2>&1; then
      ok "PostgreSQL ready at $DB_URL"
      return
    fi
    sleep 1
  done
  die "PostgreSQL did not become ready; inspect: docker logs $PG_CONTAINER"
}

setup_direct_prereqs() {
  c "3/7  direct Firecracker prerequisites"
  command -v ip >/dev/null 2>&1 || die "ip is required; install iproute2"
  mkdir -p "$SOCKET_DIR" "$LOG_DIR" "$STATE_DIR/images" "$STATE_DIR/custom"
  if [ "$IS_ROOT" -eq 1 ]; then
    sysctl -w net.ipv4.ip_forward=1 >/dev/null 2>&1 || warn "could not enable net.ipv4.ip_forward"
    ok "TAP/socket/log directories ready under $STATE_DIR"
  else
    warn "not root; privileged TAP setup will be checked when Porter boots a replica"
  fi
}

setup_firecracker() {
  c "4/7  Firecracker binary"
  local arch fc_arch helper
  arch="$(uname -m)"
  case "$arch" in x86_64) fc_arch=x86_64;; aarch64) fc_arch=aarch64;; *) die "unsupported architecture: $arch";; esac
  helper="$REPO_DIR/scripts/backend/install-firecracker.sh"
  [ -x "$helper" ] || chmod +x "$helper"
  if "$helper" "$FIRECRACKER_VERSION" "$fc_arch" "$FIRECRACKER_DIR"; then
    ok "verified Firecracker $FIRECRACKER_VERSION at $FIRECRACKER_DIR"
  elif [ "${PORTER_ALLOW_FIRECRACKER_FALLBACK:-0}" = "1" ] && [ "$FIRECRACKER_VERSION" != "v1.16.0" ]; then
    warn "stable Firecracker $FIRECRACKER_VERSION failed; trying pinned fallback v1.16.0"
    FIRECRACKER_VERSION=v1.16.0 "$helper" v1.16.0 "$fc_arch" "$FIRECRACKER_DIR" || die "stable and fallback Firecracker downloads failed"
  else
    die "Firecracker setup failed; set PORTER_ALLOW_FIRECRACKER_FALLBACK=1 to permit the pinned fallback"
  fi
}

setup_base_image() {
  c "5/8  base microVM image"
  mkdir -p "$BASE_IMAGE_DIR"
  if [ -f "$KERNEL_PATH" ] && [ -f "$ROOTFS_PATH" ]; then
    ok "using local base artifacts: $KERNEL_PATH + $ROOTFS_PATH"
    return
  fi
  if [ -z "$BASE_IMAGE_URL" ]; then
    warn "base image is not installed; set PORTER_GITHUB_REPOSITORY, PORTER_RELEASE_TAG, PORTER_BASE_IMAGE_ASSET, and PORTER_BASE_IMAGE_SHA256, then rerun"
    return
  fi
  [ -n "$BASE_IMAGE_SHA256" ] || die "PORTER_BASE_IMAGE_SHA256 is required for a remote base image bundle"
  local tmp archive kernel rootfs
  tmp="$(mktemp -d)"
  archive="$tmp/base-image"
  curl --connect-timeout 15 --max-time 900 --retry 3 -fL --progress-bar -o "$archive" "$BASE_IMAGE_URL" \
    || die "base image download failed: $BASE_IMAGE_URL"
  printf '%s  %s\n' "$BASE_IMAGE_SHA256" "$archive" | sha256sum -c - || die "base image checksum mismatch"
  mkdir -p "$tmp/unpacked"
  case "$BASE_IMAGE_URL" in
    *.zip|*.zip\?*) command -v unzip >/dev/null 2>&1 || die "unzip is required for a .zip base image bundle"; unzip -q "$archive" -d "$tmp/unpacked" ;;
    *) tar --extract --file "$archive" --directory "$tmp/unpacked" --no-same-owner --no-same-permissions ;;
  esac
  kernel="$(find "$tmp/unpacked" -type f -name vmlinux -print -quit)"
  rootfs="$(find "$tmp/unpacked" -type f -name rootfs.ext4 -print -quit)"
  [ -n "$kernel" ] && [ -n "$rootfs" ] || die "base bundle must contain vmlinux and rootfs.ext4"
  install -m 0644 "$kernel" "$BASE_IMAGE_DIR/vmlinux"
  install -m 0644 "$rootfs" "$BASE_IMAGE_DIR/rootfs.ext4"
  KERNEL_PATH="$BASE_IMAGE_DIR/vmlinux"
  ROOTFS_PATH="$BASE_IMAGE_DIR/rootfs.ext4"
  ok "installed verified base image at $BASE_IMAGE_DIR"
}

check_artifacts() {
  c "6/8  kernel and rootfs artifact readiness"
  if [ -f "$KERNEL_PATH" ] && [ -s "$KERNEL_PATH" ]; then
    ok "kernel: $KERNEL_PATH"
  else
    warn "missing kernel: $KERNEL_PATH"
  fi
  if [ -f "$ROOTFS_PATH" ] && [ -s "$ROOTFS_PATH" ]; then
    ok "rootfs: $ROOTFS_PATH"
  else
    warn "missing rootfs: $ROOTFS_PATH"
  fi
  if [ -f "$KERNEL_PATH" ] && [ -f "$ROOTFS_PATH" ]; then
    sha256sum "$KERNEL_PATH" "$ROOTFS_PATH" > "$BASE_IMAGE_DIR/artifacts.sha256"
    ok "wrote $BASE_IMAGE_DIR/artifacts.sha256"
  fi
  ok "custom bundles: $STATE_DIR/custom"
}

build_porter() {
  c "7/8  build Porter"
  command -v go >/dev/null 2>&1 || die "Go 1.25+ is required"
  (cd "$BACKEND_DIR" && go build -trimpath -ldflags "-X main.Version=$(git describe --tags --always 2>/dev/null || echo beta-dev)" -o "$BIN_DIR/porter" ./cmd/porter)
  ok "built $BIN_DIR/porter"
}

write_config() {
  c "8/8  write direct-only porter.toml"
  local fc_bin
  fc_bin="$FIRECRACKER_BIN"
  cat > "$CONF_DIR/porter.toml" <<EOF
# Generated by scripts/backend/install.sh. Prefer environment variables for secrets.
[server]
listen_addr = "127.0.0.1:8080"
base_domain = "porter.test"

[database]
url = "$DB_URL"
auto_migrate = true

[firecracker]
runtime_mode = "direct"
base_image_ref = "base://default"
api_socket_dir = "$SOCKET_DIR"
kernel_image = "$KERNEL_PATH"
rootfs_path = "$ROOTFS_PATH"
firecracker_bin = "$fc_bin"
logs_dir = "$LOG_DIR"
images_dir = "$STATE_DIR/images"
custom_images_dir = "$STATE_DIR/custom"

[health]
enabled = true

[ssh]
enabled = false
listen_addr = ":2222"
EOF
  ok "wrote $CONF_DIR/porter.toml"
  if [ -z "$PORTER_BOOTSTRAP_ADMIN_PASSWORD" ]; then
    warn "set PORTER_BOOTSTRAP_ADMIN_PASSWORD (12+ chars) for the first database-backed admin login"
  fi
  if [ -z "$PORTER_SECRET_KEY" ]; then
    warn "set PORTER_SECRET_KEY before creating encrypted project secrets"
  fi
}

print_status() {
  c "status"
  printf '  KVM        : %s\n' "$([ -e /dev/kvm ] && echo present || echo missing)"
  printf '  Firecracker: %s\n' "$([ -x "$FIRECRACKER_BIN" ] && echo "$FIRECRACKER_BIN" || echo missing)"
  printf '  kernel     : %s\n' "$([ -f "$KERNEL_PATH" ] && echo present || echo missing)"
  printf '  rootfs     : %s\n' "$([ -f "$ROOTFS_PATH" ] && echo present || echo missing)"
  printf '  socket dir : %s\n' "$SOCKET_DIR"
  printf '  database   : %s\n' "$DB_URL"
  printf '  config     : %s\n' "$CONF_DIR/porter.toml"
  printf '\nRun: %s run\n' "$0"
}

nuke() {
  read -r -p "Delete .dev and the dev PostgreSQL volume? [y/N] " answer
  case "$answer" in y|Y)
    docker rm -f "$PG_CONTAINER" >/dev/null 2>&1 || true
    docker volume rm "$PG_VOLUME" >/dev/null 2>&1 || true
    rm -rf "$DEV_DIR"
    ok "removed local dev state"
    ;;
    *) echo "aborted" ;;
  esac
}

case "${1:-install}" in
  install) check_kvm; setup_postgres; setup_direct_prereqs; setup_firecracker; setup_base_image; check_artifacts; build_porter; write_config; print_status ;;
  run) build_porter; write_config; exec "$BIN_DIR/porter" -config "$CONF_DIR/porter.toml" ;;
  status) print_status ;;
  nuke) nuke ;;
  *) die "usage: $0 [install|run|status|nuke]" ;;
esac
