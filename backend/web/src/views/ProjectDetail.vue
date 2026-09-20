<template>
  <div>
    <div class="d-flex align-center mb-3">
      <v-btn variant="text" icon="mdi-arrow-left" to="/projects"></v-btn>
      <div class="ml-1">
        <div class="text-h5 font-weight-bold">{{ project.name || 'Project' }}</div>
        <div class="text-caption text-grey">{{ project.id }} · {{ project.image || project.source || '—' }}</div>
      </div>
      <v-spacer></v-spacer>
      <v-chip :color="stateColor" size="small">{{ project.state || status || '—' }}</v-chip>
    </div>

    <v-alert v-if="error" type="error" density="compact" class="mb-3">{{ error }}</v-alert>

    <v-tabs v-model="tab" color="primary">
      <v-tab value="replicas">MicroVMs</v-tab>
      <v-tab value="networks">Networks</v-tab>
      <v-tab value="domains">Domains</v-tab>
    </v-tabs>

    <v-window v-model="tab" class="mt-3">
      <!-- Replicas / microVMs -->
      <v-window-item value="replicas">
        <v-card v-if="replicas.length" color="surface">
          <v-table density="comfortable">
            <thead>
              <tr>
                <th>#</th>
                <th>State</th>
                <th>Health</th>
                <th>IP</th>
                <th>vCPU</th>
                <th>Mem</th>
                <th align="right">Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="r in replicas" :key="r.id">
                <td>{{ r.replica_index ?? r.name || r.id.slice(0, 6) }}</td>
                <td><v-chip :color="vmStateColor(r.state)" size="x-small">{{ r.state }}</v-chip></td>
                <td class="text-caption text-grey">{{ r.health_status }}</td>
                <td class="font-mono text-caption">{{ r.ip_address || '—' }}</td>
                <td>{{ r.vcpus }}</td>
                <td>{{ r.mem_mib }}</td>
                <td align="right">
                  <v-btn v-if="r.state === 'running'" icon size="small" variant="text" color="error" title="stop" @click="act('stop', r)"><v-icon size="small">mdi-stop-circle</v-icon></v-btn>
                  <v-btn v-else icon size="small" variant="text" color="success" title="start" @click="act('start', r)"><v-icon size="small">mdi-play-circle</v-icon></v-btn>
                  <v-btn icon size="small" variant="text" color="warning" title="restart" @click="act('restart', r)"><v-icon size="small">mdi-restart</v-icon></v-btn>
                  <v-btn icon size="small" variant="text" color="info" title="snapshot" @click="act('snapshot', r)"><v-icon size="small">mdi-camera</v-icon></v-btn>
                </td>
              </tr>
            </tbody>
          </v-table>
        </v-card>
        <v-card v-else color="surface" class="text-center pa-8 text-grey">
          No microVMs. Scale the project from <b>Projects</b> (replicas > 0) or start one below.
        </v-card>
        <v-btn class="mt-3" color="primary" prepend-icon="mdi-plus" :loading="scaling" @click="scaleUp">
          Add microVM
        </v-btn>
      </v-window-item>

      <!-- Networks -->
      <v-window-item value="networks">
        <v-card color="surface">
          <v-card-text>
            <v-list v-if="networks.length">
              <v-list-item v-for="n in networks" :key="n.id">
                <template #prepend><v-icon>mdi-vector-polyline</v-icon></template>
                <v-list-item-title>{{ n.name }}</v-list-item-title>
                <v-list-item-subtitle class="text-caption">{{ n.cidr }} · {{ n.driver }}</v-list-item-subtitle>
              </v-list-item>
            </v-list>
            <div v-else class="text-grey text-caption">No networks for this project.</div>
          </v-card-text>
        </v-card>
      </v-window-item>

      <!-- Domains -->
      <v-window-item value="domains">
        <v-card color="surface">
          <v-card-text>
            <v-list v-if="domains.length">
              <v-list-item v-for="d in domains" :key="d.domain">
                <template #prepend><v-icon>mdi-web</v-icon></template>
                <v-list-item-title>{{ d.domain }}</v-list-item-title>
                <v-list-item-subtitle class="text-caption">{{ d.type }} · {{ d.status }}</v-list-item-subtitle>
              </v-list-item>
            </v-list>
            <div v-else class="text-grey text-caption">No domains for this project.</div>
          </v-card-text>
        </v-card>
      </v-window-item>
    </v-window>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '../api'

const route = useRoute()
const id = route.params.id
const project = ref({})
const replicas = ref([])
const networks = ref([])
const domains = ref([])
const tab = ref('replicas')
const error = ref('')
const status = ref('')
const scaling = ref(false)

const stateColor = (s) => (s === 'running' ? 'success' : s === 'failed' ? 'error' : 'info')
const vmStateColor = (s) =>
  s === 'running' ? 'success' : ['stopped', 'deleted'].includes(s) ? 'grey' : s === 'failed' ? 'error' : 'info'

async function load() {
  try {
    project.value = (await api.project(id)) || {}
    status.value = project.value.status || ''
  } catch (e) {
    error.value = e.message
  }
  try {
    const list = await api.replicas(id)
    replicas.value = Array.isArray(list) ? list : (list && list.replicas) || []
  } catch (e) {
    error.value = 'replicas: ' + e.message
  }
  try {
    const n = await api.networks(id)
    networks.value = Array.isArray(n) ? n : (n && n.networks) || []
  } catch {}
  try {
    const d = await api.domains(id)
    domains.value = Array.isArray(d) ? d : (d && d.domains) || []
  } catch {}
}

onMounted(() => {
  load()
  const t = setInterval(load, 4000)
  // cleanup is fine for a single detail view
  window.addEventListener('beforeunload', () => clearInterval(t))
})

async function act(kind, r) {
  const n = r.replica_index != null ? r.replica_index : r.name
  const fns = { start: api.replicaStart, stop: api.replicaStop, restart: api.replicaRestart, snapshot: api.replicaSnapshot }
  try {
    error.value = ''
    await fns[kind](id, n)
    setTimeout(load, 800)
  } catch (e) {
    error.value = `${kind} failed: ${e.message}`
  }
}

async function scaleUp() {
  scaling.value = true
  try {
    await api.raw(`/projects/${id}/scale`, { method: 'PATCH', body: { replicas: replicas.value.length + 1 } })
    setTimeout(load, 800)
  } catch (e) {
    error.value = 'scale: ' + e.message
  } finally {
    scaling.value = false
  }
}
</script>