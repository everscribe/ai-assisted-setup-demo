import test from 'node:test'
import assert from 'node:assert/strict'
import { setupDom, flush } from '../testutil/dom.ts'
import { acmeResourcesInContractOrder, globexResources, grace, ada } from '../testutil/fixtures.ts'

setupDom()
const { renderResourcesTab } = await import('../src/views/ResourcesTab.ts')

function stubApi(resources: unknown[]) {
  return {
    listResources: async () => resources,
    listResourceRequests: async () => [],
    listMembers: async () => [],
  } as any
}

test('resources render in the contract order with sensitivity badges', async () => {
  const el = renderResourcesTab({ me: ada, api: stubApi(acmeResourcesInContractOrder) })
  await flush()

  const items = [...el.querySelectorAll('.board-list li')]
  const names = items.map((li) => li.querySelector('code')?.textContent)
  // Contract order (created_at asc, then id asc) is
  // production-postgres, audit-logs-bucket, stripe-live-dashboard -
  // not alphabetical by name, so a view that resorted by name would
  // fail this exact-order check.
  assert.deepEqual(names, ['production-postgres', 'audit-logs-bucket', 'stripe-live-dashboard'])

  const badges = items.map((li) => li.querySelector('.sensitivity-pill')?.textContent)
  assert.deepEqual(badges, ['restricted', 'confidential', 'restricted'])
})

test('signed in as grace, only globex resources are visible', async () => {
  // The real API scopes GET /api/resources to the actor's org; this
  // stub returns what Grace's org-scoped call would.
  const el = renderResourcesTab({ me: grace, api: stubApi(globexResources) })
  await flush()

  const text = el.textContent ?? ''
  // Positive control first: an empty render would otherwise also
  // "pass" the negative assertion below.
  assert.ok(text.includes('k8s-prod-cluster'), 'expected a globex resource to be present')
  assert.ok(!text.includes('production-postgres'), 'acme resource must not leak into globex view')
})
