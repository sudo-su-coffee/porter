<script setup>
import { ref, onMounted, computed } from "vue";
import { api } from "../api/client";

const traffic = ref([]);
const error = ref("");
const filter = ref("all");
const loading = ref(true);
const query = ref("");

const filtered = computed(() => {
  if (filter.value === "all") return traffic.value;
  if (filter.value === "errors") return traffic.value.filter((t) => t.status >= 400);
  if (filter.value === "success") return traffic.value.filter((t) => t.status < 400);
  return traffic.value;
});

async function load() {
  loading.value = true;
  error.value = "";
  try {
    const endpoint = query.value.trim() ? `/traffic/search?q=${encodeURIComponent(query.value.trim())}` : "/traffic";
    const t = await api(endpoint);
    traffic.value = Array.isArray(t) ? t : t?.results || [];
  } catch (e) {
    error.value = e.message;
  } finally {
    loading.value = false;
  }
}

async function clearTraffic() {
  if (!confirm("Clear all traffic history?")) return;
  await api("/traffic", { method: "DELETE" });
  load();
}

onMounted(load);
</script>

<template>
  <div class="page-header">
    <div>
      <div class="page-title">Traffic</div>
      <div class="page-sub">Requests proxied through the host-routing gateway</div>
    </div>
    <button class="btn btn-sm btn-danger" @click="clearTraffic">Clear</button>
  </div>

  <div v-if="error" class="error-box">{{ error }}</div>

  <div class="filter-bar">
    <input v-model="query" placeholder="Search path, host, or method…" @keyup.enter="load" />
    <button class="btn btn-sm" @click="load">Search</button>
    <div class="seg">
      <button :class="{ active: filter === 'all' }" @click="filter = 'all'">All</button>
      <button :class="{ active: filter === 'success' }" @click="filter = 'success'">2xx–3xx</button>
      <button :class="{ active: filter === 'errors' }" @click="filter = 'errors'">4xx–5xx</button>
    </div>
    <span class="hint">{{ traffic.length }} request(s)</span>
  </div>

  <div class="table-wrap">
    <table class="data-table">
      <thead>
        <tr><th>Time</th><th>Method</th><th>Host</th><th>Path</th><th>Status</th><th>ms</th><th>Remote IP</th></tr>
      </thead>
      <tbody>
        <tr v-for="(t, i) in filtered" :key="i">
          <td class="num muted">{{ new Date(t.timestamp).toLocaleTimeString() }}</td>
          <td class="mono">{{ t.method }}</td>
          <td class="mono">{{ t.host }}</td>
          <td class="mono">{{ t.path }}</td>
          <td><span class="num" :style="{ color: t.status < 400 ? 'var(--green)' : 'var(--red)' }">{{ t.status }}</span></td>
          <td class="num">{{ t.duration_ms }}</td>
          <td class="mono muted">{{ t.remote_ip }}</td>
        </tr>
        <tr v-if="!filtered.length">
          <td colspan="7" class="hint" style="text-align:center; padding:18px">
            {{ loading ? 'Loading…' : 'No traffic yet — requests through the gateway appear here.' }}
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
