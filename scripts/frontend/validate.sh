#!/usr/bin/env bash
# Validate the Vue dashboard without starting a development server.
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_DIR/frontend"
npm run build
cd "$REPO_DIR"
for script in scripts/backend/*.sh scripts/frontend/*.sh; do
  bash -n "$script"
done
echo "Frontend build and repository shell syntax checks passed."
