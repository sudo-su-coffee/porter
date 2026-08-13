<!-- Porter dashboard style: direct source and runtime controls with visible validation, persisted state, and honest Firecracker boundaries. -->
<script setup>
import { computed, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api } from "../api/client";
import { toast } from "../components/toast";

const route = useRoute();
const router = useRouter();
const projectId = route.params.projectId;
const tab = ref(route.query.tab || "compose");
const error = ref("");
const busy = ref(false);
const loading = ref(true);
const composeYaml = ref("");
const composePreview = ref({ services: [] });
const validation = ref(null);
const scale = ref({ desired: 0, current: 0 });
const healthcheck = ref({ enabled: false, type: "http", path: "/", port: 80, interval_sec: 30 });
const autoscale = ref({ enabled: false, min_replicas: 1, max_replicas: 3, target_cpu_percent: 80, scale_down_cpu_percent: 0, cooldown_seconds: 300 });

const tabs = ["compose", "scale", "healthcheck", "autoscale"];
const title = computed(() => ({ compose: "Compose source", scale: "Replica scaling", healthcheck: "Healthcheck", autoscale: "Autoscaling" }[tab.value] || "Project runtime"));

async function loadCompose() {
  const [source, preview] = await Promise.all([api(`/projects/${projectId}/compose`), api(`/projects/${projectId}/compose/preview`)]);
  composeYaml.value = source?.compose_yaml || "";
  composePreview.value = preview || { services: [] };
}

async function loadRuntime() {
  const [s, h, a] = await Promise.all([api(`/projects/${projectId}/scale`), api(`/projects/${projectId}/healthcheck`), api(`/projects/${projectId}/autoscale`)]);
  scale.value = s || { desired: 0, current: 0 };
  healthcheck.value = { ...healthcheck.value, ...(h || {}) };
  autoscale.value = { ...autoscale.value, ...(a || {}) };
}

async function load() {
  loading.value = true;
  error.value = "";
  try { await Promise.all([loadCompose(), loadRuntime()]); }
  catch (err) { error.value = err.message; }
  finally { loading.value = false; }
}

async function validateCompose() {
  busy.value = true;
  try {
    validation.value = await api(`/projects/${projectId}/compose/validate`, { method: "POST", body: JSON.stringify({ compose_yaml: composeYaml.value }) });
    toast("Compose parsed successfully", "success");
    composePreview.value = { ...composePreview.value, services: validation.value.services || [] };
  } catch (err) { validation.value = { valid: false, error: err.message }; toast(err.message, "error"); }
  finally { busy.value = false; }
}

async function saveCompose() {
  busy.value = true;
  try { await api(`/projects/${projectId}/compose`, { method: "PUT", body: JSON.stringify({ compose_yaml: composeYaml.value }) }); toast("Compose source saved", "success"); await loadCompose(); }
  catch (err) { toast(err.message, "error"); }
  finally { busy.value = false; }
}

async function saveScale() {
  busy.value = true;
  try { await api(`/projects/${projectId}/scale`, { method: "PATCH", body: JSON.stringify({ replicas: Number(scale.value.desired) }) }); toast("Replica scale applied", "success"); await loadRuntime(); }
  catch (err) { toast(err.message, "error"); }
  finally { busy.value = false; }
}

async function saveHealthcheck() {
  busy.value = true;
  try { await api(`/projects/${projectId}/healthcheck`, { method: "PUT", body: JSON.stringify(healthcheck.value) }); toast("Healthcheck saved", "success"); await loadRuntime(); }
  catch (err) { toast(err.message, "error"); }
  finally { busy.value = false; }
}

async function saveAutoscale() {
  busy.value = true;
  try { await api(`/projects/${projectId}/autoscale`, { method: "PUT", body: JSON.stringify(autoscale.value) }); toast("Autoscaling policy saved", "success"); await loadRuntime(); }
  catch (err) { toast(err.message, "error"); }
  finally { busy.value = false; }
}

onMounted(load);
</script>

<template>
  <a class="back-link" @click="router.push({ name: 'project', params: { id: projectId } })">&larr; Project workspace</a>
  <header class="page-header"><div><div class="page-title">{{ title }}</div><div class="page-sub">Source and runtime controls backed by real project APIs. Compose-to-guest conversion remains an explicit backend limitation.</div></div><button class="btn btn-sm" :disabled="loading || busy" @click="load">Refresh</button></header>
  <div v-if="error" class="error-box"><span>{{ error }}</span><button class="btn btn-sm" @click="load">Retry</button></div>
  <div class="seg stream-tabs"><button v-for="item in tabs" :key="item" :class="{ active: tab === item }" @click="tab = item">{{ item }}</button></div>
  <div v-if="loading" class="page-sub">Loading persisted source and runtime state…</div>

  <template v-else-if="tab === 'compose'">
    <section class="card"><div class="card-head"><div class="card-title">Compose YAML</div><span class="tag tag-amber">conversion boundary</span></div><p class="page-sub">Porter can parse and persist this source. It does not claim that arbitrary Docker Compose services boot directly as Firecracker guests.</p><textarea v-model="composeYaml" rows="18" class="source-editor" placeholder="services:\n  web:\n    image: base://validated-artifact"></textarea><div class="detail-actions" style="margin-top:14px"><button class="btn btn-sm" :disabled="busy" @click="validateCompose">Validate Compose</button><button class="btn btn-primary btn-sm" :disabled="busy" @click="saveCompose">Save source</button></div></section>
    <section class="card" style="margin-top:16px"><div class="card-title">Parsed service preview</div><div v-if="validation" class="page-sub" :class="validation.valid ? 'surface-good' : 'surface-warn'">{{ validation.valid ? 'Valid Compose syntax' : validation.error }}</div><div v-if="composePreview.services?.length" class="resource-link-grid"><div v-for="service in composePreview.services" :key="service" class="resource-link"><strong>{{ service }}</strong><span>parsed service</span></div></div><div v-else class="empty-state">No parsed services returned by the backend.</div></section>
  </template>

  <template v-else-if="tab === 'scale'"><section class="card"><div class="card-title">Replica pool</div><div class="resource-object-grid"><div class="stat-card"><div class="stat-label">Current</div><div class="stat-value">{{ scale.current }}</div></div><div class="stat-card"><div class="stat-label">Desired</div><div class="stat-value">{{ scale.desired }}</div></div></div><div class="field"><label>Desired replicas</label><input v-model.number="scale.desired" type="number" min="0" /></div><button class="btn btn-primary btn-sm" :disabled="busy" @click="saveScale">Apply scale</button></section></template>

  <template v-else-if="tab === 'healthcheck'"><section class="card"><div class="card-title">Healthcheck policy</div><div class="field"><label>Enabled</label><label class="toggle"><input v-model="healthcheck.enabled" type="checkbox" /><span></span></label></div><div class="field-row"><div class="field"><label>Type</label><select v-model="healthcheck.type"><option value="http">HTTP</option><option value="tcp">TCP</option></select></div><div class="field"><label>Path</label><input v-model="healthcheck.path" placeholder="/health" /></div></div><div class="field-row"><div class="field"><label>Port</label><input v-model.number="healthcheck.port" type="number" min="1" /></div><div class="field"><label>Interval (seconds)</label><input v-model.number="healthcheck.interval_sec" type="number" min="1" /></div></div><button class="btn btn-primary btn-sm" :disabled="busy" @click="saveHealthcheck">Save healthcheck</button></section></template>

  <template v-else><section class="card"><div class="card-title">Autoscaling policy</div><div class="field"><label>Enabled</label><label class="toggle"><input v-model="autoscale.enabled" type="checkbox" /><span></span></label></div><div class="field-row"><div class="field"><label>Minimum replicas</label><input v-model.number="autoscale.min_replicas" type="number" min="0" /></div><div class="field"><label>Maximum replicas</label><input v-model.number="autoscale.max_replicas" type="number" min="1" /></div></div><div class="field-row"><div class="field"><label>Scale-up CPU %</label><input v-model.number="autoscale.target_cpu_percent" type="number" min="1" max="100" /></div><div class="field"><label>Scale-down CPU %</label><input v-model.number="autoscale.scale_down_cpu_percent" type="number" min="0" max="100" /></div></div><div class="field"><label>Cooldown seconds</label><input v-model.number="autoscale.cooldown_seconds" type="number" min="1" /></div><button class="btn btn-primary btn-sm" :disabled="busy" @click="saveAutoscale">Save autoscaling</button></section></template>
</template>
