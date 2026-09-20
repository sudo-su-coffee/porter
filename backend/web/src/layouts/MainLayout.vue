<template>
  <v-app>
    <v-navigation-drawer v-model="drawer" :permanent="!isMobile" color="surface" width="220">
      <div class="d-flex align-center pa-4">
        <v-icon color="primary">mdi-console</v-icon>
        <div class="ml-2">
          <div class="text-subtitle-1 font-weight-bold">Porter</div>
          <div class="text-caption text-grey">MicroVM compute</div>
        </div>
      </div>
      <v-divider></v-divider>
      <v-list nav>
        <v-list-item v-for="item in nav" :key="item.to" :to="item.to" prepend-icon="mdi-undefined" :title="item.title" :value="item.to" @click="navigate(item)">
          <template #prepend><v-icon>{{ item.icon }}</v-icon></template>
        </v-list-item>
      </v-list>
      <template #append>
        <v-divider></v-divider>
        <div class="pa-3">
          <div class="d-flex align-center">
            <v-avatar color="primary" size="30">
              <span class="text-overline">{{ initials }}</span>
            </v-avatar>
            <div class="ml-2 overflow-hidden">
              <div class="text-caption font-weight-medium text-truncate">{{ user.username || 'admin' }}</div>
              <div class="text-caption text-grey">{{ user.role }}</div>
            </div>
            <v-spacer></v-spacer>
            <v-btn v-if="isMobile" icon="mdi-close" size="x-small" @click="drawer = false"></v-btn>
          </div>
          <v-btn variant="text" size="small" class="mt-2 text-grey" prepend-icon="mdi-logout" @click="logout">
            Sign out
          </v-btn>
        </div>
      </template>
    </v-navigation-drawer>

    <v-main>
      <v-container fluid class="pa-4">
        <router-view />
      </v-container>
    </v-main>
  </v-app>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { store } from '../api'

const router = useRouter()
const drawer = ref(true)
const isMobile = ref(false)
const user = store.user
const initials = computed(() => (user.username || 'a').slice(0, 2).toUpperCase())

const nav = [
  { title: 'Dashboard', to: '/', icon: 'mdi-view-dashboard' },
  { title: 'Projects', to: '/projects', icon: 'mdi-layers-triple-outline' },
  { title: 'Images', to: '/images', icon: 'mdi-database-arrow-down-outline' },
  { title: 'Host', to: '/host', icon: 'mdi-server' },
  { title: 'Admin', to: '/admin', icon: 'mdi-shield-account' }
]

function navigate(item) {
  if (isMobile.value) drawer.value = false
}

function logout() {
  store.token = ''
  store.user = {}
  router.push({ name: 'login' })
}

onMounted(() => {
  isMobile.value = window.innerWidth < 800
  window.addEventListener('resize', () => (isMobile.value = window.innerWidth < 800))
})
</script>