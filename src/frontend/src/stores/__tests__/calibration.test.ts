import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

// ---- mock Wails 绑定 + runtime ----
// 显式声明 Events.On/Off 签名，使 mock.calls 元组类型可索引
type EventsOnFn = (channel: string, handler: (e: any) => void) => () => void
type EventsOffFn = (channel: string) => void

const { mockCalibrationService, mockEventsOn, mockEventsOff } = vi.hoisted(() => ({
  mockCalibrationService: {
    StartCalibration: vi.fn(),
    PauseCalibration: vi.fn(),
    ResumeCalibration: vi.fn(),
    StopCalibration: vi.fn(),
    GetCalibrationStatus: vi.fn(),
  },
  mockEventsOn: vi.fn<EventsOnFn>(() => () => {}),
  mockEventsOff: vi.fn<EventsOffFn>(),
}))

vi.mock('@bindings/yx-daq/internal/app', () => ({
  CalibrationService: mockCalibrationService,
}))

vi.mock('@wailsio/runtime', () => ({
  Events: { On: mockEventsOn, Off: mockEventsOff },
}))

import { useCalibrationStore } from '../calibration'

describe('stores/calibration', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockCalibrationService.StartCalibration.mockReset()
    mockCalibrationService.PauseCalibration.mockReset()
    mockCalibrationService.ResumeCalibration.mockReset()
    mockCalibrationService.StopCalibration.mockReset()
    mockCalibrationService.GetCalibrationStatus.mockReset()
    mockEventsOn.mockClear()
    mockEventsOff.mockClear()
  })

  describe('isRunning computed', () => {
    it('taskStatus=null → false', () => {
      const store = useCalibrationStore()
      expect(store.isRunning).toBe(false)
    })

    it('status=running → true', () => {
      const store = useCalibrationStore()
      store.taskStatus = { status: 'running' } as any
      expect(store.isRunning).toBe(true)
    })

    it('status=paused → true', () => {
      const store = useCalibrationStore()
      store.taskStatus = { status: 'paused' } as any
      expect(store.isRunning).toBe(true)
    })

    it('status=completed → false', () => {
      const store = useCalibrationStore()
      store.taskStatus = { status: 'completed' } as any
      expect(store.isRunning).toBe(false)
    })

    it('status=stopped → false', () => {
      const store = useCalibrationStore()
      store.taskStatus = { status: 'stopped' } as any
      expect(store.isRunning).toBe(false)
    })
  })

  describe('action 委托', () => {
    it('startCalibration → 调用 StartCalibration + fetchStatus', async () => {
      mockCalibrationService.StartCalibration.mockResolvedValue(undefined)
      mockCalibrationService.GetCalibrationStatus.mockResolvedValue({ status: 'running' })
      const store = useCalibrationStore()
      await store.startCalibration({ foo: 'bar' })
      expect(mockCalibrationService.StartCalibration).toHaveBeenCalledWith({ foo: 'bar' })
      expect(mockCalibrationService.GetCalibrationStatus).toHaveBeenCalledTimes(1)
      expect(store.taskStatus?.status).toBe('running')
    })

    it('pauseCalibration → 调用 PauseCalibration', async () => {
      mockCalibrationService.PauseCalibration.mockResolvedValue(undefined)
      const store = useCalibrationStore()
      await store.pauseCalibration()
      expect(mockCalibrationService.PauseCalibration).toHaveBeenCalledTimes(1)
    })

    it('resumeCalibration → 调用 ResumeCalibration', async () => {
      mockCalibrationService.ResumeCalibration.mockResolvedValue(undefined)
      const store = useCalibrationStore()
      await store.resumeCalibration()
      expect(mockCalibrationService.ResumeCalibration).toHaveBeenCalledTimes(1)
    })

    it('stopCalibration → 调用 StopCalibration + fetchStatus', async () => {
      mockCalibrationService.StopCalibration.mockResolvedValue(undefined)
      mockCalibrationService.GetCalibrationStatus.mockResolvedValue({ status: 'stopped' })
      const store = useCalibrationStore()
      await store.stopCalibration()
      expect(mockCalibrationService.StopCalibration).toHaveBeenCalledTimes(1)
      expect(store.taskStatus?.status).toBe('stopped')
    })

    it('fetchStatus → 更新 taskStatus', async () => {
      mockCalibrationService.GetCalibrationStatus.mockResolvedValue({
        taskId: 't1', status: 'running', totalPoints: 10, completedPoints: 3,
      })
      const store = useCalibrationStore()
      await store.fetchStatus()
      expect(store.taskStatus).toMatchObject({ taskId: 't1', status: 'running', completedPoints: 3 })
    })

    it('StartCalibration 抛错 → 不抛出，taskStatus 保持 null', async () => {
      mockCalibrationService.StartCalibration.mockRejectedValue(new Error('boom'))
      // 抑制 console.error 噪音
      const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
      const store = useCalibrationStore()
      await store.startCalibration({})
      expect(store.taskStatus).toBeNull()
      spy.mockRestore()
    })

    it('GetCalibrationStatus 抛错 → fetchStatus 不抛出', async () => {
      mockCalibrationService.GetCalibrationStatus.mockRejectedValue(new Error('rpc err'))
      const spy = vi.spyOn(console, 'warn').mockImplementation(() => {})
      const store = useCalibrationStore()
      await store.fetchStatus()
      expect(store.taskStatus).toBeNull()
      spy.mockRestore()
    })
  })

  describe('事件监听', () => {
    it('startListening 注册 3 个 calibration:* 事件通道', () => {
      const store = useCalibrationStore()
      store.startListening()
      const channels = mockEventsOn.mock.calls.map(c => c[0])
      expect(channels).toContain('calibration:progress')
      expect(channels).toContain('calibration:realtime')
      expect(channels).toContain('calibration:complete')
      expect(mockEventsOn).toHaveBeenCalledTimes(3)
    })

    it('startListening 幂等：重复调用不重复注册', () => {
      const store = useCalibrationStore()
      store.startListening()
      store.startListening()
      expect(mockEventsOn).toHaveBeenCalledTimes(3)
    })

    it('progress 事件 → 更新 progress ref', () => {
      const store = useCalibrationStore()
      store.startListening()
      const progressHandler = mockEventsOn.mock.calls.find(c => c[0] === 'calibration:progress')![1] as (e: any) => void
      const payload = { taskId: 't1', progress: 50, completedPoints: 5 }
      progressHandler({ data: payload })
      expect(store.progress).toMatchObject({ taskId: 't1', progress: 50 })
    })

    it('realtime 事件 → 更新 realtime ref', () => {
      const store = useCalibrationStore()
      store.startListening()
      const realtimeHandler = mockEventsOn.mock.calls.find(c => c[0] === 'calibration:realtime')![1] as (e: any) => void
      const payload = { taskId: 't1', pointId: 'p1', rawData: { p1: 1, p2: 2, p3: 3, p4: 4, p5: 5, pAtm: 100, tAtm: 25 } }
      realtimeHandler({ data: payload })
      expect(store.realtime).toMatchObject({ pointId: 'p1' })
    })

    it('complete 事件 → 触发 fetchStatus', async () => {
      mockCalibrationService.GetCalibrationStatus.mockResolvedValue({ status: 'completed' })
      const store = useCalibrationStore()
      store.startListening()
      const completeHandler = mockEventsOn.mock.calls.find(c => c[0] === 'calibration:complete')![1] as (e: any) => void
      completeHandler({})
      // fetchStatus 是 async，等微任务刷新
      await new Promise(r => setTimeout(r, 0))
      expect(mockCalibrationService.GetCalibrationStatus).toHaveBeenCalled()
      expect(store.taskStatus?.status).toBe('completed')
    })

    it('stopListening → 调用 EventsOff 注销', () => {
      const store = useCalibrationStore()
      store.startListening()
      store.stopListening()
      expect(mockEventsOff).toHaveBeenCalledTimes(3)
    })

    it('stopListening 幂等：未 start 时调用不抛错', () => {
      const store = useCalibrationStore()
      store.stopListening()
      expect(mockEventsOff).not.toHaveBeenCalled()
    })
  })
})
