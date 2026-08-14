<script setup>
// Design philosophy: adapt Whatomate's calm operator workspace for Porter—quiet
// navigation, explicit runtime state, and a responsive shell that keeps the
// working surface primary.
import { computed, onMounted, onUnmounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { connectionLive, connectEvents, disconnectEvents } from "./api/events";
import NewProjectModal from "./components/NewProjectModal.vue";
import ToastContainer from "./components/ToastContainer.vue";
import { clearToken, getToken } from "./api/client";

const route = useRoute();
const router = useRouter();
const showNewProject = ref(false);
const isCollapsed = ref(false);
const isMobileMenuOpen = ref(false);
const authed = computed(() => route.name !== "login");
const user = computed(() => (getToken() ? "operator" : ""));
const previewMode = import.meta.env.VITE_PORTER_PREVIEW === "true";

const navSections = [
  { label: "Operate", items: [{ name: "Projects", to: "/projects", icon: "▦" }, { name: "Deployments", to: "/deployments", icon: "◧" }, { name: "Replicas", to: "/replicas", icon: "◌" }, { name: "Domains", to: "/domains", icon: "◉" }] },
  { label: "Observe", items: [{ name: "Traffic", to: "/traffic", icon: "⇄" }, { name: "Analytics", to: "/analytics", icon: "◫" }, { name: "Logs", to: "/logs", icon: "≡" }, { name: "Live events", to: "/events", icon: "✦" }, { name: "System status", to: "/system", icon: "●" }, { name: "Daemon logs", to: "/daemon-logs", icon: "⌁" }] },
  { label: "Manage", items: [{ name: "Images", to: "/images", icon: "▤" }, { name: "Servers", to: "/servers", icon: "⬒" }, { name: "Host overview", to: "/host/overview", icon: "⌂" }, { name: "Host readiness", to: "/host/prerequisites", icon: "✓" }, { name: "Runtime config", to: "/host/runtime", icon: "⌘" }] },
  { label: "Access", items: [{ name: "Teams & RBAC", to: "/teams", icon: "⚑" }, { name: "Organizations", to: "/access/organizations", icon: "◎" }, { name: "Roles", to: "/access/roles", icon: "◇" }, { name: "API keys", to: "/access/api-keys", icon: "⌕" }, { name: "Account", to: "/account", icon: "◌" }, { name: "Feedback", to: "/feedback", icon: "✎" }, { name: "Audit log", to: "/access/audit", icon: "≣" }, { name: "Settings", to: "/settings", icon: "⚙" }] },
];

function isActive(to) {
  if (to === "/projects") return route.path === "/" || route.path === "/projects" || route.path.startsWith("/projects/");
  if (to === "/replicas") return route.path === "/replicas" || route.path.startsWith("/replicas/") || route.path.startsWith("/vms/");
  return route.path === to || route.path.startsWith(`${to}/`);
}

function navigate(to) {
  isMobileMenuOpen.value = false;
  router.push(to);
}

function logout() {
  clearToken();
  isMobileMenuOpen.value = false;
  router.push({ name: "login" });
}

onMounted(() => connectEvents(() => {}));
onUnmounted(() => disconnectEvents());
</script>

<template>
  <div v-if="authed" class="shell" :class="{ 'shell-collapsed': isCollapsed }">
    <header class="mobile-topbar">
      <button class="mobile-brand" type="button" @click="navigate('/projects')" aria-label="Open Porter projects">
        <span class="brand-mark">▣</span><span>Porter</span>
      </button>
      <button class="icon-button" type="button" :aria-expanded="isMobileMenuOpen" aria-label="Toggle navigation" @click="isMobileMenuOpen = !isMobileMenuOpen">
        {{ isMobileMenuOpen ? "×" : "☰" }}
      </button>
    </header>

    <div v-if="isMobileMenuOpen" class="shell-scrim" @click="isMobileMenuOpen = false" />

    <aside class="sidebar" :class="{ 'sidebar-open': isMobileMenuOpen }" aria-label="Porter workspace navigation">
      <div class="side-brand-row">
        <button class="side-brand" type="button" @click="navigate('/projects')">
          <span class="brand-mark">▣</span><span v-if="!isCollapsed" class="brand-wordmark">Porter</span>
        </button>
        <button class="collapse-button" type="button" :aria-label="isCollapsed ? 'Expand sidebar' : 'Collapse sidebar'" @click="isCollapsed = !isCollapsed">
          {{ isCollapsed ? "›" : "‹" }}
        </button>
      </div>

      <nav class="side-nav" aria-label="Primary navigation">
        <section v-for="section in navSections" :key="section.label" class="nav-section">
          <div v-if="!isCollapsed" class="side-section">{{ section.label }}</div>
          <div v-else class="side-section-rule" aria-hidden="true" />
          <router-link v-for="item in section.items" :key="item.to" class="side-link" :class="{ active: isActive(item.to) }" :to="item.to" :aria-current="isActive(item.to) ? 'page' : undefined" @click="isMobileMenuOpen = false">
            <span class="ico" aria-hidden="true">{{ item.icon }}</span><span v-if="!isCollapsed">{{ item.name }}</span>
          </router-link>
        </section>
      </nav>

      <div class="side-footer">
        <div class="side-user" :title="user"><span class="dot" aria-hidden="true"></span><span v-if="!isCollapsed">{{ user }}</span></div>
        <button class="btn btn-primary" :class="{ 'btn-icon-only': isCollapsed }" type="button" @click="showNewProject = true"><span aria-hidden="true">+</span><span v-if="!isCollapsed">New Project</span></button>
        <button class="btn" :class="{ 'btn-icon-only': isCollapsed }" type="button" @click="logout"><span aria-hidden="true">↪</span><span v-if="!isCollapsed">Sign out</span></button>
      </div>
    </aside>

    <div class="content">
      <div class="workspace-bar">
        <div><span class="workspace-kicker">PORTER / CONTROL PLANE</span><span class="workspace-route">{{ route.name || "workspace" }}</span><span v-if="previewMode" class="preview-badge">DEVELOPMENT PREVIEW</span></div>
        <div class="workspace-tools">
          <div class="workspace-status"><span class="conn-dot" :class="connectionLive ? 'live' : 'down'" aria-hidden="true"></span><span>{{ connectionLive ? "Live events" : "Reconnecting" }}</span></div>
          <button class="workspace-launch" type="button" @click="showNewProject = true"><span aria-hidden="true">+</span> New project</button>
        </div>
      </div>
      <main><router-view /></main>
    </div>
  </div>
  <router-view v-else />
  <NewProjectModal v-if="showNewProject" @close="showNewProject = false" @created="(to) => { showNewProject = false; router.push(to); }" />
  <ToastContainer />
</template>
