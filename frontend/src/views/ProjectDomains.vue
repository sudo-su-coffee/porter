<!-- Porter dashboard style: domain ownership and DNS state are explicit, live, and project-scoped. -->
<script setup>
import { onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api } from "../api/client";
import { toast } from "../components/toast";

const route = useRoute();
const router = useRouter();
const projectId = route.params.projectId;
const domains = ref([]);
const records = ref(null);
const domain = ref("");
const error = ref("");
const busy = ref(false);

async function load() {
  error.value = "";
  try {
    const data = await api(`/projects/${projectId}/domains`);
    domains.value = Array.isArray(data) ? data : data?.domains || [];
  } catch (err) { error.value = err.message; }
}

async function add() {
  if (!domain.value.trim()) { error.value = "Domain is required."; return; }
  busy.value = true;
  try {
    await api(`/projects/${projectId}/domains`, { method: "POST", body: JSON.stringify({ domain: domain.value.trim() }) });
    domain.value = "";
    toast("Domain added", "success");
    await load();
  } catch (err) { toast(err.message, "error"); } finally { busy.value = false; }
}

async function verify(item) {
  busy.value = true;
  try { await api(`/projects/${projectId}/domains/${encodeURIComponent(item.id || item.domain)}/verify`, { method: "POST" }); toast("Domain verification requested", "success"); await load(); }
  catch (err) { toast(err.message, "error"); } finally { busy.value = false; }
}

async function remove(item) {
  if (!confirm(`Remove domain "${item.domain}"?`)) return;
  busy.value = true;
  try { await api(`/projects/${projectId}/domains/${encodeURIComponent(item.id || item.domain)}`, { method: "DELETE" }); toast("Domain removed", "success"); await load(); }
  catch (err) { toast(err.message, "error"); } finally { busy.value = false; }
}

async function loadRecords() {
  try { records.value = await api(`/projects/${projectId}/domains/records`); }
  catch (err) { toast(err.message, "error"); }
}

onMounted(load);
</script>

<template>
  <a class="back-link" @click="router.push({ name: 'project', params: { id: projectId } })">&larr; Project workspace</a>
  <header class="page-header"><div><div class="page-title">Project domains</div><div class="page-sub">Attach domains, request verification, and inspect the DNS records returned by Porter.</div></div><div class="detail-actions"><button class="btn btn-sm" :disabled="busy" @click="load">Refresh</button><button class="btn btn-sm" :disabled="busy" @click="loadRecords">DNS records</button></div></header>
  <div v-if="error" class="error-box"><span>{{ error }}</span><button class="btn btn-sm" @click="load">Retry</button></div>
  <section class="card resource-create-card"><div class="card-title">Add domain</div><div class="field-row"><div class="field"><label>Domain</label><input v-model="domain" placeholder="app.example.com" /></div><button class="btn btn-primary btn-sm" :disabled="busy" @click="add">Add domain</button></div></section>
  <div v-if="!domains.length" class="empty-state"><strong>No project domains yet.</strong><span>Porter did not fabricate DNS records.</span></div>
  <div v-else class="table-wrap"><table class="data-table"><thead><tr><th>Domain</th><th>Status</th><th>Type</th><th>Actions</th></tr></thead><tbody><tr v-for="item in domains" :key="item.id || item.domain"><td class="mono">{{ item.domain }}</td><td><span class="tag" :class="item.status === 'active' || item.status === 'verified' ? 'tag-green' : 'tag-amber'">{{ item.status || 'pending' }}</span></td><td>{{ item.type || 'custom' }}</td><td><button v-if="item.status !== 'active' && item.status !== 'verified'" class="btn btn-sm" :disabled="busy" @click="verify(item)">Verify</button><button class="btn btn-danger btn-sm" :disabled="busy" @click="remove(item)">Remove</button></td></tr></tbody></table></div>
  <section v-if="records" class="card" style="margin-top:18px"><div class="card-title">DNS records</div><pre class="mono settings-json">{{ JSON.stringify(records, null, 2) }}</pre></section>
</template>
