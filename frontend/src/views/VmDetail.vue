<script setup>
import { ref, onMounted, onUnmounted } from "vue";
import { useRouter } from "vue-router";
import { api } from "../api/client";
import { connectEvents, disconnectEvents } from "../api/events";
import StatusBadge from "../components/StatusBadge.vue";
import HealthPill from "../components/HealthPill.vue";
import AddDomainModal from "../components/AddDomainModal.vue";
import { toast } from "../components/toast";

const props = defineProps({ id: { type: String, required: true } });
const router = useRouter();

const vm = ref(null);
const domains = ref([]);
const traffic = ref([]);
const error = ref("");
const showAddDomain = ref(false);

async function load() {
  error.value = "";
  try {
    vm.value = await api(`/vms/${props.id}`);
    const [d, t] = await Promise.all([
      api(`/vms/${props.id}/domains`).catch(() => []),
      api(`/vms/${props.id}/traffic?limit=50`).catch(() => []),
    ]);
    domains.value = d || [];
    traffic.value = t || [];
  } catch (e) {
    error.value = e.message;
  }
}

async function stop() {
  await api(`/vms/${props.id}/stop`, { method: "POST" });
  load();
}
async function start() {
  await api(`/vms/${props.id}/start`, { method: "POST" });
  load();
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

    <div class="section-title">Traffic (last {{ traffic.length }})</div>
    <div class="card">
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
  </template>

  <AddDomainModal v-if="showAddDomain" :vm-id="id" @close="showAddDomain = false" @added="() => { showAddDomain = false; load(); }" />
</template>
