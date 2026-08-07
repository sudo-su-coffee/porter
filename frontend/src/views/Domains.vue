<script setup>
import { ref, onMounted, computed } from "vue";
import { api } from "../api/client";
import AddDomainModal from "../components/AddDomainModal.vue";

const rows = ref([]);
const error = ref("");
const loading = ref(true);
const showAdd = ref(false);
const addProject = ref(null);

async function load() {
  loading.value = true;
  error.value = "";
  try {
    const projects = Array.isArray(await api("/projects")) ? await api("/projects") : [];
    const all = [];
    for (const p of projects) {
      const ds = await api(`/projects/${p.id}/domains`).catch(() => []);
      for (const d of Array.isArray(ds) ? ds : []) {
        all.push({ project_id: p.id, project: p.name, domain: d.domain, status: d.status || d.type || "pending", data: d });
      }
    }
    rows.value = all;
  } catch (e) {
    error.value = e.message;
  } finally {
    loading.value = false;
  }
}

async function verify(r) {
  await api(`/projects/${r.project_id}/domains/${r.data.id || r.domain}/verify`, { method: "POST" });
  load();
}
async function remove(r) {
  if (!confirm(`Remove domain "${r.domain}"?`)) return;
  await api(`/projects/${r.project_id}/domains/${r.data.id || r.domain}`, { method: "DELETE" });
  load();
}

function openAdd(projectId) {
  addProject.value = projectId;
  showAdd.value = true;
}

onMounted(load);
</script>

<template>
  <div class="page-header">
    <div>
      <div class="page-title">Domains</div>
      <div class="page-sub">Custom domains across all projects</div>
    </div>
  </div>

  <div v-if="error" class="error-box">{{ error }}</div>

  <div class="table-wrap">
    <table class="data-table">
      <thead><tr><th>Domain</th><th>Project</th><th>Status</th><th style="text-align:right">Actions</th></tr></thead>
      <tbody>
        <tr v-for="(r, i) in rows" :key="i">
          <td class="mono">{{ r.domain }}</td>
          <td>{{ r.project }}</td>
          <td><span class="tag" :class="r.status === 'active' ? 'tag-green' : 'tag-amber'">{{ r.status }}</span></td>
          <td style="text-align:right">
            <div class="actions">
              <button class="icon-btn" title="Verify" @click="verify(r)">✓</button>
              <button class="icon-btn danger" title="Remove" @click="remove(r)">✕</button>
            </div>
          </td>
        </tr>
        <tr v-if="!rows.length">
          <td colspan="4" class="hint" style="text-align:center; padding:18px">
            {{ loading ? 'Loading…' : 'No domains yet.' }}
          </td>
        </tr>
      </tbody>
    </table>
  </div>

  <AddDomainModal v-if="showAdd" :project-id="addProject" @close="showAdd = false" @added="() => { showAdd = false; load(); }" />
</template>
