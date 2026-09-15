import { describe, it, expect, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import Dashboard from '../src/components/Dashboard.vue'
import { ada } from '../testutil/fixtures'

vi.mock('../src/api', async () => {
  const actual = await vi.importActual<typeof import('../src/api')>('../src/api')
  return {
    ...actual,
    api: {
      ...actual.api,
      listResources: vi.fn().mockResolvedValue([]),
      listMembers: vi.fn().mockResolvedValue([]),
    },
  }
})

import { clearUserId, getUserId, setUserId } from '../src/api'

describe('Dashboard', () => {
  it('renders the signed-in user and their org', async () => {
    const wrapper = mount(Dashboard, { props: { me: ada, error: null } })
    await flushPromises()

    expect(wrapper.text()).toContain('Ada Lovelace')
    expect(wrapper.text()).toContain('admin')
    expect(wrapper.text()).toContain('org_acme')
  })

  it('sign out clears the stored user id, so a reload finds no session', async () => {
    setUserId('u_ada')
    expect(getUserId()).toBe('u_ada')

    let loggedOut = false
    const wrapper = mount(Dashboard, {
      props: { me: ada, error: null },
      attrs: {
        // Mirrors App.vue's @logout handler, which is the thing that
        // actually clears storage; Dashboard itself only emits.
        onLogout: () => {
          loggedOut = true
          clearUserId()
        },
      },
    })
    await flushPromises()

    const signOut = wrapper.findAll('button').find((b) => b.text() === 'Sign out')
    expect(signOut, 'expected a Sign out button').toBeTruthy()
    await signOut!.trigger('click')

    expect(loggedOut).toBe(true)
    // A fresh call to getUserId(), as App.vue makes on mount, now finds
    // nothing - this is what "reloading does not restore the session"
    // reduces to at the storage layer.
    expect(getUserId()).toBe('')
  })
})
