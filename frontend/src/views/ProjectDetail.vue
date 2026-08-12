<script setup>
import { ref, onMounted, onUnmounted, computed } from "vue";
import { useRouter } from "vue-router";
import { api } from "../api/client";
import { connectEvents, disconnectEvents } from "../api/events";
import StatusBadge from "../components/StatusBadge.vue";
import HealthPill from "../components/HealthPill.vue";
import ScaleModal from "../components/ScaleModal.vue";
import AddDomainModal from "../components/AddDomainModal.vue";
import ProjectAnalytics from "../components/ProjectAnalytics.vue";
import ProjectFirewall from "../components/ProjectFirewall.vue";
import ProjectCron from "../components/ProjectCron.vue";
import ProjectSecrets from "../components/ProjectSecrets.vue";
import ProjectSettings from "../components/ProjectSettings.vue";

const props = defineProps({ id: { type: String, required: true } });
const router = useRouter();

const project = ref(null);
const vms = ref([]);
const deployments = ref([]);
const traffic = ref([]);
const logs = ref([]);
const domains = ref([]);
const envVars = ref([]);
const error = ref("");
const tab = ref("overview");
const showScale = ref(false);
const showDomain = ref(false);
const newEnv = ref({ key: "", value: "" });

const TABS = ["overview", "deployments", "analytics", "traffic", "logs", "domains", "environment", "secrets", "crons", "firewall", "settings"];

async function load() {
  error.value = "";
  try {
    const data = await api(`/projects/${props.id}?expand=vms`);
    project.value = data.project || data;
    vms.value = data.vms || [];
    await Promise.allSettled([
      loadDeployments(), loadTraffic(), loadLogs(), loadDomains(), loadEnv(),
    ]);
  } catch (e) {
    error.value = e.message;
  }
}

async function loadDeployments() {
  deployments.value = (await api(`/projects/${props.id}/deployments`)) || [];
}
async function loadTraffic() {
  const t = await api(`/projects/${props.id}/traffic`);
  traffic.value = Array.isArray(t) ? t : [];
}
async function loadLogs() {
  const l = await api(`/projects/${props.id}/logs`);
  logs.value = Array.isArray(l) ? l : (l && l.logs) || [];
}
async function loadDomains() {
  const d = await api(`/projects/${props.id}/domains`);
  domains.value = Array.isArray(d) ? d : [];
}
async function loadEnv() {
  const e = await api(`/projects/${props.id}/env`);
  envVars.value = Array.isArray(e) ? e : [];
}

async function replicaAct(v, kind) {
  await api(`/vms/${v.id}/${kind}`, { method: "POST" });
  load();
}

async function redeploy() {
  await api(`/projects/${props.id}/redeploy`, { method: "POST" });
  load();
}

async function addEnv() {
  if (!newEnv.value.key.trim()) return;
  await api(`/projects/${props.id}/env`, {
    method: "POST",
    body: JSON.stringify(newEnv.value),
  });
  newEnv.value = { key: "", value: "" };
  loadEnv();
}

async function removeDomain(d) {
  await api(`/projects/${props.id}/domains/${d.id}`, { method: "DELETE" });
  loadDomains();
}

async function deleteProject() {
  if (!confirm(`Delete project "${project.value.name}" and all its replicas?`)) return;
  await api(`/projects/${props.id}`, { method: "DELETE" });
  router.push({ name: "list" });
}

async function promote(d) {
  if (!confirm(`Promote deployment #${d.revision ?? d.id.slice(0, 8)} to 100%?`)) return;
  try {
    await api(`/projects/${props.id}/deployments/${d.id}/promote`, { method: "POST" });
    loadDeployments();
  } catch (e) {
    alert(e.message);
  }
}

async function rollback(d) {
  if (!confirm(`Roll back deployment #${d.revision ?? d.id.slice(0, 8)} to its previous release?`)) return;
  try {
    await api(`/projects/${props.id}/deployments/${d.id}/rollback`, { method: "POST" });
    load();
  } catch (e) {
    alert(e.message);
  }
}

onMounted(() => {
  load();
  connectEvents(() => load());
});
onUnmounted(() => disconnectEvents());
</script>

<template>
  <a class="back-link" @click="router.push({ name: 'list' })">&larr; Deployments</a>

  <div v-if="error" class="error-box">{{ error }}</div>

  <template v-else-if="project">
    <div class="detail-header">
      <div class="detail-title">
        {{ project.name }}
        <span class="tag tag-accent" style="vertical-align:middle">{{ project.kind || 'image' }}</span>
      </div>
      <div class="page-sub">
        Created {{ new Date(project.created_at).toLocaleString() }} &middot; network {{ project.network }}
        &middot; <StatusBadge :state="project.status || 'pending'" />
      </div>
      <div class="detail-actions">
        <button class="btn btn-sm" @click="redeploy">Redeploy</button>
        <button class="btn btn-sm" @click="showScale = true">Scale</button>
        <button class="btn btn-danger btn-sm" @click="deleteProject">Delete Project</button>
      </div>
    </div>

    <div class="seg" style="margin-bottom:18px">
      <button v-for="t in TABS" :key="t" :class="{ active: tab === t }" @click="tab = t">{{ t }}</button>
    </div>

    <!-- Overview -->
    <div v-if="tab === 'overview'">
      <div class="table-wrap">
        <table class="data-table">
          <thead>
            <tr><th>Replica</th><th>IP</th><th>Status</th><th>Health</th><th>Image</th><th style="text-align:right">Actions</th></tr>
          </thead>
          <tbody>
            <tr v-for="v in vms" :key="v.id" style="cursor:pointer" @click="router.push({ name: 'vm', params: { id: v.id } })">
              <td class="mono">{{ v.name }}</td>
              <td class="mono">{{ v.ip_address || '—' }}</td>
              <td><StatusBadge :state="v.state" /></td>
              <td><HealthPill :health="v.health_status" /></td>
              <td class="mono">{{ v.image }}</td>
              <td style="text-align:right">
                <div class="actions" @click.stop>
                  <button class="icon-btn green" title="Start" @click="replicaAct(v, 'start')">▶</button>
                  <button class="icon-btn" title="Stop" @click="replicaAct(v, 'stop')">■</button>
                  <button class="icon-btn" title="Restart" @click="replicaAct(v, 'restart')">↻</button>
                  <button class="icon-btn" title="SSH" @click="router.push({ name: 'vm', params: { id: v.id }, query: { tab: 'ssh' } })">⇄</button>
                </div>
              </td>
            </tr>
            <tr v-if="!vms.length"><td colspan="6" class="hint" style="text-align:center; padding:18px">No replicas yet — deploy or scale up.</td></tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Deployments -->
    <div v-if="tab === 'deployments'">
      <div class="filter-bar">
        <button class="btn btn-sm" @click="loadDeployments">Refresh</button>
        <span class="hint">{{ deployments.length }} deployment(s)</span>
      </div>
      <div class="table-wrap">
        <table class="data-table">
          <thead><tr><th>Revision</th><th>Status</th><th>Image</th><th>Rollout</th><th>Created</th><th style="text-align:right">Actions</th></tr></thead>
          <tbody>
            <tr v-for="d in deployments" :key="d.id">
              <td class="num">#{{ d.revision ?? d.id.slice(0, 8) }}</td>
              <td><StatusBadge :state="d.build_status || 'ready'" /></td>
              <td class="mono">{{ d.image_digest || d.image || '—' }}</td>
              <td class="num">{{ d.rollout_percent != null ? d.rollout_percent + '%' : '—' }}</td>
              <td class="num muted">{{ new Date(d.created_at).toLocaleString() }}</td>
              <td style="text-align:right">
                <div class="actions">
                  <button class="icon-btn green" title="Promote" @click="promote(d)">▲</button>
                  <button class="icon-btn" title="Rollback" @click="rollback(d)">↩</button>
                </div>
              </td>
            </tr>
            <tr v-if="!deployments.length"><td colspan="6" class="hint" style="text-align:center; padding:18px">No deployments recorded.</td></tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Analytics -->
    <div v-if="tab === 'analytics'">
      <ProjectAnalytics :project-id="id" />
    </div>

    <!-- Secrets -->
    <div v-if="tab === 'secrets'">
      <ProjectSecrets :project-id="id" />
    </div>

    <!-- Cron -->
    <div v-if="tab === 'crons'">
      <ProjectCron :project-id="id" />
    </div>

    <!-- Firewall -->
    <div v-if="tab === 'firewall'">
      <ProjectFirewall :project-id="id" />
    </div>

    <!-- Traffic -->
    <div v-if="tab === 'traffic'">
      <div class="table-wrap">
        <table class="data-table">
          <thead><tr><th>Time</th><th>Method</th><th>Host</th><th>Path</th><th>Status</th><th>ms</th></tr></thead>
          <tbody>
            <tr v-for="(t, i) in traffic" :key="i">
              <td class="num muted">{{ new Date(t.timestamp).toLocaleTimeString() }}</td>
              <td class="mono">{{ t.method }}</td>
              <td class="mono">{{ t.host }}</td>
              <td class="mono">{{ t.path }}</td>
              <td><span class="num" :style="{ color: t.status < 400 ? 'var(--green)' : 'var(--red)' }">{{ t.status }}</span></td>
              <td class="num">{{ t.duration_ms }}</td>
            </tr>
            <tr v-if="!traffic.length"><td colspan="6" class="hint" style="text-align:center; padding:18px">No traffic yet — requests through the gateway appear here.</td></tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Logs -->
    <div v-if="tab === 'logs'">
      <div class="terminal">
        <div v-for="(l, i) in logs" :key="i" class="tline">{{ l }}</div>
        <div v-if="!logs.length" class="t-empty">No log output yet.</div>
      </div>
    </div>

    <!-- Domains -->
    <div v-if="tab === 'domains'">
      <div class="filter-bar"><button class="btn btn-sm" @click="showDomain = true">+ Add Domain</button></div>
      <div class="table-wrap">
        <table class="data-table">
          <thead><tr><th>Domain</th><th>Status</th><th></th></tr></thead>
          <tbody>
            <tr v-for="d in domains" :key="d.id || d.domain">
              <td class="mono">{{ d.domain }}</td>
              <td><span class="tag" :class="d.status === 'active' ? 'tag-green' : 'tag-amber'">{{ d.status || d.type || 'pending' }}</span></td>
              <td style="text-align:right">
                <button class="icon-btn danger" title="Remove" @click="removeDomain(d)">✕</button>
              </td>
            </tr>
            <tr v-if="!domains.length"><td colspan="3" class="hint" style="text-align:center; padding:18px">No domains.</td></tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Environment -->
    <div v-if="tab === 'environment'">
      <div class="card" style="margin-bottom:16px">
        <div class="field-row">
          <div class="field"><label>Key</label><input v-model="newEnv.key" placeholder="DATABASE_URL" /></div>
          <div class="field"><label>Value</label><input v-model="newEnv.value" placeholder="postgres://…" /></div>
          <div style="align-self:flex-end"><button class="btn btn-primary btn-sm" @click="addEnv">Add</button></div>
        </div>
      </div>
      <div class="table-wrap">
        <table class="data-table">
          <thead><tr><th>Key</th><th>Value</th></tr></thead>
          <tbody>
            <tr v-for="e in envVars" :key="e.key">
              <td class="mono">{{ e.key }}</td>
              <td class="mono muted">{{ e.value }}</td>
            </tr>
            <tr v-if="!envVars.length"><td colspan="2" class="hint" style="text-align:center; padding:18px">No environment variables.</td></tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Settings -->
    <div v-if="tab === 'settings'">
      <div class="card" style="margin-bottom:16px">
        <div class="page-sub" style="margin-bottom:12px">
          Restart policy: <b>{{ project.restart_policy || 'on-failure' }}</b>
          &middot; Desired replicas: <b>{{ project.replicas_desired }}</b>
        </div>
        <ProjectSettings :project-id="id" />
      </div>
      <div class="card">
        <div class="page-sub">Danger zone — permanently stop and remove this project and all replicas.</div>
        <div style="margin-top:12px">
          <button class="btn btn-danger btn-sm" @click="deleteProject">Delete Project</button>
        </div>
      </div>
    </div>
  </template>

  <ScaleModal v-if="showScale" :project-id="id" :service="project?.name || 'web'" :current="project?.replicas_desired || 1" @close="showScale = false" @applied="load" />
  <AddDomainModal v-if="showDomain" :project-id="id" @close="showDomain = false" @added="() => { showDomain = false; loadDomains(); }" />
</template>
