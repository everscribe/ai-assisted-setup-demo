<script lang="ts">
  import { onMount } from 'svelte'
  import { api, clearUserId, getUserId, setUserId, type User } from './api'
  import LoginScreen from './lib/LoginScreen.svelte'
  import Dashboard from './lib/Dashboard.svelte'

  let me: User | null = null
  let error: string | null = null
  let loginEmail = ''

  onMount(() => {
    if (!getUserId()) return
    api
      .me()
      .then((u) => (me = u))
      .catch(() => {
        clearUserId()
      })
  })

  async function handleLogin(email: string) {
    try {
      const user = await api.login(email)
      setUserId(user.id)
      me = user
      loginEmail = ''
      error = null
    } catch (err) {
      error = String(err)
    }
  }

  async function handleLogout() {
    try {
      await api.logout()
    } catch {
      // server-side logout has no state; ignore the result
    }
    clearUserId()
    me = null
  }

  function dismissError() {
    error = null
  }
</script>

{#if !me}
  <LoginScreen
    bind:email={loginEmail}
    {error}
    onLogin={handleLogin}
    onDismissError={dismissError}
  />
{:else}
  <Dashboard
    {me}
    onLogout={handleLogout}
    onError={(msg) => (error = msg)}
    {error}
    onDismissError={dismissError}
  />
{/if}
