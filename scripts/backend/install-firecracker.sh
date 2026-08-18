#!/usr/bin/env bash
# Download and install a checksum-pinned official Firecracker release from
# GitHub into a local Porter artifact directory. This helper never installs
# containerd or an OCI runtime and never downloads a guest rootfs implicitly.
set -euo pipefail

VERSION="${1:-${FIRECRACKER_VERSION:-v1.16.1}}"
ARCH="${2:-$(uname -m)}"
DEST_DIR="${3:-${PORTER_FIRECRACKER_DIR:-/var/lib/porter/bin}}"
case "$ARCH" in
  x86_64|aarch64) ;;
  *) echo "unsupported architecture: $ARCH" >&2; exit 2 ;;
esac

case "$VERSION:$ARCH" in
  v1.16.1:x86_64) EXPECTED="382a02a869e4d6d5cb14c40577f9545e8458021ea8b0b2d3fc10ec14d9c242e6" ;;
  v1.16.1:aarch64) EXPECTED="8d0e69f6d6f9a1724551f607f18504052c16c1828ee3d4d7b6e6c73380871e0e" ;;
  v1.16.0:x86_64) EXPECTED="bd04e26952d4e158085778c6230a0b383d2619c319182e27eaa9d61a212e92d6" ;;
  v1.16.0:aarch64) EXPECTED="531c713cdbc37d4b8bc2533d851aabc0267096afa1768086a37672abb668efd7" ;;
  *) echo "version is not pinned in release/firecracker-versions.json: $VERSION/$ARCH" >&2; exit 2 ;;
esac

mkdir -p "$DEST_DIR"
TARGET="$DEST_DIR/firecracker-$VERSION-$ARCH"
JAILER_TARGET="$DEST_DIR/jailer-$VERSION-$ARCH"
if [ -x "$TARGET" ] && [ -x "$JAILER_TARGET" ] && printf '%s  %s\n' "$EXPECTED" "$TARGET" | sha256sum -c - >/dev/null 2>&1; then
  ln -sfn "$(basename "$TARGET")" "$DEST_DIR/firecracker"
  ln -sfn "$(basename "$JAILER_TARGET")" "$DEST_DIR/jailer"
  echo "Firecracker and jailer $VERSION/$ARCH already verified in $DEST_DIR"
  exit 0
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT
ARCHIVE="$TMP_DIR/firecracker.tgz"
URL="https://github.com/firecracker-microvm/firecracker/releases/download/${VERSION}/firecracker-${VERSION}-${ARCH}.tgz"
curl --fail --location --retry 3 --connect-timeout 15 --max-time 300 -o "$ARCHIVE" "$URL"
printf '%s  %s\n' "$EXPECTED" "$ARCHIVE" | sha256sum -c -
tar -xzf "$ARCHIVE" -C "$TMP_DIR"
SOURCE="$TMP_DIR/firecracker-${VERSION}-${ARCH}"
[ -f "$SOURCE" ] || SOURCE="$(find "$TMP_DIR" -type f -name "firecracker-${VERSION}-${ARCH}" ! -name '*.debug' | head -1)"
[ -n "$SOURCE" ] && [ -f "$SOURCE" ] || { echo "Firecracker binary missing from verified archive" >&2; exit 1; }
JAILER_SOURCE="$TMP_DIR/jailer-${VERSION}-${ARCH}"
[ -f "$JAILER_SOURCE" ] || JAILER_SOURCE="$(find "$TMP_DIR" -type f -name "jailer-${VERSION}-${ARCH}" ! -name '*.debug' | head -1)"
[ -n "$JAILER_SOURCE" ] && [ -f "$JAILER_SOURCE" ] || { echo "jailer binary missing from verified archive" >&2; exit 1; }
install -m 0755 "$SOURCE" "$TARGET"
install -m 0755 "$JAILER_SOURCE" "$JAILER_TARGET"
printf '%s  %s\n' "$EXPECTED" "$TARGET" > "$TARGET.sha256"
printf '%s  %s\n' "$EXPECTED" "$JAILER_TARGET" > "$JAILER_TARGET.sha256"
ln -sfn "$(basename "$TARGET")" "$DEST_DIR/firecracker"
ln -sfn "$(basename "$JAILER_TARGET")" "$DEST_DIR/jailer"
echo "Installed verified Firecracker and jailer $VERSION/$ARCH in $DEST_DIR"
