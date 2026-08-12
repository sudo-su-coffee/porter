<script setup>
import { ref, onMounted } from "vue";
import { api } from "../api/client";
import { toast } from "../components/toast";

const feedbackList = ref([]);
const loading = ref(true);
const error = ref("");
const showAdd = ref(false);
const newFeedback = ref({ subject: "", message: "", category: "general", project_id: "" });

async function load() {
  error.value = "";
  try {
    const list = await api("/feedback");
    feedbackList.value = Array.isArray(list) ? list : [];
  } catch (e) {
    error.value = e.message;
  } finally {
    loading.value = false;
  }
}

async function submitFeedback() {
  if (!newFeedback.value.message.trim()) {
    toast("Feedback message is required", "error");
    return;
  }
  try {
    await api("/feedback", {
      method: "POST",
      body: JSON.stringify(newFeedback.value),
    });
    toast("Feedback submitted successfully!", "success");
    newFeedback.value = { subject: "", message: "", category: "general", project_id: "" };
    showAdd.value = false;
    load();
  } catch (e) {
    toast(e.message, "error");
  }
}

onMounted(load);
</script>

<template>
  <header class="page-header">
    <div>
      <div class="page-title">Feedback Hub</div>
      <div class="page-sub">Send feedback to the system operator and review submission history.</div>
    </div>
  </header>

  <div v-if="error" class="error-box">{{ error }}</div>

  <div class="filter-bar">
    <button class="btn btn-sm btn-primary" @click="showAdd = true">+ New Feedback</button>
    <span class="hint">{{ feedbackList.length }} submission(s)</span>
  </div>

  <p v-if="loading" class="page-sub">Loading feedback history…</p>

  <div v-else class="table-wrap">
    <table class="data-table">
      <thead>
        <tr>
          <th>Category</th>
          <th>Subject</th>
          <th>Message</th>
          <th>User</th>
          <th>Project ID</th>
          <th>Submitted At</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="f in feedbackList" :key="f.id">
          <td>
            <span class="tag" :class="f.category === 'bug' ? 'tag-red' : f.category === 'feature' ? 'tag-green' : 'tag-accent'">
              {{ f.category }}
            </span>
          </td>
          <td><b>{{ f.subject || 'No Subject' }}</b></td>
          <td style="max-width: 300px; word-break: break-word;">{{ f.message }}</td>
          <td class="mono">{{ f.username }}</td>
          <td class="mono muted">{{ f.project_id || '—' }}</td>
          <td class="num muted">{{ new Date(f.created_at).toLocaleString() }}</td>
        </tr>
        <tr v-if="!feedbackList.length">
          <td colspan="6" class="hint" style="text-align:center; padding:18px">
            No feedback submissions found.
          </td>
        </tr>
      </tbody>
    </table>
  </div>

  <div class="modal-overlay" v-if="showAdd" @click.self="showAdd = false">
    <div class="modal" style="width:450px">
      <div class="modal-title">Send Feedback</div>
      <div class="field">
        <label>Category</label>
        <select v-model="newFeedback.category">
          <option value="general">General</option>
          <option value="bug">Bug Report</option>
          <option value="feature">Feature Request</option>
        </select>
      </div>
      <div class="field">
        <label>Subject</label>
        <input v-model="newFeedback.subject" placeholder="e.g. Volume mounting issue" />
      </div>
      <div class="field">
        <label>Message *</label>
        <textarea v-model="newFeedback.message" rows="4" placeholder="Type your feedback here..." style="width: 100%; box-sizing: border-box;"></textarea>
      </div>
      <div class="field">
        <label>Project ID (optional)</label>
        <input v-model="newFeedback.project_id" placeholder="Associated project UUID" />
      </div>
      <div class="modal-footer">
        <button class="btn" @click="showAdd = false">Cancel</button>
        <button class="btn btn-primary" @click="submitFeedback">Submit</button>
      </div>
    </div>
  </div>
</template>
