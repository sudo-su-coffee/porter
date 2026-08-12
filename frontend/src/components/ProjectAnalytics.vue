<script setup>
import { ref, onMounted, computed } from "vue";
import { api } from "../api/client";
import Sparkline from "../components/Sparkline.vue";

const props = defineProps({ projectId: { type: String, required: true } });
const error = ref("");
const usage = ref(null);
const timeseries = ref([]);
const paths = ref([]);
const codes = ref(null);
const band = ref(null);

const series = computed(() => (timeseries.value || []).map((p) => p.requests || 0));

function fmtBytes(n) {
  const v = Number(n) || 0;
  if (v >= 1e9) return (v / 1e9).toFixed(2) + " GB";
  if (v >= 1e6) return (v / 1e6).toFixed(1) + " MB";
  if (v >= 1e3) return (v / 1e3).toFixed(1) + " KB";
  return v + " B";
}

const codeRows = computed(() => {
  const c = codes.value || {};
  const keys = Object.keys(c).sort((a, b) => Number(b) - Number(a));
  return keys.map((k) => ({ code: k, hits: c[k] }));
});

async function load() {
  error.value = "";
  const base = `/projects/${props.projectId}/analytics`;
  try {
    const [u, ts, p, c, b] = await Promise.allSettled([
      api(`${base}/usage`),
      api(`${base}/usage/timeseries`),
      api(`${base}/paths`),
      api(`${base}/status-codes`),
      api(`${base}/bandwidth`),
    ]);
    usage.value = u.status === "fulfilled" ? u.value : null;
    timeseries.value = ts.status === "fulfilled" ? ts.value?.series || [] : [];
    paths.value = p.status === "fulfilled" ? p.value?.paths || [] : [];
    codes.value = c.status === "fulfilled" ? c.value?.status_codes || {} : {};
    band.value = b.status === "fulfilled" ? b.value : null;
  } catch (e) {
    error.value = e.message;
  }
}

onMounted(load);
</script>

<template>
  <div v-if="error" class="error-box">{{ error }}</div>

  <div class="stat-grid" v-if="usage">
    <div class="stat-card"><div class="stat-label">Requests</div><div class="stat-value">{{ usage.requests || 0 }}</div></div>
    <div class="stat-card"><div class="stat-label">Bandwidth</div><div class="stat-value">{{ fmtBytes(usage.bandwidth) }}</div><div class="stat-sub">{{ fmtBytes(usage.bytes_in) }} in / {{ fmtBytes(usage.bytes_out) }} out</div></div>
    <div class="stat-card"><div class="stat-label">Invocations</div><div class="stat-value">{{ usage.invocations || 0 }}</div></div>
  </div>

  <div class="card" style="margin-bottom:18px">
    <div class="page-sub" style="margin-bottom:6px">Requests per 5-min bucket</div>
    <Sparkline :data="series" />
  </div>

  <div style="display:grid; grid-template-columns: 1fr 1fr; gap:18px">
    <div class="card">
      <div class="page-sub" style="margin-bottom:8px">Top paths</div>
      <table class="data-table">
        <tbody>
          <tr v-for="row in paths.slice(0, 12)" :key="row.path">
            <td class="mono">{{ row.path }}</td>
            <td class="num" style="text-align:right">{{ row.hits }}</td>
          </tr>
          <tr v-if="!paths.length"><td class="hint" style="padding:14px; text-align:center">No paths yet.</td></tr>
        </tbody>
      </table>
    </div>
    <div class="card">
      <div class="page-sub" style="margin-bottom:8px">Status codes</div>
      <table class="data-table">
        <tbody>
          <tr v-for="row in codeRows" :key="row.code">
            <td class="mono"><span :style="{ color: Number(row.code) < 400 ? 'var(--green)' : 'var(--red)' }">{{ row.code }}</span></td>
            <td class="num" style="text-align:right">{{ row.hits }}</td>
          </tr>
          <tr v-if="!codeRows.length"><td class="hint" style="padding:14px; text-align:center">No responses yet.</td></tr>
        </tbody>
      </table>
    </div>
  </div>
</template>