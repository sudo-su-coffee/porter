<script setup>
import { ref, onMounted } from "vue";
import { api } from "../api/client";
import { toast } from "../components/toast";

const images = ref([]);
const loading = ref(true);

// Offline-friendly placeholder for when an image logo URL can't load
// (e.g. the catalog references a remote URL and the host is offline).
const FALLBACK =
  "data:image/svg+xml;utf8," +
  "%3Csvg xmlns='http://www.w3.org/2000/svg' width='64' height='64'%3E" +
  "%3Crect width='64' height='64' rx='12' fill='%23171a22'/%3E" +
  "%3Ctext x='32' y='42' font-family='sans-serif' font-size='26' fill='%235b8cff' text-anchor='middle'%3E▣%3C/text%3E%3C/svg%3E";

function onLogoError(e) { e.target.src = FALLBACK; }

onMounted(async () => {
  try {
    images.value = (await api("/images")) || [];
  } catch (err) {
    toast(err.message, "error");
    images.value = [];
  } finally {
    loading.value = false;
  }
});
</script>

<template>
  <header class="page-header">
    <div>
      <div class="page-title">Image Library</div>
      <div class="page-sub">Reusable microVM images — pick one to deploy a starter project.</div>
    </div>
  </header>

  <p v-if="loading" class="page-sub">Loading catalog…</p>
  <p v-else-if="!images.length" class="page-sub">No images found in the catalog (vms/images/*.json).</p>

  <div class="image-grid" v-else>
    <div class="image-card" v-for="img in images" :key="img.id">
      <div class="image-logo">
        <img :src="img.logo || FALLBACK" :alt="img.name" @error="onLogoError" loading="lazy" />
      </div>
      <div class="image-card-name">{{ img.name }}</div>
      <div class="image-card-ref">{{ img.image }}</div>
      <div class="image-card-meta">
        <span class="image-tag">{{ img.type }}</span>
        <span class="image-tag">{{ img.mem_mib }}MB</span>
        <span class="image-tag">{{ img.vcpus }}vcpu</span>
      </div>
      <div class="image-card-hint">{{ img.description }}</div>
    </div>
  </div>
</template>