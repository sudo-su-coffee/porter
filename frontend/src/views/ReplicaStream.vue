<script setup>
import { computed, onMounted, onUnmounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api } from "../api/client";
import { toast } from "../components/toast";

const route = useRoute();
const router = useRouter();
const replica = ref(null);
const payload = ref(null);
const loading = ref(true);
const error = ref("");
const live = ref(true);
let timer;

const id = computed(() => route.params.id);
const kind = computed(() => route.meta.stream || "logs");
const title = computed(() => ({ logs: "Replica logs", health: "Replica health", metrics: "Replica metrics", traffic: "Replica traffic" }[kind.value] || "Replica stream"));
const endpoint = computed(() => `/vms/${encodeURIComponent(id.value)}/${kind.value}`);
const lines = computed(() => Array.isArray(payload.value) ? payload.value : payload.value?.logs || []);

async function load() {
  try {
    const [vm, stream] = await Promise.all([api(`/vms/${id.value}`), api(endpoint.value)]);
    replica.value = vm;
    payload.value = stream;
    error.value = "";
  } catch (e) {
    error.value = e.message;
  } finally {
    loading.value = false;
  }
}

async function act(action) {
  try {
    await api(`/vms/${id.value}/${action}`, { method: "POST" });
    toast(`Replica ${action} requested`, "success");
    load();
  } catch (e) { toast(e.message, "error"); }
}

function toggleLive() {
  live.value = !live.value;
  if (live.value) timer = setInterval(load, 3000);
  else clearInterval(timer);
}

onMounted(() => { load(); timer = setInterval(() => live.value && load(), 3000); });
onUnmounted(() => clearInterval(timer));
</script>

<template>
  <a class="back-link" @click="router.push({ name: 'vm', params: { id } })">&larr; Replica detail</a>
  <header class="page-header"><div><div class="page-title">{{ title }}</div><div class="page-sub mono">{{ id }} · {{ replica?.name || 'replica' }} · auto-refresh {{ live ? 'on' : 'off' }}</div></div><div class="detail-actions"><button class="btn btn-sm" @click="toggleLive">{{ live ? 'Pause stream' : 'Resume stream' }}</button><button class="btn btn-sm" @click="load">Refresh</button><button class="btn btn-sm btn-primary" @click="act('start')">Start</button><button class="btn btn-sm" @click="act('stop')">Stop</button><button class="btn btn-sm" @click="act('restart')">Restart</button></div></header>
  <div v-if="error" class="error-box">{{ error }}</div>
  <div class="seg stream-tabs"><router-link :to="{ name: 'vm-logs', params: { id } }">Logs</router-link><router-link :to="{ name: 'vm-health', params: { id } }">Health</router-link><router-link :to="{ name: 'vm-metrics', params: { id } }">Metrics</router-link><router-link :to="{ name: 'vm-traffic', params: { id } }">Traffic</router-link><router-link :to="{ name: 'vm-ssh', params: { id } }">SSH info</router-link></div>
  <p v-if="loading" class="page-sub">Reading replica telemetry…</p>
  <template v-else-if="kind === 'logs'"><div class="terminal stream-terminal"><div v-for="(line, index) in lines" :key="index" class="tline">{{ typeof line === 'string' ? line : JSON.stringify(line) }}</div><div v-if="!lines.length" class="t-empty">No log lines recorded for this replica.</div></div></template>
  <template v-else><div class="terminal"><pre>{{ JSON.stringify(payload, null, 2) }}</pre></div></template>
</template>
