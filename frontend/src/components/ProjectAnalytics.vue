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
const requests = ref(null);
const invocations = ref(null);
const vitals = ref(null);
const vitalSeries = ref(null);
const vitalBreakdowns = ref({});
const beacon = ref({ path: "/", lcp_ms: 0, cls: 0, inp_ms: 0, ttfb_ms: 0 });
const beaconMessage = ref("");

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
    const [u, ts, p, c, b, r, i, v, vs, lcp, cls, fid] = await Promise.allSettled([
      api(`${base}/usage`),
      api(`${base}/usage/timeseries`),
      api(`${base}/paths`),
      api(`${base}/status-codes`),
      api(`${base}/bandwidth`),
      api(`${base}/requests`),
      api(`${base}/invocations`),
      api(`/projects/${props.projectId}/observability/web-vitals`),
      api(`/projects/${props.projectId}/observability/web-vitals/timeseries`),
      api(`/projects/${props.projectId}/observability/lcp`),
      api(`/projects/${props.projectId}/observability/cls`),
      api(`/projects/${props.projectId}/observability/fid`),
    ]);
    usage.value = u.status === "fulfilled" ? u.value : null;
    timeseries.value = ts.status === "fulfilled" ? ts.value?.series || [] : [];
    paths.value = p.status === "fulfilled" ? p.value?.paths || [] : [];
    codes.value = c.status === "fulfilled" ? c.value?.status_codes || {} : {};
    band.value = b.status === "fulfilled" ? b.value : null;
    requests.value = r.status === "fulfilled" ? r.value : null;
    invocations.value = i.status === "fulfilled" ? i.value : null;
    vitals.value = v.status === "fulfilled" ? v.value : null;
    vitalSeries.value = vs.status === "fulfilled" ? vs.value : null;
    vitalBreakdowns.value = { lcp: lcp.status === "fulfilled" ? lcp.value : null, cls: cls.status === "fulfilled" ? cls.value : null, fid: fid.status === "fulfilled" ? fid.value : null };
  } catch (e) {
    error.value = e.message;
  }
}

async function recordBeacon() {
  try {
    const values = Object.fromEntries(Object.entries(beacon.value).filter(([key, value]) => key !== "path" && Number(value) > 0).map(([key, value]) => [key, Number(value)]));
    const result = await api(`/projects/${props.projectId}/observability/web-vitals/beacon`, { method: "POST", body: JSON.stringify({ path: beacon.value.path, values }) });
    beaconMessage.value = result?.status || "recorded";
    await load();
  } catch (e) { beaconMessage.value = e.message; }
}

onMounted(load);
</script>

<template>
  <div v-if="error" class="error-box">{{ error }}</div>

  <div class="stat-grid" v-if="usage">
    <div class="stat-card"><div class="stat-label">Requests</div><div class="stat-value">{{ usage.requests || 0 }}</div></div>
    <div class="stat-card"><div class="stat-label">Bandwidth</div><div class="stat-value">{{ fmtBytes(usage.bandwidth) }}</div><div class="stat-sub">{{ fmtBytes(usage.bytes_in) }} in / {{ fmtBytes(usage.bytes_out) }} out</div></div>
    <div class="stat-card"><div class="stat-label">Invocations</div><div class="stat-value">{{ usage.invocations || invocations?.invocations || 0 }}</div></div>
    <div class="stat-card"><div class="stat-label">Request endpoint</div><div class="stat-value">{{ requests?.requests ?? usage.requests ?? 0 }}</div></div>
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
  <section class="card" style="margin-top:18px"><div class="card-head"><div class="card-title">Web Vitals</div><span class="hint">{{ vitals?.length || vitals?.metrics?.length || 0 }} metric group(s)</span></div><pre class="settings-json">{{ JSON.stringify({ aggregate: vitals, timeseries: vitalSeries, lcp: vitalBreakdowns.lcp, cls: vitalBreakdowns.cls, fid: vitalBreakdowns.fid }, null, 2) }}</pre><div class="resource-form-grid"><div class="field"><label>Path</label><input v-model="beacon.path" /></div><div class="field"><label>LCP (ms)</label><input v-model.number="beacon.lcp_ms" type="number" /></div><div class="field"><label>CLS</label><input v-model.number="beacon.cls" type="number" step="0.01" /></div><div class="field"><label>INP (ms)</label><input v-model.number="beacon.inp_ms" type="number" /></div><div class="field"><label>TTFB (ms)</label><input v-model.number="beacon.ttfb_ms" type="number" /></div></div><div class="detail-actions"><button class="btn btn-sm" @click="recordBeacon">Record Web Vitals beacon</button><span class="hint">{{ beaconMessage }}</span></div></section>
</template>
