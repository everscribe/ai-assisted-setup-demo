import { useEffect, useMemo, useState, type FormEvent } from 'react'
import {
  api,
  clearUserId,
  getUserId,
  setUserId,
  type AccessRequest,
  type Comment,
  type RequestStatus,
  type Resource,
  type User,
} from './api.js'
import { AuditTrail } from '@everscribe/components-react'
import '@everscribe/components-styles/default.css'

// A handful of seeded email addresses ship with the Go backend; the
// login screen renders these as quick-pick buttons so anyone landing
// on the demo can sign in without typing.
const QUICK_PICKS: { email: string; label: string }[] = [
  { email: 'ada@acme.test', label: 'Ada (Acme · admin)' },
  { email: 'linus@acme.test', label: 'Linus (Acme · member)' },
  { email: 'grace@globex.test', label: 'Grace (Globex · admin)' },
  { email: 'alan@globex.test', label: 'Alan (Globex · member)' },
]

export function App() {
  const [me, setMe] = useState<User | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loginEmail, setLoginEmail] = useState('')

  useEffect(() => {
    if (!getUserId()) return
    api
      .me()
      .then(setMe)
      .catch(() => {
        clearUserId()
      })
  }, [])

  const handleLogin = async (email: string) => {
    try {
      const user = await api.login(email)
      setUserId(user.id)
      setMe(user)
      setLoginEmail('')
      setError(null)
    } catch (err) {
      setError(String(err))
    }
  }

  const handleLogout = async () => {
    try {
      await api.logout()
    } catch {
      // server-side logout has no state; ignore the result
    }
    clearUserId()
    setMe(null)
  }

  if (!me) {
    return (
      <LoginScreen
        email={loginEmail}
        onEmailChange={setLoginEmail}
        onLogin={handleLogin}
        error={error}
        onDismissError={() => setError(null)}
      />
    )
  }

  return (
    <Dashboard
      me={me}
      onLogout={handleLogout}
      onError={setError}
      error={error}
      onDismissError={() => setError(null)}
    />
  )
}

// ============================================================
// Login
// ============================================================

interface LoginScreenProps {
  email: string
  onEmailChange: (v: string) => void
  onLogin: (email: string) => void
  error: string | null
  onDismissError: () => void
}

function LoginScreen(p: LoginScreenProps) {
  return (
    <div className="login">
      <h1>Access portal</h1>
      <p className="sub">Demo · React + Go · multi-tenant</p>
      {p.error && (
        <div className="error">
          {p.error} <button onClick={p.onDismissError}>dismiss</button>
        </div>
      )}
      <form
        className="login-form"
        onSubmit={(e: FormEvent) => {
          e.preventDefault()
          if (p.email) p.onLogin(p.email)
        }}
      >
        <label>
          <span>Email</span>
          <input
            type="email"
            value={p.email}
            onChange={(e) => p.onEmailChange(e.target.value)}
            placeholder="ada@acme.test"
            required
          />
        </label>
        <button type="submit" className="primary">
          Sign in
        </button>
      </form>
      <div className="quick-picks">
        <span className="muted">Quick sign-in:</span>
        {QUICK_PICKS.map((q) => (
          <button key={q.email} onClick={() => p.onLogin(q.email)}>
            {q.label}
          </button>
        ))}
      </div>
    </div>
  )
}

// ============================================================
// Dashboard
// ============================================================

interface DashboardProps {
  me: User
  onLogout: () => void
  onError: (msg: string | null) => void
  error: string | null
  onDismissError: () => void
}

type Tab = 'resources' | 'members' | 'audit'

function Dashboard(p: DashboardProps) {
  const [tab, setTab] = useState<Tab>('resources')

  return (
    <div className="app">
      <header className="topbar">
        <div>
          <h1>Access portal</h1>
          <p className="sub">
            Signed in as <strong>{p.me.name}</strong>
            <span className="role-pill">{p.me.role}</span>
            · org <code>{p.me.organization_id}</code>
          </p>
        </div>
        <button className="logout" onClick={p.onLogout}>
          Sign out
        </button>
      </header>

      {p.error && (
        <div className="error">
          {p.error} <button onClick={p.onDismissError}>dismiss</button>
        </div>
      )}

      <nav className="tabs" role="tablist">
        <button
          role="tab"
          className={tab === 'resources' ? 'active' : ''}
          onClick={() => setTab('resources')}
        >
          Resources
        </button>
        <button
          role="tab"
          className={tab === 'members' ? 'active' : ''}
          onClick={() => setTab('members')}
        >
          Members
        </button>
        <button
          role="tab"
          className={tab === 'audit' ? 'active' : ''}
          onClick={() => setTab('audit')}
        >
          Audit
        </button>
      </nav>

      {tab === 'resources' && <ResourcesTab me={p.me} onError={p.onError} />}
      {tab === 'members' && <MembersTab me={p.me} onError={p.onError} />}
      {tab === 'audit' && <AuditTab me={p.me} onError={p.onError} />}
    </div>
  )
}

// ============================================================
// Audit tab
// ============================================================

interface AuditTabProps {
  me: User
  onError: (msg: string | null) => void
}

function AuditTab(p: AuditTabProps) {
  return (
    <AuditTrail
      tokenEndpoint="/everscribe/embed-token"
      onError={(err) => p.onError(err.message)}
    />
  )
}

// ============================================================
// Resources tab
// ============================================================

interface ResourcesTabProps {
  me: User
  onError: (msg: string | null) => void
}

function ResourcesTab(p: ResourcesTabProps) {
  const [resources, setResources] = useState<Resource[]>([])
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)
  const [newName, setNewName] = useState('')
  const [newDescription, setNewDescription] = useState('')
  const [newSensitivity, setNewSensitivity] =
    useState<Resource['sensitivity']>('internal')

  const refresh = async () => {
    try {
      const r = await api.listResources()
      setResources(r)
      if (!selectedId && r.length > 0) setSelectedId(r[0].id)
    } catch (err) {
      p.onError(String(err))
    }
  }

  useEffect(() => {
    void refresh()
  }, [])

  const isAdmin = p.me.role === 'admin'

  const onRegister = async (e: FormEvent) => {
    e.preventDefault()
    if (!newName.trim()) return
    try {
      await api.registerResource({
        name: newName.trim(),
        description: newDescription.trim(),
        sensitivity: newSensitivity,
      })
      setNewName('')
      setNewDescription('')
      setNewSensitivity('internal')
      setCreating(false)
      await refresh()
    } catch (err) {
      p.onError(String(err))
    }
  }

  const onDecommission = async (id: string) => {
    if (!confirm('Decommission this resource? New access requests will be refused.')) return
    try {
      await api.decommissionResource(id)
      await refresh()
    } catch (err) {
      p.onError(String(err))
    }
  }

  return (
    <div className="boards-tab">
      <aside className="board-list">
        <div className="board-list-header">
          <h2>Resources</h2>
          {isAdmin && <button onClick={() => setCreating(true)}>＋ Register</button>}
        </div>
        {creating && (
          <form onSubmit={onRegister} className="inline-form vstack">
            <input
              autoFocus
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              placeholder="resource name (e.g. production-postgres)"
              required
            />
            <input
              value={newDescription}
              onChange={(e) => setNewDescription(e.target.value)}
              placeholder="description"
            />
            <label>
              <span>Sensitivity</span>
              <select
                value={newSensitivity}
                onChange={(e) =>
                  setNewSensitivity(e.target.value as Resource['sensitivity'])
                }
              >
                <option value="public">public</option>
                <option value="internal">internal</option>
                <option value="confidential">confidential</option>
                <option value="restricted">restricted</option>
              </select>
            </label>
            <div className="row">
              <button type="submit">Register</button>
              <button
                type="button"
                className="secondary"
                onClick={() => {
                  setCreating(false)
                  setNewName('')
                  setNewDescription('')
                }}
              >
                Cancel
              </button>
            </div>
          </form>
        )}
        <ul>
          {resources.map((r) => (
            <li
              key={r.id}
              className={r.id === selectedId ? 'selected' : ''}
              onClick={() => setSelectedId(r.id)}
            >
              <div className="board-name">
                <code>{r.name}</code>
                <span className={`sensitivity-pill sens-${r.sensitivity}`}>
                  {r.sensitivity}
                </span>
                {r.decommission_at && (
                  <span className="badge">decommissioned</span>
                )}
              </div>
              {isAdmin && !r.decommission_at && (
                <div className="board-actions" onClick={(e) => e.stopPropagation()}>
                  <button className="danger" onClick={() => onDecommission(r.id)}>
                    Decommission
                  </button>
                </div>
              )}
            </li>
          ))}
          {resources.length === 0 && !creating && (
            <li className="empty">
              No resources yet - {isAdmin ? 'register one' : 'ask an admin to register one'}.
            </li>
          )}
        </ul>
      </aside>
      <main className="board-detail">
        {selectedId ? (
          <ResourceView
            resource={resources.find((r) => r.id === selectedId)!}
            me={p.me}
            onError={p.onError}
          />
        ) : (
          <div className="empty-state">Pick a resource to see its access requests.</div>
        )}
      </main>
    </div>
  )
}

// ============================================================
// One resource's access requests
// ============================================================

interface ResourceViewProps {
  resource: Resource
  me: User
  onError: (msg: string | null) => void
}

function ResourceView(p: ResourceViewProps) {
  const [requests, setRequests] = useState<AccessRequest[]>([])
  const [members, setMembers] = useState<User[]>([])
  const [requesting, setRequesting] = useState(false)
  const [reason, setReason] = useState('')
  const [expanded, setExpanded] = useState<string | null>(null)

  const refresh = async () => {
    try {
      const [r, m] = await Promise.all([
        api.listResourceRequests(p.resource.id),
        api.listMembers(),
      ])
      setRequests(r)
      setMembers(m)
    } catch (err) {
      p.onError(String(err))
    }
  }

  useEffect(() => {
    void refresh()
  }, [p.resource.id])

  const isAdmin = p.me.role === 'admin'

  const grouped = useMemo(() => {
    const out: Record<RequestStatus, AccessRequest[]> = {
      pending: [],
      approved: [],
      denied: [],
      revoked: [],
    }
    for (const r of requests) out[r.status].push(r)
    return out
  }, [requests])

  const onRequest = async (e: FormEvent) => {
    e.preventDefault()
    if (!reason.trim()) return
    try {
      await api.requestAccess(p.resource.id, reason.trim())
      setReason('')
      setRequesting(false)
      await refresh()
    } catch (err) {
      p.onError(String(err))
    }
  }

  const onApprove = async (id: string) => {
    try {
      await api.approveRequest(id)
      await refresh()
    } catch (err) {
      p.onError(String(err))
    }
  }
  const onDeny = async (id: string) => {
    if (!confirm('Deny this request?')) return
    try {
      await api.denyRequest(id)
      await refresh()
    } catch (err) {
      p.onError(String(err))
    }
  }
  const onRevoke = async (id: string) => {
    if (!confirm('Revoke this approved access?')) return
    try {
      await api.revokeRequest(id)
      await refresh()
    } catch (err) {
      p.onError(String(err))
    }
  }

  return (
    <div>
      <div className="board-detail-header">
        <div>
          <h2>
            <code>{p.resource.name}</code>
            <span className={`sensitivity-pill sens-${p.resource.sensitivity}`}>
              {p.resource.sensitivity}
            </span>
          </h2>
          {p.resource.description && (
            <p className="muted">{p.resource.description}</p>
          )}
        </div>
        {!p.resource.decommission_at && (
          <button onClick={() => setRequesting(true)}>＋ Request access</button>
        )}
      </div>
      {requesting && (
        <form className="inline-form vstack" onSubmit={onRequest}>
          <textarea
            autoFocus
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            placeholder="Why do you need access? (e.g. debugging stuck payment #PAY-4821)"
            rows={2}
            required
          />
          <div className="row">
            <button type="submit">Submit request</button>
            <button
              type="button"
              className="secondary"
              onClick={() => {
                setRequesting(false)
                setReason('')
              }}
            >
              Cancel
            </button>
          </div>
        </form>
      )}
      <div className="columns">
        {(['pending', 'approved', 'denied', 'revoked'] as const).map((status) => (
          <section className="column" key={status}>
            <h3>
              {label(status)} <span className="muted">· {grouped[status].length}</span>
            </h3>
            <div className="card-list">
              {grouped[status].map((req) => (
                <RequestCard
                  key={req.id}
                  request={req}
                  members={members}
                  isAdmin={isAdmin}
                  expanded={expanded === req.id}
                  onToggle={() =>
                    setExpanded(expanded === req.id ? null : req.id)
                  }
                  onApprove={onApprove}
                  onDeny={onDeny}
                  onRevoke={onRevoke}
                  onError={p.onError}
                />
              ))}
            </div>
          </section>
        ))}
      </div>
    </div>
  )
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

// ============================================================
// Access-request card (expandable, holds comment thread)
// ============================================================

interface RequestCardProps {
  request: AccessRequest
  members: User[]
  isAdmin: boolean
  expanded: boolean
  onToggle: () => void
  onApprove: (id: string) => void
  onDeny: (id: string) => void
  onRevoke: (id: string) => void
  onError: (msg: string | null) => void
}

function RequestCard(p: RequestCardProps) {
  const [comments, setComments] = useState<Comment[]>([])
  const [draft, setDraft] = useState('')

  useEffect(() => {
    if (!p.expanded) return
    api
      .listComments(p.request.id)
      .then(setComments)
      .catch((err) => p.onError(String(err)))
  }, [p.expanded, p.request.id])

  const onAddComment = async (e: FormEvent) => {
    e.preventDefault()
    if (!draft.trim()) return
    try {
      await api.addComment(p.request.id, draft.trim())
      setDraft('')
      const c = await api.listComments(p.request.id)
      setComments(c)
    } catch (err) {
      p.onError(String(err))
    }
  }

  const requester = p.members.find((m) => m.id === p.request.requested_by)
  const decider = p.members.find((m) => m.id === p.request.decided_by)
  const revoker = p.members.find((m) => m.id === p.request.revoked_by)

  return (
    <div className="task-card">
      <div className="task-summary" onClick={p.onToggle}>
        <div>
          <div className="task-title">{p.request.reason}</div>
          <div className="task-desc">
            requested by <strong>{requester?.name ?? p.request.requested_by}</strong>
            {decider && (
              <>
                {' · '}
                {p.request.status === 'approved' || p.request.status === 'revoked'
                  ? 'approved'
                  : 'denied'}{' '}
                by <strong>{decider.name}</strong>
              </>
            )}
            {revoker && (
              <>
                {' · revoked by '}
                <strong>{revoker.name}</strong>
              </>
            )}
          </div>
        </div>
      </div>
      {p.expanded && (
        <div className="task-detail">
          <div className="task-actions">
            {p.isAdmin && p.request.status === 'pending' && (
              <>
                <button onClick={() => p.onApprove(p.request.id)}>Approve</button>
                <button
                  className="danger"
                  onClick={() => p.onDeny(p.request.id)}
                >
                  Deny
                </button>
              </>
            )}
            {p.isAdmin && p.request.status === 'approved' && (
              <button
                className="danger"
                onClick={() => p.onRevoke(p.request.id)}
              >
                Revoke access
              </button>
            )}
            {!p.isAdmin && p.request.status === 'pending' && (
              <span className="muted small">Awaiting an admin's decision.</span>
            )}
            {p.request.status === 'denied' && (
              <span className="muted small">Denied - file a new request to try again.</span>
            )}
            {p.request.status === 'revoked' && (
              <span className="muted small">Access has been revoked.</span>
            )}
          </div>
          <div className="comments">
            <h4>Comments ({comments.length})</h4>
            <ul>
              {comments.map((c) => (
                <li key={c.id}>
                  <strong>{authorName(p.members, c.author_id)}:</strong> {c.body}
                  <span className="muted small"> · {fmtTime(c.created_at)}</span>
                </li>
              ))}
              {comments.length === 0 && (
                <li className="muted">No comments yet.</li>
              )}
            </ul>
            <form onSubmit={onAddComment} className="comment-form">
              <input
                value={draft}
                onChange={(e) => setDraft(e.target.value)}
                placeholder="Add a review note…"
              />
              <button type="submit" disabled={!draft.trim()}>
                Post
              </button>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}

// ============================================================
// Members tab
// ============================================================

interface MembersTabProps {
  me: User
  onError: (msg: string | null) => void
}

function MembersTab(p: MembersTabProps) {
  const [members, setMembers] = useState<User[]>([])
  const [inviting, setInviting] = useState(false)
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [role, setRole] = useState<'admin' | 'member'>('member')

  const refresh = async () => {
    try {
      setMembers(await api.listMembers())
    } catch (err) {
      p.onError(String(err))
    }
  }
  useEffect(() => {
    void refresh()
  }, [])

  const onInvite = async (e: FormEvent) => {
    e.preventDefault()
    try {
      await api.inviteMember({ name: name.trim(), email: email.trim(), role })
      setName('')
      setEmail('')
      setRole('member')
      setInviting(false)
      await refresh()
    } catch (err) {
      p.onError(String(err))
    }
  }

  const onChangeRole = async (id: string, next: 'admin' | 'member') => {
    try {
      await api.changeMemberRole(id, next)
      await refresh()
    } catch (err) {
      p.onError(String(err))
    }
  }

  const onRemove = async (id: string) => {
    if (!confirm('Remove this member?')) return
    try {
      await api.removeMember(id)
      await refresh()
    } catch (err) {
      p.onError(String(err))
    }
  }

  const isAdmin = p.me.role === 'admin'

  return (
    <div className="members-tab">
      <div className="members-header">
        <h2>Organization members ({members.length})</h2>
        {isAdmin && <button onClick={() => setInviting(true)}>＋ Invite</button>}
      </div>
      {!isAdmin && (
        <p className="muted">
          Only admins can invite, remove, or change roles for members.
        </p>
      )}
      {inviting && (
        <form className="inline-form vstack" onSubmit={onInvite}>
          <input
            autoFocus
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Name"
            required
          />
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="email@org.test"
            required
          />
          <label>
            <span>Role</span>
            <select
              value={role}
              onChange={(e) => setRole(e.target.value as 'admin' | 'member')}
            >
              <option value="member">Member</option>
              <option value="admin">Admin</option>
            </select>
          </label>
          <div className="row">
            <button type="submit">Invite</button>
            <button
              type="button"
              className="secondary"
              onClick={() => setInviting(false)}
            >
              Cancel
            </button>
          </div>
        </form>
      )}
      <table className="members-table">
        <thead>
          <tr>
            <th>Name</th>
            <th>Email</th>
            <th>Role</th>
            {isAdmin && <th></th>}
          </tr>
        </thead>
        <tbody>
          {members.map((m) => (
            <tr key={m.id}>
              <td>{m.name}</td>
              <td>{m.email}</td>
              <td>
                {isAdmin && m.id !== p.me.id ? (
                  <select
                    value={m.role}
                    onChange={(e) =>
                      onChangeRole(m.id, e.target.value as 'admin' | 'member')
                    }
                  >
                    <option value="member">member</option>
                    <option value="admin">admin</option>
                  </select>
                ) : (
                  <span className="role-pill">{m.role}</span>
                )}
              </td>
              {isAdmin && (
                <td>
                  {m.id !== p.me.id && (
                    <button
                      className="danger"
                      onClick={() => onRemove(m.id)}
                    >
                      Remove
                    </button>
                  )}
                </td>
              )}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

// ============================================================
// Helpers
// ============================================================

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
