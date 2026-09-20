import json, urllib.request, urllib.error

BASE = "http://localhost:8080/api/v1"
results = []

def call(method, path, body=None, token=None, headers=None):
    h = {"Content-Type": "application/json"}
    if token:
        h["Authorization"] = "Bearer " + token
    for k, v in (headers or {}).items():
        h[k] = v
    req = urllib.request.Request(BASE + path, method=method,
        data=json.dumps(body).encode() if body is not None else None, headers=h)
    try:
        with urllib.request.urlopen(req, timeout=15) as r:
            return r.status, r.read()[:600]
    except urllib.error.HTTPError as e:
        return e.code, e.read()[:600]
    except Exception as e:
        return -1, ("ERR:" + str(e)).encode()[:200]

def check(name, method, path, body=None, token=None, headers=None, want=(200, 201, 202, 304)):
    st, b = call(method, path, body, token, headers)
    ok = st in want
    results.append((ok, name, st, b[:160].decode("utf8", "replace")))
    return st, b

st, b = call("POST", "/auth/login", {"username": "admin", "password": "porter-admin-123"})
TOK = json.loads(b)["token"]
print("login:", st)
st, b = call("GET", "/csrf", token=TOK)
CSRF = json.loads(b).get("csrf_token", "")
print("csrf:", st, "token:" + ("yes" if CSRF else "NO"))

_orig_call = call
def call(method, path, body=None, token=None, headers=None):
    h = dict(headers or {})
    if method not in ("GET", "HEAD") and CSRF:
        h["X-CSRF-Token"] = CSRF
    return _orig_call(method, path, body, token, h)

check("users/me", "GET", "/users/me", token=TOK)
check("orgs", "GET", "/orgs", token=TOK)
check("orgs/default", "GET", "/orgs/default", token=TOK)
check("orgs/members", "GET", "/orgs/members", token=TOK)
check("orgs/events", "GET", "/orgs/events", token=TOK)
check("orgs/audit", "GET", "/orgs/audit", token=TOK)
check("roles", "GET", "/roles", token=TOK)
check("permissions", "GET", "/permissions", token=TOK)
check("groups", "GET", "/groups", token=TOK)
st, b = check("create-group", "POST", "/groups", {"name": "walk-g"}, token=TOK)
GID = ""
try:
    GID = json.loads(b).get("id", "")
except Exception:
    pass
check("servers", "GET", "/servers", token=TOK)
st, b = check("create-server", "POST", "/servers", {"hostname": "walk-node"}, token=TOK)
check("projects-list", "GET", "/projects", token=TOK)
st, b = check("create-project", "POST", "/projects", {"name": "walk-app"}, token=TOK)
PID = ""
try:
    PID = json.loads(b).get("id", "") or json.loads(b).get("project", {}).get("id", "")
except Exception:
    pass
print("PID:", PID)
if PID:
    P = "/projects/" + PID
    check("get-project", "GET", P, token=TOK)
    check("get-project-fields", "GET", P + "?fields=id,name", token=TOK)
    check("project-status", "GET", P + "/status", token=TOK)
    check("project-liveness", "GET", P + "/liveness", token=TOK)
    check("project-events", "GET", P + "/events", token=TOK)
    check("project-logs", "GET", P + "/logs", token=TOK)
    check("project-metrics", "GET", P + "/metrics", token=TOK)
    check("project-traffic", "GET", P + "/traffic", token=TOK)
    check("project-pool", "GET", P + "/pool", token=TOK)
    check("set-env", "POST", P + "/env", {"key": "WALK", "value": "1"}, token=TOK)
    check("list-env", "GET", P + "/env", token=TOK)
    check("create-secret", "POST", P + "/secrets", {"name": "WALK_KEY", "value": "s3cr3t"}, token=TOK)
    check("list-secrets", "GET", P + "/secrets", token=TOK)
    check("create-volume", "POST", P + "/volumes", {"name": "walk-vol", "size_mib": 100}, token=TOK)
    check("list-volumes", "GET", P + "/volumes", token=TOK)
    check("add-domain", "POST", P + "/domains", {"domain": "walk-app.porter.test"}, token=TOK)
    check("list-domains", "GET", P + "/domains", token=TOK)
    check("project-dns", "GET", P + "/dns", token=TOK)
    check("list-builds", "GET", P + "/builds", token=TOK)
    check("list-deployments", "GET", P + "/deployments", token=TOK)
    check("list-replicas", "GET", P + "/replicas", token=TOK)
    check("get-healthcheck", "GET", P + "/healthcheck", token=TOK)
    check("get-autoscale", "GET", P + "/autoscale", token=TOK)
    check("get-scale", "GET", P + "/scale", token=TOK)
    check("patch-scale", "PATCH", P + "/scale", {"replicas": 1}, token=TOK)
    check("crons", "GET", P + "/crons", token=TOK)
    check("hooks", "GET", P + "/hooks", token=TOK)
    check("alerts", "GET", P + "/alerts", token=TOK)
    check("drains", "GET", P + "/drains", token=TOK)
    check("redirects", "GET", P + "/redirects", token=TOK)
check("vms", "GET", "/vms", token=TOK)
check("images", "GET", "/images", token=TOK)
check("images-base", "GET", "/images/base", token=TOK)
check("guest-bases", "GET", "/guest-bases", token=TOK)
check("global-replicas", "GET", "/replicas", token=TOK)
check("usage", "GET", "/usage", token=TOK)
check("global-analytics", "GET", "/global/analytics", token=TOK)
check("host-overview", "GET", "/host/overview", token=TOK)
check("host-prereq", "GET", "/host/prerequisites", token=TOK)
check("overview", "GET", "/overview", token=TOK)
pass  # SSE streams tested separately (holding the connection == working)
check("unauth-denied", "GET", "/projects", want=(401,))

npass = sum(1 for r in results if r[0])
print("==== %d/%d passed ====" % (npass, len(results)))
for ok, name, st, b in results:
    if not ok:
        print("FAIL", name, st, b)
