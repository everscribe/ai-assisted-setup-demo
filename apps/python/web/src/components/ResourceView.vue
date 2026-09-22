<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { api, type AccessRequest, type RequestStatus, type Resource, type User } from '../api'
import RequestCard from './RequestCard.vue'

const props = defineProps<{
  resource: Resource
  me: User
}>()

const emit = defineEmits<{
  error: [msg: string | null]
}>()

const requests = ref<AccessRequest[]>([])
const members = ref<User[]>([])
const requesting = ref(false)
const reason = ref('')
const expanded = ref<string | null>(null)

async function refresh() {
  try {
    const [r, m] = await Promise.all([
      api.listResourceRequests(props.resource.id),
      api.listMembers(),
    ])
    requests.value = r
    members.value = m
  } catch (err) {
    emit('error', String(err))
  }
}

watch(() => props.resource.id, refresh, { immediate: true })

const isAdmin = computed(() => props.me.role === 'admin')

const STATUSES: RequestStatus[] = ['pending', 'approved', 'denied', 'revoked']

const grouped = computed(() => {
  const out: Record<RequestStatus, AccessRequest[]> = {
    pending: [],
    approved: [],
    denied: [],
    revoked: [],
  }
  for (const r of requests.value) out[r.status].push(r)
  return out
})

function label(status: RequestStatus): string {
  switch (status) {
    case 'pending':
      return 'Pending'
    case 'approved':
      return 'Approved'
    case 'denied':
      return 'Denied'
    case 'revoked':
      return 'Revoked'
  }
}

async function onRequest() {
  if (!reason.value.trim()) return
  try {
    await api.requestAccess(props.resource.id, reason.value.trim())
    reason.value = ''
    requesting.value = false
    await refresh()
  } catch (err) {
    emit('error', String(err))
  }
}

function onCancelRequest() {
  requesting.value = false
  reason.value = ''
}

async function onApprove(id: string) {
  try {
    await api.approveRequest(id)
    await refresh()
  } catch (err) {
    emit('error', String(err))
  }
}

async function onDeny(id: string) {
  if (!confirm('Deny this request?')) return
  try {
    await api.denyRequest(id)
    await refresh()
  } catch (err) {
    emit('error', String(err))
  }
}

async function onRevoke(id: string) {
  if (!confirm('Revoke this approved access?')) return
  try {
    await api.revokeRequest(id)
    await refresh()
  } catch (err) {
    emit('error', String(err))
  }
}
</script>

<template>
  <div>
    <div class="board-detail-header">
      <div>
        <h2>
          <code>{{ resource.name }}</code>
          <span :class="`sensitivity-pill sens-${resource.sensitivity}`">{{ resource.sensitivity }}</span>
        </h2>
        <p v-if="resource.description" class="muted">{{ resource.description }}</p>
      </div>
      <button v-if="!resource.decommission_at" @click="requesting = true">+ Request access</button>
    </div>
    <form v-if="requesting" class="inline-form vstack" @submit.prevent="onRequest">
      <textarea
        v-model="reason"
        placeholder="Why do you need access? (e.g. debugging stuck payment #PAY-4821)"
        rows="2"
        required
      ></textarea>
      <div class="row">
        <button type="submit">Submit request</button>
        <button type="button" class="secondary" @click="onCancelRequest">
          Cancel
        </button>
      </div>
    </form>
    <div class="columns">
      <section v-for="status in STATUSES" :key="status" class="column">
        <h3>{{ label(status) }} <span class="muted">- {{ grouped[status].length }}</span></h3>
        <div class="card-list">
          <RequestCard
            v-for="req in grouped[status]"
            :key="req.id"
            :request="req"
            :members="members"
            :is-admin="isAdmin"
            :expanded="expanded === req.id"
            @toggle="expanded = expanded === req.id ? null : req.id"
            @approve="onApprove"
            @deny="onDeny"
            @revoke="onRevoke"
            @error="emit('error', $event)"
          />
        </div>
      </section>
    </div>
  </div>
</template>
