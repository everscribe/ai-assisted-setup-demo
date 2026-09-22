<script lang="ts">
  // A handful of seeded email addresses ship with the Node backend; the
  // login screen renders these as quick-pick buttons so anyone landing
  // on the demo can sign in without typing.
  const QUICK_PICKS: { email: string; label: string }[] = [
    { email: 'ada@acme.test', label: 'Ada (Acme · admin)' },
    { email: 'linus@acme.test', label: 'Linus (Acme · member)' },
    { email: 'grace@globex.test', label: 'Grace (Globex · admin)' },
    { email: 'alan@globex.test', label: 'Alan (Globex · member)' },
  ]

  export let email: string
  export let onLogin: (email: string) => void
  export let error: string | null
  export let onDismissError: () => void

  function handleSubmit(e: Event) {
    e.preventDefault()
    if (email) onLogin(email)
  }
</script>

<div class="login">
  <h1>Access portal</h1>
  <p class="sub">Demo · Svelte + Node · multi-tenant</p>
  {#if error}
    <div class="error">
      {error} <button on:click={onDismissError}>dismiss</button>
    </div>
  {/if}
  <form class="login-form" on:submit={handleSubmit}>
    <label>
      <span>Email</span>
      <input
        type="email"
        bind:value={email}
        placeholder="ada@acme.test"
        required
      />
    </label>
    <button type="submit" class="primary">Sign in</button>
  </form>
  <div class="quick-picks">
    <span class="muted">Quick sign-in:</span>
    {#each QUICK_PICKS as q (q.email)}
      <button on:click={() => onLogin(q.email)}>{q.label}</button>
    {/each}
  </div>
</div>
