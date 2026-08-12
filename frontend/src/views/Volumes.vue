<script setup>
import { ref, onMounted } from "vue";
import { api } from "../api/client";
import { toast } from "../components/toast";

const volumes = ref([]);
const loading = ref(true);
const error = ref("");
const showCreate = ref(false);
const newVolume = ref({ name: "", size_mib: 1024 });

async function load() {
  error.value = "";
  try {
    const list = await api("/volumes");
    volumes.value = Array.isArray(list) ? list : [];
  } catch (e) {
    error.value = e.message;
  } finally {
    loading.value = false;
  }
}

async function createVolume() {
  if (!newVolume.value.name.trim()) {
    toast("Volume name is required", "error");
    return;
  }
  try {
    await api("/volumes", {
      method: "POST",
      body: JSON.stringify(newVolume.value),
    });
    toast("Volume created successfully!", "success");
    newVolume.value = { name: "", size_mib: 1024 };
    showCreate.value = false;
    load();
  } catch (e) {
    toast(e.message, "error");
  }
}

async function deleteVolume(id) {
  if (!confirm("Are you sure you want to permanently delete this volume? All data inside it will be lost.")) return;
  try {
    await api(`/volumes/${id}`, { method: "DELETE" });
    toast("Volume deleted successfully", "success");
    load();
  } catch (e) {
    toast(e.message, "error");
  }
}

async function resizeVolume(v) {
  const newSize = prompt(`Enter new size in MiB for volume "${v.name}" (current size: ${v.size_mib} MiB):`, v.size_mib);
  if (newSize === null) return;
  const size = parseInt(newSize, 10);
  if (isNaN(size) || size <= 0) {
    toast("Invalid size entered", "error");
    return;
  }
  try {
    await api(`/volumes/${v.id}/resize`, {
      method: "POST",
      body: JSON.stringify({ size_mib: size }),
    });
    toast("Volume resized successfully!", "success");
    load();
  } catch (e) {
    toast(e.message, "error");
  }
}

async function refreshUsage(v) {
  try {
    const res = await api(`/volumes/${v.id}/usage`);
    toast(`Volume disk usage: ${res.usage_bytes || 0} bytes used`, "success");
    load();
  } catch (e) {
    toast(e.message, "error");
  }
}

onMounted(load);
</script>

<template>
  <header class="page-header">
    <div>
      <div class="page-title">Persistent Volumes</div>
      <div class="page-sub">Virtual block devices mountable into microVM instances for database and file persistence.</div>
    </div>
  </header>

  <div v-if="error" class="error-box">{{ error }}</div>

  <div class="filter-bar">
    <button class="btn btn-sm btn-primary" @click="showCreate = true">+ Create Volume</button>
    <span class="hint">{{ volumes.length }} volume(s)</span>
  </div>

  <p v-if="loading" class="page-sub">Loading persistent storage volumes…</p>

  <div v-else class="table-wrap">
    <table class="data-table">
      <thead>
        <tr>
          <th>Name</th>
          <th>Size</th>
          <th>Host Path</th>
          <th style="text-align:right">Actions</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="v in volumes" :key="v.id">
          <td>
            <div style="font-weight: 600;" class="mono">{{ v.name }}</div>
            <div class="hint" style="margin-top: 3px;">UUID: <span class="mono">{{ v.id }}</span></div>
          </td>
          <td class="num"><b>{{ v.size_mib }} MiB</b></td>
          <td class="mono muted" style="max-width: 300px; word-break: break-all;">{{ v.path || '—' }}</td>
          <td style="text-align:right">
            <div class="actions">
              <button class="icon-btn green" title="Refresh Disk Usage" @click="refreshUsage(v)">↻</button>
              <button class="icon-btn" title="Resize Volume" @click="resizeVolume(v)">⇳</button>
              <button class="icon-btn danger" title="Delete Volume" @click="deleteVolume(v.id)">✕</button>
            </div>
          </td>
        </tr>
        <tr v-if="!volumes.length">
          <td colspan="4" class="hint" style="text-align:center; padding:18px">
            No persistent volumes provisioned yet.
          </td>
        </tr>
      </tbody>
    </table>
  </div>

  <div class="modal-overlay" v-if="showCreate" @click.self="showCreate = false">
    <div class="modal" style="width:400px">
      <div class="modal-title">Create Persistent Volume</div>
      <div class="field">
        <label>Volume Name (slug)</label>
        <input v-model="newVolume.name" placeholder="postgres-data" />
      </div>
      <div class="field">
        <label>Size (MiB)</label>
        <input v-model.number="newVolume.size_mib" type="number" min="64" placeholder="1024" />
      </div>
      <div class="modal-footer">
        <button class="btn" @click="showCreate = false">Cancel</button>
        <button class="btn btn-primary" @click="createVolume">Create</button>
      </div>
    </div>
  </div>
</template>
