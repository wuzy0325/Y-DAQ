import { ref, computed } from 'vue'
import { defineStore } from 'pinia'
import {
  GetMotionProfiles, GetMotionStatusAll, ConnectMotion, DisconnectMotion,
  MotionMoveTo, MotionMoveBy, MotionJog, MotionStop, MotionEmergencyStop,
  MotionHome, MotionDefinePosition, MotionIsAxisMoving, MotionMotorOff,
  AddMotionProfile, UpdateMotionProfile, RemoveMotionProfile, MotionStopAll,
} from '../wails-compat/app'
import { types } from '../wails-compat/models'
import { EventsOn } from '../../wailsjs/runtime/runtime'

  // 轴类型
type AxisKind = 'LINEAR' | 'ROTARY'
type AxisName = 'X' | 'Y' | 'Z' | 'U'

// 轴运行状态
type AxisRunState = 'idle' | 'running' | 'jogging_minus' | 'jogging_plus' | 'error'

interface AxisStatus {
  name: string
  position: number
  moving: boolean
  homed: boolean
  posLimit: boolean
  negLimit: boolean
  compensating: boolean
}

interface MotionControllerStatus {
  id: string
  name: string
  type: string
  status: string
  axes: AxisStatus[]
  lastError: string
}

interface AxisConfig {
  name: string
  enabled: boolean
  kind: AxisKind
  inverted: boolean
  stepAngleDeg: number
  microSteps: number
    lead: number
    gearRatio: number
    maxSpeed: number
  encoderScale: number
  encoderCompensation: {
    enabled: boolean
    tolerance: number
    maxCycles: number
    settleMs: number
    minStep: number
    timeoutMs: number
  }
}

interface MotionControllerProfile {
  id: string
  name: string
  type: string
  address: string
  port: number
  timeoutMs: number
  axes: AxisConfig[]
}

// 轴扩展状态（前端本地维护）
interface AxisUIState {
  name: string
  kind: AxisKind
  currentPosition: number
  targetPosition: number
  relativeDistance: number
  runState: AxisRunState
  isHomed: boolean
  posLimitActive: boolean
  negLimitActive: boolean
  config: AxisConfig
}

// 轴单位
function getAxisUnit(kind: AxisKind): string {
  return kind === 'LINEAR' ? 'mm' : '°'
}

// 轴类型中文
function getAxisKindText(kind: AxisKind): string {
  return kind === 'LINEAR' ? '平移轴' : '旋转轴'
}

// 运行状态中文
function getRunStateText(state: AxisRunState): string {
  const map: Record<AxisRunState, string> = {
    idle: '空闲',
    running: '运行中',
    jogging_minus: '反向点动',
    jogging_plus: '正向点动',
    error: '错误'
  }
  return map[state]
}

// 创建默认轴配置
function createDefaultAxisConfig(name: string, kind: AxisKind): AxisConfig {
  return {
    name,
    enabled: true,
    kind,
    inverted: false,
    stepAngleDeg: 1.8,
    microSteps: 16,
    lead: kind === 'LINEAR' ? 5.0 : 4,
    gearRatio: 1,
    maxSpeed: kind === 'LINEAR' ? 50 : 30,
    encoderScale: 0.005,
    encoderCompensation: {
      enabled: false,
      tolerance: 0.01,
      maxCycles: 3,
      settleMs: 100,
      minStep: 0,
      timeoutMs: 5000
    }
  }
}

// 创建默认轴UI状态
function createDefaultAxisUIState(name: string, kind: AxisKind): AxisUIState {
  return {
    name,
    kind,
    currentPosition: 0,
    targetPosition: 0,
    relativeDistance: kind === 'LINEAR' ? 10 : 5,
    runState: 'idle',
    isHomed: false,
    posLimitActive: false,
    negLimitActive: false,
    config: createDefaultAxisConfig(name, kind)
  }
}

// 本地存储key前缀
const MOTION_CONFIG_STORAGE_PREFIX = 'motionControllerConfig:'

export const useMotionStore = defineStore('motion', () => {
  // 基础状态
  const profiles = ref<MotionControllerProfile[]>([])
  const statuses = ref<MotionControllerStatus[]>([])

  // 当前选中的控制器
  const activeControllerId = ref<string | null>(null)
  const selectedAxis = ref<string>('X')

  // 轴UI状态（本地维护，不来自后端轮询）
  const axisUIStates = ref<Record<string, AxisUIState>>({
    X: createDefaultAxisUIState('X', 'LINEAR'),
    Y: createDefaultAxisUIState('Y', 'LINEAR'),
    Z: createDefaultAxisUIState('Z', 'LINEAR'),
    U: createDefaultAxisUIState('U', 'ROTARY')
  })

  // 运行日志
  const logs = ref<string[]>([])

  // 连接状态
  const connectionStatus = ref<'disconnected' | 'connecting' | 'connected' | 'error'>('disconnected')

  // 正在连接中的控制器ID集合（用于按钮 loading 状态）
  const connectingIds = ref<Set<string>>(new Set())
  // 响应式快照，确保模板能追踪 connectingIds 变化
  const connectingIdSet = computed(() => connectingIds.value)

  // 计算属性
  const isConnected = computed(() => connectionStatus.value === 'connected')

  const currentAxis = computed(() => axisUIStates.value[selectedAxis.value])

  const allAxes = computed(() => [
    axisUIStates.value.X,
    axisUIStates.value.Y,
    axisUIStates.value.Z,
    axisUIStates.value.U
  ])

  const isAnyAxisRunning = computed(() =>
    Object.values(axisUIStates.value).some(
      axis => axis.runState !== 'idle' && axis.runState !== 'error'
    )
  )

  // 日志
  function addLog(message: string) {
    const timestamp = new Date().toLocaleTimeString()
    logs.value.unshift(`[${timestamp}] ${message}`)
    if (logs.value.length > 100) {
      logs.value.pop()
    }
  }

  function clearLogs() {
    logs.value = []
  }

  // 配置持久化
  function saveConfigToLocal() {
    try {
      const data = {
        activeControllerId: activeControllerId.value,
        profiles: profiles.value.map(p => ({
          id: p.id,
          name: p.name,
          type: p.type,
          address: p.address,
          port: p.port,
          timeoutMs: p.timeoutMs,
        })),
        axes: Object.fromEntries(
          Object.entries(axisUIStates.value).map(([name, state]) => [
            name,
            {
              kind: state.kind,
              config: state.config,
              relativeDistance: state.relativeDistance
            }
          ])
        )
      }
      localStorage.setItem(MOTION_CONFIG_STORAGE_PREFIX + 'default', JSON.stringify(data))
    } catch (e) {
      console.error('保存运动控制配置失败:', e)
    }
  }

  function loadConfigFromLocal() {
    try {
      const raw = localStorage.getItem(MOTION_CONFIG_STORAGE_PREFIX + 'default')
      if (!raw) return
      const data = JSON.parse(raw)
      if (data.axes) {
        for (const [name, saved] of Object.entries(data.axes) as [string, any][]) {
          if (axisUIStates.value[name]) {
            const state = axisUIStates.value[name]
            if (saved.kind) state.kind = saved.kind
            if (saved.config) Object.assign(state.config, saved.config)
            if (saved.relativeDistance !== undefined) state.relativeDistance = saved.relativeDistance
          }
        }
      }
    } catch (e) {
      console.error('加载运动控制配置失败:', e)
    }
  }

  // 基础API
  async function fetchProfiles() {
    try {
      profiles.value = await GetMotionProfiles() as MotionControllerProfile[]
    } catch (e) {
      console.warn('fetchMotionProfiles failed:', e)
    }
  }

  async function fetchStatuses() {
    try {
      statuses.value = await GetMotionStatusAll() as MotionControllerStatus[]
      syncConnectionFromStatuses(statuses.value)
    } catch (e) {
      console.warn('fetchMotionStatuses failed:', e)
    }
  }

  function syncConnectionFromStatuses(allStatuses: MotionControllerStatus[]) {
    if (activeControllerId.value) {
      const active = allStatuses.find(s => s.id === activeControllerId.value && s.status === 'Connected')
      if (active) {
        connectionStatus.value = 'connected'
        return
      }
    }
    if (connectionStatus.value !== 'connecting') {
      connectionStatus.value = 'disconnected'
    }
  }

  // 判断控制器是否正在连接中（store 状态或后端报告 Connecting）
  // 注意：访问 connectingIdSet.value 确保响应式追踪
  function isControllerConnecting(id: string): boolean {
    if (connectingIdSet.value.has(id)) return true
    const st = statuses.value.find(s => s.id === id)
    return st?.status === 'Connecting'
  }

  async function persistActiveProfileAxes() {
    const profile = profiles.value.find(p => p.id === activeControllerId.value)
    if (!profile) return
    const axes = profile.axes.map(axis => {
      const state = axisUIStates.value[axis.name]
      return state ? types.AxisConfig.createFrom({ ...axis, ...state.config, kind: state.kind }) : axis
    })
    await UpdateMotionProfile(types.MotionControllerProfile.createFrom({ ...profile, axes }))
    await fetchProfiles()
  }

  // 连接/断开
  async function connectController(id: string): Promise<{ success: boolean; error?: string }> {
    connectingIds.value = new Set([...connectingIds.value, id])
    connectionStatus.value = 'connecting'
    addLog('正在连接运动控制器...')
    try {
      await ConnectMotion(id)
      activeControllerId.value = id
      connectionStatus.value = 'connected'
      addLog('运动控制器连接成功')
      await syncPositionsFromStatus()
      return { success: true }
    } catch (e: any) {
      connectionStatus.value = 'error'
      const msg = e?.message || String(e)
      addLog(`连接失败: ${msg}`)
      return { success: false, error: msg }
    } finally {
      const newSet = new Set(connectingIds.value)
      newSet.delete(id)
      connectingIds.value = newSet
    }
  }

  async function disconnectController(id?: string): Promise<{ success: boolean; error?: string }> {
    // 目标 id：显式传入优先，否则回退到活动控制器（保留旧调用方兼容）
    const targetId = id ?? activeControllerId.value
    if (!targetId) return { success: true }
    const isActive = targetId === activeControllerId.value
    try {
      // 仅断开活动控制器时才停轴/关电机/清轴 UI 状态（避免误伤其他控制器的轴控制卡片）
      if (isActive) {
        await MotionStopAll(targetId)
        for (const state of Object.values(axisUIStates.value)) {
          state.runState = 'idle'
        }
        try {
          await MotionMotorOff(targetId)
        } catch (_) { /* ignore */ }
      }
      await DisconnectMotion(targetId)
      if (isActive) {
        connectionStatus.value = 'disconnected'
      }
      addLog(`运动控制器 ${targetId} 已断开`)
      return { success: true }
    } catch (e: any) {
      return { success: false, error: e?.message || String(e) }
    }
  }

  async function syncPositionsFromStatus() {
    try {
      const allStatuses = await GetMotionStatusAll() as MotionControllerStatus[]
      statuses.value = allStatuses
      syncConnectionFromStatuses(allStatuses)
      for (const ctrl of allStatuses) {
        if (ctrl.status === 'Connected' && ctrl.id === activeControllerId.value) {
          for (const ax of ctrl.axes) {
            const uiState = axisUIStates.value[ax.name]
            if (uiState) {
              uiState.currentPosition = ax.position
              uiState.isHomed = ax.homed
              uiState.posLimitActive = ax.posLimit
              uiState.negLimitActive = ax.negLimit
              if (ax.moving && uiState.runState === 'idle') {
                uiState.runState = 'running'
              } else if (!ax.moving && uiState.runState !== 'idle' && uiState.runState !== 'error') {
                uiState.runState = 'idle'
              }
            }
          }
        }
      }
    } catch (_) { /* ignore */ }
  }

  // 运动控制
  function isAxisName(axis: string): axis is AxisName {
    return axis === 'X' || axis === 'Y' || axis === 'Z' || axis === 'U'
  }

  function toBindingAxisName(axis: AxisName): types.AxisName {
    const map: Record<AxisName, types.AxisName> = {
      X: types.AxisName.AxisX,
      Y: types.AxisName.AxisY,
      Z: types.AxisName.AxisZ,
      U: types.AxisName.AxisU,
    }
    return map[axis]
  }

  async function moveTo(axis: string, position: number): Promise<{ success: boolean; error?: string }> {
    if (!activeControllerId.value) return { success: false, error: '控制器未连接' }
    if (!isAxisName(axis)) return { success: false, error: '未知轴' }
    const uiState = axisUIStates.value[axis]
    if (!uiState) return { success: false, error: '未知轴' }

    if (uiState.runState !== 'idle') {
      try {
          const moving = await MotionIsAxisMoving(activeControllerId.value, toBindingAxisName(axis))
        if (!moving) {
          uiState.runState = 'idle'
        } else {
          return { success: false, error: '轴当前不在空闲状态' }
        }
      } catch (_) {
        return { success: false, error: '轴当前不在空闲状态' }
      }
    }

    try {
      await MotionMoveTo(activeControllerId.value, toBindingAxisName(axis), position)
      uiState.runState = 'running'
      addLog(`${axis}轴运动到目标位置 ${position}${getAxisUnit(uiState.kind)}`)
      return { success: true }
    } catch (e: any) {
      uiState.runState = 'idle'
      const msg = e?.message || String(e)
      addLog(`${axis}轴运动失败: ${msg}`)
      return { success: false, error: msg }
    }
  }

  async function moveBy(axis: string, delta: number): Promise<{ success: boolean; error?: string }> {
    if (!activeControllerId.value) return { success: false, error: '控制器未连接' }
    if (!isAxisName(axis)) return { success: false, error: '未知轴' }
    try {
      await MotionMoveBy(activeControllerId.value, toBindingAxisName(axis), delta)
      addLog(`${axis}轴相对移动 ${delta}`)
      return { success: true }
    } catch (e: any) {
      return { success: false, error: e?.message || String(e) }
    }
  }

  async function startJog(axis: string, direction: 'minus' | 'plus'): Promise<{ success: boolean; error?: string }> {
    if (!activeControllerId.value) return { success: false, error: '控制器未连接' }
    if (!isAxisName(axis)) return { success: false, error: '未知轴' }
    const uiState = axisUIStates.value[axis]
    if (!uiState) return { success: false, error: '未知轴' }
    if (uiState.runState !== 'idle') return { success: false, error: '轴当前不在空闲状态' }

    try {
      const dir = direction === 'plus' ? 1 : -1
      await MotionJog(activeControllerId.value, toBindingAxisName(axis), dir, uiState.relativeDistance, uiState.config.maxSpeed)
      uiState.runState = direction === 'minus' ? 'jogging_minus' : 'jogging_plus'
      addLog(`${axis}轴开始${direction === 'minus' ? '反向' : '正向'}点动`)
      return { success: true }
    } catch (e: any) {
      uiState.runState = 'idle'
      return { success: false, error: e?.message || String(e) }
    }
  }

  async function stopJog(axis: string): Promise<{ success: boolean; error?: string }> {
    const uiState = axisUIStates.value[axis]
    if (!uiState) return { success: false, error: '未知轴' }
    if (uiState.runState !== 'jogging_minus' && uiState.runState !== 'jogging_plus') {
      return { success: true }
    }
    return stopAxis(axis)
  }

  async function stopAxis(axis: string): Promise<{ success: boolean; error?: string }> {
    if (!activeControllerId.value) return { success: false, error: '控制器未连接' }
    if (!isAxisName(axis)) return { success: false, error: '未知轴' }
    try {
      await MotionStop(activeControllerId.value, toBindingAxisName(axis))
      const uiState = axisUIStates.value[axis]
      if (uiState) uiState.runState = 'idle'
      addLog(`${axis}轴已停止`)
      return { success: true }
    } catch (e: any) {
      return { success: false, error: e?.message || String(e) }
    }
  }

  async function stopAllAxes(): Promise<{ success: boolean; error?: string }> {
    if (!activeControllerId.value) return { success: true }
    try {
      await MotionStopAll(activeControllerId.value)
      for (const state of Object.values(axisUIStates.value)) {
        state.runState = 'idle'
      }
      addLog('所有轴已停止')
      return { success: true }
    } catch (e: any) {
      return { success: false, error: e?.message || String(e) }
    }
  }

  async function home(axis: string): Promise<{ success: boolean; error?: string }> {
    if (!activeControllerId.value) return { success: false, error: '控制器未连接' }
    if (!isAxisName(axis)) return { success: false, error: '未知轴' }
    try {
      await MotionHome(activeControllerId.value, toBindingAxisName(axis))
      addLog(`${axis}轴回零`)
      return { success: true }
    } catch (e: any) {
      return { success: false, error: e?.message || String(e) }
    }
  }

  async function definePosition(axis: string, position: number): Promise<{ success: boolean; error?: string }> {
    if (!activeControllerId.value) return { success: false, error: '控制器未连接' }
    if (!isAxisName(axis)) return { success: false, error: '未知轴' }
    try {
      await MotionDefinePosition(activeControllerId.value, toBindingAxisName(axis), position)
      const uiState = axisUIStates.value[axis]
      if (uiState) {
        uiState.currentPosition = position
        uiState.isHomed = position === 0
        uiState.targetPosition = position
      }
      addLog(`${axis}轴置位为 ${position}`)
      return { success: true }
    } catch (e: any) {
      return { success: false, error: e?.message || String(e) }
    }
  }

  async function emergencyStop(): Promise<{ success: boolean; error?: string }> {
    if (!activeControllerId.value) return { success: true }
    try {
      await MotionEmergencyStop(activeControllerId.value)
      for (const state of Object.values(axisUIStates.value)) {
        state.runState = 'idle'
      }
      addLog('紧急停止已触发')
      return { success: true }
    } catch (e: any) {
      return { success: false, error: e?.message || String(e) }
    }
  }

  // 轴配置更新
  function updateAxisKind(axisName: string, newKind: AxisKind) {
    const state = axisUIStates.value[axisName]
    if (!state || state.kind === newKind) return
    state.kind = newKind
    state.config.kind = newKind
    state.relativeDistance = newKind === 'LINEAR' ? 10 : 5
    state.config.maxSpeed = newKind === 'LINEAR' ? 50 : 30
    state.config.lead = newKind === 'LINEAR' ? 5.0 : 4
    state.config.gearRatio = newKind === 'LINEAR' ? 1 : 4
    addLog(`${axisName}轴类型切换为${getAxisKindText(newKind)}`)
    saveConfigToLocal()
  }

  async function updateAxisConfig(axisName: string, config: Partial<AxisConfig>) {
    const state = axisUIStates.value[axisName]
    if (!state) return
    Object.assign(state.config, config)
    addLog(`${axisName}轴配置已更新`)
    saveConfigToLocal()
    await persistActiveProfileAxes()
  }

  function updateAxisTarget(axisName: string, target: number) {
    const state = axisUIStates.value[axisName]
    if (state) state.targetPosition = target
  }

  function updateAxisRelativeDistance(axisName: string, distance: number) {
    const state = axisUIStates.value[axisName]
    if (state) {
      state.relativeDistance = distance
      saveConfigToLocal()
    }
  }

  function selectAxis(axisName: string) {
    selectedAxis.value = axisName
  }

  // 添加控制器（仅创建 profile，不自动连接）
  async function addController(profile: { name: string; type: string; address: string; port: number; timeoutMs?: number }): Promise<{ success: boolean; error?: string }> {
    const id = `mc-${Date.now()}`
    try {
      const defaultAxes = [
        types.AxisConfig.createFrom({ name: 'X', enabled: true, kind: 'LINEAR', inverted: false, stepAngleDeg: 1.8, microSteps: 16, lead: 5, gearRatio: 1, maxSpeed: 50, encoderScale: 0.005, encoderCompensation: types.EncoderCompensationConfig.createFrom({ enabled: false, tolerance: 0.01, maxCycles: 3, settleMs: 100, minStep: 0, timeoutMs: 5000 }) }),
        types.AxisConfig.createFrom({ name: 'Y', enabled: true, kind: 'LINEAR', inverted: false, stepAngleDeg: 1.8, microSteps: 16, lead: 5, gearRatio: 1, maxSpeed: 50, encoderScale: 0.005, encoderCompensation: types.EncoderCompensationConfig.createFrom({ enabled: false, tolerance: 0.01, maxCycles: 3, settleMs: 100, minStep: 0, timeoutMs: 5000 }) }),
        types.AxisConfig.createFrom({ name: 'Z', enabled: true, kind: 'LINEAR', inverted: false, stepAngleDeg: 1.8, microSteps: 16, lead: 5, gearRatio: 1, maxSpeed: 50, encoderScale: 0.005, encoderCompensation: types.EncoderCompensationConfig.createFrom({ enabled: false, tolerance: 0.01, maxCycles: 3, settleMs: 100, minStep: 0, timeoutMs: 5000 }) }),
        types.AxisConfig.createFrom({ name: 'U', enabled: true, kind: 'ROTARY', inverted: false, stepAngleDeg: 1.8, microSteps: 16, lead: 0, gearRatio: 4, maxSpeed: 30, encoderScale: 0.005, encoderCompensation: types.EncoderCompensationConfig.createFrom({ enabled: false, tolerance: 0.01, maxCycles: 3, settleMs: 100, minStep: 0, timeoutMs: 5000 }) }),
      ]
      const fullProfile = types.MotionControllerProfile.createFrom({
        id,
        name: profile.name || '新控制器',
        type: profile.type,
        address: profile.address,
        port: profile.port,
        timeoutMs: profile.timeoutMs ?? 5000,
        axes: defaultAxes,
      })
      await AddMotionProfile(fullProfile)
      await fetchProfiles()
      addLog(`控制器 ${profile.name || '新控制器'} 已添加`)
      return { success: true }
    } catch (e: any) {
      return { success: false, error: e?.message || String(e) }
    }
  }

  // 删除控制器
  async function removeController(id: string): Promise<{ success: boolean; error?: string }> {
    try {
      await RemoveMotionProfile(id)
      // 若删除的是当前活动控制器，清除活动状态
      if (activeControllerId.value === id) {
        activeControllerId.value = null
        connectionStatus.value = 'disconnected'
      }
      // 清理 connectingIds 残留（删除正在连接中的控制器时避免 loading 状态遗留）
      if (connectingIds.value.has(id)) {
        const next = new Set(connectingIds.value)
        next.delete(id)
        connectingIds.value = next
      }
      await fetchProfiles()
      await fetchStatuses()
      addLog(`控制器已删除`)
      return { success: true }
    } catch (e: any) {
      return { success: false, error: e?.message || String(e) }
    }
  }

  // 更新控制器 profile
  async function updateControllerProfile(profile: MotionControllerProfile): Promise<{ success: boolean; error?: string }> {
    try {
      await UpdateMotionProfile(types.MotionControllerProfile.createFrom({ ...profile }))
      await fetchProfiles()
      // 若该控制器已连接，刷新状态以反映轴配置同步
      await fetchStatuses()
      addLog(`控制器 ${profile.name} 配置已更新`)
      return { success: true }
    } catch (e: any) {
      return { success: false, error: e?.message || String(e) }
    }
  }

  // 事件监听
  function startListening() {
    try {
      EventsOn('motion:status-updated', (data: MotionControllerStatus[]) => {
        statuses.value = data
        syncConnectionFromStatuses(data)
        // 多控制器场景下，根据后端状态变化对 connectingIds 做兜底清理：
        // 一旦某控制器进入 Connected / Error / Disconnected，都意味着连接流程结束
        let changed = false
        for (const ctrl of data) {
          if (ctrl.status === 'Connected' || ctrl.status === 'Error' || ctrl.status === 'Disconnected') {
            if (connectingIds.value.has(ctrl.id)) {
              if (!changed) {
                connectingIds.value = new Set(connectingIds.value)
              }
              connectingIds.value.delete(ctrl.id)
              changed = true
            }
          }
        }
        for (const ctrl of data) {
          if (ctrl.status === 'Connected' && ctrl.id === activeControllerId.value) {
            for (const ax of ctrl.axes) {
              const uiState = axisUIStates.value[ax.name]
              if (uiState) {
                uiState.currentPosition = ax.position
                uiState.isHomed = ax.homed
                uiState.posLimitActive = ax.posLimit
                uiState.negLimitActive = ax.negLimit
                if (ax.moving && uiState.runState === 'idle') {
                  uiState.runState = 'running'
                } else if (!ax.moving && uiState.runState !== 'idle' && uiState.runState !== 'error') {
                  uiState.runState = 'idle'
                }
              }
            }
          }
        }
      })
    } catch (e) {
      console.warn('motion startListening failed:', e)
    }
    fetchProfiles()
    fetchStatuses()
    loadConfigFromLocal()
    // 兜底：IPC 首次调用可能因 runtime 未完全就绪而失败，延迟 800ms 单次重试
    // （800ms 经验值，略大于 Wails v3 runtime 典型就绪时间）
    // 注意：statuses 即使本次仍失败，也会被 broadcastMotionStatus 事件推送恢复；
    //       profiles 无广播兜底，若本次仍失败需用户手动刷新
    setTimeout(() => {
      if (profiles.value.length === 0) fetchProfiles()
      if (statuses.value.length === 0) fetchStatuses()
    }, 800)
  }

  return {
    profiles, statuses,
    activeControllerId, selectedAxis,
    axisUIStates, connectionStatus, logs,
    connectingIds, isControllerConnecting,
    isConnected, currentAxis, allAxes, isAnyAxisRunning,
    fetchProfiles, fetchStatuses,
    connectController, disconnectController,
    moveTo, moveBy, startJog, stopJog,
    stopAxis, stopAllAxes, home, definePosition, emergencyStop,
    updateAxisKind, updateAxisConfig, updateAxisTarget, updateAxisRelativeDistance,
    selectAxis,
    addController, removeController, updateControllerProfile,
    addLog, clearLogs,
    getAxisUnit, getAxisKindText, getRunStateText,
    startListening,
  }
})
