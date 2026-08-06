<script setup>
import { computed } from "vue";

const props = defineProps({
  overview: { type: Object, default: null },
  loading: { type: Boolean, default: true },
});

// "1d 4h 22m" from an ISO started_at — or a static placeholder while loading.
const uptime = computed(() => {
  const o = props.overview;
  if (!o || !o.started_at) return null;
  const s = Math.max(0, (Date.now() - new Date(o.started_at).getTime()) / 1000);
  const d = Math.floor(s / 86400);
  const h = Math.floor((s % 86400) / 3600);
  const m = Math.floor((s % 3600) / 60);
  if (d > 0) return `${d}d ${h}h`;
  if (h > 0) return `${h}h ${m}m`;
  return `${Math.max(1, Math.floor(s / 60))}m`;
});

const vmLabel = computed(() => {
  if (!props.overview) return null;
  const { vm_running, vm_total } = props.overview;
  return `${vm_running ?? 0}/${vm_total ?? 0}`;
});
</script>

<template>
  <div v-if="loading && !overview" class="stat-grid">
    <div v-for="i in 6" :key="i" class="stat-card"><div class="skeleton skeleton-stat"></div></div>
  </div>
  <div v-else class="stat-grid">
    <div class="stat-card">
      <div class="stat-label">Host</div>
      <div class="stat-sub">{{ overview?.host || "—" }}</div>
    </div>
    <div class="stat-card">
      <div class="stat-label">VMs running</div>
      <div class="stat-value">{{ vmLabel }}<span class="muted"> online</span></div>
    </div>
    <div class="stat-card">
      <div class="stat-label">Failed</div>
      <div class="stat-value" :style="{ color: overview?.vm_failed ? 'var(--red)' : 'var(--text)' }">
        {{ overview?.vm_failed ?? 0 }}
      </div>
    </div>
    <div class="stat-card">
      <div class="stat-label">Projects</div>
      <div class="stat-value">{{ overview?.projects ?? 0 }}</div>
    </div>
    <div class="stat-card">
      <div class="stat-label">Images</div>
      <div class="stat-value">{{ overview?.images ?? 0 }}</div>
    </div>
    <div class="stat-card">
      <div class="stat-label">Uptime</div>
      <div class="stat-value">{{ uptime || "—" }}</div>
      <div v-if="overview?.version" class="stat-sub">v{{ overview.version }}</div>
    </div>
  </div>
</template>
