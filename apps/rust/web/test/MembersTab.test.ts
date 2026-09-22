import test from 'node:test'
import assert from 'node:assert/strict'
import { setupDom, flush, fire } from '../testutil/dom.ts'
import { ada, alan, grace, linus } from '../testutil/fixtures.ts'

setupDom()
const { renderMembersTab } = await import('../src/views/MembersTab.ts')

function rowFor(el: HTMLElement, name: string): HTMLTableRowElement | undefined {
  return [...el.querySelectorAll('tbody tr')].find((tr) =>
    tr.textContent?.includes(name),
  ) as HTMLTableRowElement | undefined
}

test('members list renders, invite appears, role change and removal are reflected', async () => {
  let members = [ada, linus]
  const api = {
    listMembers: async () => members,
    inviteMember: async (input: { name: string; email: string; role: 'admin' | 'member' }) => {
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
    changeMemberRole: async (id: string, role: 'admin' | 'member') => {
      members = members.map((m) => (m.id === id ? { ...m, role } : m))
      return members.find((m) => m.id === id)!
    },
    removeMember: async (id: string) => {
      members = members.filter((m) => m.id !== id)
    },
  } as any

  const el = renderMembersTab({ me: ada, api })
  await flush()

  assert.ok(rowFor(el, 'Ada Lovelace'))
  assert.ok(rowFor(el, 'Linus Torvalds'))

  // Invite a member.
  const inviteOpen = [...el.querySelectorAll('button')].find((b) => b.textContent === '+ Invite')!
  inviteOpen.click()
  const nameInput = el.querySelector('.inline-form input') as HTMLInputElement
  nameInput.value = 'New Person'
  fire(nameInput, 'input')
  const emailInput = el.querySelector('.inline-form input[type="email"]') as HTMLInputElement
  emailInput.value = 'new@acme.test'
  fire(emailInput, 'input')
  fire(el.querySelector('.inline-form') as HTMLFormElement, 'submit')
  await flush()

  const newRow = rowFor(el, 'New Person')
  assert.ok(newRow, 'invited member should appear in the table')

  // Change that member's role via its row select.
  const roleSelect = newRow!.querySelector('select') as HTMLSelectElement
  assert.ok(roleSelect, 'expected a role select for a non-self member')
  roleSelect.value = 'admin'
  fire(roleSelect, 'change')
  await flush()

  const updatedRow = rowFor(el, 'New Person')!
  assert.ok(
    (updatedRow.querySelector('select') as HTMLSelectElement).value === 'admin',
    'role change should be reflected in the row',
  )

  // Remove the member.
  const removeBtn = [...updatedRow.querySelectorAll('button')].find((b) => b.textContent === 'Remove')!
  removeBtn.click()
  await flush()

  assert.equal(rowFor(el, 'New Person'), undefined, 'removed member must disappear')
  assert.ok(rowFor(el, 'Ada Lovelace'), 'unrelated members remain')
})

test('signed in as grace, only globex members are visible', async () => {
  const el = renderMembersTab({
    me: grace,
    api: { listMembers: async () => [grace, alan] } as any,
  })
  await flush()

  const text = el.textContent ?? ''
  // Positive control first: an empty render would otherwise also
  // "pass" the negative assertion below.
  assert.ok(text.includes('Grace Hopper'), 'expected a globex member to be present')
  assert.ok(text.includes('Alan Turing'), 'expected a globex member to be present')
  assert.ok(!text.includes('Ada Lovelace'), 'acme member must not leak into globex view')
  assert.ok(!text.includes('Linus Torvalds'), 'acme member must not leak into globex view')
})
