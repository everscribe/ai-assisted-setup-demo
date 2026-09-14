import test from 'node:test'
import assert from 'node:assert/strict'
import { setupDom, flush } from '../testutil/dom.ts'
import { acmePg, ada, linus, reqNew, reqOld } from '../testutil/fixtures.ts'

setupDom()
const { renderResourceView } = await import('../src/views/ResourceView.ts')

test('access requests render newest first', async () => {
  const el = renderResourceView({
    resource: acmePg,
    me: ada,
    api: {
      listResourceRequests: async () => [reqNew, reqOld], // already server-sorted, newest first
      listMembers: async () => [linus],
    } as any,
  })
  await flush()

  const pendingColumn = [...el.querySelectorAll('.column')].find((c) =>
    c.querySelector('h3')?.textContent?.startsWith('Pending'),
  )!
  const titles = [...pendingColumn.querySelectorAll('.task-title')].map((t) => t.textContent)
  assert.deepEqual(titles, ['Newest request', 'Oldest request'])
})

test('approving a pending request updates that card in place, without a full reload', async () => {
  let approved = false
  const hrefBefore = (globalThis as any).window.location.href

  const el = renderResourceView({
    resource: acmePg,
    me: ada,
    api: {
      listResourceRequests: async () => (approved ? [{ ...reqNew, status: 'approved', decided_by: 'u_ada' }] : [reqNew]),
      listMembers: async () => [linus, ada],
      approveRequest: async (id: string) => {
        assert.equal(id, reqNew.id)
        approved = true
        return { ...reqNew, status: 'approved', decided_by: 'u_ada' }
      },
      listComments: async () => [],
    } as any,
  })
  await flush()

  // Expand the card first: actions only render when a card is expanded.
  const summary = el.querySelector('.task-summary') as HTMLElement
  summary.click()
  await flush()

  const approveBtn = [...el.querySelectorAll('button')].find((b) => b.textContent === 'Approve')
  assert.ok(approveBtn, 'expected an Approve button on the expanded pending card')
  approveBtn!.click()
  await flush()

  assert.equal(approved, true)
  assert.equal(
    (globalThis as any).window.location.href,
    hrefBefore,
    'approving must not trigger a full page reload/navigation',
  )

  // The card is still expanded (its detail pane, with the now-relevant
  // Revoke button, is visible) rather than the view having collapsed
  // back to a blank state - that is the "in place" part.
  const revokeBtn = [...el.querySelectorAll('button')].find((b) => b.textContent === 'Revoke access')
  assert.ok(revokeBtn, 'expected the same card to now show Revoke access, still expanded')
  const stillApprove = [...el.querySelectorAll('button')].find((b) => b.textContent === 'Approve')
  assert.equal(stillApprove, undefined, 'Approve/Deny should be gone now that the request is approved')
})
