<!-- Porter dashboard style: permission-aware project administration with no fabricated users or roles. -->
<script setup>
import { onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api } from "../api/client";
import { toast } from "../components/toast";

const route = useRoute();
const router = useRouter();
const projectId = route.params.projectId;
const members = ref([]);
const form = ref({ username: "", role: "member" });
const invite = ref({ username: "", email: "" });
const error = ref("");
const busy = ref(false);

function username(member) { return member.username || member.user?.username || member.user_id || "unknown"; }

async function load() {
  error.value = "";
  try {
    const data = await api(`/projects/${projectId}/members`);
    members.value = Array.isArray(data) ? data : data?.members || [];
  } catch (err) {
    error.value = err.message;
  }
}

async function add() {
  if (!form.value.username.trim()) { error.value = "Username is required."; return; }
  busy.value = true;
  try {
    await api(`/projects/${projectId}/members`, { method: "POST", body: JSON.stringify(form.value) });
    form.value = { username: "", role: "member" };
    toast("Project member added", "success");
    await load();
  } catch (err) { toast(err.message, "error"); } finally { busy.value = false; }
}

async function sendInvite() {
  if (!invite.value.email.trim() && !invite.value.username.trim()) { error.value = "Email or username is required."; return; }
  busy.value = true;
  try {
    await api(`/projects/${projectId}/members/invite`, { method: "POST", body: JSON.stringify(invite.value) });
    invite.value = { username: "", email: "" };
    toast("Invitation recorded", "success");
    await load();
  } catch (err) { toast(err.message, "error"); } finally { busy.value = false; }
}

async function changeRole(member, role) {
  const name = username(member);
  if (name === "unknown" || name === member.user_id) { error.value = "This membership response does not include a username for role updates."; return; }
  busy.value = true;
  try {
    await api(`/projects/${projectId}/members/${encodeURIComponent(name)}`, { method: "PATCH", body: JSON.stringify({ role }) });
    toast("Project role updated", "success");
    await load();
  } catch (err) { toast(err.message, "error"); } finally { busy.value = false; }
}

async function remove(member) {
  const name = username(member);
  if (name === "unknown" || name === member.user_id) { error.value = "This membership response does not include a username for removal."; return; }
  if (!confirm(`Remove ${name} from this project?`)) return;
  busy.value = true;
  try {
    await api(`/projects/${projectId}/members/${encodeURIComponent(name)}`, { method: "DELETE" });
    toast("Project member removed", "success");
    await load();
  } catch (err) { toast(err.message, "error"); } finally { busy.value = false; }
}

onMounted(load);
</script>

<template>
  <a class="back-link" @click="router.push({ name: 'project', params: { id: projectId } })">&larr; Project workspace</a>
  <header class="page-header"><div><div class="page-title">Project members</div><div class="page-sub">Project-scoped membership and roles resolved through persisted Porter RBAC.</div></div><button class="btn btn-sm" :disabled="busy" @click="load">Refresh</button></header>
  <div v-if="error" class="error-box"><span>{{ error }}</span><button class="btn btn-sm" @click="load">Retry</button></div>
  <div class="two-column-grid"><section class="card"><div class="card-title">Add existing user</div><div class="field"><label>Username</label><input v-model="form.username" placeholder="user@example.com" /></div><div class="field"><label>Role</label><select v-model="form.role"><option value="member">Member</option><option value="viewer">Viewer</option><option value="admin">Admin</option></select></div><button class="btn btn-primary btn-sm" :disabled="busy" @click="add">Add member</button></section><section class="card"><div class="card-title">Invite</div><div class="field"><label>Email</label><input v-model="invite.email" type="email" placeholder="builder@example.com" /></div><div class="field"><label>Existing username (optional)</label><input v-model="invite.username" placeholder="builder" /></div><button class="btn btn-sm" :disabled="busy" @click="sendInvite">Record invitation</button></section></div>
  <div v-if="!members.length" class="empty-state"><strong>No project members yet.</strong><span>Porter did not fabricate membership records.</span></div>
  <div v-else class="table-wrap"><table class="data-table"><thead><tr><th>Member</th><th>Role</th><th>Created</th><th>Actions</th></tr></thead><tbody><tr v-for="member in members" :key="member.id || member.user_id"><td class="mono">{{ username(member) }}</td><td><select :value="member.role || 'member'" :disabled="busy || username(member) === member.user_id" @change="changeRole(member, $event.target.value)"><option value="viewer">Viewer</option><option value="member">Member</option><option value="admin">Admin</option><option value="owner">Owner</option></select></td><td class="num">{{ member.created_at ? new Date(member.created_at).toLocaleString() : '—' }}</td><td><button class="btn btn-danger btn-sm" :disabled="busy || username(member) === member.user_id" @click="remove(member)">Remove</button></td></tr></tbody></table></div>
</template>
