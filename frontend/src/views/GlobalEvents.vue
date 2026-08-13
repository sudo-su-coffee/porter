<!-- Porter event stream view: authenticated dashboard navigation around the global live event hub. -->
<script setup>
import { onMounted, onUnmounted, ref } from "vue";
import { useRouter } from "vue-router";

const router = useRouter();
const live = ref(false);
const events = ref([]);
let source;
const names = ["vm.state", "replica.health", "project.progress", "pool.updated", "domain.status", "traffic.request", "cache.purged"];

function connect() {
  try {
    source = new EventSource("/events");
    source.onopen = () => { live.value = true; };
    source.onerror = () => { live.value = false; };
    names.forEach((name) => source.addEventListener(name, (event) => events.value.unshift({ name, data: event.data, at: new Date().toISOString() })));
  } catch (_) { live.value = false; }
}
onMounted(connect);
onUnmounted(() => source?.close());
</script>

<template>
  <a class="back-link" @click="router.back()">&larr; Back</a>
  <header class="page-header"><div><div class="page-title">Live events</div><div class="page-sub">Global `/events` hub messages for replica, pool, domain, traffic, and cache transitions.</div></div><span class="tag" :class="live ? 'tag-green' : 'tag-amber'">{{ live ? 'connected' : 'reconnecting' }}</span></header>
  <div v-if="!events.length" class="empty-state">No live events received yet. Idle systems do not generate synthetic messages.</div>
  <div v-else class="table-wrap"><table class="data-table"><thead><tr><th>Timestamp</th><th>Event</th><th>Payload</th></tr></thead><tbody><tr v-for="event in events" :key="`${event.at}-${event.name}-${event.data}`"><td class="mono">{{ event.at }}</td><td class="mono">{{ event.name }}</td><td class="mono">{{ event.data }}</td></tr></tbody></table></div>
</template>
