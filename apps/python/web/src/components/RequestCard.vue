<script setup lang="ts">
import { ref, watch } from 'vue'
import { api, type AccessRequest, type Comment, type User } from '../api'

const props = defineProps<{
  request: AccessRequest
  members: User[]
  isAdmin: boolean
  expanded: boolean
}>()

const emit = defineEmits<{
  toggle: []
  approve: [id: string]
  deny: [id: string]
  revoke: [id: string]
  error: [msg: string | null]
}>()

const comments = ref<Comment[]>([])
const draft = ref('')

watch(
  () => props.expanded,
  (expanded) => {
    if (!expanded) return
    api
      .listComments(props.request.id)
      .then((c) => (comments.value = c))
      .catch((err) => emit('error', String(err)))
  },
  { immediate: true },
)

async function onAddComment() {
  if (!draft.value.trim()) return
  try {
    await api.addComment(props.request.id, draft.value.trim())
    draft.value = ''
    comments.value = await api.listComments(props.request.id)
  } catch (err) {
    emit('error', String(err))
  }
}

function authorName(id: string): string {
  return props.members.find((m) => m.id === id)?.name ?? id
}

function fmtTime(iso: string): string {
  return new Date(iso).toLocaleString(undefined, {
    hour: '2-digit',
    minute: '2-digit',
    month: 'short',
    day: 'numeric',
  })
}
</script>

<template>
  <div class="task-card">
    <div class="task-summary" @click="emit('toggle')">
      <div>
        <div class="task-title">{{ request.reason }}</div>
        <div class="task-desc">
          requested by <strong>{{ authorName(request.requested_by) }}</strong>
          <template v-if="request.decided_by">
            -
            {{
              request.status === 'approved' || request.status === 'revoked'
                ? 'approved'
                : 'denied'
            }}
            by <strong>{{ authorName(request.decided_by) }}</strong>
          </template>
          <template v-if="request.revoked_by">
            - revoked by <strong>{{ authorName(request.revoked_by) }}</strong>
          </template>
        </div>
      </div>
    </div>
    <div v-if="expanded" class="task-detail">
      <div class="task-actions">
        <template v-if="isAdmin && request.status === 'pending'">
          <button @click="emit('approve', request.id)">Approve</button>
          <button class="danger" @click="emit('deny', request.id)">Deny</button>
        </template>
        <button
          v-else-if="isAdmin && request.status === 'approved'"
          class="danger"
          @click="emit('revoke', request.id)"
        >
          Revoke access
        </button>
        <span v-else-if="!isAdmin && request.status === 'pending'" class="muted small">
          Awaiting an admin's decision.
        </span>
        <span v-else-if="request.status === 'denied'" class="muted small">
          Denied - file a new request to try again.
        </span>
        <span v-else-if="request.status === 'revoked'" class="muted small">
          Access has been revoked.
        </span>
      </div>
      <div class="comments">
        <h4>Comments ({{ comments.length }})</h4>
        <ul>
          <li v-for="c in comments" :key="c.id">
            <strong>{{ authorName(c.author_id) }}:</strong> {{ c.body }}
            <span class="muted small"> - {{ fmtTime(c.created_at) }}</span>
          </li>
          <li v-if="comments.length === 0" class="muted">No comments yet.</li>
        </ul>
        <form class="comment-form" @submit.prevent="onAddComment">
          <input v-model="draft" placeholder="Add a review note..." />
          <button type="submit" :disabled="!draft.trim()">Post</button>
        </form>
      </div>
    </div>
  </div>
</template>
