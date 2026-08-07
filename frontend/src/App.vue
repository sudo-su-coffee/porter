<script setup>
import { computed, ref, onMounted, onUnmounted } from "vue";
import { useRoute, useRouter } from "vue-router";
import { connectionLive, connectEvents, disconnectEvents } from "./api/events";
import NewProjectModal from "./components/NewProjectModal.vue";
import ToastContainer from "./components/ToastContainer.vue";
import { clearToken } from "./api/client";

const route = useRoute();
const router = useRouter();
const showNewProject = ref(false);
const showTopbar = computed(() => route.name !== "login");

function logout() {
  clearToken();
  router.push({ name: "login" });
}

onMounted(() => connectEvents(() => {}));
onUnmounted(() => disconnectEvents());
</script>

<template>
  <header class="topbar" v-if="showTopbar">
    <div class="brand" @click="router.push({ name: 'list' })">
      <span class="brand-mark">▣</span> Porter
    </div>
    <nav class="topnav">
      <router-link class="nav-link" :to="{ name: 'list' }">Deployments</router-link>
      <router-link class="nav-link" :to="{ name: 'images' }">Images</router-link>
      <router-link class="nav-link" :to="{ name: 'servers' }">Servers</router-link>
      <router-link class="nav-link" :to="{ name: 'logs' }">Logs</router-link>
    </nav>
    <div class="topbar-right">
      <span class="conn-dot" :class="connectionLive ? 'live' : 'down'" :title="connectionLive ? 'Live' : 'Connecting…'"></span>
      <button class="btn btn-primary" @click="showNewProject = true">New Project</button>
      <button class="btn" @click="logout">Sign out</button>
    </div>
  </header>

  <main>
    <router-view />
  </main>

  <NewProjectModal v-if="showNewProject" @close="showNewProject = false" @created="(t) => { showNewProject = false; router.push(t); }" />
  <ToastContainer />
</template>