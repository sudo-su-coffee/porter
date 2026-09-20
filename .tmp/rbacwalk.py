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
            return r.status, r.read()[:500]
    except urllib.error.HTTPError as e:
        return e.code, e.read()[:500]
    except Exception as e:
        return -1, ("ERR:" + str(e)).encode()[:200]

def expect(name, got, want):
    T.append((name, got, want, got in want))

st, b = call("POST", "/auth/login", {"username": "admin", "password": "porter-admin-123"})
ADM = json.loads(b)["token"]
st, b = call("GET", "/csrf", token=ADM)
H = {"X-CSRF-Token": json.loads(b)["csrf_token"]}

expect("create viewer bob", call("POST", "/users", {"username": "bob", "password": "bob-pass-1", "role": "viewer"}, token=ADM, headers=H)[0], (201,))
st, b = call("POST", "/auth/login", {"username": "bob", "password": "bob-pass-1"})
BOB = json.loads(b)["token"]
expect("bob login", st, (200,))
expect("bob reads projects pre-grant (403)", call("GET", "/projects", token=BOB)[0], (403,))
expect("bob creates project pre-grant (403)", call("POST", "/projects", {"name": "bob-app"}, token=BOB, headers=H)[0], (403,))
expect("bob lists users (403)", call("GET", "/users", token=BOB)[0], (403,))
expect("bob reads roles (403)", call("GET", "/roles", token=BOB)[0], (403,))
expect("bob reads secrets w/o membership (403/404)", call("GET", "/projects/x/secrets", token=BOB)[0], (403, 404))
expect("grant bob member (lowercase)", call("POST", "/orgs/members", {"username": "bob", "role": "member"}, token=ADM, headers=H)[0], (200, 201))
st, b = call("POST", "/projects", {"name": "bob-app"}, token=BOB, headers=H)
expect("bob creates project post-grant", st, (200, 201))
PID = ""
try:
    d = json.loads(b)
    PID = d.get("id", "") if isinstance(d, dict) else ""
except Exception:
    pass
st, b = call("GET", "/projects", token=BOB)
names = []
try:
    d = json.loads(b)
    items = d if isinstance(d, list) else d.get("projects", d.get("items", []))
    names = [p.get("name") for p in items if isinstance(p, dict)]
except Exception:
    pass
expect("bob sees own project in list", 200 if "bob-app" in names else -1, (200,))
if PID:
    expect("bob deletes own project", call("DELETE", "/projects/" + PID, token=BOB, headers=H)[0], (200,))
expect("revoke bob", call("DELETE", "/orgs/members/bob", token=ADM, headers=H)[0], (200,))
expect("bob reads projects post-revoke (403)", call("GET", "/projects", token=BOB)[0], (403,))
expect("delete user bob", call("DELETE", "/users/bob", token=ADM, headers=H)[0], (200,))
expect("no-token denied (401)", call("GET", "/projects")[0], (401,))

npass = sum(1 for r in T if r[3])
print("==== %d/%d RBAC checks passed ====" % (npass, len(T)))
for name, got, want, ok in T:
    print(("PASS " if ok else "FAIL "), name, "got=%s want=%s" % (got, want))
