<template>
  <div>
    <div class="text-h5 font-weight-bold mb-4">Admin · RBAC &amp; Tenancy</div>

    <v-row>
      <v-col cols="12" md="6">
        <v-card color="surface">
          <v-card-title class="text-subtitle-1 font-weight-bold">Users ({{ users.length }})</v-card-title>
          <v-table v-if="users.length" density="comfortable">
            <thead><tr><th>Username</th><th>Role</th><th>Email</th></tr></thead>
            <tbody>
              <tr v-for="u in users" :key="u.id || u.username">
                <td>{{ u.username }}</td>
                <td><v-chip size="x-small">{{ u.role }}</v-chip></td>
                <td class="text-grey text-caption">{{ u.email || '—' }}</td>
              </tr>
            </tbody>
          </v-table>
          <v-card-text v-else class="text-grey text-caption">No users.</v-card-text>
        </v-card>
      </v-col>
      <v-col cols="12" md="6">
        <v-card color="surface" class="mb-4">
          <v-card-title class="text-subtitle-1 font-weight-bold">Roles</v-card-title>
          <v-list v-if="roles.length">
            <v-list-item v-for="r in roles" :key="r.id">
              <v-list-item-title>{{ r.name }}</v-list-item-title>
              <v-list-item-subtitle class="text-caption">{{ r.description }}</v-list-item-subtitle>
            </v-list-item>
          </v-list>
          <v-card-text v-else class="text-grey text-caption">No roles.</v-card-text>
        </v-card>
        <v-card color="surface">
          <v-card-title class="text-subtitle-1 font-weight-bold">Current org</v-card-title>
          <v-card-text class="text-grey">{{ JSON.stringify(org) }}</v-card-text>
        </v-card>
      </v-col>
    </v-row>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { api } from '../api'

const users = ref([])
const roles = ref([])
const org = ref({})

onMounted(async () => {
  try {
    const list = await api.users()
    users.value = Array.isArray(list) ? list : (list && list.users) || []
  } catch {}
  try {
    const list = await api.roles()
    roles.value = Array.isArray(list) ? list : (list && list.roles) || []
  } catch {}
  try {
    org.value = (await api.org()) || {}
  } catch {}
})
</script>