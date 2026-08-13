<!-- Porter dashboard style: operational glass workspace, graphite hierarchy, teal actions, and honest live/empty/error states. -->
<script setup>
import { computed, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api } from "../api/client";
import { toast } from "../components/toast";

const route = useRoute();
const router = useRouter();
const projectId = computed(() => route.params.projectId);
const environments = ref([]);
const available = ref([]);
const ranges = ref({});
const error = ref("");
const busy = ref(false);
const form = ref({ name: "", branch: "main", url: "", env_domain: "" });
const editing = ref(null);

async function load() {
  error.value = "";
  try {
    const [data, options] = await Promise.all([api(`/projects/${projectId.value}/environments`), api(`/projects/${projectId.value}/environments/available`)]);
    environments.value = Array.isArray(data) ? data : data?.environments || [];
    available.value = options?.available || [];
  } catch (err) {
    error.value = err.message;
  }
}

async function create() {
  if (!form.value.name.trim()) {
    error.value = "Environment name is required.";
    return;
  }
  busy.value = true;
  try {
    await api(`/projects/${projectId.value}/environments`, { method: "POST", body: JSON.stringify(form.value) });
    form.value = { name: "", branch: "main", url: "", env_domain: "" };
    toast("Environment created", "success");
    await load();
  } catch (err) {
    toast(err.message, "error");
  } finally {
    busy.value = false;
  }
}

function beginEdit(environment) {
  editing.value = { ...environment };
}

async function saveEdit() {
  if (!editing.value) return;
  busy.value = true;
  try {
    const { id, project_id: _projectId, name: _name, created_at: _createdAt, ...body } = editing.value;
    await api(`/projects/${projectId.value}/environments/${encodeURIComponent(editing.value.id)}`, { method: "PATCH", body: JSON.stringify(body) });
    editing.value = null;
    toast("Environment updated", "success");
    await load();
  } catch (err) {
    toast(err.message, "error");
  } finally {
    busy.value = false;
  }
}

async function remove(environment) {
  if (!confirm(`Delete environment "${environment.name}"?`)) return;
  busy.value = true;
  try {
    await api(`/projects/${projectId.value}/environments/${encodeURIComponent(environment.id)}`, { method: "DELETE" });
    toast("Environment deleted", "success");
    await load();
  } catch (err) {
    toast(err.message, "error");
  } finally {
    busy.value = false;
  }
}

async function setBranch(environment) {
  const branch = prompt("Branch", environment.branch || "main");
  if (branch === null) return;
  try { await api(`/projects/${projectId.value}/environments/${encodeURIComponent(environment.id)}/branch`, { method: "POST", body: JSON.stringify({ branch }) }); toast("Environment branch updated", "success"); await load(); }
  catch (err) { toast(err.message, "error"); }
}

async function setDomain(environment) {
  const envDomain = prompt("Preview domain", environment.env_domain || "");
  if (envDomain === null) return;
  try { await api(`/projects/${projectId.value}/environments/${encodeURIComponent(environment.id)}/domain`, { method: "POST", body: JSON.stringify({ env_domain: envDomain }) }); toast("Environment domain updated", "success"); await load(); }
  catch (err) { toast(err.message, "error"); }
}

async function loadRange(environment) {
  try { ranges.value = { ...ranges.value, [environment.id]: await api(`/projects/${projectId.value}/environments/${encodeURIComponent(environment.id)}/range`) }; }
  catch (err) { toast(err.message, "error"); }
}

onMounted(load);
</script>

<template>
  <a class="back-link" @click="router.push({ name: 'project', params: { id: projectId } })">&larr; Project workspace</a>
  <header class="page-header"><div><div class="page-title">Environments</div><div class="page-sub">Branches, preview domains, and deployment targets backed by the control plane.</div></div><button class="btn btn-sm" :disabled="busy" @click="load">Refresh</button></header>
  <div v-if="error" class="error-box"><span>{{ error }}</span><button class="btn btn-sm" @click="load">Retry</button></div>
  <section class="card" style="margin-bottom:16px"><div class="card-title">Available targets</div><div class="resource-link-grid"><div v-for="name in available" :key="name" class="resource-link"><strong class="mono">{{ name }}</strong><span>available environment target</span></div><div v-if="!available.length" class="empty-state">No environment targets returned.</div></div></section>

  <section class="card resource-create-card">
    <div class="card-title">Add environment</div>
    <div class="resource-form-grid">
      <div class="field"><label>Name</label><input v-model="form.name" placeholder="preview" required /></div>
      <div class="field"><label>Branch</label><input v-model="form.branch" placeholder="main" /></div>
      <div class="field"><label>URL</label><input v-model="form.url" placeholder="https://preview.example.com" /></div>
      <div class="field"><label>Environment domain</label><input v-model="form.env_domain" placeholder="preview.example.com" /></div>
    </div>
    <button class="btn btn-primary btn-sm" :disabled="busy" @click="create">{{ busy ? "Saving…" : "Create environment" }}</button>
  </section>

  <div v-if="!environments.length" class="empty-state"><strong>No environments yet.</strong><span>Porter did not fabricate environment records.</span></div>
  <div v-else class="table-wrap"><table class="data-table"><thead><tr><th>Name</th><th>Branch</th><th>URL</th><th>Domain</th><th>Actions</th></tr></thead><tbody>
    <tr v-for="environment in environments" :key="environment.id">
      <template v-if="editing?.id === environment.id"><td class="mono">{{ environment.name }}</td><td><input v-model="editing.branch" /></td><td><input v-model="editing.url" /></td><td><input v-model="editing.env_domain" /></td><td><button class="btn btn-sm btn-primary" :disabled="busy" @click="saveEdit">Save</button><button class="btn btn-sm" @click="editing = null">Cancel</button></td></template>
      <template v-else><td class="mono">{{ environment.name }}</td><td class="mono">{{ environment.branch || '—' }}</td><td class="mono">{{ environment.url || '—' }}</td><td class="mono">{{ environment.env_domain || '—' }}</td><td><button class="btn btn-sm" @click="beginEdit(environment)">Edit</button><button class="btn btn-sm" @click="setBranch(environment)">Branch</button><button class="btn btn-sm" @click="setDomain(environment)">Domain</button><button class="btn btn-sm" @click="loadRange(environment)">Range</button><button class="btn btn-danger btn-sm" :disabled="busy" @click="remove(environment)">Delete</button><pre v-if="ranges[environment.id]" class="settings-json">{{ JSON.stringify(ranges[environment.id]) }}</pre></td></template>
    </tr>
  </tbody></table></div>
</template>
