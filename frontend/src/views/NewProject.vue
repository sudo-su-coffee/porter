<!-- Porter dashboard style: an operational creation flow that accepts only real direct artifacts or explicit source boundaries. -->
<script setup>
import { ref } from "vue";
import { useRouter } from "vue-router";
import { api } from "../api/client";
import { toast } from "../components/toast";

const router = useRouter();
const mode = ref("image");
const busy = ref(false);
const error = ref("");
const imageForm = ref({ name: "", image: "", replicas: 1, restart_policy: "on-failure", ssh_enabled: false, git_url: "", branch: "main" });
const composeForm = ref({ name: "", compose_yaml: "" });
const composeValidation = ref(null);

async function createImageProject() {
  if (!imageForm.value.name.trim() || (!imageForm.value.image.trim() && !imageForm.value.git_url.trim())) { error.value = "Name and a registered direct image or Git URL are required."; return; }
  busy.value = true; error.value = "";
  try {
    const result = await api("/projects", { method: "POST", body: JSON.stringify({ ...imageForm.value, name: imageForm.value.name.trim(), image: imageForm.value.image.trim(), git_url: imageForm.value.git_url.trim() }) });
    toast("Project creation accepted", "success");
    if (result?.project?.id) router.push({ name: "project", params: { id: result.project.id } }); else router.push({ name: "list" });
  } catch (err) { error.value = err.message; } finally { busy.value = false; }
}

async function validateCompose() {
  if (!composeForm.value.compose_yaml.trim()) { error.value = "Compose YAML is required."; return; }
  try { composeValidation.value = await api("/projects/compose/validate", { method: "POST", body: JSON.stringify({ compose_yaml: composeForm.value.compose_yaml }) }); toast("Compose parsed successfully", "success"); }
  catch (err) { composeValidation.value = { valid: false, error: err.message }; }
}

async function createComposeProject() {
  if (!composeForm.value.compose_yaml.trim()) { error.value = "Compose YAML is required."; return; }
  busy.value = true; error.value = "";
  try {
    const result = await api("/projects/compose", { method: "POST", body: JSON.stringify(composeForm.value) });
    toast("Compose stack created", "success");
    const first = result?.projects?.[0];
    if (first?.id) router.push({ name: "project", params: { id: first.id } }); else router.push({ name: "list" });
  } catch (err) { error.value = err.message; } finally { busy.value = false; }
}
</script>

<template>
  <a class="back-link" @click="router.push({ name: 'list' })">&larr; Deployments</a>
  <header class="page-header"><div><div class="page-title">New project</div><div class="page-sub">Create a project from a registered direct Firecracker artifact, a Git repository that contains one, or a validated Compose stack.</div></div></header>
  <div v-if="error" class="error-box">{{ error }}</div>
  <div class="seg stream-tabs"><button :class="{ active: mode === 'image' }" @click="mode = 'image'">Direct image / Git</button><button :class="{ active: mode === 'compose' }" @click="mode = 'compose'">Compose stack</button></div>

  <section v-if="mode === 'image'" class="card"><div class="card-title">Project source</div><p class="page-sub">Docker/OCI references are not bootable microVM images. Use a registered <span class="mono">base://</span> or <span class="mono">custom://</span> manifest, or a Git repository with validated <span class="mono">vmlinux</span> and <span class="mono">rootfs.ext4</span>.</p><div class="field-row"><div class="field"><label>Name</label><input v-model="imageForm.name" placeholder="checkout-api" required /></div><div class="field"><label>Registered direct image (optional when Git URL is set)</label><input v-model="imageForm.image" placeholder="base://porter-default" /></div></div><div class="field-row"><div class="field"><label>Git URL (optional)</label><input v-model="imageForm.git_url" placeholder="https://github.com/org/repo.git" /></div><div class="field"><label>Branch</label><input v-model="imageForm.branch" placeholder="main" /></div></div><div class="field-row"><div class="field"><label>Replicas</label><input v-model.number="imageForm.replicas" type="number" min="1" /></div><div class="field"><label>Restart policy</label><select v-model="imageForm.restart_policy"><option value="on-failure">On failure</option><option value="always">Always</option><option value="never">Never</option></select></div></div><div class="field"><label>SSH gateway metadata</label><label class="toggle"><input v-model="imageForm.ssh_enabled" type="checkbox" /><span></span></label></div><button class="btn btn-primary" :disabled="busy" @click="createImageProject">{{ busy ? 'Creating…' : 'Create project' }}</button></section>

  <section v-else class="card"><div class="card-title">Compose stack</div><p class="page-sub">Porter parses and persists Compose service definitions, but arbitrary Dockerfile/Compose-to-guest conversion remains a separate BuildKit and guest-conversion boundary.</p><div class="field"><label>Stack name</label><input v-model="composeForm.name" placeholder="storefront" /></div><div class="field"><label>Compose YAML</label><textarea v-model="composeForm.compose_yaml" rows="18" placeholder="services:\n  web:\n    image: base://validated-artifact"></textarea></div><div class="detail-actions" style="margin-top:14px"><button class="btn btn-sm" :disabled="busy" @click="validateCompose">Validate Compose</button><button class="btn btn-primary btn-sm" :disabled="busy" @click="createComposeProject">Create stack</button></div><div v-if="composeValidation" class="card" style="margin-top:16px"><div :class="composeValidation.valid ? 'surface-good' : 'surface-warn'">{{ composeValidation.valid ? `Valid services: ${(composeValidation.services || []).join(', ') || 'none'}` : composeValidation.error }}</div></div></section>
</template>
