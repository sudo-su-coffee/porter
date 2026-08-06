<script setup>
import { ref, computed } from "vue";
import { api } from "../api/client";
import StatusBadge from "./StatusBadge.vue";
import HealthPill from "./HealthPill.vue";
import ScaleModal from "./ScaleModal.vue";
import { toast } from "./toast";

const props = defineProps({
  projectId: { type: String, required: true },
  serviceName: { type: String, required: true },
  pool: { type: Object, default: () => ({}) },
  vms: { type: Array, default: () => [] },
});
const emit = defineEmits(["changed"]);

const open = ref(false);
const showScale = ref(false);

const healthy = computed(() => props.vms.filter((v) => v.health_status === "healthy").length);
const running = computed(() => props.vms.filter((v) => v.state === "running").length);
const first = computed(() => props.vms[0]);
const desired = computed(() => props.pool.desired ?? props.vms.length);
const overallHealth = computed(() =>
  healthy.value === props.vms.length && props.vms.length ? "healthy" : "checking"
);

function copySSH() {
  const target = first.value ? first.value.name : props.serviceName;
  navigator.clipboard?.writeText(`porter ssh ${target}`).catch(() => {});
  toast(`Copied: porter ssh ${target}`);
}

async function removeService() {
  if (!confirm(`Remove service "${props.serviceName}" (stops and deletes all its replicas)?`)) return;
  await api(`/projects/${props.projectId}/services/${props.serviceName}`, { method: "DELETE" });
  emit("changed");
}

function onScaled() {
  showScale.value = false;
  emit("changed");
}
</script>

<template>
  <div class="card service-card">
    <div class="service-head">
      <div>
        <div class="service-name">{{ serviceName }}</div>
        <div class="service-image">{{ first ? first.image : "" }}</div>
      </div>
      <div style="text-align:right">
        <div>{{ running }}/{{ desired }} running</div>
        <div class="health-pill" :class="`health-${overallHealth}`">{{ healthy }}/{{ vms.length }} healthy</div>
      </div>
    </div>
    <div class="action-row">
      <button class="btn btn-sm" @click="showScale = true">Scale</button>
      <button class="btn btn-sm" @click="open = !open">Replicas</button>
      <button class="btn btn-sm" @click="copySSH">SSH</button>
      <button class="btn btn-danger btn-sm" @click="removeService">Remove Service</button>
    </div>
    <div class="replica-list" :class="{ open }">
      <div v-for="v in vms" :key="v.id" class="replica-row">
        <span class="replica-id">{{ v.name }}<template v-if="v.ip_address"> &middot; {{ v.ip_address }}</template></span>
        <span><StatusBadge :state="v.state" /> <HealthPill :health="v.health_status" /></span>
      </div>
    </div>
  </div>

  <ScaleModal
    v-if="showScale"
    :project-id="projectId"
    :service="serviceName"
    :current="desired"
    @close="showScale = false"
    @applied="onScaled"
  />
</template>
