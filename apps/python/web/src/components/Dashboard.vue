<script setup lang="ts">
import { ref } from 'vue'
import type { User } from '../api'
import ResourcesTab from './ResourcesTab.vue'
import MembersTab from './MembersTab.vue'

defineProps<{
  me: User
  error: string | null
}>()

const emit = defineEmits<{
  logout: []
  error: [msg: string | null]
  dismissError: []
}>()

type Tab = 'resources' | 'members'
const tab = ref<Tab>('resources')
</script>

<template>
  <div class="app">
    <header class="topbar">
      <div>
        <h1>Access portal</h1>
        <p class="sub">
          Signed in as <strong>{{ me.name }}</strong>
          <span class="role-pill">{{ me.role }}</span>
          - org <code>{{ me.organization_id }}</code>
        </p>
      </div>
      <button class="logout" @click="emit('logout')">Sign out</button>
    </header>

    <div v-if="error" class="error">
      {{ error }} <button @click="emit('dismissError')">dismiss</button>
    </div>

    <nav class="tabs" role="tablist">
      <button
        role="tab"
        :class="{ active: tab === 'resources' }"
        @click="tab = 'resources'"
      >
        Resources
      </button>
      <button
        role="tab"
        :class="{ active: tab === 'members' }"
        @click="tab = 'members'"
      >
        Members
      </button>
    </nav>

    <ResourcesTab
      v-if="tab === 'resources'"
      :me="me"
      @error="emit('error', $event)"
    />
    <MembersTab
      v-if="tab === 'members'"
      :me="me"
      @error="emit('error', $event)"
    />
  </div>
</template>
