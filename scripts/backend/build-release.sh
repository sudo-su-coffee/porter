#!/usr/bin/env bash
# Build a self-contained, GitHub-Release-ready Porter package.
# A release is intentionally refused unless a real base image is supplied.
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BACKEND_DIR="$REPO_DIR/backend"
FRONTEND_DIR="$REPO_DIR/frontend"
REPOSITORY="${PORTER_GITHUB_REPOSITORY:-sudo-su-coffee/porter}"
RELEASE_TAG="${1:-${PORTER_RELEASE_TAG:-v1.0.0-beta-dev}}"
ARCH="${2:-$(uname -m)}"
BASE_IMAGE_DIR="${PORTER_BASE_IMAGE_DIR:-}"
DIST_DIR="${PORTER_RELEASE_DIST_DIR:-$BACKEND_DIR/../release-dist/$RELEASE_TAG/$ARCH}"
GOARCH=""
case "$ARCH" in
  x86_64) GOARCH=amd64 ;;
  aarch64) GOARCH=arm64 ;;
  *) echo "unsupported architecture: $ARCH" >&2; exit 2 ;;
esac
case "$REPOSITORY" in
  */*) ;;
  *) echo "PORTER_GITHUB_REPOSITORY must be owner/repository" >&2; exit 2 ;;
esac
[ -n "$BASE_IMAGE_DIR" ] || { echo "PORTER_BASE_IMAGE_DIR is required; refusing to build a release without real guest artifacts" >&2; exit 2; }
[ -f "$BASE_IMAGE_DIR/vmlinux" ] && [ -s "$BASE_IMAGE_DIR/vmlinux" ] || { echo "missing non-empty $BASE_IMAGE_DIR/vmlinux" >&2; exit 2; }
[ -f "$BASE_IMAGE_DIR/rootfs.ext4" ] && [ -s "$BASE_IMAGE_DIR/rootfs.ext4" ] || { echo "missing non-empty $BASE_IMAGE_DIR/rootfs.ext4" >&2; exit 2; }

command -v npm >/dev/null 2>&1 || { echo "npm is required to build and embed the Vue dashboard" >&2; exit 2; }
if [ -f "$FRONTEND_DIR/package-lock.json" ]; then
  (cd "$FRONTEND_DIR" && npm ci --no-audit --no-fund)
else
  (cd "$FRONTEND_DIR" && npm install --no-audit --no-fund)
fi
(cd "$FRONTEND_DIR" && npm run build)
[ -s "$BACKEND_DIR/web/dist/index.html" ] || { echo "Vue build did not produce backend/web/dist/index.html" >&2; exit 2; }

rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR/base-images/default" "$DIST_DIR/release"
CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" go -C "$BACKEND_DIR" build -trimpath -ldflags "-s -w -X main.Version=$RELEASE_TAG" -o "$DIST_DIR/porter" ./cmd/porter
install -m 0755 "$REPO_DIR/scripts/backend/install.sh" "$DIST_DIR/install.sh"
install -m 0755 "$REPO_DIR/scripts/backend/install-porter.sh" "$DIST_DIR/install-porter.sh"
install -m 0755 "$REPO_DIR/scripts/backend/install-firecracker.sh" "$DIST_DIR/install-firecracker.sh"
install -m 0755 "$REPO_DIR/scripts/backend/postgres.sh" "$DIST_DIR/postgres.sh"
install -m 0644 "$REPO_DIR/release/porter.service" "$DIST_DIR/porter.service"
install -m 0644 "$REPO_DIR/release/porter.env.example" "$DIST_DIR/porter.env.example"
install -m 0644 "$REPO_DIR/docs/backend/FIRECRACKER_ARTIFACTS.md" "$DIST_DIR/FIRECRACKER_ARTIFACTS.md"
install -m 0644 "$REPO_DIR/release/firecracker-versions.json" "$DIST_DIR/release/firecracker-versions.json"
install -m 0644 "$BASE_IMAGE_DIR/vmlinux" "$DIST_DIR/base-images/default/vmlinux"
install -m 0644 "$BASE_IMAGE_DIR/rootfs.ext4" "$DIST_DIR/base-images/default/rootfs.ext4"

DAEMON_SHA256="$(sha256sum "$DIST_DIR/porter" | awk '{print $1}')"
KERNEL_SHA256="$(sha256sum "$DIST_DIR/base-images/default/vmlinux" | awk '{print $1}')"
ROOTFS_SHA256="$(sha256sum "$DIST_DIR/base-images/default/rootfs.ext4" | awk '{print $1}')"
PACKAGE="porter-${RELEASE_TAG}-${ARCH}.tar.gz"
BASE_PACKAGE="porter-base-image-${RELEASE_TAG}-${ARCH}.tar.gz"
cat > "$DIST_DIR/release/porter-release-manifest.json" <<EOF
{
  "schema_version": 1,
  "repository": "$REPOSITORY",
  "release_tag": "$RELEASE_TAG",
  "architecture": "$ARCH",
  "distribution": "github-release-only",
  "daemon": {"asset": "porter", "sha256": "$DAEMON_SHA256", "dashboard": "embedded:backend/web/dist"},
  "base_image": {
    "reference": "base://default",
    "kernel_asset": "base-images/default/vmlinux",
    "kernel_sha256": "$KERNEL_SHA256",
    "rootfs_asset": "base-images/default/rootfs.ext4",
    "rootfs_sha256": "$ROOTFS_SHA256"
  },
  "firecracker": {"manifest": "release/firecracker-versions.json"}
}
EOF
(cd "$DIST_DIR" && sha256sum porter install.sh base-images/default/vmlinux base-images/default/rootfs.ext4 release/porter-release-manifest.json > release/SHA256SUMS)
tar -C "$DIST_DIR" -czf "$DIST_DIR/../$BASE_PACKAGE" base-images/default/vmlinux base-images/default/rootfs.ext4
tar -C "$DIST_DIR" -czf "$DIST_DIR/../$PACKAGE" porter install.sh install-porter.sh install-firecracker.sh postgres.sh porter.service porter.env.example FIRECRACKER_ARTIFACTS.md base-images release
PACKAGE_SHA256="$(sha256sum "$DIST_DIR/../$PACKAGE" | awk '{print $1}')"
BASE_PACKAGE_SHA256="$(sha256sum "$DIST_DIR/../$BASE_PACKAGE" | awk '{print $1}')"
printf '%s  %s\n' "$PACKAGE_SHA256" "$PACKAGE" > "$DIST_DIR/../$PACKAGE.sha256"
printf '%s  %s\n' "$BASE_PACKAGE_SHA256" "$BASE_PACKAGE" > "$DIST_DIR/../$BASE_PACKAGE.sha256"
printf '%s\n' "Built $DIST_DIR/../$PACKAGE" "Package SHA-256: $PACKAGE_SHA256" "Built $DIST_DIR/../$BASE_PACKAGE" "Base package SHA-256: $BASE_PACKAGE_SHA256" "Upload both files to GitHub Releases for $REPOSITORY/$RELEASE_TAG; no upload is performed by this script."
