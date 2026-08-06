<script setup>
import { computed } from "vue";

const props = defineProps({ state: { type: String, required: true } });

const STATE_LABEL = {
  pending: "Pending",
  booting: "Booting",
  running: "Running",
  stopping: "Stopping",
  stopped: "Stopped",
  failed: "Failed",
};

const pulsing = computed(() => ["pending", "booting", "stopping"].includes(props.state));
const label = computed(() => STATE_LABEL[props.state] || props.state);
</script>

<template>
  <span class="status" :class="`state-${state}`">
    <span class="status-dot" :class="{ pulsing }"></span>{{ label }}
  </span>
</template>
