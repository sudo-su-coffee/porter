import json, sys, urllib.request

BASE = "http://localhost:8080/api/v1"

def call(method, path, body=None, token=None):
    req = urllib.request.Request(BASE + path, method=method,
        data=json.dumps(body).encode() if body is not None else None,
        headers={"Content-Type": "application/json"})
    if token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req, timeout=10) as r:
            return r.status, r.read()[:400]
    except urllib.error.HTTPError as e:
        return e.code, e.read()[:400]
    except Exception as e:
        return -1, str(e).encode()[:200]

import urllib.error

print("== public ==")
for p in ["/health", "/version"]:
    print(p, call("GET", p))

print("== login ==")
st, body = call("POST", "/auth/login", {"username": "admin", "password": "porter-admin-123"})
print(st, body)
tok = None
try:
    tok = json.loads(body).get("token") or json.loads(body).get("api_key")
except Exception:
    pass
print("TOKEN:", "yes" if tok else "NO")
