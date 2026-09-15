import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import LoginScreen from '../src/components/LoginScreen.vue'

describe('LoginScreen', () => {
  it('quick-pick button logs in with that email', async () => {
    const wrapper = mount(LoginScreen, { props: { error: null } })
    const button = wrapper.findAll('button').find((b) => b.text().includes('Grace'))
    expect(button, 'expected a Grace quick-pick button').toBeTruthy()
    await button!.trigger('click')
    expect(wrapper.emitted('login')).toEqual([['grace@globex.test']])
  })

  it('submitting the form logs in with the typed email', async () => {
    const wrapper = mount(LoginScreen, { props: { error: null } })
    await wrapper.find('input[type="email"]').setValue('ada@acme.test')
    await wrapper.find('form').trigger('submit')
    expect(wrapper.emitted('login')).toEqual([['ada@acme.test']])
  })

  it('error banner renders the message and dismiss fires the event', async () => {
    const wrapper = mount(LoginScreen, { props: { error: 'no user with that email' } })
    expect(wrapper.text()).toContain('no user with that email')
    const dismissBtn = wrapper.findAll('button').find((b) => b.text() === 'dismiss')
    expect(dismissBtn).toBeTruthy()
    await dismissBtn!.trigger('click')
    expect(wrapper.emitted('dismissError')).toBeTruthy()
  })
})
