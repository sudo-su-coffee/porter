<script setup>
import { ref, onMounted, computed } from "vue";
import { useRouter } from "vue-router";
import { api } from "../api/client";
import { toast } from "../components/toast";

const router = useRouter();
const images = ref([]);
const loading = ref(true);
const error = ref("");
const q = ref("");
const kind = ref("all");

// Offline-friendly placeholder for when an image logo URL can't load.
const FALLBACK =
  "data:image/svg+xml;utf8," +
  "%3Csvg xmlns='http://www.w3.org/2000/svg' width='64' height='64'%3E" +
  "%3Crect width='64' height='64' rx='12' fill='%23171a22'/%3E" +
  "%3Ctext x='32' y='42' font-family='sans-serif' font-size='26' fill='%235b8cff' text-anchor='middle'%3E▣%3C/text%3E%3C/svg%3E";

function onLogoError(e) { e.target.src = FALLBACK; }

const filtered = computed(() => {
  let out = images.value;
  if (kind.value !== "all") out = out.filter((i) => (i.kind || "oci") === kind.value);
  if (q.value.trim()) {
    const s = q.value.toLowerCase();
    out = out.filter((i) => [i.name, i.image, i.description].filter(Boolean).join(" ").toLowerCase().includes(s));
  }
  return out;
});

const kinds = computed(() => {
  const set = new Set(["oci"]);
  for (const i of images.value) set.add(i.kind || "oci");
  return [...set];
});

function copyRef(img) {
  const ref = img.image || img.id;
  navigator.clipboard?.writeText(ref).catch(() => {});
  toast(`Copied: ${ref}`);
}

function deployFrom(img) {
  // New Project flow takes the image ref; prefill by routing to the list and
  // letting the user pick New Project → Single Image → paste the ref.
  router.push({ name: "list" });
  navigator.clipboard?.writeText(img.image || "").catch(() => {});
  toast("Ref copied — use it in New Project → Single Image");
}

onMounted(async () => {
  try {
    images.value = (await api("/images")) || [];
  } catch (err) {
    error.value = err.message;
  } finally {
    loading.value = false;
  }
});
</script>

<template>
  <div class="page-header">
    <div>
      <div class="page-title">Image Library</div>
      <div class="page-sub">Reusable microVM images — OCI refs and your uploaded custom microVMs.</div>
    </div>
  </div>

  <div v-if="error" class="error-box">{{ error }}</div>

  <div class="filter-bar">
    <input v-model="q" placeholder="Search images…" style="flex:1; max-width:320px" />
    <div class="seg">
      <button :class="{ active: kind === 'all' }" @click="kind = 'all'">All</button>
      <button v-for="k in kinds" :key="k" :class="{ active: kind === k }" @click="kind = k">{{ k }}</button>
    </div>
    <span class="hint">{{ filtered.length }} image(s)</span>
  </div>

  <div v-if="loading && !images.length" class="image-grid">
    <div v-for="i in 6" :key="i" class="image-card"><div class="skeleton skeleton-line" style="height:110px"></div></div>
  </div>

  <div v-else-if="!images.length" class="empty-state">
    <div style="font-size:15px; margin-bottom:8px">No images in the library</div>
    Upload your own microVM (.zip) via <b>New Project → Custom MicroVM</b>.
  </div>

  <div v-else class="image-grid">
    <div class="image-card" v-for="img in filtered" :key="img.id">
      <div class="image-logo">
        <img :src="img.logo || FALLBACK" :alt="img.name" @error="onLogoError" loading="lazy" />
      </div>
      <div class="image-card-name">{{ img.name }}</div>
      <div class="image-card-ref">{{ img.image }}</div>
      <div class="image-card-meta">
        <span class="image-tag">{{ img.kind || 'oci' }}</span>
        <span class="image-tag">{{ img.mem_mib }}MB</span>
        <span class="image-tag">{{ img.vcpus }}vcpu</span>
        <span v-if="img.rootfs" class="image-tag" :title="img.rootfs">rootfs</span>
        <span v-for="t in img.tags || []" :key="t" class="image-tag">{{ t }}</span>
      </div>
      <div class="image-card-hint">{{ img.description }}</div>
      <div class="detail-actions" style="margin-top:10px">
        <button class="btn btn-sm" @click="copyRef(img)">Copy ref</button>
        <button class="btn btn-sm btn-primary" @click="deployFrom(img)">Deploy</button>
      </div>
    </div>
  </div>
</template>
