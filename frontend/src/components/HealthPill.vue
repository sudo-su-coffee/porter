<script setup>
defineProps({ health: { type: String, default: "" } });
</script>

<template>
  <span v-if="health" class="health-pill" :class="`health-${health}`">{{ health }}</span>
</template>
