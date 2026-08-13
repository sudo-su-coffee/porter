<!-- Harbor Glass / Whatomate-inspired Porter workspace: dense operator typography, explicit stream state, no chat-specific UI. -->
<script setup>
import { computed, onMounted, onUnmounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { getToken } from "../api/client";

const route = useRoute();
const router = useRouter();
const lines = ref([]);
const status = ref("connecting");
const error = ref("");
const connected = ref(false);
let controller;

const title = computed(() => route.meta.streamTitle || "Live logs");
const endpoint = computed(() => String(route.meta.streamEndpoint || route.path).replace(/:([A-Za-z0-9_]+)/g, (_, key) => encodeURIComponent(route.params[key] || "")));
const backTarget = computed(() => String(route.meta.back || "/").replace(/:([A-Za-z0-9_]+)/g, (_, key) => encodeURIComponent(route.params[key] || "")));

function consumeEvent(block) {
  const rows = block.split("\n");
  const event = rows.find((row) => row.startsWith("event:"))?.slice(6).trim() || "message";
  const raw = rows.filter((row) => row.startsWith("data:")).map((row) => row.slice(5).trim()).join("\n");
  if (!raw) return;
  if (event === "end") {
    status.value = "complete";
    connected.value = false;
    return;
  }
  try {
    const payload = JSON.parse(raw);
    lines.value = Array.isArray(payload.lines) ? payload.lines : lines.value;
    status.value = payload.status || "live";
  } catch (_) {
    lines.value = [...lines.value, raw];
  }
}

async function connect() {
  controller?.abort();
  controller = new AbortController();
  error.value = "";
  status.value = "connecting";
  try {
    const res = await fetch(endpoint.value, { headers: { Authorization: `Bearer ${getToken()}` }, signal: controller.signal });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    if (!res.body) throw new Error("Streaming is not supported by this browser");
    connected.value = true;
    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";
    while (true) {
      const { value, done } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      let boundary = buffer.indexOf("\n\n");
      while (boundary >= 0) {
        consumeEvent(buffer.slice(0, boundary));
        buffer = buffer.slice(boundary + 2);
        boundary = buffer.indexOf("\n\n");
      }
    }
  } catch (e) {
    if (e.name !== "AbortError") {
      error.value = e.message;
      status.value = "error";
      connected.value = false;
    }
  }
}

function goBack() {
  if (backTarget.value === "/") router.push("/");
  else router.push(backTarget.value);
}

onMounted(connect);
onUnmounted(() => controller?.abort());
</script>

<template>
  <a class="back-link" @click="goBack">&larr; Back to workspace</a>
  <header class="page-header">
    <div><div class="page-title">{{ title }}</div><div class="page-sub mono">{{ endpoint }} · {{ connected ? "connected" : status }}</div></div>
    <div class="detail-actions"><button class="btn btn-sm" @click="connect">Reconnect</button></div>
  </header>
  <div v-if="error" class="error-box">{{ error }}</div>
  <div class="notice-box">This stream reflects real persisted runtime/build log lines. It does not generate application output when the guest or build worker is idle.</div>
  <div class="terminal stream-terminal"><div v-for="(line, index) in lines" :key="index" class="tline">{{ typeof line === "string" ? line : JSON.stringify(line) }}</div><div v-if="!lines.length" class="t-empty">Waiting for real log lines…</div></div>
</template>
