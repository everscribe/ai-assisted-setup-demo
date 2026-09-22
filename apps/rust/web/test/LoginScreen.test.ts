import test from 'node:test'
import assert from 'node:assert/strict'
import { setupDom, fire } from '../testutil/dom.ts'

setupDom()
const { renderLoginScreen } = await import('../src/views/LoginScreen.ts')

test('quick-pick button logs in with that email', () => {
  const calls: string[] = []
  const el = renderLoginScreen({
    error: null,
    onLogin: (email) => calls.push(email),
    onDismissError: () => {},
  })
  const button = [...el.querySelectorAll('button')].find((b) => b.textContent?.includes('Grace'))
  assert.ok(button, 'expected a Grace quick-pick button')
  button!.click()
  assert.deepEqual(calls, ['grace@globex.test'])
})

test('submitting the form logs in with the typed email', () => {
  const calls: string[] = []
  const el = renderLoginScreen({
    error: null,
    onLogin: (email) => calls.push(email),
    onDismissError: () => {},
  })
  const input = el.querySelector('input[type="email"]') as HTMLInputElement
  input.value = 'ada@acme.test'
  fire(input, 'input')
  const form = el.querySelector('form')!
  fire(form, 'submit')
  assert.deepEqual(calls, ['ada@acme.test'])
})

test('error banner renders the message and dismiss fires the callback', () => {
  let dismissed = false
  const el = renderLoginScreen({
    error: 'no user with that email',
    onLogin: () => {},
    onDismissError: () => {
      dismissed = true
    },
  })
  assert.ok(el.textContent?.includes('no user with that email'))
  const dismissBtn = [...el.querySelectorAll('button')].find((b) => b.textContent === 'dismiss')
  assert.ok(dismissBtn)
  dismissBtn!.click()
  assert.equal(dismissed, true)
})
