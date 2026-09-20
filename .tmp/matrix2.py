import json, urllib.request, urllib.error

BASE = "http://localhost:8080/api/v1"
T = []

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
            return r.status, r.read()
    except urllib.error.HTTPError as e:
        return e.code, e.read()
    except Exception as e:
        return -1, ("ERR:" + str(e)).encode()

def expect(name, got, want):
    T.append((name, got, want, got in want))

st, b = call("POST", "/auth/login", {"username": "admin", "password": "porter-admin-123"})
ADM = json.loads(b)["token"]
st, b = call("GET", "/csrf", token=ADM)
H = {"X-CSRF-Token": json.loads(b)["csrf_token"]}
expect("admin role is admin", json.loads(call("GET", "/users/me", token=ADM)[1])["role"], ("admin",))
expect("roles list has 2", len(json.loads(call("GET", "/roles", token=ADM)[1])), (2,))
expect("admin creates erin (no role -> default member)", call("POST", "/users", {"username": "erin", "password": "erin-pw-1"}, token=ADM, headers=H)[0], (201,))
st, b = call("GET", "/groups", token=ADM)
groups = json.loads(b)
gnames = [g.get("name") for g in (groups if isinstance(groups, list) else groups.get("groups", []))]
expect("erin personal team exists", 200 if "erin-team" in gnames else -1, (200,))
st, b = call("POST", "/auth/login", {"username": "erin", "password": "erin-pw-1"})
ERIN = json.loads(b)["token"]
st, b = call("GET", "/projects", token=ERIN)
items = json.loads(b)
items = items if isinstance(items, list) else items.get("projects", [])
expect("erin sees empty list", 200 if items == [] else -1, (200,))
import time as _t
APPNAME = "erin-app-%d" % int(_t.time())
st, b = call("POST", "/projects", {"name": APPNAME}, token=ERIN, headers=H)
expect("erin creates project", st, (200, 201, 202))
EPID = ""
try:
    d = json.loads(b)
    EPID = (d.get("project", {}) or d).get("id", "")
except Exception:
    pass
st, b = call("GET", "/projects", token=ERIN)
items = json.loads(b)
items = items if isinstance(items, list) else items.get("projects", [])
expect("erin sees own project", 200 if any(p.get("name") == APPNAME for p in items) else -1, (200,))
expect("admin still sees walk-app", 200 if any("walk-app" in json.dumps(p) for p in [json.loads(call("GET", "/projects", token=ADM)[1])]) else -1, (200,))
st, b = call("GET", "/projects/%s/secrets" % EPID, token=ADM)
expect("admin reads member project secrets", st, (200,))
expect("creator recorded", json.loads(call("GET", "/projects/" + EPID, token=ERIN)[1]).get("created_by"), ("erin",))
expect("delete erin-app", call("DELETE", "/projects/" + EPID, token=ERIN, headers=H)[0], (200,))
expect("delete user erin", call("DELETE", "/users/erin", token=ADM, headers=H)[0], (200,))

npass = sum(1 for r in T if r[3])
print("==== %d/%d matrix checks passed ====" % (npass, len(T)))
for name, got, want, ok in T:
    print(("PASS " if ok else "FAIL "), name, "got=%s want=%s" % (got, want))
