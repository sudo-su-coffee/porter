#!/usr/bin/env bash
# Canonical Linux installer entrypoint.
# Development containers remain in scripts/backend/dev.sh; this entrypoint
# installs the Go daemon, embedded Vue dashboard, official Firecracker binary,
# systemd unit, and an explicit local/remote non-Docker PostgreSQL mode.
set -euo pipefail
REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
exec "$REPO_DIR/scripts/backend/install-linux.sh" "$@"
