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
const q = ref("");
const view = ref(localStorage.getItem("porter:view") || "list");

// Status tabs always show the same set so the segmented control is stable.
const STATUS_TABS = ["all", "running", "stopped", "failed"];
const statusOf = (p) => p.status || "pending";

const tags = computed(() => {
  const set = new Set();
  for (const p of projects.value) for (const t of p.tags || []) set.add(t);
  return [...set];
});

// Count per status tab (computed once, reused by both the seg buttons and
// the grid cards so the numbers always agree).
const counts = computed(() => {
  const m = { all: projects.value.length, running: 0, stopped: 0, failed: 0 };
  for (const p of projects.value) {
    const s = statusOf(p);
    if (s === "running") m.running++;
    else if (s === "stopped" || s === "stopping") m.stopped++;
    else if (s === "failed") m.failed++;
  }
  return m;
});

const filtered = computed(() => {
  let out = projects.value;
  if (filter.value !== "all") {
    out = out.filter((p) => {
      const s = statusOf(p);
      // "stopping" projects roll up under "stopped" so the seg-count badge
      // and the row list always agree.
      if (filter.value === "stopped") return s === "stopped" || s === "stopping";
      return s === filter.value;
    });
  }
  if (tagFilter.value) out = out.filter((p) => (p.tags || []).includes(tagFilter.value));
  const s = q.value.trim().toLowerCase();
  if (s) {
    out = out.filter((p) =>
      [p.name, p.image, (p.tags || []).join(" ")].filter(Boolean).join(" ").toLowerCase().includes(s)
    );
  }
  return out;
});

const anyActive = computed(() => filter.value !== "all" || !!tagFilter.value || !!q.value.trim());

function clearFilters() {
  filter.value = "all";
  tagFilter.value = "";
  q.value = "";
}

function setView(v) {
  view.value = v;
  try {
    localStorage.setItem("porter:view", v);
  } catch {}
}

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
    <div class="view-toggle" role="tablist" aria-label="Layout">
      <button :class="{ active: view === 'list' }" title="List view" aria-label="List view" @click="setView('list')">☷</button>
      <button :class="{ active: view === 'grid' }" title="Grid view" aria-label="Grid view" @click="setView('grid')">▦</button>
    </div>
  </div>

  <div v-if="error" class="error-box">{{ error }}</div>

  <OverviewBar :overview="overview" :loading="loading" />

  <div class="filter-bar">
    <input v-model="q" placeholder="Search name, tag, image…" class="filter-search" aria-label="Search projects" />
    <div class="seg">
      <button
        v-for="t in STATUS_TABS"
        :key="t"
        :class="{ active: filter === t }"
        @click="filter = t"
      >
        {{ t.charAt(0).toUpperCase() + t.slice(1) }}
        <span class="seg-count">{{ counts[t] }}</span>
      </button>
    </div>
    <span class="filter-spacer"></span>
    <span v-for="t in tags" :key="t" class="tag" :class="tagFilter === t ? 'tag-accent' : ''" @click="tagFilter = tagFilter === t ? '' : t">{{ t }}</span>
    <button v-if="anyActive" class="clear-btn" @click="clearFilters">✕ Clear filters</button>
  </div>

  <div v-if="loading && !projects.length" class="table-wrap">
    <div v-for="i in 3" :key="i" class="skeleton skeleton-line" style="margin: 14px"></div>
  </div>

  <div v-else-if="!projects.length" class="empty-state">
    <div style="font-size: 15px; margin-bottom: 8px">No projects yet</div>
    Deploy your first image or compose file — click <b>New Project</b> in the sidebar.
  </div>

  <div v-else-if="!filtered.length" class="empty-state">
    <div style="font-size: 15px; margin-bottom: 8px">No projects match your filters</div>
    <button class="btn btn-sm" @click="clearFilters" style="margin-top: 8px">Clear filters</button>
  </div>

  <!-- Grid view -->
  <div v-else-if="view === 'grid'" class="project-grid">
    <div v-for="p in filtered" :key="p.id" class="project-card" @click="router.push({ name: 'project', params: { id: p.id } })">
      <div class="pc-head">
        <div>
          <div class="pc-name">{{ p.name }}</div>
          <div class="pc-image">{{ p.image || (p.compose_yaml ? 'compose' : '—') }}</div>
        </div>
        <StatusBadge :state="statusOf(p)" />
      </div>
      <div v-if="(p.tags || []).length || p.kind === 'compose'" class="pc-tags">
        <span v-for="t in p.tags || []" :key="t" class="tag">{{ t }}</span>
        <span v-if="p.kind === 'compose'" class="tag tag-accent">compose</span>
      </div>
      <div class="pc-stats">
        <div class="pc-stat">
          <div class="pc-stat-label">Replicas</div>
          <div class="pc-stat-value num">{{ projectStats(p).running }}/{{ p.replicas_desired ?? projectStats(p).pvms.length }}</div>
        </div>
        <div class="pc-stat">
          <div class="pc-stat-label">Network</div>
          <div class="pc-stat-value mono">{{ p.network || '—' }}</div>
        </div>
        <div class="pc-stat">
          <div class="pc-stat-label">Created</div>
          <div class="pc-stat-value num">{{ fmtDate(p.created_at) }}</div>
        </div>
      </div>
      <div class="pc-footer">
        <HealthPill v-if="projectStats(p).pvms.length" :health="projectStats(p).healthy === projectStats(p).pvms.length ? 'healthy' : 'checking'" />
        <span v-else class="hint">No replicas</span>
        <div class="actions" @click.stop>
          <button class="icon-btn green" title="Start" @click="act(p, 'start')">▶</button>
          <button class="icon-btn" title="Stop" @click="act(p, 'stop')">■</button>
          <button class="icon-btn" title="Restart" @click="act(p, 'restart')">↻</button>
          <button class="icon-btn danger" title="Delete" @click="del(p)">✕</button>
        </div>
      </div>
    </div>
  </div>

  <!-- List view -->
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
          <td><StatusBadge :state="statusOf(p)" /></td>
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
