import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { ref, effectScope, defineComponent } from 'vue'
import { mount } from '@vue/test-utils'
import { useAutoSaveConfig } from '../useAutoSaveConfig'

describe('composables/useAutoSaveConfig', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  function setup<TConfig>(config: TConfig, options: Partial<{
    saveConfig: ReturnType<typeof vi.fn>
    stopRealtimeMonitor: ReturnType<typeof vi.fn>
    startRealtimeMonitor: ReturnType<typeof vi.fn>
    isRunning: boolean
    debounceMs: number
  }> = {}) {
    const saveConfig = options.saveConfig ?? vi.fn()
    const stopRealtimeMonitor = options.stopRealtimeMonitor ?? vi.fn()
    const startRealtimeMonitor = options.startRealtimeMonitor ?? vi.fn()
    const configRef = ref(config) as any
    const isRunningRef = ref(options.isRunning ?? false)

    const scope = effectScope()
    const result = scope.run(() => useAutoSaveConfig({
      config: configRef,
      isRunning: isRunningRef,
      // MockInstance 可调用，但 TS 不识别其与函数签名兼容，用 any 桥接
      saveConfig: saveConfig as any,
      stopRealtimeMonitor: stopRealtimeMonitor as any,
      startRealtimeMonitor: startRealtimeMonitor as any,
      debounceMs: options.debounceMs,
    }))!

    return { ...result, saveConfig, stopRealtimeMonitor, startRealtimeMonitor, configRef, isRunningRef, scope }
  }

  it('初始 isInitializing=true', () => {
    const { isInitializing } = setup({ a: 1 })
    expect(isInitializing.value).toBe(true)
  })

  it('markInitialized 后 isInitializing=false', () => {
    const { isInitializing, markInitialized } = setup({ a: 1 })
    markInitialized()
    expect(isInitializing.value).toBe(false)
  })

  it('isInitializing=true 时 config 变更不触发 saveConfig', () => {
    const { configRef, saveConfig } = setup({ a: 1 })
    configRef.value.a = 2
    vi.advanceTimersByTime(1000)
    expect(saveConfig).not.toHaveBeenCalled()
  })

  it('markInitialized 后 config 变更触发防抖 saveConfig', async () => {
    const { configRef, saveConfig, markInitialized } = setup({ a: 1 })
    markInitialized()
    configRef.value.a = 2
    // 防抖未到，不调用（watch flush:'pre' 走 microtask，需用 Async 推进器刷新）
    await vi.advanceTimersByTimeAsync(499)
    expect(saveConfig).not.toHaveBeenCalled()
    // 防抖到 500ms，调用
    await vi.advanceTimersByTimeAsync(1)
    expect(saveConfig).toHaveBeenCalledTimes(1)
  })

  it('未运行测试时，saveConfig 后重启实时监控（stop + start）', async () => {
    const { configRef, stopRealtimeMonitor, startRealtimeMonitor, markInitialized } = setup({ a: 1 }, { isRunning: false })
    markInitialized()
    configRef.value.a = 2
    await vi.advanceTimersByTimeAsync(500)
    expect(stopRealtimeMonitor).toHaveBeenCalledTimes(1)
    expect(startRealtimeMonitor).toHaveBeenCalledTimes(1)
  })

  it('isRunning=true 时跳过实时监控重启（仅 saveConfig）', async () => {
    const { configRef, saveConfig, stopRealtimeMonitor, startRealtimeMonitor, markInitialized } = setup({ a: 1 }, { isRunning: true })
    markInitialized()
    configRef.value.a = 2
    await vi.advanceTimersByTimeAsync(500)
    // saveConfig 仍应调用
    expect(saveConfig).toHaveBeenCalledTimes(1)
    // 但 stop/start 不调用
    expect(stopRealtimeMonitor).not.toHaveBeenCalled()
    expect(startRealtimeMonitor).not.toHaveBeenCalled()
  })

  it('连续变更只触发一次 saveConfig（防抖）', async () => {
    const { configRef, saveConfig, markInitialized } = setup({ a: 1 })
    markInitialized()
    configRef.value.a = 2
    await vi.advanceTimersByTimeAsync(300)
    configRef.value.a = 3
    await vi.advanceTimersByTimeAsync(300)
    configRef.value.a = 4
    await vi.advanceTimersByTimeAsync(500)
    expect(saveConfig).toHaveBeenCalledTimes(1)
  })

  it('自定义 debounceMs 生效', async () => {
    const { configRef, saveConfig, markInitialized } = setup({ a: 1 }, { debounceMs: 1000 })
    markInitialized()
    configRef.value.a = 2
    await vi.advanceTimersByTimeAsync(999)
    expect(saveConfig).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(1)
    expect(saveConfig).toHaveBeenCalledTimes(1)
  })

  it('unmount 后清理防抖定时器（onUnmounted 触发 clearTimeout）', async () => {
    // onUnmounted 仅在组件上下文触发，effectScope 中不会触发
    // 因此用真实组件 mount/unmount 验证清理逻辑
    const saveConfig = vi.fn()
    const stopRealtimeMonitor = vi.fn()
    const startRealtimeMonitor = vi.fn()
    const configRef = ref({ a: 1 })
    const isRunningRef = ref(false)

    const TestComp = defineComponent({
      setup() {
        const result = useAutoSaveConfig({
          config: configRef as any,
          isRunning: isRunningRef,
          saveConfig,
          stopRealtimeMonitor,
          startRealtimeMonitor,
        })
        return { ...result }
      },
    })
    const wrapper = mount(TestComp)
    const vm = wrapper.vm as any
    vm.markInitialized()
    configRef.value.a = 2
    await vi.advanceTimersByTimeAsync(300)
    // 卸载前未触发 saveConfig
    expect(saveConfig).not.toHaveBeenCalled()
    // 卸载 —— 触发 onUnmounted → clearTimeout
    wrapper.unmount()
    // 推进时间，saveConfig 不应被调用（定时器已清理）
    await vi.advanceTimersByTimeAsync(1000)
    expect(saveConfig).not.toHaveBeenCalled()
  })
})
