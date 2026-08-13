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
ARCHIVE="$TMP_DIR/$PACKAGE"
CHECKSUM_FILE="$TMP_DIR/$PACKAGE.sha256"

die() { echo "FAIL: $1" >&2; exit 1; }
[ "$(uname -s)" = Linux ] || die "this installer supports Linux only"
[ "$(id -u)" -eq 0 ] || die "run as root: sudo bash install-from-github.sh"
command -v curl >/dev/null 2>&1 || die "curl is required"
command -v sha256sum >/dev/null 2>&1 || die "sha256sum is required"
command -v tar >/dev/null 2>&1 || die "tar is required"
case "$ARCH" in x86_64|aarch64) ;; *) die "unsupported architecture: $ARCH" ;; esac

curl --fail --location --retry 3 --connect-timeout 15 --max-time 900 -o "$ARCHIVE" "$BASE_URL/$PACKAGE"
EXPECTED="${PORTER_RELEASE_PACKAGE_SHA256:-}"
if [ -z "$EXPECTED" ]; then
  if curl --fail --location --retry 3 --connect-timeout 15 --max-time 30 -o "$CHECKSUM_FILE" "$BASE_URL/$PACKAGE.sha256"; then
    EXPECTED="$(awk 'NF {print $1; exit}' "$CHECKSUM_FILE")"
  else
    die "release checksum sidecar is unavailable; set PORTER_RELEASE_PACKAGE_SHA256 explicitly"
  fi
fi
[ -n "$EXPECTED" ] || die "empty release checksum"
printf '%s  %s\n' "$EXPECTED" "$ARCHIVE" | sha256sum -c -

mkdir -p "$TMP_DIR/package"
tar --extract --file "$ARCHIVE" --directory "$TMP_DIR/package" --no-same-owner --no-same-permissions
[ -x "$TMP_DIR/package/install-porter.sh" ] || die "release package did not contain install-porter.sh"
PORTER_RELEASE_ARCHIVE="$ARCHIVE" \
PORTER_RELEASE_PACKAGE_SHA256="$EXPECTED" \
  "$TMP_DIR/package/install-porter.sh" "$ARCH"
