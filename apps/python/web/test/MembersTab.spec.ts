import { describe, it, expect, vi } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import MembersTab from '../src/components/MembersTab.vue'
import { ada, alan, grace, linus } from '../testutil/fixtures'

vi.mock('../src/api', async () => {
  const actual = await vi.importActual<typeof import('../src/api')>('../src/api')
  return {
    ...actual,
    api: {
      ...actual.api,
      listMembers: vi.fn(),
      inviteMember: vi.fn(),
      changeMemberRole: vi.fn(),
      removeMember: vi.fn(),
    },
  }
})

import { api } from '../src/api'

// Removal goes through window.confirm; jsdom has no real implementation,
// so stub it to behave as if the user clicked OK.
window.confirm = () => true

function rowFor(wrapper: VueWrapper, name: string) {
  return wrapper.findAll('tbody tr').find((tr) => tr.text().includes(name))
}

describe('MembersTab', () => {
  it('members list renders, invite appears, role change and removal are reflected', async () => {
    let members = [ada, linus]
    vi.mocked(api.listMembers).mockImplementation(async () => members)
    vi.mocked(api.inviteMember).mockImplementation(
      async (input: { name: string; email: string; role: 'admin' | 'member' }) => {
        const created = {
          id: 'u_new',
          name: input.name,
          email: input.email,
          role: input.role,
          organization_id: 'org_acme',
          created_at: '2026-03-14T13:00:00Z',
        }
        members = [...members, created]
        return created
      },
    )
    vi.mocked(api.changeMemberRole).mockImplementation(async (id: string, role: 'admin' | 'member') => {
      members = members.map((m) => (m.id === id ? { ...m, role } : m))
      return members.find((m) => m.id === id)!
    })
    vi.mocked(api.removeMember).mockImplementation(async (id: string) => {
      members = members.filter((m) => m.id !== id)
    })

    const wrapper = mount(MembersTab, { props: { me: ada } })
    await flushPromises()

    expect(rowFor(wrapper, 'Ada Lovelace')).toBeTruthy()
    expect(rowFor(wrapper, 'Linus Torvalds')).toBeTruthy()

    // Invite a member.
    const inviteOpen = wrapper.findAll('button').find((b) => b.text() === '+ Invite')!
    await inviteOpen.trigger('click')
    await wrapper.find('.inline-form input').setValue('New Person')
    await wrapper.find('.inline-form input[type="email"]').setValue('new@acme.test')
    await wrapper.find('.inline-form').trigger('submit')
    await flushPromises()

    const newRow = rowFor(wrapper, 'New Person')
    expect(newRow, 'invited member should appear in the table').toBeTruthy()

    // Change that member's role via its row select.
    const roleSelect = newRow!.find('select')
    expect(roleSelect.exists(), 'expected a role select for a non-self member').toBe(true)
    await roleSelect.setValue('admin')
    await flushPromises()

    // setValue() writes 'admin' straight onto the native select element, so
    // reading that same element's .value back proves nothing: the select is
    // bound with :value/@change rather than v-model, so nothing forces a
    // render from application state, and the value read is just test-utils'
    // own write. Assert what only the component can produce instead: that
    // its change handler called the API with the right id and role.
    expect(api.changeMemberRole).toHaveBeenCalledWith('u_new', 'admin')

    // Then prove the row itself reflects the new role from the app's own
    // state, not from the write above. A fresh mount reads the mock's
    // updated data (changeMemberRole mutated the shared `members` array)
    // into a select element setValue never touched.
    const freshWrapper = mount(MembersTab, { props: { me: ada } })
    await flushPromises()
    const freshRow = rowFor(freshWrapper, 'New Person')!
    expect(
      (freshRow.find('select').element as HTMLSelectElement).value,
      'role change should be reflected the next time this data is rendered',
    ).toBe('admin')

    // Remove the member.
    const updatedRow = rowFor(wrapper, 'New Person')!
    const removeBtn = updatedRow.findAll('button').find((b) => b.text() === 'Remove')!
    await removeBtn.trigger('click')
    await flushPromises()

    expect(rowFor(wrapper, 'New Person'), 'removed member must disappear').toBeUndefined()
    expect(rowFor(wrapper, 'Ada Lovelace'), 'unrelated members remain').toBeTruthy()
  })

  it('signed in as grace, only globex members are visible', async () => {
    vi.mocked(api.listMembers).mockResolvedValue([grace, alan])

    const wrapper = mount(MembersTab, { props: { me: grace } })
    await flushPromises()

    const text = wrapper.text()
    // Positive control first: an empty render would otherwise also
    // "pass" the negative assertions below.
    expect(text).toContain('Grace Hopper')
    expect(text).toContain('Alan Turing')
    expect(text).not.toContain('Ada Lovelace')
    expect(text).not.toContain('Linus Torvalds')
  })
})
