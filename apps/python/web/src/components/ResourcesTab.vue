<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, type Resource, type User } from '../api'
import ResourceView from './ResourceView.vue'

const props = defineProps<{
  me: User
}>()

const emit = defineEmits<{
  error: [msg: string | null]
}>()

const resources = ref<Resource[]>([])
const selectedId = ref<string | null>(null)
const creating = ref(false)
const newName = ref('')
const newDescription = ref('')
const newSensitivity = ref<Resource['sensitivity']>('internal')

async function refresh() {
  try {
    const r = await api.listResources()
    resources.value = r
    if (!selectedId.value && r.length > 0) selectedId.value = r[0].id
  } catch (err) {
    emit('error', String(err))
  }
}

onMounted(refresh)

const isAdmin = computed(() => props.me.role === 'admin')
const selectedResource = computed(() =>
  resources.value.find((r) => r.id === selectedId.value),
)

async function onRegister() {
  if (!newName.value.trim()) return
  try {
    await api.registerResource({
      name: newName.value.trim(),
      description: newDescription.value.trim(),
      sensitivity: newSensitivity.value,
    })
    newName.value = ''
    newDescription.value = ''
    newSensitivity.value = 'internal'
    creating.value = false
    await refresh()
  } catch (err) {
    emit('error', String(err))
  }
}

function onCancelCreate() {
  creating.value = false
  newName.value = ''
  newDescription.value = ''
}

async function onDecommission(id: string) {
  if (!confirm('Decommission this resource? New access requests will be refused.')) return
  try {
    await api.decommissionResource(id)
    await refresh()
  } catch (err) {
    emit('error', String(err))
  }
}
</script>

<template>
  <div class="boards-tab">
    <aside class="board-list">
      <div class="board-list-header">
        <h2>Resources</h2>
        <button v-if="isAdmin" @click="creating = true">+ Register</button>
      </div>
      <form v-if="creating" class="inline-form vstack" @submit.prevent="onRegister">
        <input
          v-model="newName"
          placeholder="resource name (e.g. production-postgres)"
          required
        />
        <input v-model="newDescription" placeholder="description" />
        <label>
          <span>Sensitivity</span>
          <select v-model="newSensitivity">
            <option value="public">public</option>
            <option value="internal">internal</option>
            <option value="confidential">confidential</option>
            <option value="restricted">restricted</option>
          </select>
        </label>
        <div class="row">
          <button type="submit">Register</button>
          <button type="button" class="secondary" @click="onCancelCreate">
            Cancel
          </button>
        </div>
      </form>
      <ul>
        <li
          v-for="r in resources"
          :key="r.id"
          :class="{ selected: r.id === selectedId }"
          @click="selectedId = r.id"
        >
          <div class="board-name">
            <code>{{ r.name }}</code>
            <span :class="`sensitivity-pill sens-${r.sensitivity}`">{{ r.sensitivity }}</span>
            <span v-if="r.decommission_at" class="badge">decommissioned</span>
          </div>
          <div
            v-if="isAdmin && !r.decommission_at"
            class="board-actions"
            @click.stop
          >
            <button class="danger" @click="onDecommission(r.id)">Decommission</button>
          </div>
        </li>
        <li v-if="resources.length === 0 && !creating" class="empty">
          No resources yet - {{ isAdmin ? 'register one' : 'ask an admin to register one' }}.
        </li>
      </ul>
    </aside>
    <main class="board-detail">
      <ResourceView
        v-if="selectedResource"
        :resource="selectedResource"
        :me="me"
        @error="emit('error', $event)"
      />
      <div v-else class="empty-state">Pick a resource to see its access requests.</div>
    </main>
  </div>
</template>
