<script lang="ts">
  import { api, type AccessRequest, type Comment, type User } from '../api'

  export let request: AccessRequest
  export let members: User[]
  export let isAdmin: boolean
  export let expanded: boolean
  export let onToggle: () => void
  export let onApprove: (id: string) => void
  export let onDeny: (id: string) => void
  export let onRevoke: (id: string) => void
  export let onError: (msg: string | null) => void

  let comments: Comment[] = []
  let draft = ''

  // Mirrors the React effect keyed on [expanded, request.id]: load
  // comments whenever the card opens, or opens on a different request.
  let loadedKey = ''
  $: {
    const key = `${expanded}:${request.id}`
    if (key !== loadedKey) {
      loadedKey = key
      if (expanded) {
        api
          .listComments(request.id)
          .then((c) => (comments = c))
          .catch((err) => onError(String(err)))
      }
    }
  }

  async function onAddComment(e: Event) {
    e.preventDefault()
    if (!draft.trim()) return
    try {
      await api.addComment(request.id, draft.trim())
      draft = ''
      comments = await api.listComments(request.id)
    } catch (err) {
      onError(String(err))
    }
  }

  function authorName(members: User[], id: string): string {
    return members.find((m) => m.id === id)?.name ?? id
  }

  function fmtTime(iso: string): string {
    return new Date(iso).toLocaleString(undefined, {
      hour: '2-digit',
      minute: '2-digit',
      month: 'short',
      day: 'numeric',
    })
  }

  $: requester = members.find((m) => m.id === request.requested_by)
  $: decider = members.find((m) => m.id === request.decided_by)
  $: revoker = members.find((m) => m.id === request.revoked_by)
</script>

<div class="task-card">
  <div class="task-summary" on:click={onToggle}>
    <div>
      <div class="task-title">{request.reason}</div>
      <div class="task-desc">
        requested by <strong>{requester?.name ?? request.requested_by}</strong>
        {#if decider}
          · {request.status === 'approved' || request.status === 'revoked' ? 'approved' : 'denied'}
          by <strong>{decider.name}</strong>
        {/if}
        {#if revoker}
          · revoked by <strong>{revoker.name}</strong>
        {/if}
      </div>
    </div>
  </div>
  {#if expanded}
    <div class="task-detail">
      <div class="task-actions">
        {#if isAdmin && request.status === 'pending'}
          <button on:click={() => onApprove(request.id)}>Approve</button>
          <button class="danger" on:click={() => onDeny(request.id)}>Deny</button>
        {/if}
        {#if isAdmin && request.status === 'approved'}
          <button class="danger" on:click={() => onRevoke(request.id)}>Revoke access</button>
        {/if}
        {#if !isAdmin && request.status === 'pending'}
          <span class="muted small">Awaiting an admin's decision.</span>
        {/if}
        {#if request.status === 'denied'}
          <span class="muted small">Denied - file a new request to try again.</span>
        {/if}
        {#if request.status === 'revoked'}
          <span class="muted small">Access has been revoked.</span>
        {/if}
      </div>
      <div class="comments">
        <h4>Comments ({comments.length})</h4>
        <ul>
          {#each comments as c (c.id)}
            <li>
              <strong>{authorName(members, c.author_id)}:</strong> {c.body}
              <span class="muted small"> · {fmtTime(c.created_at)}</span>
            </li>
          {/each}
          {#if comments.length === 0}
            <li class="muted">No comments yet.</li>
          {/if}
        </ul>
        <form on:submit={onAddComment} class="comment-form">
          <input bind:value={draft} placeholder="Add a review note…" />
          <button type="submit" disabled={!draft.trim()}>Post</button>
        </form>
      </div>
    </div>
  {/if}
</div>
