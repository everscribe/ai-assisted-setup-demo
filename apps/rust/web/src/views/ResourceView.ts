import type { AccessRequest, Api, Resource, RequestStatus, User } from '../api.ts'
import { renderRequestCard } from './RequestCard.ts'

export interface ResourceViewProps {
  resource: Resource
  me: User
  api: Api
}

const STATUSES: RequestStatus[] = ['pending', 'approved', 'denied', 'revoked']

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

export function renderResourceView(props: ResourceViewProps): HTMLElement {
  const container = document.createElement('div')

  let requests: AccessRequest[] = []
  let members: User[] = []
  let requesting = false
  let reason = ''
  let expandedId: string | null = null
  let error: string | null = null

  const isAdmin = props.me.role === 'admin'

  async function refresh() {
    try {
      const [r, m] = await Promise.all([
        props.api.listResourceRequests(props.resource.id),
        props.api.listMembers(),
      ])
      requests = r
      members = m
      error = null
    } catch (err) {
      error = String(err)
    }
    draw()
  }

  function grouped(): Record<RequestStatus, AccessRequest[]> {
    const out: Record<RequestStatus, AccessRequest[]> = {
      pending: [],
      approved: [],
      denied: [],
      revoked: [],
    }
    for (const r of requests) out[r.status].push(r)
    return out
  }

  async function onApprove(id: string) {
    try {
      await props.api.approveRequest(id)
      await refresh()
    } catch (err) {
      error = String(err)
      draw()
    }
  }
  async function onDeny(id: string) {
    if (!confirm('Deny this request?')) return
    try {
      await props.api.denyRequest(id)
      await refresh()
    } catch (err) {
      error = String(err)
      draw()
    }
  }
  async function onRevoke(id: string) {
    if (!confirm('Revoke this approved access?')) return
    try {
      await props.api.revokeRequest(id)
      await refresh()
    } catch (err) {
      error = String(err)
      draw()
    }
  }

  function draw() {
    container.replaceChildren()

    if (error) {
      const banner = document.createElement('div')
      banner.className = 'error'
      banner.append(document.createTextNode(error + ' '))
      const dismiss = document.createElement('button')
      dismiss.textContent = 'dismiss'
      dismiss.addEventListener('click', () => {
        error = null
        draw()
      })
      banner.appendChild(dismiss)
      container.appendChild(banner)
    }

    const header = document.createElement('div')
    header.className = 'board-detail-header'
    const headerLeft = document.createElement('div')
    const h2 = document.createElement('h2')
    const code = document.createElement('code')
    code.textContent = props.resource.name
    const pill = document.createElement('span')
    pill.className = `sensitivity-pill sens-${props.resource.sensitivity}`
    pill.textContent = props.resource.sensitivity
    h2.append(code, pill)
    headerLeft.appendChild(h2)
    if (props.resource.description) {
      const p = document.createElement('p')
      p.className = 'muted'
      p.textContent = props.resource.description
      headerLeft.appendChild(p)
    }
    header.appendChild(headerLeft)
    if (!props.resource.decommission_at) {
      const btn = document.createElement('button')
      btn.textContent = '+ Request access'
      btn.addEventListener('click', () => {
        requesting = true
        draw()
      })
      header.appendChild(btn)
    }
    container.appendChild(header)

    if (requesting) {
      const form = document.createElement('form')
      form.className = 'inline-form vstack'
      const textarea = document.createElement('textarea')
      textarea.placeholder = 'Why do you need access? (e.g. debugging stuck payment #PAY-4821)'
      textarea.rows = 2
      textarea.required = true
      textarea.value = reason
      textarea.addEventListener('input', () => {
        reason = textarea.value
      })
      form.appendChild(textarea)

      const row = document.createElement('div')
      row.className = 'row'
      const submitBtn = document.createElement('button')
      submitBtn.type = 'submit'
      submitBtn.textContent = 'Submit request'
      const cancelBtn = document.createElement('button')
      cancelBtn.type = 'button'
      cancelBtn.className = 'secondary'
      cancelBtn.textContent = 'Cancel'
      cancelBtn.addEventListener('click', () => {
        requesting = false
        reason = ''
        draw()
      })
      row.append(submitBtn, cancelBtn)
      form.appendChild(row)

      form.addEventListener('submit', async (e) => {
        e.preventDefault()
        if (!reason.trim()) return
        try {
          await props.api.requestAccess(props.resource.id, reason.trim())
          reason = ''
          requesting = false
          await refresh()
        } catch (err) {
          error = String(err)
          draw()
        }
      })
      container.appendChild(form)
    }

    const columns = document.createElement('div')
    columns.className = 'columns'
    const byStatus = grouped()
    for (const status of STATUSES) {
      const section = document.createElement('section')
      section.className = 'column'
      const h3 = document.createElement('h3')
      h3.append(`${label(status)} `)
      const count = document.createElement('span')
      count.className = 'muted'
      count.textContent = `- ${byStatus[status].length}`
      h3.appendChild(count)
      section.appendChild(h3)

      const list = document.createElement('div')
      list.className = 'card-list'
      for (const req of byStatus[status]) {
        const card = renderRequestCard({
          request: req,
          members,
          isAdmin,
          expanded: expandedId === req.id,
          api: props.api,
          onToggle: () => {
            expandedId = expandedId === req.id ? null : req.id
            draw()
          },
          onApprove,
          onDeny,
          onRevoke,
          onError: (msg) => {
            error = msg
            draw()
          },
        })
        list.appendChild(card)
      }
      section.appendChild(list)
      columns.appendChild(section)
    }
    container.appendChild(columns)
  }

  draw()
  void refresh()

  return container
}
