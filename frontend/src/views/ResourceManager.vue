<!-- Style: Whatomate-inspired Porter workspace—quiet operator surfaces, explicit
     runtime state, and reusable data/action patterns instead of chat-specific UI. -->
<script setup>
import { computed, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api } from "../api/client";
import { toast } from "../components/toast";

const route = useRoute();
const router = useRouter();
const resource = computed(() => route.meta.resource || {});
const data = ref(null);
const loading = ref(true);
const error = ref("");
const busy = ref(false);
const showCreate = ref(false);
const query = ref("");
const form = ref({});
const formError = ref("");

function endpoint(template, row = null) {
  return String(template || route.path).replace(/:([A-Za-z0-9_]+)/g, (_, key) => {
    if (route.params[key]) return encodeURIComponent(route.params[key]);
    if (row && row[key]) return encodeURIComponent(row[key]);
    if (row && key === "id" && row.id) return encodeURIComponent(row.id);
    return "";
  });
}

function rowsFrom(value) {
  if (Array.isArray(value)) return value;
  if (!value || typeof value !== "object") return [];
  for (const key of ["items", "data", "members", "builds", "events", "logs", "services", "volumes", "domains", "rules", "alerts", "hooks", "crons", "redirects", "environments"]) {
    if (Array.isArray(value[key])) return value[key];
  }
  return [];
}

const rows = computed(() => {
  const list = rowsFrom(data.value);
  const needle = query.value.trim().toLowerCase();
  if (!needle) return list;
  return list.filter((row) => JSON.stringify(row).toLowerCase().includes(needle));
});

const columns = computed(() => {
  if (Array.isArray(resource.value.columns) && resource.value.columns.length) return resource.value.columns;
  const first = rows.value.find((row) => row && typeof row === "object" && !Array.isArray(row));
  return first ? Object.keys(first).filter((key) => !["secret", "token", "value_encrypted"].includes(key)).slice(0, 8) : [];
});

const objectEntries = computed(() => {
  if (!data.value || typeof data.value !== "object" || Array.isArray(data.value) || rows.value.length) return [];
  return Object.entries(data.value).filter(([key]) => !["secret", "token", "value_encrypted"].includes(key));
});

function format(value) {
  if (value === null || value === undefined || value === "") return "—";
  if (typeof value === "boolean") return value ? "Yes" : "No";
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}

function resetForm() {
  const next = {};
  for (const field of resource.value.createFields || []) next[field.key] = field.default ?? (field.type === "checkbox" ? false : "");
  form.value = next;
}

function valueFor(field) {
  if (field.type === "number") return form.value[field.key] === "" ? 0 : Number(form.value[field.key]);
  if (field.type === "checkbox") return Boolean(form.value[field.key]);
  if (field.type === "csv") return String(form.value[field.key] || "").split(",").map((value) => value.trim()).filter(Boolean);
  return String(form.value[field.key] ?? "").trim();
}

function openCreate() {
  resetForm();
  formError.value = "";
  showCreate.value = true;
}

async function load() {
  loading.value = true;
  error.value = "";
  try {
    data.value = await api(endpoint(resource.value.endpoint));
  } catch (err) {
    error.value = err.message;
  } finally {
    loading.value = false;
  }
}

async function create() {
  const body = {};
  for (const field of resource.value.createFields || []) {
    const value = valueFor(field);
    if (field.required !== false && value === "") {
      formError.value = `${field.label || field.key} is required.`;
      return;
    }
    body[field.key] = value;
  }
  busy.value = true;
  formError.value = "";
  try {
    await api(endpoint(resource.value.endpoint), { method: resource.value.createMethod || "POST", body: JSON.stringify(body) });
    toast(resource.value.createSuccess || "Resource created", "success");
    showCreate.value = false;
    await load();
  } catch (err) {
    toast(err.message, "error");
  } finally {
    busy.value = false;
  }
}

async function invoke(action, row) {
  if (action.confirm && !confirm(action.confirm.replace("{name}", row?.name || row?.id || "this resource"))) return;
  let promptValue;
  if (action.prompt) {
    promptValue = prompt(action.prompt, action.promptDefault ?? "");
    if (promptValue === null) return;
  }
  busy.value = true;
  try {
    const body = typeof action.body === "function" ? action.body(row, promptValue) : action.body;
    await api(endpoint(action.endpoint, row), { method: action.method || "POST", body: body ? JSON.stringify(body) : undefined });
    toast(action.success || "Action completed", "success");
    await load();
  } catch (err) {
    toast(err.message, "error");
  } finally {
    busy.value = false;
  }
}

function visible(action, row) {
  if (!action.when) return true;
  return row?.[action.when.key] === action.when.equals;
}

onMounted(() => {
  resetForm();
  load();
});
</script>

<template>
  <a v-if="resource.back" class="back-link" @click="router.back()">&larr; Back</a>
  <header class="page-header">
    <div>
      <div class="page-title">{{ resource.title || "Resource" }}</div>
      <div class="page-sub">{{ resource.description || "Live data and actions from the Porter control plane." }}</div>
    </div>
    <div class="detail-actions">
      <button v-if="resource.createFields?.length" class="btn btn-sm btn-primary" type="button" @click="openCreate">{{ resource.createLabel || "Create" }}</button>
      <button class="btn btn-sm" type="button" :disabled="loading || busy" @click="load">Refresh</button>
    </div>
  </header>

  <div v-if="error" class="error-box"><span>{{ error }}</span><button class="btn btn-sm" type="button" @click="load">Retry</button></div>
  <div v-if="resource.note" class="notice-box">{{ resource.note }}</div>
  <div v-if="resource.endpoint" class="page-sub resource-endpoint">Live endpoint: <code>{{ endpoint(resource.endpoint) }}</code></div>

  <section v-if="showCreate" class="card resource-create-card">
    <div class="card-title">{{ resource.createLabel || "Create resource" }}</div>
    <div v-if="formError" class="error-box">{{ formError }}</div>
    <div class="resource-form-grid">
      <div v-for="field in resource.createFields || []" :key="field.key" class="field" :class="field.wide ? 'field-wide' : ''">
        <label :for="`resource-${field.key}`">{{ field.label }}</label>
        <textarea v-if="field.type === 'textarea'" :id="`resource-${field.key}`" v-model="form[field.key]" :placeholder="field.placeholder" :rows="field.rows || 3" />
        <select v-else-if="field.type === 'select'" :id="`resource-${field.key}`" v-model="form[field.key]">
          <option v-for="option in field.options || []" :key="option.value ?? option" :value="option.value ?? option">{{ option.label ?? option }}</option>
        </select>
        <input v-else-if="field.type === 'checkbox'" :id="`resource-${field.key}`" v-model="form[field.key]" type="checkbox" />
        <input v-else :id="`resource-${field.key}`" v-model="form[field.key]" :type="field.type === 'number' ? 'number' : 'text'" :placeholder="field.placeholder" :required="field.required !== false" />
      </div>
    </div>
    <div class="detail-actions" style="margin-top:14px">
      <button class="btn btn-primary btn-sm" type="button" :disabled="busy" @click="create">{{ busy ? "Saving…" : "Save" }}</button>
      <button class="btn btn-sm" type="button" @click="showCreate = false">Cancel</button>
    </div>
  </section>

  <div v-if="loading" class="page-sub">Loading live resource data…</div>
  <template v-else>
    <div v-if="rows.length || columns.length" class="filter-bar">
      <input v-if="rows.length > 4" v-model="query" placeholder="Filter live records…" />
      <span class="hint">{{ rows.length }} record(s)</span>
    </div>
    <div v-if="objectEntries.length" class="resource-object-grid">
      <div v-for="([key, value]) in objectEntries" :key="key" class="stat-card"><div class="stat-label">{{ key.replaceAll("_", " ") }}</div><div class="stat-value mono">{{ format(value) }}</div></div>
    </div>
    <div v-if="objectEntries.length && resource.objectActions?.length" class="detail-actions" style="margin-bottom:16px">
      <button v-for="action in resource.objectActions" :key="action.label" class="btn btn-sm" :class="action.danger ? 'btn-danger' : ''" :disabled="busy" type="button" @click="invoke(action, data)">{{ action.label }}</button>
    </div>
    <div v-if="rows.length" class="table-wrap">
      <table class="data-table">
        <thead><tr><th v-for="column in columns" :key="column">{{ column.replaceAll("_", " ") }}</th><th v-if="resource.rowActions?.length">Actions</th></tr></thead>
        <tbody>
          <tr v-for="(row, index) in rows" :key="row.id || row.name || index">
            <td v-for="column in columns" :key="column" class="mono">{{ format(row[column]) }}</td>
            <td v-if="resource.rowActions?.length" class="detail-actions">
              <button v-for="action in resource.rowActions" v-show="visible(action, row)" :key="action.label" class="btn btn-sm" :class="action.danger ? 'btn-danger' : ''" :disabled="busy" type="button" @click="invoke(action, row)">{{ action.label }}</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div v-else-if="!objectEntries.length" class="empty-state"><strong>No {{ (resource.title || "resource").toLowerCase() }} data yet.</strong><span>This is a real empty state; Porter did not fabricate records.</span></div>
  </template>
</template>
