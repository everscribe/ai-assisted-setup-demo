import type { AccessRequest, Api, Comment, RequestStatus, User } from '../api.ts'

export interface RequestCardProps {
  request: AccessRequest
  members: User[]
  isAdmin: boolean
  expanded: boolean
  api: Api
  onToggle: () => void
  onApprove: (id: string) => void
  onDeny: (id: string) => void
  onRevoke: (id: string) => void
  onError: (msg: string | null) => void
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

function statusVerb(status: RequestStatus): string {
  return status === 'approved' || status === 'revoked' ? 'approved' : 'denied'
}

export function renderRequestCard(props: RequestCardProps): HTMLElement {
  const container = document.createElement('div')
  container.className = 'task-card'

  let comments: Comment[] = []
  let draft = ''

  function draw() {
    container.replaceChildren()

    const summary = document.createElement('div')
    summary.className = 'task-summary'
    summary.addEventListener('click', props.onToggle)

    const left = document.createElement('div')
    const title = document.createElement('div')
    title.className = 'task-title'
    title.textContent = props.request.reason
    left.appendChild(title)

    const desc = document.createElement('div')
    desc.className = 'task-desc'
    const requester = props.members.find((m) => m.id === props.request.requested_by)
    desc.append('requested by ')
    const reqStrong = document.createElement('strong')
    reqStrong.textContent = requester?.name ?? props.request.requested_by
    desc.appendChild(reqStrong)

    const decider = props.members.find((m) => m.id === props.request.decided_by)
    if (decider) {
      desc.append(` - ${statusVerb(props.request.status)} by `)
      const s = document.createElement('strong')
      s.textContent = decider.name
      desc.appendChild(s)
    }
    const revoker = props.members.find((m) => m.id === props.request.revoked_by)
    if (revoker) {
      desc.append(' - revoked by ')
      const s = document.createElement('strong')
      s.textContent = revoker.name
      desc.appendChild(s)
    }
    left.appendChild(desc)
    summary.appendChild(left)
    container.appendChild(summary)

    if (!props.expanded) return

    const detail = document.createElement('div')
    detail.className = 'task-detail'

    const actions = document.createElement('div')
    actions.className = 'task-actions'
    if (props.isAdmin && props.request.status === 'pending') {
      const approve = document.createElement('button')
      approve.textContent = 'Approve'
      approve.addEventListener('click', () => props.onApprove(props.request.id))
      const deny = document.createElement('button')
      deny.className = 'danger'
      deny.textContent = 'Deny'
      deny.addEventListener('click', () => props.onDeny(props.request.id))
      actions.append(approve, deny)
    }
    if (props.isAdmin && props.request.status === 'approved') {
      const revoke = document.createElement('button')
      revoke.className = 'danger'
      revoke.textContent = 'Revoke access'
      revoke.addEventListener('click', () => props.onRevoke(props.request.id))
      actions.appendChild(revoke)
    }
    if (!props.isAdmin && props.request.status === 'pending') {
      const span = document.createElement('span')
      span.className = 'muted small'
      span.textContent = "Awaiting an admin's decision."
      actions.appendChild(span)
    }
    if (props.request.status === 'denied') {
      const span = document.createElement('span')
      span.className = 'muted small'
      span.textContent = 'Denied - file a new request to try again.'
      actions.appendChild(span)
    }
    if (props.request.status === 'revoked') {
      const span = document.createElement('span')
      span.className = 'muted small'
      span.textContent = 'Access has been revoked.'
      actions.appendChild(span)
    }
    detail.appendChild(actions)

    const commentsSection = document.createElement('div')
    commentsSection.className = 'comments'
    const h4 = document.createElement('h4')
    h4.textContent = `Comments (${comments.length})`
    commentsSection.appendChild(h4)

    const ul = document.createElement('ul')
    for (const c of comments) {
      const li = document.createElement('li')
      const strong = document.createElement('strong')
      strong.textContent = `${authorName(props.members, c.author_id)}:`
      li.append(strong, ` ${c.body}`)
      const time = document.createElement('span')
      time.className = 'muted small'
      time.textContent = ` - ${fmtTime(c.created_at)}`
      li.appendChild(time)
      ul.appendChild(li)
    }
    if (comments.length === 0) {
      const li = document.createElement('li')
      li.className = 'muted'
      li.textContent = 'No comments yet.'
      ul.appendChild(li)
    }
    commentsSection.appendChild(ul)

    const form = document.createElement('form')
    form.className = 'comment-form'
    const input = document.createElement('input')
    input.placeholder = 'Add a review note...'
    input.value = draft
    input.addEventListener('input', () => {
      draft = input.value
      postBtn.disabled = !draft.trim()
    })
    const postBtn = document.createElement('button')
    postBtn.type = 'submit'
    postBtn.textContent = 'Post'
    postBtn.disabled = !draft.trim()
    form.append(input, postBtn)
    form.addEventListener('submit', async (e) => {
      e.preventDefault()
      if (!draft.trim()) return
      try {
        await props.api.addComment(props.request.id, draft.trim())
        draft = ''
        comments = await props.api.listComments(props.request.id)
        draw()
      } catch (err) {
        props.onError(String(err))
      }
    })
    commentsSection.appendChild(form)

    detail.appendChild(commentsSection)
    container.appendChild(detail)
  }

  draw()

  if (props.expanded) {
    props.api
      .listComments(props.request.id)
      .then((c) => {
        comments = c
        draw()
      })
      .catch((err) => props.onError(String(err)))
  }

  return container
}
