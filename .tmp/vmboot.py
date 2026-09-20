import json, time, urllib.request, urllib.error

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
            return r.status, json.loads(r.read() or b"null")
    except urllib.error.HTTPError as e:
        return e.code, {"err": e.read()[:200].decode("utf8", "replace")}
    except Exception as e:
        return -1, {"err": str(e)[:200]}

st, b = call("POST", "/auth/login", {"username": "admin", "password": "porter-admin-123"})
TOK = b["token"]
st, b = call("GET", "/csrf", token=TOK)
CSRF = b["csrf_token"]
H = {"X-CSRF-Token": CSRF}

st, projs = call("GET", "/projects", token=TOK)
pid = [p for p in projs if p.get("name") == "walk-app"][0]["id"]
print("PID:", pid)
for i in range(4):
    st, vms = call("GET", "/projects/%s/replicas" % pid, token=TOK)
    print("t+%ds replicas=%s" % (i * 20, json.dumps(vms)[:600]))
    time.sleep(20)
st, pool = call("GET", "/projects/%s/pool" % pid, token=TOK)
print("pool:", json.dumps(pool)[:400])
