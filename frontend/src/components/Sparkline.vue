<script setup>
import { computed } from "vue";

// Lightweight inline SVG sparkline (no chart library — the project has none).
// Given a series of numbers it renders a filled area + a line in a fixed
// viewBox stretched to fill its container (preserveAspectRatio="none").
const props = defineProps({
  data: { type: Array, default: () => [] },
  width: { type: Number, default: 560 },
  height: { type: Number, default: 56 },
});

const points = computed(() => {
  const d = props.data;
  if (!d.length) return [];
  const max = Math.max(...d, 1);
  const n = d.length;
  const step = props.width / Math.max(n - 1, 1);
  const pad = 2;
  const usable = props.height - pad * 2;
  return d.map((v, i) => {
    const x = i * step;
    const y = pad + usable - (v / max) * usable;
    return [x, y];
  });
});

const line = computed(() => points.value.map((p) => p.map((n) => n.toFixed(2)).join(",")).join(" "));

const area = computed(() => {
  if (!points.value.length) return "";
  return `M0,${props.height} L${points.value.map((p) => `${p[0].toFixed(2)},${p[1].toFixed(2)}`).join(" L")} L${props.width},${props.height} Z`;
});
</script>

<template>
  <svg
    v-if="points.length"
    class="sparkline"
    :width="width"
    :height="height"
    :viewBox="`0 0 ${width} ${height}`"
    preserveAspectRatio="none"
  >
    <path class="spark-area" :d="area" />
    <polyline class="spark-line" :points="line" />
  </svg>
</template>
