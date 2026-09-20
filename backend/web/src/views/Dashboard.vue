<template>
  <div>
    <div class="d-flex align-center justify-space-between mb-4">
      <div>
        <div class="text-h5 font-weight-bold">Control Plane</div>
        <div class="text-caption text-grey">{{ version }}</div>
      </div>
      <v-btn color="primary" prepend-icon="mdi-plus" to="/projects?new=1">New Project</v-btn>
    </div>

    <v-row>
      <v-col cols="12" md="3" v-for="c in cards" :key="c.label">
        <v-card color="surface">
          <v-card-text class="d-flex align-center">
            <v-icon size="34" :color="c.color">{{ c.icon }}</v-icon>
            <div class="ml-3">
              <div class="text-h5 font-weight-bold">{{ c.value }}</div>
              <div class="text-caption text-grey">{{ c.label }}</div>
            </div>
          </v-card-text>
        </v-card>
      </v-col>
    </v-row>

    <v-row class="mt-2">
      <v-col cols="12" md="6">
        <v-card color="surface">
          <v-card-title class="text-subtitle-1 font-weight-bold">Host prerequisites</v-card-title>
          <v-list v-if="prereqs.length">
            <v-list-item v-for="p in prereqs" :key="p.name">
              <template #prepend>
                <v-icon :color="p.ok ? 'success' : 'error'">{{ p.ok ? 'mdi-check-circle' : 'mdi-alert-circle' }}</v-icon>
              </template>
              <v-list-item-title>{{ p.name }}</v-list-item-title>
              <v-list-item-subtitle class="text-caption">{{ p.message }}</v-list-item-subtitle>
            </v-list-item>
          </v-list>
          <v-card-text v-else class="text-grey text-caption">No prerequisite report available.</v-card-text>
        </v-card>
      </v-col>
      <v-col cols="12" md="6">
        <v-card color="surface">
          <v-card-title class="text-subtitle-1 font-weight-bold">Recent projects</v-card-title>
          <v-list v-if="projects.length">
            <v-list-item v-for="p in projects" :key="p.id" :to="`/projects/${p.id}`">
              <template #prepend><v-icon>mdi-layers-triple-outline</v-icon></template>
              <v-list-item-title>{{ p.name }}</v-list-item-title>
              <v-list-item-subtitle class="text-caption">{{ p.image || p.source || '—' }}</v-list-item-subtitle>
            </v-list-item>
          </v-list>
          <v-card-text v-else class="text-grey text-caption">No projects yet. Create one to boot a microVM.</v-card-text>
        </v-card>
      </v-col>
    </v-row>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { api } from '../api'

const version = ref('')
const prereqs = ref([])
const projects = ref([])
const summary = ref({ replicas: 0, networks: 0, domains: 0 })

const cards = computed(() => [
  { label: 'Projects', value: projects.value.length, icon: 'mdi-layers-triple-outline', color: 'primary' },
  { label: 'MicroVMs', value: summary.value.replicas, icon: 'mdi-server', color: 'accent' },
  { label: 'Networks', value: summary.value.networks, icon: 'mdi-vector-polyline', color: 'info' },
  { label: 'Domains', value: summary.value.domains, icon: 'mdi-web', color: 'warning' }
])

onMounted(async () => {
  try {
    const v = await api.version()
    version.value = v.version || v
  } catch {}
  try {
    prereqs.value = (await api.hostPrereqs()) || []
  } catch {}
  try {
    const list = (await api.projects()) || []
    projects.value = Array.isArray(list) ? list : list.projects || []
  } catch {}
  try {
    const h = await api.hostOverview()
    if (h && h.replicas != null) summary.value = h
  } catch {}
})
</script>