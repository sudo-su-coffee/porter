<script setup>
import { ref, onMounted, onUnmounted, computed } from "vue";
import { useRouter } from "vue-router";
import { api } from "../api/client";
import { connectEvents, disconnectEvents } from "../api/events";
import StatusBadge from "../components/StatusBadge.vue";
import HealthPill from "../components/HealthPill.vue";

const router = useRouter();
const projects = ref([]);
const vms = ref([]);
const loading = ref(true);
const error = ref("");

const standalone = computed(() => vms.value.filter((v) => !v.project_id));
const isEmpty = computed(() => projects.value.length === 0 && standalone.value.length === 0);

function projectStats(p) {
  const pvms = vms.value.filter((v) => v.project_id === p.id);
  const running = pvms.filter((v) => v.state === "running").length;
  const healthy = pvms.filter((v) => v.health_status === "healthy").length;
  const withHC = pvms.filter((v) => v.health_status !== "checking" || v.healthcheck).length;
  return { pvms, running, healthy, withHC };
}

async function load() {
  error.value = "";
  try {
    const [p, v] = await Promise.all([api("/projects"), api("/vms")]);
    projects.value = p || [];
    vms.value = v || [];
  } catch (e) {
    error.value = e.message;
  } finally {
    loading.value = false;
  }
}

onMounted(() => {
  load();
  connectEvents(() => load());
});
onUnmounted(() => disconnectEvents());
</script>

<template>
  <div class="page-header">
    <div>
      <div class="page-title">Deployments</div>
      <div class="page-sub">Projects and standalone deployments on this host</div>
    </div>
  </div>

  <div v-if="error" class="error-box">{{ error }}</div>

  <div v-else-if="isEmpty && !loading" class="empty-state">
    Deploy your first image or compose file — click <b>New Project</b> above.
  </div>

  <div v-else class="grid">
    <div
      v-for="p in projects"
      :key="p.id"
      class="card card-link"
      @click="router.push({ name: 'project', params: { id: p.id } })"
    >
      <div class="card-head">
        <div class="card-title">{{ p.name }}</div>
        <span class="card-badge">compose</span>
      </div>
      <div class="card-meta">
        <span>
          {{ projectStats(p).running }}/{{ projectStats(p).pvms.length }} running
          <template v-if="projectStats(p).withHC">
            &middot; {{ projectStats(p).healthy }}/{{ projectStats(p).withHC }} healthy
          </template>
        </span>
        <span>{{ Object.keys(p.service_pools || {}).length }} service(s)</span>
      </div>
    </div>

    <div
      v-for="v in standalone"
      :key="v.id"
      class="card card-link"
      @click="router.push({ name: 'vm', params: { id: v.id } })"
    >
      <div class="card-head">
        <div class="card-title">{{ v.name }}</div>
        <span class="card-badge">single-image</span>
      </div>
      <div class="card-meta">
        <StatusBadge :state="v.state" />
        <HealthPill :health="v.health_status" />
        <span class="mono">{{ v.image }}</span>
        <span v-if="v.ip_address" class="mono">{{ v.ip_address }}</span>
      </div>
    </div>
  </div>
</template>
