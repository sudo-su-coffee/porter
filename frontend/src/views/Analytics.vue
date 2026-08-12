<script setup>
import { ref, onMounted, computed } from "vue";
import { api } from "../api/client";
import Sparkline from "../components/Sparkline.vue";

const usage = ref(null);
const error = ref("");
const loading = ref(true);
const period = ref("24h");

const PERIODS = [
  { label: "24h", value: "24h" },
  { label: "7d", value: "168h" },
  { label: "30d", value: "720h" },
];

const reqSeries = computed(() =>
  (usage.value?.series || []).map((d) => Number(d.requests) || 0)
);
const bwSeries = computed(() =>
  (usage.value?.series || []).map((d) => Number(d.bandwidth) || 0)
);

const byProject = computed(() => {
  const out = [];
  const bp = usage.value?.by_project || {};
  for (const pid of Object.keys(bp)) {
    out.push({ project_id: pid, ...bp[pid] });
  }
  return out.sort((a, b) => (b.requests || 0) - (a.requests || 0));
});

function fmtBytes(n) {
  const v = Number(n) || 0;
  if (v >= 1e9) return (v / 1e9).toFixed(2) + " GB";
  if (v >= 1e6) return (v / 1e6).toFixed(1) + " MB";
  if (v >= 1e3) return (v / 1e3).toFixed(1) + " KB";
  return v + " B";
}

async function load() {
  loading.value = true;
  error.value = "";
  try {
    usage.value = await api(`/usage?period=${period.value}`);
  } catch (e) {
    error.value = e.message;
  } finally {
    loading.value = false;
  }
}

onMounted(load);
</script>

<template>
  <div class="page-header">
    <div>
      <div class="page-title">Usage &amp; Analytics</div>
      <div class="page-sub">Platform-wide edge requests, data transfer, and per-project breakdown</div>
    </div>
    <div class="seg">
      <button
        v-for="p in PERIODS"
        :key="p.value"
        :class="{ active: period === p.value }"
        @click="period = p.value; load()"
      >{{ p.label }}</button>
    </div>
  </div>

  <div v-if="error" class="error-box">{{ error }}</div>
  <p v-if="loading && !usage" class="page-sub">Loading…</p>

  <template v-if="usage">
    <div class="stat-grid">
      <div class="stat-card">
        <div class="stat-label">Edge requests</div>
        <div class="stat-value">{{ usage.edge_requests || 0 }}</div>
        <div class="stat-sub">{{ usage.period }} window</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">Function invocations</div>
        <div class="stat-value">{{ usage.function_invocations || 0 }}</div>
        <div class="stat-sub">path-based</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">Data transfer</div>
        <div class="stat-value">{{ fmtBytes(usage.fast_data_transfer) }}</div>
        <div class="stat-sub">{{ fmtBytes(usage.data_transfer_in) }} in / {{ fmtBytes(usage.data_transfer_out) }} out</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">Projects</div>
        <div class="stat-value">{{ usage.projects || 0 }}</div>
      </div>
    </div>

    <div class="card" style="margin-bottom:18px">
      <div class="page-sub" style="margin-bottom:6px">Requests by day</div>
      <Sparkline :data="reqSeries" />
    </div>

    <div class="card" style="margin-bottom:18px">
      <div class="page-sub" style="margin-bottom:6px">Data transfer by day</div>
      <Sparkline :data="bwSeries" />
    </div>

    <div class="page-sub" style="margin-bottom:8px">Per-project usage</div>
    <div class="table-wrap">
      <table class="data-table">
        <thead>
          <tr><th>Project</th><th>Requests</th><th>Bandwidth</th></tr>
        </thead>
        <tbody>
          <tr v-for="p in byProject" :key="p.project_id">
            <td class="mono">{{ p.project_id }}</td>
            <td class="num">{{ p.requests || 0 }}</td>
            <td class="num">{{ fmtBytes(p.bandwidth) }}</td>
          </tr>
          <tr v-if="!byProject.length">
            <td colspan="3" class="hint" style="text-align:center; padding:18px">No traffic in this window.</td>
          </tr>
        </tbody>
      </table>
    </div>
  </template>
</template>