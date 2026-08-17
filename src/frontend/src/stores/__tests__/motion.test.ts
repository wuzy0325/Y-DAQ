import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

// ---- mock Wails 绑定 + runtime + types ----
// motion store 同时依赖：
//   - @bindings/.../app 的 MotionService（IPC 调用）
//   - @bindings/.../types 的 AxisConfig / AxisName / EncoderCompensationConfig / MotionControllerProfile 类
//   - @wailsio/runtime 的 Events.On/Off（经 createWailsEventListener）
type EventsOnFn = (channel: string, handler: (e: any) => void) => () => void
type EventsOffFn = (channel: string) => void

const { mockMotionService, mockEventsOn, mockEventsOff, mockTypes } = vi.hoisted(() => ({
  mockMotionService: {
    GetMotionProfiles: vi.fn(),
    GetMotionStatusAll: vi.fn(),
    ConnectMotion: vi.fn(),
    DisconnectMotion: vi.fn(),
    AddMotionProfile: vi.fn(),
    RemoveMotionProfile: vi.fn(),
    UpdateMotionProfile: vi.fn(),
    MotionMoveTo: vi.fn(),
    MotionMoveBy: vi.fn(),
    MotionJog: vi.fn(),
    MotionStop: vi.fn(),
    MotionStopAll: vi.fn(),
    MotionHome: vi.fn(),
    MotionDefinePosition: vi.fn(),
    MotionEmergencyStop: vi.fn(),
    MotionMotorOff: vi.fn(),
    MotionIsAxisMoving: vi.fn(),
    MotionIsMoving: vi.fn(),
    MotionSetAcceleration: vi.fn(),
    MotionSetDeceleration: vi.fn(),
    MotionSetAxisDirection: vi.fn(),
    MotionGetLimitStatus: vi.fn(),
    MotionWaitForComplete: vi.fn(),
    OpenMotionWindow: vi.fn(),
  },
  mockEventsOn: vi.fn<EventsOnFn>(() => () => {}),
  mockEventsOff: vi.fn<EventsOffFn>(),
  // types 类的 createFrom 实际就是透传（合并字段），mock 为透传即可；枚举需提供真实值
  mockTypes: {
    AxisName: { AxisX: 'X', AxisY: 'Y', AxisZ: 'Z', AxisU: 'U', $zero: '' },
    AxisKind: { AxisKindLinear: 'LINEAR', AxisKindRotary: 'ROTARY', $zero: '' },
    AxisConfig: { createFrom: (x: any) => x },
    MotionControllerProfile: { createFrom: (x: any) => x },
    EncoderCompensationConfig: { createFrom: (x: any) => x },
  },
}))

vi.mock('@bindings/yx-daq/internal/app', () => ({
  MotionService: mockMotionService,
}))

vi.mock('@bindings/yx-daq/internal/types', () => mockTypes)

vi.mock('@wailsio/runtime', () => ({
  Events: { On: mockEventsOn, Off: mockEventsOff },
}))

import { useMotionStore } from '../motion'

describe('stores/motion', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    Object.values(mockMotionService).forEach(fn => fn.mockReset())
    mockEventsOn.mockClear()
    mockEventsOff.mockClear()
  })

  // ============ 计算属性 ============
  describe('computed', () => {
    it('初始 connectionStatus=disconnected → isConnected=false', () => {
      const store = useMotionStore()
      expect(store.connectionStatus).toBe('disconnected')
      expect(store.isConnected).toBe(false)
    })

    it('connectionStatus=connected → isConnected=true', () => {
      const store = useMotionStore()
      store.connectionStatus = 'connected'
      expect(store.isConnected).toBe(true)
    })

    it('selectedAxis 默认 X → currentAxis 为 X 轴状态', () => {
      const store = useMotionStore()
      expect(store.selectedAxis).toBe('X')
      expect(store.currentAxis.name).toBe('X')
      expect(store.currentAxis.kind).toBe('LINEAR')
    })

    it('selectAxis 切换后 currentAxis 跟随', () => {
      const store = useMotionStore()
      store.selectAxis('U')
      expect(store.selectedAxis).toBe('U')
      expect(store.currentAxis.name).toBe('U')
      expect(store.currentAxis.kind).toBe('ROTARY')
    })

    it('allAxes 返回 [X, Y, Z, U]', () => {
      const store = useMotionStore()
      expect(store.allAxes.map(a => a.name)).toEqual(['X', 'Y', 'Z', 'U'])
    })

    it('isAnyAxisRunning：所有轴 idle → false', () => {
      const store = useMotionStore()
      expect(store.isAnyAxisRunning).toBe(false)
    })

    it('isAnyAxisRunning：任一轴 running → true', () => {
      const store = useMotionStore()
      store.axisUIStates.X.runState = 'running'
      expect(store.isAnyAxisRunning).toBe(true)
    })

    it('isAnyAxisRunning：error 状态不算 running', () => {
      const store = useMotionStore()
      store.axisUIStates.X.runState = 'error'
      expect(store.isAnyAxisRunning).toBe(false)
    })
  })

  // ============ 日志 ============
  describe('logs', () => {
    it('addLog 在前插入带时间戳消息', () => {
      const store = useMotionStore()
      store.addLog('hello')
      expect(store.logs[0]).toContain('hello')
      expect(store.logs[0]).toMatch(/^\[/)
    })

    it('addLog 超过 100 条时丢弃最旧（pop）', () => {
      const store = useMotionStore()
      for (let i = 0; i < 105; i++) store.addLog(`msg${i}`)
      expect(store.logs.length).toBe(100)
      // 最新在前：logs[0] 是最后一次添加
      expect(store.logs[0]).toContain('msg104')
    })

    it('clearLogs 清空', () => {
      const store = useMotionStore()
      store.addLog('a')
      store.clearLogs()
      expect(store.logs).toEqual([])
    })
  })

  // ============ 配置持久化（saveConfigToLocal / loadConfigFromLocal 为内部函数，经 public action 间接验证）============
  describe('配置持久化', () => {
    it('updateAxisRelativeDistance 触发 saveConfigToLocal → localStorage 有数据', () => {
      const store = useMotionStore()
      store.updateAxisRelativeDistance('X', 25)
      const raw = localStorage.getItem('motionControllerConfig:default')
      expect(raw).toBeTruthy()
      const data = JSON.parse(raw!)
      expect(data.axes.X.relativeDistance).toBe(25)
    })

    it('updateAxisKind 触发 saveConfigToLocal', () => {
      const store = useMotionStore()
      store.updateAxisKind('X', 'ROTARY')
      const raw = localStorage.getItem('motionControllerConfig:default')
      expect(raw).toBeTruthy()
      const data = JSON.parse(raw!)
      expect(data.axes.X.kind).toBe('ROTARY')
    })

    it('startListening 触发 loadConfigFromLocal（恢复 relativeDistance）', () => {
      localStorage.setItem('motionControllerConfig:default', JSON.stringify({
        axes: {
          X: { kind: 'LINEAR', relativeDistance: 15, config: { maxSpeed: 80 } },
        },
      }))
      mockMotionService.GetMotionProfiles.mockResolvedValue([])
      mockMotionService.GetMotionStatusAll.mockResolvedValue([])
      const store = useMotionStore()
      store.startListening()
      expect(store.axisUIStates.X.relativeDistance).toBe(15)
      expect(store.axisUIStates.X.config.maxSpeed).toBe(80)
    })

    it('loadConfigFromLocal 无数据时不动状态（startListening 路径）', () => {
      mockMotionService.GetMotionProfiles.mockResolvedValue([])
      mockMotionService.GetMotionStatusAll.mockResolvedValue([])
      const store = useMotionStore()
      const before = store.axisUIStates.X.relativeDistance
      store.startListening()
      expect(store.axisUIStates.X.relativeDistance).toBe(before)
    })
  })

  // ============ 基本 API ============
  describe('fetchProfiles / fetchStatuses', () => {
    it('fetchProfiles 成功 → 写入 profiles', async () => {
      mockMotionService.GetMotionProfiles.mockResolvedValue([{ id: 'mc-1' }])
      const store = useMotionStore()
      await store.fetchProfiles()
      expect(store.profiles).toEqual([{ id: 'mc-1' }])
    })

    it('fetchProfiles 失败 → 仅 console.warn，不抛出', async () => {
      mockMotionService.GetMotionProfiles.mockRejectedValue(new Error('boom'))
      const store = useMotionStore()
      await expect(store.fetchProfiles()).resolves.toBeUndefined()
    })

    it('fetchStatuses 成功 → 写入 statuses 并同步 connectionStatus', async () => {
      mockMotionService.GetMotionStatusAll.mockResolvedValue([])
      const store = useMotionStore()
      await store.fetchStatuses()
      expect(store.statuses).toEqual([])
      expect(store.connectionStatus).toBe('disconnected')
    })
  })

  // ============ syncConnectionFromStatuses ============
  describe('syncConnectionFromStatuses', () => {
    it('活动控制器 Connected → connectionStatus=connected', async () => {
      mockMotionService.GetMotionStatusAll.mockResolvedValue([
        { id: 'mc-1', status: 'Connected', axes: [] },
      ])
      const store = useMotionStore()
      store.activeControllerId = 'mc-1'
      await store.fetchStatuses()
      expect(store.connectionStatus).toBe('connected')
    })

    it('活动控制器不在线 + 当前非 connecting → disconnected', async () => {
      mockMotionService.GetMotionStatusAll.mockResolvedValue([
        { id: 'mc-1', status: 'Disconnected', axes: [] },
      ])
      const store = useMotionStore()
      store.activeControllerId = 'mc-1'
      store.connectionStatus = 'connected'
      await store.fetchStatuses()
      expect(store.connectionStatus).toBe('disconnected')
    })

    it('当前 connecting → 保持 connecting（不被覆盖）', async () => {
      mockMotionService.GetMotionStatusAll.mockResolvedValue([
        { id: 'mc-1', status: 'Disconnected', axes: [] },
      ])
      const store = useMotionStore()
      store.activeControllerId = 'mc-1'
      store.connectionStatus = 'connecting'
      await store.fetchStatuses()
      expect(store.connectionStatus).toBe('connecting')
    })
  })

  // ============ isControllerConnecting ============
  describe('isControllerConnecting', () => {
    it('connectingIds 包含 id → true', () => {
      const store = useMotionStore()
      store.connectingIds = new Set(['mc-1'])
      expect(store.isControllerConnecting('mc-1')).toBe(true)
    })

    it('statuses 中 status=Connecting → true', () => {
      const store = useMotionStore()
      store.statuses = [{ id: 'mc-1', status: 'Connecting', axes: [] } as any]
      expect(store.isControllerConnecting('mc-1')).toBe(true)
    })

    it('都不在 → false', () => {
      const store = useMotionStore()
      expect(store.isControllerConnecting('mc-1')).toBe(false)
    })
  })

  // ============ connectController / disconnectController ============
  describe('connectController', () => {
    it('成功 → activeControllerId 设置 + connectionStatus=connected + 同步位置', async () => {
      mockMotionService.ConnectMotion.mockResolvedValue(undefined)
      mockMotionService.GetMotionStatusAll.mockResolvedValue([
        { id: 'mc-1', status: 'Connected', axes: [{ name: 'X', position: 12.5, homed: true, moving: false, posLimit: false, negLimit: false, compensating: false }] },
      ])
      const store = useMotionStore()
      const result = await store.connectController('mc-1')
      expect(result.success).toBe(true)
      expect(store.activeControllerId).toBe('mc-1')
      expect(store.connectionStatus).toBe('connected')
      expect(store.connectingIds.has('mc-1')).toBe(false)
      expect(store.axisUIStates.X.currentPosition).toBe(12.5)
      expect(store.axisUIStates.X.isHomed).toBe(true)
    })

    it('失败 → connectionStatus=error + 返回 error 消息 + 仍清理 connectingIds', async () => {
      mockMotionService.ConnectMotion.mockRejectedValue(new Error('connect boom'))
      const store = useMotionStore()
      const result = await store.connectController('mc-1')
      expect(result.success).toBe(false)
      expect(result.error).toBe('connect boom')
      expect(store.connectionStatus).toBe('error')
      expect(store.connectingIds.has('mc-1')).toBe(false)
    })
  })

  describe('disconnectController', () => {
    it('未传 id → 回退到 activeControllerId', async () => {
      mockMotionService.MotionStopAll.mockResolvedValue(undefined)
      mockMotionService.MotionMotorOff.mockResolvedValue(undefined)
      mockMotionService.DisconnectMotion.mockResolvedValue(undefined)
      const store = useMotionStore()
      store.activeControllerId = 'mc-1'
      const result = await store.disconnectController()
      expect(result.success).toBe(true)
      expect(mockMotionService.DisconnectMotion).toHaveBeenCalledWith('mc-1')
      expect(store.connectionStatus).toBe('disconnected')
    })

    it('无目标 id 且无 active → 直接 success', async () => {
      const store = useMotionStore()
      const result = await store.disconnectController()
      expect(result.success).toBe(true)
      expect(mockMotionService.DisconnectMotion).not.toHaveBeenCalled()
    })

    it('断开活动控制器 → 停轴 + 关电机 + 清轴 UI runState', async () => {
      mockMotionService.MotionStopAll.mockResolvedValue(undefined)
      mockMotionService.MotionMotorOff.mockResolvedValue(undefined)
      mockMotionService.DisconnectMotion.mockResolvedValue(undefined)
      const store = useMotionStore()
      store.activeControllerId = 'mc-1'
      store.axisUIStates.X.runState = 'running'
      store.axisUIStates.Y.runState = 'jogging_plus'
      const result = await store.disconnectController('mc-1')
      expect(result.success).toBe(true)
      expect(mockMotionService.MotionStopAll).toHaveBeenCalledWith('mc-1')
      expect(mockMotionService.MotionMotorOff).toHaveBeenCalledWith('mc-1')
      expect(store.axisUIStates.X.runState).toBe('idle')
      expect(store.axisUIStates.Y.runState).toBe('idle')
      expect(store.connectionStatus).toBe('disconnected')
    })

    it('断开非活动控制器 → 不调用 StopAll/MotorOff', async () => {
      mockMotionService.DisconnectMotion.mockResolvedValue(undefined)
      const store = useMotionStore()
      store.activeControllerId = 'mc-1'
      const result = await store.disconnectController('mc-2')
      expect(result.success).toBe(true)
      expect(mockMotionService.MotionStopAll).not.toHaveBeenCalled()
      expect(mockMotionService.MotionMotorOff).not.toHaveBeenCalled()
      expect(mockMotionService.DisconnectMotion).toHaveBeenCalledWith('mc-2')
    })

    it('MotionMotorOff 抛错 → 被吞掉（不影响整体成功）', async () => {
      mockMotionService.MotionStopAll.mockResolvedValue(undefined)
      mockMotionService.MotionMotorOff.mockRejectedValue(new Error('motor off boom'))
      mockMotionService.DisconnectMotion.mockResolvedValue(undefined)
      const store = useMotionStore()
      store.activeControllerId = 'mc-1'
      const result = await store.disconnectController('mc-1')
      expect(result.success).toBe(true)
    })

    it('DisconnectMotion 失败 → 返回 error', async () => {
      mockMotionService.DisconnectMotion.mockRejectedValue(new Error('disconnect boom'))
      const store = useMotionStore()
      const result = await store.disconnectController('mc-1')
      expect(result.success).toBe(false)
      expect(result.error).toBe('disconnect boom')
    })
  })

  // ============ applyControllerStatusToUI（内部函数，经 motion:status-updated 事件回调间接验证）============
  describe('applyControllerStatusToUI（事件回调路径）', () => {
    async function setupListening() {
      mockMotionService.GetMotionProfiles.mockResolvedValue([])
      mockMotionService.GetMotionStatusAll.mockResolvedValue([])
      const store = useMotionStore()
      store.startListening()
      // 等待 fetchX 微任务完成
      await Promise.resolve()
      await Promise.resolve()
      const cb = mockEventsOn.mock.calls.find(c => c[0] === 'motion:status-updated')![1] as (e: any) => void
      return { store, cb }
    }

    it('只同步活动控制器 + Connected 的轴', async () => {
      const { store, cb } = await setupListening()
      store.activeControllerId = 'mc-1'
      cb({ data: [
        { id: 'mc-1', status: 'Connected', axes: [{ name: 'X', position: 5, homed: true, moving: false, posLimit: false, negLimit: false, compensating: false }] },
        { id: 'mc-2', status: 'Connected', axes: [{ name: 'Y', position: 99, homed: true, moving: false, posLimit: false, negLimit: false, compensating: false }] },
      ] })
      expect(store.axisUIStates.X.currentPosition).toBe(5)
      // mc-2 不是活动控制器，Y 不应被同步
      expect(store.axisUIStates.Y.currentPosition).toBe(0)
    })

    it('活动控制器非 Connected → 不同步', async () => {
      const { store, cb } = await setupListening()
      store.activeControllerId = 'mc-1'
      cb({ data: [
        { id: 'mc-1', status: 'Disconnected', axes: [{ name: 'X', position: 5, homed: true, moving: false, posLimit: false, negLimit: false, compensating: false }] },
      ] })
      expect(store.axisUIStates.X.currentPosition).toBe(0)
    })

    it('moving=true 且 idle → runState 切换为 running', async () => {
      const { store, cb } = await setupListening()
      store.activeControllerId = 'mc-1'
      cb({ data: [
        { id: 'mc-1', status: 'Connected', axes: [{ name: 'X', position: 0, homed: false, moving: true, posLimit: false, negLimit: false, compensating: false }] },
      ] })
      expect(store.axisUIStates.X.runState).toBe('running')
    })

    it('moving=false 且 runState=running → 回到 idle', async () => {
      const { store, cb } = await setupListening()
      store.activeControllerId = 'mc-1'
      store.axisUIStates.X.runState = 'running'
      cb({ data: [
        { id: 'mc-1', status: 'Connected', axes: [{ name: 'X', position: 0, homed: false, moving: false, posLimit: false, negLimit: false, compensating: false }] },
      ] })
      expect(store.axisUIStates.X.runState).toBe('idle')
    })

    it('error 状态不被 moving=false 复位为 idle', async () => {
      const { store, cb } = await setupListening()
      store.activeControllerId = 'mc-1'
      store.axisUIStates.X.runState = 'error'
      cb({ data: [
        { id: 'mc-1', status: 'Connected', axes: [{ name: 'X', position: 0, homed: false, moving: false, posLimit: false, negLimit: false, compensating: false }] },
      ] })
      expect(store.axisUIStates.X.runState).toBe('error')
    })

    it('轴名不在 axisUIStates → 跳过', async () => {
      const { store, cb } = await setupListening()
      store.activeControllerId = 'mc-1'
      expect(() => cb({ data: [
        { id: 'mc-1', status: 'Connected', axes: [{ name: 'W', position: 99, homed: false, moving: false, posLimit: false, negLimit: false, compensating: false }] },
      ] })).not.toThrow()
    })
  })

  // ============ withAxisAction 边界 ============
  describe('withAxisAction 边界（经 moveTo 等暴露）', () => {
    it('无 activeControllerId → 返回 控制器未连接', async () => {
      const store = useMotionStore()
      const r = await store.moveTo('X', 10)
      expect(r.success).toBe(false)
      expect(r.error).toBe('控制器未连接')
    })

    it('activeControllerId 存在但未知轴 → 未知轴', async () => {
      const store = useMotionStore()
      store.activeControllerId = 'mc-1'
      const r = await store.moveTo('W', 10)
      expect(r.success).toBe(false)
      expect(r.error).toBe('未知轴')
    })

    it('requireIdle 路径：startJog 在非 idle → 拒绝', async () => {
      const store = useMotionStore()
      store.activeControllerId = 'mc-1'
      store.axisUIStates.X.runState = 'running'
      const r = await store.startJog('X', 'plus')
      expect(r.success).toBe(false)
      expect(r.error).toBe('轴当前不在空闲状态')
    })
  })

  // ============ 轴运动 action ============
  describe('轴运动 action（activeControllerId=mc-1）', () => {
    beforeEach(() => {
      // 默认大多数 action 成功
      Object.values(mockMotionService).forEach(fn => {
        if ('mockResolvedValue' in fn) (fn as any).mockResolvedValue(undefined)
      })
    })

    describe('moveTo', () => {
      it('runState=idle → 调用 MotionMoveTo 并切到 running', async () => {
        mockMotionService.MotionIsAxisMoving.mockResolvedValue(false)
        mockMotionService.MotionMoveTo.mockResolvedValue(undefined)
        const store = useMotionStore()
        store.activeControllerId = 'mc-1'
        const r = await store.moveTo('X', 25)
        expect(r.success).toBe(true)
        expect(mockMotionService.MotionMoveTo).toHaveBeenCalledWith('mc-1', 'X', 25)
        expect(store.axisUIStates.X.runState).toBe('running')
      })

      it('runState=running 且轴仍在动 → 拒绝', async () => {
        mockMotionService.MotionIsAxisMoving.mockResolvedValue(true)
        const store = useMotionStore()
        store.activeControllerId = 'mc-1'
        store.axisUIStates.X.runState = 'running'
        const r = await store.moveTo('X', 25)
        expect(r.success).toBe(false)
        expect(r.error).toBe('轴当前不在空闲状态')
      })

      it('runState=running 但轴已停 → 复位 idle 并继续', async () => {
        mockMotionService.MotionIsAxisMoving.mockResolvedValue(false)
        mockMotionService.MotionMoveTo.mockResolvedValue(undefined)
        const store = useMotionStore()
        store.activeControllerId = 'mc-1'
        store.axisUIStates.X.runState = 'running'
        const r = await store.moveTo('X', 25)
        expect(r.success).toBe(true)
        expect(store.axisUIStates.X.runState).toBe('running')
      })

      it('MotionMoveTo 抛错 → 返回 error', async () => {
        mockMotionService.MotionIsAxisMoving.mockResolvedValue(false)
        mockMotionService.MotionMoveTo.mockRejectedValue(new Error('move boom'))
        const store = useMotionStore()
        store.activeControllerId = 'mc-1'
        const r = await store.moveTo('X', 25)
        expect(r.success).toBe(false)
        expect(r.error).toBe('move boom')
      })
    })

    it('moveBy → 调用 MotionMoveBy', async () => {
      mockMotionService.MotionMoveBy.mockResolvedValue(undefined)
      const store = useMotionStore()
      store.activeControllerId = 'mc-1'
      const r = await store.moveBy('Y', 5)
      expect(r.success).toBe(true)
      expect(mockMotionService.MotionMoveBy).toHaveBeenCalledWith('mc-1', 'Y', 5)
    })

    describe('startJog', () => {
      it('plus 方向 → jogging_plus + 调用 MotionJog(dir=1)', async () => {
        mockMotionService.MotionJog.mockResolvedValue(undefined)
        const store = useMotionStore()
        store.activeControllerId = 'mc-1'
        const r = await store.startJog('X', 'plus')
        expect(r.success).toBe(true)
        expect(mockMotionService.MotionJog).toHaveBeenCalledWith('mc-1', 'X', 1, 10, 50)
        expect(store.axisUIStates.X.runState).toBe('jogging_plus')
      })

      it('minus 方向 → jogging_minus + MotionJog(dir=-1)', async () => {
        mockMotionService.MotionJog.mockResolvedValue(undefined)
        const store = useMotionStore()
        store.activeControllerId = 'mc-1'
        const r = await store.startJog('U', 'minus')
        expect(r.success).toBe(true)
        expect(mockMotionService.MotionJog).toHaveBeenCalledWith('mc-1', 'U', -1, 5, 30)
        expect(store.axisUIStates.U.runState).toBe('jogging_minus')
      })
    })

    describe('stopJog', () => {
      it('非 jogging 状态 → 直接 success（不调用 Stop）', async () => {
        const store = useMotionStore()
        store.activeControllerId = 'mc-1'
        const r = await store.stopJog('X')
        expect(r.success).toBe(true)
        expect(mockMotionService.MotionStop).not.toHaveBeenCalled()
      })

      it('jogging_plus → 走 stopAxis', async () => {
        mockMotionService.MotionStop.mockResolvedValue(undefined)
        const store = useMotionStore()
        store.activeControllerId = 'mc-1'
        store.axisUIStates.X.runState = 'jogging_plus'
        const r = await store.stopJog('X')
        expect(r.success).toBe(true)
        expect(mockMotionService.MotionStop).toHaveBeenCalledWith('mc-1', 'X')
        expect(store.axisUIStates.X.runState).toBe('idle')
      })

      it('未知轴 → 未知轴', async () => {
        const store = useMotionStore()
        const r = await store.stopJog('W')
        expect(r.success).toBe(false)
        expect(r.error).toBe('未知轴')
      })
    })

    it('stopAxis → 调用 MotionStop + 复位 idle', async () => {
      mockMotionService.MotionStop.mockResolvedValue(undefined)
      const store = useMotionStore()
      store.activeControllerId = 'mc-1'
      const r = await store.stopAxis('X')
      expect(r.success).toBe(true)
      expect(store.axisUIStates.X.runState).toBe('idle')
    })

    describe('stopAllAxes', () => {
      it('无 activeControllerId → 直接 success', async () => {
        const store = useMotionStore()
        const r = await store.stopAllAxes()
        expect(r.success).toBe(true)
        expect(mockMotionService.MotionStopAll).not.toHaveBeenCalled()
      })

      it('有 activeControllerId → 调用 MotionStopAll + 全部 runState=idle', async () => {
        mockMotionService.MotionStopAll.mockResolvedValue(undefined)
        const store = useMotionStore()
        store.activeControllerId = 'mc-1'
        store.axisUIStates.X.runState = 'running'
        store.axisUIStates.U.runState = 'jogging_minus'
        const r = await store.stopAllAxes()
        expect(r.success).toBe(true)
        expect(mockMotionService.MotionStopAll).toHaveBeenCalledWith('mc-1')
        expect(store.axisUIStates.X.runState).toBe('idle')
        expect(store.axisUIStates.U.runState).toBe('idle')
      })

      it('MotionStopAll 抛错 → 返回 error，runState 不复位', async () => {
        mockMotionService.MotionStopAll.mockRejectedValue(new Error('stopall boom'))
        const store = useMotionStore()
        store.activeControllerId = 'mc-1'
        store.axisUIStates.X.runState = 'running'
        const r = await store.stopAllAxes()
        expect(r.success).toBe(false)
        expect(r.error).toBe('stopall boom')
        expect(store.axisUIStates.X.runState).toBe('running')
      })
    })

    it('home → 调用 MotionHome', async () => {
      mockMotionService.MotionHome.mockResolvedValue(undefined)
      const store = useMotionStore()
      store.activeControllerId = 'mc-1'
      const r = await store.home('Z')
      expect(r.success).toBe(true)
      expect(mockMotionService.MotionHome).toHaveBeenCalledWith('mc-1', 'Z')
    })

    describe('definePosition', () => {
      it('position=0 → isHomed=true', async () => {
        mockMotionService.MotionDefinePosition.mockResolvedValue(undefined)
        const store = useMotionStore()
        store.activeControllerId = 'mc-1'
        const r = await store.definePosition('X', 0)
        expect(r.success).toBe(true)
        expect(store.axisUIStates.X.currentPosition).toBe(0)
        expect(store.axisUIStates.X.isHomed).toBe(true)
        expect(store.axisUIStates.X.targetPosition).toBe(0)
      })

      it('position=10 → isHomed=false', async () => {
        mockMotionService.MotionDefinePosition.mockResolvedValue(undefined)
        const store = useMotionStore()
        store.activeControllerId = 'mc-1'
        const r = await store.definePosition('X', 10)
        expect(r.success).toBe(true)
        expect(store.axisUIStates.X.currentPosition).toBe(10)
        expect(store.axisUIStates.X.isHomed).toBe(false)
        expect(store.axisUIStates.X.targetPosition).toBe(10)
      })
    })

    describe('emergencyStop', () => {
      it('无 activeControllerId → 直接 success', async () => {
        const store = useMotionStore()
        const r = await store.emergencyStop()
        expect(r.success).toBe(true)
        expect(mockMotionService.MotionEmergencyStop).not.toHaveBeenCalled()
      })

      it('有 activeControllerId → 调用 MotionEmergencyStop + 全部复位', async () => {
        mockMotionService.MotionEmergencyStop.mockResolvedValue(undefined)
        const store = useMotionStore()
        store.activeControllerId = 'mc-1'
        store.axisUIStates.X.runState = 'running'
        const r = await store.emergencyStop()
        expect(r.success).toBe(true)
        expect(mockMotionService.MotionEmergencyStop).toHaveBeenCalledWith('mc-1')
        expect(store.axisUIStates.X.runState).toBe('idle')
      })
    })
  })

  // ============ 轴配置 ============
  describe('轴配置', () => {
    it('updateAxisKind LINEAR → ROTARY：kind/relativeDistance/maxSpeed/lead/gearRatio 同步切换', () => {
      const store = useMotionStore()
      store.updateAxisKind('X', 'ROTARY')
      expect(store.axisUIStates.X.kind).toBe('ROTARY')
      expect(store.axisUIStates.X.config.kind).toBe('ROTARY')
      expect(store.axisUIStates.X.relativeDistance).toBe(5)
      expect(store.axisUIStates.X.config.maxSpeed).toBe(30)
      expect(store.axisUIStates.X.config.lead).toBe(4)
      expect(store.axisUIStates.X.config.gearRatio).toBe(4)
    })

    it('updateAxisKind 同 kind → 无操作', () => {
      const store = useMotionStore()
      const before = JSON.parse(JSON.stringify(store.axisUIStates.X))
      store.updateAxisKind('X', 'LINEAR')
      expect(store.axisUIStates.X).toEqual(before)
    })

    it('updateAxisKind 未知轴 → 无操作', () => {
      const store = useMotionStore()
      store.updateAxisKind('W', 'ROTARY')
      // 不抛错即可
      expect(store.axisUIStates.X.kind).toBe('LINEAR')
    })

    it('updateAxisConfig → Object.assign + saveConfigToLocal + 持久化到活动控制器', async () => {
      mockMotionService.UpdateMotionProfile.mockResolvedValue(undefined)
      mockMotionService.GetMotionProfiles.mockResolvedValue([])
      const store = useMotionStore()
      store.profiles = [{
        id: 'mc-1', name: 'C1', type: 'b140', address: 'a', port: 1, timeoutMs: 5,
        axes: [{ name: 'X', kind: 'LINEAR', enabled: true, inverted: false, stepAngleDeg: 1.8, microSteps: 16, lead: 5, gearRatio: 1, maxSpeed: 50, encoderScale: 0.005, encoderCompensation: {} } as any]
      }]
      store.activeControllerId = 'mc-1'
      await store.updateAxisConfig('X', { maxSpeed: 100 })
      expect(store.axisUIStates.X.config.maxSpeed).toBe(100)
      expect(mockMotionService.UpdateMotionProfile).toHaveBeenCalledTimes(1)
    })

    it('updateAxisConfig 未知轴 → 无操作', async () => {
      const store = useMotionStore()
      await store.updateAxisConfig('W', { maxSpeed: 100 })
      expect(mockMotionService.UpdateMotionProfile).not.toHaveBeenCalled()
    })

    it('updateAxesConfig 多轴更新 → 一次 UpdateMotionProfile 提交，未修改轴保持 profile 原值', async () => {
      mockMotionService.UpdateMotionProfile.mockResolvedValue(undefined)
      mockMotionService.GetMotionProfiles.mockResolvedValue([])
      const store = useMotionStore()
      const mkAxis = (name: string, microSteps: number) => ({ name, kind: 'LINEAR', enabled: true, inverted: false, stepAngleDeg: 1.8, microSteps, lead: 5, gearRatio: 1, maxSpeed: 50, encoderScale: 0.005, encoderCompensation: {} } as any)
      store.profiles = [{
        id: 'mc-1', name: 'C1', type: 'b140', address: 'a', port: 1, timeoutMs: 5,
        axes: [mkAxis('X', 16), mkAxis('Y', 16), mkAxis('Z', 16)]
      }]
      store.activeControllerId = 'mc-1'
      await store.updateAxesConfig({ X: { microSteps: 32 }, Y: { microSteps: 64 } })
      expect(mockMotionService.UpdateMotionProfile).toHaveBeenCalledTimes(1)
      const arg = mockMotionService.UpdateMotionProfile.mock.calls[0][0]
      expect(arg.id).toBe('mc-1')
      expect(arg.axes.find((a: any) => a.name === 'X').microSteps).toBe(32)
      expect(arg.axes.find((a: any) => a.name === 'Y').microSteps).toBe(64)
      // 未修改的轴以 profile 原值为基准，不被全局 UI 状态覆盖
      expect(arg.axes.find((a: any) => a.name === 'Z').microSteps).toBe(16)
    })

    it('updateAxesConfig 以 profile 原值为基准合并，axisUIStates 过期值不污染 profile', async () => {
      mockMotionService.UpdateMotionProfile.mockResolvedValue(undefined)
      mockMotionService.GetMotionProfiles.mockResolvedValue([])
      const store = useMotionStore()
      const mkAxis = (name: string, microSteps: number) => ({ name, kind: 'LINEAR', enabled: true, inverted: false, stepAngleDeg: 1.8, microSteps, lead: 5, gearRatio: 1, maxSpeed: 50, encoderScale: 0.005, encoderCompensation: {} } as any)
      store.profiles = [{
        id: 'mc-1', name: 'C1', type: 'b140', address: 'a', port: 1, timeoutMs: 5,
        axes: [mkAxis('X', 16)]
      }]
      store.activeControllerId = 'mc-1'
      // 模拟 axisUIStates 残留其他控制器的值（与 profile 不一致）
      store.axisUIStates.X.config.microSteps = 99
      store.axisUIStates.X.config.maxSpeed = 77
      await store.updateAxesConfig({ X: { microSteps: 32 } })
      const arg = mockMotionService.UpdateMotionProfile.mock.calls[0][0]
      const x = arg.axes.find((a: any) => a.name === 'X')
      expect(x.microSteps).toBe(32)   // 显式修改的字段生效
      expect(x.maxSpeed).toBe(50)     // 未修改字段保持 profile 原值，而非过期 UI 值 77
    })

    it('updateAxesConfig kind 变更 → 轴 UI kind/relativeDistance 同步切换', async () => {
      mockMotionService.UpdateMotionProfile.mockResolvedValue(undefined)
      mockMotionService.GetMotionProfiles.mockResolvedValue([])
      const store = useMotionStore()
      store.profiles = [{
        id: 'mc-1', name: 'C1', type: 'b140', address: 'a', port: 1, timeoutMs: 5,
        axes: [{ name: 'X', kind: 'LINEAR', enabled: true, inverted: false, stepAngleDeg: 1.8, microSteps: 16, lead: 5, gearRatio: 1, maxSpeed: 50, encoderScale: 0.005, encoderCompensation: {} } as any]
      }]
      store.activeControllerId = 'mc-1'
      await store.updateAxesConfig({ X: { kind: 'ROTARY', gearRatio: 10, maxSpeed: 30 } })
      expect(store.axisUIStates.X.kind).toBe('ROTARY')
      expect(store.axisUIStates.X.config.kind).toBe('ROTARY')
      expect(store.axisUIStates.X.relativeDistance).toBe(5)
      const arg = mockMotionService.UpdateMotionProfile.mock.calls[0][0]
      expect(arg.axes.find((a: any) => a.name === 'X').kind).toBe('ROTARY')
    })

    it('updateAxisTarget → 写入 targetPosition', () => {
      const store = useMotionStore()
      store.updateAxisTarget('X', 42)
      expect(store.axisUIStates.X.targetPosition).toBe(42)
    })

    it('updateAxisTarget 未知轴 → 无操作', () => {
      const store = useMotionStore()
      store.updateAxisTarget('W', 42)
      // 不抛错即可
    })

    it('updateAxisRelativeDistance → 写入 + saveConfigToLocal', () => {
      const store = useMotionStore()
      store.updateAxisRelativeDistance('X', 25)
      expect(store.axisUIStates.X.relativeDistance).toBe(25)
      expect(localStorage.getItem('motionControllerConfig:default')).toBeTruthy()
    })
  })

  // ============ 控制器 CRUD ============
  describe('addController', () => {
    it('成功 → 调用 AddMotionProfile + fetchProfiles + 写日志', async () => {
      mockMotionService.AddMotionProfile.mockResolvedValue(undefined)
      mockMotionService.GetMotionProfiles.mockResolvedValue([{ id: 'mc-1' }])
      const store = useMotionStore()
      const r = await store.addController({ name: 'C1', type: 'b140', address: '127.0.0.1', port: 5000 })
      expect(r.success).toBe(true)
      expect(mockMotionService.AddMotionProfile).toHaveBeenCalledTimes(1)
      // 第一个参数是 createFrom 透传后的 profile 对象
      const profileArg = mockMotionService.AddMotionProfile.mock.calls[0][0]
      expect(profileArg.name).toBe('C1')
      expect(profileArg.axes).toHaveLength(4)
    })

    it('失败 → 返回 error', async () => {
      mockMotionService.AddMotionProfile.mockRejectedValue(new Error('add boom'))
      const store = useMotionStore()
      const r = await store.addController({ name: 'C1', type: 'b140', address: 'a', port: 1 })
      expect(r.success).toBe(false)
      expect(r.error).toBe('add boom')
    })
  })

  describe('removeController', () => {
    it('删除非活动控制器 → active 不变 + connectionStatus 不变', async () => {
      mockMotionService.RemoveMotionProfile.mockResolvedValue(undefined)
      mockMotionService.GetMotionProfiles.mockResolvedValue([])
      // 活动控制器仍 Connected → syncConnectionFromStatuses 保持 connected
      mockMotionService.GetMotionStatusAll.mockResolvedValue([
        { id: 'mc-1', status: 'Connected', axes: [] },
      ])
      const store = useMotionStore()
      store.activeControllerId = 'mc-1'
      store.connectionStatus = 'connected'
      const r = await store.removeController('mc-2')
      expect(r.success).toBe(true)
      expect(store.activeControllerId).toBe('mc-1')
      expect(store.connectionStatus).toBe('connected') // 未变动
    })

    it('删除活动控制器 → 清除 active + connectionStatus=disconnected', async () => {
      mockMotionService.RemoveMotionProfile.mockResolvedValue(undefined)
      mockMotionService.GetMotionProfiles.mockResolvedValue([])
      mockMotionService.GetMotionStatusAll.mockResolvedValue([])
      const store = useMotionStore()
      store.activeControllerId = 'mc-1'
      store.connectionStatus = 'connected'
      const r = await store.removeController('mc-1')
      expect(r.success).toBe(true)
      expect(store.activeControllerId).toBeNull()
      expect(store.connectionStatus).toBe('disconnected')
    })

    it('删除正在 connecting 的控制器 → 从 connectingIds 清理', async () => {
      mockMotionService.RemoveMotionProfile.mockResolvedValue(undefined)
      mockMotionService.GetMotionProfiles.mockResolvedValue([])
      mockMotionService.GetMotionStatusAll.mockResolvedValue([])
      const store = useMotionStore()
      store.connectingIds = new Set(['mc-1'])
      await store.removeController('mc-1')
      expect(store.connectingIds.has('mc-1')).toBe(false)
    })

    it('RemoveMotionProfile 抛错 → 返回 error', async () => {
      mockMotionService.RemoveMotionProfile.mockRejectedValue(new Error('rm boom'))
      const store = useMotionStore()
      const r = await store.removeController('mc-1')
      expect(r.success).toBe(false)
      expect(r.error).toBe('rm boom')
    })
  })

  describe('updateControllerProfile', () => {
    it('成功 → 调用 UpdateMotionProfile + fetchProfiles + fetchStatuses', async () => {
      mockMotionService.UpdateMotionProfile.mockResolvedValue(undefined)
      mockMotionService.GetMotionProfiles.mockResolvedValue([])
      mockMotionService.GetMotionStatusAll.mockResolvedValue([])
      const store = useMotionStore()
      const r = await store.updateControllerProfile({ id: 'mc-1', name: 'C1', type: 'b140', address: 'a', port: 1, timeoutMs: 5, axes: [] } as any)
      expect(r.success).toBe(true)
      expect(mockMotionService.UpdateMotionProfile).toHaveBeenCalledTimes(1)
      expect(mockMotionService.GetMotionProfiles).toHaveBeenCalledTimes(1)
      expect(mockMotionService.GetMotionStatusAll).toHaveBeenCalledTimes(1)
    })

    it('失败 → 返回 error', async () => {
      mockMotionService.UpdateMotionProfile.mockRejectedValue(new Error('upd boom'))
      const store = useMotionStore()
      const r = await store.updateControllerProfile({ id: 'mc-1' } as any)
      expect(r.success).toBe(false)
      expect(r.error).toBe('upd boom')
    })
  })

  // ============ 事件监听 ============
  describe('startListening / stopListening', () => {
    it('startListening → Events.On 注册 motion:status-updated + 触发 fetchProfiles/fetchStatuses', async () => {
      mockMotionService.GetMotionProfiles.mockResolvedValue([])
      mockMotionService.GetMotionStatusAll.mockResolvedValue([])
      const store = useMotionStore()
      store.startListening()
      // createWailsEventListener 用 Events.On 注册
      expect(mockEventsOn).toHaveBeenCalledWith('motion:status-updated', expect.any(Function))
      // 异步 fetchX 走微任务
      await Promise.resolve()
      await Promise.resolve()
      expect(mockMotionService.GetMotionProfiles).toHaveBeenCalledTimes(1)
      expect(mockMotionService.GetMotionStatusAll).toHaveBeenCalledTimes(1)
    })

    it('startListening 同时调 loadConfigFromLocal（无数据时无副作用）', () => {
      const store = useMotionStore()
      store.startListening()
      // 验证不抛错即可（loadConfigFromLocal 已在配置持久化 describe 中测过）
      expect(store.axisUIStates.X.relativeDistance).toBe(10)
    })

    it('stopListening → Events.Off 注销 motion:status-updated', () => {
      const store = useMotionStore()
      store.startListening()
      store.stopListening()
      expect(mockEventsOff).toHaveBeenCalledWith('motion:status-updated')
    })

    it('stopListening 未先 start → 无操作（listening 标志位保护）', () => {
      const store = useMotionStore()
      store.stopListening()
      expect(mockEventsOff).not.toHaveBeenCalled()
    })

    it('motion:status-updated 事件回调：写入 statuses + 同步 connectionStatus', async () => {
      mockMotionService.GetMotionProfiles.mockResolvedValue([])
      mockMotionService.GetMotionStatusAll.mockResolvedValue([])
      const store = useMotionStore()
      store.startListening()
      store.activeControllerId = 'mc-1'
      // 取出 On 注册的回调
      const cb = mockEventsOn.mock.calls.find(c => c[0] === 'motion:status-updated')![1] as (e: any) => void
      cb({ data: [{ id: 'mc-1', status: 'Connected', axes: [] }] })
      expect(store.statuses).toHaveLength(1)
      expect(store.connectionStatus).toBe('connected')
    })

    it('事件回调：Connected 控制器从 connectingIds 兜底清理', async () => {
      mockMotionService.GetMotionProfiles.mockResolvedValue([])
      mockMotionService.GetMotionStatusAll.mockResolvedValue([])
      const store = useMotionStore()
      store.connectingIds = new Set(['mc-1'])
      store.startListening()
      const cb = mockEventsOn.mock.calls.find(c => c[0] === 'motion:status-updated')![1] as (e: any) => void
      cb({ data: [{ id: 'mc-1', status: 'Connected', axes: [] }] })
      expect(store.connectingIds.has('mc-1')).toBe(false)
    })

    it('事件回调：Error 状态也清理 connectingIds', async () => {
      mockMotionService.GetMotionProfiles.mockResolvedValue([])
      mockMotionService.GetMotionStatusAll.mockResolvedValue([])
      const store = useMotionStore()
      store.connectingIds = new Set(['mc-1'])
      store.startListening()
      const cb = mockEventsOn.mock.calls.find(c => c[0] === 'motion:status-updated')![1] as (e: any) => void
      cb({ data: [{ id: 'mc-1', status: 'Error', axes: [] }] })
      expect(store.connectingIds.has('mc-1')).toBe(false)
    })

    it('事件回调：Disconnected 状态也清理 connectingIds', async () => {
      mockMotionService.GetMotionProfiles.mockResolvedValue([])
      mockMotionService.GetMotionStatusAll.mockResolvedValue([])
      const store = useMotionStore()
      store.connectingIds = new Set(['mc-1'])
      store.startListening()
      const cb = mockEventsOn.mock.calls.find(c => c[0] === 'motion:status-updated')![1] as (e: any) => void
      cb({ data: [{ id: 'mc-1', status: 'Disconnected', axes: [] }] })
      expect(store.connectingIds.has('mc-1')).toBe(false)
    })

    it('事件回调：Connecting 状态不清理 connectingIds', async () => {
      mockMotionService.GetMotionProfiles.mockResolvedValue([])
      mockMotionService.GetMotionStatusAll.mockResolvedValue([])
      const store = useMotionStore()
      store.connectingIds = new Set(['mc-1'])
      store.startListening()
      const cb = mockEventsOn.mock.calls.find(c => c[0] === 'motion:status-updated')![1] as (e: any) => void
      cb({ data: [{ id: 'mc-1', status: 'Connecting', axes: [] }] })
      expect(store.connectingIds.has('mc-1')).toBe(true)
    })
  })

  // ============ startListening 800ms 重试（fake timers）============
  describe('startListening 800ms 重试', () => {
    beforeEach(() => vi.useFakeTimers())
    afterEach(() => vi.useRealTimers())

    it('首次 fetchProfiles 返回空 → 800ms 后重试一次', async () => {
      // 首次空、第二次有数据
      mockMotionService.GetMotionProfiles
        .mockResolvedValueOnce([])
        .mockResolvedValueOnce([{ id: 'mc-1' }])
      mockMotionService.GetMotionStatusAll.mockResolvedValue([])
      const store = useMotionStore()
      store.startListening()
      // 推进微任务让首次 fetchX 完成
      await vi.advanceTimersByTimeAsync(0)
      expect(mockMotionService.GetMotionProfiles).toHaveBeenCalledTimes(1)
      // 推进到 800ms 触发重试
      await vi.advanceTimersByTimeAsync(800)
      expect(mockMotionService.GetMotionProfiles).toHaveBeenCalledTimes(2)
    })

    it('首次 fetchStatuses 返回空 → 800ms 后重试一次', async () => {
      mockMotionService.GetMotionProfiles.mockResolvedValue([{ id: 'mc-1' }])
      mockMotionService.GetMotionStatusAll
        .mockResolvedValueOnce([])
        .mockResolvedValueOnce([{ id: 'mc-1', status: 'Connected', axes: [] }])
      const store = useMotionStore()
      store.startListening()
      await vi.advanceTimersByTimeAsync(0)
      expect(mockMotionService.GetMotionStatusAll).toHaveBeenCalledTimes(1)
      await vi.advanceTimersByTimeAsync(800)
      expect(mockMotionService.GetMotionStatusAll).toHaveBeenCalledTimes(2)
    })

    it('首次已有数据 → 800ms 后不重试', async () => {
      mockMotionService.GetMotionProfiles.mockResolvedValue([{ id: 'mc-1' }])
      mockMotionService.GetMotionStatusAll.mockResolvedValue([{ id: 'mc-1', status: 'Connected', axes: [] }])
      const store = useMotionStore()
      store.startListening()
      await vi.advanceTimersByTimeAsync(0)
      await vi.advanceTimersByTimeAsync(800)
      expect(mockMotionService.GetMotionProfiles).toHaveBeenCalledTimes(1)
      expect(mockMotionService.GetMotionStatusAll).toHaveBeenCalledTimes(1)
    })
  })
})
