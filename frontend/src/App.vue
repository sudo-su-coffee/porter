<script setup>
import { computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { connectionLive } from "./api/events";
import NewProjectModal from "./components/NewProjectModal.vue";
import ToastContainer from "./components/ToastContainer.vue";
import { ref } from "vue";

const route = useRoute();
const router = useRouter();
const showTopbar = computed(() => route.name !== "login");
const showNewProjectModal = ref(false);

function openNewProject() {
  showNewProjectModal.value = true;
}

function onCreated(target) {
  showNewProjectModal.value = false;
  router.push(target);
}
</script>

<template>
  <header class="topbar" v-if="showTopbar">
    <div class="brand"><span class="brand-mark">▣</span> Porter</div>
    <nav class="topnav">
      <button class="nav-link active" @click="router.push({ name: 'list' })">
        Deployments
      </button>
    </nav>
    <div class="topbar-right">
      <span
        class="conn-dot"
        :class="{ live: connectionLive, down: !connectionLive }"
        :title="connectionLive ? 'Live' : 'Connecting...'"
      ></span>
      <button class="btn btn-primary" @click="openNewProject">New Project</button>
    </div>
  </header>

  <main id="app-main">
    <router-view />
  </main>

  <NewProjectModal v-if="showNewProjectModal" @close="showNewProjectModal = false" @created="onCreated" />
  <ToastContainer />
</template>
