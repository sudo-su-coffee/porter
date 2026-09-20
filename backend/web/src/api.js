// Minimal API client for the Porter control plane.
// Auth: Bearer <token>; non-GET requests additionally send X-CSRF-Token.

const BASE = '/api/v1'

const store = {
  get token() {
    return localStorage.getItem('porter_token') || ''
  },
  set token(v) {
    v ? localStorage.setItem('porter_token', v) : localStorage.removeItem('porter_token')
  },
  get user() {
    try {
      return JSON.parse(localStorage.getItem('porter_user') || '{}')
    } catch {
      return {}
    }
  },
  set user(v) {
    localStorage.setItem('porter_user', JSON.stringify(v))
  }
}

let csrfCache = ''
async function getCSRF() {
  if (csrfCache) return csrfCache
  try {
    const r = await fetch(`${BASE}/csrf`, { headers: { Authorization: `Bearer ${store.token}` } })
    if (r.ok) {
      const body = await r.json()
      csrfCache = body.token || body.csrf || ''
    }
  } catch {
    /* csrf is best-effort */
  }
  return csrfCache
}

export async function doFetch(path, { method = 'GET', body } = {}) {
  const headers = { 'Content-Type': 'application/json' }
  if (store.token) headers.Authorization = `Bearer ${store.token}`
  if (method !== 'GET' && method !== 'HEAD') headers['X-CSRF-Token'] = await getCSRF()
  let res
  try {
    res = await fetch(`${BASE}${path}`, {
      method,
      headers,
      body: body !== undefined ? JSON.stringify(body) : undefined
    })
  } catch (e) {
    throw new Error(`network error: ${e.message}`)
  }
  const text = await res.text()
  let data = null
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      data = { raw: text }
    }
  }
  if (!res.ok) {
    const err = new Error((data && data.error) || `HTTP ${res.status}`)
    err.status = res.status
    throw err
  }
  return data
}

export const api = {
  // auth
  login: (username, password) =>
    doFetch('/auth/login', { method: 'POST', body: { username, password } }),
  logout: () => doFetch('/auth/logout', { method: 'POST', body: {} }),
  me: () => doFetch('/users/me'),

  // host / health
  hostOverview: () => doFetch('/host/overview'),
  hostPrereqs: () => doFetch('/host/prerequisites'),
  health: () => doFetch('/health'),
  version: () => doFetch('/version'),

  // projects
  projects: () => doFetch('/projects'),
  createProject: (body) => doFetch('/projects', { method: 'POST', body }),
  project: (id) => doFetch(`/projects/${id}`),
  deleteProject: (id) => doFetch(`/projects/${id}`, { method: 'DELETE' }),
  status: (id) => doFetch(`/projects/${id}/status`),

  // replicas (VMs)
  replicas: (id) => doFetch(`/projects/${id}/replicas`),
  replicaStart: (id, n) => doFetch(`/projects/${id}/replicas/${n}/start`, { method: 'POST', body: {} }),
  replicaStop: (id, n) => doFetch(`/projects/${id}/replicas/${n}/stop`, { method: 'POST', body: {} }),
  replicaRestart: (id, n) => doFetch(`/projects/${id}/replicas/${n}/restart`, { method: 'POST', body: {} }),
  replicaSnapshot: (id, n) => doFetch(`/projects/${id}/replicas/${n}/snapshot`, { method: 'POST', body: {} }),

  // networks / domains (project-scoped)
  networks: (id) => doFetch(`/projects/${id}/networks`),
  createNetwork: (id, body) => doFetch(`/projects/${id}/networks`, { method: 'POST', body }),
  domains: (id) => doFetch(`/projects/${id}/domains`),
  createDomain: (id, body) => doFetch(`/projects/${id}/domains`, { method: 'POST', body }),

  // images
  images: () => doFetch('/images'),
  guestBases: () => doFetch('/guest-bases'),

  // rbac / org
  users: () => doFetch('/users'),
  org: () => doFetch('/orgs/current'),
  roles: () => doFetch('/roles'),

  raw: doFetch
}

export { store }