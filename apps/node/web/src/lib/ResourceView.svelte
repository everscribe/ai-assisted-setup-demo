<script lang="ts">
  import { api, type AccessRequest, type Resource, type RequestStatus, type User } from '../api'
  import RequestCard from './RequestCard.svelte'

  export let resource: Resource
  export let me: User
  export let onError: (msg: string | null) => void

  const STATUSES: RequestStatus[] = ['pending', 'approved', 'denied', 'revoked']

  let requests: AccessRequest[] = []
  let members: User[] = []
  let requesting = false
  let reason = ''
  let expanded: string | null = null

  async function refresh() {
    try {
      const [r, m] = await Promise.all([
        api.listResourceRequests(resource.id),
        api.listMembers(),
      ])
      requests = r
      members = m
    } catch (err) {
      onError(String(err))
    }
  }

  // Re-fetch whenever the selected resource changes, mirroring the
  // React effect keyed on resource.id.
  let loadedFor: string | null = null
  $: if (resource.id !== loadedFor) {
    loadedFor = resource.id
    void refresh()
  }

  $: isAdmin = me.role === 'admin'

  $: grouped = ((): Record<RequestStatus, AccessRequest[]> => {
    const out: Record<RequestStatus, AccessRequest[]> = {
      pending: [],
      approved: [],
      denied: [],
      revoked: [],
    }
    for (const r of requests) out[r.status].push(r)
    return out
  })()

  async function onRequest(e: Event) {
    e.preventDefault()
    if (!reason.trim()) return
    try {
      await api.requestAccess(resource.id, reason.trim())
      reason = ''
      requesting = false
      await refresh()
    } catch (err) {
      onError(String(err))
    }
  }

  async function onApprove(id: string) {
    try {
      await api.approveRequest(id)
      await refresh()
    } catch (err) {
      onError(String(err))
    }
  }
  async function onDeny(id: string) {
    if (!confirm('Deny this request?')) return
    try {
      await api.denyRequest(id)
      await refresh()
    } catch (err) {
      onError(String(err))
    }
  }
  async function onRevoke(id: string) {
    if (!confirm('Revoke this approved access?')) return
    try {
      await api.revokeRequest(id)
      await refresh()
    } catch (err) {
      onError(String(err))
    }
  }

  function label(status: RequestStatus): string {
    switch (status) {
      case 'pending':
        return 'Pending'
      case 'approved':
        return 'Approved'
      case 'denied':
        return 'Denied'
      case 'revoked':
        return 'Revoked'
    }
  }
</script>

<div>
  <div class="board-detail-header">
    <div>
      <h2>
        <code>{resource.name}</code>
        <span class={`sensitivity-pill sens-${resource.sensitivity}`}>{resource.sensitivity}</span>
      </h2>
      {#if resource.description}
        <p class="muted">{resource.description}</p>
      {/if}
    </div>
    {#if !resource.decommission_at}
      <button on:click={() => (requesting = true)}>＋ Request access</button>
    {/if}
  </div>
  {#if requesting}
    <form class="inline-form vstack" on:submit={onRequest}>
      <textarea
        autofocus
        bind:value={reason}
        placeholder="Why do you need access? (e.g. debugging stuck payment #PAY-4821)"
        rows="2"
        required
      ></textarea>
      <div class="row">
        <button type="submit">Submit request</button>
        <button
          type="button"
          class="secondary"
          on:click={() => {
            requesting = false
            reason = ''
          }}
        >
          Cancel
        </button>
      </div>
    </form>
  {/if}
  <div class="columns">
    {#each STATUSES as status (status)}
      <section class="column">
        <h3>{label(status)} <span class="muted">· {grouped[status].length}</span></h3>
        <div class="card-list">
          {#each grouped[status] as req (req.id)}
            <RequestCard
              request={req}
              {members}
              {isAdmin}
              expanded={expanded === req.id}
              onToggle={() => (expanded = expanded === req.id ? null : req.id)}
              {onApprove}
              {onDeny}
              {onRevoke}
              {onError}
            />
          {/each}
        </div>
      </section>
    {/each}
  </div>
</div>
