<script setup>
import { ref, computed, onMounted, onUnmounted } from "vue";
import { useRouter, useRoute } from "vue-router";
import { api } from "../api/client";
import { connectEvents, disconnectEvents } from "../api/events";
import StatusBadge from "../components/StatusBadge.vue";
import HealthPill from "../components/HealthPill.vue";

const props = defineProps({ id: { type: String, required: true } });
const router = useRouter();
const route = useRoute();

const vm = ref(null);
const traffic = ref([]);
const logs = ref([]);
const sshInfo = ref(null);
const sshCertificate = ref(null);
const consoleInfo = ref(null);
const execCommand = ref("");
const execResult = ref(null);
const health = ref(null);
const metrics = ref([]);
const error = ref("");
const tab = ref(route.query.tab || "overview");
const TABS = ["overview", "health", "metrics", "logs", "traffic", "ssh"];

const healthy = computed(() => vm.value?.health_status === "healthy");

async function load() {
  error.value = "";
  try {
    vm.value = await api(`/vms/${props.id}`);
    await Promise.allSettled([loadHealth(), loadMetrics(), loadLogs(), loadTraffic(), loadSSH()]);
  } catch (e) {
    error.value = e.message;
  }
}
async function loadLogs() {
  logs.value = (await api(`/vms/${props.id}/logs`))?.logs || [];
}
async function loadTraffic() {
  const t = await api(`/vms/${props.id}/traffic`);
  traffic.value = Array.isArray(t) ? t : [];
}
async function loadSSH() {
  sshInfo.value = await api(`/vms/${props.id}/ssh-info`);
}
async function requestSSHCertificate() {
  try { sshCertificate.value = await api(`/vms/${props.id}/ssh-cert`, { method: "POST" }); }
  catch (e) { error.value = e.message; }
}
async function inspectConsole() {
  try { consoleInfo.value = await api(`/vms/${props.id}/console`); }
  catch (e) { error.value = e.message; }
}
async function execReplica() {
  const command = execCommand.value.trim();
  if (!command) return;
  try { execResult.value = await api(`/vms/${props.id}/exec`, { method: "POST", body: JSON.stringify({ cmd: command.split(/\s+/) }) }); }
  catch (e) { execResult.value = { status: "request error", error: e.message }; }
}
async function loadHealth() {
  health.value = await api(`/vms/${props.id}/health`);
}
async function loadMetrics() {
  const result = await api(`/vms/${props.id}/metrics`);
  metrics.value = Array.isArray(result) ? result : result?.metrics || [];
}

async function act(kind) {
  await api(`/vms/${props.id}/${kind}`, { method: "POST" });
  load();
}

async function del() {
  if (!confirm(`Delete microVM "${vm.value.name}"?`)) return;
  await api(`/vms/${props.id}`, { method: "DELETE" });
  router.push({ name: "list" });
}

function copySSH() {
  const cmd = sshInfo.value?.port
    ? `ssh -p ${sshInfo.value.port} ${sshInfo.value.user}@${sshInfo.value.host}`
    : `porter ssh ${vm.value?.name || props.id}`;
  navigator.clipboard?.writeText(cmd).catch(() => {});
  alert(`Copied: ${cmd}`);
}

onMounted(() => {
  load();
  connectEvents(() => load());
});
onUnmounted(() => disconnectEvents());
</script>

<template>
  <a class="back-link" @click="router.push({ name: 'list' })">&larr; Deployments</a>

  <div v-if="error" class="error-box">{{ error }}</div>

  <template v-else-if="vm">
    <div class="detail-header">
      <div class="detail-title">
        {{ vm.name }}
        <span class="tag tag-accent" style="vertical-align:middle">{{ vm.image }}</span>
      </div>
      <div class="page-sub">
        <StatusBadge :state="vm.state" /> &middot; <HealthPill :health="vm.health_status" />
        <template v-if="vm.ip_address"> &middot; <span class="mono">{{ vm.ip_address }}</span></template>
        <template v-if="vm.ports && vm.ports.length"> &middot; port {{ vm.ports[0].container_port }}</template>
        <template v-if="vm.error" style="color: var(--red)"> &middot; {{ vm.error }}</template>
      </div>
      <div class="detail-actions">
        <button class="btn btn-sm" @click="copySSH">SSH</button>
        <button class="btn btn-sm" @click="act('start')">Start</button>
        <button class="btn btn-sm" @click="act('stop')">Stop</button>
        <button class="btn btn-sm" @click="act('restart')">Restart</button>
        <button class="btn btn-danger btn-sm" @click="del">Delete</button>
      </div>
    </div>

    <div class="seg" style="margin-bottom:18px">
      <button v-for="t in TABS" :key="t" :class="{ active: tab === t }" @click="tab = t">{{ t }}</button>
    </div>

    <div v-if="tab === 'overview'" class="table-wrap">
      <table class="data-table">
        <tbody>
          <tr><td class="hint">State</td><td><StatusBadge :state="vm.state" /></td></tr>
          <tr><td class="hint">Health</td><td><HealthPill :health="vm.health_status" /></td></tr>
          <tr><td class="hint">Image</td><td class="mono">{{ vm.image }}</td></tr>
          <tr><td class="hint">IP</td><td class="mono">{{ vm.ip_address || '—' }}</td></tr>
          <tr><td class="hint">Replica index</td><td class="num">{{ vm.replica_index }}</td></tr>
          <tr><td class="hint">Project</td><td><a class="back-link" @click="router.push({ name: 'project', params: { id: vm.project_id } })">{{ vm.project_id }}</a></td></tr>
          <tr v-if="sshInfo"><td class="hint">SSH</td><td class="mono">{{ sshInfo.user }}@{{ sshInfo.host }}:{{ sshInfo.port }}</td></tr>
        </tbody>
      </table>
    </div>

    <div v-if="tab === 'health'" class="card">
      <div class="card-head"><div class="card-title">Replica health</div><HealthPill :health="health?.health || vm.health_status" /></div>
      <div class="runtime-check-list">
        <div class="runtime-check-row"><span class="runtime-check-icon" :class="health?.running ? 'check-ok' : 'check-fail'">{{ health?.running ? '✓' : '!' }}</span><div><div class="runtime-check-name">Runtime state</div><div class="hint">{{ health?.state || vm.state || 'unknown' }}</div></div></div>
        <div class="runtime-check-row"><span class="runtime-check-icon" :class="health?.error ? 'check-fail' : 'check-ok'">{{ health?.error ? '!' : '✓' }}</span><div><div class="runtime-check-name">Last health detail</div><div class="hint">{{ health?.error || 'No current health error reported.' }}</div></div></div>
      </div>
    </div>

    <div v-if="tab === 'metrics'">
      <div class="logs-header"><span class="page-sub mono">last {{ metrics.length }} samples</span><button class="btn btn-sm" @click="loadMetrics">Refresh</button></div>
      <div class="table-wrap">
        <table class="data-table">
          <thead><tr><th>Metric</th><th>Value</th><th>Observed</th></tr></thead>
          <tbody>
            <tr v-for="m in metrics" :key="m.id || `${m.metric}-${m.ts}`"><td class="mono">{{ m.metric }}</td><td class="num">{{ Number(m.value).toFixed(3) }}</td><td class="num muted">{{ m.ts ? new Date(m.ts).toLocaleString() : '—' }}</td></tr>
            <tr v-if="!metrics.length"><td colspan="3" class="hint" style="text-align:center; padding:18px">No metric samples recorded yet.</td></tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-if="tab === 'logs'">
      <div class="logs-header">
        <span class="page-sub mono">tail=200 &middot; auto-refreshes</span>
        <button class="btn btn-sm" @click="loadLogs">Refresh</button>
      </div>
      <div class="terminal">
        <div v-for="(l, i) in logs" :key="i" class="tline">{{ l }}</div>
        <div v-if="!logs.length" class="t-empty">No log output yet. Boot may still be in progress.</div>
      </div>
    </div>

    <div v-if="tab === 'traffic'">
      <div class="table-wrap">
        <table class="data-table">
          <thead><tr><th>Time</th><th>Method</th><th>Host</th><th>Path</th><th>Status</th><th>ms</th></tr></thead>
          <tbody>
            <tr v-for="(t, i) in traffic" :key="i">
              <td class="num muted">{{ new Date(t.timestamp).toLocaleTimeString() }}</td>
              <td class="mono">{{ t.method }}</td>
              <td class="mono">{{ t.host }}</td>
              <td class="mono">{{ t.path }}</td>
              <td><span class="num" :style="{ color: t.status < 400 ? 'var(--green)' : 'var(--red)' }">{{ t.status }}</span></td>
              <td class="num">{{ t.duration_ms }}</td>
            </tr>
            <tr v-if="!traffic.length"><td colspan="6" class="hint" style="text-align:center; padding:18px">No traffic yet.</td></tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-if="tab === 'ssh'" class="card">
      <div class="page-sub">SSH access is provided by the Porter SSH gateway — no sshd inside the guest.</div>
      <div v-if="sshInfo" class="terminal" style="margin-top:12px">
        <div class="tline">ssh -p {{ sshInfo.port }} {{ sshInfo.user }}@{{ sshInfo.host }}</div>
      </div>
      <div class="detail-actions" style="margin-top:14px"><button class="btn btn-sm" @click="requestSSHCertificate">Request certificate readiness</button><button class="btn btn-sm" @click="inspectConsole">Inspect console capability</button></div>
      <pre v-if="sshCertificate" class="mono settings-json" style="margin-top:12px">{{ JSON.stringify(sshCertificate, null, 2) }}</pre>
      <pre v-if="consoleInfo" class="mono settings-json" style="margin-top:12px">{{ JSON.stringify(consoleInfo, null, 2) }}</pre>
      <div class="field" style="margin-top:14px"><label>Non-interactive exec command</label><div class="filter-bar"><input v-model="execCommand" placeholder="uname -a" /><button class="btn btn-sm" :disabled="!execCommand.trim()" @click="execReplica">Run</button></div></div>
      <pre v-if="execResult" class="mono settings-json" style="margin-top:12px">{{ JSON.stringify(execResult, null, 2) }}</pre>
      <p class="hint" style="margin-top:12px">The backend may return an explicit unsupported status until a guest-vsock agent is enabled. This UI does not claim arbitrary SSH or guest execution is ready.</p>
    </div>
  </template>
</template>
