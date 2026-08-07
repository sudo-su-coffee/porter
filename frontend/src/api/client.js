// Thin wrapper around fetch() for the Porter Control API.
//
// Auth: a single admin login (configured in porter.toml, no user
// database) gates access to this dashboard. Signing in just hands back
// the same bearer token the Control API has always used — /login is a
// gate in front of that token, not a new auth scheme for the REST API
// itself.
//
// CSRF: the Control API requires an X-CSRF-Token header on every
// state-changing (non-GET) request. We fetch it once from GET /csrf after
// login and attach it automatically. All dashboard writes (create project,
// scale, start/stop/restart, domains, teams, …) depend on this.

const TOKEN_KEY = "porter_token";

export function getToken() {
  return localStorage.getItem(TOKEN_KEY) || "";
}

export function setToken(token) {
  localStorage.setItem(TOKEN_KEY, token);
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY);
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
    throw err;
  }

  return body;
}

export async function login(username, password) {
  const res = await fetch("/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password }),
  });
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
