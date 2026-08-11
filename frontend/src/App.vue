<script setup>
import { computed, ref, onMounted, onUnmounted } from "vue";
import { useRoute, useRouter } from "vue-router";
import { connectionLive, connectEvents, disconnectEvents } from "./api/events";
import NewProjectModal from "./components/NewProjectModal.vue";
import ToastContainer from "./components/ToastContainer.vue";
import { clearToken, getToken } from "./api/client";

const route = useRoute();
const router = useRouter();
const showNewProject = ref(false);
const authed = computed(() => route.name !== "login");
const user = computed(() => getToken() ? "admin" : "");

const nav = [
  { section: "Overview" },
  { name: "Deployments", to: "/", icon: "◧" },
  { name: "Traffic", to: "/traffic", icon: "⇄" },
  { name: "Analytics", to: "/analytics", icon: "◫" },
  { name: "Domains", to: "/domains", icon: "◉" },
  { section: "Manage" },
  { name: "Images", to: "/images", icon: "▤" },
  { name: "Servers", to: "/servers", icon: "⬒" },
  { name: "Logs", to: "/logs", icon: "≡" },
  { section: "Access" },
  { name: "Teams", to: "/teams", icon: "⚑" },
  { name: "Settings", to: "/settings", icon: "⚙" },
];

function isActive(to) {
  if (to === "/") return route.path === "/" || route.path.startsWith("/projects") || route.path.startsWith("/vms");
  return route.path === to;
}

function logout() {
  clearToken();
  router.push({ name: "login" });
}

onMounted(() => connectEvents(() => {}));
onUnmounted(() => disconnectEvents());
</script>

<template>
  <div v-if="authed" class="shell">
    <aside class="sidebar">
      <div class="side-brand" @click="router.push({ name: 'list' })">
        <span class="brand-mark">▣</span><span>Porter</span>
      </div>
      <nav class="side-nav">
        <template v-for="(item, i) in nav" :key="i">
          <div v-if="item.section" class="side-section">{{ item.section }}</div>
          <router-link v-else class="side-link" :class="{ active: isActive(item.to) }" :to="item.to">
            <span class="ico">{{ item.icon }}</span><span>{{ item.name }}</span>
          </router-link>
        </template>
      </nav>
      <div class="side-footer">
        <div class="side-user">
          <span class="dot"></span><span>{{ user }}</span>
        </div>
        <button class="btn btn-primary" style="width: 100%" @click="showNewProject = true">+ New Project</button>
        <button class="btn" style="width: 100%" @click="logout">Sign out</button>
      </div>
    </aside>

    <div class="content">
      <main>
        <router-view />
      </main>
    </div>
  </div>
  <router-view v-else />

  <NewProjectModal v-if="showNewProject" @close="showNewProject = false" @created="(t) => { showNewProject = false; router.push(t); }" />
  <ToastContainer />
</template>
