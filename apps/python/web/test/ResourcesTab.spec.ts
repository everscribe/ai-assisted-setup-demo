import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ResourcesTab from '../src/components/ResourcesTab.vue'
import { acmeResourcesInContractOrder, ada, globexResources, grace } from '../testutil/fixtures'

vi.mock('../src/api', async () => {
  const actual = await vi.importActual<typeof import('../src/api')>('../src/api')
  return {
    ...actual,
    api: {
      ...actual.api,
      listResources: vi.fn(),
      listResourceRequests: vi.fn(),
      listMembers: vi.fn(),
    },
  }
})

import { api } from '../src/api'

beforeEach(() => {
  vi.mocked(api.listResourceRequests).mockResolvedValue([])
  vi.mocked(api.listMembers).mockResolvedValue([])
})

describe('ResourcesTab', () => {
  it('renders the acme resources in the contract order with sensitivity badges', async () => {
    // Contract order (created_at asc, then id asc) is
    // production-postgres, audit-logs-bucket, stripe-live-dashboard -
    // not alphabetical by name, so a view that resorted by name would
    // fail this exact-order check.
    vi.mocked(api.listResources).mockResolvedValue(acmeResourcesInContractOrder)

    const wrapper = mount(ResourcesTab, { props: { me: ada } })
    await flushPromises()

    const items = wrapper.findAll('.board-list li')
    const names = items.map((li) => li.find('code').text())
    expect(names).toEqual(['production-postgres', 'audit-logs-bucket', 'stripe-live-dashboard'])

    const badges = items.map((li) => li.find('.sensitivity-pill').text())
    expect(badges).toEqual(['restricted', 'confidential', 'restricted'])
  })

  it('signed in as grace, only globex resources are visible', async () => {
    // The real API scopes GET /api/resources to the actor's org; this
    // stub returns what Grace's org-scoped call would.
    vi.mocked(api.listResources).mockResolvedValue(globexResources)

    const wrapper = mount(ResourcesTab, { props: { me: grace } })
    await flushPromises()

    const text = wrapper.text()
    // Positive control first: an empty render would otherwise also
    // "pass" the negative assertion below.
    expect(text).toContain('k8s-prod-cluster')
    expect(text).not.toContain('production-postgres')
  })
})
