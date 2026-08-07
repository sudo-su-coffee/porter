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
    await api("/servers", { method: "POST", body: JSON.stringify(newServer.value) });
    toast("server registered", "success");
    showRegister.value = false;
    newServer.value = { name: "", address: "" };
    load();
  } catch (e) { toast(e.message, "error"); }
}

async function unregister(id) {
  try {
    await api(`/servers/${id}`, { method: "DELETE" });
    toast("server removed", "success");
    load();
  } catch (e) { toast(e.message, "error"); }
}

async function createVolume() {
  try {
    await api("/volumes", { method: "POST", body: JSON.stringify(newVolume.value) });
    toast("volume created", "success");
    newVolume.value = { name: "", size_mib: 1024 };
    load();
  } catch (e) { toast(e.message, "error"); }
}

async function deleteVolume(id) {
  try {
    await api(`/volumes/${id}`, { method: "DELETE" });
    toast("volume deleted", "success");
    load();
  } catch (e) { toast(e.message, "error"); }
}

onMounted(load);
</script>

<template>
  <header class="page-header">
    <div>
      <div class="page-title">Servers</div>
      <div class="page-sub">This host, the microVMs on it, registered cluster hosts (Phase 8), and volumes (Phase 7).</div>
    </div>
  </header>

  <p v-if="loading" class="page-sub">Loading…</p>

  <template v-else>
    <div class="stat-grid" v-if="overview">
      <div class="stat-card"><div class="stat-label">Host</div><div class="stat-value">{{ overview.host }}</div><div class="stat-sub">version {{ overview.version }}</div></div>
      <div class="stat-card"><div class="stat-label">Running</div><div class="stat-value">{{ overview.vm_running || 0 }}</div><div class="stat-sub">of {{ overview.vm_total || 0 }} VMs</div></div>
      <div class="stat-card"><div class="stat-label">Projects</div><div class="stat-value">{{ overview.projects || 0 }}</div></div>
      <div class="stat-card"><div class="stat-label">Images</div><div class="stat-value">{{ overview.images || 0 }}</div></div>
      <div class="stat-card"><div class="stat-label">Uptime</div><div class="stat-value" style="font-size:16px">{{ uptime() }}</div></div>
    </div>

    <div class="page-sub" style="margin-bottom:8px">Registered servers</div>
    <div class="filter-bar">
      <button class="btn btn-sm btn-primary" @click="showRegister = true">+ Register</button>
    </div>
    <div class="table-wrap" style="margin-bottom:22px">
      <table class="data-table">
        <thead><tr><th>Name</th><th>Address</th><th>Status</th><th style="text-align:right">Actions</th></tr></thead>
        <tbody>
          <tr v-for="s in servers" :key="s.id">
            <td class="mono">{{ s.name }}</td>
            <td class="mono">{{ s.address }}</td>
            <td><span class="tag" :class="s.status === 'registered' ? 'tag-green' : 'tag-amber'">{{ s.status }}</span></td>
            <td style="text-align:right"><button class="icon-btn danger" title="Remove" @click="unregister(s.id)">✕</button></td>
          </tr>
          <tr v-if="!servers.length"><td colspan="4" class="hint" style="text-align:center; padding:18px">No hosts registered yet.</td></tr>
        </tbody>
      </table>
    </div>

    <div class="page-sub" style="margin-bottom:8px">Storage (optional mounts)</div>
    <div class="filter-bar">
      <input v-model="newVolume.name" placeholder="mount name" style="width:160px" />
      <input v-model.number="newVolume.size_mib" type="number" placeholder="MiB" style="width:90px" />
      <button class="btn btn-sm btn-primary" @click="createVolume">+ Create mount</button>
    </div>
    <div class="table-wrap">
      <table class="data-table">
        <thead><tr><th>Name</th><th>Size</th><th>Path</th><th style="text-align:right">Actions</th></tr></thead>
        <tbody>
          <tr v-for="v in volumes" :key="v.id">
            <td class="mono">{{ v.name }}</td>
            <td class="num">{{ v.size_mib }} MiB</td>
            <td class="mono muted">{{ v.path }}</td>
            <td style="text-align:right"><button class="icon-btn danger" title="Delete" @click="deleteVolume(v.id)">✕</button></td>
          </tr>
          <tr v-if="!volumes.length"><td colspan="4" class="hint" style="text-align:center; padding:18px">No extra mounts defined yet.</td></tr>
        </tbody>
      </table>
    </div>

    <div class="modal-overlay" v-if="showRegister" @click.self="showRegister = false">
      <div class="modal">
        <div class="modal-title">Register a host</div>
        <div class="field"><label>Name</label><input v-model="newServer.name" placeholder="worker-01" /></div>
        <div class="field"><label>Address</label><input v-model="newServer.address" placeholder="10.0.0.5:8080" /></div>
        <div class="modal-footer">
          <button class="btn" @click="showRegister = false">Cancel</button>
          <button class="btn btn-primary" @click="register">Register</button>
        </div>
      </div>
    </div>
  </template>
</template>