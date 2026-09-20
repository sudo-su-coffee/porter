import json, urllib.request, urllib.error

BASE = "http://localhost:8080/api/v1"

def call(method, path, body=None, token=None):
    h = {"Content-Type": "application/json"}
    if token:
        h["Authorization"] = "Bearer " + token
    req = urllib.request.Request(BASE + path, method=method,
        data=json.dumps(body).encode() if body is not None else None, headers=h)
    try:
        with urllib.request.urlopen(req, timeout=15) as r:
            return r.status, r.read()[:1500]
    except urllib.error.HTTPError as e:
        return e.code, e.read()[:500]

st, b = call("POST", "/auth/login", {"username": "admin", "password": "porter-admin-123"})
ADM = json.loads(b)["token"]
for p in ["/images", "/images/base", "/images/base/readiness", "/guest-bases"]:
    st, b = call("GET", p, token=ADM)
    print(p, st)
    print(b[:800].decode("utf8", "replace"))
    print("---")
