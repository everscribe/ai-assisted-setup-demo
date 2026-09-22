import type { Api, User } from '../api.ts'
import { renderResourcesTab } from './ResourcesTab.ts'
import { renderMembersTab } from './MembersTab.ts'

export interface DashboardProps {
  me: User
  api: Api
  onLogout: () => void
}

type Tab = 'resources' | 'members'

export function renderDashboard(props: DashboardProps): HTMLElement {
  const container = document.createElement('div')
  container.className = 'app'

  let tab: Tab = 'resources'
  const content = document.createElement('div')

  function drawContent() {
    content.replaceChildren(
      tab === 'resources'
        ? renderResourcesTab({ me: props.me, api: props.api })
        : renderMembersTab({ me: props.me, api: props.api }),
    )
  }

  const header = document.createElement('header')
  header.className = 'topbar'
  const left = document.createElement('div')
  const h1 = document.createElement('h1')
  h1.textContent = 'Access portal'
  left.appendChild(h1)
  const sub = document.createElement('p')
  sub.className = 'sub'
  sub.append('Signed in as ')
  const strong = document.createElement('strong')
  strong.textContent = props.me.name
  sub.appendChild(strong)
  const rolePill = document.createElement('span')
  rolePill.className = 'role-pill'
  rolePill.textContent = props.me.role
  sub.appendChild(rolePill)
  sub.append(' - org ')
  const orgCode = document.createElement('code')
  orgCode.textContent = props.me.organization_id
  sub.appendChild(orgCode)
  left.appendChild(sub)
  header.appendChild(left)

  const logoutBtn = document.createElement('button')
  logoutBtn.className = 'logout'
  logoutBtn.textContent = 'Sign out'
  logoutBtn.addEventListener('click', props.onLogout)
  header.appendChild(logoutBtn)
  container.appendChild(header)

  const nav = document.createElement('nav')
  nav.className = 'tabs'
  nav.setAttribute('role', 'tablist')

  const resourcesBtn = document.createElement('button')
  resourcesBtn.setAttribute('role', 'tab')
  resourcesBtn.textContent = 'Resources'

  const membersBtn = document.createElement('button')
  membersBtn.setAttribute('role', 'tab')
  membersBtn.textContent = 'Members'

  function updateTabButtons() {
    resourcesBtn.className = tab === 'resources' ? 'active' : ''
    membersBtn.className = tab === 'members' ? 'active' : ''
  }

  resourcesBtn.addEventListener('click', () => {
    if (tab === 'resources') return
    tab = 'resources'
    updateTabButtons()
    drawContent()
  })
  membersBtn.addEventListener('click', () => {
    if (tab === 'members') return
    tab = 'members'
    updateTabButtons()
    drawContent()
  })
  updateTabButtons()

  nav.append(resourcesBtn, membersBtn)
  container.appendChild(nav)

  drawContent()
  container.appendChild(content)

  return container
}
