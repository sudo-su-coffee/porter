import json, urllib.request, urllib.error

BASE = "http://localhost:8080/api/v1"

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
            return r.status, r.read()[:800]
    except urllib.error.HTTPError as e:
        return e.code, e.read()[:800]
    except Exception as e:
        return -1, ("ERR:" + str(e)).encode()[:200]

st, b = call("POST", "/auth/login", {"username": "admin", "password": "porter-admin-123"})
ADM = json.loads(b)["token"]
st, b = call("GET", "/csrf", token=ADM)
H = {"X-CSRF-Token": json.loads(b)["csrf_token"]}
print("admin creates carol(viewer):", call("POST", "/users", {"username": "carol", "password": "carol-pw-1", "role": "viewer"}, token=ADM, headers=H)[0])
st, b = call("POST", "/auth/login", {"username": "carol", "password": "carol-pw-1"})
CAROL = json.loads(b)["token"]
st, b = call("GET", "/projects", token=CAROL)
print("carol list projects:", st, b[:300])
st, b = call("GET", "/projects", token=ADM)
print("admin list projects:", st, b[:300])
WALK = "aaf43af7-8b62-4599-ae04-0c29204750f6"
st, b = call("GET", "/projects/%s/secrets" % WALK, token=CAROL)
print("carol reads walk-app secrets:", st, b[:300])
st, b = call("GET", "/projects/%s/env" % WALK, token=CAROL)
print("carol reads walk-app env:", st, b[:300])
st, b = call("POST", "/projects", {"name": "carol-app"}, token=CAROL, headers=H)
print("carol creates project:", st, b[:300])
print("cleanup carol:", call("DELETE", "/users/carol", token=ADM, headers=H)[0])
