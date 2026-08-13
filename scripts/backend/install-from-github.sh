#!/usr/bin/env bash
# One-command Linux bootstrap for a published Porter GitHub Release.
# Usage:
#   sudo bash install-from-github.sh
# or:
#   curl -fsSL https://raw.githubusercontent.com/sudo-su-coffee/porter/main/scripts/backend/install-from-github.sh | sudo bash
set -euo pipefail

REPOSITORY="${PORTER_GITHUB_REPOSITORY:-sudo-su-coffee/porter}"
RELEASE_TAG="${PORTER_RELEASE_TAG:-v1.0.0-beta-dev}"
ARCH="${1:-$(uname -m)}"
PACKAGE="porter-${RELEASE_TAG}-${ARCH}.tar.gz"
BASE_URL="https://github.com/${REPOSITORY}/releases/download/${RELEASE_TAG}"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT
CACHE_DIR="${PORTER_CACHE_DIR:-/var/cache/porter/releases}"
ARCHIVE="$CACHE_DIR/$PACKAGE"
CHECKSUM_FILE="$CACHE_DIR/$PACKAGE.sha256"

die() { echo "FAIL: $1" >&2; exit 1; }
prompt_tty() {
  local prompt="$1" variable="$2" input='/dev/stdin'
  if [ -r /dev/tty ]; then input='/dev/tty'; fi
  IFS= read -r -p "$prompt" "$variable" < "$input"
}
[ "$(uname -s)" = Linux ] || die "this installer supports Linux only"
[ "$(id -u)" -eq 0 ] || die "run as root: sudo bash install-from-github.sh"
command -v curl >/dev/null 2>&1 || die "curl is required"
command -v sha256sum >/dev/null 2>&1 || die "sha256sum is required"
command -v tar >/dev/null 2>&1 || die "tar is required"
case "$ARCH" in x86_64|aarch64) ;; *) die "unsupported architecture: $ARCH" ;; esac

mkdir -p "$CACHE_DIR"
chmod 0750 "$CACHE_DIR"

EXPECTED="${PORTER_RELEASE_PACKAGE_SHA256:-}"
if [ -z "$EXPECTED" ] && [ -s "$CHECKSUM_FILE" ]; then
  EXPECTED="$(awk 'NF {print $1; exit}' "$CHECKSUM_FILE")"
fi
if [ -z "$EXPECTED" ]; then
  CHECKSUM_TMP="$CACHE_DIR/.${PACKAGE}.sha256.$$"
  if curl --fail --location --retry 3 --connect-timeout 15 --max-time 30 -o "$CHECKSUM_TMP" "$BASE_URL/$PACKAGE.sha256"; then
    mv -f "$CHECKSUM_TMP" "$CHECKSUM_FILE"
    EXPECTED="$(awk 'NF {print $1; exit}' "$CHECKSUM_FILE")"
  else
    rm -f "$CHECKSUM_TMP"
    die "release checksum sidecar is unavailable; set PORTER_RELEASE_PACKAGE_SHA256 explicitly"
  fi
fi
[ -n "$EXPECTED" ] || die "empty release checksum"

case "$EXPECTED" in
  ''|*[!0-9a-fA-F]*) die "release checksum is not hexadecimal" ;;
esac
[ "${#EXPECTED}" -eq 64 ] || die "release checksum must contain 64 hex characters"

if [ -s "$ARCHIVE" ] && (cd "$CACHE_DIR" && printf '%s  %s\n' "$EXPECTED" "$PACKAGE" | sha256sum -c - >/dev/null 2>&1); then
  echo "Using verified cached release archive: $ARCHIVE"
else
  rm -f "$ARCHIVE"
  ARCHIVE_TMP="$CACHE_DIR/.${PACKAGE}.tmp.$$"
  curl --fail --location --retry 3 --connect-timeout 15 --max-time 900 -o "$ARCHIVE_TMP" "$BASE_URL/$PACKAGE"
  printf '%s  %s\n' "$EXPECTED" "$ARCHIVE_TMP" | sha256sum -c -
  mv -f "$ARCHIVE_TMP" "$ARCHIVE"
  printf '%s  %s\n' "$EXPECTED" "$PACKAGE" > "$CHECKSUM_FILE"
  echo "Downloaded and cached verified release archive: $ARCHIVE"
fi

printf '\nRelease package is ready. PostgreSQL is required for Porter control-plane data.\n'
POSTGRES_MODE="${PORTER_POSTGRES_MODE:-}"
if [ -z "$POSTGRES_MODE" ]; then
  if [ -t 0 ] || [ -r /dev/tty ]; then
    printf 'Choose a database location:\n'
    printf '  1) local PostgreSQL on this Linux host\n'
    printf '  2) remote PostgreSQL (enter a connection URL)\n'
    prompt_tty 'Select [1/2]: ' POSTGRES_CHOICE
    case "$POSTGRES_CHOICE" in
      1) POSTGRES_MODE=local ;;
      2) POSTGRES_MODE=remote ;;
      *) die 'choose 1 or 2 for PostgreSQL setup' ;;
    esac
  else
    die "PostgreSQL mode is required in a non-interactive shell; use sudo PORTER_POSTGRES_MODE=local bash or sudo PORTER_POSTGRES_MODE=remote PORTER_DATABASE_URL='postgres://...' bash"
  fi
fi
POSTGRES_MODE="${POSTGRES_MODE,,}"
case "$POSTGRES_MODE" in
  local) ;;
  remote)
    if [ -z "${PORTER_DATABASE_URL:-}" ]; then
      if [ -t 0 ] || [ -r /dev/tty ]; then
        prompt_tty 'PostgreSQL connection URL: ' PORTER_DATABASE_URL
      else
        die "PORTER_DATABASE_URL is required for remote PostgreSQL mode"
      fi
    fi
    [ -n "${PORTER_DATABASE_URL:-}" ] || die "PORTER_DATABASE_URL is required for remote PostgreSQL mode"
    ;;
  *) die "PORTER_POSTGRES_MODE must be local or remote" ;;
esac

mkdir -p "$TMP_DIR/package"
tar --extract --file "$ARCHIVE" --directory "$TMP_DIR/package" --no-same-owner --no-same-permissions
[ -x "$TMP_DIR/package/install-porter.sh" ] || die "release package did not contain install-porter.sh"
if [ "$POSTGRES_MODE" = remote ]; then
  PORTER_RELEASE_ARCHIVE="$ARCHIVE" \
  PORTER_RELEASE_PACKAGE_SHA256="$EXPECTED" \
  PORTER_POSTGRES_MODE="$POSTGRES_MODE" \
  PORTER_DATABASE_URL="$PORTER_DATABASE_URL" \
    "$TMP_DIR/package/install-porter.sh" "$ARCH"
else
  PORTER_RELEASE_ARCHIVE="$ARCHIVE" \
  PORTER_RELEASE_PACKAGE_SHA256="$EXPECTED" \
  PORTER_POSTGRES_MODE="$POSTGRES_MODE" \
    "$TMP_DIR/package/install-porter.sh" "$ARCH"
fi
