import { describe, it, expect, vi, beforeEach } from 'vitest'

// vi.hoisted 确保 mock 变量在 vi.mock 工厂执行时已初始化
const { mockOn, mockOff } = vi.hoisted(() => ({
  mockOn: vi.fn(),
  mockOff: vi.fn(),
}))

vi.mock('@wailsio/runtime', () => ({
  Events: {
    On: mockOn,
    Off: mockOff,
  },
}))

import { createWailsEventListener } from '../wailsEvents'

describe('utils/wailsEvents', () => {
  beforeEach(() => {
    mockOn.mockClear()
    mockOff.mockClear()
    mockOn.mockImplementation(() => {})
    mockOff.mockImplementation(() => {})
  })

  it('初始状态 isListening=false', () => {
    const listener = createWailsEventListener('test', [])
    expect(listener.isListening()).toBe(false)
  })

  it('start 注册所有订阅通道', () => {
    const sub1 = { channel: 'ch1', handler: vi.fn() }
    const sub2 = { channel: 'ch2', handler: vi.fn() }
    const listener = createWailsEventListener('test', [sub1, sub2])

    listener.start()
    expect(listener.isListening()).toBe(true)
    expect(mockOn).toHaveBeenCalledTimes(2)
    expect(mockOn).toHaveBeenCalledWith('ch1', sub1.handler)
    expect(mockOn).toHaveBeenCalledWith('ch2', sub2.handler)
  })

  it('start 幂等：重复调用不重复注册', () => {
    const sub = { channel: 'ch1', handler: vi.fn() }
    const listener = createWailsEventListener('test', [sub])

    listener.start()
    listener.start()
    expect(mockOn).toHaveBeenCalledTimes(1)
    expect(listener.isListening()).toBe(true)
  })

  it('stop 注销所有订阅通道', () => {
    const sub1 = { channel: 'ch1', handler: vi.fn() }
    const sub2 = { channel: 'ch2', handler: vi.fn() }
    const listener = createWailsEventListener('test', [sub1, sub2])

    listener.start()
    listener.stop()
    expect(listener.isListening()).toBe(false)
    expect(mockOff).toHaveBeenCalledTimes(2)
    expect(mockOff).toHaveBeenCalledWith('ch1')
    expect(mockOff).toHaveBeenCalledWith('ch2')
  })

  it('stop 幂等：未 start 时调用 stop 不报错', () => {
    const listener = createWailsEventListener('test', [{ channel: 'ch1', handler: vi.fn() }])
    listener.stop() // 不应抛错
    expect(listener.isListening()).toBe(false)
    expect(mockOff).not.toHaveBeenCalled()
  })

  it('stop 幂等：重复调用不重复注销', () => {
    const sub = { channel: 'ch1', handler: vi.fn() }
    const listener = createWailsEventListener('test', [sub])

    listener.start()
    listener.stop()
    listener.stop()
    expect(mockOff).toHaveBeenCalledTimes(1)
  })

  it('start 失败时不抛错（try/catch 包裹）', () => {
    mockOn.mockImplementation(() => {
      throw new Error('mock error')
    })
    const spy = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const listener = createWailsEventListener('test', [{ channel: 'ch1', handler: vi.fn() }])

    expect(() => listener.start()).not.toThrow()
    expect(spy).toHaveBeenCalled()
    spy.mockRestore()
  })

  it('stop 失败时不抛错（try/catch 包裹）', () => {
    const sub = { channel: 'ch1', handler: vi.fn() }
    const listener = createWailsEventListener('test', [sub])
    listener.start()
    mockOff.mockImplementation(() => {
      throw new Error('mock error')
    })
    const spy = vi.spyOn(console, 'warn').mockImplementation(() => {})

    expect(() => listener.stop()).not.toThrow()
    expect(spy).toHaveBeenCalled()
    spy.mockRestore()
  })

  it('空订阅列表：start/stop 正常工作', () => {
    const listener = createWailsEventListener('empty', [])
    listener.start()
    expect(listener.isListening()).toBe(true)
    listener.stop()
    expect(listener.isListening()).toBe(false)
    expect(mockOn).not.toHaveBeenCalled()
    expect(mockOff).not.toHaveBeenCalled()
  })

  it('handler 在事件触发时被调用（通过 Events.On 注册）', () => {
    const handler = vi.fn()
    const listener = createWailsEventListener('test', [{ channel: 'evt', handler }])

    listener.start()
    // 模拟后端发射事件：取出 On 注册时的回调并调用
    const registeredCallback = mockOn.mock.calls[0][1]
    registeredCallback({ data: { foo: 'bar' } })
    expect(handler).toHaveBeenCalledWith({ data: { foo: 'bar' } })
  })
})
