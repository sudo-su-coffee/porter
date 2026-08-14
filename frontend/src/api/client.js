// Thin wrapper around fetch() for the Porter Control API.
//
// Auth: login and bearer tokens are resolved from the database-seeded RBAC
// model. Users, organizations, roles, permissions, sessions, and API keys
// are persisted resources; porter.toml contains no admin bypass or shared
// control token.
//
// CSRF: the Control API requires an X-CSRF-Token header on every
// state-changing (non-GET) request. We fetch it once from GET /csrf after
// login and attach it automatically. All dashboard writes (create project,
// scale, start/stop/restart, domains, teams, …) depend on this.

import { captureApiFailure } from "../observability";

const TOKEN_KEY = "porter_token";
const ORG_KEY = "porter_org_id";

export function getToken() {
  return localStorage.getItem(TOKEN_KEY) || "";
}

export function setToken(token) {
  localStorage.setItem(TOKEN_KEY, token);
}

export function clearToken() {
	localStorage.removeItem(TOKEN_KEY);
}

export function getOrgId() {
	return localStorage.getItem(ORG_KEY) || "";
}

export function setOrgId(orgId) {
	if (orgId) localStorage.setItem(ORG_KEY, orgId);
	else localStorage.removeItem(ORG_KEY);
}

// Set by main.js so the client can force a redirect to /login on 401
// without importing the router here (keeps this file dependency-free).
let onUnauthorized = () => {};
export function setUnauthorizedHandler(fn) {
  onUnauthorized = fn;
}

let csrfToken = "";
export function getCSRFToken() {
  return csrfToken;
}

// ensureCsrf fetches (and caches) the CSRF token via GET /csrf. GET is not
// CSRF-protected, so this works as soon as the bearer token is valid.
export async function ensureCsrf() {
  if (csrfToken) return csrfToken;
  try {
    const res = await fetch("/csrf", {
      headers: { Authorization: `Bearer ${getToken()}` },
    });
    if (res.ok) {
      const body = await res.json();
      csrfToken = body.csrf_token || "";
    }
  } catch (_) {
    // Keep the cached value (likely empty) — the actual request will surface
    // a 401/403 if auth is genuinely broken.
  }
  return csrfToken;
}

export async function api(path, opts = {}) {
  const method = (opts.method || "GET").toUpperCase();
	const headers = {
		"Content-Type": "application/json",
		Authorization: `Bearer ${getToken()}`,
		...(getOrgId() ? { "X-Porter-Org-Id": getOrgId() } : {}),
		...(opts.headers || {}),
	};
  if (method !== "GET" && method !== "HEAD") {
    const csrf = await ensureCsrf();
    if (csrf) headers["X-CSRF-Token"] = csrf;
  }

  const res = await fetch(path, { ...opts, method, headers });

  if (res.status === 401) {
    clearToken();
    csrfToken = "";
    onUnauthorized();
    throw new Error("Session expired — please sign in again");
  }

  let body = null;
  try {
    body = await res.json();
  } catch (_) {
    // No JSON body (e.g. 204 No Content) — fine.
  }

	if (!res.ok) {
		const err = new Error((body && body.error) || `HTTP ${res.status}`);
		err.status = res.status;
		captureApiFailure(err, {
			path,
			method,
			status: res.status,
			requestId: res.headers.get("X-Request-Id") || "",
		});
		throw err;
	}

  return body;
}

export async function login(username, password) {
  let res = await fetch("/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password }),
  });
  if (res.status === 404) {
    res = await fetch("/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username, password }),
    });
  }
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(body.error || "Login failed");
  setToken(body.token);
  csrfToken = ""; // invalidate, re-fetch with the fresh token
  await ensureCsrf();
  return body.token;
}

export function clearCSRF() {
  csrfToken = "";
}

// uploadCustomImage uploads the user's OWN microVM image (.zip with
// rootfs.ext4 + vmlinux) plus name/vcpu/mem. The daemon unpacks it and
// registers a "custom" image; returns the GoldenImage manifest.
export async function uploadCustomImage(file, { name, vcpus = 1, mem_mib = 256 }) {
  const csrf = await ensureCsrf();
  const fd = new FormData();
  fd.append("file", file);
  fd.append("name", name);
  fd.append("vcpus", String(vcpus));
  fd.append("mem_mib", String(mem_mib));
  const res = await fetch("/images/custom", {
    method: "POST",
    headers: {
      Authorization: `Bearer ${getToken()}`,
      ...(csrf ? { "X-CSRF-Token": csrf } : {}),
    },
    body: fd,
  });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(body.error || `HTTP ${res.status}`);
  return body;
}
