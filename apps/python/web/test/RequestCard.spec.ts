import { describe, it, expect, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import RequestCard from '../src/components/RequestCard.vue'
import { ada, c1, c2, linus, reqNew } from '../testutil/fixtures'

vi.mock('../src/api', async () => {
  const actual = await vi.importActual<typeof import('../src/api')>('../src/api')
  return {
    ...actual,
    api: {
      ...actual.api,
      listComments: vi.fn(),
      addComment: vi.fn(),
    },
  }
})

import { api } from '../src/api'

describe('RequestCard', () => {
  it('adding a comment appends it to the end of the list', async () => {
    let comments = [c1, c2]
    vi.mocked(api.listComments).mockImplementation(async () => comments)
    vi.mocked(api.addComment).mockImplementation(async (_id: string, body: string) => {
      const created = {
        id: 'c_new',
        request_id: reqNew.id,
        organization_id: 'org_acme',
        author_id: ada.id,
        body,
        created_at: '2026-03-14T13:00:00Z',
      }
      comments = [...comments, created]
      return created
    })

    const wrapper = mount(RequestCard, {
      props: {
        request: reqNew,
        members: [ada, linus],
        isAdmin: true,
        expanded: true,
      },
    })
    await flushPromises()

    let bodies = wrapper.findAll('.comments li').map((li) => li.text())
    expect(bodies.length, 'expected the two seeded comments to load').toBe(2)

    const input = wrapper.find('.comment-form input')
    await input.setValue('Will drop the grant once this closes.')
    await wrapper.find('.comment-form').trigger('submit')
    await flushPromises()

    bodies = wrapper.findAll('.comments li').map((li) => li.text())
    expect(bodies.length).toBe(3)
    expect(bodies[2]).toContain('Will drop the grant once this closes.')
  })
})
