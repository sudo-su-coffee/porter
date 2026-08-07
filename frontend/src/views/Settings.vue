<script setup>
import { ref, onMounted } from "vue";
import { api } from "../api/client";

const host = ref(null);
const kernel = ref(null);
const ports = ref([]);
const daemonLogs = ref([]);
const error = ref("");

async function load() {
  error.value = "";
  try {
    const [h, k, p, l] = await Promise.allSettled([
      api("/host/overview"), api("/host/kernel"), api("/host/ports"), api("/logs"),
    ]);
    host.value = h.status === "fulfilled" ? h.value : null;
    kernel.value = k.status === "fulfilled" ? k.value : null;
    ports.value = p.status === "fulfilled" && Array.isArray(p.value) ? p.value : [];
    daemonLogs.value = l.status === "fulfilled" && Array.isArray(l.value) ? l.value : (l.status === "fulfilled" && l.value ? l.value.logs || [] : []);
  } catch (e) {
    error.value = e.message;
  }
}

onMounted(load);
</script>

<template>
  <div class="page-header">
    <div>
      <div class="page-title">Settings</div>
      <div class="page-sub">Host, kernel and daemon information</div>
    </div>
    <button class="btn btn-sm" @click="load">Refresh</button>
  </div>

  <div v-if="error" class="error-box">{{ error }}</div>

  <div class="table-wrap" style="margin-bottom:18px">
    <table class="data-table">
      <tbody>
        <tr><td class="hint">Hostname</td><td>{{ host?.hostname || host?.host || '—' }}</td></tr>
        <tr><td class="hint">Platform</td><td>{{ host?.os || host?.platform || '—' }}</td></tr>
        <tr><td class="hint">Arch</td><td class="mono">{{ host?.arch || '—' }}</td></tr>
        <tr><td class="hint">Kernel</td><td class="mono">{{ kernel?.path || kernel?.kernel || '—' }}</td></tr>
        <tr><td class="hint">CPU</td><td>{{ host?.cpus || host?.cpu || '—' }}</td></tr>
        <tr><td class="hint">Memory</td><td>{{ host?.mem_total_mb || host?.mem_mb || '—' }} MiB</td></tr>
        <tr><td class="hint">Version</td><td>{{ host?.version || '—' }}</td></tr>
      </tbody>
    </table>
  </div>

  <div class="page-sub" style="margin-bottom:8px">Mapped host ports</div>
  <div class="table-wrap" style="margin-bottom:18px">
    <table class="data-table">
      <thead><tr><th>Host</th><th>Container</th><th>Project</th></tr></thead>
      <tbody>
        <tr v-for="(p, i) in ports" :key="i">
          <td class="num">{{ p.host_port || p.host }}</td>
          <td class="num">{{ p.container_port || p.container }}</td>
          <td class="mono">{{ p.project_id || '—' }}</td>
        </tr>
        <tr v-if="!ports.length"><td colspan="3" class="hint" style="text-align:center; padding:18px">No mapped ports.</td></tr>
      </tbody>
    </table>
  </div>

  <div class="page-sub" style="margin-bottom:8px">Daemon log</div>
  <div class="terminal">
    <div v-for="(l, i) in daemonLogs" :key="i" class="tline">{{ l }}</div>
    <div v-if="!daemonLogs.length" class="t-empty">No daemon log entries.</div>
  </div>
</template>
