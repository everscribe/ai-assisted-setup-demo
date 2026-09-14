import './styles.css'
import { api, clearUserId, getUserId, setUserId, type User } from './api.ts'
import { renderLoginScreen } from './views/LoginScreen.ts'
import { renderDashboard } from './views/Dashboard.ts'

// One module-level state object, plus a render() that rebuilds the
// active top-level view (login vs dashboard). Nested views own their
// own local state and re-render themselves; this object only tracks
// what decides which top-level view is active.
interface AppState {
  me: User | null
  error: string | null
}

const state: AppState = { me: null, error: null }

const root = document.getElementById('app')!

function render() {
  root.replaceChildren(
    state.me
      ? renderDashboard({ me: state.me, api, onLogout: handleLogout })
      : renderLoginScreen({
          error: state.error,
          onLogin: handleLogin,
          onDismissError: () => {
            state.error = null
            render()
          },
        }),
  )
}

async function handleLogin(email: string) {
  try {
    const user = await api.login(email)
    setUserId(user.id)
    state.me = user
    state.error = null
  } catch (err) {
    state.error = String(err)
  }
  render()
}

async function handleLogout() {
  try {
    await api.logout()
  } catch {
    // server-side logout has no state to clear; ignore the result
  }
  clearUserId()
  state.me = null
  render()
}

async function init() {
  if (getUserId()) {
    try {
      state.me = await api.me()
    } catch {
      clearUserId()
    }
  }
  render()
}

void init()
