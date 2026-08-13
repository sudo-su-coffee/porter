#!/usr/bin/env bash
# Install a compiled Porter Linux release package from a GitHub Release.
# The package contains the Go daemon with embedded Vue assets, official
# Firecracker installer metadata, and a real vmlinux/rootfs.ext4 bundle.
set -euo pipefail

REPOSITORY="${PORTER_GITHUB_REPOSITORY:-sudo-su-coffee/porter}"
RELEASE_TAG="${PORTER_RELEASE_TAG:-v1.0.0-beta-dev}"
ARCH="${1:-$(uname -m)}"
PREFIX="${PORTER_INSTALL_DIR:-/opt/porter}"
STATE_DIR="${PORTER_STATE_DIR:-/var/porter}"
ETC_DIR="${PORTER_ETC_DIR:-$STATE_DIR}"
RUN_DIR="${PORTER_RUN_DIR:-/run/porter}"
LOG_DIR="${PORTER_LOG_DIR:-/var/log/porter}"
ENV_FILE="${PORTER_ENV_FILE:-$ETC_DIR/porter.env}"
PACKAGE_SHA256="${PORTER_RELEASE_PACKAGE_SHA256:-}"
FIRECRACKER_VERSION="${FIRECRACKER_VERSION:-v1.16.1}"
. "$(dirname "${BASH_SOURCE[0]}")/postgres.sh" 2>/dev/null || true

die() { echo "FAIL: $1" >&2; exit 1; }
ok() { echo "ok: $1"; }
generate_password() {
  if command -v openssl >/dev/null 2>&1; then openssl rand -hex 24; else od -An -N24 -tx1 /dev/urandom | tr -d ' \n'; fi
}
generate_secret() {
  if command -v openssl >/dev/null 2>&1; then openssl rand -hex 32; else od -An -N32 -tx1 /dev/urandom | tr -d ' \n'; fi
}

[ "$(uname -s)" = Linux ] || die "this installer supports Linux only"
[ "$(id -u)" -eq 0 ] || die "run as root: sudo $0"
command -v systemctl >/dev/null 2>&1 || die "systemd is required"
command -v curl >/dev/null 2>&1 || die "curl is required"
command -v sha256sum >/dev/null 2>&1 || die "sha256sum is required"
case "$ARCH" in x86_64|aarch64) ;; *) die "unsupported architecture: $ARCH" ;; esac

case "$REPOSITORY" in */*) ;; *) die "PORTER_GITHUB_REPOSITORY must be owner/repository" ;; esac
[ -n "$PACKAGE_SHA256" ] || die "PORTER_RELEASE_PACKAGE_SHA256 is required; obtain it from the release manifest"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT
if [ -n "${PORTER_RELEASE_ARCHIVE:-}" ]; then
  ARCHIVE="$PORTER_RELEASE_ARCHIVE"
  [ -f "$ARCHIVE" ] || die "PORTER_RELEASE_ARCHIVE does not exist: $ARCHIVE"
  PACKAGE="$(basename "$ARCHIVE")"
else
  PACKAGE="porter-${RELEASE_TAG}-${ARCH}.tar.gz"
  URL="https://github.com/${REPOSITORY}/releases/download/${RELEASE_TAG}/${PACKAGE}"
  ARCHIVE="$TMP_DIR/$PACKAGE"
  curl --fail --location --retry 3 --connect-timeout 15 --max-time 900 -o "$ARCHIVE" "$URL"
fi
[ -n "$PACKAGE_SHA256" ] || PACKAGE_SHA256="$(sha256sum "$ARCHIVE" | awk '{print $1}')"
printf '%s  %s\n' "$PACKAGE_SHA256" "$ARCHIVE" | sha256sum -c -
mkdir -p "$TMP_DIR/package"
tar --extract --file "$ARCHIVE" --directory "$TMP_DIR/package" --no-same-owner --no-same-permissions
[ -s "$TMP_DIR/package/porter" ] || die "release package did not contain the Porter daemon"
[ -s "$TMP_DIR/package/base-images/default/vmlinux" ] || die "release package did not contain vmlinux"
[ -s "$TMP_DIR/package/base-images/default/rootfs.ext4" ] || die "release package did not contain rootfs.ext4"

if ! getent group porter >/dev/null 2>&1; then groupadd --system porter; fi
if ! id -u porter >/dev/null 2>&1; then useradd --system --gid porter --home-dir "$STATE_DIR" --shell /usr/sbin/nologin porter; fi
if getent group kvm >/dev/null 2>&1; then usermod -a -G kvm porter; fi
install -d -o root -g porter -m 0750 "$PREFIX/bin" "$ETC_DIR"
install -d -o porter -g porter -m 0750 "$STATE_DIR/base-images/default" "$STATE_DIR/images" "$STATE_DIR/custom" "$STATE_DIR/volumes" "$RUN_DIR/firecracker" "$LOG_DIR"

if [ -f "$ENV_FILE" ]; then
  set -a
  # shellcheck disable=SC1090
  . "$ENV_FILE"
  set +a
fi
if declare -F porter_pg_setup >/dev/null 2>&1; then porter_pg_setup; else
  [ -n "${PORTER_DATABASE_URL:-}" ] || die "set PORTER_DATABASE_URL before package installation; PostgreSQL setup helper was not included"
fi
install -o root -g porter -m 0755 "$TMP_DIR/package/porter" "$PREFIX/bin/porter"
install -o porter -g porter -m 0640 "$TMP_DIR/package/base-images/default/vmlinux" "$STATE_DIR/base-images/default/vmlinux"
install -o porter -g porter -m 0640 "$TMP_DIR/package/base-images/default/rootfs.ext4" "$STATE_DIR/base-images/default/rootfs.ext4"
sha256sum "$STATE_DIR/base-images/default/vmlinux" "$STATE_DIR/base-images/default/rootfs.ext4" > "$STATE_DIR/base-images/default/artifacts.sha256"

install -m 0755 "$TMP_DIR/package/install-firecracker.sh" "$PREFIX/install-firecracker.sh"
"$PREFIX/install-firecracker.sh" "$FIRECRACKER_VERSION" "$ARCH" "$PREFIX/bin"

umask 077
if [ -f "$ENV_FILE" ]; then
  set -a
  # shellcheck disable=SC1090
  . "$ENV_FILE"
  set +a
fi
: "${PORTER_DATABASE_URL:=postgres://porter:change-me@127.0.0.1:5432/porter?sslmode=disable}"
: "${PORTER_BOOTSTRAP_ADMIN_PASSWORD:=$(generate_password)}"
: "${PORTER_SECRET_KEY:=$(generate_secret)}"
[ "${#PORTER_BOOTSTRAP_ADMIN_PASSWORD}" -ge 12 ] || die "bootstrap password must be at least 12 characters"
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
chown root:porter "$ENV_FILE"
chmod 0640 "$ENV_FILE"
cat > "$ETC_DIR/porter.toml" <<EOF
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
chown root:porter "$ETC_DIR/porter.toml"
chmod 0640 "$ETC_DIR/porter.toml"

SERVICE_TMP="$TMP_DIR/porter.service.rendered"
sed "s#/var/porter#$STATE_DIR#g" "$TMP_DIR/package/porter.service" > "$SERVICE_TMP"
install -o root -g root -m 0644 "$SERVICE_TMP" /etc/systemd/system/porter.service
systemctl daemon-reload
systemctl enable porter.service >/dev/null
if [ "${PORTER_NO_START:-0}" = 1 ]; then
  echo "Porter service installed but not started (PORTER_NO_START=1)"
else
  systemctl restart porter.service
fi

echo "Installed Porter $RELEASE_TAG/$ARCH at $PREFIX"
LISTEN_ADDR="${PORTER_LISTEN_ADDR:-:8080}"
case "$LISTEN_ADDR" in :*) DASHBOARD_URL="http://127.0.0.1$LISTEN_ADDR" ;; *) DASHBOARD_URL="http://$LISTEN_ADDR" ;; esac
echo "Dashboard URL: $DASHBOARD_URL"
echo "Editable TOML: $STATE_DIR/porter.toml"
echo "Protected environment: $ENV_FILE"
echo "Super-admin username: admin"
echo "Bootstrap password: $PORTER_BOOTSTRAP_ADMIN_PASSWORD"
echo "Credential file: $ENV_FILE (mode 0640; rotate after first login)"
echo "Check service: systemctl status porter.service"
