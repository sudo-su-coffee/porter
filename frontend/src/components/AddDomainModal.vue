<script setup>
import { ref } from "vue";
import { api } from "../api/client";

const props = defineProps({ projectId: { type: String, required: true } });
const emit = defineEmits(["close", "added"]);

const domain = ref("");
const error = ref("");

function onOverlayClick(e) {
  if (e.target === e.currentTarget) emit("close");
}

async function add() {
  error.value = "";
  try {
    await api(`/projects/${props.projectId}/domains`, {
      method: "POST",
      body: JSON.stringify({ domain: domain.value.trim() }),
    });
    emit("added");
  } catch (e) {
    error.value = e.message;
  }
}
</script>

<template>
  <div class="modal-overlay" @click="onOverlayClick">
    <div class="modal" style="width:400px">
      <div class="modal-title">Add domain</div>
      <div v-if="error" class="error-box">{{ error }}</div>
      <div class="field"><label>Domain</label><input v-model="domain" placeholder="shop.mybrand.com" /></div>
      <div class="modal-footer">
        <button class="btn" @click="emit('close')">Cancel</button>
        <button class="btn btn-primary" @click="add">Add</button>
      </div>
    </div>
  </div>
</template>
