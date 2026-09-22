// Tiny fetch wrapper around the Go backend.
//
// Auth model: after POST /api/login, the returned user's id is stashed
// in localStorage and sent as X-User-ID on every subsequent request.
// The Go backend's requireActor() reads that header to identify the
// caller. Real apps would use sessions/JWTs; the auto-instrument agent
// works with either shape, so we keep the demo minimal.

export interface Organization {
  id: string
  name: string
  created_at: string
}

export interface User {
  id: string
  name: string
  email: string
  role: 'admin' | 'member'
  organization_id: string
  created_at: string
}

export interface Resource {
  id: string
  name: string
  description: string
  sensitivity: 'public' | 'internal' | 'confidential' | 'restricted'
  organization_id: string
  registered_by: string
  created_at: string
  decommission_at?: string
}

export type RequestStatus = 'pending' | 'approved' | 'denied' | 'revoked'

export interface AccessRequest {
  id: string
  resource_id: string
  organization_id: string
  requested_by: string
  reason: string
  status: RequestStatus
  decided_by?: string
  decided_at?: string
  revoked_by?: string
  revoked_at?: string
  created_at: string
}

export interface Comment {
  id: string
  request_id: string
  organization_id: string
  author_id: string
  body: string
  created_at: string
}

const ACTOR_KEY = 'demo.userId'

export function getUserId(): string {
  return localStorage.getItem(ACTOR_KEY) ?? ''
}

export function setUserId(id: string) {
  localStorage.setItem(ACTOR_KEY, id)
}

export function clearUserId() {
  localStorage.removeItem(ACTOR_KEY)
}

async function call<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body) headers.set('Content-Type', 'application/json')
  const id = getUserId()
  if (id) headers.set('X-User-ID', id)
  const res = await fetch(path, { ...init, headers })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(`${res.status} ${res.statusText}: ${text || path}`)
  }
  return res.status === 204 ? (undefined as T) : (res.json() as Promise<T>)
}

export const api = {
  // auth
  login: (email: string) =>
    call<User>('/api/login', {
      method: 'POST',
      body: JSON.stringify({ email }),
    }),
  logout: () => call<void>('/api/logout', { method: 'POST' }),
  me: () => call<User>('/api/me'),

  // members
  listMembers: () => call<User[]>('/api/members'),
  inviteMember: (input: { name: string; email: string; role: 'admin' | 'member' }) =>
    call<User>('/api/members', { method: 'POST', body: JSON.stringify(input) }),
  removeMember: (id: string) =>
    call<void>(`/api/members/${id}`, { method: 'DELETE' }),
  changeMemberRole: (id: string, role: 'admin' | 'member') =>
    call<User>(`/api/members/${id}/role`, {
      method: 'PUT',
      body: JSON.stringify({ role }),
    }),

  // resources
  listResources: () => call<Resource[]>('/api/resources'),
  registerResource: (input: {
    name: string
    description: string
    sensitivity: Resource['sensitivity']
  }) =>
    call<Resource>('/api/resources', {
      method: 'POST',
      body: JSON.stringify(input),
    }),
  decommissionResource: (id: string) =>
    call<Resource>(`/api/resources/${id}/decommission`, { method: 'POST' }),

  // access requests
  listResourceRequests: (resourceId: string) =>
    call<AccessRequest[]>(`/api/resources/${resourceId}/requests`),
  requestAccess: (resourceId: string, reason: string) =>
    call<AccessRequest>(`/api/resources/${resourceId}/requests`, {
      method: 'POST',
      body: JSON.stringify({ reason }),
    }),
  approveRequest: (requestId: string) =>
    call<AccessRequest>(`/api/requests/${requestId}/approve`, { method: 'POST' }),
  denyRequest: (requestId: string) =>
    call<AccessRequest>(`/api/requests/${requestId}/deny`, { method: 'POST' }),
  revokeRequest: (requestId: string) =>
    call<AccessRequest>(`/api/requests/${requestId}/revoke`, { method: 'POST' }),

  // comments
  listComments: (requestId: string) =>
    call<Comment[]>(`/api/requests/${requestId}/comments`),
  addComment: (requestId: string, body: string) =>
    call<Comment>(`/api/requests/${requestId}/comments`, {
      method: 'POST',
      body: JSON.stringify({ body }),
    }),
}
