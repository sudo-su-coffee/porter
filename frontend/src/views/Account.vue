<!-- Porter dashboard account surface: persisted profile/session/API-key state, no shared admin fallback. -->
<script setup>
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { api, clearToken } from "../api/client";
import { toast } from "../components/toast";

const router = useRouter();
const profile = ref(null);
const session = ref(null);
const keys = ref([]);
const keyName = ref("");
const createdToken = ref("");
const loading = ref(true);
const error = ref("");
const busy = ref(false);

async function load() {
  loading.value = true;
  error.value = "";
  try {
    const [me, current, apiKeys] = await Promise.all([api("/users/me"), api("/auth/session"), api("/users/me/api-keys")]);
    profile.value = me;
    session.value = current;
    keys.value = Array.isArray(apiKeys) ? apiKeys : apiKeys?.keys || [];
  } catch (err) {
    error.value = err.message;
  } finally {
    loading.value = false;
  }
}

async function createKey() {
  busy.value = true;
  try {
    const result = await api("/users/me/api-keys", { method: "POST", body: JSON.stringify({ name: keyName.value.trim() || "dashboard-key" }) });
    createdToken.value = result?.token || "";
    keyName.value = "";
    toast("API key created. Copy the token now; it is only returned once.", "success");
    await load();
  } catch (err) { toast(err.message, "error"); }
  finally { busy.value = false; }
}

async function revokeKey(key) {
  if (!confirm(`Revoke API key ${key.name || key.id}?`)) return;
  try { await api(`/users/me/api-keys/${encodeURIComponent(key.id)}`, { method: "DELETE" }); toast("API key revoked", "success"); await load(); }
  catch (err) { toast(err.message, "error"); }
}

async function logout() {
  try {
    try { await api("/auth/logout", { method: "POST" }); }
    catch (err) { if (err.status === 404) await api("/logout", { method: "POST" }); else throw err; }
  } catch (_) { /* local session still must be cleared */ }
  clearToken();
  router.push({ name: "login" });
}

async function requestUnsupportedProfileUpdate() {
  try { await api("/users/me", { method: "PATCH", body: JSON.stringify({}) }); }
  catch (err) { toast(err.message, "error"); }
}

async function requestSelfDelete() {
  if (!confirm("Request deletion of this account? Beta-dev deliberately requires an organization owner.")) return;
  try { await api("/users/me", { method: "DELETE" }); }
  catch (err) { toast(err.message, "error"); }
}

onMounted(load);
</script>

<template>
  <a class="back-link" @click="router.back()">&larr; Back</a>
  <header class="page-header"><div><div class="page-title">Account</div><div class="page-sub">Persisted identity, session state, scoped API keys, and explicit beta-dev account boundaries.</div></div><button class="btn btn-sm" :disabled="loading" @click="load">Refresh</button></header>
  <div v-if="error" class="error-box"><span>{{ error }}</span><button class="btn btn-sm" @click="load">Retry</button></div>
  <p v-if="loading" class="page-sub">Loading account state…</p>
  <template v-else>
    <section class="card"><div class="card-head"><div class="card-title">Profile and session</div><span class="tag tag-green">database-backed</span></div><div class="resource-object-grid"><div class="stat-card"><div class="stat-label">Username</div><div class="stat-value mono">{{ profile?.username || '—' }}</div></div><div class="stat-card"><div class="stat-label">Session</div><div class="stat-value">{{ session?.authenticated ? 'Active' : 'Unknown' }}</div></div><div class="stat-card"><div class="stat-label">Role</div><div class="stat-value">{{ profile?.role || '—' }}</div></div></div><p class="hint">Profile PATCH is mapped to the backend and reports its explicit beta-dev unsupported response rather than pretending it saved.</p><div class="detail-actions"><button class="btn btn-sm" @click="requestUnsupportedProfileUpdate">Test profile update contract</button><button class="btn btn-sm btn-danger" @click="requestSelfDelete">Request account deletion</button><button class="btn btn-sm" @click="logout">Sign out</button></div></section>
    <section class="card" style="margin-top:16px"><div class="card-head"><div class="card-title">Scoped API keys</div><span class="hint">{{ keys.length }} key(s)</span></div><div v-if="createdToken" class="notice-box"><strong>Copy this token now:</strong> <code>{{ createdToken }}</code></div><div class="filter-bar"><input v-model="keyName" placeholder="Key name" /><button class="btn btn-sm btn-primary" :disabled="busy" @click="createKey">Create key</button></div><div class="table-wrap"><table class="data-table"><thead><tr><th>Name</th><th>Created</th><th>Last used</th><th>Actions</th></tr></thead><tbody><tr v-for="key in keys" :key="key.id"><td class="mono">{{ key.name || '—' }}</td><td>{{ key.created_at || '—' }}</td><td>{{ key.last_used_at || 'never' }}</td><td><button class="btn btn-sm btn-danger" @click="revokeKey(key)">Revoke</button></td></tr><tr v-if="!keys.length"><td colspan="4" class="empty-state">No API keys yet.</td></tr></tbody></table></div></section>
  </template>
</template>
