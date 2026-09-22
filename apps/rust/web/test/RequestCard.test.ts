import test from 'node:test'
import assert from 'node:assert/strict'
import { setupDom, flush, fire } from '../testutil/dom.ts'
import { ada, c1, c2, linus, reqNew } from '../testutil/fixtures.ts'

setupDom()
const { renderRequestCard } = await import('../src/views/RequestCard.ts')

test('adding a comment appends it to the end of the list', async () => {
  let comments = [c1, c2]
  const api = {
    listComments: async () => comments,
    addComment: async (_id: string, body: string) => {
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
    },
  } as any

  const el = renderRequestCard({
    request: reqNew,
    members: [ada, linus],
    isAdmin: true,
    expanded: true,
    api,
    onToggle: () => {},
    onApprove: () => {},
    onDeny: () => {},
    onRevoke: () => {},
    onError: (msg) => assert.fail(`unexpected error: ${msg}`),
  })
  await flush()

  let bodies = [...el.querySelectorAll('.comments li')].map((li) => li.textContent)
  assert.equal(bodies.length, 2, 'expected the two seeded comments to load')

  const input = el.querySelector('.comment-form input') as HTMLInputElement
  input.value = 'Will drop the grant once this closes.'
  fire(input, 'input')
  const form = el.querySelector('.comment-form') as HTMLFormElement
  fire(form, 'submit')
  await flush()

  bodies = [...el.querySelectorAll('.comments li')].map((li) => li.textContent ?? '')
  assert.equal(bodies.length, 3)
  assert.ok(
    bodies[2].includes('Will drop the grant once this closes.'),
    'new comment must be the last item in the list',
  )
})
