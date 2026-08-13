<!-- Porter project cache surface: traffic-derived cache statistics and scoped purge actions. -->
<script setup>
import { onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api } from "../api/client";
import { toast } from "../components/toast";

const route = useRoute();
const router = useRouter();
const projectId = route.params.projectId;
const stats = ref(null);
const path = ref("");
const loading = ref(true);
const busy = ref(false);
const error = ref("");

async function load() {
  loading.value = true;
  error.value = "";
  try { stats.value = await api(`/projects/${projectId}/cache/stats`); }
  catch (err) { error.value = err.message; }
  finally { loading.value = false; }
}
async function purge(endpoint, body) {
  busy.value = true;
  try { await api(`/projects/${projectId}/cache/${endpoint}`, { method: "POST", ...(body ? { body: JSON.stringify(body) } : {}) }); toast(endpoint === "purge" ? "Project cache purged" : "Cache path purged", "success"); path.value = ""; await load(); }
  catch (err) { toast(err.message, "error"); }
  finally { busy.value = false; }
}
onMounted(load);
</script>

<template>
  <a class="back-link" @click="router.push({ name: 'project', params: { id: projectId } })">&larr; Project workspace</a>
  <header class="page-header"><div><div class="page-title">Cache</div><div class="page-sub">Traffic-derived cache statistics and scoped invalidation for this project.</div></div><button class="btn btn-sm" :disabled="loading" @click="load">Refresh</button></header>
  <div v-if="error" class="error-box"><span>{{ error }}</span><button class="btn btn-sm" @click="load">Retry</button></div>
  <div v-if="loading" class="page-sub">Loading cache state…</div>
  <template v-else><section class="resource-object-grid"><div class="stat-card"><div class="stat-label">Hit rate</div><div class="stat-value">{{ Number(stats?.hit_rate || 0).toFixed(1) }}%</div></div><div class="stat-card"><div class="stat-label">Entries</div><div class="stat-value">{{ stats?.entries || 0 }}</div></div><div class="stat-card"><div class="stat-label">Hits</div><div class="stat-value">{{ stats?.hits || 0 }}</div></div></section><section class="card" style="margin-top:16px"><div class="card-title">Invalidate cache</div><p class="page-sub">These actions only clear traffic-derived state for this project.</p><div class="filter-bar"><input v-model="path" placeholder="/path/to/purge (optional)" /><button class="btn btn-sm" :disabled="busy || !path.trim()" @click="purge('purge/path', { path: path.trim() })">Purge path</button><button class="btn btn-sm btn-danger" :disabled="busy" @click="purge('purge')">Purge project cache</button></div></section></template>
</template>
