#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
FAKE_BIN="$WORK/bin"
OUTPUT="$WORK/output"
CHILD="$WORK/child.sh"
mkdir -p "$FAKE_BIN"

cat > "$FAKE_BIN/psql" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
chmod 0755 "$FAKE_BIN/psql"

cat > "$CHILD" <<EOF
#!/usr/bin/env bash
set -euo pipefail
. "$ROOT_DIR/scripts/backend/postgres.sh"
unset PORTER_POSTGRES_MODE PORTER_DATABASE_URL
porter_pg_setup
printf 'mode=%s\nurl=%s\n' "\${PORTER_POSTGRES_MODE:-remote}" "\$PORTER_DATABASE_URL" > "$OUTPUT"
EOF
chmod 0755 "$CHILD"

printf '2\npostgres://porter:secret@db.example.test:5432/porter?sslmode=require\n' \
  | script -qefc "env PATH=\"$FAKE_BIN:\$PATH\" bash \"$CHILD\"" /dev/null >/dev/null

grep -qx 'mode=remote' "$OUTPUT"
grep -qx 'url=postgres://porter:secret@db.example.test:5432/porter?sslmode=require' "$OUTPUT"
printf '%s\n' 'interactive PostgreSQL mode and URL prompts passed'
