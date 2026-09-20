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
            return r.status, r.read()
    except urllib.error.HTTPError as e:
        return e.code, e.read()
    except Exception as e:
        return -1, ("ERR:" + str(e)).encode()

st, b = call("POST", "/auth/login", {"username": "admin", "password": "porter-admin-123"})
ADM = json.loads(b)["token"]
st, b = call("GET", "/csrf", token=ADM)
H = {"X-CSRF-Token": json.loads(b)["csrf_token"]}
print("create dave:", call("POST", "/users", {"username": "dave", "password": "dave-pw-1", "role": "viewer"}, token=ADM, headers=H)[0])
st, b = call("POST", "/auth/login", {"username": "dave", "password": "dave-pw-1"})
DAVE = json.loads(b)["token"]
st, b = call("GET", "/projects", token=DAVE)
print("dave list:", st, b[:200])
st, b = call("GET", "/projects/aaf43af7-8b62-4599-ae04-0c29204750f6", token=DAVE)
print("dave detail:", st, b[:200])
print("cleanup dave:", call("DELETE", "/users/dave", token=ADM, headers=H)[0])
