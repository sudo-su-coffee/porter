<template>
  <div>
    <div class="d-flex align-center justify-space-between mb-4">
      <div>
        <div class="text-h5 font-weight-bold">Projects</div>
        <div class="text-caption text-grey">{{ projects.length }} project(s) · each boots a Firecracker microVM</div>
      </div>
      <v-btn color="primary" prepend-icon="mdi-plus" @click="openCreate">New Project</v-btn>
    </div>

    <v-alert v-if="loadError" type="error" density="compact" class="mb-4">{{ loadError }}</v-alert>

    <v-card v-if="projects.length" color="surface">
      <v-table density="comfortable">
        <thead>
          <tr>
            <th>Name</th>
            <th>Image</th>
            <th>Network</th>
            <th>Replicas</th>
            <th align="right">Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in projects" :key="p.id">
            <td>
              <router-link :to="`/projects/${p.id}`" class="text-decoration-none">{{ p.name }}</router-link>
            </td>
            <td class="text-grey">{{ p.image || p.source || '—' }}</td>
            <td class="text-grey">{{ p.network || '—' }}</td>
            <td>
              <v-chip size="small">{{ (p.replicas || []).length }}</v-chip>
            </td>
            <td align="right">
              <v-btn icon size="small" variant="text" :to="`/projects/${p.id}`"><v-icon>mdi-open-in-new</v-icon></v-btn>
              <v-btn icon size="small" variant="text" color="error" @click="destroy(p)"><v-icon>mdi-delete-outline</v-icon></v-btn>
            </td>
          </tr>
        </tbody>
      </v-table>
    </v-card>
    <v-card v-else color="surface" class="text-center pa-8 text-grey">
      No projects. Create one to boot your first microVM.
    </v-card>

    <v-dialog v-model="dialog" max-width="520">
      <v-card>
        <v-card-title>New project</v-card-title>
        <v-card-text>
          <v-form @submit.prevent="create">
            <v-text-field v-model="form.name" label="Name" :rules="[(v) => !!v || 'Name required']"></v-text-field>
            <v-text-field v-model="form.image" label="Image / guest base" placeholder="alpine" hint="Guest base or docker-style image ref"></v-text-field>
            <v-row>
              <v-col cols="6"><v-text-field v-model.number="form.vcpus" label="vCPUs" type="number"></v-text-field></v-col>
              <v-col cols="6"><v-text-field v-model.number="form.mem_mib" label="Memory (MiB)" type="number"></v-text-field></v-col>
            </v-row>
            <v-text-field v-model.number="form.replicas" label="Replicas" type="number" hint="0 = create project only, no boot"></v-text-field>
          </v-form>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn variant="text" @click="dialog = false">Cancel</v-btn>
          <v-btn color="primary" :loading="creating" @click="create">Deploy</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { api } from '../api'

const projects = ref([])
const loadError = ref('')
const dialog = ref(false)
const creating = ref(false)
const form = ref({ name: '', image: 'alpine', vcpus: 1, mem_mib: 512, replicas: 1 })

async function load() {
  try {
    const list = await api.projects()
    projects.value = Array.isArray(list) ? list : (list && list.projects) || []
    loadError.value = ''
  } catch (e) {
    loadError.value = e.message
  }
}
onMounted(load)

function openCreate() {
  dialog.value = true
}

async function create() {
  if (!form.value.name) return
  creating.value = true
  try {
    await api.createProject({
      name: form.value.name,
      image: form.value.image || 'alpine',
      vcpus: form.value.vcpus || 1,
      mem_mib: form.value.mem_mib || 256,
      replicas: form.value.replicas != null ? form.value.replicas : 1
    })
    dialog.value = false
    form.value = { name: '', image: 'alpine', vcpus: 1, mem_mib: 256, replicas: 1 }
    setTimeout(load, 600)
  } catch (e) {
    loadError.value = 'Create failed: ' + e.message
  } finally {
    creating.value = false
  }
}

async function destroy(p) {
  if (!confirm(`Delete project ${p.name}?`)) return
  try {
    await api.deleteProject(p.id)
    load()
  } catch (e) {
    loadError.value = e.message
  }
}
</script>