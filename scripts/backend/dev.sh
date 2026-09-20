#!/usr/bin/env bash
# Porter local dev — Docker Postgres for the backend (referenced by backend/Makefile `make dev`).
# Usage: bash scripts/backend/dev.sh up|down|logs  (run from the repo root)
set -euo pipefail

CONTAINER="${PORTER_DEV_PG:-porter-dev-pg}"
DB="${PORTER_DEV_DB:-porter}"
USER="${PORTER_DEV_USER:-porter}"
PASS="${PORTER_DEV_PASSWORD:-porter}"
PORT="${PORTER_DEV_PGPORT:-5433}"

cmd="${1:-up}"
case "$cmd" in
  up)
    if docker ps --format '{{.Names}}' | grep -qx "$CONTAINER"; then
      echo "dev pg already running ($CONTAINER)"
    else
      docker run -d --name "$CONTAINER" \
        -e POSTGRES_DB="$DB" -e POSTGRES_USER="$USER" -e POSTGRES_PASSWORD="$PASS" \
        -p "$PORT:5432" postgres:16-alpine
      echo "dev pg started on localhost:$PORT (db=$DB user=$USER)"
    fi
    echo "point the backend at it:"
    echo "  PORTER_DATABASE_URL=postgres://$USER:$PASS@localhost:$PORT/$DB?sslmode=disable"
    ;;
  down)
    docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
    echo "dev pg stopped"
    ;;
  logs)
    docker logs -f "$CONTAINER"
    ;;
  *)
    echo "usage: dev.sh up|down|logs" >&2
    exit 1
    ;;
esac
