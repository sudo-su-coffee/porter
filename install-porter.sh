#!/usr/bin/env bash
# Install a compiled Porter release package from a GitHub Release.
# The package contains the Go daemon, the checksum-pinned Firecracker helper,
# the release manifest, and a real vmlinux/rootfs.ext4 base-image bundle.
set -euo pipefail

REPOSITORY="${PORTER_GITHUB_REPOSITORY:-sudo-su-coffee/porter}"
RELEASE_TAG="${PORTER_RELEASE_TAG:-v1.0.0-beta-dev}"
ARCH="${1:-$(uname -m)}"
DEST_DIR="${PORTER_INSTALL_DIR:-/opt/porter}"
PACKAGE_SHA256="${PORTER_RELEASE_PACKAGE_SHA256:-}"

case "$ARCH" in
  x86_64|aarch64) ;;
  *) echo "unsupported architecture: $ARCH" >&2; exit 2 ;;
esac
case "$REPOSITORY" in
  */*) ;;
  *) echo "PORTER_GITHUB_REPOSITORY must be owner/repository" >&2; exit 2 ;;
esac
[ -n "$PACKAGE_SHA256" ] || {
  echo "PORTER_RELEASE_PACKAGE_SHA256 is required; obtain it from the signed release manifest or release operator" >&2
  exit 2
}

PACKAGE="porter-${RELEASE_TAG}-${ARCH}.tar.gz"
URL="https://github.com/${REPOSITORY}/releases/download/${RELEASE_TAG}/${PACKAGE}"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT
ARCHIVE="$TMP_DIR/$PACKAGE"
curl --fail --location --retry 3 --connect-timeout 15 --max-time 900 -o "$ARCHIVE" "$URL"
printf '%s  %s\n' "$PACKAGE_SHA256" "$ARCHIVE" | sha256sum -c -
mkdir -p "$DEST_DIR"
tar --extract --file "$ARCHIVE" --directory "$DEST_DIR" --no-same-owner --no-same-permissions
[ -x "$DEST_DIR/porter" ] || { echo "verified package did not contain an executable porter daemon" >&2; exit 1; }
[ -s "$DEST_DIR/base-images/default/vmlinux" ] || { echo "verified package did not contain vmlinux" >&2; exit 1; }
[ -s "$DEST_DIR/base-images/default/rootfs.ext4" ] || { echo "verified package did not contain rootfs.ext4" >&2; exit 1; }
echo "Installed Porter $RELEASE_TAG/$ARCH at $DEST_DIR"
