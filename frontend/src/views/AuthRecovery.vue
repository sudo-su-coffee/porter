<!-- Porter auth contract surface: signup and explicit password-recovery responses, without pretending a provider exists. -->
<script setup>
import { ref } from "vue";
import { useRouter } from "vue-router";
import { api } from "../api/client";

const router = useRouter();
const mode = ref("signup");
const form = ref({ username: "", password: "", email: "", token: "", new_password: "" });
const result = ref(null);
const error = ref("");
const busy = ref(false);

async function submit() {
  busy.value = true;
  error.value = "";
  const bodies = { signup: { username: form.value.username, password: form.value.password, email: form.value.email }, forgot: { username: form.value.username, email: form.value.email }, reset: { token: form.value.token, password: form.value.new_password } };
  const paths = { signup: "/auth/signup", forgot: "/auth/password/forgot", reset: "/auth/password/reset" };
  try { result.value = await fetch(paths[mode.value], { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(bodies[mode.value]) }).then(async (response) => { const body = await response.json().catch(() => ({})); if (!response.ok) throw new Error(body.error || `HTTP ${response.status}`); return body; }); }
  catch (err) { error.value = err.message; }
  finally { busy.value = false; }
}
</script>

<template>
  <a class="back-link" @click="router.push({ name: 'login' })">&larr; Sign in</a>
  <header class="page-header"><div><div class="page-title">Account access</div><div class="page-sub">Real signup and password-recovery contracts. Beta-dev may return an explicit unsupported response when no provider is configured.</div></div></header>
  <div class="seg stream-tabs"><button v-for="item in [['signup','Create account'],['forgot','Forgot password'],['reset','Reset password']]" :key="item[0]" :class="{ active: mode === item[0] }" @click="mode = item[0]; result = null; error = ''">{{ item[1] }}</button></div>
  <section class="card"><div v-if="error" class="error-box">{{ error }}</div><div v-if="result" class="notice-box"><pre class="settings-json">{{ JSON.stringify(result, null, 2) }}</pre></div><div v-if="mode === 'signup'"><div class="field"><label>Username</label><input v-model="form.username" required /></div><div class="field"><label>Password</label><input v-model="form.password" type="password" required /></div><div class="field"><label>Email (optional)</label><input v-model="form.email" type="email" /></div></div><div v-else-if="mode === 'forgot'"><div class="field"><label>Username</label><input v-model="form.username" /></div><div class="field"><label>Email</label><input v-model="form.email" type="email" /></div></div><div v-else><div class="field"><label>Reset token</label><input v-model="form.token" /></div><div class="field"><label>New password</label><input v-model="form.new_password" type="password" /></div></div><button class="btn btn-primary btn-sm" :disabled="busy" @click="submit">{{ busy ? 'Sending…' : mode === 'signup' ? 'Create account' : mode === 'forgot' ? 'Request reset' : 'Reset password' }}</button></section>
</template>
