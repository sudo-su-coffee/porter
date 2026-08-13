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
import { toast } from "../components/toast";

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
const editingEnv = ref(null);
const envDraft = ref({ value: "" });

const TABS = ["overview", "deployments", "builds", "analytics", "traffic", "logs", "domains", "environment", "secrets", "crons", "firewall", "settings"];

const runningReplicas = computed(() => vms.value.filter((v) => v.state === "running").length);
const healthyReplicas = computed(() => vms.value.filter((v) => v.health_status === "healthy").length);
const latestDeployment = computed(() => deployments.value[0] || null);
const directImage = computed(() => project.value?.image || latestDeployment.value?.image || "—");
const auxiliaryRoutes = [
  ["project-environments", "Environments", "Branches, preview domains, and deployment targets"],
  ["project-volumes", "Volumes", "Host-backed persistent storage records"],
  ["project-hooks", "Webhooks", "Delivery endpoints and manual triggers"],
  ["project-crons", "Cron jobs", "Scheduled direct-image jobs"],
  ["project-alerts", "Alerts", "Metric thresholds and silencing"],
  ["project-redirects", "Redirects", "HTTP source-to-target rules"],
  ["project-firewall", "Firewall", "Inbound/outbound policy rules"],
  ["project-networks", "Networks", "TAP and project network allocations"],
  ["project-members", "Project members", "Scoped member roles"],
  ["project-source", "Source & runtime", "Compose, scaling, healthchecks, and autoscale"],
  ["project-general-settings", "General settings", "Avatar, transfer, and project SSH controls"],
  ["project-env-vars", "Environment variables", "Runtime configuration values"],
  ["project-domains", "Project domains", "Verification and DNS records"],
  ["project-git-settings", "Git settings", "Repository and branch provenance"],
  ["project-framework", "Framework detection", "Backend-detected source framework"],
  ["project-status", "Project status", "Current control-plane lifecycle status"],
  ["project-liveness", "Replica liveness", "Current liveness signals"],
  ["project-metrics", "Project metrics", "Replica metric samples"],
  ["project-events", "Project events", "Deployment and health event records"],
  ["project-pool", "Replica pool", "Desired/current pool and drain control"],
  ["project-rollouts", "Rollouts", "Project rollout records and weights"],
  ["project-cache", "Project cache", "Cache statistics and purge operations"],
];

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

async function restartProject() {
  if (!confirm(`Restart all replicas for "${project.value?.name || props.id}"?`)) return;
  try { await api(`/projects/${props.id}/restart`, { method: "POST" }); toast("Project restart requested", "success"); await load(); }
  catch (e) { toast(e.message, "error"); }
}

async function batchReplicas(action) {
  if (!confirm(`${action === "start" ? "Start" : "Stop"} every replica in this project?`)) return;
  try { await api(`/projects/${props.id}/replicas/batch/${action}`, { method: "POST" }); toast(`Replica batch ${action} requested`, "success"); await load(); }
  catch (e) { toast(e.message, "error"); }
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

async function verifyDomain(d) {
  try {
    await api(`/projects/${props.id}/domains/${d.id}/verify`, { method: "POST" });
    toast("Domain verification requested", "success");
    await loadDomains();
  } catch (e) {
    toast(e.message, "error");
  }
}

function beginEnvEdit(entry) {
  editingEnv.value = entry.key;
  envDraft.value = { value: entry.value || "" };
}

async function saveEnvEdit(entry) {
  try {
    await api(`/projects/${props.id}/env/${encodeURIComponent(entry.key)}`, { method: "PATCH", body: JSON.stringify({ value: envDraft.value.value }) });
    editingEnv.value = null;
    toast("Environment variable updated", "success");
    await loadEnv();
  } catch (e) {
    toast(e.message, "error");
  }
}

async function removeEnv(entry) {
  if (!confirm(`Delete environment variable ${entry.key}?`)) return;
  try {
    await api(`/projects/${props.id}/env/${encodeURIComponent(entry.key)}`, { method: "DELETE" });
    toast("Environment variable deleted", "success");
    await loadEnv();
  } catch (e) {
    toast(e.message, "error");
  }
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
        <button class="btn btn-sm" @click="router.push({ name: 'new-deployment', params: { projectId: id } })">New deployment</button>
        <button class="btn btn-sm" @click="redeploy">Redeploy</button>
        <button class="btn btn-sm" @click="restartProject">Restart project</button>
        <button class="btn btn-sm" @click="showScale = true">Scale</button>
        <button class="btn btn-danger btn-sm" @click="deleteProject">Delete Project</button>
      </div>
    </div>

    <section class="project-surface">
      <div class="surface-stat surface-stat-wide"><span class="stat-label">Direct image</span><strong class="mono">{{ directImage }}</strong><span class="hint">kernel + rootfs selected by the control plane</span></div>
      <div class="surface-stat"><span class="stat-label">Replica pool</span><strong>{{ runningReplicas }}/{{ vms.length }}</strong><span class="hint">running now</span></div>
      <div class="surface-stat"><span class="stat-label">Health</span><strong :class="healthyReplicas === vms.length && vms.length ? 'surface-good' : 'surface-warn'">{{ healthyReplicas }}/{{ vms.length }}</strong><span class="hint">healthy replicas</span></div>
      <div class="surface-stat"><span class="stat-label">Latest release</span><strong>#{{ latestDeployment?.revision ?? '—' }}</strong><span class="hint">{{ latestDeployment?.build_status || 'no deployment yet' }}</span></div>
    </section>

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

      <section class="card project-resource-directory">
        <div class="card-title">Project workspace</div>
        <div class="page-sub">Open the full operational surfaces for this project. Each page reads and mutates real control-plane resources.</div>
        <div class="resource-link-grid">
          <button v-for="[name, label, description] in auxiliaryRoutes" :key="name" class="resource-link" type="button" @click="router.push({ name, params: { projectId: id } })"><strong>{{ label }}</strong><span>{{ description }}</span><span aria-hidden="true">→</span></button>
        </div>
      </section>

      <section class="card project-resource-directory">
        <div class="card-head"><div class="card-title">Replica operations</div><span class="hint">project-scoped actions</span></div>
        <div class="detail-actions"><button class="btn btn-sm btn-primary" @click="batchReplicas('start')">Start all replicas</button><button class="btn btn-sm" @click="batchReplicas('stop')">Stop all replicas</button></div>
      </section>

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
              <td><a class="back-link" @click="router.push({ name: 'deployment', params: { projectId: id, deploymentId: d.id } })">#{{ d.revision ?? d.id.slice(0, 8) }}</a></td>
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

    <!-- Builds -->
    <div v-if="tab === 'builds'" class="card">
      <div class="card-title">Source builds</div>
      <div class="page-sub">Inspect GitHub provenance, direct artifact validation, and live build output in the dedicated build workspace.</div>
      <button class="btn btn-primary" @click="router.push({ name: 'project-builds', params: { projectId: id } })">Open build workspace</button>
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
      <div class="filter-bar"><span class="hint">Historical tail from the project log store.</span><button class="btn btn-sm btn-primary" @click="router.push({ name: 'project-logs', params: { projectId: id } })">Open live stream</button></div>
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
                <button v-if="d.status !== 'active' && d.status !== 'verified'" class="btn btn-sm" @click="verifyDomain(d)">Verify</button>
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
          <thead><tr><th>Key</th><th>Value</th><th style="text-align:right">Actions</th></tr></thead>
          <tbody>
            <tr v-for="e in envVars" :key="e.key">
              <td class="mono">{{ e.key }}</td>
              <td v-if="editingEnv !== e.key" class="mono muted">{{ e.value }}</td>
              <td v-else><input v-model="envDraft.value" class="mono" /></td>
              <td style="text-align:right">
                <template v-if="editingEnv === e.key"><button class="btn btn-sm btn-primary" @click="saveEnvEdit(e)">Save</button><button class="btn btn-sm" @click="editingEnv = null">Cancel</button></template>
                <template v-else><button class="btn btn-sm" @click="beginEnvEdit(e)">Edit</button><button class="icon-btn danger" title="Delete" @click="removeEnv(e)">✕</button></template>
              </td>
            </tr>
            <tr v-if="!envVars.length"><td colspan="3" class="hint" style="text-align:center; padding:18px">No environment variables.</td></tr>
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
