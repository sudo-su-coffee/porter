<script setup>
import { ref } from "vue";
import { api } from "../api/client";

const props = defineProps({
  projectId: { type: String, required: true },
  service: { type: String, required: true },
  current: { type: Number, required: true },
});
const emit = defineEmits(["close", "applied"]);

const replicas = ref(props.current);
const error = ref("");

function onOverlayClick(e) {
  if (e.target === e.currentTarget) emit("close");
}

async function apply() {
  error.value = "";
  try {
    await api(`/projects/${props.projectId}/services/${props.service}/scale`, {
      method: "PATCH",
      body: JSON.stringify({ replicas: parseInt(replicas.value, 10) }),
    });
    emit("applied");
  } catch (e) {
    error.value = e.message;
  }
}
</script>

<template>
  <div class="modal-overlay" @click="onOverlayClick">
    <div class="modal" style="width:340px">
      <div class="modal-title">Scale "{{ service }}"</div>
      <div v-if="error" class="error-box">{{ error }}</div>
      <div class="field"><label>Replica count</label><input v-model="replicas" type="number" min="0" /></div>
      <div class="modal-footer">
        <button class="btn" @click="emit('close')">Cancel</button>
        <button class="btn btn-primary" @click="apply">Apply</button>
      </div>
    </div>
  </div>
</template>
