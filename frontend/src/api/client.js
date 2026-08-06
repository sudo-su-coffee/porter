// Thin wrapper around fetch() for the Porter Control API.
//
// Auth: a single admin login (configured in porter.toml, no user
// database) gates access to this dashboard. Signing in just hands back
// the same bearer token the Control API has always used — /login is a
// gate in front of that token, not a new auth scheme for the REST API
// itself.

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

export async function api(path, opts = {}) {
  const res = await fetch(path, {
    ...opts,
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${getToken()}`,
      ...(opts.headers || {}),
    },
  });

  if (res.status === 401) {
    clearToken();
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
  return body.token;
}
