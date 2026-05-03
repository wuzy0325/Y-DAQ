import { mount } from '@vue/test-utils'
import { describe, it, expect } from 'vitest'
import ValueDisplay from '../ValueDisplay.vue'

describe('ValueDisplay', () => {
  it('renders numeric value with default precision', () => {
    const wrapper = mount(ValueDisplay, {
      props: { value: 123.456789 },
    })
    expect(wrapper.text()).toContain('123.457')
  })

  it('renders value with custom precision', () => {
    const wrapper = mount(ValueDisplay, {
      props: { value: 123.456789, precision: 2 },
    })
    expect(wrapper.text()).toContain('123.46')
  })

  it('renders unit when provided', () => {
    const wrapper = mount(ValueDisplay, {
      props: { value: 25.5, unit: '°C' },
    })
    expect(wrapper.text()).toContain('°C')
  })

  it('renders "--" for NaN value', () => {
    const wrapper = mount(ValueDisplay, {
      props: { value: NaN },
    })
    expect(wrapper.text()).toContain('--')
  })

  it('applies custom color style', () => {
    const wrapper = mount(ValueDisplay, {
      props: { value: 100, color: '#ff3366' },
    })
    const valueEl = wrapper.find('.value')
    expect(valueEl.attributes('style')).toContain('color: #ff3366')
  })

  it('uses default cyan color when no color specified', () => {
    const wrapper = mount(ValueDisplay, {
      props: { value: 100 },
    })
    const valueEl = wrapper.find('.value')
    expect(valueEl.attributes('style')).toContain('color: #00f5ff')
  })
})
