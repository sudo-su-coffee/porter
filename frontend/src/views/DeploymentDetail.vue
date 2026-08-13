<script setup>
import { computed, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api } from "../api/client";
import { toast } from "../components/toast";

const route = useRoute();
const router = useRouter();
const projectId = computed(() => route.params.projectId);
const deploymentId = computed(() => route.params.deploymentId);
const deployment = ref(null);
const checks = ref(null);
const source = ref(null);
const logs = ref([]);
const uploadInfo = ref(null);
const error = ref("");
const loading = ref(true);
const actionBusy = ref(false);

const checkList = computed(() => checks.value?.checks || deployment.value?.checks || []);
const passedChecks = computed(() => checkList.value.filter((c) => c.status === "passed").length);
const checkState = computed(() => checks.value?.all_passed ? "passed" : checkList.value.some((c) => c.status === "failed") ? "failed" : checkList.value.length ? "pending" : "not configured");

async function load() {
  loading.value = true;
  error.value = "";
  try {
    deployment.value = await api(`/projects/${projectId.value}/deployments/${deploymentId.value}`);
    const [c, s, l, u] = await Promise.allSettled([
      api(`/projects/${projectId.value}/deployments/${deploymentId.value}/checks`),
      api(`/projects/${projectId.value}/deployments/${deploymentId.value}/source`),
      api(`/projects/${projectId.value}/deployments/${deploymentId.value}/logs`),
      api(`/projects/${projectId.value}/deployments/upload?image=${encodeURIComponent(deployment.value?.image_digest || "")}`),
    ]);
    checks.value = c.status === "fulfilled" ? c.value : null;
    source.value = s.status === "fulfilled" ? s.value : null;
    logs.value = l.status === "fulfilled" ? l.value?.logs || [] : [];
    uploadInfo.value = u.status === "fulfilled" ? u.value : null;
  } catch (e) {
    error.value = e.message;
  } finally {
    loading.value = false;
  }
}

async function setCheck(check, status) {
  try { await api(`/projects/${projectId.value}/deployments/${deploymentId.value}/checks/${encodeURIComponent(check.name)}`, { method: "PATCH", body: JSON.stringify({ status, detail: status === "passed" ? "Marked from dashboard" : "Marked for operator review" }) }); toast(`Check ${check.name} set to ${status}`, "success"); await load(); }
  catch (err) { toast(err.message, "error"); }
}

async function replaceChecks() {
  const value = prompt("Required checks as JSON array", JSON.stringify(checkList.value));
  if (value === null) return;
  let next;
  try { next = JSON.parse(value); } catch (_) { toast("Checks must be valid JSON.", "error"); return; }
  if (!Array.isArray(next)) { toast("Checks must be a JSON array.", "error"); return; }
  try {
    await api(`/projects/${projectId.value}/deployments/${deploymentId.value}/checks`, { method: "PUT", body: JSON.stringify({ checks: next }) });
    toast("Required checks replaced", "success");
    await load();
  } catch (err) { toast(err.message, "error"); }
}

async function setRollout() {
  const value = prompt("Rollout percentage (0-100)", String(deployment.value?.rollout_percent ?? 0));
  if (value === null) return;
  const percent = Number(value);
  if (!Number.isInteger(percent) || percent < 0 || percent > 100) { toast("Rollout must be an integer from 0 to 100", "error"); return; }
  try { await api(`/projects/${projectId.value}/deployments/${deploymentId.value}/rollout`, { method: "PUT", body: JSON.stringify({ percent }) }); toast("Rollout updated", "success"); await load(); }
  catch (err) { toast(err.message, "error"); }
}

async function act(action, body) {
  actionBusy.value = true;
  try {
    await api(`/projects/${projectId.value}/deployments/${deploymentId.value}/${action}`, {
      method: "POST",
      ...(body ? { body: JSON.stringify(body) } : {}),
    });
    toast(action === "promote" ? "Deployment promoted" : "Rollback requested", "success");
    await load();
  } catch (e) {
    toast(e.message, "error");
  } finally {
    actionBusy.value = false;
  }
}

function formatDate(value) {
  return value ? new Date(value).toLocaleString() : "—";
}

onMounted(load);
</script>

<template>
  <a class="back-link" @click="router.push({ name: 'project', params: { id: projectId } })">&larr; Project workspace</a>

  <div v-if="error" class="error-box">{{ error }}</div>
  <p v-if="loading && !deployment" class="page-sub">Loading deployment detail…</p>

  <template v-if="deployment">
    <div class="detail-header">
      <div>
        <div class="detail-title">Deployment #{{ deployment.revision }} <span class="tag" :class="deployment.build_status === 'live' ? 'tag-green' : deployment.build_status === 'failed' ? 'tag-red' : 'tag-accent'">{{ deployment.build_status }}</span></div>
        <div class="page-sub mono">{{ deployment.id }} · created {{ formatDate(deployment.created_at) }}</div>
      </div>
      <div class="detail-actions">
        <button class="btn btn-sm btn-primary" :disabled="actionBusy || checkState !== 'passed'" @click="act('promote')">Promote</button>
        <button class="btn btn-sm" :disabled="actionBusy" @click="act('rollback')">Rollback</button>
        <button class="btn btn-sm" :disabled="actionBusy" @click="setRollout">Set rollout</button>
      </div>
    </div>

    <section class="deployment-detail-grid">
      <div class="card deployment-detail-card deployment-detail-wide">
        <div class="card-head"><div class="card-title">Release summary</div><span class="tag tag-accent">direct runtime</span></div>
        <div class="deployment-summary-grid">
          <div><span class="stat-label">Image digest / reference</span><strong class="mono">{{ deployment.image_digest || '—' }}</strong></div>
          <div><span class="stat-label">Rollout</span><strong>{{ deployment.rollout_percent ?? 0 }}%</strong></div>
          <div><span class="stat-label">Source</span><strong>{{ source?.source || (deployment.git_url ? 'git' : 'image') }}</strong></div>
          <div><span class="stat-label">Commit</span><strong class="mono">{{ source?.commit || deployment.git_commit || '—' }}</strong></div>
        </div>
      </div>

      <div class="card deployment-detail-card">
        <div class="card-head"><div class="card-title">Required checks</div><div class="detail-actions"><span class="tag" :class="checkState === 'passed' ? 'tag-green' : checkState === 'failed' ? 'tag-red' : 'tag-amber'">{{ checkState }}</span><button class="btn btn-sm" @click="replaceChecks">Replace all</button></div></div>
        <div class="deployment-check-count">{{ passedChecks }}/{{ checkList.length || 0 }} <span class="hint">passed</span></div>
        <div v-for="check in checkList" :key="check.name" class="deployment-check-row"><span class="runtime-check-icon" :class="check.status === 'passed' ? 'check-ok' : check.status === 'failed' ? 'check-fail' : ''">{{ check.status === 'passed' ? '✓' : check.status === 'failed' ? '!' : '·' }}</span><div><div class="runtime-check-name">{{ check.name }}</div><div class="hint">{{ check.detail || check.status }}</div></div><div class="detail-actions"><button class="btn btn-sm" @click="setCheck(check, 'passed')">Pass</button><button class="btn btn-sm" @click="setCheck(check, 'failed')">Fail</button><button class="btn btn-sm" @click="setCheck(check, 'running')">Run</button></div></div>
        <div v-if="!checkList.length" class="hint" style="margin-top:12px">No promotion checks configured for this release.</div>
      </div>

      <div class="card deployment-detail-card">
        <div class="card-head"><div class="card-title">Source</div><span class="tag tag-accent">{{ source?.source || 'image' }}</span></div>
        <div class="deployment-source-line"><span class="hint">Git URL</span><span class="mono">{{ source?.git_url || deployment.git_url || 'Direct image artifact' }}</span></div>
        <div class="deployment-source-line"><span class="hint">Commit</span><span class="mono">{{ source?.commit || deployment.git_commit || deployment.image_digest || '—' }}</span></div>
        <div class="deployment-source-line"><span class="hint">Rollback target</span><span class="mono">{{ deployment.rollback_to || 'previous release' }}</span></div>
        <div class="deployment-source-line"><span class="hint">Upload contract</span><span class="mono">{{ uploadInfo?.status || 'not requested' }} {{ uploadInfo?.image || '' }}</span></div>
      </div>
    </section>

    <div class="page-sub deployment-section-label">Build and deployment logs</div>
    <div class="terminal"><div v-for="(line, i) in logs" :key="i" class="tline">{{ line }}</div><div v-if="!logs.length" class="t-empty">No deployment log output recorded.</div></div>
  </template>
</template>
