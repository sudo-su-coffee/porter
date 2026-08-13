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
    const [d, c, s, l] = await Promise.allSettled([
      api(`/projects/${projectId.value}/deployments/${deploymentId.value}`),
      api(`/projects/${projectId.value}/deployments/${deploymentId.value}/checks`),
      api(`/projects/${projectId.value}/deployments/${deploymentId.value}/source`),
      api(`/projects/${projectId.value}/deployments/${deploymentId.value}/logs`),
    ]);
    if (d.status === "rejected") throw d.reason;
    deployment.value = d.value;
    checks.value = c.status === "fulfilled" ? c.value : null;
    source.value = s.status === "fulfilled" ? s.value : null;
    logs.value = l.status === "fulfilled" ? l.value?.logs || [] : [];
  } catch (e) {
    error.value = e.message;
  } finally {
    loading.value = false;
  }
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
        <div class="card-head"><div class="card-title">Required checks</div><span class="tag" :class="checkState === 'passed' ? 'tag-green' : checkState === 'failed' ? 'tag-red' : 'tag-amber'">{{ checkState }}</span></div>
        <div class="deployment-check-count">{{ passedChecks }}/{{ checkList.length || 0 }} <span class="hint">passed</span></div>
        <div v-for="check in checkList" :key="check.name" class="deployment-check-row"><span class="runtime-check-icon" :class="check.status === 'passed' ? 'check-ok' : check.status === 'failed' ? 'check-fail' : ''">{{ check.status === 'passed' ? '✓' : check.status === 'failed' ? '!' : '·' }}</span><div><div class="runtime-check-name">{{ check.name }}</div><div class="hint">{{ check.detail || check.status }}</div></div></div>
        <div v-if="!checkList.length" class="hint" style="margin-top:12px">No promotion checks configured for this release.</div>
      </div>

      <div class="card deployment-detail-card">
        <div class="card-head"><div class="card-title">Source</div><span class="tag tag-accent">{{ source?.source || 'image' }}</span></div>
        <div class="deployment-source-line"><span class="hint">Git URL</span><span class="mono">{{ source?.git_url || deployment.git_url || 'Direct image artifact' }}</span></div>
        <div class="deployment-source-line"><span class="hint">Commit</span><span class="mono">{{ source?.commit || deployment.git_commit || deployment.image_digest || '—' }}</span></div>
        <div class="deployment-source-line"><span class="hint">Rollback target</span><span class="mono">{{ deployment.rollback_to || 'previous release' }}</span></div>
      </div>
    </section>

    <div class="page-sub deployment-section-label">Build and deployment logs</div>
    <div class="terminal"><div v-for="(line, i) in logs" :key="i" class="tline">{{ line }}</div><div v-if="!logs.length" class="t-empty">No deployment log output recorded.</div></div>
  </template>
</template>
