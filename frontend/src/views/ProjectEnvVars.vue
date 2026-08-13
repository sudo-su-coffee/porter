<!-- Porter dashboard style: calm data-dense workspace with explicit secret-safe boundaries and real API feedback. -->
<script setup>
import { onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api } from "../api/client";
import { toast } from "../components/toast";

const route = useRoute();
const router = useRouter();
const projectId = route.params.projectId;
const variables = ref([]);
const form = ref({ key: "", value: "" });
const bulkText = ref("{}\n");
const editingKey = ref("");
const editingValue = ref("");
const error = ref("");
const busy = ref(false);

async function load() {
  error.value = "";
  try {
    const data = await api(`/projects/${projectId}/env`);
    variables.value = Array.isArray(data) ? data : [];
  } catch (err) {
    error.value = err.message;
  }
}

async function add() {
  if (!form.value.key.trim()) {
    error.value = "Variable key is required.";
    return;
  }
  busy.value = true;
  try {
    await api(`/projects/${projectId}/env`, { method: "POST", body: JSON.stringify(form.value) });
    form.value = { key: "", value: "" };
    toast("Environment variable saved", "success");
    await load();
  } catch (err) {
    toast(err.message, "error");
  } finally {
    busy.value = false;
  }
}

async function saveBulk() {
  let values;
  try { values = JSON.parse(bulkText.value); }
  catch (_) { error.value = "Bulk variables must be a JSON object."; return; }
  if (!values || Array.isArray(values) || typeof values !== "object") { error.value = "Bulk variables must be a JSON object."; return; }
  busy.value = true;
  try { await api(`/projects/${projectId}/env/bulk`, { method: "POST", body: JSON.stringify(Object.fromEntries(Object.entries(values).map(([key, value]) => [key, String(value)]))) }); toast("Bulk environment variables saved", "success"); await load(); }
  catch (err) { toast(err.message, "error"); }
  finally { busy.value = false; }
}

function beginEdit(variable) {
  editingKey.value = variable.key;
  editingValue.value = variable.value || "";
}

async function save(variable) {
  busy.value = true;
  try {
    await api(`/projects/${projectId}/env/${encodeURIComponent(variable.key)}`, { method: "PATCH", body: JSON.stringify({ value: editingValue.value }) });
    editingKey.value = "";
    toast("Environment variable updated", "success");
    await load();
  } catch (err) {
    toast(err.message, "error");
  } finally {
    busy.value = false;
  }
}

async function remove(variable) {
  if (!confirm(`Delete environment variable ${variable.key}?`)) return;
  busy.value = true;
  try {
    await api(`/projects/${projectId}/env/${encodeURIComponent(variable.key)}`, { method: "DELETE" });
    toast("Environment variable deleted", "success");
    await load();
  } catch (err) {
    toast(err.message, "error");
  } finally {
    busy.value = false;
  }
}

onMounted(load);
</script>

<template>
  <a class="back-link" @click="router.push({ name: 'project', params: { id: projectId } })">&larr; Project workspace</a>
  <header class="page-header"><div><div class="page-title">Environment variables</div><div class="page-sub">Runtime variables returned by the project API. Use project secrets for encrypted values.</div></div><button class="btn btn-sm" :disabled="busy" @click="load">Refresh</button></header>
  <div v-if="error" class="error-box"><span>{{ error }}</span><button class="btn btn-sm" @click="load">Retry</button></div>
  <section class="card resource-create-card"><div class="card-title">Add variable</div><div class="field-row"><div class="field"><label>Key</label><input v-model="form.key" placeholder="DATABASE_URL" /></div><div class="field"><label>Value</label><input v-model="form.value" placeholder="postgres://…" /></div><button class="btn btn-primary btn-sm" :disabled="busy" @click="add">Save</button></div></section>
  <section class="card" style="margin-bottom:16px"><div class="card-title">Bulk variables</div><p class="page-sub">Send a JSON object to the persisted bulk endpoint. Values are converted to strings by the backend contract.</p><textarea v-model="bulkText" rows="7" class="source-editor"></textarea><div class="detail-actions" style="margin-top:12px"><button class="btn btn-sm" :disabled="busy" @click="saveBulk">Save bulk variables</button></div></section>
  <div v-if="!variables.length" class="empty-state"><strong>No environment variables yet.</strong><span>Porter did not fabricate runtime configuration.</span></div>
  <div v-else class="table-wrap"><table class="data-table"><thead><tr><th>Key</th><th>Value</th><th>Actions</th></tr></thead><tbody>
    <tr v-for="variable in variables" :key="variable.key"><td class="mono">{{ variable.key }}</td><td v-if="editingKey !== variable.key" class="mono">{{ variable.value }}</td><td v-else><input v-model="editingValue" class="mono" /></td><td><template v-if="editingKey === variable.key"><button class="btn btn-sm btn-primary" :disabled="busy" @click="save(variable)">Save</button><button class="btn btn-sm" @click="editingKey = ''">Cancel</button></template><template v-else><button class="btn btn-sm" @click="beginEdit(variable)">Edit</button><button class="btn btn-danger btn-sm" :disabled="busy" @click="remove(variable)">Delete</button></template></td></tr>
  </tbody></table></div>
</template>
