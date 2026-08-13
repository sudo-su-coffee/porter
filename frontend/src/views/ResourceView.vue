<script setup>
import { computed, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api } from "../api/client";
import { toast } from "../components/toast";

const route = useRoute();
const router = useRouter();
const data = ref(null);
const loading = ref(true);
const error = ref("");
const actionBusy = ref(false);
const resource = computed(() => route.meta.resource || {});

function endpoint(template) {
  return String(template || route.path).replace(/:([A-Za-z0-9_]+)/g, (_, key) => encodeURIComponent(route.params[key] || ""));
}

function extractRows(value) {
  if (Array.isArray(value)) return value;
  if (!value || typeof value !== "object") return [];
  for (const key of ["items", "data", "events", "members", "builds", "rollouts", "services", "networks", "volumes", "domains", "paths", "series", "logs"]) {
    if (Array.isArray(value[key])) return value[key];
  }
  return [];
}

const rows = computed(() => extractRows(data.value));
const columns = computed(() => {
  const first = rows.value.find((row) => row && typeof row === "object" && !Array.isArray(row));
  return first ? Object.keys(first).filter((key) => !["value_encrypted", "secret", "token"].includes(key)).slice(0, 8) : [];
});
const objectEntries = computed(() => {
  if (!data.value || typeof data.value !== "object" || Array.isArray(data.value) || rows.value.length) return [];
  return Object.entries(data.value).filter(([key]) => !["value_encrypted", "secret", "token"].includes(key));
});

function format(value) {
  if (value === null || value === undefined || value === "") return "—";
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}

async function load() {
  loading.value = true;
  error.value = "";
  try {
    data.value = await api(endpoint(resource.value.endpoint));
  } catch (e) {
    error.value = e.message;
  } finally {
    loading.value = false;
  }
}

async function invoke(action) {
  if (action.confirm && !confirm(action.confirm)) return;
  actionBusy.value = true;
  try {
    await api(endpoint(action.endpoint), { method: action.method || "POST", body: action.body ? JSON.stringify(action.body) : undefined });
    toast(action.success || "Action completed", "success");
    await load();
  } catch (e) {
    toast(e.message, "error");
  } finally {
    actionBusy.value = false;
  }
}

onMounted(load);
</script>

<template>
  <a v-if="resource.back" class="back-link" @click="router.back()">&larr; Back</a>
  <header class="page-header">
    <div><div class="page-title">{{ resource.title || 'Resource' }}</div><div class="page-sub">{{ resource.description || 'Live data from the Porter control plane.' }}</div></div>
    <div class="detail-actions"><button class="btn btn-sm" @click="load">Refresh</button><button v-for="action in resource.actions || []" :key="action.label" class="btn btn-sm" :class="action.primary ? 'btn-primary' : ''" :disabled="actionBusy" @click="invoke(action)">{{ action.label }}</button></div>
  </header>
  <div v-if="error" class="error-box">{{ error }}</div>
  <p v-if="loading" class="page-sub">Loading live resource data…</p>
  <template v-else>
    <div v-if="resource.note" class="notice-box">{{ resource.note }}</div>
    <div v-if="objectEntries.length" class="resource-object-grid">
      <div v-for="([key, value]) in objectEntries" :key="key" class="stat-card"><div class="stat-label">{{ key.replaceAll('_', ' ') }}</div><div class="stat-value mono">{{ format(value) }}</div></div>
    </div>
    <div v-if="rows.length" class="table-wrap">
      <table class="data-table"><thead><tr><th v-for="column in columns" :key="column">{{ column.replaceAll('_', ' ') }}</th></tr></thead><tbody><tr v-for="(row, index) in rows" :key="row.id || row.name || index"><td v-for="column in columns" :key="column" class="mono">{{ format(row[column]) }}</td></tr></tbody></table>
    </div>
    <div v-else-if="data && !objectEntries.length" class="terminal"><pre>{{ JSON.stringify(data, null, 2) }}</pre></div>
    <div v-else class="empty-state"><strong>No {{ (resource.title || 'resource').toLowerCase() }} data yet.</strong><span>This is a real empty state; Porter did not fabricate records.</span></div>
  </template>
</template>
