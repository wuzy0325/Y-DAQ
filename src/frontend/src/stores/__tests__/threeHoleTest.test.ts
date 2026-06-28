import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

// ---- mock 边界 ----
// threeHoleTest store 依赖：
//   - @bindings/.../app 的 ThreeHoleService + ConfigService
//   - @wailsio/runtime 的 Events.On/Off（直接使用，不走 createWailsEventListener）
//   - ../motion 的 useMotionStore（用于检查运动控制器连接状态）
//   - ../device 的 ensureDevicesAcquiring（用于自动恢复采集）
//   - ../utils/csv 的 downloadCSV
//   - ../api/enums 的 ThreeHoleChannelRole / TraversalPattern（纯常量，直接 import 真实模块）
type EventsOnFn = (channel: string, handler: (e: any) => void) => () => void
type EventsOffFn = (channel: string) => void

const { mockThreeHoleService, mockConfigService, mockEventsOn, mockEventsOff, mockMotionStore, mockEnsureDevicesAcquiring, mockDownloadCSV } = vi.hoisted(() => ({
  mockThreeHoleService: {
    SelectThreeHoleCalibFiles: vi.fn(),
    LoadThreeHoleCalibFiles: vi.fn(),
    GetThreeHoleCalibInfo: vi.fn(),
    StartThreeHoleTraversal: vi.fn(),
    PauseThreeHoleTraversal: vi.fn(),
    ResumeThreeHoleTraversal: vi.fn(),
    StopThreeHoleTraversal: vi.fn(),
    GetThreeHoleTraversalStatus: vi.fn(),
    StartThreeHoleRealtimeMonitor: vi.fn(),
    StopThreeHoleRealtimeMonitor: vi.fn(),
  },
  mockConfigService: {
    SaveThreeHoleProbe1Config: vi.fn(),
    SaveThreeHoleProbe2Config: vi.fn(),
    LoadThreeHoleProbe1Config: vi.fn(),
    LoadThreeHoleProbe2Config: vi.fn(),
  },
  mockEventsOn: vi.fn<EventsOnFn>(() => () => {}),
  mockEventsOff: vi.fn<EventsOffFn>(),
  // motion store 的 mock：仅暴露 startTest 守卫中用到的 statuses
  mockMotionStore: {
    statuses: [] as any[],
  },
  mockEnsureDevicesAcquiring: vi.fn<(ids: Iterable<string>) => Promise<string[]>>(() => Promise.resolve([])),
  mockDownloadCSV: vi.fn<(prefix: string, headers: string[], rows: (string | number)[][]) => void>(),
}))

vi.mock('@bindings/yx-daq/internal/app', () => ({
  ThreeHoleService: mockThreeHoleService,
  ConfigService: mockConfigService,
}))

vi.mock('@wailsio/runtime', () => ({
  Events: { On: mockEventsOn, Off: mockEventsOff },
}))

vi.mock('../motion', () => ({
  useMotionStore: () => mockMotionStore,
}))

vi.mock('../device', () => ({
  ensureDevicesAcquiring: mockEnsureDevicesAcquiring,
}))

vi.mock('../../utils/csv', () => ({
  downloadCSV: mockDownloadCSV,
}))

import { useThreeHoleTestStore } from '../threeHoleTest'
import { ThreeHoleChannelRole, TraversalPattern } from '../../api/enums'

describe('stores/threeHoleTest', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    Object.values(mockThreeHoleService).forEach(fn => fn.mockReset())
    Object.values(mockConfigService).forEach(fn => fn.mockReset())
    mockEventsOn.mockClear()
    mockEventsOff.mockClear()
    mockMotionStore.statuses = []
    mockEnsureDevicesAcquiring.mockReset()
    mockEnsureDevicesAcquiring.mockResolvedValue([])
    mockDownloadCSV.mockReset()
  })

  // ============ 初始状态 / computed ============
  describe('初始状态 / computed', () => {
    it('默认 probeID=probe1', () => {
      const store = useThreeHoleTestStore()
      expect(store.probeID).toBe('probe1')
    })

    it('init(id) 设置 probeID', () => {
      const store = useThreeHoleTestStore()
      store.init('probe2')
      expect(store.probeID).toBe('probe2')
    })

    it('taskStatus=null → isRunning=false, isPaused=false, statusText=未启动, hasResults=false', () => {
      const store = useThreeHoleTestStore()
      expect(store.isRunning).toBe(false)
      expect(store.isPaused).toBe(false)
      expect(store.statusText).toBe('未启动')
      expect(store.hasResults).toBe(false)
    })

    it('status=running → isRunning=true, statusText=运行中', () => {
      const store = useThreeHoleTestStore()
      store.taskStatus = { status: 'running', dataPoints: [] } as any
      expect(store.isRunning).toBe(true)
      expect(store.isPaused).toBe(false)
      expect(store.statusText).toBe('运行中')
    })

    it('status=paused → isRunning=true, isPaused=true, statusText=已暂停', () => {
      const store = useThreeHoleTestStore()
      store.taskStatus = { status: 'paused', dataPoints: [] } as any
      expect(store.isRunning).toBe(true)
      expect(store.isPaused).toBe(true)
      expect(store.statusText).toBe('已暂停')
    })

    it('status=completed → isRunning=false, statusText=已完成', () => {
      const store = useThreeHoleTestStore()
      store.taskStatus = { status: 'completed', dataPoints: [] } as any
      expect(store.isRunning).toBe(false)
      expect(store.statusText).toBe('已完成')
    })

    it('status=error → statusText=错误', () => {
      const store = useThreeHoleTestStore()
      store.taskStatus = { status: 'error', dataPoints: [] } as any
      expect(store.statusText).toBe('错误')
    })

    it('status=未知值 → 原样返回', () => {
      const store = useThreeHoleTestStore()
      store.taskStatus = { status: 'weird', dataPoints: [] } as any
      expect(store.statusText).toBe('weird')
    })

    it('hasResults=true 当 dataPoints 非空', () => {
      const store = useThreeHoleTestStore()
      store.taskStatus = { status: 'completed', dataPoints: [{ pointId: 'p1' }] } as any
      expect(store.hasResults).toBe(true)
    })

    it('默认 config 包含 5 个 probeChannels（P1/P2/P3/P_ATM/T_ATM）', () => {
      const store = useThreeHoleTestStore()
      expect(store.config.probeChannels).toHaveLength(5)
      expect(store.config.probeChannels.map(c => c.role)).toEqual([
        ThreeHoleChannelRole.P1,
        ThreeHoleChannelRole.P2,
        ThreeHoleChannelRole.P3,
        ThreeHoleChannelRole.P_ATM,
        ThreeHoleChannelRole.T_ATM,
      ])
    })

    it('默认 layout.pattern=RECTANGLE', () => {
      const store = useThreeHoleTestStore()
      expect(store.config.layout.pattern).toBe(TraversalPattern.RECTANGLE)
    })

    it('默认 motionAlpha.axis=X, motionBeta.axis=Y', () => {
      const store = useThreeHoleTestStore()
      expect(store.config.motionAlpha.axis).toBe('X')
      expect(store.config.motionBeta.axis).toBe('Y')
    })
  })

  // ============ clearError ============
  describe('clearError', () => {
    it('清空 lastError', () => {
      const store = useThreeHoleTestStore()
      store.lastError = 'some error'
      store.clearError()
      expect(store.lastError).toBe('')
    })
  })

  // ============ fetchStatus ============
  describe('fetchStatus', () => {
    it('成功 → 写入 taskStatus', async () => {
      mockThreeHoleService.GetThreeHoleTraversalStatus.mockResolvedValue({ status: 'running' })
      const store = useThreeHoleTestStore()
      await store.fetchStatus()
      expect(store.taskStatus).toEqual({ status: 'running' })
    })

    it('失败 → 仅 console.warn，不抛出', async () => {
      mockThreeHoleService.GetThreeHoleTraversalStatus.mockRejectedValue(new Error('boom'))
      const store = useThreeHoleTestStore()
      await expect(store.fetchStatus()).resolves.toBeUndefined()
    })
  })

  // ============ ensureDeviceAcquiring ============
  describe('ensureDeviceAcquiring', () => {
    it('deviceId 为空 → 直接返回，不调用 ensureDevicesAcquiring', async () => {
      const store = useThreeHoleTestStore()
      store.config.deviceId = ''
      await store.ensureDeviceAcquiring()
      expect(mockEnsureDevicesAcquiring).not.toHaveBeenCalled()
    })

    it('deviceId 非空 → 调用 ensureDevicesAcquiring([deviceId])', async () => {
      mockEnsureDevicesAcquiring.mockResolvedValue([])
      const store = useThreeHoleTestStore()
      store.config.deviceId = 'dev-1'
      await store.ensureDeviceAcquiring()
      expect(mockEnsureDevicesAcquiring).toHaveBeenCalledWith(['dev-1'])
      expect(store.lastError).toBe('')
    })

    it('返回错误 → 写入 lastError', async () => {
      mockEnsureDevicesAcquiring.mockResolvedValue(['connect failed'])
      const store = useThreeHoleTestStore()
      store.config.deviceId = 'dev-1'
      await store.ensureDeviceAcquiring()
      expect(store.lastError).toBe('connect failed')
    })
  })

  // ============ selectCalibFiles ============
  describe('selectCalibFiles', () => {
    it('用户取消（返回空数组）→ 不加载、不调用 ensureDeviceAcquiring', async () => {
      mockThreeHoleService.SelectThreeHoleCalibFiles.mockResolvedValue([])
      const store = useThreeHoleTestStore()
      await store.selectCalibFiles()
      expect(mockThreeHoleService.LoadThreeHoleCalibFiles).not.toHaveBeenCalled()
      expect(mockEnsureDevicesAcquiring).not.toHaveBeenCalled()
      expect(store.calibLoaded).toBe(false)
    })

    it('选择文件 → 加载 + 获取 calibInfo + calibLoaded=true + 写入 config.calibFiles + 触发 ensureDeviceAcquiring', async () => {
      mockThreeHoleService.SelectThreeHoleCalibFiles.mockResolvedValue(['C:\\path\\a.prb', 'C:\\path\\b.prb'])
      mockThreeHoleService.LoadThreeHoleCalibFiles.mockResolvedValue(undefined)
      mockThreeHoleService.GetThreeHoleCalibInfo.mockResolvedValue([{ cMa: 1.1 }, { cMa: 2.2 }])
      mockEnsureDevicesAcquiring.mockResolvedValue([])
      const store = useThreeHoleTestStore()
      store.config.deviceId = 'dev-1'
      await store.selectCalibFiles()
      expect(mockThreeHoleService.LoadThreeHoleCalibFiles).toHaveBeenCalledWith('probe1', ['C:\\path\\a.prb', 'C:\\path\\b.prb'])
      expect(store.calibLoaded).toBe(true)
      expect(store.calibFiles).toEqual(['C:\\path\\a.prb', 'C:\\path\\b.prb'])
      expect(store.config.calibFiles).toHaveLength(2)
      expect(store.config.calibFiles[0]).toEqual({
        filePath: 'C:\\path\\a.prb',
        fileName: 'a.prb',
        cMa: 1.1,
      })
      expect(store.config.calibFiles[1].fileName).toBe('b.prb')
      expect(store.config.calibFiles[1].cMa).toBe(2.2)
      expect(mockEnsureDevicesAcquiring).toHaveBeenCalled()
    })

    it('SelectThreeHoleCalibFiles 抛错 → 写入 lastError', async () => {
      mockThreeHoleService.SelectThreeHoleCalibFiles.mockRejectedValue(new Error('cancel'))
      const store = useThreeHoleTestStore()
      await store.selectCalibFiles()
      expect(store.calibLoaded).toBe(false)
      expect(store.lastError).toContain('加载校准文件失败')
    })

    it('LoadThreeHoleCalibFiles 抛错 → 写入 lastError', async () => {
      mockThreeHoleService.SelectThreeHoleCalibFiles.mockResolvedValue(['a.prb'])
      mockThreeHoleService.LoadThreeHoleCalibFiles.mockRejectedValue(new Error('load boom'))
      const store = useThreeHoleTestStore()
      await store.selectCalibFiles()
      expect(store.calibLoaded).toBe(false)
      expect(store.lastError).toContain('加载校准文件失败')
    })
  })

  // ============ startTest 守卫 ============
  describe('startTest 守卫', () => {
    it('isRunning=true → 直接返回，不调用 StartThreeHoleTraversal', async () => {
      const store = useThreeHoleTestStore()
      store.taskStatus = { status: 'running', dataPoints: [] } as any
      await store.startTest()
      expect(mockThreeHoleService.StartThreeHoleTraversal).not.toHaveBeenCalled()
    })

    it('taskStatus=paused → isRunning=true 触发静默早退（line 212），lastError 保持空', async () => {
      const store = useThreeHoleTestStore()
      store.taskStatus = { status: 'paused', dataPoints: [] } as any
      await store.startTest()
      // isRunning 对 paused 为 true，line 212 早退，line 215-217 不可达
      expect(store.lastError).toBe('')
      expect(mockThreeHoleService.StartThreeHoleTraversal).not.toHaveBeenCalled()
    })

    it('calibLoaded=false → 拒绝（请先加载校准文件）', async () => {
      const store = useThreeHoleTestStore()
      store.calibLoaded = false
      await store.startTest()
      expect(store.lastError).toBe('请先加载校准文件')
      expect(mockThreeHoleService.StartThreeHoleTraversal).not.toHaveBeenCalled()
    })

    it('motionControllerId 非空但控制器未连接 → 拒绝', async () => {
      const store = useThreeHoleTestStore()
      store.calibLoaded = true
      store.config.motionControllerId = 'mc-1'
      mockMotionStore.statuses = [] // 不含 mc-1
      await store.startTest()
      expect(store.lastError).toBe('运动控制器未连接，请先在设备页面连接运动控制器')
      expect(mockThreeHoleService.StartThreeHoleTraversal).not.toHaveBeenCalled()
    })

    it('motionControllerId 非空且控制器在 statuses 中但状态非 Connected → 拒绝', async () => {
      const store = useThreeHoleTestStore()
      store.calibLoaded = true
      store.config.motionControllerId = 'mc-1'
      mockMotionStore.statuses = [{ id: 'mc-1', status: 'Disconnected' }]
      await store.startTest()
      expect(store.lastError).toBe('运动控制器未连接，请先在设备页面连接运动控制器')
    })

    it('motionControllerId 非空且控制器 Connected → 通过守卫', async () => {
      mockThreeHoleService.StartThreeHoleTraversal.mockResolvedValue(undefined)
      mockThreeHoleService.GetThreeHoleTraversalStatus.mockResolvedValue({ status: 'running' })
      const store = useThreeHoleTestStore()
      store.calibLoaded = true
      store.config.motionControllerId = 'mc-1'
      mockMotionStore.statuses = [{ id: 'mc-1', status: 'Connected' }]
      await store.startTest()
      expect(mockThreeHoleService.StartThreeHoleTraversal).toHaveBeenCalledTimes(1)
      expect(store.taskStatus).toEqual({ status: 'running' })
    })

    it('守卫全通过 → 启动前清空 taskStatus/realtime/progress', async () => {
      mockThreeHoleService.StartThreeHoleTraversal.mockResolvedValue(undefined)
      mockThreeHoleService.GetThreeHoleTraversalStatus.mockResolvedValue({ status: 'running' })
      const store = useThreeHoleTestStore()
      store.calibLoaded = true
      store.taskStatus = { status: 'completed', dataPoints: [{ pointId: 'old' }] } as any
      store.progress = { progress: 50 } as any
      store.realtime = { pointId: 'old' } as any
      await store.startTest()
      // 启动后 taskStatus 来自 fetchStatus，progress/realtime 在启动前已清空
      expect(store.taskStatus).toEqual({ status: 'running' })
    })

    it('StartThreeHoleTraversal 抛错 → lastError 写入 + taskStatus=null', async () => {
      mockThreeHoleService.StartThreeHoleTraversal.mockRejectedValue(new Error('start boom'))
      const store = useThreeHoleTestStore()
      store.calibLoaded = true
      await store.startTest()
      expect(store.lastError).toContain('启动测试失败')
      expect(store.taskStatus).toBeNull()
      expect(store.realtime).toBeNull()
      expect(store.progress).toBeNull()
    })

    it('motionControllerId 为空 → 跳过运动控制器检查', async () => {
      mockThreeHoleService.StartThreeHoleTraversal.mockResolvedValue(undefined)
      mockThreeHoleService.GetThreeHoleTraversalStatus.mockResolvedValue({ status: 'running' })
      const store = useThreeHoleTestStore()
      store.calibLoaded = true
      store.config.motionControllerId = ''
      await store.startTest()
      expect(mockThreeHoleService.StartThreeHoleTraversal).toHaveBeenCalledTimes(1)
    })
  })

  // ============ pause / resume ============
  describe('pauseTest / resumeTest', () => {
    it('pauseTest → 调用 PauseThreeHoleTraversal + fetchStatus', async () => {
      mockThreeHoleService.PauseThreeHoleTraversal.mockResolvedValue(undefined)
      mockThreeHoleService.GetThreeHoleTraversalStatus.mockResolvedValue({ status: 'paused' })
      const store = useThreeHoleTestStore()
      await store.pauseTest()
      expect(mockThreeHoleService.PauseThreeHoleTraversal).toHaveBeenCalledWith('probe1')
      expect(store.taskStatus).toEqual({ status: 'paused' })
    })

    it('pauseTest 抛错 → 不抛出（仅 console.error）', async () => {
      mockThreeHoleService.PauseThreeHoleTraversal.mockRejectedValue(new Error('pause boom'))
      const store = useThreeHoleTestStore()
      await expect(store.pauseTest()).resolves.toBeUndefined()
    })

    it('resumeTest → 调用 ResumeThreeHoleTraversal + fetchStatus', async () => {
      mockThreeHoleService.ResumeThreeHoleTraversal.mockResolvedValue(undefined)
      mockThreeHoleService.GetThreeHoleTraversalStatus.mockResolvedValue({ status: 'running' })
      const store = useThreeHoleTestStore()
      await store.resumeTest()
      expect(mockThreeHoleService.ResumeThreeHoleTraversal).toHaveBeenCalledWith('probe1')
      expect(store.taskStatus).toEqual({ status: 'running' })
    })
  })

  // ============ stopTest（含 fake timers 轮询）============
  describe('stopTest', () => {
    beforeEach(() => vi.useFakeTimers())
    afterEach(() => vi.useRealTimers())

    it('首次 fetchStatus 返回 idle → 立即退出循环 + 清空 realtime/progress', async () => {
      mockThreeHoleService.StopThreeHoleTraversal.mockResolvedValue(undefined)
      mockThreeHoleService.GetThreeHoleTraversalStatus.mockResolvedValue({ status: 'idle' })
      const store = useThreeHoleTestStore()
      store.realtime = { pointId: 'x' } as any
      store.progress = { progress: 50 } as any
      await store.stopTest()
      expect(mockThreeHoleService.StopThreeHoleTraversal).toHaveBeenCalledWith('probe1')
      expect(mockThreeHoleService.GetThreeHoleTraversalStatus).toHaveBeenCalledTimes(1)
      expect(store.realtime).toBeNull()
      expect(store.progress).toBeNull()
    })

    it('前 2 次 running、第 3 次 idle → 轮询 3 次后退出', async () => {
      mockThreeHoleService.StopThreeHoleTraversal.mockResolvedValue(undefined)
      mockThreeHoleService.GetThreeHoleTraversalStatus
        .mockResolvedValueOnce({ status: 'running' })
        .mockResolvedValueOnce({ status: 'running' })
        .mockResolvedValueOnce({ status: 'idle' })
      const store = useThreeHoleTestStore()
      const promise = store.stopTest()
      // 推进 2 个 200ms 等待循环
      await vi.advanceTimersByTimeAsync(200)
      await vi.advanceTimersByTimeAsync(200)
      await promise
      expect(mockThreeHoleService.GetThreeHoleTraversalStatus).toHaveBeenCalledTimes(3)
    })

    it('10 次仍 running → 超时强制清理 realtime/progress', async () => {
      mockThreeHoleService.StopThreeHoleTraversal.mockResolvedValue(undefined)
      mockThreeHoleService.GetThreeHoleTraversalStatus.mockResolvedValue({ status: 'running' })
      const store = useThreeHoleTestStore()
      store.realtime = { pointId: 'x' } as any
      store.progress = { progress: 50 } as any
      const promise = store.stopTest()
      // 推进 10 个 200ms（共 2 秒）
      for (let i = 0; i < 10; i++) {
        await vi.advanceTimersByTimeAsync(200)
      }
      await promise
      expect(mockThreeHoleService.GetThreeHoleTraversalStatus).toHaveBeenCalledTimes(10)
      expect(store.realtime).toBeNull()
      expect(store.progress).toBeNull()
    })

    it('StopThreeHoleTraversal 抛错 → 清空 realtime/progress，不抛出', async () => {
      mockThreeHoleService.StopThreeHoleTraversal.mockRejectedValue(new Error('stop boom'))
      const store = useThreeHoleTestStore()
      store.realtime = { pointId: 'x' } as any
      await expect(store.stopTest()).resolves.toBeUndefined()
      expect(store.realtime).toBeNull()
      expect(store.progress).toBeNull()
    })
  })

  // ============ 实时数据监控 ============
  describe('realtime monitor', () => {
    it('startRealtimeMonitor → 调用 StartThreeHoleRealtimeMonitor', async () => {
      mockThreeHoleService.StartThreeHoleRealtimeMonitor.mockResolvedValue(undefined)
      const store = useThreeHoleTestStore()
      await store.startRealtimeMonitor()
      expect(mockThreeHoleService.StartThreeHoleRealtimeMonitor).toHaveBeenCalledWith('probe1', store.config)
    })

    it('startRealtimeMonitor 抛错 → 不抛出', async () => {
      mockThreeHoleService.StartThreeHoleRealtimeMonitor.mockRejectedValue(new Error('boom'))
      const store = useThreeHoleTestStore()
      await expect(store.startRealtimeMonitor()).resolves.toBeUndefined()
    })

    it('stopRealtimeMonitor → 调用 StopThreeHoleRealtimeMonitor', async () => {
      mockThreeHoleService.StopThreeHoleRealtimeMonitor.mockResolvedValue(undefined)
      const store = useThreeHoleTestStore()
      await store.stopRealtimeMonitor()
      expect(mockThreeHoleService.StopThreeHoleRealtimeMonitor).toHaveBeenCalledWith('probe1')
    })
  })

  // ============ 事件监听（probeID-scoped）============
  describe('startListening / stopListening', () => {
    it('startListening → Events.On 注册 4 个 three-hole:probe1:* 通道', () => {
      const store = useThreeHoleTestStore()
      store.startListening()
      const channels = mockEventsOn.mock.calls.map(c => c[0])
      expect(channels).toContain('three-hole:probe1:progress')
      expect(channels).toContain('three-hole:probe1:realtime')
      expect(channels).toContain('three-hole:probe1:complete')
      expect(channels).toContain('three-hole:probe1:error')
      expect(mockEventsOn).toHaveBeenCalledTimes(4)
    })

    it('init(probe2) 后 startListening → 注册 three-hole:probe2:* 通道', () => {
      const store = useThreeHoleTestStore()
      store.init('probe2')
      store.startListening()
      const channels = mockEventsOn.mock.calls.map(c => c[0])
      expect(channels).toContain('three-hole:probe2:progress')
    })

    it('startListening 重复同 probeID → 幂等（不重复注册）', () => {
      const store = useThreeHoleTestStore()
      store.startListening()
      store.startListening()
      expect(mockEventsOn).toHaveBeenCalledTimes(4)
    })

    it('startListening 切换 probeID → 注销旧 + 注册新', () => {
      const store = useThreeHoleTestStore()
      store.startListening() // probe1
      store.init('probe2')
      store.startListening()
      // 注销 probe1 的 4 个通道
      expect(mockEventsOff).toHaveBeenCalledWith('three-hole:probe1:progress')
      expect(mockEventsOff).toHaveBeenCalledWith('three-hole:probe1:realtime')
      expect(mockEventsOff).toHaveBeenCalledWith('three-hole:probe1:complete')
      expect(mockEventsOff).toHaveBeenCalledWith('three-hole:probe1:error')
      // 注册 probe2 的 4 个通道
      expect(mockEventsOn).toHaveBeenCalledWith('three-hole:probe2:progress', expect.any(Function))
    })

    it('stopListening → Events.Off 注销 4 个通道', () => {
      const store = useThreeHoleTestStore()
      store.startListening()
      store.stopListening()
      expect(mockEventsOff).toHaveBeenCalledWith('three-hole:probe1:progress')
      expect(mockEventsOff).toHaveBeenCalledWith('three-hole:probe1:realtime')
      expect(mockEventsOff).toHaveBeenCalledWith('three-hole:probe1:complete')
      expect(mockEventsOff).toHaveBeenCalledWith('three-hole:probe1:error')
    })

    it('stopListening 未先 start → 无操作', () => {
      const store = useThreeHoleTestStore()
      store.stopListening()
      expect(mockEventsOff).not.toHaveBeenCalled()
    })

    it('progress 回调 → 写入 progress', () => {
      const store = useThreeHoleTestStore()
      store.startListening()
      const cb = mockEventsOn.mock.calls.find(c => c[0] === 'three-hole:probe1:progress')![1] as (e: any) => void
      cb({ data: { progress: 75, currentX: 1, currentY: 2 } })
      expect(store.progress).toEqual({ progress: 75, currentX: 1, currentY: 2 })
    })

    it('realtime 回调 → 写入 realtime', () => {
      const store = useThreeHoleTestStore()
      store.startListening()
      const cb = mockEventsOn.mock.calls.find(c => c[0] === 'three-hole:probe1:realtime')![1] as (e: any) => void
      cb({ data: { pointId: 'p1', rawData: { p1: 1 } } })
      expect(store.realtime).toEqual({ pointId: 'p1', rawData: { p1: 1 } })
    })

    it('complete 回调 → 清空 progress + fetchStatus', async () => {
      mockThreeHoleService.GetThreeHoleTraversalStatus.mockResolvedValue({ status: 'completed' })
      const store = useThreeHoleTestStore()
      store.progress = { progress: 99 } as any
      store.startListening()
      const cb = mockEventsOn.mock.calls.find(c => c[0] === 'three-hole:probe1:complete')![1] as (e: any) => void | Promise<void>
      await cb({ data: {} })
      expect(store.progress).toBeNull()
      expect(store.taskStatus).toEqual({ status: 'completed' })
    })

    it('error 回调 非致命 → 写入 lastError + 保留 progress', () => {
      const store = useThreeHoleTestStore()
      store.progress = { progress: 50 } as any
      store.startListening()
      const cb = mockEventsOn.mock.calls.find(c => c[0] === 'three-hole:probe1:error')![1] as (e: any) => void
      cb({ data: { error: 'stuck', isFatal: false } })
      expect(store.lastError).toBe('stuck')
      expect(store.progress).not.toBeNull() // 非致命不清理
    })

    it('error 回调 致命 → 写入 lastError + 清空 progress', () => {
      const store = useThreeHoleTestStore()
      store.progress = { progress: 50 } as any
      store.startListening()
      const cb = mockEventsOn.mock.calls.find(c => c[0] === 'three-hole:probe1:error')![1] as (e: any) => void
      cb({ data: { error: 'fatal boom', isFatal: true } })
      expect(store.lastError).toBe('fatal boom')
      expect(store.progress).toBeNull()
    })
  })

  // ============ exportCSV ============
  describe('exportCSV', () => {
    it('无 dataPoints → 直接返回，不调用 downloadCSV', () => {
      const store = useThreeHoleTestStore()
      store.taskStatus = { status: 'completed', dataPoints: [] } as any
      store.exportCSV()
      expect(mockDownloadCSV).not.toHaveBeenCalled()
    })

    it('taskStatus=null → 直接返回', () => {
      const store = useThreeHoleTestStore()
      store.exportCSV()
      expect(mockDownloadCSV).not.toHaveBeenCalled()
    })

    it('有 dataPoints → 调用 downloadCSV（filenamePrefix=three-hole-traversal-probe1）', () => {
      const store = useThreeHoleTestStore()
      store.taskStatus = {
        status: 'completed',
        dataPoints: [{
          pointId: 'p1', x: 1.5, y: 2.5,
          rawData: { p1: 100, p2: 200, p3: 300, pAtm: 101, tAtm: 25 },
          interpResult: {
            ptProbe: 1, psProbe: 2, machProbe: 0.5, alphaProbe: 5, velocityProbe: 100,
            iterationCount: 3, converged: true, valid: true,
          },
          sampleCount: 10, timestamp: 1234567890,
        }],
      } as any
      store.exportCSV()
      expect(mockDownloadCSV).toHaveBeenCalledTimes(1)
      const [prefix, headers, rows] = mockDownloadCSV.mock.calls[0]
      expect(prefix).toBe('three-hole-traversal-probe1')
      expect(headers).toHaveLength(16)
      expect(headers[0]).toBe('点号')
      expect(rows).toHaveLength(1)
      expect(rows[0][0]).toBe('p1')
    })

    it('probeID=probe2 → filenamePrefix 含 probe2', () => {
      const store = useThreeHoleTestStore()
      store.init('probe2')
      store.taskStatus = {
        status: 'completed',
        dataPoints: [{
          pointId: 'p1', x: 0, y: 0,
          rawData: { p1: 0, p2: 0, p3: 0, pAtm: 0, tAtm: 0 },
          interpResult: {
            ptProbe: 0, psProbe: 0, machProbe: 0, alphaProbe: 0, velocityProbe: 0,
            iterationCount: 0, converged: true, valid: true,
          },
          sampleCount: 0, timestamp: 0,
        }],
      } as any
      store.exportCSV()
      expect(mockDownloadCSV.mock.calls[0][0]).toBe('three-hole-traversal-probe2')
    })
  })

  // ============ saveConfig / loadConfig ============
  describe('saveConfig', () => {
    it('probe1 → 调用 SaveThreeHoleProbe1Config + 写 localStorage', () => {
      mockConfigService.SaveThreeHoleProbe1Config.mockResolvedValue(undefined)
      const store = useThreeHoleTestStore()
      store.config.dwellTimeMs = 5000
      store.saveConfig()
      expect(localStorage.getItem('threeHoleTestConfig_probe1')).toBeTruthy()
      expect(mockConfigService.SaveThreeHoleProbe1Config).toHaveBeenCalledTimes(1)
      const savedArg = mockConfigService.SaveThreeHoleProbe1Config.mock.calls[0][0]
      expect(savedArg.dwellTimeMs).toBe(5000)
    })

    it('probe2 → 调用 SaveThreeHoleProbe2Config', () => {
      mockConfigService.SaveThreeHoleProbe2Config.mockResolvedValue(undefined)
      const store = useThreeHoleTestStore()
      store.init('probe2')
      store.saveConfig()
      expect(mockConfigService.SaveThreeHoleProbe2Config).toHaveBeenCalledTimes(1)
      expect(localStorage.getItem('threeHoleTestConfig_probe2')).toBeTruthy()
    })

    it('SaveThreeHoleProbe1Config 抛错 → 仅 console.error，不抛出', () => {
      mockConfigService.SaveThreeHoleProbe1Config.mockRejectedValue(new Error('save boom'))
      const store = useThreeHoleTestStore()
      expect(() => store.saveConfig()).not.toThrow()
    })

    it('localStorage.setItem 抛错 → 不抛出', () => {
      const orig = localStorage.setItem
      localStorage.setItem = () => { throw new Error('quota') }
      const store = useThreeHoleTestStore()
      expect(() => store.saveConfig()).not.toThrow()
      localStorage.setItem = orig
    })
  })

  describe('loadConfig', () => {
    it('后端有数据 → 使用后端配置 + 写 localStorage', async () => {
      const backendConfig = {
        ...useThreeHoleTestStore().config,
        dwellTimeMs: 9999,
        probeChannels: [{ name: 'X', role: 'threeHole.p1', channel: 5, enabled: true }],
      }
      mockConfigService.LoadThreeHoleProbe1Config.mockResolvedValue(backendConfig)
      // loadConfig 末尾会尝试恢复校准文件，但 config.calibFiles=[] → 不进入恢复分支
      const store = useThreeHoleTestStore()
      await store.loadConfig()
      expect(store.config.dwellTimeMs).toBe(9999)
      expect(localStorage.getItem('threeHoleTestConfig_probe1')).toBeTruthy()
    })

    it('后端返回 null/空 → 回退到 loadConfigFromLocal', async () => {
      mockConfigService.LoadThreeHoleProbe1Config.mockResolvedValue(null)
      localStorage.setItem('threeHoleTestConfig_probe1', JSON.stringify({
        ...useThreeHoleTestStore().config,
        dwellTimeMs: 7777,
        probeChannels: [{ name: 'X', role: 'threeHole.p1', channel: 0, enabled: true }],
      }))
      const store = useThreeHoleTestStore()
      await store.loadConfig()
      expect(store.config.dwellTimeMs).toBe(7777)
    })

    it('后端抛错 → 回退到 loadConfigFromLocal', async () => {
      mockConfigService.LoadThreeHoleProbe1Config.mockRejectedValue(new Error('boom'))
      localStorage.setItem('threeHoleTestConfig_probe1', JSON.stringify({
        ...useThreeHoleTestStore().config,
        dwellTimeMs: 6666,
        probeChannels: [{ name: 'X', role: 'threeHole.p1', channel: 0, enabled: true }],
      }))
      const store = useThreeHoleTestStore()
      await store.loadConfig()
      expect(store.config.dwellTimeMs).toBe(6666)
    })

    it('probe2 → 调用 LoadThreeHoleProbe2Config', async () => {
      mockConfigService.LoadThreeHoleProbe2Config.mockResolvedValue(null)
      const store = useThreeHoleTestStore()
      store.init('probe2')
      await store.loadConfig()
      expect(mockConfigService.LoadThreeHoleProbe2Config).toHaveBeenCalledTimes(1)
    })

    it('config 中有 calibFiles → 尝试恢复加载', async () => {
      const backendConfig = {
        ...useThreeHoleTestStore().config,
        calibFiles: [{ filePath: 'C:\\a.prb', fileName: 'a.prb', cMa: 0 }],
      }
      mockConfigService.LoadThreeHoleProbe1Config.mockResolvedValue(backendConfig)
      mockThreeHoleService.LoadThreeHoleCalibFiles.mockResolvedValue(undefined)
      mockThreeHoleService.GetThreeHoleCalibInfo.mockResolvedValue([{ cMa: 1.5 }])
      const store = useThreeHoleTestStore()
      await store.loadConfig()
      expect(mockThreeHoleService.LoadThreeHoleCalibFiles).toHaveBeenCalledWith('probe1', ['C:\\a.prb'])
      expect(store.calibLoaded).toBe(true)
      expect(store.calibFiles).toEqual(['C:\\a.prb'])
      expect(store.config.calibFiles[0].cMa).toBe(1.5)
    })

    it('恢复校准文件失败 → 不抛出（仅 console.error）', async () => {
      const backendConfig = {
        ...useThreeHoleTestStore().config,
        calibFiles: [{ filePath: 'C:\\a.prb', fileName: 'a.prb', cMa: 0 }],
      }
      mockConfigService.LoadThreeHoleProbe1Config.mockResolvedValue(backendConfig)
      mockThreeHoleService.LoadThreeHoleCalibFiles.mockRejectedValue(new Error('load fail'))
      const store = useThreeHoleTestStore()
      await expect(store.loadConfig()).resolves.toBeUndefined()
      expect(store.calibLoaded).toBe(false)
    })
  })
})
