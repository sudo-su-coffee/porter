<!-- Porter system surface: real health/version checks and the unauthenticated event hub status. -->
<script setup>
import { onMounted, onUnmounted, ref } from "vue";
import { useRouter } from "vue-router";
import { api } from "../api/client";

const router = useRouter();
const health = ref(null);
const healthz = ref(null);
const version = ref(null);
const eventsLive = ref(false);
const events = ref([]);
const loading = ref(true);
const error = ref("");
let source;

async function load() {
  loading.value = true;
  error.value = "";
  try { [health.value, healthz.value, version.value] = await Promise.all([api("/health"), api("/healthz"), api("/version")]); }
  catch (err) { error.value = err.message; }
  finally { loading.value = false; }
}

function connect() {
  try {
    source = new EventSource("/events");
    source.onopen = () => { eventsLive.value = true; };
    source.onerror = () => { eventsLive.value = false; };
    ["vm.state", "replica.health", "project.progress", "pool.updated", "domain.status", "traffic.request", "cache.purged"].forEach((name) => source.addEventListener(name, (event) => events.value.unshift({ name, data: event.data, at: new Date().toLocaleTimeString() })));
  } catch (_) { eventsLive.value = false; }
}

onMounted(() => { load(); connect(); });
onUnmounted(() => source?.close());
</script>

<template>
  <header class="page-header"><div><div class="page-title">System status</div><div class="page-sub">Health, version, and the live control-plane event hub. This is operational plumbing exposed as a real operator surface.</div></div><button class="btn btn-sm" :disabled="loading" @click="load">Refresh</button></header>
  <div v-if="error" class="error-box">{{ error }}</div>
  <div v-if="loading" class="page-sub">Checking control-plane health…</div>
  <template v-else><section class="resource-object-grid"><div class="stat-card"><div class="stat-label">/health</div><div class="stat-value">{{ health?.status || 'unknown' }}</div></div><div class="stat-card"><div class="stat-label">/healthz</div><div class="stat-value">{{ healthz?.status || 'unknown' }}</div></div><div class="stat-card"><div class="stat-label">Version</div><div class="stat-value mono">{{ version?.version || version?.build || '—' }}</div></div><div class="stat-card"><div class="stat-label">/events</div><div class="stat-value">{{ eventsLive ? 'Live' : 'Disconnected' }}</div></div></section><section class="card" style="margin-top:16px"><div class="card-head"><div class="card-title">Recent event-hub messages</div><button class="btn btn-sm" @click="router.push({ name: 'logs' })">Daemon logs</button></div><div v-if="!events.length" class="empty-state">Waiting for real control-plane events.</div><div v-else class="table-wrap"><table class="data-table"><thead><tr><th>Time</th><th>Event</th><th>Payload</th></tr></thead><tbody><tr v-for="event in events.slice(0, 30)" :key="`${event.at}-${event.name}-${event.data}`"><td>{{ event.at }}</td><td class="mono">{{ event.name }}</td><td class="mono">{{ event.data }}</td></tr></tbody></table></div></section></template>
</template>
