<script setup>
import { ref } from "vue";
import { api } from "../api/client";

const emit = defineEmits(["close", "created"]);

const activeTab = ref("single");
const error = ref("");

const name = ref("");
const image = ref("");
const vcpus = ref(1);
const memMib = ref(256);

const composeName = ref("");
const composeYaml = ref(`services:
  api:
    image: myapp/api:latest
    ports:
      - "3000:3000"
    deploy:
      replicas: 2
  worker:
    image: myapp/worker:latest
    depends_on:
      - api`);

function onOverlayClick(e) {
  if (e.target === e.currentTarget) emit("close");
}

async function deploy() {
  error.value = "";
  try {
    if (activeTab.value === "single") {
      if (!image.value.trim()) throw new Error('"image" is required');
      const vm = await api("/vms", {
        method: "POST",
        body: JSON.stringify({
          name: name.value.trim(),
          image: image.value.trim(),
          vcpus: parseInt(vcpus.value, 10) || 1,
          mem_mib: parseInt(memMib.value, 10) || 256,
        }),
      });
      emit("created", { name: "vm", params: { id: vm.id } });
    } else {
      if (!composeName.value.trim()) throw new Error('"name" is required');
      if (!composeYaml.value.trim()) throw new Error("compose YAML is empty");
      const proj = await api("/projects/compose", {
        method: "POST",
        body: JSON.stringify({ name: composeName.value.trim(), compose_yaml: composeYaml.value }),
      });
      emit("created", { name: "project", params: { id: proj.id } });
    }
  } catch (e) {
    error.value = e.message;
  }
}
</script>

<template>
  <div class="modal-overlay" @click="onOverlayClick">
    <div class="modal">
      <div class="modal-title">New Project</div>
      <div class="tabs">
        <button class="tab" :class="{ active: activeTab === 'single' }" @click="activeTab = 'single'">
          Single Image
        </button>
        <button class="tab" :class="{ active: activeTab === 'compose' }" @click="activeTab = 'compose'">
          docker-compose.yml
        </button>
      </div>

      <div v-if="error" class="error-box">{{ error }}</div>

      <div v-show="activeTab === 'single'">
        <div class="field"><label>Name</label><input v-model="name" placeholder="cache" /></div>
        <div class="field"><label>Image</label><input v-model="image" placeholder="redis:7" /></div>
        <div class="field-row">
          <div class="field"><label>vCPUs</label><input v-model="vcpus" type="number" min="1" /></div>
          <div class="field">
            <label>Memory (MiB)</label>
            <input v-model="memMib" type="number" min="128" step="128" />
          </div>
        </div>
      </div>

      <div v-show="activeTab === 'compose'">
        <div class="field"><label>Project name</label><input v-model="composeName" placeholder="my-app" /></div>
        <div class="field">
          <label>compose.yaml</label>
          <textarea v-model="composeYaml"></textarea>
        </div>
      </div>

      <div class="modal-footer">
        <button class="btn" @click="emit('close')">Cancel</button>
        <button class="btn btn-primary" @click="deploy">Deploy</button>
      </div>
    </div>
  </div>
</template>
