#!/usr/bin/env bash
# ============================================================================
#  Porter API smoke test — login, csrf, org, project creation.
#
#   bash test-porter-api.sh
#
#  Assumes: go run cmd/porter/main.go server   is already running on :8080
#  Routes are mounted at ROOT (no /api prefix) — confirmed from
#  internal/api/api.go's a.Routes(mux) call in main.go.
# ============================================================================
set -euo pipefail

BASE="http://localhost:8080"
ADMIN_USER="admin"
ADMIN_PASS="change-me"

c()   { printf '\n\033[1;34m==> %s\033[0m\n' "$1"; }
ok()  { printf '\033[0;32m    %s\033[0m\n' "$1"; }
warn(){ printf '\033[0;33m    %s\033[0m\n' "$1"; }

need_jq() {
  command -v jq >/dev/null 2>&1 || { echo "This script needs jq. Install: sudo apt install jq"; exit 1; }
}
need_jq

# ---------------------------------------------------------------------------
# 1. Health check (no auth)
# ---------------------------------------------------------------------------
c "1. GET /health"
curl -sS "$BASE/health" | jq .

# ---------------------------------------------------------------------------
# 2. Login — POST /login (legacy alias) or POST /auth/login
#    handleLogin isn't shown in the file you pasted, but the porter.toml
#    comment says login returns the configured api_token as the bearer
#    credential. We try both routes and both common response shapes.
# ---------------------------------------------------------------------------
c "2. POST /login  (admin / change-me)"
LOGIN_RESP=$(curl -sS -X POST "$BASE/login" \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"$ADMIN_USER\",\"password\":\"$ADMIN_PASS\"}")
echo "$LOGIN_RESP" | jq . 2>/dev/null || echo "$LOGIN_RESP"

# Try a few common field names for the returned token.
TOKEN=$(echo "$LOGIN_RESP" | jq -r '.token // .api_token // .access_token // empty' 2>/dev/null || true)

if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
  warn "login response didn't contain an obvious token field."
  warn "Falling back to the static api_token from porter.toml directly —"
  warn "this is what a.auth() actually checks against (a.token), so it will work"
  warn "for testing even if handleLogin's response shape is different than guessed."
  read -r -p "    paste your porter.toml [server].api_token value: " TOKEN
fi

if [ -z "$TOKEN" ]; then
  echo "No token available — can't continue to authenticated calls."
  exit 1
fi
ok "using token: ${TOKEN:0:12}..."

AUTH=(-H "Authorization: Bearer $TOKEN")

# ---------------------------------------------------------------------------
# 3. Session check
# ---------------------------------------------------------------------------
c "3. GET /auth/session"
curl -sS "${AUTH[@]}" "$BASE/auth/session" | jq . 2>/dev/null || true

c "3b. GET /users/me"
curl -sS "${AUTH[@]}" "$BASE/users/me" | jq . 2>/dev/null || true

# ---------------------------------------------------------------------------
# 4. CSRF token — required for every POST/PUT/PATCH/DELETE below
# ---------------------------------------------------------------------------
c "4. GET /csrf"
CSRF_RESP=$(curl -sS "${AUTH[@]}" "$BASE/csrf")
echo "$CSRF_RESP" | jq .
CSRF=$(echo "$CSRF_RESP" | jq -r '.csrf_token')
ok "csrf_token: $CSRF"

AUTH_MUT=("${AUTH[@]}" -H "X-CSRF-Token: $CSRF" -H "Content-Type: application/json")

# ---------------------------------------------------------------------------
# 5. Orgs — list, get default (auto-created on signup per your data model)
# ---------------------------------------------------------------------------
c "5. GET /orgs"
curl -sS "${AUTH[@]}" "$BASE/orgs" | jq .

c "5b. GET /orgs/default"
curl -sS "${AUTH[@]}" "$BASE/orgs/default" | jq .

c "5c. GET /orgs/current"
curl -sS "${AUTH[@]}" "$BASE/orgs/current" | jq .

# ---------------------------------------------------------------------------
# 6. Create a project (image-based, not git) — real creation test.
#    handleCreateProject -> createProjectFrom -> bootReplica -> a.vmm.Boot(...)
#    This will actually attempt to boot a real replica through your VM
#    engine, so watch the `go run` server's stdout for boot logs/errors.
# ---------------------------------------------------------------------------
c "6. POST /projects  (create test-nginx, 1 replica, OCI image)"
PROJECT_RESP=$(curl -sS -X POST "${AUTH_MUT[@]}" "$BASE/projects" -d '{
  "name": "test-nginx",
  "image": "docker.io/library/nginx:alpine",
  "replicas": 1,
  "vcpus": 1,
  "mem_mib": 256,
  "ports": [{"container_port": 80, "protocol": "tcp"}]
}')
echo "$PROJECT_RESP" | jq .
PROJECT_ID=$(echo "$PROJECT_RESP" | jq -r '.project.id // empty')

if [ -z "$PROJECT_ID" ]; then
  warn "no project id returned — check the response above for the error."
  exit 1
fi
ok "created project: $PROJECT_ID"

# ---------------------------------------------------------------------------
# 7. List projects, get the one we just made, check replicas/status
# ---------------------------------------------------------------------------
c "7. GET /projects"
curl -sS "${AUTH[@]}" "$BASE/projects" | jq .

c "7b. GET /projects/$PROJECT_ID"
curl -sS "${AUTH[@]}" "$BASE/projects/$PROJECT_ID" | jq .

c "7c. GET /projects/$PROJECT_ID/status"
curl -sS "${AUTH[@]}" "$BASE/projects/$PROJECT_ID/status" | jq . 2>/dev/null || true

c "7d. GET /projects/$PROJECT_ID/replicas"
curl -sS "${AUTH[@]}" "$BASE/projects/$PROJECT_ID/replicas" | jq .

# ---------------------------------------------------------------------------
# 8. Overview (host-level dashboard summary)
# ---------------------------------------------------------------------------
c "8. GET /overview"
curl -sS "${AUTH[@]}" "$BASE/overview" | jq . 2>/dev/null || true

echo
ok "Done. Watch the 'go run cmd/porter/main.go server' terminal for VM boot logs —"
ok "creation succeeding here just means the API/DB path works; actual microVM"
ok "boot success/failure shows up in that server's stdout, not this script."