<script setup>
import { ref, onMounted, onUnmounted } from "vue";
import { api } from "../api/client";
import { connectEvents, disconnectEvents } from "../api/events";

const lines = ref([]);
const auto = ref(true);
let timer = null;

async function load() {
  try {
    const r = await api("/logs?tail=500");
    lines.value = r.lines || [];
  } catch (_) {
    /* /logs may be missing on older binaries */
  }
}

function toggleAuto() {
  auto.value = !auto.value;
  if (auto.value) { load(); start(); } else stop();
}
function start() {
  stop();
  timer = setInterval(load, 3000);
}
function stop() {
  if (timer) clearInterval(timer);
  timer = null;
}

onMounted(() => {
  load();
  start();
  connectEvents(() => { if (auto.value) load(); });
});
onUnmounted(() => {
  stop();
  disconnectEvents();
});
</script>

<template>
  <header class="page-header">
    <div>
      <div class="page-title">Logs</div>
      <div class="page-sub">Porter daemon / audit log — server start &amp; stop, lifecycle events.</div>
    </div>
    <div class="detail-actions">
      <button class="btn btn-sm" @click="load">Refresh</button>
      <button class="btn btn-sm" @click="toggleAuto">{{ auto ? 'Pause' : 'Resume' }} auto-refresh</button>
    </div>
  </header>

  <div class="terminal" style="max-height:70vh;">
    <div v-if="lines.length" v-for="(l, i) in lines" :key="i" class="tline">{{ l }}</div>
    <div v-else class="t-empty">No daemon log output yet — start the server to see events.</div>
  </div>
</template>