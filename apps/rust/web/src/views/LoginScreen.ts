// A handful of seeded email addresses ship with every backend; the
// login screen renders these as quick-pick buttons so anyone landing
// on the demo can sign in without typing.
const QUICK_PICKS: { email: string; label: string }[] = [
  { email: 'ada@acme.test', label: 'Ada (Acme - admin)' },
  { email: 'linus@acme.test', label: 'Linus (Acme - member)' },
  { email: 'grace@globex.test', label: 'Grace (Globex - admin)' },
  { email: 'alan@globex.test', label: 'Alan (Globex - member)' },
]

export interface LoginScreenProps {
  error: string | null
  onLogin: (email: string) => void
  onDismissError: () => void
}

export function renderLoginScreen(props: LoginScreenProps): HTMLElement {
  const container = document.createElement('div')
  container.className = 'login'

  const h1 = document.createElement('h1')
  h1.textContent = 'Access portal'
  container.appendChild(h1)

  const sub = document.createElement('p')
  sub.className = 'sub'
  sub.textContent = 'Demo - vanilla TypeScript + Rust - multi-tenant'
  container.appendChild(sub)

  if (props.error) {
    const banner = document.createElement('div')
    banner.className = 'error'
    banner.append(document.createTextNode(props.error + ' '))
    const dismiss = document.createElement('button')
    dismiss.textContent = 'dismiss'
    dismiss.addEventListener('click', props.onDismissError)
    banner.appendChild(dismiss)
    container.appendChild(banner)
  }

  let email = ''

  const form = document.createElement('form')
  form.className = 'login-form'
  const label = document.createElement('label')
  const span = document.createElement('span')
  span.textContent = 'Email'
  const input = document.createElement('input')
  input.type = 'email'
  input.placeholder = 'ada@acme.test'
  input.required = true
  input.addEventListener('input', () => {
    email = input.value
  })
  label.append(span, input)
  form.appendChild(label)

  const submit = document.createElement('button')
  submit.type = 'submit'
  submit.className = 'primary'
  submit.textContent = 'Sign in'
  form.appendChild(submit)

  form.addEventListener('submit', (e) => {
    e.preventDefault()
    if (email) props.onLogin(email)
  })
  container.appendChild(form)

  const quickPicks = document.createElement('div')
  quickPicks.className = 'quick-picks'
  const label2 = document.createElement('span')
  label2.className = 'muted'
  label2.textContent = 'Quick sign-in:'
  quickPicks.appendChild(label2)
  for (const q of QUICK_PICKS) {
    const btn = document.createElement('button')
    btn.textContent = q.label
    btn.addEventListener('click', () => props.onLogin(q.email))
    quickPicks.appendChild(btn)
  }
  container.appendChild(quickPicks)

  return container
}
