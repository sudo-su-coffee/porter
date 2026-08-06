<script setup>
import { ref, onMounted, onUnmounted, computed } from "vue";
import { useRouter } from "vue-router";
import { api } from "../api/client";
import { connectEvents, disconnectEvents } from "../api/events";
import ServiceCard from "../components/ServiceCard.vue";

const props = defineProps({ id: { type: String, required: true } });
const router = useRouter();

const project = ref(null);
const vms = ref([]);
const error = ref("");

const byService = computed(() => {
  const map = {};
  for (const v of vms.value) {
    (map[v.service_name] = map[v.service_name] || []).push(v);
  }
  Object.values(map).forEach((list) => list.sort((a, b) => a.replica_index - b.replica_index));
  return map;
});

const serviceNames = computed(() =>
  project.value?.service_pools ? Object.keys(project.value.service_pools) : Object.keys(byService.value)
);

async function load() {
  error.value = "";
  try {
    const data = await api(`/projects/${props.id}?expand=vms`);
    project.value = data.project;
    vms.value = data.vms || [];
  } catch (e) {
    error.value = e.message;
  }
}

async function deleteProject() {
  if (!confirm(`Delete project "${project.value.name}" and all its replicas?`)) return;
  await api(`/projects/${props.id}`, { method: "DELETE" });
  router.push({ name: "list" });
}

onMounted(() => {
  load();
  connectEvents(() => load());
});
onUnmounted(() => disconnectEvents());
</script>

<template>
  <a class="back-link" @click="router.push({ name: 'list' })">&larr; Deployments</a>

  <div v-if="error" class="error-box">{{ error }}</div>

  <template v-else-if="project">
    <div class="detail-header">
      <div class="detail-title">{{ project.name }} <span class="card-badge">compose</span></div>
      <div class="page-sub">
        Created {{ new Date(project.created_at).toLocaleString() }} &middot; network {{ project.network }}
      </div>
      <div class="detail-actions">
        <button class="btn btn-danger btn-sm" @click="deleteProject">Delete Project</button>
      </div>
    </div>

    <div id="services">
      <ServiceCard
        v-for="svc in serviceNames"
        :key="svc"
        :project-id="id"
        :service-name="svc"
        :pool="(project.service_pools || {})[svc] || {}"
        :vms="byService[svc] || []"
        @changed="load"
      />
    </div>
  </template>
</template>
