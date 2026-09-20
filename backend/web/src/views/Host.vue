<template>
  <div>
    <div class="text-h5 font-weight-bold mb-4">Host &amp; Runtime</div>
    <v-row>
      <v-col cols="12" md="6">
        <v-card color="surface">
          <v-card-title class="text-subtitle-1 font-weight-bold">Kernel &amp; prerequisites</v-card-title>
          <v-list v-if="prereqs.length">
            <v-list-item v-for="p in prereqs" :key="p.name">
              <template #prepend>
                <v-icon :color="p.ok ? 'success' : 'error'">{{ p.ok ? 'mdi-check-circle' : 'mdi-alert-circle' }}</v-icon>
              </template>
              <v-list-item-title>{{ p.name }}</v-list-item-title>
              <v-list-item-subtitle class="text-caption">{{ p.message }}</v-list-item-subtitle>
            </v-list-item>
          </v-list>
          <v-card-text v-else class="text-grey text-caption">No data.</v-card-text>
        </v-card>
      </v-col>
      <v-col cols="12" md="6">
        <v-card color="surface">
          <v-card-title class="text-subtitle-1 font-weight-bold">Overview</v-card-title>
          <v-list dense>
            <v-list-item v-for="(v, k) in overview" :key="k">
              {{ k }} : <span class="font-mono text-caption">{{ JSON.stringify(v) }}</span>
            </v-list-item>
            <v-list-item v-if="!Object.keys(overview).length" class="text-grey text-caption">No data.</v-list-item>
          </v-list>
        </v-card>
      </v-col>
    </v-row>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { api } from '../api'

const prereqs = ref([])
const overview = ref({})

onMounted(async () => {
  try {
    prereqs.value = (await api.hostPrereqs()) || []
  } catch {}
  try {
    const o = (await api.hostOverview()) || {}
    overview.value = typeof o === 'object' ? o : { raw: o }
  } catch {}
})
</script>