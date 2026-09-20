#!/usr/bin/env python3
"""Generate docs/postman/porter.postman_collection.json from apiRoutes in
backend/internal/api/api.go, so the collection can never drift from the code.

Usage:  python3 scripts/gen_postman.py
"""
import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
SRC = ROOT / "backend/internal/api/api.go"
OUT = ROOT / "docs/postman/porter.postman_collection.json"
ENV_OUT = ROOT / "docs/postman/porter.postman_environment.json"

src = SRC.read_text(encoding="utf-8")
body = src[src.index("var apiRoutes = []routeDef{"):]
ROUTES = [
    (m, p, perm, auth == "true")
    for m, p, perm, auth in re.findall(
        r'^\s*\{"(GET|POST|PUT|PATCH|DELETE|HEAD)",\s*"([^"]+)",\s*"([^"]*)",\s*(true|false),',
        body,
        re.M,
    )
]
if len(ROUTES) < 100:
    sys.exit(f"parsed only {len(ROUTES)} routes; api.go format changed?")

# Path params -> collection variables (set by the login/bootstrap flow or by hand).
PARAM_DEFAULT = {"projectId": "", "username": "", "roleId": "", "groupId": ""}

# Only the real request bodies the handlers require; everything else sends {}.
BODIES = {
    ("POST", "/auth/login"): {"username": "{{admin_user}}", "password": "{{admin_pass}}"},
    ("POST", "/login"): {"username": "{{admin_user}}", "password": "{{admin_pass}}"},
}

MUTATING = {"POST", "PUT", "PATCH", "DELETE"}

# Requests that must not run in a blind "run all" pass (destructive / need real infra).
DESTRUCTIVE = re.compile(r"(DELETE|/prune|/transfer|/restore|/recover|/exec|/ssh-cert|/purge)")


def group_of(path: str) -> str:
    parts = [x for x in path.split("/") if x]
    if not parts:
        return "root"
    top = parts[0]
    if top == "projects" and len(parts) > 2:
        return f"projects/{parts[2].split(':')[0]}"
    return top


def to_postman_path(p: str):
    return [seg for seg in re.sub(r"\{(\w+)\}", r":\1", p).split("/") if seg]


def prepare(method, path, perm, auth):
    tests = [
        "// Every route must answer; a 5xx is always a bug. 401/403/404 are valid",
        "// outcomes when the collection variable for a resource is unset.",
        "pm.test('no 5xx', () => pm.expect(pm.response.code).to.be.below(500));",
    ]
    if not auth:
        tests.append("pm.test('public route reachable', () => pm.expect(pm.response.code).to.not.equal(401));")
    return tests


def make_item(method, path, perm, auth):
    headers = []
    if auth:
        headers.append({"key": "Authorization", "value": "Bearer {{token}}"})
        if method in MUTATING:
            headers.append({"key": "X-CSRF-Token", "value": "{{csrf}}"})
    body = BODIES.get((method, path))
    if method in MUTATING and body is None:
        body = {}
    req = {
        "method": method,
        "header": headers,
        "url": {
            "raw": "{{base_url}}" + re.sub(r"\{(\w+)\}", r":\1", path),
            "host": ["{{base_url}}"],
            "path": to_postman_path(path),
            "variable": [
                {"key": k, "value": "{{" + k + "}}"} for k in re.findall(r"\{(\w+)\}", path)
            ],
        },
        "description": f"perm: `{perm or 'none'}` | auth: {'bearer' if auth else 'public'}",
    }
    if body is not None:
        headers.append({"key": "Content-Type", "value": "application/json"})
        req["body"] = {"mode": "raw", "raw": json.dumps(body, indent=2)}
    events = [
        {"listen": "test", "script": {"type": "text/javascript", "exec": prepare(method, path, perm, auth)}}
    ]
    if (method, path) in BODIES and path.endswith("login"):
        events[0]["script"]["exec"] += [
            "if (pm.response.code === 200) { pm.collectionVariables.set('token', pm.response.json().token); }"
        ]
    if path == "/csrf":
        events[0]["script"]["exec"] += [
            "if (pm.response.code === 200) { pm.collectionVariables.set('csrf', pm.response.json().csrf_token); }"
        ]
    name = f"{method} {path}"
    if DESTRUCTIVE.search(f"{method} {path}"):
        name = "[destructive] " + name
    return {"name": name, "event": events, "request": req}


def main():
    groups = {}
    for r in ROUTES:
        groups.setdefault(group_of(r[1]), []).append(make_item(*r))

    # Auth bootstrap first: login -> csrf. Everything after depends on these.
    bootstrap = [
        make_item("POST", "/auth/login", "", False),
        make_item("GET", "/csrf", "", True),
    ]

    # RBAC negative cases: run with member_token (a non-admin). Each must be 401/403.
    def neg(name, method, path, headers_auth, expect, body=None):
        h = []
        if headers_auth == "none":
            pass
        elif headers_auth == "member":
            h.append({"key": "Authorization", "value": "Bearer {{member_token}}"})
            if method in MUTATING:
                h.append({"key": "X-CSRF-Token", "value": "{{csrf}}"})
        elif headers_auth == "admin_no_csrf":
            h.append({"key": "Authorization", "value": "Bearer {{token}}"})
        if body is not None:
            h.append({"key": "Content-Type", "value": "application/json"})
        req = {
            "method": method,
            "header": h,
            "url": {"raw": "{{base_url}}" + path, "host": ["{{base_url}}"], "path": [x for x in path.split("/") if x]},
        }
        if body is not None:
            req["body"] = {"mode": "raw", "raw": json.dumps(body)}
        return {
            "name": name,
            "event": [
                {
                    "listen": "test",
                    "script": {
                        "type": "text/javascript",
                        "exec": [f"pm.test('expect {expect}', () => pm.expect(pm.response.code).to.be.oneOf({json.dumps(expect)}));"],
                    },
                }
            ],
            "request": req,
        }

    negatives = [
        neg("no token -> 401 on /projects", "GET", "/projects", "none", [401]),
        neg("no token -> 401 on /users", "GET", "/users", "none", [401]),
        neg("bad token -> 401", "GET", "/projects", "none", [401]),
        neg("write without CSRF -> 403", "POST", "/projects", "admin_no_csrf", [403], {}),
        neg("member cannot list users -> 403", "GET", "/users", "member", [403]),
        neg("member cannot create role -> 403", "POST", "/roles", "member", [403], {}),
        neg("member cannot delete user -> 403", "DELETE", "/users/nobody", "member", [403]),
        neg("member cannot register server -> 403", "POST", "/servers", "member", [403], {}),
        neg("member cannot prune images -> 403", "POST", "/images/prune", "member", [403], {}),
        neg("wrong login -> 401", "POST", "/auth/login", "none", [401], {"username": "nobody", "password": "wrong"}),
    ]

    collection = {
        "info": {
            "name": "Porter API",
            "description": (
                "Generated by scripts/gen_postman.py from apiRoutes in backend/internal/api/api.go "
                f"({len(ROUTES)} routes). Run folder '00 bootstrap' first, then any group. "
                "Set member_token for the RBAC negative folder. Requests tagged [destructive] "
                "mutate real state; skip them in blind runs."
            ),
            "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
        },
        "variable": [
            {"key": "base_url", "value": "http://127.0.0.1:8080"},
            {"key": "admin_user", "value": "admin"},
            {"key": "admin_pass", "value": ""},
            {"key": "token", "value": ""},
            {"key": "csrf", "value": ""},
            {"key": "member_token", "value": ""},
        ]
        + [{"key": k, "value": v} for k, v in sorted(PARAM_DEFAULT.items())]
        + [
            {"key": k, "value": ""}
            for k in sorted({m for _, p, _, _ in ROUTES for m in re.findall(r"\{(\w+)\}", p)} - set(PARAM_DEFAULT))
        ],
        "item": [
            {"name": "00 bootstrap", "item": bootstrap},
            {"name": "01 rbac negative", "item": negatives},
        ]
        + [{"name": f"10 {g}", "item": items} for g, items in sorted(groups.items())],
    }

    OUT.parent.mkdir(parents=True, exist_ok=True)
    OUT.write_text(json.dumps(collection, indent=2) + "\n", encoding="utf-8")
    env = {
        "name": "Porter local",
        "values": [
            {"key": "base_url", "value": "http://127.0.0.1:8080", "enabled": True},
            {"key": "admin_user", "value": "admin", "enabled": True},
            {"key": "admin_pass", "value": "", "type": "secret", "enabled": True},
        ],
        "_postman_variable_scope": "environment",
    }
    ENV_OUT.write_text(json.dumps(env, indent=2) + "\n", encoding="utf-8")
    n_req = sum(len(g["item"]) for g in collection["item"])
    print(f"wrote {OUT.relative_to(ROOT)}: {len(ROUTES)} routes, {n_req} requests")


if __name__ == "__main__":
    main()
