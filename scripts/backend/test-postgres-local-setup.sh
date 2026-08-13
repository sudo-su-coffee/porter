#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
BIN="$WORK/bin"
LOG="$WORK/commands.log"
mkdir -p "$BIN"

cat > "$BIN/apt-get" <<'EOF'
#!/usr/bin/env bash
printf 'apt-get %s\n' "$*" >> "${PORTER_PG_TEST_LOG:?}"
EOF
cat > "$BIN/curl" <<'EOF'
#!/usr/bin/env bash
printf 'curl %s\n' "$*" >> "${PORTER_PG_TEST_LOG:?}"
printf '%s\n' 'fake-postgresql-signing-key'
EOF
cat > "$BIN/gpg" <<'EOF'
#!/usr/bin/env bash
output=''
while [ "$#" -gt 0 ]; do
  if [ "$1" = -o ]; then output="$2"; shift 2; else shift; fi
done
[ -n "$output" ] && : > "$output"
EOF
cat > "$BIN/pg_isready" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
cat > "$BIN/systemctl" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
cat > "$BIN/service" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
cat > "$BIN/runuser" <<'EOF'
#!/usr/bin/env bash
shift 2
shift
exec "$@"
EOF
cat > "$BIN/createdb" <<'EOF'
#!/usr/bin/env bash
printf 'createdb %s\n' "$*" >> "${PORTER_PG_TEST_LOG:?}"
EOF
cat > "$BIN/psql" <<'EOF'
#!/usr/bin/env bash
input="$(cat)"
printf 'psql %s\n%s\n' "$*" "$input" >> "${PORTER_PG_TEST_LOG:?}"
case "$*" in
  *"FROM pg_roles"*) exit 1 ;;
  *"FROM pg_database"*) exit 1 ;;
  *) exit 0 ;;
esac
EOF
chmod 0755 "$BIN"/*

PATH="$BIN:/usr/bin:/bin" PORTER_PG_TEST_LOG="$LOG" \
  PORTER_PG_PGDG_KEYRING="$WORK/pgdg/apt.postgresql.org.gpg" \
  PORTER_PG_PGDG_SOURCES="$WORK/pgdg.list" bash -c '
  . "$1/scripts/backend/postgres.sh"
  porter_pg_install_packages
  PORTER_PG_PASSWORD=fixed-test-password porter_pg_setup_local
' _ "$ROOT_DIR"

grep -Fqx 'apt-get update' "$LOG"
grep -Fqx 'apt-get install -y --no-install-recommends ca-certificates gnupg' "$LOG"
grep -Fqx 'apt-get install -y --no-install-recommends postgresql postgresql-client' "$LOG"
grep -F 'NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS' "$LOG" >/dev/null
grep -Fqx 'createdb -O porter porter' "$LOG"

: > "$LOG"
PATH="$BIN:/usr/bin:/bin" PORTER_PG_TEST_LOG="$LOG" \
  PORTER_DATABASE_URL='postgres://porter:fixed-test-password@127.0.0.1:5432/porter?sslmode=disable' \
  bash -c '
    . "$1/scripts/backend/postgres.sh"
    porter_pg_setup_local
  ' _ "$ROOT_DIR"
! grep -E 'ALTER ROLE|CREATE ROLE|createdb' "$LOG" >/dev/null

printf '%s\n' 'automatic local PostgreSQL setup checks passed'
