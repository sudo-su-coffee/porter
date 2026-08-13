<script setup>
import { ref, onMounted } from "vue";
import { api } from "../api/client";

const host = ref(null);
const kernel = ref(null);
const prerequisites = ref(null);
const runtime = ref(null);
const ports = ref([]);
const daemonLogs = ref([]);
const error = ref("");

async function load() {
  error.value = "";
  try {
    const [h, k, r, c, p, l] = await Promise.allSettled([
      api("/host/overview"), api("/host/kernel"), api("/host/prerequisites"), api("/host/runtime"), api("/host/ports"), api("/logs"),
    ]);
    host.value = h.status === "fulfilled" ? h.value : null;
    kernel.value = k.status === "fulfilled" ? k.value : null;
    prerequisites.value = r.status === "fulfilled" ? r.value : null;
    runtime.value = c.status === "fulfilled" ? c.value : null;
    ports.value = p.status === "fulfilled" && Array.isArray(p.value) ? p.value : (p.status === "fulfilled" ? p.value?.ports || [] : []);
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
      <div class="page-sub">Host readiness, direct Firecracker configuration, ports, and daemon signals</div>
    </div>
    <button class="btn btn-sm" @click="load">Refresh</button>
  </div>

  <div v-if="error" class="error-box">{{ error }}</div>

  <div class="runtime-summary-grid">
    <div class="runtime-summary-card" :class="prerequisites?.ready ? 'runtime-ok' : 'runtime-warn'">
      <div class="stat-label">Direct runtime</div>
      <div class="runtime-summary-title">{{ prerequisites?.ready ? 'Ready to boot' : 'Needs attention' }}</div>
      <div class="stat-sub">{{ runtime?.runtime_mode || 'direct' }} · Unix API sockets</div>
    </div>
    <div class="runtime-summary-card">
      <div class="stat-label">Firecracker binary</div>
      <div class="runtime-summary-title mono">{{ runtime?.firecracker_bin || '—' }}</div>
      <div class="stat-sub">{{ runtime?.api_socket_dir || 'socket directory unavailable' }}</div>
    </div>
    <div class="runtime-summary-card">
      <div class="stat-label">Artifacts</div>
      <div class="runtime-summary-title">Kernel + rootfs</div>
      <div class="stat-sub mono">{{ runtime?.kernel_image || 'kernel not configured' }}</div>
    </div>
  </div>

  <div v-if="prerequisites?.checks?.length" class="card runtime-check-card">
    <div class="card-head"><div class="card-title">Host prerequisites</div><span class="tag" :class="prerequisites.ready ? 'tag-green' : 'tag-amber'">{{ prerequisites.ready ? 'ready' : 'review' }}</span></div>
    <div class="runtime-check-list">
      <div v-for="check in prerequisites.checks" :key="check.name" class="runtime-check-row">
        <span class="runtime-check-icon" :class="check.ok ? 'check-ok' : 'check-fail'">{{ check.ok ? '✓' : '!' }}</span>
        <div><div class="runtime-check-name">{{ check.name }}</div><div class="hint">{{ check.message }}</div></div>
      </div>
    </div>
  </div>

  <div class="table-wrap settings-host-table" style="margin-bottom:18px">
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

  <div v-if="runtime" class="card runtime-config-card">
    <div class="card-head"><div class="card-title">Runtime configuration</div><span class="tag tag-accent">read-only</span></div>
    <div class="runtime-config-grid">
      <div><span class="hint">Kernel</span><span class="mono">{{ runtime.kernel_image || '—' }}</span></div>
      <div><span class="hint">Rootfs</span><span class="mono">{{ runtime.rootfs_path || '—' }}</span></div>
      <div><span class="hint">Images</span><span class="mono">{{ runtime.images_dir || '—' }}</span></div>
      <div><span class="hint">Custom images</span><span class="mono">{{ runtime.custom_images_dir || '—' }}</span></div>
    </div>
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
