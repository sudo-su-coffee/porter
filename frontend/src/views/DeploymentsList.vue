<script setup>
import { ref, onMounted, onUnmounted, computed } from "vue";
import { useRouter } from "vue-router";
import { api } from "../api/client";
import { connectEvents, disconnectEvents } from "../api/events";
import OverviewBar from "../components/OverviewBar.vue";
import StatusBadge from "../components/StatusBadge.vue";
import HealthPill from "../components/HealthPill.vue";

const router = useRouter();
const projects = ref([]);
const vms = ref([]);
const overview = ref(null);
const loading = ref(true);
const error = ref("");
const filter = ref("all");
const tagFilter = ref("");

const tags = computed(() => {
  const set = new Set();
  for (const p of projects.value) for (const t of p.tags || []) set.add(t);
  return [...set];
});

const filtered = computed(() => {
  let out = projects.value;
  if (filter.value !== "all") out = out.filter((p) => (p.status || "pending") === filter.value);
  if (tagFilter.value) out = out.filter((p) => (p.tags || []).includes(tagFilter.value));
  return out;
});

function projectStats(p) {
  const pvms = vms.value.filter((v) => v.project_id === p.id);
  const running = pvms.filter((v) => v.state === "running").length;
  const healthy = pvms.filter((v) => v.health_status === "healthy").length;
  return { pvms, running, healthy };
}

function fmtDate(iso) {
  if (!iso) return "—";
  return new Date(iso).toLocaleDateString();
}

async function load() {
  error.value = "";
  try {
    const [o, p, v] = await Promise.all([api("/overview"), api("/projects"), api("/vms")]);
    overview.value = o;
    projects.value = Array.isArray(p) ? p : p?.projects || [];
    vms.value = Array.isArray(v) ? v : v?.vms || [];
  } catch (e) {
    error.value = e.message;
  } finally {
    loading.value = false;
  }
}

async function act(p, kind) {
  const map = {
    start: `/projects/${p.id}/replicas/batch/start`,
    stop: `/projects/${p.id}/replicas/batch/stop`,
    restart: `/projects/${p.id}/restart`,
  };
  await api(map[kind], { method: "POST" });
  load();
}

async function del(p) {
  if (!confirm(`Delete project "${p.name}" and all its replicas?`)) return;
  await api(`/projects/${p.id}`, { method: "DELETE" });
  load();
}

onMounted(() => {
  load();
  connectEvents(() => load());
});
onUnmounted(() => disconnectEvents());
</script>

<template>
  <div class="page-header">
    <div>
      <div class="page-title">Deployments</div>
      <div class="page-sub">Projects and their replica pools on this host</div>
    </div>
  </div>

  <div v-if="error" class="error-box">{{ error }}</div>

  <OverviewBar :overview="overview" :loading="loading" />

  <div class="filter-bar">
    <div class="seg">
      <button :class="{ active: filter === 'all' }" @click="filter = 'all'">All</button>
      <button :class="{ active: filter === 'running' }" @click="filter = 'running'">Running</button>
      <button :class="{ active: filter === 'stopped' }" @click="filter = 'stopped'">Stopped</button>
      <button :class="{ active: filter === 'failed' }" @click="filter = 'failed'">Failed</button>
    </div>
    <span v-for="t in tags" :key="t" class="tag" :class="tagFilter === t ? 'tag-accent' : ''" @click="tagFilter = tagFilter === t ? '' : t">{{ t }}</span>
  </div>

  <div v-if="loading && !projects.length" class="table-wrap">
    <div v-for="i in 3" :key="i" class="skeleton skeleton-line" style="margin: 14px"></div>
  </div>

  <div v-else-if="!projects.length" class="empty-state">
    <div style="font-size: 15px; margin-bottom: 8px">No projects yet</div>
    Deploy your first image or compose file — click <b>New Project</b> in the sidebar.
  </div>

  <div v-else class="table-wrap">
    <table class="data-table">
      <thead>
        <tr>
          <th>Project</th>
          <th>Status</th>
          <th>Image</th>
          <th>Replicas</th>
          <th>Network</th>
          <th>Created</th>
          <th style="text-align:right">Actions</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="p in filtered" :key="p.id" style="cursor:pointer" @click="router.push({ name: 'project', params: { id: p.id } })">
          <td>
            <div class="side-user" style="padding:0; font-weight:600">{{ p.name }}</div>
            <div class="hint" style="margin-top:3px">
              <span v-for="t in p.tags || []" :key="t" class="tag" style="margin-right:4px">{{ t }}</span>
              <span v-if="p.kind === 'compose'" class="tag tag-accent">compose</span>
            </div>
          </td>
          <td><StatusBadge :state="p.status || 'pending'" /></td>
          <td class="mono">{{ p.image || (p.compose_yaml ? 'compose' : '—') }}</td>
          <td>
            <span class="num">{{ projectStats(p).running }}/{{ p.replicas_desired ?? projectStats(p).pvms.length }}</span>
            <span v-if="projectStats(p).pvms.length" class="muted"> · <HealthPill :health="projectStats(p).healthy === projectStats(p).pvms.length ? 'healthy' : 'checking'" /></span>
          </td>
          <td class="mono">{{ p.network || '—' }}</td>
          <td class="num muted">{{ fmtDate(p.created_at) }}</td>
          <td style="text-align:right">
            <div class="actions" @click.stop>
              <button class="icon-btn green" title="Start" @click="act(p, 'start')">▶</button>
              <button class="icon-btn" title="Stop" @click="act(p, 'stop')">■</button>
              <button class="icon-btn" title="Restart" @click="act(p, 'restart')">↻</button>
              <button class="icon-btn danger" title="Delete" @click="del(p)">✕</button>
            </div>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
