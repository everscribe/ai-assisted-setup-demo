<script lang="ts">
  import { onMount } from 'svelte'
  import { api, type Resource, type User } from '../api'
  import ResourceView from './ResourceView.svelte'

  export let me: User
  export let onError: (msg: string | null) => void

  let resources: Resource[] = []
  let selectedId: string | null = null
  let creating = false
  let newName = ''
  let newDescription = ''
  let newSensitivity: Resource['sensitivity'] = 'internal'

  async function refresh() {
    try {
      const r = await api.listResources()
      resources = r
      if (!selectedId && r.length > 0) selectedId = r[0].id
    } catch (err) {
      onError(String(err))
    }
  }

  onMount(() => {
    void refresh()
  })

  $: isAdmin = me.role === 'admin'
  $: selected = resources.find((r) => r.id === selectedId) ?? null

  async function onRegister(e: Event) {
    e.preventDefault()
    if (!newName.trim()) return
    try {
      await api.registerResource({
        name: newName.trim(),
        description: newDescription.trim(),
        sensitivity: newSensitivity,
      })
      newName = ''
      newDescription = ''
      newSensitivity = 'internal'
      creating = false
      await refresh()
    } catch (err) {
      onError(String(err))
    }
  }

  async function onDecommission(id: string) {
    if (!confirm('Decommission this resource? New access requests will be refused.')) return
    try {
      await api.decommissionResource(id)
      await refresh()
    } catch (err) {
      onError(String(err))
    }
  }
</script>

<div class="boards-tab">
  <aside class="board-list">
    <div class="board-list-header">
      <h2>Resources</h2>
      {#if isAdmin}
        <button on:click={() => (creating = true)}>＋ Register</button>
      {/if}
    </div>
    {#if creating}
      <form on:submit={onRegister} class="inline-form vstack">
        <input
          autofocus
          bind:value={newName}
          placeholder="resource name (e.g. production-postgres)"
          required
        />
        <input bind:value={newDescription} placeholder="description" />
        <label>
          <span>Sensitivity</span>
          <select bind:value={newSensitivity}>
            <option value="public">public</option>
            <option value="internal">internal</option>
            <option value="confidential">confidential</option>
            <option value="restricted">restricted</option>
          </select>
        </label>
        <div class="row">
          <button type="submit">Register</button>
          <button
            type="button"
            class="secondary"
            on:click={() => {
              creating = false
              newName = ''
              newDescription = ''
            }}
          >
            Cancel
          </button>
        </div>
      </form>
    {/if}
    <ul>
      {#each resources as r (r.id)}
        <li class={r.id === selectedId ? 'selected' : ''} on:click={() => (selectedId = r.id)}>
          <div class="board-name">
            <code>{r.name}</code>
            <span class={`sensitivity-pill sens-${r.sensitivity}`}>{r.sensitivity}</span>
            {#if r.decommission_at}
              <span class="badge">decommissioned</span>
            {/if}
          </div>
          {#if isAdmin && !r.decommission_at}
            <div class="board-actions" on:click|stopPropagation>
              <button class="danger" on:click={() => onDecommission(r.id)}>Decommission</button>
            </div>
          {/if}
        </li>
      {/each}
      {#if resources.length === 0 && !creating}
        <li class="empty">
          No resources yet - {isAdmin ? 'register one' : 'ask an admin to register one'}.
        </li>
      {/if}
    </ul>
  </aside>
  <main class="board-detail">
    {#if selected}
      <ResourceView resource={selected} {me} {onError} />
    {:else}
      <div class="empty-state">Pick a resource to see its access requests.</div>
    {/if}
  </main>
</div>
