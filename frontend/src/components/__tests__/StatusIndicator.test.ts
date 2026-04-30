import { mount } from '@vue/test-utils'
import { describe, it, expect } from 'vitest'
import StatusIndicator from '../StatusIndicator.vue'

describe('StatusIndicator', () => {
  it('renders dot element', () => {
    const wrapper = mount(StatusIndicator, {
      props: { status: 'connected' },
    })
    expect(wrapper.find('.dot').exists()).toBe(true)
  })

  it('applies correct status class', () => {
    const wrapper = mount(StatusIndicator, {
      props: { status: 'error' },
    })
    expect(wrapper.find('.status-indicator').classes()).toContain('error')
  })

  it('renders label when provided', () => {
    const wrapper = mount(StatusIndicator, {
      props: { status: 'connected', label: '已连接' },
    })
    expect(wrapper.text()).toContain('已连接')
  })

  it('does not render label when not provided', () => {
    const wrapper = mount(StatusIndicator, {
      props: { status: 'connected' },
    })
    expect(wrapper.find('.label').exists()).toBe(false)
  })

  it('applies pulse class when animated', () => {
    const wrapper = mount(StatusIndicator, {
      props: { status: 'running', animated: true },
    })
    expect(wrapper.find('.status-indicator').classes()).toContain('pulse')
  })

  it('does not apply pulse class by default', () => {
    const wrapper = mount(StatusIndicator, {
      props: { status: 'running' },
    })
    expect(wrapper.find('.status-indicator').classes()).not.toContain('pulse')
  })

  it('supports all status types', () => {
    const statuses = ['connected', 'disconnected', 'error', 'running', 'warning'] as const
    for (const status of statuses) {
      const wrapper = mount(StatusIndicator, {
        props: { status },
      })
      expect(wrapper.find('.status-indicator').classes()).toContain(status)
    }
  })
})
