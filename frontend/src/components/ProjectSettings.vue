<script setup>
import { ref, onMounted, watch } from "vue";
import { api } from "../api/client";
import { toast } from "./toast";

const props = defineProps({ projectId: { type: String, required: true } });
const tab = ref("general");
const error = ref("");

const TABS = ["general", "build", "git", "security", "rollout"];

const general = ref({});
const build = ref({});
const git = ref({});
const security = ref({});
const rollout = ref({});
const sections = { general, build, git, security, rollout };
const SAVE_VERB = { general: "PATCH", build: "PUT", git: "PUT", security: "PUT", rollout: "PUT" };

async function loadSection(name) {
  error.value = "";
  try {
    const data = await api(`/projects/${props.projectId}/settings/${name}`);
    sections[name].value = data || {};
  } catch (e) {
    error.value = e.message;
  }
}

async function save(name) {
  error.value = "";
  try {
    const data = await api(`/projects/${props.projectId}/settings/${name}`, {
      method: SAVE_VERB[name],
      body: JSON.stringify(sections[name].value),
    });
    sections[name].value = data || {};
    toast(`${name} settings saved`, "success");
  } catch (e) {
    toast(e.message, "error");
  }
}

watch(tab, (t) => loadSection(t), { immediate: true });
onMounted(() => loadSection("general"));
</script>

<template>
  <div v-if="error" class="error-box">{{ error }}</div>

  <div class="seg" style="margin-bottom:18px">
    <button v-for="t in TABS" :key="t" :class="{ active: tab === t }" @click="tab = t">{{ t }}</button>
  </div>

  <template v-if="tab === 'general'">
    <div class="card">
      <div class="field-row">
        <div class="field"><label>Name</label><input v-model="general.name" /></div>
        <div class="field"><label>Description</label><input v-model="general.description" /></div>
      </div>
      <div class="field-row">
        <div class="field"><label>Git repository URL</label><input v-model="general.git_repo" /></div>
        <div class="field"><label>Framework</label><input v-model="general.framework" /></div>
      </div>
      <div class="field-row">
        <div class="field"><label>Install command</label><input v-model="general.install_command" placeholder="npm install" /></div>
        <div class="field"><label>Build command</label><input v-model="general.build_command" placeholder="npm run build" /></div>
      </div>
      <div class="field-row">
        <div class="field"><label>Output directory</label><input v-model="general.output_directory" placeholder="dist" /></div>
        <div class="field"><label>Dev command</label><input v-model="general.dev_command" placeholder="npm run dev" /></div>
      </div>
      <div class="modal-footer"><button class="btn btn-primary" @click="save('general')">Save</button></div>
    </div>
  </template>

  <template v-else-if="tab === 'build'">
    <div class="card">
      <div class="field-row">
        <div class="field"><label>Build command</label><input v-model="build.command" /></div>
        <div class="field"><label>Build mode</label>
          <select v-model="build.mode">
            <option value="dockerfile">Dockerfile</option>
            <option value="buildpacks">Buildpacks</option>
          </select>
        </div>
      </div>
      <div class="field"><label>Dockerfile path</label><input v-model="build.dockerfile" placeholder="Dockerfile" /></div>
      <div class="field-row">
        <div class="field"><label>CPU (vcpu)</label><input v-model.number="build.vcpus" type="number" min="1" /></div>
        <div class="field"><label>Memory (MiB)</label><input v-model.number="build.mem_mib" type="number" min="64" /></div>
      </div>
      <div class="modal-footer"><button class="btn btn-primary" @click="save('build')">Save</button></div>
    </div>
  </template>

  <template v-else-if="tab === 'git'">
    <div class="card">
      <div class="field"><label>Repository URL</label><input v-model="git.repository" /></div>
      <div class="field-row">
        <div class="field"><label>Branch</label><input v-model="git.branch" placeholder="main" /></div>
        <div class="field"><label>Root directory</label><input v-model="git.root_directory" /></div>
      </div>
      <div class="field"><label>Auto-deploy on push</label>
        <label class="toggle"><input type="checkbox" v-model="git.auto_deploy" /><span></span></label>
      </div>
      <div class="modal-footer"><button class="btn btn-primary" @click="save('git')">Save</button></div>
    </div>
  </template>

  <template v-else-if="tab === 'security'">
    <div class="card">
      <div class="field"><label>SSH access</label>
        <label class="toggle"><input type="checkbox" v-model="security.ssh_enabled" /><span></span></label>
      </div>
      <div class="field"><label>Rate limit (requests/min)</label><input v-model.number="security.rate_limit" type="number" /></div>
      <div class="field-row">
        <div class="field"><label>IP allowlist</label><input v-model="security.ip_allowlist" placeholder="203.0.113.0/24, …" /></div>
        <div class="field"><label>IP denylist</label><input v-model="security.ip_denylist" placeholder="…" /></div>
      </div>
      <div class="modal-footer"><button class="btn btn-primary" @click="save('security')">Save</button></div>
    </div>
  </template>

  <template v-else-if="tab === 'rollout'">
    <div class="card">
      <div class="field"><label>Rollout strategy</label>
        <select v-model="rollout.strategy">
          <option value="immediate">Immediate</option>
          <option value="rolling">Rolling</option>
          <option value="bluegreen">Blue/Green</option>
          <option value="canary">Canary</option>
        </select>
      </div>
      <div class="field"><label>Traffic percentage for new deployment</label>
        <input v-model.number="rollout.traffic_pct" type="number" min="0" max="100" />
      </div>
      <div class="modal-footer"><button class="btn btn-primary" @click="save('rollout')">Save</button></div>
    </div>
  </template>
</template>