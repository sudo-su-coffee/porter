import hashlib, json, urllib.request, urllib.error

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

digest = "sha256:" + hashlib.sha256(open("/tmp/sample-app/node-rootfs.ext4", "rb").read()).hexdigest()
print("rootfs digest:", digest)
st, b = call("POST", "/auth/login", {"username": "admin", "password": "porter-admin-123"})
ADM = json.loads(b)["token"]
st, b = call("GET", "/csrf", token=ADM)
H = {"X-CSRF-Token": json.loads(b)["csrf_token"]}
import time as _t
st, b = call("POST", "/projects", {"name": "node-sample-%d" % int(_t.time())}, token=ADM, headers=H)
PID = json.loads(b)["project"]["id"]
print("project:", st, PID)
st, b = call("POST", "/projects/%s/deployments" % PID,
             {"image": digest, "git_url": "https://github.com/heroku/node-js-sample"},
             token=ADM, headers=H)
print("create deployment:", st, b[:200])
st, b = call("GET", "/projects/%s/deployments" % PID, token=ADM)
print("list deployments:", st)
print(b[:600].decode("utf8", "replace"))
