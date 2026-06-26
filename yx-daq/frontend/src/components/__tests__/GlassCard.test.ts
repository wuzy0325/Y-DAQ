import { mount } from '@vue/test-utils'
import { describe, it, expect } from 'vitest'
import GlassCard from '../GlassCard.vue'

describe('GlassCard', () => {
  it('renders slot content', () => {
    const wrapper = mount(GlassCard, {
      slots: { default: 'Card content' },
    })
    expect(wrapper.text()).toContain('Card content')
  })

  it('renders title when provided', () => {
    const wrapper = mount(GlassCard, {
      props: { title: 'Test Card' },
    })
    expect(wrapper.text()).toContain('Test Card')
  })

  it('does not render header when no title', () => {
    const wrapper = mount(GlassCard, {})
    expect(wrapper.find('.card-header').exists()).toBe(false)
  })

  it('renders icon when provided', () => {
    const wrapper = mount(GlassCard, {
      props: { title: 'Test', icon: '🔬' },
    })
    expect(wrapper.find('.card-icon').text()).toBe('🔬')
  })

  it('renders actions slot', () => {
    const wrapper = mount(GlassCard, {
      props: { title: 'Test' },
      slots: { actions: '<button>Action</button>' },
    })
    expect(wrapper.find('.card-actions').text()).toContain('Action')
  })

  it('applies elevated class when elevated prop is true', () => {
    const wrapper = mount(GlassCard, {
      props: { elevated: true },
    })
    expect(wrapper.find('.glass-card').classes()).toContain('elevated')
  })

  it('does not apply elevated class by default', () => {
    const wrapper = mount(GlassCard, {})
    expect(wrapper.find('.glass-card').classes()).not.toContain('elevated')
  })
})
