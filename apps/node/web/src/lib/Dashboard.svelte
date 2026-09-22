<script lang="ts">
  import type { User } from '../api'
  import ResourcesTab from './ResourcesTab.svelte'
  import MembersTab from './MembersTab.svelte'

  export let me: User
  export let onLogout: () => void
  export let onError: (msg: string | null) => void
  export let error: string | null
  export let onDismissError: () => void

  type Tab = 'resources' | 'members'
  let tab: Tab = 'resources'
</script>

<div class="app">
  <header class="topbar">
    <div>
      <h1>Access portal</h1>
      <p class="sub">
        Signed in as <strong>{me.name}</strong>
        <span class="role-pill">{me.role}</span>
        · org <code>{me.organization_id}</code>
      </p>
    </div>
    <button class="logout" on:click={onLogout}>Sign out</button>
  </header>

  {#if error}
    <div class="error">
      {error} <button on:click={onDismissError}>dismiss</button>
    </div>
  {/if}

  <nav class="tabs" role="tablist">
    <button
      role="tab"
      class={tab === 'resources' ? 'active' : ''}
      on:click={() => (tab = 'resources')}
    >
      Resources
    </button>
    <button
      role="tab"
      class={tab === 'members' ? 'active' : ''}
      on:click={() => (tab = 'members')}
    >
      Members
    </button>
  </nav>

  {#if tab === 'resources'}
    <ResourcesTab {me} {onError} />
  {:else if tab === 'members'}
    <MembersTab {me} {onError} />
  {/if}
</div>
