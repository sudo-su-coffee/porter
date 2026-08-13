<script setup>
import { computed, ref, onMounted } from "vue";
import { useRoute } from "vue-router";
import { api, getOrgId, setOrgId } from "../api/client";

const route = useRoute();
const tab = ref(route.meta.accessTab || "orgs");
const error = ref("");
const orgs = ref([]);
const groups = ref([]);
const projects = ref([]);
const groupProjects = ref({});
const members = ref([]);
const users = ref([]);
const apiKeys = ref([]);
const events = ref([]);
const audit = ref([]);
const orgDetail = ref(null);
const defaultOrg = ref(null);
const groupDetails = ref({});

const newOrg = ref("");
const newGroup = ref("");
const newUser = ref({ username: "", password: "", role: "member" });
const newMember = ref({ username: "", role: "member" });
const newKeyName = ref("");
const keyToken = ref("");

const roles = ref([]);
const permissions = ref([]);
const rolePerms = ref({});
const roleDetails = ref({});
const newRole = ref({ id: "", name: "", description: "" });
const activeOrgId = ref(getOrgId());
const roleChoices = computed(() => roles.value.length ? roles.value : [{ id: "member" }, { id: "admin" }, { id: "owner" }, { id: "super_admin" }]);

const TABS = ["orgs", "groups", "members", "users", "apikeys", "roles", "events"];

async function load() {
  error.value = "";
  try {
    const [o, g, p, m, u, k, ev, au, od, current, def] = await Promise.allSettled([
      api("/orgs"), api("/groups"), api("/projects"), api("/orgs/members"), api("/users"), api("/users/me/api-keys"), api("/orgs/events"), api("/orgs/audit"), api("/org"), api("/orgs/current"), api("/orgs/default"),
    ]);
		orgs.value = o.status === "fulfilled" ? o.value || [] : [];
		if (!activeOrgId.value && orgs.value.length) {
			activeOrgId.value = (orgs.value.find((org) => org.is_default) || orgs.value[0]).id;
			setOrgId(activeOrgId.value);
		}
			groups.value = g.status === "fulfilled" && g.value ? (g.value.groups || g.value) : [];
    projects.value = p.status === "fulfilled" ? (p.value || []) : [];
    members.value = m.status === "fulfilled" && m.value ? (m.value.members || []) : [];
    users.value = u.status === "fulfilled" ? u.value || [] : [];
			apiKeys.value = k.status === "fulfilled" ? k.value || [] : [];
	    events.value = ev.status === "fulfilled" && ev.value ? (ev.value.events || ev.value) : [];
    audit.value = au.status === "fulfilled" ? (au.value?.events || au.value?.audit || au.value || []) : [];
    orgDetail.value = od.status === "fulfilled" ? od.value : (current.status === "fulfilled" ? current.value : null);
    defaultOrg.value = def.status === "fulfilled" ? def.value : null;
    for (const group of groups.value) loadGroupProjects(group);
  } catch (e) {
    error.value = e.message;
	}
}

async function loadGroupProjects(group) {
  try { groupProjects.value = { ...groupProjects.value, [group.id]: (await api(`/groups/${group.id}/projects`))?.projects || [] }; }
  catch (_) { groupProjects.value = { ...groupProjects.value, [group.id]: [] }; }
}

async function inspectGroup(group) {
  try { groupDetails.value = { ...groupDetails.value, [group.id]: await api(`/groups/${encodeURIComponent(group.id)}`) }; }
  catch (err) { error.value = err.message; }
}

async function updateGroup(group) {
  const name = prompt("Group name", group.name || "");
  if (name === null) return;
  try { await api(`/groups/${group.id}`, { method: "PATCH", body: JSON.stringify({ name }) }); await load(); }
  catch (err) { error.value = err.message; }
}

async function deleteGroup(group) {
  if (!confirm(`Delete group ${group.name}?`)) return;
  try { await api(`/groups/${group.id}`, { method: "DELETE" }); await load(); }
  catch (err) { error.value = err.message; }
}

async function addProjectToGroup(group) {
  const projectId = prompt("Project ID to add", "");
  if (!projectId) return;
  try { await api(`/groups/${group.id}/projects/${encodeURIComponent(projectId)}`, { method: "POST" }); await loadGroupProjects(group); }
  catch (err) { error.value = err.message; }
}

async function removeProjectFromGroup(group, project) {
  if (!confirm(`Remove ${project.name || project.id} from ${group.name}?`)) return;
  try { await api(`/groups/${group.id}/projects/${encodeURIComponent(project.id)}`, { method: "DELETE" }); await loadGroupProjects(group); }
  catch (err) { error.value = err.message; }
}

async function patchOrg() {
  const name = prompt("Organization name", orgDetail.value?.name || "");
  if (name === null) return;
  try { orgDetail.value = await api("/orgs/current", { method: "PATCH", body: JSON.stringify({ name }) }); await load(); }
  catch (err) { error.value = err.message; }
}

async function patchLegacyOrg() {
  const name = prompt("Legacy organization name", orgDetail.value?.name || "");
  if (name === null) return;
  try { orgDetail.value = await api("/org", { method: "PATCH", body: JSON.stringify({ name }) }); await load(); }
  catch (err) { error.value = err.message; }
}

async function transferOrg() {
  const orgId = prompt("Destination organization ID", "");
  if (!orgId) return;
  try { await api("/orgs/transfer", { method: "POST", body: JSON.stringify({ org_id: orgId }) }); await load(); }
  catch (err) { error.value = err.message; }
}

async function deleteCurrentOrg() {
  if (!confirm("Delete the current organization? This is destructive and may require moving projects first.")) return;
  try { await api("/orgs/current", { method: "DELETE" }); setOrgId(""); activeOrgId.value = ""; await load(); }
  catch (err) { error.value = err.message; }
}

function selectOrg() {
	setOrgId(activeOrgId.value);
	load();
}

async function createOrg() {
  if (!newOrg.value.trim()) return;
  await api("/orgs", { method: "POST", body: JSON.stringify({ name: newOrg.value.trim() }) });
  newOrg.value = "";
  load();
}
async function createGroup() {
  if (!newGroup.value.trim()) return;
  await api("/groups", { method: "POST", body: JSON.stringify({ name: newGroup.value.trim() }) });
  newGroup.value = "";
  load();
}
async function addMember() {
	if (!newMember.value.username.trim()) return;
	await api("/orgs/members", { method: "POST", body: JSON.stringify(newMember.value) });
	newMember.value = { username: "", role: "member" };
	load();
}

async function updateMember(member) {
	await api(`/orgs/members/${encodeURIComponent(member.username)}`, { method: "PATCH", body: JSON.stringify({ role: member.role }) });
	load();
}

async function removeMember(member) {
	if (!confirm(`Remove ${member.username} from this organization?`)) return;
	await api(`/orgs/members/${encodeURIComponent(member.username)}`, { method: "DELETE" });
	load();
}
async function addUser() {
  if (!newUser.value.username.trim() || !newUser.value.password) return;
  await api("/users", { method: "POST", body: JSON.stringify(newUser.value) });
  newUser.value = { username: "", password: "", role: "member" };
  load();
}
async function addKey() {
	if (!newKeyName.value.trim()) return;
	const res = await api("/users/me/api-keys", { method: "POST", body: JSON.stringify({ name: newKeyName.value.trim() }) });
	keyToken.value = res.token;
	newKeyName.value = "";
	load();
}

async function deleteKey(key) {
	if (!confirm(`Revoke API key "${key.name}"?`)) return;
	await api(`/users/me/api-keys/${key.id}`, { method: "DELETE" });
	load();
}

async function deleteUser(user) {
	if (!confirm(`Delete user "${user.username}"? This removes the account, not just one membership.`)) return;
	await api(`/users/${encodeURIComponent(user.username)}`, { method: "DELETE" });
	load();
}

async function loadRoles() {
  try {
    const [r, p] = await Promise.allSettled([api("/roles"), api("/permissions")]);
    roles.value = r.status === "fulfilled" ? r.value || [] : [];
    permissions.value = p.status === "fulfilled" ? p.value || [] : [];
  } catch (_) {}
}

function rolePermsOf(roleId) {
  return rolePerms.value[roleId] || [];
}

async function createRole() {
  if (!newRole.value.id.trim() || !newRole.value.name.trim()) return;
  await api("/roles", { method: "POST", body: JSON.stringify(newRole.value) });
  newRole.value = { id: "", name: "", description: "" };
  loadRoles();
}

async function editRole(r) {
  await api(`/roles/${r.id}`, { method: "PATCH", body: JSON.stringify({ name: r.name, description: r.description }) });
  loadRoles();
}

async function deleteRole(r) {
  if (!confirm(`Delete role "${r.name}"? Members assigned to it keep their memberships.`)) return;
  await api(`/roles/${r.id}`, { method: "DELETE" });
  loadRoles();
}

async function togglePerm(r, permId) {
  const has = rolePermsOf(r.id).includes(permId);
  const path = `/roles/${r.id}/permissions/${permId}`;
  await api(path, { method: has ? "DELETE" : "POST" });
  refreshRolePerms(r.id);
}

async function refreshRolePerms(roleId) {
  try {
    const res = await api(`/roles/${roleId}/permissions`);
    rolePerms.value[roleId] = (res && res.permissions) || [];
  } catch (_) {
    rolePerms.value[roleId] = [];
  }
}

async function inspectRole(role) {
  try { roleDetails.value = { ...roleDetails.value, [role.id]: await api(`/roles/${encodeURIComponent(role.id)}`) }; }
  catch (err) { error.value = err.message; }
}

async function replacePermissions(role) {
  const value = prompt("Permission IDs as JSON array", JSON.stringify(rolePermsOf(role.id)));
  if (value === null) return;
  let selected;
  try { selected = JSON.parse(value); }
  catch (_) { error.value = "Permissions must be a JSON array."; return; }
  if (!Array.isArray(selected)) { error.value = "Permissions must be a JSON array."; return; }
  try { await api(`/roles/${encodeURIComponent(role.id)}/permissions`, { method: "PUT", body: JSON.stringify({ permissions: selected }) }); await refreshRolePerms(role.id); }
  catch (err) { error.value = err.message; }
}


onMounted(() => { load(); loadRoles(); });
</script>

<template>
  <div class="page-header">
    <div>
      <div class="page-title">Teams &amp; Access</div>
      <div class="page-sub">Orgs, groups, members, users and API keys (RBAC)</div>
    </div>
  </div>

	<div v-if="error" class="error-box">{{ error }}</div>
	<div v-if="orgs.length" class="filter-bar"><label class="hint">Active organization</label><select v-model="activeOrgId" @change="selectOrg"><option v-for="org in orgs" :key="org.id" :value="org.id">{{ org.name }}{{ org.is_default ? ' · default' : '' }}</option></select></div>

  <div class="seg" style="margin-bottom:18px">
    <button v-for="t in TABS" :key="t" :class="{ active: tab === t }" @click="tab = t">{{ t }}</button>
  </div>

  <!-- Orgs -->
  <div v-if="tab === 'orgs'">
    <div class="filter-bar">
      <input v-model="newOrg" placeholder="New org name" @keyup.enter="createOrg" />
      <button class="btn btn-sm btn-primary" @click="createOrg">+ Create</button>
      <button class="btn btn-sm" @click="patchOrg">Edit current organization</button>
      <button class="btn btn-sm" @click="patchLegacyOrg">Edit legacy org contract</button>
      <button class="btn btn-sm" @click="transferOrg">Transfer organization</button>
      <button class="btn btn-sm btn-danger" @click="deleteCurrentOrg">Delete current organization</button>
    </div>
    <section v-if="orgDetail" class="card" style="margin-bottom:16px"><div class="card-title">Current organization</div><pre class="settings-json">{{ JSON.stringify(orgDetail, null, 2) }}</pre><div v-if="defaultOrg" class="hint">Default organization contract: <span class="mono">{{ defaultOrg.name || defaultOrg.id || JSON.stringify(defaultOrg) }}</span></div></section>
    <div class="table-wrap">
      <table class="data-table">
        <thead><tr><th>Name</th><th>Default</th><th>Created</th></tr></thead>
        <tbody>
          <tr v-for="o in orgs" :key="o.id">
            <td>{{ o.name }}</td>
            <td><span v-if="o.is_default" class="tag tag-green">default</span></td>
            <td class="num muted">{{ new Date(o.created_at).toLocaleDateString() }}</td>
          </tr>
          <tr v-if="!orgs.length"><td colspan="3" class="hint" style="text-align:center; padding:18px">No orgs yet.</td></tr>
        </tbody>
      </table>
    </div>
  </div>

  <!-- Groups -->
  <div v-if="tab === 'groups'">
    <div class="filter-bar">
      <input v-model="newGroup" placeholder="Group name" @keyup.enter="createGroup" />
      <button class="btn btn-sm btn-primary" @click="createGroup">+ Create</button>
    </div>
    <div class="table-wrap">
      <table class="data-table">
        <thead><tr><th>Name</th><th>Org</th><th>Projects</th><th>Actions</th></tr></thead>
        <tbody>
          <tr v-for="g in groups" :key="g.id"><td>{{ g.name }}</td><td class="mono muted">{{ g.org_id }}</td><td><span class="hint">{{ (groupProjects[g.id] || []).length }} project(s)</span><div v-for="project in (groupProjects[g.id] || [])" :key="project.id" class="hint mono">{{ project.name || project.id }} <button class="btn btn-sm" @click="removeProjectFromGroup(g, project)">Remove</button></div><pre v-if="groupDetails[g.id]" class="settings-json">{{ JSON.stringify(groupDetails[g.id], null, 2) }}</pre></td><td><button class="btn btn-sm" @click="inspectGroup(g)">Details</button><button class="btn btn-sm" @click="updateGroup(g)">Edit</button><button class="btn btn-sm" @click="addProjectToGroup(g)">Add project</button><button class="btn btn-danger btn-sm" @click="deleteGroup(g)">Delete</button></td></tr>
          <tr v-if="!groups.length"><td colspan="4" class="hint" style="text-align:center; padding:18px">No groups yet.</td></tr>
        </tbody>
      </table>
    </div>
  </div>

  <!-- Members -->
  <div v-if="tab === 'members'">
    <div class="filter-bar">
      <input v-model="newMember.username" placeholder="Username" />
		<select v-model="newMember.role"><option v-for="role in roleChoices" :key="role.id" :value="role.id">{{ role.id }}</option></select>
      <button class="btn btn-sm btn-primary" @click="addMember">+ Invite</button>
    </div>
    <div class="table-wrap">
      <table class="data-table">
        <thead><tr><th>User</th><th>Role</th></tr></thead>
        <tbody>
		<tr v-for="(m, i) in members" :key="m.user_id || i"><td>{{ m.username }}</td><td><select v-model="m.role"><option v-for="role in roleChoices" :key="role.id" :value="role.id">{{ role.id }}</option></select></td><td><button class="btn btn-sm" @click="updateMember(m)">Save</button><button class="btn btn-sm" @click="removeMember(m)">Remove</button></td></tr>
          <tr v-if="!members.length"><td colspan="2" class="hint" style="text-align:center; padding:18px">No members yet.</td></tr>
        </tbody>
      </table>
    </div>
  </div>

  <!-- Users -->
  <div v-if="tab === 'users'">
    <div class="filter-bar">
      <input v-model="newUser.username" placeholder="Username" />
      <input v-model="newUser.password" type="password" placeholder="Password" />
		<select v-model="newUser.role"><option v-for="role in roleChoices" :key="role.id" :value="role.id">{{ role.id }}</option></select>
      <button class="btn btn-sm btn-primary" @click="addUser">+ Add User</button>
    </div>
    <div class="table-wrap">
      <table class="data-table">
        <thead><tr><th>Username</th><th>Role</th><th>Created</th></tr></thead>
        <tbody>
          <tr v-for="u in users" :key="u.id">
            <td>{{ u.username }}</td>
            <td><span class="tag" :class="u.role === 'admin' ? 'tag-accent' : ''">{{ u.role }}</span></td>
			<td class="num muted">{{ new Date(u.created_at).toLocaleDateString() }}</td><td><button class="btn btn-sm" @click="deleteUser(u)">Delete</button></td>
          </tr>
          <tr v-if="!users.length"><td colspan="3" class="hint" style="text-align:center; padding:18px">No users yet.</td></tr>
        </tbody>
      </table>
    </div>
  </div>

  <!-- API keys -->
  <div v-if="tab === 'apikeys'">
    <div class="filter-bar">
      <input v-model="newKeyName" placeholder="Key name (e.g. ci)" @keyup.enter="addKey" />
      <button class="btn btn-sm btn-primary" @click="addKey">+ Create</button>
    </div>
    <div v-if="keyToken" class="terminal" style="margin-bottom:14px">
      <div class="tline">Token: <b>{{ keyToken }}</b> — copy now, it won't be shown again.</div>
    </div>
    <div class="table-wrap">
      <table class="data-table">
        <thead><tr><th>Name</th><th>Created</th></tr></thead>
        <tbody>
		<tr v-for="k in apiKeys" :key="k.id"><td>{{ k.name }}</td><td class="num muted">{{ new Date(k.created_at).toLocaleDateString() }}</td><td><button class="btn btn-sm" @click="deleteKey(k)">Revoke</button></td></tr>
          <tr v-if="!apiKeys.length"><td colspan="2" class="hint" style="text-align:center; padding:18px">No API keys yet.</td></tr>
        </tbody>
      </table>
    </div>
  </div>

  <!-- Roles & Permissions -->
  <div v-if="tab === 'roles'">
    <div class="filter-bar">
      <input v-model="newRole.id" placeholder="role id (e.g. deployer)" style="width:150px" />
      <input v-model="newRole.name" placeholder="Name" />
      <input v-model="newRole.description" placeholder="Description" style="flex:1; max-width:280px" />
      <button class="btn btn-sm btn-primary" @click="createRole">+ Create Role</button>
    </div>
    <div class="table-wrap">
      <table class="data-table">
        <thead><tr><th>Role</th><th>Description</th><th>Permissions</th><th style="text-align:right">Actions</th></tr></thead>
        <tbody>
          <tr v-for="r in roles" :key="r.id">
            <td><span class="mono">{{ r.id }}</span> <span class="tag tag-accent">{{ r.name }}</span></td>
            <td class="hint">{{ r.description }}</td>
            <td class="hint">{{ rolePermsOf(r.id).length || 0 }} grant(s)</td>
            <td style="text-align:right">
              <div class="actions">
                <button class="icon-btn" title="Details" @click="inspectRole(r)">⌕</button>
                <button class="icon-btn" title="Edit" @click="editRole(r)">✎</button>
                <button class="icon-btn" title="Replace permissions" @click="replacePermissions(r)">✓</button>
                <button class="icon-btn danger" title="Delete" @click="deleteRole(r)">✕</button>
              </div>
            </td>
          </tr>
          <tr v-if="!roles.length"><td colspan="4" class="hint" style="text-align:center; padding:18px">No roles yet — create one to grant fine-grained permissions.</td></tr>
        </tbody>
      </table>
    </div>

    <div v-for="r in roles" :key="`detail-${r.id}`" v-if="roleDetails[r.id]" class="card" style="margin-top:10px"><div class="card-title">{{ r.name }} detail</div><pre class="settings-json">{{ JSON.stringify(roleDetails[r.id], null, 2) }}</pre></div>
    <div class="page-sub" style="margin:20px 0 10px">Permissions by role</div>
    <div class="card" v-for="r in roles" :key="r.id">
      <div class="page-sub" style="margin-bottom:10px">
        <b>{{ r.name }}</b>
        <button class="btn btn-sm" style="float:right" @click="refreshRolePerms(r.id)">Reload perms</button>
      </div>
      <div class="perm-grid">
        <label class="perm-chip" v-for="p in permissions" :key="p.id">
          <input type="checkbox" :checked="rolePermsOf(r.id).includes(p.id)" @change="togglePerm(r, p.id)" />
          <span>{{ p.id }}</span>
        </label>
        <div v-if="!permissions.length" class="hint">No permission catalog returned.</div>
      </div>
    </div>
  </div>

  <!-- Organization events -->
  <div v-if="tab === 'events'">
    <div class="filter-bar"><span class="hint">Persisted organization health and security events.</span><button class="btn btn-sm" @click="load">Refresh</button></div>
    <div v-if="!events.length" class="empty-state"><strong>No organization events.</strong><span>The backend returned no persisted events for the active organization.</span></div>
    <div v-else class="table-wrap"><table class="data-table"><thead><tr><th>Time</th><th>Event</th></tr></thead><tbody><tr v-for="(event, index) in events" :key="event.id || index"><td class="num muted">{{ event.ts || event.created_at ? new Date(event.ts || event.created_at).toLocaleString() : '—' }}</td><td class="mono">{{ event.event || event.message || JSON.stringify(event) }}</td></tr></tbody></table></div>
    <div class="page-sub" style="margin:20px 0 10px">Organization audit endpoint</div><pre class="settings-json">{{ JSON.stringify(audit, null, 2) }}</pre>
  </div>
</template>
