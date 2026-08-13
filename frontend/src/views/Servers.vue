<!-- Porter dashboard style: dense operational tables, graphite hierarchy, teal actions, and explicit host/runtime boundaries. -->
<script setup>
import { ref, onMounted } from "vue";
import { api } from "../api/client";
import { toast } from "../components/toast";

const overview = ref(null);
const servers = ref([]);
const volumes = ref([]);
const loading = ref(true);
const showRegister = ref(false);
const newServer = ref({ name: "", address: "" });
const newVolume = ref({ name: "", size_mib: 1024 });
const serverSSH = ref(null);
const volumeUsage = ref({});

async function load() {
  try { overview.value = await api("/overview").catch(() => null); } catch (_) {}
  servers.value = await api("/servers").catch(() => []);
  volumes.value = await api("/volumes").catch(() => []);
  loading.value = false;
}

function uptime() {
  if (!overview.value || !overview.value.started_at) return "—";
  const s = new Date(overview.value.started_at).getTime();
  const secs = Math.floor((Date.now() - s) / 1000);
  const d = Math.floor(secs / 86400), h = Math.floor((secs % 86400) / 3600), m = Math.floor((secs % 3600) / 60);
  return d > 0 ? `${d}d ${h}h ${m}m` : h > 0 ? `${h}h ${m}m` : `${m}m`;
}

async function register() {
  try {
    await api("/servers", { method: "POST", body: JSON.stringify({ hostname: newServer.value.name.trim(), address: newServer.value.address.trim() }) });
    toast("server registered", "success");
    showRegister.value = false;
    newServer.value = { name: "", address: "" };
    await load();
  } catch (e) { toast(e.message, "error"); }
}

async function unregister(id) {
  if (!confirm("Remove this registered server?")) return;
  try { await api(`/servers/${id}`, { method: "DELETE" }); toast("server removed", "success"); await load(); }
  catch (e) { toast(e.message, "error"); }
}

async function heartbeat(server) {
  try { await api(`/servers/${server.id}/heartbeat`, { method: "POST" }); toast("heartbeat sent", "success"); await load(); }
  catch (e) { toast(e.message, "error"); }
}

async function showSSH(server) {
  try { serverSSH.value = { server, data: await api(`/servers/${server.id}/ssh`) }; }
  catch (e) { toast(e.message, "error"); }
}

async function createVolume() {
  if (!newVolume.value.name.trim()) { toast("volume name is required", "error"); return; }
  try {
    await api("/volumes", { method: "POST", body: JSON.stringify(newVolume.value) });
    toast("volume created", "success");
    newVolume.value = { name: "", size_mib: 1024 };
    await load();
  } catch (e) { toast(e.message, "error"); }
}

async function deleteVolume(id) {
  if (!confirm("Delete this volume?")) return;
  try { await api(`/volumes/${id}`, { method: "DELETE" }); toast("volume deleted", "success"); await load(); }
  catch (e) { toast(e.message, "error"); }
}

async function showVolumeUsage(volume) {
  try { volumeUsage.value = { ...volumeUsage.value, [volume.id]: await api(`/volumes/${volume.id}/usage`) }; }
  catch (e) { toast(e.message, "error"); }
}

async function resizeVolume(volume) {
  const value = prompt("New size in MiB", String(volume.size_mib || 1024));
  if (value === null) return;
  const size = Number(value);
  if (!Number.isFinite(size) || size <= 0) { toast("size must be a positive number", "error"); return; }
  try { await api(`/volumes/${volume.id}/resize`, { method: "POST", body: JSON.stringify({ size_mib: size }) }); toast("volume resize requested", "success"); await load(); }
  catch (e) { toast(e.message, "error"); }
}

onMounted(load);
</script>

<template>
  <header class="page-header"><div><div class="page-title">Servers</div><div class="page-sub">Host capacity, registered cluster servers, persistent volumes, and direct Firecracker runtime metadata.</div></div><button class="btn btn-sm" :disabled="loading" @click="load">Refresh</button></header>
  <p v-if="loading" class="page-sub">Loading host and storage state…</p>

  <template v-else>
    <div class="stat-grid" v-if="overview"><div class="stat-card"><div class="stat-label">Host</div><div class="stat-value">{{ overview.host }}</div><div class="stat-sub">version {{ overview.version }}</div></div><div class="stat-card"><div class="stat-label">Running</div><div class="stat-value">{{ overview.vm_running || 0 }}</div><div class="stat-sub">of {{ overview.vm_total || 0 }} VMs</div></div><div class="stat-card"><div class="stat-label">Projects</div><div class="stat-value">{{ overview.projects || 0 }}</div></div><div class="stat-card"><div class="stat-label">Images</div><div class="stat-value">{{ overview.images || 0 }}</div></div><div class="stat-card"><div class="stat-label">Uptime</div><div class="stat-value" style="font-size:16px">{{ uptime() }}</div></div></div>

    <div class="page-sub" style="margin-bottom:8px">Registered servers</div><div class="filter-bar"><button class="btn btn-sm btn-primary" @click="showRegister = true">+ Register</button></div>
    <div class="table-wrap" style="margin-bottom:22px"><table class="data-table"><thead><tr><th>Name</th><th>Address</th><th>Status</th><th style="text-align:right">Actions</th></tr></thead><tbody><tr v-for="s in servers" :key="s.id"><td class="mono">{{ s.name }}</td><td class="mono">{{ s.address }}</td><td><span class="tag" :class="s.status === 'registered' ? 'tag-green' : 'tag-amber'">{{ s.status }}</span></td><td style="text-align:right"><button class="btn btn-sm" @click="heartbeat(s)">Heartbeat</button><button class="btn btn-sm" @click="showSSH(s)">SSH info</button><button class="icon-btn danger" title="Remove" @click="unregister(s.id)">✕</button></td></tr><tr v-if="!servers.length"><td colspan="4" class="hint" style="text-align:center; padding:18px">No hosts registered yet.</td></tr></tbody></table></div>

    <div class="page-sub" style="margin-bottom:8px">Storage (optional mounts)</div><div class="filter-bar"><input v-model="newVolume.name" placeholder="mount name" style="width:160px" /><input v-model.number="newVolume.size_mib" type="number" placeholder="MiB" style="width:90px" /><button class="btn btn-sm btn-primary" @click="createVolume">+ Create mount</button></div>
    <div class="table-wrap"><table class="data-table"><thead><tr><th>Name</th><th>Size</th><th>Path</th><th style="text-align:right">Actions</th></tr></thead><tbody><tr v-for="v in volumes" :key="v.id"><td class="mono">{{ v.name }}</td><td class="num">{{ v.size_mib }} MiB</td><td class="mono muted">{{ v.path }}</td><td style="text-align:right"><button class="btn btn-sm" @click="showVolumeUsage(v)">Usage</button><button class="btn btn-sm" @click="resizeVolume(v)">Resize</button><button class="icon-btn danger" title="Delete" @click="deleteVolume(v.id)">✕</button><div v-if="volumeUsage[v.id]" class="hint mono">{{ JSON.stringify(volumeUsage[v.id]) }}</div></td></tr><tr v-if="!volumes.length"><td colspan="4" class="hint" style="text-align:center; padding:18px">No extra mounts defined yet.</td></tr></tbody></table></div>

    <section v-if="serverSSH" class="card" style="margin-top:18px"><div class="card-head"><div class="card-title">SSH metadata · {{ serverSSH.server.name }}</div><button class="btn btn-sm" @click="serverSSH = null">Close</button></div><pre class="mono settings-json">{{ JSON.stringify(serverSSH.data, null, 2) }}</pre><p class="hint">This is server SSH metadata only; Porter does not claim guest SSH execution for direct microVMs.</p></section>

    <div class="modal-overlay" v-if="showRegister" @click.self="showRegister = false"><div class="modal"><div class="modal-title">Register a host</div><div class="field"><label>Name</label><input v-model="newServer.name" placeholder="worker-01" /></div><div class="field"><label>Address</label><input v-model="newServer.address" placeholder="10.0.0.5:8080" /></div><div class="modal-footer"><button class="btn" @click="showRegister = false">Cancel</button><button class="btn btn-primary" @click="register">Register</button></div></div></div>
  </template>
</template>
