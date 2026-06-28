import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

// ---- mock 边界 ----
// fiveHoleTest store 依赖：
//   - @bindings/.../app 的 FiveHoleService（无 ConfigService，配置走 FiveHoleService.SaveFiveHoleConfig/LoadFiveHoleConfig）
//   - @wailsio/runtime 的 Events.On/Off（经 createWailsEventListener 间接调用）
//   - ../motion 的 useMotionStore（用于检查每探针 motionAlpha/motionBeta 控制器连接状态）
//   - ../device 的 ensureDevicesAcquiring（聚合多设备 IDs 自动恢复采集）
//   - ../utils/csv 的 downloadCSV（exportProbeCSV 按探针独立导出）
//   - ../api/enums 的 FiveHoleChannelRole / TraversalPattern / AxisName（纯常量，直接 import 真实模块）
type EventsOnFn = (channel: string, handler: (e: any) => void) => () => void
type EventsOffFn = (channel: string) => void

const { mockFiveHoleService, mockEventsOn, mockEventsOff, mockMotionStore, mockEnsureDevicesAcquiring, mockDownloadCSV } = vi.hoisted(() => ({
  mockFiveHoleService: {
    SelectFiveHoleCalibFiles: vi.fn(),
    LoadFiveHoleCalibFiles: vi.fn(),
    GetFiveHoleCalibInfo: vi.fn(),
    StartFiveHoleTraversal: vi.fn(),
    PauseFiveHoleTraversal: vi.fn(),
    ResumeFiveHoleTraversal: vi.fn(),
    StopFiveHoleTraversal: vi.fn(),
    GetFiveHoleTraversalStatus: vi.fn(),
    StartFiveHoleRealtimeMonitor: vi.fn(),
    StopFiveHoleRealtimeMonitor: vi.fn(),
    SelectAndStartFiveHoleRealtimeRecording: vi.fn(),
    StopFiveHoleRealtimeRecording: vi.fn(),
    SaveFiveHoleConfig: vi.fn(),
    LoadFiveHoleConfig: vi.fn(),
  },
  mockEventsOn: vi.fn<EventsOnFn>(() => () => {}),
  mockEventsOff: vi.fn<EventsOffFn>(),
  // motion store mock：仅暴露 startTest 守卫中用到的 statuses
  mockMotionStore: {
    statuses: [] as any[],
  },
  mockEnsureDevicesAcquiring: vi.fn<(ids: Iterable<string>) => Promise<string[]>>(() => Promise.resolve([])),
  mockDownloadCSV: vi.fn<(prefix: string, headers: string[], rows: (string | number)[][]) => void>(),
}))

vi.mock('@bindings/yx-daq/internal/app', () => ({
  FiveHoleService: mockFiveHoleService,
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

import { useFiveHoleTestStore } from '../fiveHoleTest'
import { FiveHoleChannelRole, TraversalPattern, AxisName } from '../../api/enums'

describe('stores/fiveHoleTest', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    Object.values(mockFiveHoleService).forEach(fn => fn.mockReset())
    mockEventsOn.mockClear()
    mockEventsOff.mockClear()
    mockMotionStore.statuses = []
    mockEnsureDevicesAcquiring.mockReset()
    mockEnsureDevicesAcquiring.mockResolvedValue([])
    mockDownloadCSV.mockReset()
  })

  // ==================== 初始状态 + computed ====================
  describe('初始状态 + computed', () => {
    it('默认 taskStatus=null → isRunning=false, isPaused=false, statusText=未启动', () => {
      const store = useFiveHoleTestStore()
      expect(store.taskStatus).toBeNull()
      expect(store.isRunning).toBe(false)
      expect(store.isPaused).toBe(false)
      expect(store.statusText).toBe('未启动')
      expect(store.lastError).toBe('')
    })

    it('status=running → isRunning=true, statusText=运行中', () => {
      const store = useFiveHoleTestStore()
      store.taskStatus = { status: 'running' } as any
      expect(store.isRunning).toBe(true)
      expect(store.isPaused).toBe(false)
      expect(store.statusText).toBe('运行中')
    })

    it('status=paused → isRunning=true, isPaused=true, statusText=已暂停', () => {
      const store = useFiveHoleTestStore()
      store.taskStatus = { status: 'paused' } as any
      expect(store.isRunning).toBe(true)
      expect(store.isPaused).toBe(true)
      expect(store.statusText).toBe('已暂停')
    })

    it('status=completed → isRunning=false, statusText=已完成', () => {
      const store = useFiveHoleTestStore()
      store.taskStatus = { status: 'completed' } as any
      expect(store.isRunning).toBe(false)
      expect(store.statusText).toBe('已完成')
    })

    it('status=error → statusText=错误', () => {
      const store = useFiveHoleTestStore()
      store.taskStatus = { status: 'error' } as any
      expect(store.statusText).toBe('错误')
    })

    it('status=未知值 → 原样返回', () => {
      const store = useFiveHoleTestStore()
      store.taskStatus = { status: 'weird' } as any
      expect(store.statusText).toBe('weird')
    })

    it('默认 config 包含 3 个 probes（probe1/probe2/probe3），均 enabled', () => {
      const store = useFiveHoleTestStore()
      expect(store.config.probes).toHaveLength(3)
      expect(store.config.probes.map(p => p.probeId)).toEqual(['probe1', 'probe2', 'probe3'])
      expect(store.config.probes.every(p => p.enabled)).toBe(true)
    })

    it('默认每探针 5 个 probeChannels（P1-P5），role 与 FiveHoleChannelRole 对齐', () => {
      const store = useFiveHoleTestStore()
      const p1 = store.config.probes[0]
      expect(p1.probeChannels.map(c => c.role)).toEqual([
        FiveHoleChannelRole.P1, FiveHoleChannelRole.P2, FiveHoleChannelRole.P3,
        FiveHoleChannelRole.P4, FiveHoleChannelRole.P5,
      ])
    })

    it('默认 layout.pattern=RECTANGLE，motionAlpha.axis=X, motionBeta.axis=Y', () => {
      const store = useFiveHoleTestStore()
      expect(store.config.layout.pattern).toBe(TraversalPattern.RECTANGLE)
      expect(store.config.probes[0].motionAlpha.axis).toBe(AxisName.X)
      expect(store.config.probes[0].motionBeta.axis).toBe(AxisName.Y)
    })

    it('默认 calibLoadedMap 三个探针均为 false，allCalibLoaded=false', () => {
      const store = useFiveHoleTestStore()
      expect(store.calibLoadedMap).toEqual({ probe1: false, probe2: false, probe3: false })
      expect(store.allCalibLoaded).toBe(false)
    })

    it('enabledProbes 默认返回全部 3 个', () => {
      const store = useFiveHoleTestStore()
      expect(store.enabledProbes).toHaveLength(3)
    })

    it('禁用 probe3 后 enabledProbes 仅含 probe1/probe2，allCalibLoaded 仅依赖前两者', () => {
      const store = useFiveHoleTestStore()
      store.config.probes[2].enabled = false
      expect(store.enabledProbes.map(p => p.probeId)).toEqual(['probe1', 'probe2'])
      // 设置 probe1/probe2 已加载，probe3 未加载 → allCalibLoaded=true（probe3 被排除）
      store.calibLoadedMap = { probe1: true, probe2: true, probe3: false }
      expect(store.allCalibLoaded).toBe(true)
    })

    it('completeProbeDataPoints 初始为 null', () => {
      const store = useFiveHoleTestStore()
      expect(store.completeProbeDataPoints).toBeNull()
    })
  })

  // ==================== clearError ====================
  describe('clearError', () => {
    it('清空 lastError', () => {
      const store = useFiveHoleTestStore()
      store.lastError = 'some error'
      store.clearError()
      expect(store.lastError).toBe('')
    })
  })

  // ==================== fetchStatus ====================
  describe('fetchStatus', () => {
    it('成功 → 写入 taskStatus', async () => {
      mockFiveHoleService.GetFiveHoleTraversalStatus.mockResolvedValue({ status: 'running', totalPoints: 10 })
      const store = useFiveHoleTestStore()
      await store.fetchStatus()
      expect(store.taskStatus).toMatchObject({ status: 'running', totalPoints: 10 })
    })

    it('失败 → 仅 console.warn，不抛出，taskStatus 保持原值', async () => {
      mockFiveHoleService.GetFiveHoleTraversalStatus.mockRejectedValue(new Error('boom'))
      const spy = vi.spyOn(console, 'warn').mockImplementation(() => {})
      const store = useFiveHoleTestStore()
      store.taskStatus = { status: 'idle' } as any
      await store.fetchStatus()
      expect(store.taskStatus?.status).toBe('idle')
      spy.mockRestore()
    })
  })

  // ==================== ensureDevicesAcquiring（聚合多设备） ====================
  describe('ensureDevicesAcquiring（聚合多设备 IDs）', () => {
    it('pAtm/tAtm + 多探针通道的设备 IDs 全部聚合传入 ensureDevicesAcquiringByIds', async () => {
      const store = useFiveHoleTestStore()
      store.config.pAtmDeviceId = 'dev-atm'
      store.config.tAtmDeviceId = 'dev-atm' // 与 pAtm 相同，应去重
      store.config.probes[0].probeChannels[0].deviceId = 'dev-p1'
      store.config.probes[1].probeChannels[0].deviceId = 'dev-p2'
      store.config.probes[2].probeChannels[0].deviceId = 'dev-p1' // 与 probe1 相同，应去重
      mockEnsureDevicesAcquiring.mockResolvedValue([])

      await store.ensureDevicesAcquiring()

      // Set 去重后应包含 dev-atm/dev-p1/dev-p2 共 3 个
      const passedIds = mockEnsureDevicesAcquiring.mock.calls[0][0] as Iterable<string>
      expect([...passedIds].sort()).toEqual(['dev-atm', 'dev-p1', 'dev-p2'])
    })

    it('禁用的探针通道不参与聚合', async () => {
      const store = useFiveHoleTestStore()
      store.config.probes[2].enabled = false
      store.config.probes[0].probeChannels[0].deviceId = 'dev-active'
      store.config.probes[2].probeChannels[0].deviceId = 'dev-disabled'
      mockEnsureDevicesAcquiring.mockResolvedValue([])

      await store.ensureDevicesAcquiring()

      const passedIds = [...(mockEnsureDevicesAcquiring.mock.calls[0][0] as Iterable<string>)]
      expect(passedIds).toContain('dev-active')
      expect(passedIds).not.toContain('dev-disabled')
    })

    it('返回错误 → 最后一个写入 lastError', async () => {
      const store = useFiveHoleTestStore()
      store.config.pAtmDeviceId = 'd1'
      mockEnsureDevicesAcquiring.mockResolvedValue(['d1 连接失败', 'd1 启动采集失败'])

      await store.ensureDevicesAcquiring()

      // 实现按顺序赋值，最后一个生效
      expect(store.lastError).toBe('d1 启动采集失败')
    })
  })

  // ==================== selectCalibFiles（按探针独立） ====================
  describe('selectCalibFiles（按探针独立）', () => {
    it('用户取消（返回空数组）→ 不加载、不调用 ensureDevicesAcquiring', async () => {
      mockFiveHoleService.SelectFiveHoleCalibFiles.mockResolvedValue([])
      const store = useFiveHoleTestStore()
      await store.selectCalibFiles('probe2')
      expect(mockFiveHoleService.LoadFiveHoleCalibFiles).not.toHaveBeenCalled()
      expect(mockFiveHoleService.GetFiveHoleCalibInfo).not.toHaveBeenCalled()
      expect(mockEnsureDevicesAcquiring).not.toHaveBeenCalled()
      expect(store.calibLoadedMap.probe2).toBe(false)
    })

    it('选择文件 → 加载 + 获取 calibInfo + calibLoadedMap[probeId]=true + 写入 config.probes[i].calibFiles + 触发 ensureDevicesAcquiring', async () => {
      mockFiveHoleService.SelectFiveHoleCalibFiles.mockResolvedValue(['C:/a.prb'])
      mockFiveHoleService.LoadFiveHoleCalibFiles.mockResolvedValue(undefined)
      mockFiveHoleService.GetFiveHoleCalibInfo.mockResolvedValue([
        { cMa: 1.5, validRange: { alphaMin: -20, alphaMax: 20, betaMin: -15, betaMax: 15, machMin: 0, machMax: 0.8 } },
      ])
      const store = useFiveHoleTestStore()
      await store.selectCalibFiles('probe1')
      expect(mockFiveHoleService.LoadFiveHoleCalibFiles).toHaveBeenCalledWith('probe1', ['C:/a.prb'])
      expect(store.calibLoadedMap.probe1).toBe(true)
      expect(store.calibFilesMap.probe1).toEqual(['C:/a.prb'])
      const probe1 = store.config.probes.find(p => p.probeId === 'probe1')!
      expect(probe1.calibFiles).toHaveLength(1)
      expect(probe1.calibFiles[0]).toMatchObject({ filePath: 'C:/a.prb', fileName: 'a.prb', cMa: 1.5 })
      expect(mockEnsureDevicesAcquiring).toHaveBeenCalledTimes(1)
    })

    it('SelectFiveHoleCalibFiles 抛错 → 写入 lastError（带 probeId）', async () => {
      mockFiveHoleService.SelectFiveHoleCalibFiles.mockRejectedValue(new Error('cancel'))
      const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
      const store = useFiveHoleTestStore()
      await store.selectCalibFiles('probe3')
      expect(store.lastError).toContain('probe3')
      expect(store.lastError).toContain('cancel')
      expect(store.calibLoadedMap.probe3).toBe(false)
      spy.mockRestore()
    })

    it('LoadFiveHoleCalibFiles 抛错 → 写入 lastError', async () => {
      mockFiveHoleService.SelectFiveHoleCalibFiles.mockResolvedValue(['C:/b.prb'])
      mockFiveHoleService.LoadFiveHoleCalibFiles.mockRejectedValue(new Error('load boom'))
      const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
      const store = useFiveHoleTestStore()
      await store.selectCalibFiles('probe1')
      expect(store.lastError).toContain('load boom')
      expect(store.calibLoadedMap.probe1).toBe(false)
      spy.mockRestore()
    })
  })

  // ==================== startTest 守卫 ====================
  describe('startTest 守卫', () => {
    it('isRunning=true → 直接返回，不调用 StartFiveHoleTraversal', async () => {
      const store = useFiveHoleTestStore()
      store.taskStatus = { status: 'running' } as any
      // 即使其他守卫条件不满足也不应进入后续检查
      store.calibLoadedMap = { probe1: false, probe2: false, probe3: false }
      await store.startTest()
      expect(mockFiveHoleService.StartFiveHoleTraversal).not.toHaveBeenCalled()
    })

    it('taskStatus=paused → isRunning=true 触发静默早退（line 165），lastError 保持空', async () => {
      const store = useFiveHoleTestStore()
      store.taskStatus = { status: 'paused' } as any
      await store.startTest()
      // isRunning 对 paused 为 true，line 165 早退，line 166-168 不可达
      expect(store.lastError).toBe('')
      expect(mockFiveHoleService.StartFiveHoleTraversal).not.toHaveBeenCalled()
    })

    it('allCalibLoaded=false → 拒绝（请为所有启用的探针加载校准文件）', async () => {
      const store = useFiveHoleTestStore()
      // 默认 calibLoadedMap 全 false → allCalibLoaded=false
      await store.startTest()
      expect(store.lastError).toBe('请为所有启用的探针加载校准文件')
      expect(mockFiveHoleService.StartFiveHoleTraversal).not.toHaveBeenCalled()
    })

    it('allCalibLoaded=true 但 probe1.motionAlpha.controllerId 非空且未在 statuses 中 → 拒绝', async () => {
      const store = useFiveHoleTestStore()
      store.calibLoadedMap = { probe1: true, probe2: true, probe3: true }
      store.config.probes[0].motionAlpha.controllerId = 'mc-x'
      // motionStore.statuses 为空 → 未找到 mc-x
      await store.startTest()
      expect(store.lastError).toContain('probe1')
      expect(store.lastError).toContain('mc-x')
      expect(store.lastError).toContain('未连接')
      expect(mockFiveHoleService.StartFiveHoleTraversal).not.toHaveBeenCalled()
    })

    it('motionAlpha 控制器已连接但 motionBeta 未连接 → 拒绝（带 probeId 与 beta controllerId）', async () => {
      const store = useFiveHoleTestStore()
      store.calibLoadedMap = { probe1: true, probe2: true, probe3: true }
      store.config.probes[0].motionAlpha.controllerId = 'mc-alpha'
      store.config.probes[0].motionBeta.controllerId = 'mc-beta'
      mockMotionStore.statuses = [{ id: 'mc-alpha', status: 'Connected' }] // beta 未在列表中
      await store.startTest()
      expect(store.lastError).toContain('probe1')
      expect(store.lastError).toContain('mc-beta')
      expect(mockFiveHoleService.StartFiveHoleTraversal).not.toHaveBeenCalled()
    })

    it('controllerId 存在但 status !== Connected → 拒绝', async () => {
      const store = useFiveHoleTestStore()
      store.calibLoadedMap = { probe1: true, probe2: true, probe3: true }
      store.config.probes[1].motionAlpha.controllerId = 'mc-dis'
      mockMotionStore.statuses = [{ id: 'mc-dis', status: 'Disconnected' }]
      await store.startTest()
      expect(store.lastError).toContain('probe2')
      expect(store.lastError).toContain('mc-dis')
      expect(mockFiveHoleService.StartFiveHoleTraversal).not.toHaveBeenCalled()
    })

    it('controllerId 为空 → 跳过该轴检查（不视为未连接）', async () => {
      const store = useFiveHoleTestStore()
      store.calibLoadedMap = { probe1: true, probe2: true, probe3: true }
      // 默认所有 controllerId 为空，应跳过运动控制器检查
      mockFiveHoleService.StartFiveHoleTraversal.mockResolvedValue('task-1')
      mockFiveHoleService.GetFiveHoleTraversalStatus.mockResolvedValue({ status: 'running' })
      await store.startTest()
      expect(mockFiveHoleService.StartFiveHoleTraversal).toHaveBeenCalledTimes(1)
      expect(store.taskStatus?.status).toBe('running')
    })

    it('StartFiveHoleTraversal 抛错 → lastError 写入 + resetRuntimeState + taskStatus=null', async () => {
      const store = useFiveHoleTestStore()
      store.calibLoadedMap = { probe1: true, probe2: true, probe3: true }
      mockFiveHoleService.StartFiveHoleTraversal.mockRejectedValue(new Error('start boom'))
      const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
      // 预置一些运行时数据，验证失败回滚
      store.realtime = { pointId: 'x' } as any
      store.progress = { progress: 50 } as any
      await store.startTest()
      expect(store.lastError).toContain('启动测试失败')
      expect(store.lastError).toContain('start boom')
      expect(store.taskStatus).toBeNull()
      expect(store.realtime).toBeNull()
      expect(store.progress).toBeNull()
      spy.mockRestore()
    })

    it('启动前清理 completeProbeDataPoints（避免旧测试数据残留）', async () => {
      const store = useFiveHoleTestStore()
      store.calibLoadedMap = { probe1: true, probe2: true, probe3: true }
      // 预置旧数据
      store.setCompleteData({ probe1: [{ pointId: 'old', probeId: 'probe1', x: 0, y: 0, rawData: {} as any, interpResult: {} as any, sampleCount: 1, timestamp: 1 }] })
      expect(store.completeProbeDataPoints).not.toBeNull()
      mockFiveHoleService.StartFiveHoleTraversal.mockResolvedValue('task-1')
      mockFiveHoleService.GetFiveHoleTraversalStatus.mockResolvedValue({ status: 'running' })
      await store.startTest()
      expect(store.completeProbeDataPoints).toBeNull()
    })
  })

  // ==================== pauseTest / resumeTest ====================
  describe('pauseTest / resumeTest', () => {
    it('pauseTest → 调用 PauseFiveHoleTraversal + fetchStatus', async () => {
      mockFiveHoleService.PauseFiveHoleTraversal.mockResolvedValue(undefined)
      mockFiveHoleService.GetFiveHoleTraversalStatus.mockResolvedValue({ status: 'paused' })
      const store = useFiveHoleTestStore()
      await store.pauseTest()
      expect(mockFiveHoleService.PauseFiveHoleTraversal).toHaveBeenCalledTimes(1)
      expect(store.taskStatus?.status).toBe('paused')
    })

    it('resumeTest → 调用 ResumeFiveHoleTraversal + fetchStatus', async () => {
      mockFiveHoleService.ResumeFiveHoleTraversal.mockResolvedValue(undefined)
      mockFiveHoleService.GetFiveHoleTraversalStatus.mockResolvedValue({ status: 'running' })
      const store = useFiveHoleTestStore()
      await store.resumeTest()
      expect(mockFiveHoleService.ResumeFiveHoleTraversal).toHaveBeenCalledTimes(1)
      expect(store.taskStatus?.status).toBe('running')
    })

    it('pauseTest 抛错 → 不抛出（仅 console.error）', async () => {
      mockFiveHoleService.PauseFiveHoleTraversal.mockRejectedValue(new Error('pause boom'))
      const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
      const store = useFiveHoleTestStore()
      await expect(store.pauseTest()).resolves.toBeUndefined()
      spy.mockRestore()
    })
  })

  // ==================== stopTest（轮询 + 超时） ====================
  describe('stopTest', () => {
    beforeEach(() => vi.useFakeTimers())
    afterEach(() => vi.useRealTimers())

    it('StopFiveHoleTraversal 后状态变 idle → 立即清理 realtime/progress 并退出', async () => {
      mockFiveHoleService.StopFiveHoleTraversal.mockResolvedValue(undefined)
      mockFiveHoleService.GetFiveHoleTraversalStatus.mockResolvedValue({ status: 'idle' })
      const store = useFiveHoleTestStore()
      store.realtime = { pointId: 'x' } as any
      store.progress = { progress: 50 } as any
      await store.stopTest()
      expect(mockFiveHoleService.GetFiveHoleTraversalStatus).toHaveBeenCalledTimes(1)
      expect(store.realtime).toBeNull()
      expect(store.progress).toBeNull()
    })

    it('10 次仍 running → 超时强制清理 realtime/progress', async () => {
      mockFiveHoleService.StopFiveHoleTraversal.mockResolvedValue(undefined)
      mockFiveHoleService.GetFiveHoleTraversalStatus.mockResolvedValue({ status: 'running' })
      const store = useFiveHoleTestStore()
      store.realtime = { pointId: 'x' } as any
      store.progress = { progress: 50 } as any
      const promise = store.stopTest()
      for (let i = 0; i < 10; i++) {
        await vi.advanceTimersByTimeAsync(200)
      }
      await promise
      expect(mockFiveHoleService.GetFiveHoleTraversalStatus).toHaveBeenCalledTimes(10)
      expect(store.realtime).toBeNull()
      expect(store.progress).toBeNull()
    })

    it('StopFiveHoleTraversal 抛错 → 清空 realtime/progress，不抛出', async () => {
      mockFiveHoleService.StopFiveHoleTraversal.mockRejectedValue(new Error('stop boom'))
      const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
      const store = useFiveHoleTestStore()
      store.realtime = { pointId: 'x' } as any
      store.progress = { progress: 50 } as any
      await expect(store.stopTest()).resolves.toBeUndefined()
      expect(store.realtime).toBeNull()
      expect(store.progress).toBeNull()
      spy.mockRestore()
    })
  })

  // ==================== 实时监控 / 录制 ====================
  describe('realtime monitor / recording', () => {
    it('startRealtimeMonitor → 调用 StartFiveHoleRealtimeMonitor(config)', async () => {
      mockFiveHoleService.StartFiveHoleRealtimeMonitor.mockResolvedValue(undefined)
      const store = useFiveHoleTestStore()
      await store.startRealtimeMonitor()
      expect(mockFiveHoleService.StartFiveHoleRealtimeMonitor).toHaveBeenCalledWith(store.config)
    })

    it('stopRealtimeMonitor → 调用 StopFiveHoleRealtimeMonitor', async () => {
      mockFiveHoleService.StopFiveHoleRealtimeMonitor.mockResolvedValue(undefined)
      const store = useFiveHoleTestStore()
      await store.stopRealtimeMonitor()
      expect(mockFiveHoleService.StopFiveHoleRealtimeMonitor).toHaveBeenCalledTimes(1)
    })

    it('selectAndStartRealtimeRecording 成功 → 返回路径字符串', async () => {
      mockFiveHoleService.SelectAndStartFiveHoleRealtimeRecording.mockResolvedValue('D:/rec.csv')
      const store = useFiveHoleTestStore()
      const path = await store.selectAndStartRealtimeRecording()
      expect(path).toBe('D:/rec.csv')
    })

    it('selectAndStartRealtimeRecording 抛错 → 写入 lastError，返回空字符串', async () => {
      mockFiveHoleService.SelectAndStartFiveHoleRealtimeRecording.mockRejectedValue(new Error('cancel'))
      const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
      const store = useFiveHoleTestStore()
      const path = await store.selectAndStartRealtimeRecording()
      expect(path).toBe('')
      expect(store.lastError).toContain('开始实时录制失败')
      spy.mockRestore()
    })

    it('stopRealtimeRecording → 调用 StopFiveHoleRealtimeRecording', async () => {
      mockFiveHoleService.StopFiveHoleRealtimeRecording.mockResolvedValue(undefined)
      const store = useFiveHoleTestStore()
      await store.stopRealtimeRecording()
      expect(mockFiveHoleService.StopFiveHoleRealtimeRecording).toHaveBeenCalledTimes(1)
    })

    it('startRealtimeMonitor 抛错 → 不抛出', async () => {
      mockFiveHoleService.StartFiveHoleRealtimeMonitor.mockRejectedValue(new Error('boom'))
      const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
      const store = useFiveHoleTestStore()
      await expect(store.startRealtimeMonitor()).resolves.toBeUndefined()
      spy.mockRestore()
    })
  })

  // ==================== 事件监听（全局，不按 probeID 区分） ====================
  describe('事件监听（five-hole:* 全局通道）', () => {
    it('startListening 注册 4 个 five-hole:* 事件通道', () => {
      const store = useFiveHoleTestStore()
      store.startListening()
      const channels = mockEventsOn.mock.calls.map(c => c[0])
      expect(channels).toContain('five-hole:progress')
      expect(channels).toContain('five-hole:realtime')
      expect(channels).toContain('five-hole:complete')
      expect(channels).toContain('five-hole:error')
      expect(mockEventsOn).toHaveBeenCalledTimes(4)
    })

    it('startListening 幂等：重复调用不重复注册', () => {
      const store = useFiveHoleTestStore()
      store.startListening()
      store.startListening()
      expect(mockEventsOn).toHaveBeenCalledTimes(4)
    })

    it('progress 事件 → 更新 progress ref', () => {
      const store = useFiveHoleTestStore()
      store.startListening()
      const handler = mockEventsOn.mock.calls.find(c => c[0] === 'five-hole:progress')![1] as (e: any) => void
      const payload = { taskId: 't1', progress: 75, completedPoints: 8 }
      handler({ data: payload })
      expect(store.progress).toMatchObject({ taskId: 't1', progress: 75 })
    })

    it('realtime 事件 → 更新 realtime ref', () => {
      const store = useFiveHoleTestStore()
      store.startListening()
      const handler = mockEventsOn.mock.calls.find(c => c[0] === 'five-hole:realtime')![1] as (e: any) => void
      const payload = { taskId: 't1', pointId: 'p1', probeRealtime: [] }
      handler({ data: payload })
      expect(store.realtime).toMatchObject({ pointId: 'p1' })
    })

    it('complete 事件 → 缓存 probeDataPoints 到 completeProbeDataPoints + 清空 progress + 触发 fetchStatus', async () => {
      mockFiveHoleService.GetFiveHoleTraversalStatus.mockResolvedValue({ status: 'completed' })
      const store = useFiveHoleTestStore()
      store.startListening()
      const handler = mockEventsOn.mock.calls.find(c => c[0] === 'five-hole:complete')![1] as (e: any) => void
      const payload = {
        taskId: 't1', status: 'completed',
        probeDataPoints: {
          probe1: [{ pointId: 'p1-1', probeId: 'probe1', x: 1, y: 2, rawData: {}, interpResult: {}, sampleCount: 5, timestamp: 100 }],
          probe2: [{ pointId: 'p2-1', probeId: 'probe2', x: 3, y: 4, rawData: {}, interpResult: {}, sampleCount: 5, timestamp: 101 }],
        },
      }
      store.progress = { progress: 50 } as any
      handler({ data: payload })
      // fetchStatus 是 async，等微任务刷新
      await new Promise(r => setTimeout(r, 0))
      expect(store.completeProbeDataPoints).not.toBeNull()
      expect(store.completeProbeDataPoints?.probe1).toHaveLength(1)
      expect(store.completeProbeDataPoints?.probe2).toHaveLength(1)
      expect(store.completeProbeDataPoints?.probe3).toBeUndefined()
      expect(store.progress).toBeNull()
      expect(store.taskStatus?.status).toBe('completed')
    })

    it('error 事件 isFatal=true → 写入 lastError + 清空 progress + 触发 fetchStatus', async () => {
      mockFiveHoleService.GetFiveHoleTraversalStatus.mockResolvedValue({ status: 'error' })
      const store = useFiveHoleTestStore()
      store.startListening()
      const handler = mockEventsOn.mock.calls.find(c => c[0] === 'five-hole:error')![1] as (e: any) => void
      store.progress = { progress: 50 } as any
      handler({ data: { taskId: 't1', error: '致命错误', isFatal: true } })
      await new Promise(r => setTimeout(r, 0))
      expect(store.lastError).toBe('致命错误')
      expect(store.progress).toBeNull()
      expect(mockFiveHoleService.GetFiveHoleTraversalStatus).toHaveBeenCalled()
    })

    it('error 事件 isFatal=false → 写入 lastError 但保留 progress（数据停滞场景）', async () => {
      mockFiveHoleService.GetFiveHoleTraversalStatus.mockResolvedValue({ status: 'paused' })
      const store = useFiveHoleTestStore()
      store.startListening()
      const handler = mockEventsOn.mock.calls.find(c => c[0] === 'five-hole:error')![1] as (e: any) => void
      store.progress = { progress: 30 } as any
      handler({ data: { taskId: 't1', error: '数据停滞', isFatal: false } })
      await new Promise(r => setTimeout(r, 0))
      expect(store.lastError).toBe('数据停滞')
      expect(store.progress).toMatchObject({ progress: 30 })
      expect(mockFiveHoleService.GetFiveHoleTraversalStatus).toHaveBeenCalled()
    })

    it('stopListening → 调用 EventsOff 注销 4 个通道', () => {
      const store = useFiveHoleTestStore()
      store.startListening()
      store.stopListening()
      expect(mockEventsOff).toHaveBeenCalledTimes(4)
    })

    it('stopListening 幂等：未 start 时调用不抛错且不调用 Off', () => {
      const store = useFiveHoleTestStore()
      store.stopListening()
      expect(mockEventsOff).not.toHaveBeenCalled()
    })

    it('stopListening 后再次 startListening 可重新注册', () => {
      const store = useFiveHoleTestStore()
      store.startListening()
      store.stopListening()
      mockEventsOn.mockClear()
      mockEventsOff.mockClear()
      store.startListening()
      expect(mockEventsOn).toHaveBeenCalledTimes(4)
    })
  })

  // ==================== setCompleteData + exportProbeCSV ====================
  describe('setCompleteData + exportProbeCSV（按探针独立导出）', () => {
    it('setCompleteData 写入 completeProbeDataPoints', () => {
      const store = useFiveHoleTestStore()
      const data = { probe1: [{ pointId: 'p1', probeId: 'probe1', x: 1, y: 2, rawData: {} as any, interpResult: {} as any, sampleCount: 1, timestamp: 1 }] }
      store.setCompleteData(data)
      // Pinia ref 解包后是 reactive proxy，不保留原引用 → 用 toStrictEqual 做深比较
      expect(store.completeProbeDataPoints).toStrictEqual(data)
    })

    it('exportProbeCSV 无数据（completeProbeDataPoints=null）→ 不调用 downloadCSV', () => {
      const store = useFiveHoleTestStore()
      store.exportProbeCSV('probe1')
      expect(mockDownloadCSV).not.toHaveBeenCalled()
    })

    it('exportProbeCSV 指定 probeId 数据为空数组 → 不调用 downloadCSV', () => {
      const store = useFiveHoleTestStore()
      store.setCompleteData({ probe1: [], probe2: [] })
      store.exportProbeCSV('probe1')
      expect(mockDownloadCSV).not.toHaveBeenCalled()
    })

    it('exportProbeCSV 有数据 → 调用 downloadCSV，前缀为 five-hole-{probeId}，headers 包含 Beta 列', () => {
      const store = useFiveHoleTestStore()
      store.setCompleteData({
        probe2: [{
          pointId: 'p1', probeId: 'probe2', x: 1.5, y: 2.5,
          rawData: { p1: 100, p2: 101, p3: 102, p4: 103, p5: 104, pAtm: 90, tAtm: 25 },
          interpResult: {
            ptProbe: 1, psProbe: 2, machProbe: 0.5, alphaProbe: 5, betaProbe: -3,
            velocityProbe: 100, casProbe: 90, satProbe: 280, dynamicPressure: 10,
            density: 1.2, vxProbe: 1, vyProbe: 0, vzProbe: 0, valid: true,
          },
          sampleCount: 10, timestamp: 12345,
        }],
      })
      store.exportProbeCSV('probe2')
      expect(mockDownloadCSV).toHaveBeenCalledTimes(1)
      const [prefix, headers] = mockDownloadCSV.mock.calls[0]
      expect(prefix).toBe('five-hole-probe2')
      // 关键列存在
      expect(headers).toContain('侧滑角Beta')
      expect(headers).toContain('攻角Alpha')
      expect(headers).toContain('P1')
      expect(headers).toContain('P5')
      expect(headers).toContain('P∞')
      expect(headers).toContain('T∞')
    })
  })

  // ==================== saveConfig ====================
  describe('saveConfig', () => {
    it('成功 → localStorage 写入 + SaveFiveHoleConfig 调用', async () => {
      mockFiveHoleService.SaveFiveHoleConfig.mockResolvedValue(undefined)
      const store = useFiveHoleTestStore()
      store.config.name = 'test-config'
      store.saveConfig()
      // localStorage 同步写入
      const raw = localStorage.getItem('fiveHoleTestConfig')
      expect(raw).not.toBeNull()
      expect(JSON.parse(raw!).name).toBe('test-config')
      // SaveFiveHoleConfig 异步触发，等微任务
      await new Promise(r => setTimeout(r, 0))
      expect(mockFiveHoleService.SaveFiveHoleConfig).toHaveBeenCalledWith(store.config)
    })

    it('SaveFiveHoleConfig 抛错 → 仅 console.error，不抛出', async () => {
      mockFiveHoleService.SaveFiveHoleConfig.mockRejectedValue(new Error('save boom'))
      const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
      const store = useFiveHoleTestStore()
      store.saveConfig()
      await new Promise(r => setTimeout(r, 0))
      spy.mockRestore()
    })

    it('localStorage.setItem 抛错 → 不抛出（仅 console.error）', () => {
      const spy = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
        throw new Error('quota exceeded')
      })
      const errSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
      const store = useFiveHoleTestStore()
      expect(() => store.saveConfig()).not.toThrow()
      spy.mockRestore()
      errSpy.mockRestore()
    })
  })

  // ==================== loadConfig ====================
  describe('loadConfig', () => {
    it('后端返回有效配置 → 写入 config + 同步 localStorage，不读 localStorage', async () => {
      const loaded = {
        name: 'from-backend',
        probes: [{ probeId: 'probe1', enabled: true, probeChannels: [], motionAlpha: { controllerId: '', axis: 'X' }, motionBeta: { controllerId: '', axis: 'Y' }, calibFiles: [] }],
        layout: { pattern: 'rectangle' },
      }
      mockFiveHoleService.LoadFiveHoleConfig.mockResolvedValue(loaded)
      const store = useFiveHoleTestStore()
      await store.loadConfig()
      expect(store.config.name).toBe('from-backend')
      expect(JSON.parse(localStorage.getItem('fiveHoleTestConfig')!).name).toBe('from-backend')
      // 无 calibFiles → 不调用 LoadFiveHoleCalibFiles
      expect(mockFiveHoleService.LoadFiveHoleCalibFiles).not.toHaveBeenCalled()
    })

    it('后端返回空配置（probes 为空）→ 回退到 localStorage', async () => {
      mockFiveHoleService.LoadFiveHoleConfig.mockResolvedValue({ probes: [] })
      localStorage.setItem('fiveHoleTestConfig', JSON.stringify({ name: 'from-local', probes: [{ probeId: 'probe1', enabled: true, probeChannels: [], motionAlpha: { controllerId: '', axis: 'X' }, motionBeta: { controllerId: '', axis: 'Y' }, calibFiles: [] }] }))
      const store = useFiveHoleTestStore()
      await store.loadConfig()
      expect(store.config.name).toBe('from-local')
    })

    it('LoadFiveHoleConfig 抛错 → 回退到 localStorage', async () => {
      mockFiveHoleService.LoadFiveHoleConfig.mockRejectedValue(new Error('rpc err'))
      localStorage.setItem('fiveHoleTestConfig', JSON.stringify({ name: 'fallback', probes: [{ probeId: 'probe1', enabled: true, probeChannels: [], motionAlpha: { controllerId: '', axis: 'X' }, motionBeta: { controllerId: '', axis: 'Y' }, calibFiles: [] }] }))
      const store = useFiveHoleTestStore()
      await store.loadConfig()
      expect(store.config.name).toBe('fallback')
    })

    it('恢复校准文件：probes[i].calibFiles 非空 → 调用 LoadFiveHoleCalibFiles + GetFiveHoleCalibInfo + calibLoadedMap=true', async () => {
      const loaded = {
        name: 'with-calib',
        probes: [
          {
            probeId: 'probe1', enabled: true, probeChannels: [],
            motionAlpha: { controllerId: '', axis: 'X' }, motionBeta: { controllerId: '', axis: 'Y' },
            calibFiles: [{ filePath: 'C:/saved.prb', fileName: 'saved.prb', cMa: 0, validRange: { alphaMin: -30, alphaMax: 30, betaMin: -30, betaMax: 30, machMin: 0, machMax: 0 } }],
          },
          { probeId: 'probe2', enabled: true, probeChannels: [], motionAlpha: { controllerId: '', axis: 'X' }, motionBeta: { controllerId: '', axis: 'Y' }, calibFiles: [] },
          { probeId: 'probe3', enabled: true, probeChannels: [], motionAlpha: { controllerId: '', axis: 'X' }, motionBeta: { controllerId: '', axis: 'Y' }, calibFiles: [] },
        ],
        layout: { pattern: 'rectangle' },
      }
      mockFiveHoleService.LoadFiveHoleConfig.mockResolvedValue(loaded)
      mockFiveHoleService.LoadFiveHoleCalibFiles.mockResolvedValue(undefined)
      mockFiveHoleService.GetFiveHoleCalibInfo.mockResolvedValue([
        { cMa: 2.0, validRange: { alphaMin: -25, alphaMax: 25, betaMin: -20, betaMax: 20, machMin: 0, machMax: 0.7 } },
      ])
      const store = useFiveHoleTestStore()
      await store.loadConfig()
      expect(mockFiveHoleService.LoadFiveHoleCalibFiles).toHaveBeenCalledWith('probe1', ['C:/saved.prb'])
      expect(store.calibLoadedMap.probe1).toBe(true)
      expect(store.calibFilesMap.probe1).toEqual(['C:/saved.prb'])
      // probe1 的 calibFiles[0].cMa 被覆盖为最新值 2.0
      expect(store.config.probes[0].calibFiles[0].cMa).toBe(2.0)
      // probe2/probe3 calibFiles 为空 → 不调用
      expect(mockFiveHoleService.LoadFiveHoleCalibFiles).toHaveBeenCalledTimes(1)
    })

    it('恢复校准文件失败 → 不抛出（仅 console.error），calibLoadedMap 保持 false', async () => {
      const loaded = {
        name: 'restore-fail',
        probes: [
          {
            probeId: 'probe1', enabled: true, probeChannels: [],
            motionAlpha: { controllerId: '', axis: 'X' }, motionBeta: { controllerId: '', axis: 'Y' },
            calibFiles: [{ filePath: 'C:/gone.prb', fileName: 'gone.prb', cMa: 0, validRange: { alphaMin: -30, alphaMax: 30, betaMin: -30, betaMax: 30, machMin: 0, machMax: 0 } }],
          },
          { probeId: 'probe2', enabled: true, probeChannels: [], motionAlpha: { controllerId: '', axis: 'X' }, motionBeta: { controllerId: '', axis: 'Y' }, calibFiles: [] },
          { probeId: 'probe3', enabled: true, probeChannels: [], motionAlpha: { controllerId: '', axis: 'X' }, motionBeta: { controllerId: '', axis: 'Y' }, calibFiles: [] },
        ],
        layout: { pattern: 'rectangle' },
      }
      mockFiveHoleService.LoadFiveHoleConfig.mockResolvedValue(loaded)
      mockFiveHoleService.LoadFiveHoleCalibFiles.mockRejectedValue(new Error('文件不存在'))
      const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
      const store = useFiveHoleTestStore()
      await expect(store.loadConfig()).resolves.toBeUndefined()
      expect(store.calibLoadedMap.probe1).toBe(false)
      spy.mockRestore()
    })
  })
})
