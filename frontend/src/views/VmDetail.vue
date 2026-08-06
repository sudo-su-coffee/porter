<script setup>
import { ref, computed, onMounted, onUnmounted } from "vue";
import { useRouter } from "vue-router";
import { api } from "../api/client";
import { connectEvents, disconnectEvents } from "../api/events";
import StatusBadge from "../components/StatusBadge.vue";
import HealthPill from "../components/HealthPill.vue";
import AddDomainModal from "../components/AddDomainModal.vue";
import Sparkline from "../components/Sparkline.vue";
import { toast } from "../components/toast";

const props = defineProps({ id: { type: String, required: true } });
const router = useRouter();

const vm = ref(null);
const domains = ref([]);
const traffic = ref([]);
const logs = ref([]);
const logsLoading = ref(true);
const error = ref("");
const showAddDomain = ref(false);

let logTimer = null;

// Aggregate the traffic ring into requests-per-second buckets so we can
// draw a sparkline and surface the current / peak throughput.
const trafficSeries = computed(() => {
  const sorted = [...traffic.value].sort(
    (a, b) => new Date(a.timestamp) - new Date(b.timestamp)
  );
  if (sorted.length < 2) return [];
  const counts = [];
  let curSec = null;
  for (const t of sorted) {
    const sec = Math.floor(new Date(t.timestamp).getTime() / 1000);
    if (curSec === null || sec !== curSec) {
      curSec = sec;
      counts.push(1);
    } else {
      counts[counts.length - 1] += 1;
    }
  }
  return counts;
});
const reqPerSec = computed(() =>
  trafficSeries.value.length ? trafficSeries.value[trafficSeries.value.length - 1] : 0
);
const peakReqPerSec = computed(() =>
  trafficSeries.value.length ? Math.max(...trafficSeries.value) : 0
);

async function load() {
  error.value = "";
  try {
    vm.value = await api(`/vms/${props.id}`);
    const [d, t] = await Promise.all([
      api(`/vms/${props.id}/domains`).catch(() => []),
      api(`/vms/${props.id}/traffic?limit=200`).catch(() => []),
    ]);
    domains.value = d || [];
    traffic.value = t || [];
  } catch (e) {
    error.value = e.message;
  }
}

async function loadLogs() {
  try {
    const res = await api(`/vms/${props.id}/logs?tail=200`);
    logs.value = res.lines || [];
  } catch (_) {
    // Log endpoint may 404 while a VM is still booting — keep the panel quiet.
  } finally {
    logsLoading.value = false;
  }
}

async function start() {
  await api(`/vms/${props.id}/start`, { method: "POST" });
  load();
}
async function stop() {
  await api(`/vms/${props.id}/stop`, { method: "POST" });
  load();
}
async function restart() {
  try {
    await api(`/vms/${props.id}/restart`, { method: "POST" });
    toast("Restarting…");
    load();
  } catch (e) {
    toast(e.message);
  }
}
async function del() {
  if (!confirm(`Delete "${vm.value.name}"?`)) return;
  await api(`/vms/${props.id}`, { method: "DELETE" });
  router.push({ name: "list" });
}
async function copySSH() {
  try {
    const info = await api(`/vms/${props.id}/ssh-info`);
    navigator.clipboard?.writeText(info.command).catch(() => {});
    toast(`Copied: ${info.command}`);
  } catch (e) {
    toast(e.message);
  }
}

function statusColor(status) {
  if (status >= 500) return "var(--red)";
  if (status >= 400) return "var(--amber)";
  return "var(--green)";
}

onMounted(() => {
  load();
  loadLogs();
  connectEvents(() => load());
  logTimer = setInterval(loadLogs, 3000);
});
onUnmounted(() => {
  disconnectEvents();
  if (logTimer) clearInterval(logTimer);
});
</script>

<template>
  <a class="back-link" @click="router.push({ name: 'list' })">&larr; Deployments</a>

  <div v-if="error" class="error-box">{{ error }}</div>

  <template v-else-if="vm">
    <div class="detail-header">
      <div class="detail-title">
        {{ vm.name }} <StatusBadge :state="vm.state" /> <HealthPill :health="vm.health_status" />
      </div>
      <div class="page-sub mono">
        {{ vm.image }} &middot; {{ vm.vcpus }} vCPU &middot; {{ vm.mem_mib }} MiB
        <template v-if="vm.ip_address"> &middot; {{ vm.ip_address }}</template>
      </div>
      <div v-if="vm.error" class="error-box">{{ vm.error }}</div>
      <div class="detail-actions">
        <button class="btn btn-sm" :disabled="vm.state === 'running' || vm.state === 'booting'" @click="start">
          Start
        </button>
        <button class="btn btn-sm" :disabled="vm.state !== 'running'" @click="stop">Stop</button>
        <button class="btn btn-sm" :disabled="vm.state !== 'running'" @click="restart">Restart</button>
        <button class="btn btn-sm" @click="copySSH">SSH</button>
        <button class="btn btn-danger btn-sm" @click="del">Delete</button>
      </div>
    </div>

    <div class="section-title">Domains</div>
    <div class="card">
      <template v-if="domains.length">
        <div v-for="d in domains" :key="d.domain" class="domain-row">
          <span class="mono">{{ d.domain }}</span>
          <span>
            <span class="card-badge">{{ d.type }}</span>
            <span class="pill" :class="d.status === 'verified' ? 'pill-verified' : 'pill-pending'">{{ d.status }}</span>
          </span>
        </div>
      </template>
      <div v-else class="page-sub">No domains yet.</div>
      <div style="margin-top:10px">
        <button class="btn btn-sm" @click="showAddDomain = true">Add domain</button>
      </div>
    </div>

    <div class="section-title">Traffic</div>
    <div class="card">
      <div v-if="trafficSeries.length" class="traffic-spark">
        <div class="spark-legend">
          <span><b>{{ reqPerSec }}</b> req/s now</span>
          <span><b>{{ peakReqPerSec }}</b> req/s peak</span>
          <span class="page-sub">{{ traffic.length }} in window</span>
        </div>
        <Sparkline :data="trafficSeries" />
      </div>
      <table v-if="traffic.length" class="traffic">
        <thead>
          <tr><th>Time</th><th>Method</th><th>Path</th><th>Status</th><th>Duration</th><th>Remote IP</th></tr>
        </thead>
        <tbody>
          <tr v-for="(t, i) in [...traffic].reverse()" :key="i">
            <td>{{ new Date(t.timestamp).toLocaleTimeString() }}</td>
            <td>{{ t.method }}</td>
            <td>{{ t.path }}</td>
            <td :style="{ color: statusColor(t.status) }">{{ t.status }}</td>
            <td>{{ t.duration_ms }}ms</td>
            <td>{{ t.remote_ip }}</td>
          </tr>
        </tbody>
      </table>
      <div v-else class="page-sub">No requests recorded yet.</div>
    </div>

    <div class="section-title">Live Logs</div>
    <div class="card">
      <div class="logs-header">
        <span class="page-sub mono">tail=200 &middot; auto-refreshes</span>
        <button class="btn btn-sm" :disabled="logsLoading" @click="loadLogs">Refresh</button>
      </div>
      <div class="terminal">
        <template v-if="logs.length">
          <div v-for="(l, i) in logs" :key="i" class="tline">{{ l }}</div>
        </template>
        <div v-else class="t-empty">No log output yet. Boot may still be in progress.</div>
      </div>
    </div>
  </template>

  <AddDomainModal v-if="showAddDomain" :vm-id="id" @close="showAddDomain = false" @added="() => { showAddDomain = false; load(); }" />
</template>
