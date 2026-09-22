<script setup lang="ts">
import { ref } from 'vue'

// A handful of seeded email addresses ship with the Python backend;
// the login screen renders these as quick-pick buttons so anyone
// landing on the demo can sign in without typing.
const QUICK_PICKS: { email: string; label: string }[] = [
  { email: 'ada@acme.test', label: 'Ada (Acme - admin)' },
  { email: 'linus@acme.test', label: 'Linus (Acme - member)' },
  { email: 'grace@globex.test', label: 'Grace (Globex - admin)' },
  { email: 'alan@globex.test', label: 'Alan (Globex - member)' },
]

defineProps<{
  error: string | null
}>()

const emit = defineEmits<{
  login: [email: string]
  dismissError: []
}>()

const email = ref('')

function submit() {
  if (!email.value) return
  emit('login', email.value)
  email.value = ''
}
</script>

<template>
  <div class="login">
    <h1>Access portal</h1>
    <p class="sub">Demo - Vue + Python - multi-tenant</p>
    <div v-if="error" class="error">
      {{ error }} <button @click="emit('dismissError')">dismiss</button>
    </div>
    <form class="login-form" @submit.prevent="submit">
      <label>
        <span>Email</span>
        <input
          v-model="email"
          type="email"
          placeholder="ada@acme.test"
          required
        />
      </label>
      <button type="submit" class="primary">Sign in</button>
    </form>
    <div class="quick-picks">
      <span class="muted">Quick sign-in:</span>
      <button
        v-for="q in QUICK_PICKS"
        :key="q.email"
        @click="emit('login', q.email)"
      >
        {{ q.label }}
      </button>
    </div>
  </div>
</template>
