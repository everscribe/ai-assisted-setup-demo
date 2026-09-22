import type { Api, Resource, User } from '../api.ts'
import { renderResourceView } from './ResourceView.ts'

export interface ResourcesTabProps {
  me: User
  api: Api
}

export function renderResourcesTab(props: ResourcesTabProps): HTMLElement {
  const container = document.createElement('div')
  container.className = 'boards-tab'

  let resources: Resource[] = []
  let selectedId: string | null = null
  let creating = false
  let newName = ''
  let newDescription = ''
  let newSensitivity: Resource['sensitivity'] = 'internal'
  let error: string | null = null

  const isAdmin = props.me.role === 'admin'

  async function refresh() {
    try {
      resources = await props.api.listResources()
      if (!selectedId && resources.length > 0) selectedId = resources[0].id
      error = null
    } catch (err) {
      error = String(err)
    }
    draw()
  }

  async function onDecommission(id: string) {
    if (!confirm('Decommission this resource? New access requests will be refused.')) return
    try {
      await props.api.decommissionResource(id)
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

    const aside = document.createElement('aside')
    aside.className = 'board-list'

    const listHeader = document.createElement('div')
    listHeader.className = 'board-list-header'
    const h2 = document.createElement('h2')
    h2.textContent = 'Resources'
    listHeader.appendChild(h2)
    if (isAdmin) {
      const addBtn = document.createElement('button')
      addBtn.textContent = '+ Register'
      addBtn.addEventListener('click', () => {
        creating = true
        draw()
      })
      listHeader.appendChild(addBtn)
    }
    aside.appendChild(listHeader)

    if (creating) {
      const form = document.createElement('form')
      form.className = 'inline-form vstack'

      const nameInput = document.createElement('input')
      nameInput.placeholder = 'resource name (e.g. production-postgres)'
      nameInput.required = true
      nameInput.value = newName
      nameInput.addEventListener('input', () => {
        newName = nameInput.value
      })
      form.appendChild(nameInput)

      const descInput = document.createElement('input')
      descInput.placeholder = 'description'
      descInput.value = newDescription
      descInput.addEventListener('input', () => {
        newDescription = descInput.value
      })
      form.appendChild(descInput)

      const sensLabel = document.createElement('label')
      const sensSpan = document.createElement('span')
      sensSpan.textContent = 'Sensitivity'
      const select = document.createElement('select')
      for (const opt of ['public', 'internal', 'confidential', 'restricted'] as const) {
        const option = document.createElement('option')
        option.value = opt
        option.textContent = opt
        select.appendChild(option)
      }
      select.value = newSensitivity
      select.addEventListener('change', () => {
        newSensitivity = select.value as Resource['sensitivity']
      })
      sensLabel.append(sensSpan, select)
      form.appendChild(sensLabel)

      const row = document.createElement('div')
      row.className = 'row'
      const submitBtn = document.createElement('button')
      submitBtn.type = 'submit'
      submitBtn.textContent = 'Register'
      const cancelBtn = document.createElement('button')
      cancelBtn.type = 'button'
      cancelBtn.className = 'secondary'
      cancelBtn.textContent = 'Cancel'
      cancelBtn.addEventListener('click', () => {
        creating = false
        newName = ''
        newDescription = ''
        draw()
      })
      row.append(submitBtn, cancelBtn)
      form.appendChild(row)

      form.addEventListener('submit', async (e) => {
        e.preventDefault()
        if (!newName.trim()) return
        try {
          await props.api.registerResource({
            name: newName.trim(),
            description: newDescription.trim(),
            sensitivity: newSensitivity,
          })
          newName = ''
          newDescription = ''
          newSensitivity = 'internal'
          creating = false
          await refresh()
        } catch (err) {
          error = String(err)
          draw()
        }
      })
      aside.appendChild(form)
    }

    const ul = document.createElement('ul')
    for (const r of resources) {
      const li = document.createElement('li')
      if (r.id === selectedId) li.className = 'selected'
      li.addEventListener('click', () => {
        selectedId = r.id
        draw()
      })

      const nameRow = document.createElement('div')
      nameRow.className = 'board-name'
      const code = document.createElement('code')
      code.textContent = r.name
      nameRow.appendChild(code)
      const pill = document.createElement('span')
      pill.className = `sensitivity-pill sens-${r.sensitivity}`
      pill.textContent = r.sensitivity
      nameRow.appendChild(pill)
      if (r.decommission_at) {
        const badge = document.createElement('span')
        badge.className = 'badge'
        badge.textContent = 'decommissioned'
        nameRow.appendChild(badge)
      }
      li.appendChild(nameRow)

      if (isAdmin && !r.decommission_at) {
        const actions = document.createElement('div')
        actions.className = 'board-actions'
        actions.addEventListener('click', (e) => e.stopPropagation())
        const decommBtn = document.createElement('button')
        decommBtn.className = 'danger'
        decommBtn.textContent = 'Decommission'
        decommBtn.addEventListener('click', () => onDecommission(r.id))
        actions.appendChild(decommBtn)
        li.appendChild(actions)
      }
      ul.appendChild(li)
    }
    if (resources.length === 0 && !creating) {
      const li = document.createElement('li')
      li.className = 'empty'
      li.textContent = isAdmin
        ? 'No resources yet - register one.'
        : 'No resources yet - ask an admin to register one.'
      ul.appendChild(li)
    }
    aside.appendChild(ul)
    container.appendChild(aside)

    const main = document.createElement('main')
    main.className = 'board-detail'
    const selected = resources.find((r) => r.id === selectedId)
    if (selected) {
      main.appendChild(renderResourceView({ resource: selected, me: props.me, api: props.api }))
    } else {
      const empty = document.createElement('div')
      empty.className = 'empty-state'
      empty.textContent = 'Pick a resource to see its access requests.'
      main.appendChild(empty)
    }
    container.appendChild(main)
  }

  draw()
  void refresh()

  return container
}
