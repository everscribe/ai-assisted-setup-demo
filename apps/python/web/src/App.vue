<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, clearUserId, getUserId, setUserId, type User } from './api'
import LoginScreen from './components/LoginScreen.vue'
import Dashboard from './components/Dashboard.vue'

const me = ref<User | null>(null)
const error = ref<string | null>(null)

onMounted(() => {
  if (!getUserId()) return
  api
    .me()
    .then((u) => (me.value = u))
    .catch(() => {
      clearUserId()
    })
})

async function handleLogin(email: string) {
  try {
    const user = await api.login(email)
    setUserId(user.id)
    me.value = user
    error.value = null
  } catch (err) {
    error.value = String(err)
  }
}

async function handleLogout() {
  try {
    await api.logout()
  } catch {
    // server-side logout has no state; ignore the result
  }
  clearUserId()
  me.value = null
}
</script>

<template>
  <LoginScreen
    v-if="!me"
    :error="error"
    @login="handleLogin"
    @dismiss-error="error = null"
  />
  <Dashboard
    v-else
    :me="me"
    :error="error"
    @logout="handleLogout"
    @error="error = $event"
    @dismiss-error="error = null"
  />
</template>
