<script setup>
import { ref } from "vue";
import { api, uploadCustomImage } from "../api/client";

const emit = defineEmits(["close", "created"]);

const activeTab = ref("single");
const error = ref("");

// Single-image form
const name = ref("");
const image = ref("");
const vcpus = ref(1);
const memMib = ref(256);

// Custom microVM (user-uploaded .zip) form
const customName = ref("");
const customVcpus = ref(1);
const customMemMib = ref(256);
const customFile = ref(null);
const uploading = ref(false);

// Compose form
const composeName = ref("");
const composeYaml = ref(`services:
  api:
    image: custom://myapp-api
    ports:
      - "3000:3000"
    deploy:
      replicas: 2
  worker:
    image: custom://myapp-worker
    depends_on:
      - api`);

// Image library
const images = ref([]);
const imagesLoading = ref(false);
const imagesError = ref("");

function onOverlayClick(e) {
  if (e.target === e.currentTarget) emit("close");
}

async function loadLibrary() {
  activeTab.value = "library";
  if (images.value.length || imagesLoading.value) return;
  imagesLoading.value = true;
  imagesError.value = "";
  try {
    images.value = (await api("/images")) || [];
  } catch (e) {
    imagesError.value = e.message;
  } finally {
    imagesLoading.value = false;
  }
}

// The backend may ship a URL logo (rendered as an <img>) or a plain
// text/emoji glyph (shown inline) so the dashboard still works offline.
function logoIsUrl(img) {
  return typeof img.logo === "string" && /^https?:\/\//i.test(img.logo.trim());
}
function slug(s) {
  return String(s || "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

// Clicking a library card pre-fills the single-image form so the user
// can tweak then deploy. The image's own default ports/env are added.
function pickImage(img) {
  name.value = slug(img.name || img.image);
  image.value = img.image;
  vcpus.value = img.vcpus || 1;
  memMib.value = img.mem_mib || 256;
  activeTab.value = "single";
  error.value = "";
}

async function deploy() {
  error.value = "";
  try {
    if (activeTab.value === "single") {
      if (!image.value.trim()) throw new Error('"image" is required');
      const res = await api("/projects", {
        method: "POST",
        body: JSON.stringify({
          name: name.value.trim(),
          image: image.value.trim(),
          vcpus: parseInt(vcpus.value, 10) || 1,
          mem_mib: parseInt(memMib.value, 10) || 256,
        }),
      });
      const proj = res.project || res;
      emit("created", { name: "project", params: { id: proj.id } });
    } else if (activeTab.value === "custom") {
      if (!customName.value.trim()) throw new Error('"name" is required');
      if (!customFile.value) throw new Error("select your microVM .zip (rootfs.ext4 + vmlinux)");
      uploading.value = true;
      const gi = await uploadCustomImage(customFile.value, {
        name: customName.value.trim(),
        vcpus: parseInt(customVcpus.value, 10) || 1,
        mem_mib: parseInt(customMemMib.value, 10) || 256,
      });
      const res = await api("/projects", {
        method: "POST",
        body: JSON.stringify({
          name: customName.value.trim(),
          image: gi.image,
          vcpus: gi.vcpus,
          mem_mib: gi.mem_mib,
        }),
      });
      const proj = res.project || res;
      emit("created", { name: "project", params: { id: proj.id } });
    } else {
      if (!composeName.value.trim()) throw new Error('"name" is required');
      if (!composeYaml.value.trim()) throw new Error("compose YAML is empty");
      const res = await api("/projects/compose", {
        method: "POST",
        body: JSON.stringify({ name: composeName.value.trim(), compose_yaml: composeYaml.value }),
      });
      const proj = res.project || res;
      emit("created", { name: "project", params: { id: proj.id } });
    }
  } catch (e) {
    error.value = e.message;
  }
}
</script>

<template>
  <div class="modal-overlay" @click="onOverlayClick">
    <div class="modal">
      <div class="modal-title">New Project</div>
      <div class="tabs">
        <button class="tab" :class="{ active: activeTab === 'single' }" @click="activeTab = 'single'">
          Direct Image
        </button>
        <button class="tab" :class="{ active: activeTab === 'compose' }" @click="activeTab = 'compose'">
          Compose Manifest
        </button>
        <button class="tab" :class="{ active: activeTab === 'library' }" @click="loadLibrary">Direct Image Library</button>
        <button class="tab" :class="{ active: activeTab === 'custom' }" @click="activeTab = 'custom'">
          Custom Firecracker Bundle
        </button>
      </div>

      <div v-if="error" class="error-box">{{ error }}</div>

      <div v-show="activeTab === 'single'">
        <div class="field"><label>Name</label><input v-model="name" placeholder="cache" /></div>
        <div class="field"><label>Direct image reference</label><input v-model="image" placeholder="custom://my-app" /></div>
        <div class="hint" style="margin-top:-5px; margin-bottom:14px">Porter boots a kernel + rootfs.ext4 pair directly; registry and container-runtime references are not resolved.</div>
        <div class="field-row">
          <div class="field"><label>vCPUs</label><input v-model="vcpus" type="number" min="1" /></div>
          <div class="field">
            <label>Memory (MiB)</label>
            <input v-model="memMib" type="number" min="128" step="128" />
          </div>
        </div>
      </div>

      <div v-show="activeTab === 'compose'">
        <div class="field"><label>Project name</label><input v-model="composeName" placeholder="my-app" /></div>
        <div class="field">
          <label>compose.yaml (direct image refs)</label>
          <textarea v-model="composeYaml"></textarea>
        </div>
      </div>

      <div v-show="activeTab === 'custom'">
        <div class="field"><label>Name</label><input v-model="customName" placeholder="my-microvm" /></div>
        <div class="field-row">
          <div class="field"><label>vCPUs</label><input v-model="customVcpus" type="number" min="1" /></div>
          <div class="field"><label>Memory (MiB)</label><input v-model="customMemMib" type="number" min="128" step="128" /></div>
        </div>
        <div class="field">
          <label>Firecracker image (.zip — rootfs.ext4 + vmlinux)</label>
          <input type="file" accept=".zip,application/zip" @change="(e) => (customFile = e.target.files[0] || null)" />
          <div class="hint" style="margin-top:6px">Boots your own kernel + rootfs directly with Firecracker.</div>
        </div>
      </div>

      <div v-show="activeTab === 'library'">
        <div v-if="imagesError" class="error-box">{{ imagesError }}</div>
        <div v-else-if="imagesLoading && !images.length" class="image-grid">
          <div v-for="i in 6" :key="i" class="image-card"><div class="skeleton skeleton-line" style="height: 90px"></div></div>
        </div>
        <div v-else-if="!images.length" class="page-sub" style="padding: 12px 0">No images in the library yet.</div>
        <div v-else class="image-grid">
          <button v-for="img in images" :key="img.id" class="image-card" @click="pickImage(img)">
            <div class="image-logo">
              <img v-if="logoIsUrl(img)" :src="img.logo" :alt="img.name" />
              <template v-else>{{ img.logo || (img.name || "?")[0] }}</template>
            </div>
            <div class="image-card-name">{{ img.name }}</div>
            <div class="image-card-ref">{{ img.image }}</div>
            <div class="image-card-meta">
              <span v-if="img.vcpus" class="image-tag">{{ img.vcpus }} vCPU</span>
              <span v-if="img.mem_mib" class="image-tag">{{ img.mem_mib }} MiB</span>
              <span v-for="t in img.tags || []" :key="t" class="image-tag">{{ t }}</span>
            </div>
            <div class="image-card-hint">Click to pre-fill</div>
          </button>
        </div>
      </div>

      <div class="modal-footer">
        <button class="btn" @click="emit('close')">Cancel</button>
        <button
          class="btn btn-primary"
          :disabled="activeTab === 'library' || uploading"
          @click="deploy"
        >
          {{ uploading ? 'Uploading…' : 'Deploy' }}
        </button>
      </div>
    </div>
  </div>
</template>
