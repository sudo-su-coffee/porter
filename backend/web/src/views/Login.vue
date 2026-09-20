<template>
  <v-app>
    <v-main class="d-flex align-center justify-center">
      <v-card width="420" class="pa-8" color="surface">
        <div class="d-flex align-center mb-6">
          <v-icon color="primary" size="38">mdi-console</v-icon>
          <div class="ml-3">
            <div class="text-h5 font-weight-bold">Porter</div>
            <div class="text-caption text-grey">MicroVM compute, simplified.</div>
          </div>
        </div>
        <v-alert v-if="error" type="error" density="compact" class="mb-4">{{ error }}</v-alert>
        <v-form @submit.prevent="submit" ref="form">
          <v-text-field
            v-model="username"
            label="Username"
            prepend-inner-icon="mdi-account"
            autocomplete="username"
            :rules="[(v) => !!v || 'Username required']"
          ></v-text-field>
          <v-text-field
            v-model="password"
            label="Password"
            prepend-inner-icon="mdi-lock"
            type="password"
            autocomplete="current-password"
            :rules="[(v) => !!v || 'Password required']"
            @keyup.enter="submit"
          ></v-text-field>
          <v-btn type="submit" color="primary" block :loading="loading" class="mt-2">
            Sign in
          </v-btn>
          <v-btn variant="text" block class="mt-2 text-grey" @click="demoFill">Use seeded admin</v-btn>
        </v-form>
      </v-card>
    </v-main>
  </v-app>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { api, store } from '../api'

const router = useRouter()
const route = useRoute()
const username = ref('admin')
const password = ref('')
const error = ref('')
const loading = ref(false)
const form = ref(null)

function demoFill() {
  username.value = 'admin'
  password.value = 'admin-Porter-2026'
  submit()
}

async function submit() {
  error.value = ''
  loading.value = true
  try {
    const res = await api.login(username.value, password.value)
    store.token = res.token
    store.user = res.user
    router.push(route.query.redirect || '/')
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}
</script>