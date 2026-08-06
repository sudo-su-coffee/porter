<script setup>
import { toasts } from "./toast";
</script>

<template>
  <div class="toast-stack">
    <div v-for="t in toasts" :key="t.id" class="toast">{{ t.message }}</div>
  </div>
</template>
