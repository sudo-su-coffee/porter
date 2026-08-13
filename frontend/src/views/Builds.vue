<!-- Harbor Glass / Whatomate-inspired Porter workspace: source-first build operations with explicit provenance and honest artifact readiness. -->
<script setup>
import { onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api } from "../api/client";
import { toast } from "../components/toast";

const route = useRoute();
const router = useRouter();
const builds = ref([]);
const branches = ref([]);
const loading = ref(true);
const error = ref("");
const busy = ref(false);
const form = ref({ git_url: "", branch: "main" });
const projectId = () => route.params.projectId;

async function load() {
  loading.value = true;
  error.value = "";
  try {
    const [b, refs] = await Promise.all([api(`/projects/${projectId()}/builds`), api(`/projects/${projectId()}/git/branches`)]);
    builds.value = Array.isArray(b) ? b : b?.builds || [];
    branches.value = Array.isArray(refs) ? refs : refs?.branches || [];
  } catch (e) {
    error.value = e.message;
  } finally {
    loading.value = false;
  }
}

async function startBuild() {
  if (!form.value.git_url.trim()) return;
  busy.value = true;
  try {
    await api(`/projects/${projectId()}/builds`, { method: "POST", body: JSON.stringify({ git_url: form.value.git_url.trim(), branch: form.value.branch || "main" }) });
    toast("Source build queued", "success");
    await load();
  } catch (e) {
    toast(e.message, "error");
  } finally {
    busy.value = false;
  }
}

async function runBuild() {
  if (!form.value.git_url.trim()) return;
  busy.value = true;
  try {
    await api(`/projects/${projectId()}/builds/run`, { method: "POST", body: JSON.stringify({ git_url: form.value.git_url.trim(), branch: form.value.branch || "main" }) });
    toast("Build run requested", "success");
    await load();
  } catch (e) { toast(e.message, "error"); }
  finally { busy.value = false; }
}

async function importSource() {
  if (!form.value.git_url.trim()) return;
  busy.value = true;
  try {
    await api(`/projects/${projectId()}/git/import`, { method: "POST", body: JSON.stringify({ git_url: form.value.git_url.trim(), branch: form.value.branch || "main" }) });
    toast("Git import queued", "success");
    await load();
  } catch (e) { toast(e.message, "error"); }
  finally { busy.value = false; }
}

async function deployFromGit() {
  if (!form.value.git_url.trim()) return;
  busy.value = true;
  try {
    await api(`/projects/${projectId()}/deployments/git`, { method: "POST", body: JSON.stringify({ repository: form.value.git_url.trim(), branch: form.value.branch || "main" }) });
    toast("Git deployment queued", "success");
    await load();
  } catch (e) { toast(e.message, "error"); }
  finally { busy.value = false; }
}

onMounted(load);
</script>

<template>
  <a class="back-link" @click="router.push({ name: 'project', params: { id: projectId() } })">&larr; Project workspace</a>
  <header class="page-header"><div><div class="page-title">Builds</div><div class="page-sub">GitHub source provenance, build status, and live logs for this project.</div></div><button class="btn btn-sm" @click="load">Refresh</button></header>
  <div v-if="error" class="error-box">{{ error }}</div>
  <section class="card build-source-card">
    <div class="card-title">Start a source build</div>
    <div class="page-sub">The current direct-only builder accepts repositories that contain a verified <span class="mono">vmlinux</span> and <span class="mono">rootfs.ext4</span> at the root or under <span class="mono">.porter/</span>. Dockerfile-to-guest conversion is not reported as ready until its BuildKit and guest-conversion workers are configured.</div>
    <div class="filter-bar"><input v-model="form.git_url" placeholder="https://github.com/owner/repository.git" /><select v-model="form.branch"><option v-for="branch in (branches.length ? branches : ['main'])" :key="branch" :value="branch">{{ branch }}</option></select><button class="btn btn-sm btn-primary" :disabled="busy" @click="startBuild">{{ busy ? 'Queueing…' : 'Build source' }}</button><button class="btn btn-sm" :disabled="busy" @click="runBuild">Run build</button><button class="btn btn-sm" :disabled="busy" @click="importSource">Import source</button><button class="btn btn-sm" :disabled="busy" @click="deployFromGit">Deploy from Git</button></div>
  </section>
  <p v-if="loading" class="page-sub">Loading build history…</p>
  <div v-else class="table-wrap"><table class="data-table"><thead><tr><th>Status</th><th>Source</th><th>Branch</th><th>Image/artifact</th><th>Actions</th></tr></thead><tbody><tr v-for="build in builds" :key="build.id"><td><span class="tag" :class="build.build_status === 'ready' ? 'tag-green' : build.build_status === 'failed' ? 'tag-red' : 'tag-accent'">{{ build.build_status }}</span></td><td class="mono">{{ build.git_url || '—' }}</td><td class="mono">{{ build.branch || 'main' }}</td><td class="mono">{{ build.image || 'pending' }}</td><td><button class="btn btn-sm" @click="router.push({ name: 'build-logs', params: { projectId: projectId(), buildId: build.id } })">Live logs</button></td></tr><tr v-if="!builds.length"><td colspan="5" class="hint" style="text-align:center;padding:20px">No source builds yet.</td></tr></tbody></table></div>
</template>
