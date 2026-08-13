<script setup>
import { ref, onMounted } from "vue";
import { api } from "../api/client";
import { toast } from "./toast";

const props = defineProps({ projectId: { type: String, required: true } });
const error = ref("");
const crons = ref([]);
const showAdd = ref(false);
const newCron = ref({ name: "", schedule: "0 * * * *", job_image: "" });
const details = ref({});

async function load() {
  error.value = "";
  try {
    const list = await api(`/projects/${props.projectId}/crons`);
    crons.value = Array.isArray(list) ? list : [];
  } catch (e) {
    error.value = e.message;
  }
}

async function add() {
  error.value = "";
  try {
    await api(`/projects/${props.projectId}/crons`, {
      method: "POST",
      body: JSON.stringify(newCron.value),
    });
    newCron.value = { name: "", schedule: "0 * * * *", job_image: "" };
    showAdd.value = false;
    load();
  } catch (e) {
    toast(e.message, "error");
  }
}

async function setActive(c, active) {
  try {
    await api(`/projects/${props.projectId}/crons/${c.id}`, {
      method: "PATCH",
      body: JSON.stringify({ active }),
    });
    load();
  } catch (e) {
    toast(e.message, "error");
  }
}

async function runNow(c) {
  try {
    await api(`/projects/${props.projectId}/crons/${c.id}/run`, { method: "POST" });
    toast("job queued", "success");
  } catch (e) {
    toast(e.message, "error");
  }
}

async function inspect(c) {
  try { details.value = { ...details.value, [c.id]: await api(`/projects/${props.projectId}/crons/${encodeURIComponent(c.id)}`) }; }
  catch (e) { toast(e.message, "error"); }
}

async function remove(c) {
  if (!confirm(`Delete cron "${c.name}"?`)) return;
  await api(`/projects/${props.projectId}/crons/${c.id}`, { method: "DELETE" });
  load();
}

onMounted(load);
</script>

<template>
  <div v-if="error" class="error-box">{{ error }}</div>

  <div class="filter-bar">
    <button class="btn btn-sm btn-primary" @click="showAdd = true">+ New job</button>
    <span class="hint">{{ crons.length }} cron job(s)</span>
  </div>

  <div class="table-wrap">
    <table class="data-table">
      <thead><tr><th>Name</th><th>Schedule</th><th>Image</th><th>Last run</th><th>Active</th><th style="text-align:right">Actions</th></tr></thead>
      <tbody>
        <tr v-for="c in crons" :key="c.id">
          <td class="mono">{{ c.name }}</td>
          <td class="mono">{{ c.schedule }}</td>
          <td class="mono">{{ c.job_image || '—' }}</td>
          <td class="num muted">{{ c.last_run_at ? new Date(c.last_run_at).toLocaleString() : 'never' }}</td>
          <td>
            <label class="toggle">
              <input type="checkbox" :checked="c.active" @change="setActive(c, $event.target.checked)" />
              <span></span>
            </label>
          </td>
          <td style="text-align:right">
            <div class="actions">
              <button class="btn btn-sm" title="Inspect cron" @click="inspect(c)">Details</button>
              <button class="icon-btn green" title="Run now" @click="runNow(c)">▶</button>
              <button class="icon-btn danger" title="Delete" @click="remove(c)">✕</button>
            </div>
            <pre v-if="details[c.id]" class="settings-json">{{ JSON.stringify(details[c.id], null, 2) }}</pre>
          </td>
        </tr>
        <tr v-if="!crons.length"><td colspan="6" class="hint" style="text-align:center; padding:18px">No cron jobs yet.</td></tr>
      </tbody>
    </table>
  </div>

  <div class="modal-overlay" v-if="showAdd" @click.self="showAdd = false">
    <div class="modal" style="width:420px">
      <div class="modal-title">New cron job</div>
      <div class="field"><label>Name</label><input v-model="newCron.name" placeholder="nightly-db-backup" /></div>
      <div class="field"><label>Schedule (5-field cron)</label><input v-model="newCron.schedule" placeholder="0 * * * *" /></div>
      <div class="field"><label>Job image</label><input v-model="newCron.job_image" placeholder="postgres:15" /></div>
      <div class="modal-footer">
        <button class="btn" @click="showAdd = false">Cancel</button>
        <button class="btn btn-primary" @click="add">Create</button>
      </div>
    </div>
  </div>
</template>
