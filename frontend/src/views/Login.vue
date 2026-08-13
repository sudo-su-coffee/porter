<script setup>
import { ref } from "vue";
import { useRouter } from "vue-router";
import { login } from "../api/client";

const router = useRouter();
const username = ref("");
const password = ref("");
const error = ref("");
const busy = ref(false);

async function onSubmit() {
  error.value = "";
  busy.value = true;
  try {
    await login(username.value, password.value);
    router.push({ name: "list" });
  } catch (e) {
    error.value = e.message;
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <div class="login-mode">
    <form class="login-box" @submit.prevent="onSubmit">
      <div class="login-brand">
        <span class="brand-mark">▣</span> Porter
      </div>
      <p class="login-sub">Self-hosted PaaS — Firecracker microVMs.</p>

      <div v-if="error" class="error-box">{{ error }}</div>

      <label class="field">
        Username
        <input v-model="username" autocomplete="username" required autofocus placeholder="admin" />
      </label>
      <label class="field">
        Password
        <input v-model="password" type="password" autocomplete="current-password" required placeholder="••••••••" />
      </label>

      <button type="submit" class="btn btn-primary" style="width:100%" :disabled="busy">
        {{ busy ? "Signing in…" : "Sign in" }}
      </button>

      <p class="hint" style="margin-top:14px; text-align:center">
        Default: <span class="mono">admin</span> / the password from <span class="mono">porter.toml</span>
      </p>
      <p class="hint" style="margin-top:10px; text-align:center"><router-link class="back-link" :to="{ name: 'auth-recovery' }">Create account or recover access</router-link></p>
    </form>
  </div>
</template>
