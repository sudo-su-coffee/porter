<script setup>
import { ref, onMounted } from "vue";
import { api } from "../api/client";
import { toast } from "./toast";

const props = defineProps({ projectId: { type: String, required: true } });
const error = ref("");
const secrets = ref([]);
const newSecret = ref({ name: "", value: "" });
const reveal = ref(null);

async function load() {
  error.value = "";
  try {
    const list = await api(`/projects/${props.projectId}/secrets`);
    secrets.value = Array.isArray(list) ? list : [];
  } catch (e) {
    error.value = e.message;
  }
}

async function add() {
  error.value = "";
  try {
    await api(`/projects/${props.projectId}/secrets`, {
      method: "POST",
      body: JSON.stringify(newSecret.value),
    });
    newSecret.value = { name: "", value: "" };
    load();
  } catch (e) {
    toast(e.message, "error");
  }
}

async function remove(s) {
  if (!confirm(`Delete secret "${s.name}"? This will not affect running replicas until restart.`)) return;
  await api(`/projects/${props.projectId}/secrets/${s.id}`, { method: "DELETE" });
  load();
}

onMounted(load);
</script>

<template>
  <div v-if="error" class="error-box">{{ error }}</div>

  <div class="card" style="margin-bottom:16px">
    <div class="field-row">
      <div class="field"><label>Name</label><input v-model="newSecret.name" placeholder="DATABASE_PASSWORD" /></div>
      <div class="field"><label>Value</label><input v-model="newSecret.value" type="password" placeholder="••••••" /></div>
      <div style="align-self:flex-end"><button class="btn btn-primary btn-sm" @click="add">Add</button></div>
    </div>
    <p class="hint" style="margin:8px 0 0">Values are encrypted at rest and never returned to this dashboard.</p>
  </div>

  <div class="table-wrap">
    <table class="data-table">
      <thead><tr><th>Name</th><th>Value</th><th>Created</th><th style="text-align:right">Actions</th></tr></thead>
      <tbody>
        <tr v-for="s in secrets" :key="s.id">
          <td class="mono">{{ s.name }}</td>
          <td class="mono muted">{{ reveal === s.id ? 'AES-GCM encrypted (not shown)' : '••••••' }}</td>
          <td class="num muted">{{ new Date(s.created_at).toLocaleString() }}</td>
          <td style="text-align:right">
            <div class="actions">
              <button class="icon-btn" :title="reveal === s.id ? 'Hide' : 'Reveal'" @click="reveal = reveal === s.id ? null : s.id">
                {{ reveal === s.id ? '🙈' : '👁' }}
              </button>
              <button class="icon-btn danger" title="Delete" @click="remove(s)">✕</button>
            </div>
          </td>
        </tr>
        <tr v-if="!secrets.length"><td colspan="4" class="hint" style="text-align:center; padding:18px">No secrets yet.</td></tr>
      </tbody>
    </table>
  </div>
</template>