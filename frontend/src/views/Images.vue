<!-- Style: Porter Harbor Glass operator workspace—calm inventory, explicit
     artifact readiness, and action-first controls with no placeholder images. -->
<script setup>
import { ref, onMounted, computed } from "vue";
import { useRouter } from "vue-router";
import { api, uploadCustomImage } from "../api/client";
import { toast } from "../components/toast";

const router = useRouter();
const images = ref([]);
const base = ref(null);
const loading = ref(true);
const error = ref("");
const q = ref("");
const kind = ref("all");
const busy = ref(false);
const selectedFile = ref(null);
const upload = ref({ name: "", vcpus: 1, mem_mib: 256 });

const filtered = computed(() => {
  let out = images.value;
  if (kind.value !== "all") out = out.filter((image) => (image.kind || image.type || "direct") === kind.value);
  if (q.value.trim()) {
    const search = q.value.toLowerCase();
    out = out.filter((image) => [image.name, image.image, image.description, image.status].filter(Boolean).join(" ").toLowerCase().includes(search));
  }
  return out;
});

const kinds = computed(() => [...new Set(["all", ...images.value.map((image) => image.kind || image.type || "direct")])]);
const baseReady = computed(() => Boolean(base.value?.ready || base.value?.status === "ready" || base.value?.valid));

function fileChanged(event) {
  selectedFile.value = event.target.files?.[0] || null;
}

async function load() {
  loading.value = true;
  error.value = "";
  try {
    const [catalog, readiness] = await Promise.all([api("/images"), api("/images/base/readiness")]);
    images.value = Array.isArray(catalog) ? catalog : catalog?.images || catalog?.items || [];
    base.value = readiness;
  } catch (err) {
    error.value = err.message;
  } finally {
    loading.value = false;
  }
}

async function uploadBundle() {
  if (!selectedFile.value || !upload.value.name.trim()) {
    toast("Choose a bundle and provide a name", "error");
    return;
  }
  busy.value = true;
  try {
    await uploadCustomImage(selectedFile.value, { name: upload.value.name.trim(), vcpus: Number(upload.value.vcpus), mem_mib: Number(upload.value.mem_mib) });
    toast("Custom Firecracker image registered", "success");
    upload.value = { name: "", vcpus: 1, mem_mib: 256 };
    selectedFile.value = null;
    await load();
  } catch (err) {
    toast(err.message, "error");
  } finally {
    busy.value = false;
  }
}

async function deleteImage(image) {
  if (!confirm(`Delete image ${image.name || image.image || image.id}?`)) return;
  busy.value = true;
  try {
    await api(`/images/${encodeURIComponent(image.image || image.reference || image.id)}`, { method: "DELETE" });
    toast("Image deleted", "success");
    await load();
  } catch (err) {
    toast(err.message, "error");
  } finally {
    busy.value = false;
  }
}

async function prune() {
  if (!confirm("Prune unreferenced image catalog entries?")) return;
  busy.value = true;
  try {
    await api("/images/prune", { method: "POST", body: JSON.stringify({}) });
    toast("Image catalog pruned", "success");
    await load();
  } catch (err) {
    toast(err.message, "error");
  } finally {
    busy.value = false;
  }
}

function copyRef(image) {
  const reference = image.image || image.id;
  navigator.clipboard?.writeText(reference).catch(() => {});
  toast(`Copied: ${reference}`);
}

function deployFrom(image) {
  navigator.clipboard?.writeText(image.image || "").catch(() => {});
  router.push({ name: "list" });
  toast("Image reference copied for New Project", "success");
}

onMounted(load);
</script>

<template>
  <div class="page-header">
    <div><div class="page-title">Image Library</div><div class="page-sub">Verified direct Firecracker artifacts available to the control plane.</div></div>
    <div class="detail-actions"><button class="btn btn-sm" :disabled="loading || busy" @click="load">Refresh</button><button class="btn btn-sm" :disabled="busy" @click="prune">Prune catalog</button></div>
  </div>

  <div v-if="error" class="error-box">{{ error }}</div>

  <section class="card image-readiness-card" :class="baseReady ? 'readiness-good' : 'readiness-warn'">
    <div><div class="card-title">Base microVM image</div><div class="page-sub">The default boot contract is a real <span class="mono">vmlinux</span> plus <span class="mono">rootfs.ext4</span> bundle. Porter never treats an OCI reference as a bootable guest.</div></div>
    <div class="readiness-status"><span class="conn-dot" :class="baseReady ? 'live' : 'down'"></span><strong>{{ baseReady ? "Ready" : "Not ready" }}</strong></div>
    <div v-if="base" class="readiness-details mono">{{ base.path || base.rootfs_path || base.image || base.reason || "Readiness data returned by the host" }}</div>
  </section>

  <section class="card" style="margin-bottom:16px">
    <div class="card-title">Upload custom Firecracker bundle</div>
    <div class="page-sub">Upload a ZIP containing regular, non-symlinked <span class="mono">vmlinux</span> and <span class="mono">rootfs.ext4</span> files. The backend verifies SHA-256 digests before registration.</div>
    <div class="resource-form-grid" style="margin-top:14px">
      <div class="field"><label for="image-name">Name</label><input id="image-name" v-model="upload.name" placeholder="porter-base-v1" /></div>
      <div class="field"><label for="image-vcpus">vCPUs</label><input id="image-vcpus" v-model="upload.vcpus" type="number" min="1" /></div>
      <div class="field"><label for="image-memory">Memory (MiB)</label><input id="image-memory" v-model="upload.mem_mib" type="number" min="64" /></div>
      <div class="field field-wide"><label for="image-file">Bundle ZIP</label><input id="image-file" type="file" accept=".zip,.tar,.tgz" @change="fileChanged" /></div>
    </div>
    <div class="detail-actions" style="margin-top:14px"><button class="btn btn-sm btn-primary" :disabled="busy" @click="uploadBundle">{{ busy ? "Uploading…" : "Register bundle" }}</button><span v-if="selectedFile" class="hint">{{ selectedFile.name }}</span></div>
  </section>

  <div class="filter-bar"><input v-model="q" placeholder="Search images…" style="flex:1;max-width:320px" /><div class="seg"><button v-for="value in kinds" :key="value" :class="{ active: kind === value }" @click="kind = value">{{ value }}</button></div><span class="hint">{{ filtered.length }} image(s)</span></div>

  <div v-if="loading && !images.length" class="image-grid"><div v-for="i in 6" :key="i" class="image-card"><div class="skeleton skeleton-line" style="height:110px"></div></div></div>
  <div v-else-if="!images.length" class="empty-state"><strong>No direct images registered.</strong><span>Register a real Firecracker bundle or provision the host base image before creating a bootable project.</span></div>
  <div v-else class="image-grid">
    <div v-for="image in filtered" :key="image.id || image.image" class="image-card">
      <div class="image-card-name">{{ image.name || image.image }}</div>
      <div class="image-card-ref mono">{{ image.image || image.id }}</div>
      <div class="image-card-meta"><span class="image-tag">{{ image.kind || image.type || "direct" }}</span><span class="image-tag">{{ image.status || "registered" }}</span><span v-if="image.mem_mib" class="image-tag">{{ image.mem_mib }} MiB</span><span v-if="image.vcpus" class="image-tag">{{ image.vcpus }} vCPU</span></div>
      <div class="image-card-hint">{{ image.description || "Direct Firecracker boot artifact" }}</div>
      <div v-if="image.rootfs_sha256 || image.kernel_sha256" class="image-card-artifacts mono"><div v-if="image.rootfs_sha256">rootfs sha · {{ image.rootfs_sha256 }}</div><div v-if="image.kernel_sha256">kernel sha · {{ image.kernel_sha256 }}</div></div>
      <div class="detail-actions" style="margin-top:10px"><button class="btn btn-sm" @click="copyRef(image)">Copy ref</button><button class="btn btn-sm btn-primary" @click="deployFrom(image)">Deploy</button><button v-if="image.kind === 'custom' || image.type === 'custom'" class="btn btn-sm btn-danger" :disabled="busy" @click="deleteImage(image)">Delete</button></div>
    </div>
  </div>
</template>
