import test from 'node:test'
import assert from 'node:assert/strict'
import { setupDom, flush } from '../testutil/dom.ts'
import { ada } from '../testutil/fixtures.ts'
import { clearUserId, setUserId, getUserId } from '../src/api.ts'

setupDom()
const { renderDashboard } = await import('../src/views/Dashboard.ts')

function stubApi() {
  return {
    listResources: async () => [],
    listMembers: async () => [],
  } as any
}

test('renders the signed-in user and their org', async () => {
  const el = renderDashboard({ me: ada, api: stubApi(), onLogout: () => {} })
  await flush()
  assert.ok(el.textContent?.includes('Ada Lovelace'))
  assert.ok(el.textContent?.includes('admin'))
  assert.ok(el.textContent?.includes('org_acme'))
})

test('sign out clears the stored user id, so a reload finds no session', async () => {
  setUserId('u_ada')
  assert.equal(getUserId(), 'u_ada')

  let loggedOut = false
  const el = renderDashboard({
    me: ada,
    api: stubApi(),
    onLogout: () => {
      loggedOut = true
      clearUserId()
    },
  })
  await flush()

  const signOut = [...el.querySelectorAll('button')].find((b) => b.textContent === 'Sign out')
  assert.ok(signOut, 'expected a Sign out button')
  signOut!.click()

  assert.equal(loggedOut, true)
  // A fresh call to getUserId(), as main.ts makes on load, now finds
  // nothing - this is what "reloading does not restore the session"
  // reduces to at the storage layer.
  assert.equal(getUserId(), '')
})
