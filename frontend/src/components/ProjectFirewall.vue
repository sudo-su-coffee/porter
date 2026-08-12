<script setup>
import { ref, onMounted } from "vue";
import { api } from "../api/client";
import { toast } from "./toast";

const props = defineProps({ projectId: { type: String, required: true } });
const error = ref("");
const rules = ref([]);
const stats = ref(null);
const showAdd = ref(false);
const newRule = ref({ direction: "ingress", action: "deny", proto: "tcp", ports: "", source: "", priority: 100 });

async function load() {
  error.value = "";
  const base = `/projects/${props.projectId}/firewall`;
  try {
    const [r, s] = await Promise.allSettled([
      api(`${base}/rules`),
      api(`${base}/stats`),
    ]);
    rules.value = r.status === "fulfilled" && Array.isArray(r.value) ? r.value : [];
    stats.value = s.status === "fulfilled" ? s.value : null;
  } catch (e) {
    error.value = e.message;
  }
}

async function add() {
  error.value = "";
  try {
    await api(`/projects/${props.projectId}/firewall/rules`, {
      method: "POST",
      body: JSON.stringify(newRule.value),
    });
    newRule.value = { direction: "ingress", action: "deny", proto: "tcp", ports: "", source: "", priority: 100 };
    showAdd.value = false;
    load();
  } catch (e) {
    toast(e.message, "error");
  }
}

async function setActive(rule, active) {
  try {
    await api(`/projects/${props.projectId}/firewall/rules/${rule.id}`, {
      method: "PATCH",
      body: JSON.stringify({ active }),
    });
    load();
  } catch (e) {
    toast(e.message, "error");
  }
}

async function remove(rule) {
  if (!confirm(`Delete firewall rule "${rule.id.slice(0, 8)}"?`)) return;
  await api(`/projects/${props.projectId}/firewall/rules/${rule.id}`, { method: "DELETE" });
  load();
}

onMounted(load);
</script>

<template>
  <div v-if="error" class="error-box">{{ error }}</div>

  <div class="stat-grid" v-if="stats" style="margin-bottom:18px">
    <div class="stat-card"><div class="stat-label">Active rules</div><div class="stat-value">{{ stats.active || 0 }}</div></div>
    <div class="stat-card"><div class="stat-label">Allow</div><div class="stat-value">{{ stats.allowed || 0 }}</div></div>
    <div class="stat-card"><div class="stat-label">Block</div><div class="stat-value">{{ stats.blocked || 0 }}</div></div>
  </div>

  <div class="filter-bar">
    <button class="btn btn-sm btn-primary" @click="showAdd = true">+ Add rule</button>
    <span class="hint">{{ rules.length }} rule(s)</span>
  </div>

  <div class="table-wrap">
    <table class="data-table">
      <thead><tr><th>Dir</th><th>Action</th><th>Proto</th><th>Ports</th><th>Source</th><th>Priority</th><th>State</th><th style="text-align:right">Actions</th></tr></thead>
      <tbody>
        <tr v-for="r in rules" :key="r.id">
          <td class="mono">{{ r.direction }}</td>
          <td><span class="tag" :class="r.action === 'deny' ? 'tag-red' : 'tag-green'">{{ r.action }}</span></td>
          <td class="mono">{{ r.proto || 'any' }}</td>
          <td class="mono">{{ r.ports || 'any' }}</td>
          <td class="mono">{{ r.source || 'any' }}</td>
          <td class="num">{{ r.priority }}</td>
          <td>
            <label class="toggle">
              <input type="checkbox" :checked="r.active" @change="setActive(r, $event.target.checked)" />
              <span></span>
            </label>
          </td>
          <td style="text-align:right"><button class="icon-btn danger" title="Delete" @click="remove(r)">✕</button></td>
        </tr>
        <tr v-if="!rules.length"><td colspan="8" class="hint" style="text-align:center; padding:18px">No firewall rules — all traffic allowed by default.</td></tr>
      </tbody>
    </table>
  </div>

  <div class="modal-overlay" v-if="showAdd" @click.self="showAdd = false">
    <div class="modal" style="width:420px">
      <div class="modal-title">Add firewall rule</div>
      <div class="field"><label>Direction</label>
        <select v-model="newRule.direction">
          <option value="ingress">ingress</option><option value="egress">egress</option>
        </select>
      </div>
      <div class="field"><label>Action</label>
        <select v-model="newRule.action">
          <option value="allow">allow</option><option value="deny">deny</option>
        </select>
      </div>
      <div class="field"><label>Protocol</label>
        <select v-model="newRule.proto">
          <option value="tcp">tcp</option><option value="udp">udp</option><option value="icmp">icmp</option>
        </select>
      </div>
      <div class="field-row">
        <div class="field" style="flex:1"><label>Ports</label><input v-model="newRule.ports" placeholder="443" /></div>
        <div class="field" style="flex:1"><label>Priority</label><input v-model.number="newRule.priority" type="number" /></div>
      </div>
      <div class="field"><label>Source (IP/CIDR, empty = any)</label><input v-model="newRule.source" placeholder="203.0.113.0/24" /></div>
      <div class="modal-footer">
        <button class="btn" @click="showAdd = false">Cancel</button>
        <button class="btn btn-primary" @click="add">Add</button>
      </div>
    </div>
  </div>
</template>