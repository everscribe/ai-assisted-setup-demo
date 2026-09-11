<script lang="ts">
  import { onMount } from 'svelte'
  import { api, type User } from '../api'

  export let me: User
  export let onError: (msg: string | null) => void

  let members: User[] = []
  let inviting = false
  let name = ''
  let email = ''
  let role: 'admin' | 'member' = 'member'

  async function refresh() {
    try {
      members = await api.listMembers()
    } catch (err) {
      onError(String(err))
    }
  }

  onMount(() => {
    void refresh()
  })

  async function onInvite(e: Event) {
    e.preventDefault()
    try {
      await api.inviteMember({ name: name.trim(), email: email.trim(), role })
      name = ''
      email = ''
      role = 'member'
      inviting = false
      await refresh()
    } catch (err) {
      onError(String(err))
    }
  }

  async function onChangeRole(id: string, next: 'admin' | 'member') {
    try {
      await api.changeMemberRole(id, next)
      await refresh()
    } catch (err) {
      onError(String(err))
    }
  }

  async function onRemove(id: string) {
    if (!confirm('Remove this member?')) return
    try {
      await api.removeMember(id)
      await refresh()
    } catch (err) {
      onError(String(err))
    }
  }

  function handleRoleChange(id: string, e: Event) {
    const value = (e.currentTarget as HTMLSelectElement).value as 'admin' | 'member'
    void onChangeRole(id, value)
  }

  $: isAdmin = me.role === 'admin'
</script>

<div class="members-tab">
  <div class="members-header">
    <h2>Organization members ({members.length})</h2>
    {#if isAdmin}
      <button on:click={() => (inviting = true)}>＋ Invite</button>
    {/if}
  </div>
  {#if !isAdmin}
    <p class="muted">Only admins can invite, remove, or change roles for members.</p>
  {/if}
  {#if inviting}
    <form class="inline-form vstack" on:submit={onInvite}>
      <input autofocus bind:value={name} placeholder="Name" required />
      <input type="email" bind:value={email} placeholder="email@org.test" required />
      <label>
        <span>Role</span>
        <select bind:value={role}>
          <option value="member">Member</option>
          <option value="admin">Admin</option>
        </select>
      </label>
      <div class="row">
        <button type="submit">Invite</button>
        <button type="button" class="secondary" on:click={() => (inviting = false)}>
          Cancel
        </button>
      </div>
    </form>
  {/if}
  <table class="members-table">
    <thead>
      <tr>
        <th>Name</th>
        <th>Email</th>
        <th>Role</th>
        {#if isAdmin}<th></th>{/if}
      </tr>
    </thead>
    <tbody>
      {#each members as m (m.id)}
        <tr>
          <td>{m.name}</td>
          <td>{m.email}</td>
          <td>
            {#if isAdmin && m.id !== me.id}
              <select value={m.role} on:change={(e) => handleRoleChange(m.id, e)}>
                <option value="member">member</option>
                <option value="admin">admin</option>
              </select>
            {:else}
              <span class="role-pill">{m.role}</span>
            {/if}
          </td>
          {#if isAdmin}
            <td>
              {#if m.id !== me.id}
                <button class="danger" on:click={() => onRemove(m.id)}>Remove</button>
              {/if}
            </td>
          {/if}
        </tr>
      {/each}
    </tbody>
  </table>
</div>
