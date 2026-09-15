// Fixture data drawn from contract/seed.json, so tests exercise the
// same shapes the real API returns.
import type { AccessRequest, Comment, Resource, User } from '../src/api'

export const ada: User = {
  id: 'u_ada',
  name: 'Ada Lovelace',
  email: 'ada@acme.test',
  role: 'admin',
  organization_id: 'org_acme',
  created_at: '2026-03-13T12:00:00Z',
}

export const linus: User = {
  id: 'u_linus',
  name: 'Linus Torvalds',
  email: 'linus@acme.test',
  role: 'member',
  organization_id: 'org_acme',
  created_at: '2026-03-13T12:00:00Z',
}

export const grace: User = {
  id: 'u_grace',
  name: 'Grace Hopper',
  email: 'grace@globex.test',
  role: 'admin',
  organization_id: 'org_globex',
  created_at: '2026-03-13T12:00:00Z',
}

export const alan: User = {
  id: 'u_alan',
  name: 'Alan Turing',
  email: 'alan@globex.test',
  role: 'member',
  organization_id: 'org_globex',
  created_at: '2026-03-13T12:00:00Z',
}

// Contract order for GET /api/resources on org_acme: created_at asc,
// then id asc. All three share a created_at, so id breaks the tie:
// r_acme_pg, r_acme_s3, r_acme_stripe. That is NOT alphabetical by
// name (audit-logs-bucket would sort first), so a view that
// accidentally re-sorts by name is caught by asserting this exact
// order.
export const acmePg: Resource = {
  id: 'r_acme_pg',
  name: 'production-postgres',
  description: 'Primary customer database (us-east-1)',
  sensitivity: 'restricted',
  organization_id: 'org_acme',
  registered_by: 'u_ada',
  created_at: '2026-03-13T12:00:00Z',
}

export const acmeS3: Resource = {
  id: 'r_acme_s3',
  name: 'audit-logs-bucket',
  description: 'S3 bucket for archived audit logs',
  sensitivity: 'confidential',
  organization_id: 'org_acme',
  registered_by: 'u_ada',
  created_at: '2026-03-13T12:00:00Z',
}

export const acmeStripe: Resource = {
  id: 'r_acme_stripe',
  name: 'stripe-live-dashboard',
  description: 'Live-mode Stripe console - refund + chargeback ops',
  sensitivity: 'restricted',
  organization_id: 'org_acme',
  registered_by: 'u_ada',
  created_at: '2026-03-13T12:00:00Z',
}

export const acmeResourcesInContractOrder: Resource[] = [acmePg, acmeS3, acmeStripe]

export const globexK8s: Resource = {
  id: 'r_globex_k8s',
  name: 'k8s-prod-cluster',
  description: 'Production EKS cluster (kubectl admin)',
  sensitivity: 'restricted',
  organization_id: 'org_globex',
  registered_by: 'u_grace',
  created_at: '2026-03-13T12:00:00Z',
}

export const globexPagerduty: Resource = {
  id: 'r_globex_pagerduty',
  name: 'pagerduty-admin',
  description: 'PagerDuty account admin (rotation + escalation policies)',
  sensitivity: 'confidential',
  organization_id: 'org_globex',
  registered_by: 'u_grace',
  created_at: '2026-03-13T12:00:00Z',
}

export const globexResources: Resource[] = [globexK8s, globexPagerduty]

// Two pending requests on the same resource with distinct created_at,
// already in the server's newest-first order, so the view is only
// asked to preserve order, never to compute it.
export const reqNew: AccessRequest = {
  id: 'req_new',
  resource_id: 'r_acme_pg',
  organization_id: 'org_acme',
  requested_by: 'u_linus',
  reason: 'Newest request',
  status: 'pending',
  created_at: '2026-03-14T12:00:00Z',
}

export const reqOld: AccessRequest = {
  id: 'req_old',
  resource_id: 'r_acme_pg',
  organization_id: 'org_acme',
  requested_by: 'u_linus',
  reason: 'Oldest request',
  status: 'pending',
  created_at: '2026-03-14T09:00:00Z',
}

export const c1: Comment = {
  id: 'c_acme_1',
  request_id: 'req_acme_1',
  organization_id: 'org_acme',
  author_id: 'u_linus',
  body: 'Scope is the payments schema, read-only.',
  created_at: '2026-03-14T10:00:00Z',
}

export const c2: Comment = {
  id: 'c_acme_2',
  request_id: 'req_acme_1',
  organization_id: 'org_acme',
  author_id: 'u_ada',
  body: 'Confirmed with the on-call lead. Approving for 24 hours.',
  created_at: '2026-03-14T12:00:00Z',
}
