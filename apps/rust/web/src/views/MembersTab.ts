import type { Api, User } from '../api.ts'

export interface MembersTabProps {
  me: User
  api: Api
}

export function renderMembersTab(props: MembersTabProps): HTMLElement {
  const container = document.createElement('div')
  container.className = 'members-tab'

  let members: User[] = []
  let inviting = false
  let name = ''
  let email = ''
  let role: 'admin' | 'member' = 'member'
  let error: string | null = null

  const isAdmin = props.me.role === 'admin'

  async function refresh() {
    try {
      members = await props.api.listMembers()
      error = null
    } catch (err) {
      error = String(err)
    }
    draw()
  }

  async function onChangeRole(id: string, next: 'admin' | 'member') {
    try {
      await props.api.changeMemberRole(id, next)
      await refresh()
    } catch (err) {
      error = String(err)
      draw()
    }
  }

  async function onRemove(id: string) {
    if (!confirm('Remove this member?')) return
    try {
      await props.api.removeMember(id)
      await refresh()
    } catch (err) {
      error = String(err)
      draw()
    }
  }

  function draw() {
    container.replaceChildren()

    if (error) {
      const banner = document.createElement('div')
      banner.className = 'error'
      banner.append(document.createTextNode(error + ' '))
      const dismiss = document.createElement('button')
      dismiss.textContent = 'dismiss'
      dismiss.addEventListener('click', () => {
        error = null
        draw()
      })
      banner.appendChild(dismiss)
      container.appendChild(banner)
    }

    const header = document.createElement('div')
    header.className = 'members-header'
    const h2 = document.createElement('h2')
    h2.textContent = `Organization members (${members.length})`
    header.appendChild(h2)
    if (isAdmin) {
      const inviteBtn = document.createElement('button')
      inviteBtn.textContent = '+ Invite'
      inviteBtn.addEventListener('click', () => {
        inviting = true
        draw()
      })
      header.appendChild(inviteBtn)
    }
    container.appendChild(header)

    if (!isAdmin) {
      const p = document.createElement('p')
      p.className = 'muted'
      p.textContent = 'Only admins can invite, remove, or change roles for members.'
      container.appendChild(p)
    }

    if (inviting) {
      const form = document.createElement('form')
      form.className = 'inline-form vstack'

      const nameInput = document.createElement('input')
      nameInput.placeholder = 'Name'
      nameInput.required = true
      nameInput.value = name
      nameInput.addEventListener('input', () => {
        name = nameInput.value
      })
      form.appendChild(nameInput)

      const emailInput = document.createElement('input')
      emailInput.type = 'email'
      emailInput.placeholder = 'email@org.test'
      emailInput.required = true
      emailInput.value = email
      emailInput.addEventListener('input', () => {
        email = emailInput.value
      })
      form.appendChild(emailInput)

      const roleLabel = document.createElement('label')
      const roleSpan = document.createElement('span')
      roleSpan.textContent = 'Role'
      const select = document.createElement('select')
      const memberOpt = document.createElement('option')
      memberOpt.value = 'member'
      memberOpt.textContent = 'Member'
      const adminOpt = document.createElement('option')
      adminOpt.value = 'admin'
      adminOpt.textContent = 'Admin'
      select.append(memberOpt, adminOpt)
      select.value = role
      select.addEventListener('change', () => {
        role = select.value as 'admin' | 'member'
      })
      roleLabel.append(roleSpan, select)
      form.appendChild(roleLabel)

      const row = document.createElement('div')
      row.className = 'row'
      const submitBtn = document.createElement('button')
      submitBtn.type = 'submit'
      submitBtn.textContent = 'Invite'
      const cancelBtn = document.createElement('button')
      cancelBtn.type = 'button'
      cancelBtn.className = 'secondary'
      cancelBtn.textContent = 'Cancel'
      cancelBtn.addEventListener('click', () => {
        inviting = false
        draw()
      })
      row.append(submitBtn, cancelBtn)
      form.appendChild(row)

      form.addEventListener('submit', async (e) => {
        e.preventDefault()
        try {
          await props.api.inviteMember({ name: name.trim(), email: email.trim(), role })
          name = ''
          email = ''
          role = 'member'
          inviting = false
          await refresh()
        } catch (err) {
          error = String(err)
          draw()
        }
      })
      container.appendChild(form)
    }

    const table = document.createElement('table')
    table.className = 'members-table'
    const thead = document.createElement('thead')
    const headRow = document.createElement('tr')
    for (const text of ['Name', 'Email', 'Role']) {
      const th = document.createElement('th')
      th.textContent = text
      headRow.appendChild(th)
    }
    if (isAdmin) headRow.appendChild(document.createElement('th'))
    thead.appendChild(headRow)
    table.appendChild(thead)

    const tbody = document.createElement('tbody')
    for (const m of members) {
      const tr = document.createElement('tr')

      const nameTd = document.createElement('td')
      nameTd.textContent = m.name
      tr.appendChild(nameTd)

      const emailTd = document.createElement('td')
      emailTd.textContent = m.email
      tr.appendChild(emailTd)

      const roleTd = document.createElement('td')
      if (isAdmin && m.id !== props.me.id) {
        const select = document.createElement('select')
        const memberOpt = document.createElement('option')
        memberOpt.value = 'member'
        memberOpt.textContent = 'member'
        const adminOpt = document.createElement('option')
        adminOpt.value = 'admin'
        adminOpt.textContent = 'admin'
        select.append(memberOpt, adminOpt)
        select.value = m.role
        select.addEventListener('change', () => {
          void onChangeRole(m.id, select.value as 'admin' | 'member')
        })
        roleTd.appendChild(select)
      } else {
        const pill = document.createElement('span')
        pill.className = 'role-pill'
        pill.textContent = m.role
        roleTd.appendChild(pill)
      }
      tr.appendChild(roleTd)

      if (isAdmin) {
        const actionsTd = document.createElement('td')
        if (m.id !== props.me.id) {
          const removeBtn = document.createElement('button')
          removeBtn.className = 'danger'
          removeBtn.textContent = 'Remove'
          removeBtn.addEventListener('click', () => onRemove(m.id))
          actionsTd.appendChild(removeBtn)
        }
        tr.appendChild(actionsTd)
      }
      tbody.appendChild(tr)
    }
    table.appendChild(tbody)
    container.appendChild(table)
  }

  draw()
  void refresh()

  return container
}
