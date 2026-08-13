#!/usr/bin/env bash
# Install Porter from a Linux source checkout as one daemon package.
#
# This installer builds the Vue dashboard first, embeds backend/web/dist into
# the Go binary, installs the checksum-pinned official Firecracker binary from
# GitHub, creates a least-privilege systemd service, and never installs
# containerd or an OCI runtime. A real vmlinux + rootfs.ext4 pair is required
# unless PORTER_ALLOW_MISSING_BASE_IMAGE=1 is explicitly set for control-plane
# development only.
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BACKEND_DIR="$REPO_DIR/backend"
FRONTEND_DIR="$REPO_DIR/frontend"
PREFIX="${PORTER_INSTALL_DIR:-/opt/porter}"
STATE_DIR="${PORTER_STATE_DIR:-/var/porter}"
ETC_DIR="${PORTER_ETC_DIR:-$STATE_DIR}"
RUN_DIR="${PORTER_RUN_DIR:-/run/porter}"
LOG_DIR="${PORTER_LOG_DIR:-/var/log/porter}"
ENV_FILE="${PORTER_ENV_FILE:-$ETC_DIR/porter.env}"
CONFIG_FILE="${PORTER_CONFIG_FILE:-$ETC_DIR/porter.toml}"
SERVICE_FILE="${PORTER_SERVICE_FILE:-/etc/systemd/system/porter.service}"
PORTER_USER="${PORTER_USER:-porter}"
FIRECRACKER_VERSION="${FIRECRACKER_VERSION:-v1.16.1}"
ARCH="$(uname -m)"
RELEASE_TAG="${PORTER_RELEASE_TAG:-v1.0.0-beta-dev}"
BUILD_VERSION="${PORTER_BUILD_VERSION:-$RELEASE_TAG}"
. "$REPO_DIR/scripts/backend/postgres.sh"

die() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }
note() { printf '\n==> %s\n' "$1"; }
ok() { printf '    ok: %s\n' "$1"; }
warn() { printf '    warn: %s\n' "$1" >&2; }
generate_password() {
  if command -v openssl >/dev/null 2>&1; then openssl rand -hex 24; else od -An -N24 -tx1 /dev/urandom | tr -d ' \n'; fi
}
generate_secret() {
  if command -v openssl >/dev/null 2>&1; then openssl rand -hex 32; else od -An -N32 -tx1 /dev/urandom | tr -d ' \n'; fi
}

[ "$(uname -s)" = Linux ] || die "this installer supports Linux only"
[ "$(id -u)" -eq 0 ] || die "run as root: sudo $0"
command -v go >/dev/null 2>&1 || die "Go is required to compile Porter"
command -v systemctl >/dev/null 2>&1 || die "systemd is required for the daemon installer"
command -v curl >/dev/null 2>&1 || die "curl is required"
command -v sha256sum >/dev/null 2>&1 || die "sha256sum is required"
case "$ARCH" in x86_64|aarch64) ;; *) die "unsupported architecture: $ARCH" ;; esac

if [ "${PORTER_SKIP_FRONTEND_BUILD:-0}" != 1 ]; then
  note "Build and embed Vue dashboard"
  command -v npm >/dev/null 2>&1 || die "npm is required to build the Vue dashboard; use PORTER_SKIP_FRONTEND_BUILD=1 only when backend/web/dist is already built"
  if [ -f "$FRONTEND_DIR/package-lock.json" ]; then
    (cd "$FRONTEND_DIR" && npm ci --no-audit --no-fund)
  elif [ ! -d "$FRONTEND_DIR/node_modules" ]; then
    (cd "$FRONTEND_DIR" && npm install --no-audit --no-fund)
  fi
  (cd "$FRONTEND_DIR" && npm run build)
else
  [ -f "$BACKEND_DIR/web/dist/index.html" ] || die "backend/web/dist/index.html is missing; build the frontend first"
  warn "skipping frontend build; using existing backend/web/dist"
fi

note "Create service account and Linux directories"
if ! getent group "$PORTER_USER" >/dev/null 2>&1; then groupadd --system "$PORTER_USER"; fi
if ! id -u "$PORTER_USER" >/dev/null 2>&1; then
  useradd --system --gid "$PORTER_USER" --home-dir "$STATE_DIR" --shell /usr/sbin/nologin "$PORTER_USER"
fi
if getent group kvm >/dev/null 2>&1; then usermod -a -G kvm "$PORTER_USER"; else warn "kvm group is absent; Firecracker may require operator host setup"; fi
install -d -o "$PORTER_USER" -g "$PORTER_USER" -m 0750 "$STATE_DIR" "$STATE_DIR/base-images/default" "$STATE_DIR/images" "$STATE_DIR/custom" "$STATE_DIR/volumes"
install -d -o "$PORTER_USER" -g "$PORTER_USER" -m 0750 "$RUN_DIR/firecracker" "$LOG_DIR"
install -d -o root -g "$PORTER_USER" -m 0750 "$PREFIX/bin" "$ETC_DIR"

if [ -f "$ENV_FILE" ]; then
  set -a
  # shellcheck disable=SC1090
  . "$ENV_FILE"
  set +a
fi
porter_pg_setup

note "Install official Firecracker"
"$REPO_DIR/scripts/backend/install-firecracker.sh" "$FIRECRACKER_VERSION" "$ARCH" "$PREFIX/bin"
ok "Firecracker checksum verified at $PREFIX/bin/firecracker"

note "Install real guest artifacts"
BASE_DIR="${PORTER_BASE_IMAGE_DIR:-$STATE_DIR/base-images/default}"
if [ -f "$BASE_DIR/vmlinux" ] && [ -s "$BASE_DIR/vmlinux" ] && [ -f "$BASE_DIR/rootfs.ext4" ] && [ -s "$BASE_DIR/rootfs.ext4" ]; then
  if [ "$BASE_DIR" != "$STATE_DIR/base-images/default" ]; then
    install -o "$PORTER_USER" -g "$PORTER_USER" -m 0640 "$BASE_DIR/vmlinux" "$STATE_DIR/base-images/default/vmlinux"
    install -o "$PORTER_USER" -g "$PORTER_USER" -m 0640 "$BASE_DIR/rootfs.ext4" "$STATE_DIR/base-images/default/rootfs.ext4"
  else
    chown "$PORTER_USER":"$PORTER_USER" "$BASE_DIR/vmlinux" "$BASE_DIR/rootfs.ext4"
    chmod 0640 "$BASE_DIR/vmlinux" "$BASE_DIR/rootfs.ext4"
  fi
  sha256sum "$STATE_DIR/base-images/default/vmlinux" "$STATE_DIR/base-images/default/rootfs.ext4" > "$STATE_DIR/base-images/default/artifacts.sha256"
  ok "real vmlinux and rootfs.ext4 installed"
elif [ "${PORTER_ALLOW_MISSING_BASE_IMAGE:-0}" = 1 ]; then
  warn "installing control plane without vmlinux/rootfs.ext4; replicas cannot boot until real artifacts are installed"
else
  die "real vmlinux and rootfs.ext4 are required; set PORTER_BASE_IMAGE_DIR or explicitly use PORTER_ALLOW_MISSING_BASE_IMAGE=1 for control-plane-only development"
fi

note "Build and install the embedded Go daemon"
TMP_BIN="$(mktemp)"
trap 'rm -f "$TMP_BIN"' EXIT
(cd "$BACKEND_DIR" && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.Version=$BUILD_VERSION" -o "$TMP_BIN" ./cmd/porter)
install -o root -g "$PORTER_USER" -m 0755 "$TMP_BIN" "$PREFIX/bin/porter"
ok "installed $PREFIX/bin/porter with embedded dashboard assets"

note "Write configuration and one-time bootstrap credentials"
if [ -f "$ENV_FILE" ]; then
  # This file is created by this installer and is mode 0600; preserve existing
  # values so reruns never rotate a working admin password or secret key.
  set -a
  # shellcheck disable=SC1090
  . "$ENV_FILE"
  set +a
fi
: "${PORTER_DATABASE_URL:=postgres://porter:change-me@127.0.0.1:5432/porter?sslmode=disable}"
: "${PORTER_BOOTSTRAP_ADMIN_PASSWORD:=$(generate_password)}"
: "${PORTER_SECRET_KEY:=$(generate_secret)}"
[ "${#PORTER_BOOTSTRAP_ADMIN_PASSWORD}" -ge 12 ] || die "PORTER_BOOTSTRAP_ADMIN_PASSWORD must be at least 12 characters"
umask 077
cat > "$ENV_FILE" <<EOF
PORTER_DATABASE_URL=$PORTER_DATABASE_URL
PORTER_BOOTSTRAP_ADMIN_PASSWORD=$PORTER_BOOTSTRAP_ADMIN_PASSWORD
PORTER_SECRET_KEY=$PORTER_SECRET_KEY
PORTER_LISTEN_ADDR=${PORTER_LISTEN_ADDR:-:8080}
PORTER_RUNTIME_MODE=direct
PORTER_FIRECRACKER_BIN=$PREFIX/bin/firecracker
PORTER_FIRECRACKER_API_SOCKET_DIR=$RUN_DIR/firecracker
PORTER_KERNEL_IMAGE=$STATE_DIR/base-images/default/vmlinux
PORTER_ROOTFS_PATH=$STATE_DIR/base-images/default/rootfs.ext4
PORTER_LOGS_DIR=$LOG_DIR
PORTER_IMAGES_DIR=$STATE_DIR/images
PORTER_CUSTOM_IMAGES_DIR=$STATE_DIR/custom
PORTER_VOLUMES_DIR=$STATE_DIR/volumes
PORTER_HEALTH_ENABLED=${PORTER_HEALTH_ENABLED:-true}
EOF
chown root:"$PORTER_USER" "$ENV_FILE"
chmod 0640 "$ENV_FILE"
cat > "$CONFIG_FILE" <<EOF
[server]
listen_addr = ":8080"
base_domain = "porter.local"

[database]
auto_migrate = true

[firecracker]
runtime_mode = "direct"
base_image_ref = "base://default"
api_socket_dir = "$RUN_DIR/firecracker"
kernel_image = "$STATE_DIR/base-images/default/vmlinux"
rootfs_path = "$STATE_DIR/base-images/default/rootfs.ext4"
firecracker_bin = "$PREFIX/bin/firecracker"
logs_dir = "$LOG_DIR"
images_dir = "$STATE_DIR/images"
custom_images_dir = "$STATE_DIR/custom"
EOF
chown root:"$PORTER_USER" "$CONFIG_FILE"
chmod 0640 "$CONFIG_FILE"

note "Install and enable systemd daemon"
SERVICE_TMP="$(mktemp)"
trap 'rm -f "$SERVICE_TMP"' EXIT
sed "s#/var/porter#$STATE_DIR#g" "$REPO_DIR/release/porter.service" > "$SERVICE_TMP"
install -o root -g root -m 0644 "$SERVICE_TMP" "$SERVICE_FILE"
rm -f "$SERVICE_TMP"
systemctl daemon-reload
systemctl enable porter.service >/dev/null
if [ "${PORTER_NO_START:-0}" = 1 ]; then
  warn "PORTER_NO_START=1: service installed but not started"
else
  systemctl restart porter.service
fi

printf '\nPorter Linux installation complete.\n'
LISTEN_ADDR="${PORTER_LISTEN_ADDR:-:8080}"
case "$LISTEN_ADDR" in :*) DASHBOARD_URL="http://127.0.0.1$LISTEN_ADDR" ;; *) DASHBOARD_URL="http://$LISTEN_ADDR" ;; esac
printf 'Dashboard URL: %s\n' "$DASHBOARD_URL"
printf 'Editable TOML: %s\n' "$CONFIG_FILE"
printf 'Protected environment: %s\n' "$ENV_FILE"
printf 'Super-admin username: admin\n'
printf 'Bootstrap password: %s\n' "$PORTER_BOOTSTRAP_ADMIN_PASSWORD"
printf 'Credential file: %s (mode 0640; rotate after first login)\n' "$ENV_FILE"
printf 'Service: systemctl status porter.service\n'
