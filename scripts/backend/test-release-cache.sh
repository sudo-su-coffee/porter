#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

INSTALLER="$WORK/install.sh"
cp "$ROOT_DIR/scripts/backend/install.sh" "$INSTALLER"
chmod 0755 "$INSTALLER"

CACHE="$WORK/cache"
PACKAGE_ROOT="$WORK/package-root"
FAKE_BIN="$WORK/fake-bin"
PACKAGE="porter-v1.0.0-beta-dev-x86_64.tar.gz"
MARKER="$WORK/installer-ran"
CALLS="$WORK/curl-calls"
mkdir -p "$CACHE" "$PACKAGE_ROOT" "$FAKE_BIN"

cat > "$PACKAGE_ROOT/install.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
: "${PORTER_TEST_MARKER:?}"
printf '%s\n' "${PORTER_POSTGRES_MODE:?}" > "${PORTER_TEST_MARKER}.mode"
touch "$PORTER_TEST_MARKER"
EOF
chmod 0755 "$PACKAGE_ROOT/install.sh"
tar -C "$PACKAGE_ROOT" -czf "$CACHE/$PACKAGE" install.sh
cp "$CACHE/$PACKAGE" "$WORK/good-package"

EXPECTED="$(sha256sum "$CACHE/$PACKAGE" | awk '{print $1}')"
printf '%s  %s\n' "$EXPECTED" "$PACKAGE" > "$CACHE/$PACKAGE.sha256"

cat > "$FAKE_BIN/curl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'curl-called\n' >> "${PORTER_TEST_CURL_CALLS:?}"
dest=''
while [ "$#" -gt 0 ]; do
  if [ "$1" = -o ]; then dest="$2"; shift 2; else shift; fi
done
case "$dest" in
  *.sha256|*.sha256.*) printf '%s  %s\n' "${PORTER_TEST_EXPECTED:?}" "$(basename "${PORTER_TEST_PACKAGE:?}")" > "$dest" ;;
  *) cp "${PORTER_TEST_PACKAGE:?}" "$dest" ;;
esac
EOF
chmod 0755 "$FAKE_BIN/curl"

sudo env \
  PATH="$FAKE_BIN:$PATH" \
  PORTER_CACHE_DIR="$CACHE" \
  PORTER_RELEASE_PACKAGE_SHA256="$EXPECTED" \
  PORTER_POSTGRES_MODE=local \
  PORTER_TEST_MARKER="$MARKER" \
  PORTER_TEST_CURL_CALLS="$CALLS" \
bash "$INSTALLER" x86_64
[ -f "$MARKER" ]
[ "$(cat "$MARKER.mode")" = local ]
[ ! -e "$CALLS" ]

rm -f "$MARKER"
cat "$INSTALLER" | sudo env \
  PATH="$FAKE_BIN:$PATH" \
  PORTER_CACHE_DIR="$CACHE" \
  PORTER_RELEASE_PACKAGE_SHA256="$EXPECTED" \
  PORTER_POSTGRES_MODE=local \
  PORTER_TEST_MARKER="$MARKER" \
  PORTER_TEST_CURL_CALLS="$CALLS" \
  bash -s x86_64
[ -f "$MARKER" ]
[ "$(cat "$MARKER.mode")" = local ]
[ ! -e "$CALLS" ]

rm -f "$MARKER"
printf corrupt > "$CACHE/$PACKAGE"
sudo env \
  PATH="$FAKE_BIN:$PATH" \
  PORTER_CACHE_DIR="$CACHE" \
  PORTER_TEST_EXPECTED="$EXPECTED" \
  PORTER_TEST_PACKAGE="$WORK/good-package" \
  PORTER_POSTGRES_MODE=local \
  PORTER_TEST_MARKER="$MARKER" \
  PORTER_TEST_CURL_CALLS="$CALLS" \
  bash "$INSTALLER" x86_64
[ -f "$MARKER" ]
[ -s "$CALLS" ]

printf '%s\n' 'release cache hit and corrupt-cache redownload checks passed'
