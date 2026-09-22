import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ResourceView from '../src/components/ResourceView.vue'
import { acmePg, ada, linus, reqNew, reqOld } from '../testutil/fixtures'

vi.mock('../src/api', async () => {
  const actual = await vi.importActual<typeof import('../src/api')>('../src/api')
  return {
    ...actual,
    api: {
      ...actual.api,
      listResourceRequests: vi.fn(),
      listMembers: vi.fn(),
      approveRequest: vi.fn(),
      listComments: vi.fn(),
    },
  }
})

import { api } from '../src/api'

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(api.listComments).mockResolvedValue([])
})

describe('ResourceView', () => {
  it('renders access requests newest first', async () => {
    // Already server-sorted, newest first. The fixture ids and reasons
    // both sort the other way, so this catches a re-sort by id or by
    // title as well as a reversal.
    vi.mocked(api.listResourceRequests).mockResolvedValue([reqNew, reqOld])
    vi.mocked(api.listMembers).mockResolvedValue([linus])

    const wrapper = mount(ResourceView, { props: { resource: acmePg, me: ada } })
    await flushPromises()

    const columns = wrapper.findAll('.column')
    const pendingColumn = columns.find((c) => c.find('h3').text().startsWith('Pending'))!
    const titles = pendingColumn.findAll('.task-title').map((t) => t.text())
    expect(titles).toEqual([reqNew.reason, reqOld.reason])
  })

  it('approving a pending request updates that card in place, without a full reload', async () => {
    let approved = false
    vi.mocked(api.listMembers).mockResolvedValue([linus, ada])
    vi.mocked(api.listResourceRequests).mockImplementation(async () =>
      approved ? [{ ...reqNew, status: 'approved', decided_by: 'u_ada' }] : [reqNew],
    )
    vi.mocked(api.approveRequest).mockImplementation(async (id: string) => {
      expect(id).toBe(reqNew.id)
      approved = true
      return { ...reqNew, status: 'approved', decided_by: 'u_ada' }
    })

    const hrefBefore = window.location.href

    const wrapper = mount(ResourceView, { props: { resource: acmePg, me: ada } })
    await flushPromises()

    // Expand the card first: actions only render when a card is expanded.
    await wrapper.find('.task-summary').trigger('click')
    await flushPromises()

    const approveBtn = wrapper.findAll('button').find((b) => b.text() === 'Approve')
    expect(approveBtn, 'expected an Approve button on the expanded pending card').toBeTruthy()
    await approveBtn!.trigger('click')
    await flushPromises()

    expect(approved).toBe(true)
    expect(window.location.href).toBe(hrefBefore)

    // The card is still expanded (its detail pane, with the now-relevant
    // Revoke button, is visible) rather than the view having collapsed
    // back to a blank state - that is the "in place" part.
    const revokeBtn = wrapper.findAll('button').find((b) => b.text() === 'Revoke access')
    expect(revokeBtn, 'expected the same card to now show Revoke access, still expanded').toBeTruthy()
    const stillApprove = wrapper.findAll('button').find((b) => b.text() === 'Approve')
    expect(stillApprove).toBeUndefined()
  })
})
