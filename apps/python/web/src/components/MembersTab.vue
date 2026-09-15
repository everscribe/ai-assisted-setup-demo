<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, type User } from '../api'

const props = defineProps<{
  me: User
}>()

const emit = defineEmits<{
  error: [msg: string | null]
}>()

const members = ref<User[]>([])
const inviting = ref(false)
const name = ref('')
const email = ref('')
const role = ref<'admin' | 'member'>('member')

async function refresh() {
  try {
    members.value = await api.listMembers()
  } catch (err) {
    emit('error', String(err))
  }
}

onMounted(refresh)

const isAdmin = computed(() => props.me.role === 'admin')

async function onInvite() {
  try {
    await api.inviteMember({ name: name.value.trim(), email: email.value.trim(), role: role.value })
    name.value = ''
    email.value = ''
    role.value = 'member'
    inviting.value = false
    await refresh()
  } catch (err) {
    emit('error', String(err))
  }
}

async function onChangeRole(id: string, next: 'admin' | 'member') {
  try {
    await api.changeMemberRole(id, next)
    await refresh()
  } catch (err) {
    emit('error', String(err))
  }
}

async function onRemove(id: string) {
  if (!confirm('Remove this member?')) return
  try {
    await api.removeMember(id)
    await refresh()
  } catch (err) {
    emit('error', String(err))
  }
}
</script>

<template>
  <div class="members-tab">
    <div class="members-header">
      <h2>Organization members ({{ members.length }})</h2>
      <button v-if="isAdmin" @click="inviting = true">+ Invite</button>
    </div>
    <p v-if="!isAdmin" class="muted">
      Only admins can invite, remove, or change roles for members.
    </p>
    <form v-if="inviting" class="inline-form vstack" @submit.prevent="onInvite">
      <input v-model="name" placeholder="Name" required />
      <input v-model="email" type="email" placeholder="email@org.test" required />
      <label>
        <span>Role</span>
        <select v-model="role">
          <option value="member">Member</option>
          <option value="admin">Admin</option>
        </select>
      </label>
      <div class="row">
        <button type="submit">Invite</button>
        <button type="button" class="secondary" @click="inviting = false">Cancel</button>
      </div>
    </form>
    <table class="members-table">
      <thead>
        <tr>
          <th>Name</th>
          <th>Email</th>
          <th>Role</th>
          <th v-if="isAdmin"></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="m in members" :key="m.id">
          <td>{{ m.name }}</td>
          <td>{{ m.email }}</td>
          <td>
            <select
              v-if="isAdmin && m.id !== me.id"
              :value="m.role"
              @change="onChangeRole(m.id, ($event.target as HTMLSelectElement).value as 'admin' | 'member')"
            >
              <option value="member">member</option>
              <option value="admin">admin</option>
            </select>
            <span v-else class="role-pill">{{ m.role }}</span>
          </td>
          <td v-if="isAdmin">
            <button v-if="m.id !== me.id" class="danger" @click="onRemove(m.id)">Remove</button>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
