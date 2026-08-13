<script setup>
import { ref, onMounted } from "vue";
import { api } from "../api/client";
import { toast } from "./toast";

const props = defineProps({ projectId: { type: String, required: true } });
const error = ref("");
const rules = ref([]);
const stats = ref(null);
const events = ref([]);
const whitelist = ref([]);
const showAdd = ref(false);
const newRule = ref({ direction: "ingress", action: "deny", proto: "tcp", ports: "", source: "", priority: 100 });
const ruleDetails = ref({});

async function load() {
  error.value = "";
  const base = `/projects/${props.projectId}/firewall`;
  try {
    const [r, s, e, w] = await Promise.allSettled([
      api(`${base}/rules`),
      api(`${base}/stats`),
      api(`${base}/events`),
      api(`${base}/whitelist`, { method: "POST", body: JSON.stringify({}) }),
    ]);
    rules.value = r.status === "fulfilled" && Array.isArray(r.value) ? r.value : [];
    stats.value = s.status === "fulfilled" ? s.value : null;
    events.value = e.status === "fulfilled" ? (e.value?.events || []) : [];
    whitelist.value = w.status === "fulfilled" ? (w.value?.whitelist || []) : [];
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

async function inspect(rule) {
  try { ruleDetails.value = { ...ruleDetails.value, [rule.id]: await api(`/projects/${props.projectId}/firewall/rules/${encodeURIComponent(rule.id)}`) }; }
  catch (e) { toast(e.message, "error"); }
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
          <td style="text-align:right"><button class="btn btn-sm" title="Inspect" @click="inspect(r)">Details</button><button class="icon-btn danger" title="Delete" @click="remove(r)">✕</button><pre v-if="ruleDetails[r.id]" class="settings-json">{{ JSON.stringify(ruleDetails[r.id], null, 2) }}</pre></td>
        </tr>
        <tr v-if="!rules.length"><td colspan="8" class="hint" style="text-align:center; padding:18px">No firewall rules — all traffic allowed by default.</td></tr>
      </tbody>
    </table>
  </div>

  <section class="card" style="margin-top:16px"><div class="card-head"><div class="card-title">Effective whitelist</div><span class="hint">{{ whitelist.length }} source(s)</span></div><div v-if="!whitelist.length" class="empty-state">No allow rules currently contribute to the whitelist.</div><div v-else class="resource-link-grid"><div v-for="source in whitelist" :key="source" class="resource-link"><strong class="mono">{{ source || 'any source' }}</strong><span>allow rule source</span></div></div></section>
  <section class="card" style="margin-top:16px"><div class="card-head"><div class="card-title">Firewall events</div><span class="hint">{{ events.length }} event(s)</span></div><div v-if="!events.length" class="empty-state">No persisted firewall/health events for this project.</div><div v-else class="table-wrap"><table class="data-table"><thead><tr><th>Time</th><th>Event</th><th>Details</th></tr></thead><tbody><tr v-for="(event, index) in events" :key="event.id || index"><td>{{ event.created_at || event.time || '—' }}</td><td class="mono">{{ event.type || event.event || 'health' }}</td><td class="mono">{{ JSON.stringify(event) }}</td></tr></tbody></table></div></section>

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
