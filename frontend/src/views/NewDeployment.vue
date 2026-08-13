<!-- Porter deployment creation surface: direct Firecracker artifact release with honest rollout controls. -->
<script setup>
import { ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api } from "../api/client";
import { toast } from "../components/toast";

const route = useRoute();
const router = useRouter();
const projectId = route.params.projectId;
const form = ref({ image: "", git_url: "", commit: "", tag: "", rollout: "immediate", traffic_pct: 100, env: "{}" });
const error = ref("");
const busy = ref(false);

async function create() {
  error.value = "";
  let env = {};
  try { env = form.value.env.trim() ? JSON.parse(form.value.env) : {}; }
  catch (_) { error.value = "Environment must be valid JSON."; return; }
  busy.value = true;
  try {
    const result = await api(`/projects/${projectId}/deployments`, { method: "POST", body: JSON.stringify({ image: form.value.image.trim(), git_url: form.value.git_url.trim(), commit: form.value.commit.trim(), tag: form.value.tag.trim(), rollout: form.value.rollout, traffic_pct: Number(form.value.traffic_pct), env }) });
    toast("Deployment created in preview state", "success");
    const deployment = result?.deployment;
    if (deployment?.id) router.push({ name: "deployment", params: { projectId, deploymentId: deployment.id } });
    else router.push({ name: "project-deployments", params: { projectId } });
  } catch (err) { error.value = err.message; }
  finally { busy.value = false; }
}
</script>

<template>
  <a class="back-link" @click="router.push({ name: 'project-deployments', params: { projectId } })">&larr; Deployment history</a>
  <header class="page-header"><div><div class="page-title">New deployment</div><div class="page-sub">Create a real preview deployment from a registered Firecracker artifact, with optional Git provenance and rollout metadata.</div></div></header>
  <section class="card"><div v-if="error" class="error-box">{{ error }}</div><div class="resource-form-grid"><div class="field field-wide"><label>Image reference</label><input v-model="form.image" placeholder="base://default or registered artifact reference" /></div><div class="field"><label>Git URL (optional)</label><input v-model="form.git_url" placeholder="https://github.com/org/repo.git" /></div><div class="field"><label>Commit (optional)</label><input v-model="form.commit" placeholder="abc123" /></div><div class="field"><label>Tag</label><input v-model="form.tag" placeholder="preview-1" /></div><div class="field"><label>Rollout</label><select v-model="form.rollout"><option value="immediate">Immediate</option><option value="canary">Canary</option><option value="bluegreen">Blue/green</option></select></div><div class="field"><label>Traffic percentage</label><input v-model.number="form.traffic_pct" type="number" min="1" max="100" /></div><div class="field field-wide"><label>Environment JSON</label><textarea v-model="form.env" rows="7" placeholder="{}" /></div></div><div class="detail-actions"><button class="btn btn-primary btn-sm" :disabled="busy" @click="create">{{ busy ? 'Creating…' : 'Create preview deployment' }}</button><button class="btn btn-sm" @click="router.back()">Cancel</button></div></section>
</template>
