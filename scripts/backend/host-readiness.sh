#!/usr/bin/env bash
# Read-only acceptance checks for an installed Porter Linux host.
set -euo pipefail

die() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
note() { printf 'OK: %s\n' "$*"; }

command -v curl >/dev/null 2>&1 || die "curl is required"
command -v systemctl >/dev/null 2>&1 || die "systemctl is required"

listen_addr="${PORTER_LISTEN_ADDR:-127.0.0.1:8080}"
host="127.0.0.1"
port="8080"
case "$listen_addr" in
  :*) port="${listen_addr#:}" ;;
  *:*) host="${listen_addr%:*}"; port="${listen_addr##*:}" ;;
  *) port="$listen_addr" ;;
esac
[ "$host" = "0.0.0.0" ] && host="127.0.0.1"
base_url="${PORTER_BASE_URL:-http://${host}:${port}}"

systemctl is-active --quiet porter.service || die "porter.service is not active"
note "porter.service is active"

health="$(curl --fail --silent --show-error --max-time 10 "$base_url/health")" || die "health endpoint failed at $base_url/health"
printf '%s' "$health" | grep -q '"status":"ok"' || die "health endpoint did not report status=ok"
note "health endpoint reports ok"

if [ "${PORTER_REQUIRE_METRICS:-0}" = "1" ]; then
  metrics="$(curl --fail --silent --show-error --max-time 10 "$base_url/metrics")" || die "metrics endpoint failed at $base_url/metrics"
  printf '%s' "$metrics" | grep -q '^porter_http_requests_total ' || die "metrics endpoint is missing porter_http_requests_total"
  note "Prometheus metrics endpoint is readable"
fi

if [ "${PORTER_REQUIRE_FIRECRACKER:-0}" = "1" ]; then
  firecracker_bin="${PORTER_FIRECRACKER_BIN:-/opt/porter/bin/firecracker}"
  kernel="${PORTER_KERNEL_IMAGE:-/var/lib/porter/base-images/default/vmlinux}"
  rootfs="${PORTER_ROOTFS_PATH:-/var/lib/porter/base-images/default/rootfs.ext4}"
  [ -x "$firecracker_bin" ] || die "Firecracker binary is not executable: $firecracker_bin"
  [ -r "$kernel" ] || die "guest kernel is not readable: $kernel"
  [ -r "$rootfs" ] || die "guest rootfs is not readable: $rootfs"
  [ -e /dev/kvm ] || die "/dev/kvm is not present"
  note "Firecracker binary, guest artifacts, and /dev/kvm are present"
fi

printf '%s\n' "Porter host readiness passed for $base_url"
