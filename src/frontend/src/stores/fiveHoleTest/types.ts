import type { FiveHoleChannelRoleValue, TraversalPatternValue, AxisNameValue } from '../../api/enums'

// ==================== 原始数据 & 插值结果 ====================

export interface FiveHoleRawData {
  p1: number; p2: number; p3: number; p4: number; p5: number
  pAtm: number; tAtm: number
  pTotal?: number | null
}

export interface FiveHoleInterpolationResult {
  ptProbe: number; psProbe: number; machProbe: number
  alphaProbe: number; betaProbe: number; velocityProbe: number
  casProbe: number; satProbe: number
  dynamicPressure: number; density: number
  vxProbe: number; vyProbe: number; vzProbe: number
  valid: boolean; errorMsg?: string
}

// ==================== 数据点 & 状态 ====================

export interface FiveHoleTraversalDataPoint {
  pointId: string; probeId: string; x: number; y: number
  xControllerName: string; xAxis: AxisNameValue
  yControllerName: string; yAxis: AxisNameValue
  rawData: FiveHoleRawData; interpResult: FiveHoleInterpolationResult
  sampleCount: number; timestamp: number
}

export interface FiveHoleProbeStatus {
  probeId: string
  phase: string // moving/waiting/acquiring/completed
  currentX: number; currentY: number
  rawData?: FiveHoleRawData | null
  interpResult?: FiveHoleInterpolationResult | null
}

export interface FiveHoleProbeRealtimeItem {
  probeId: string
  rawData: FiveHoleRawData
  interpResult: FiveHoleInterpolationResult
}

export interface FiveHoleTraversalTaskStatus {
  taskId: string; status: string
  totalPoints: number; completedPoints: number; progress: number
  currentPoint?: { id: string; x: number; y: number } | null
  probeStatuses: FiveHoleProbeStatus[]
  lastError?: string
}

// ==================== 事件类型 ====================

export interface FiveHoleTraversalProgressEvent {
  taskId: string; totalPoints: number; completedPoints: number
  progress: number; currentX: number; currentY: number
  phase?: string
  probeStatuses: FiveHoleProbeStatus[]
}

export interface FiveHoleTraversalRealtimeEvent {
  taskId: string; pointId: string; phase?: string
  probeRealtime: FiveHoleProbeRealtimeItem[]
}

export interface FiveHoleTraversalCompleteEvent {
  taskId: string; status: string
  probeDataPoints: Record<string, FiveHoleTraversalDataPoint[]>
}

export interface FiveHoleTraversalErrorEvent {
  taskId: string; error: string; isFatal: boolean
}

// ==================== 布点配置（复用三孔 TraversalLayout） ====================

export interface StepSegment {
  start: number; end: number; step: number
}

export interface LineLayout {
  axis: AxisNameValue // 移动物理轴名（X/Y/Z/U）
  start: number       // 起点坐标（沿 axis 方向）
  end: number         // 终点坐标（沿 axis 方向）
  step: number        // 步长（>0，方向自动按 start→end）
  fixed: number       // 静止轴坐标值
}

export interface RectangleLayout {
  xMin: number; xMax: number; yMin: number; yMax: number
  xSteps: StepSegment[]; ySteps: StepSegment[]
  xAxis: AxisNameValue // X 方向物理轴名
  yAxis: AxisNameValue // Y 方向物理轴名
}

export interface FanLayout {
  rSteps: StepSegment[]
  thetaSteps: StepSegment[]
  rAxis: AxisNameValue
  thetaAxis: AxisNameValue
}

export interface TraversalLayout {
  pattern: TraversalPatternValue
  line?: LineLayout
  rectangle?: RectangleLayout
  fan?: FanLayout
  customPoints?: { id: string; x: number; y: number }[]
}

// ==================== 探针/通道/运动轴配置 ====================

export interface FiveHoleProbeChannelConfig {
  name: string
  role: FiveHoleChannelRoleValue
  deviceId: string // 每通道独立选采集设备
  channel: number
  enabled: boolean
}

export interface FiveHoleMotionAxisMapping {
  controllerId: string // 每轴独立选位移机构
  axis: AxisNameValue
}

// 大气压/气流温度数据源模式
export const ATM_SOURCE_MODE = {
  DEVICE: 'device', // 设备读取（压力扫描阀上的通道或温度扫描阀的通道）
  MANUAL: 'manual', // 手动写入固定值
} as const

export type FiveHoleAtmSourceMode = typeof ATM_SOURCE_MODE[keyof typeof ATM_SOURCE_MODE]

// 大气压/气流温度数据源（每探针独立配置）
export interface FiveHoleAtmSource {
  mode: FiveHoleAtmSourceMode
  deviceId: string // mode=device 时有效
  channel: number // mode=device 时有效
  manualValue: number // mode=manual 时有效
}

export interface FiveHoleCalibRange {
  alphaMin: number; alphaMax: number
  betaMin: number; betaMax: number
  machMin: number; machMax: number
}

export interface FiveHoleCalibFileInfo {
  filePath: string; fileName: string; cMa: number
  validRange: FiveHoleCalibRange
}

export interface FiveHoleProbeConfig {
  probeId: string // probe1..probeN (N≤8)
  enabled: boolean // 配几根跑几根
  probeChannels: FiveHoleProbeChannelConfig[]
  motionX: FiveHoleMotionAxisMapping // X 方向：位移机构 + 轴号
  motionY: FiveHoleMotionAxisMapping // Y 方向：位移机构 + 轴号
  calibFiles: FiveHoleCalibFileInfo[]
  // 大气压/气流温度数据源（每探针独立，不全局共享）
  pAtmSource: FiveHoleAtmSource
  tAtmSource: FiveHoleAtmSource
}

// ==================== 全局配置 ====================

export interface FiveHoleTraversalConfig {
  name: string
  layout: TraversalLayout
  dwellTimeMs: number
  samplesPerPoint: number
  sampleIntervalMs: number
  motionTimeoutMs: number
  // 共用轴位：true 时所有启用探针统一使用 sharedMotionX/Y（多探针装在同一位移机构场景），
  // 各探针独立 motionX/motionY 被忽略（配置保留，切回独立模式时仍可用）
  sharedMotion: boolean
  sharedMotionX: FiveHoleMotionAxisMapping
  sharedMotionY: FiveHoleMotionAxisMapping
  probes: FiveHoleProbeConfig[] // 1-8 根探针
  savePath: string
  saveFileName: string
}
