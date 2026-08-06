<script setup>
import { ref } from "vue";
import { useRouter } from "vue-router";
import { login } from "../api/client";

const router = useRouter();
const username = ref("");
const password = ref("");
const error = ref("");

async function onSubmit() {
  error.value = "";
  try {
    await login(username.value, password.value);
    router.push({ name: "list" });
  } catch (e) {
    error.value = e.message;
  }
}
</script>

<template>
  <div class="login-mode">
    <form class="login-box" @submit.prevent="onSubmit">
      <div class="login-brand"><span class="brand-mark">▣</span> Porter</div>
      <p class="login-sub">Sign in to manage this host's microVMs.</p>
      <div v-if="error" class="error-box">{{ error }}</div>
      <label class="field">
        Username
        <input v-model="username" autocomplete="username" required autofocus />
      </label>
      <label class="field">
        Password
        <input v-model="password" type="password" autocomplete="current-password" required />
      </label>
      <button type="submit" class="btn btn-primary" style="width:100%">Sign in</button>
    </form>
  </div>
</template>
