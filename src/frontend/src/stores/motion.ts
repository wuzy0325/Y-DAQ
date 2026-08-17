import { ref, computed, watch } from 'vue'
import { defineStore } from 'pinia'
import { MotionService } from '@bindings/yx-daq/internal/app'
import * as types from '@bindings/yx-daq/internal/types'
import { createWailsEventListener } from '../utils/wailsEvents'

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
      profiles.value = await MotionService.GetMotionProfiles() as MotionControllerProfile[]
    } catch (e) {
      console.warn('fetchMotionProfiles failed:', e)
    }
  }

  async function fetchStatuses() {
    try {
      statuses.value = await MotionService.GetMotionStatusAll() as MotionControllerStatus[]
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
  function isControllerConnecting(id: string): boolean {
    if (connectingIds.value.has(id)) return true
    const st = statuses.value.find(s => s.id === id)
    return st?.status === 'Connecting'
  }

  // 将轴配置更新持久化到当前活动控制器。
  // 以 profile 原值为基准、仅合并 axisUpdates 中出现的字段：
  // 全局 axisUIStates 可能与 profile 不同步（切换控制器/异步加载时序），
  // 整体覆盖会把其他控制器的值写进当前 profile，导致控制器间配置串扰。
  async function persistActiveControllerAxes(axisUpdates: Record<string, Partial<AxisConfig>>) {
    const profile = profiles.value.find(p => p.id === activeControllerId.value)
    if (!profile) return
    const axes = profile.axes.map(axis => {
      const upd = axisUpdates[axis.name]
      return upd ? types.AxisConfig.createFrom({ ...axis, ...upd }) : axis
    })
    await MotionService.UpdateMotionProfile(types.MotionControllerProfile.createFrom({ ...profile, axes }))
    await fetchProfiles()
    // 保存后从后端回读同步 UI，保证对话框下次打开显示的是持久化后的值
    syncAxisConfigFromActiveProfile()
  }

  // 从当前活动控制器的 profile 同步轴配置到 axisUIStates
  // 切换控制器时必须调用，确保轴配置对话框显示正确的值
  function syncAxisConfigFromActiveProfile() {
    const profile = profiles.value.find(p => p.id === activeControllerId.value)
    if (!profile) return
    for (const axis of profile.axes) {
      const state = axisUIStates.value[axis.name]
      if (!state) continue
      // 深拷贝轴配置，避免共享引用；用 spread 同步全部字段，
      // 后续 AxisConfig 新增字段时此处无需同步修改（避免 Shotgun Surgery）
      state.config = {
        ...axis,
        kind: axis.kind as AxisKind,
        encoderCompensation: { ...axis.encoderCompensation },
      }
      state.kind = axis.kind as AxisKind
    }
  }

  // 监听活动控制器切换：当用户点击侧边栏切换控制器时，同步轴配置到 UI
  watch(activeControllerId, (newId) => {
    if (newId) {
      syncAxisConfigFromActiveProfile()
    }
  }, { flush: 'sync' })

  // profiles 异步加载完成后补一次同步：
  // 覆盖 activeControllerId 先设置而 profiles 尚未就绪导致上面 watch 中 sync 落空的窗口
  // flush:'sync' —— profiles 每次替换时立即用当次值同步，避免延迟回调用旧数据覆盖刚更新的 UI
  watch(profiles, () => {
    if (activeControllerId.value) {
      syncAxisConfigFromActiveProfile()
    }
  }, { flush: 'sync' })

  // 连接/断开
  async function connectController(id: string): Promise<{ success: boolean; error?: string }> {
    connectingIds.value = new Set([...connectingIds.value, id])
    connectionStatus.value = 'connecting'
    addLog('正在连接运动控制器...')
    try {
      await MotionService.ConnectMotion(id)
      activeControllerId.value = id
      connectionStatus.value = 'connected'
      addLog('运动控制器连接成功')
      // 切换到新控制器后同步其轴配置到 UI
      syncAxisConfigFromActiveProfile()
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
        await MotionService.MotionStopAll(targetId)
        for (const state of Object.values(axisUIStates.value)) {
          state.runState = 'idle'
        }
        try {
          await MotionService.MotionMotorOff(targetId)
        } catch (_) { /* ignore */ }
      }
      await MotionService.DisconnectMotion(targetId)
      if (isActive) {
        connectionStatus.value = 'disconnected'
      }
      addLog(`运动控制器 ${targetId} 已断开`)
      return { success: true }
    } catch (e: any) {
      return { success: false, error: e?.message || String(e) }
    }
  }

  // 将控制器状态同步到轴 UI 状态（仅活动控制器的轴）
  // syncPositionsFromStatus（拉取）与 motion:status-updated 事件回调（推送）共用此逻辑
  function applyControllerStatusToUI(allStatuses: MotionControllerStatus[]) {
    for (const ctrl of allStatuses) {
      if (ctrl.status !== 'Connected' || ctrl.id !== activeControllerId.value) continue
      for (const ax of ctrl.axes) {
        const uiState = axisUIStates.value[ax.name]
        if (!uiState) continue
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

  async function syncPositionsFromStatus() {
    try {
      const allStatuses = await MotionService.GetMotionStatusAll() as MotionControllerStatus[]
      statuses.value = allStatuses
      syncConnectionFromStatuses(allStatuses)
      applyControllerStatusToUI(allStatuses)
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

  type ActionResult = { success: boolean; error?: string }

  // 统一包装：控制器连接检查 + 轴合法性检查 + try/catch 错误归一化
  // options.requireIdle=true 时额外校验轴处于空闲状态
  async function withAxisAction(
    axis: string,
    action: (controllerId: string, bindingAxis: types.AxisName, uiState: AxisUIState) => Promise<void>,
    options: { requireIdle?: boolean } = {},
  ): Promise<ActionResult> {
    if (!activeControllerId.value) return { success: false, error: '控制器未连接' }
    if (!isAxisName(axis)) return { success: false, error: '未知轴' }
    const uiState = axisUIStates.value[axis]
    if (!uiState) return { success: false, error: '未知轴' }
    if (options.requireIdle && uiState.runState !== 'idle') {
      return { success: false, error: '轴当前不在空闲状态' }
    }
    try {
      await action(activeControllerId.value, toBindingAxisName(axis), uiState)
      return { success: true }
    } catch (e: any) {
      return { success: false, error: e?.message || String(e) }
    }
  }

  async function moveTo(axis: string, position: number): Promise<ActionResult> {
    return withAxisAction(axis, async (controllerId, bindingAxis, uiState) => {
      if (uiState.runState !== 'idle') {
        const moving = await MotionService.MotionIsAxisMoving(controllerId, bindingAxis)
        if (moving) throw new Error('轴当前不在空闲状态')
        uiState.runState = 'idle'
      }
      try {
        await MotionService.MotionMoveTo(controllerId, bindingAxis, position)
      } catch (e: any) {
        addLog(`${axis}轴运动失败: ${e?.message || String(e)}`)
        throw e
      }
      uiState.runState = 'running'
      addLog(`${axis}轴运动到目标位置 ${position}${getAxisUnit(uiState.kind)}`)
    })
  }

  async function moveBy(axis: string, delta: number): Promise<ActionResult> {
    return withAxisAction(axis, async (controllerId, bindingAxis) => {
      await MotionService.MotionMoveBy(controllerId, bindingAxis, delta)
      addLog(`${axis}轴相对移动 ${delta}`)
    })
  }

  async function startJog(axis: string, direction: 'minus' | 'plus'): Promise<ActionResult> {
    return withAxisAction(axis, async (controllerId, bindingAxis, uiState) => {
      const dir = direction === 'plus' ? 1 : -1
      await MotionService.MotionJog(controllerId, bindingAxis, dir, uiState.relativeDistance, uiState.config.maxSpeed)
      uiState.runState = direction === 'minus' ? 'jogging_minus' : 'jogging_plus'
      addLog(`${axis}轴开始${direction === 'minus' ? '反向' : '正向'}点动`)
    }, { requireIdle: true })
  }

  async function stopJog(axis: string): Promise<ActionResult> {
    const uiState = axisUIStates.value[axis]
    if (!uiState) return { success: false, error: '未知轴' }
    if (uiState.runState !== 'jogging_minus' && uiState.runState !== 'jogging_plus') {
      return { success: true }
    }
    return stopAxis(axis)
  }

  async function stopAxis(axis: string): Promise<ActionResult> {
    return withAxisAction(axis, async (controllerId, bindingAxis, uiState) => {
      await MotionService.MotionStop(controllerId, bindingAxis)
      uiState.runState = 'idle'
      addLog(`${axis}轴已停止`)
    })
  }

  async function stopAllAxes(): Promise<ActionResult> {
    if (!activeControllerId.value) return { success: true }
    try {
      await MotionService.MotionStopAll(activeControllerId.value)
      for (const state of Object.values(axisUIStates.value)) {
        state.runState = 'idle'
      }
      addLog('所有轴已停止')
      return { success: true }
    } catch (e: any) {
      return { success: false, error: e?.message || String(e) }
    }
  }

  async function home(axis: string): Promise<ActionResult> {
    return withAxisAction(axis, async (controllerId, bindingAxis) => {
      await MotionService.MotionHome(controllerId, bindingAxis)
      addLog(`${axis}轴回零`)
    })
  }

  async function definePosition(axis: string, position: number): Promise<ActionResult> {
    return withAxisAction(axis, async (controllerId, bindingAxis, uiState) => {
      await MotionService.MotionDefinePosition(controllerId, bindingAxis, position)
      uiState.currentPosition = position
      uiState.isHomed = position === 0
      uiState.targetPosition = position
      addLog(`${axis}轴置位为 ${position}`)
    })
  }

  async function emergencyStop(): Promise<{ success: boolean; error?: string }> {
    if (!activeControllerId.value) return { success: true }
    try {
      await MotionService.MotionEmergencyStop(activeControllerId.value)
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

  // 批量更新活动控制器多个轴的配置（一次 IPC 提交）。
  // 对话框内多轴编辑后统一保存时使用，避免逐轴多次提交。
  async function updateAxesConfig(axisUpdates: Record<string, Partial<AxisConfig>>) {
    for (const [axisName, config] of Object.entries(axisUpdates)) {
      const state = axisUIStates.value[axisName]
      if (!state) continue
      // kind 变更时同步轴 UI 派生状态（relativeDistance / 默认参数）
      if (config.kind && config.kind !== state.kind) {
        updateAxisKind(axisName, config.kind)
      }
      Object.assign(state.config, config)
      addLog(`${axisName}轴配置已更新`)
    }
    saveConfigToLocal()
    await persistActiveControllerAxes(axisUpdates)
  }

  async function updateAxisConfig(axisName: string, config: Partial<AxisConfig>) {
    const state = axisUIStates.value[axisName]
    if (!state) return
    await updateAxesConfig({ [axisName]: config })
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
      await MotionService.AddMotionProfile(fullProfile)
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
      await MotionService.RemoveMotionProfile(id)
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
      await MotionService.UpdateMotionProfile(types.MotionControllerProfile.createFrom({ ...profile }))
      await fetchProfiles()
      // 若该控制器已连接，刷新状态以反映轴配置同步
      await fetchStatuses()
      // 编辑对话框保存后同步轴配置到 UI（确保 AxisConfigDialog 不显示旧值）
      syncAxisConfigFromActiveProfile()
      addLog(`控制器 ${profile.name} 配置已更新`)
      return { success: true }
    } catch (e: any) {
      return { success: false, error: e?.message || String(e) }
    }
  }

  // 事件监听
  const eventListener = createWailsEventListener('motion', [
    {
      channel: 'motion:status-updated',
      handler: (event: any) => {
        const data = event.data as MotionControllerStatus[]
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
        applyControllerStatusToUI(data)
      },
    },
  ])

  function startListening() {
    eventListener.start()
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

  const stopListening = eventListener.stop

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
    updateAxisKind, updateAxisConfig, updateAxesConfig, updateAxisTarget, updateAxisRelativeDistance,
    selectAxis,
    addController, removeController, updateControllerProfile,
    addLog, clearLogs,
    getAxisUnit, getAxisKindText, getRunStateText,
    startListening, stopListening,
  }
})
