<script setup>
import { ref, onMounted, onUnmounted, computed } from "vue";
import { api } from "../api/client";
import { connectEvents, disconnectEvents } from "../api/events";

const lines = ref([]);
const auto = ref(true);
const q = ref("");
let timer = null;

const filtered = computed(() => {
  if (!q.value.trim()) return lines.value;
  const s = q.value.toLowerCase();
  return lines.value.filter((l) => String(l).toLowerCase().includes(s));
});

async function load() {
  try {
    const r = await api("/logs?tail=500");
    const list = Array.isArray(r) ? r : r?.logs || r?.lines || [];
    lines.value = list;
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
  <div class="page-header">
    <div>
      <div class="page-title">Logs</div>
      <div class="page-sub">Porter daemon / audit log — server start &amp; stop, lifecycle events.</div>
    </div>
    <div class="detail-actions">
      <button class="btn btn-sm" @click="load">Refresh</button>
      <button class="btn btn-sm" @click="toggleAuto">{{ auto ? 'Pause' : 'Resume' }}</button>
    </div>
  </div>

  <div class="filter-bar">
    <input v-model="q" placeholder="Filter log lines…" style="flex:1; max-width:320px" />
    <span class="hint">{{ filtered.length }} of {{ lines.length }} line(s)</span>
  </div>

  <div class="terminal" style="max-height:70vh">
    <template v-if="filtered.length">
      <div v-for="(l, i) in filtered" :key="i" class="tline">{{ l }}</div>
    </template>
    <div v-else class="t-empty">No daemon log output yet — start the server to see events.</div>
  </div>
</template>
