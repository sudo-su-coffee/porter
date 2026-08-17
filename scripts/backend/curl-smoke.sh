#!/usr/bin/env bash
set -Eeuo pipefail

# Porter production API smoke test.
# Required: PORTER_URL, PORTER_USERNAME, PORTER_PASSWORD, PORTER_PROJECT_ID.
# Optional: PORTER_REPLICA_INDEX (default 0), PORTER_ORG_ID.
# Run after the backend and PostgreSQL are started with a real Firecracker host.

: "${PORTER_URL:=http://127.0.0.1:8080}"
: "${PORTER_USERNAME:=admin}"
: "${PORTER_PASSWORD:?set PORTER_PASSWORD to the database-seeded bootstrap password}"
: "${PORTER_PROJECT_ID:?set PORTER_PROJECT_ID to an existing project UUID}"
: "${PORTER_REPLICA_INDEX:=0}"
: "${PORTER_ORG_ID:=}"

command -v curl >/dev/null || { echo "curl is required" >&2; exit 1; }
command -v jq >/dev/null || { echo "jq is required" >&2; exit 1; }

url="${PORTER_URL%/}"

request() {
  local method="$1" path="$2" body="${3:-}"
  local args=(--fail-with-body --silent --show-error -X "$method" "$url$path")
  args+=( -H "Authorization: Bearer $TOKEN" )
  [[ -n "$PORTER_ORG_ID" ]] && args+=( -H "X-Porter-Org-Id: $PORTER_ORG_ID" )
  if [[ "$method" != GET ]]; then
    args+=( -H "X-CSRF-Token: $CSRF" -H 'Content-Type: application/json' )
  fi
  [[ -n "$body" ]] && args+=( --data "$body" )
  curl "${args[@]}"
}

printf '1/8 health: '
curl --fail-with-body --silent --show-error "$url/health" | jq -c .
printf '2/8 readiness/database: '
curl --fail-with-body --silent --show-error "$url/healthz" | jq -c .
printf '3/8 login: '
login_json="$(curl --fail-with-body --silent --show-error -X POST "$url/login" -H 'Content-Type: application/json' --data "$(jq -nc --arg u "$PORTER_USERNAME" --arg p "$PORTER_PASSWORD" '{username:$u,password:$p}')")"
TOKEN="$(jq -er '.token' <<<"$login_json")"
printf '%s\n' "$login_json" | jq -c '{token_present:(.token != null),user:.user}'
printf '4/8 csrf: '
csrf_json="$(curl --fail-with-body --silent --show-error "$url/csrf" -H "Authorization: Bearer $TOKEN")"
CSRF="$(jq -er '.csrf_token' <<<"$csrf_json")"
printf '%s\n' "$csrf_json" | jq -c '{csrf_present:(.csrf_token != null)}'
printf '5/8 project: '
request GET "/projects/$PORTER_PROJECT_ID" | jq -c '{id,name,replicas_desired,vm_count:(.vms|length?)}'
printf '6/8 start replica: '
request POST "/projects/$PORTER_PROJECT_ID/replicas/$PORTER_REPLICA_INDEX/start" '{}' | jq -c .
printf '7/8 stop replica: '
request POST "/projects/$PORTER_PROJECT_ID/replicas/$PORTER_REPLICA_INDEX/stop" '{}' | jq -c .
printf '8/8 final VM state: '
request GET /vms | jq -c .

cat <<'EOF'
Smoke test passed. Snapshot checks are intentionally opt-in because they pause a live VM:
  curl -sS -X POST "$PORTER_URL/projects/$PORTER_PROJECT_ID/replicas/$PORTER_REPLICA_INDEX/snapshot" \
    -H "Authorization: Bearer $TOKEN" -H "X-CSRF-Token: $CSRF" -H 'Content-Type: application/json' -d '{}'
  curl -sS -X POST "$PORTER_URL/projects/$PORTER_PROJECT_ID/replicas/$PORTER_REPLICA_INDEX/restore" \
    -H "Authorization: Bearer $TOKEN" -H "X-CSRF-Token: $CSRF" -H 'Content-Type: application/json' -d '{}'
EOF
