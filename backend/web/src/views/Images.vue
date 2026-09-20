<template>
  <div>
    <div class="text-h5 font-weight-bold mb-4">Images &amp; Guest Bases</div>
    <v-card color="surface">
      <v-card-title class="text-subtitle-1 font-weight-bold">Catalog</v-card-title>
      <v-table v-if="images.length" density="comfortable">
        <thead>
          <tr>
            <th>Reference</th>
            <th>Arch</th>
            <th>Kernel</th>
            <th>Rootfs</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="img in images" :key="typeof img === 'string' ? img : img.reference">
            <td>{{ typeof img === 'string' ? img : img.reference }}</td>
            <td class="text-caption">{{ (img && img.arch) || '—' }}</td>
            <td class="text-caption">{{ (img && img.kernel) || (img && img.kernel_image) || '—' }}</td>
            <td class="text-caption">{{ (img && img.rootfs) || (img && img.rootfs_path) || '—' }}</td>
          </tr>
        </tbody>
      </v-table>
      <v-card-text v-else class="text-grey text-caption">No image catalog entries.</v-card-text>
    </v-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { api } from '../api'

const images = ref([])

onMounted(async () => {
  try {
    const list = await api.images()
    images.value = Array.isArray(list) ? list : (list && list.images) || []
  } catch {}
})
</script>