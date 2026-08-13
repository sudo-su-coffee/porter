<!-- Porter dashboard style: focused settings workspace with explicit section contracts and editable persisted JSON fields. -->
<script setup>
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api } from "../api/client";
import { toast } from "../components/toast";

const route = useRoute();
const router = useRouter();
const section = computed(() => route.meta.settingsSection || "build");
const projectId = computed(() => route.params.projectId);
const title = computed(() => route.meta.settingsTitle || `${section.value.replaceAll('-', ' ')} settings`);
const readOnly = computed(() => Boolean(route.meta.settingsReadOnly));
const saveMethod = computed(() => route.meta.settingsMethod || (section.value === "general" ? "PATCH" : "PUT"));
const settingsActions = computed(() => route.meta.settingsActions || []);
const values = ref({});
const rows = ref([]);
const error = ref("");
const loading = ref(false);
const busy = ref(false);

function stringify(value) {
  if (typeof value === "string") return value;
  return JSON.stringify(value);
}

function parse(value, original) {
  const text = String(value ?? "").trim();
  if (typeof original === "boolean") return text === "true";
  if (typeof original === "number") return text === "" ? 0 : Number(text);
  if ((text.startsWith("{") && text.endsWith("}")) || (text.startsWith("[") && text.endsWith("]"))) {
    try { return JSON.parse(text); } catch (_) { return text; }
  }
  return text;
}

async function load() {
  loading.value = true;
  error.value = "";
  try {
    const data = await api(`/projects/${projectId.value}/settings/${section.value}`);
    values.value = data && typeof data === "object" ? data : {};
    rows.value = Object.entries(values.value).map(([key, value]) => ({ key, value: stringify(value), original: value }));
  } catch (err) { error.value = err.message; } finally { loading.value = false; }
}

function addRow() { rows.value.push({ key: "", value: "", original: "" }); }
function removeRow(index) { rows.value.splice(index, 1); }

async function save() {
  if (readOnly.value) return;
  const body = {};
  for (const row of rows.value) {
    if (!row.key.trim()) { error.value = "Every setting needs a key."; return; }
    body[row.key.trim()] = parse(row.value, row.original);
  }
  busy.value = true;
  error.value = "";
  try {
    const data = await api(`/projects/${projectId.value}/settings/${section.value}`, { method: saveMethod.value, body: JSON.stringify(body) });
    values.value = data || body;
    rows.value = Object.entries(values.value).map(([key, value]) => ({ key, value: stringify(value), original: value }));
    toast(`${title.value} saved`, "success");
  } catch (err) { error.value = err.message; toast(err.message, "error"); } finally { busy.value = false; }
}

function resolve(path) {
  return String(path).replaceAll(":projectId", encodeURIComponent(projectId.value));
}

async function runAction(action) {
  let value;
  if (action.prompt) {
    value = prompt(action.prompt, action.promptDefault || "");
    if (value === null) return;
  }
  if (action.confirm && !confirm(action.confirm)) return;
  busy.value = true;
  try {
    const body = typeof action.body === "function" ? action.body(value, values.value) : action.body;
    await api(resolve(action.endpoint), { method: action.method || "POST", body: body ? JSON.stringify(body) : undefined });
    toast(action.success || `${action.label} completed`, "success");
    await load();
  } catch (err) {
    error.value = err.message;
    toast(err.message, "error");
  } finally {
    busy.value = false;
  }
}

watch([section, projectId], load);
onMounted(load);
</script>

<template>
  <a class="back-link" @click="router.push({ name: 'project', params: { id: projectId } })">&larr; Project workspace</a>
  <header class="page-header"><div><div class="page-title">{{ title }}</div><div class="page-sub">This page reads the persisted <code>/projects/:projectId/settings/{{ section }}</code> contract.</div></div><div class="detail-actions"><button class="btn btn-sm" :disabled="loading || busy" @click="load">Refresh</button><button v-for="action in settingsActions" :key="action.label" class="btn btn-sm" :disabled="loading || busy" @click="runAction(action)">{{ action.label }}</button><button v-if="!readOnly" class="btn btn-primary btn-sm" :disabled="loading || busy" @click="save">{{ busy ? "Saving…" : `Save ${section}` }}</button></div></header>
  <div v-if="error" class="error-box"><span>{{ error }}</span><button class="btn btn-sm" @click="load">Retry</button></div>
  <div v-if="loading" class="page-sub">Loading persisted settings…</div>
  <section v-else class="card"><div class="card-title">{{ section }} fields</div><p v-if="readOnly" class="page-sub">This endpoint is read-only and reports detected state from the project source and persisted artifacts.</p><p v-else-if="!rows.length" class="page-sub">No saved values exist for this section yet. Add only settings supported by your deployment policy.</p><div v-for="(row, index) in rows" :key="index" class="field-row settings-row"><div class="field"><label>Key</label><input v-model="row.key" :readonly="readOnly" placeholder="setting_name" /></div><div class="field field-wide"><label>Value</label><textarea v-model="row.value" :readonly="readOnly" rows="2" placeholder="value or JSON" /></div><button v-if="!readOnly" class="btn btn-danger btn-sm" @click="removeRow(index)">Remove</button></div><div v-if="!readOnly" class="detail-actions" style="margin-top:16px"><button class="btn btn-sm" @click="addRow">Add field</button><button class="btn btn-primary btn-sm" :disabled="busy" @click="save">Save settings</button></div></section>
</template>
