<!-- Porter dashboard style: concise operator feedback surface with persisted submissions and clear states. -->
<script setup>
import { onMounted, ref } from "vue";
import { api } from "../api/client";
import { toast } from "../components/toast";

const rows = ref([]);
const form = ref({ subject: "", category: "general", project_id: "", message: "" });
const error = ref("");
const loading = ref(true);
const busy = ref(false);

async function load() {
  loading.value = true;
  error.value = "";
  try { const data = await api("/feedback?limit=100"); rows.value = Array.isArray(data) ? data : data?.feedback || []; }
  catch (err) { error.value = err.message; }
  finally { loading.value = false; }
}

async function submit() {
  if (!form.value.message.trim()) { error.value = "Message is required."; return; }
  busy.value = true;
  try { await api("/feedback", { method: "POST", body: JSON.stringify(form.value) }); form.value = { subject: "", category: "general", project_id: "", message: "" }; toast("Feedback submitted", "success"); await load(); }
  catch (err) { toast(err.message, "error"); }
  finally { busy.value = false; }
}

onMounted(load);
</script>

<template>
  <header class="page-header"><div><div class="page-title">Feedback</div><div class="page-sub">Send an authenticated product note and review persisted submissions available to your role.</div></div><button class="btn btn-sm" :disabled="loading || busy" @click="load">Refresh</button></header>
  <div v-if="error" class="error-box"><span>{{ error }}</span><button class="btn btn-sm" @click="load">Retry</button></div>
  <section class="card resource-create-card"><div class="card-title">New feedback</div><div class="field-row"><div class="field"><label>Subject</label><input v-model="form.subject" placeholder="Deployment workflow" /></div><div class="field"><label>Category</label><select v-model="form.category"><option value="general">General</option><option value="bug">Bug</option><option value="feature">Feature request</option><option value="operations">Operations</option></select></div><div class="field"><label>Project ID (optional)</label><input v-model="form.project_id" placeholder="project-id" /></div></div><div class="field"><label>Message</label><textarea v-model="form.message" rows="4" placeholder="Describe the issue or improvement…"></textarea></div><button class="btn btn-primary btn-sm" :disabled="busy" @click="submit">Submit feedback</button></section>
  <div v-if="loading" class="page-sub">Loading persisted feedback…</div>
  <div v-else-if="!rows.length" class="empty-state"><strong>No feedback records available.</strong><span>The backend returned no persisted submissions for this account.</span></div>
  <div v-else class="table-wrap"><table class="data-table"><thead><tr><th>Created</th><th>Category</th><th>Subject</th><th>Message</th><th>User</th></tr></thead><tbody><tr v-for="row in rows" :key="row.id"><td class="num muted">{{ row.created_at ? new Date(row.created_at).toLocaleString() : '—' }}</td><td><span class="tag">{{ row.category || 'general' }}</span></td><td>{{ row.subject || '—' }}</td><td class="feedback-message">{{ row.message }}</td><td class="mono">{{ row.username || '—' }}</td></tr></tbody></table></div>
</template>
