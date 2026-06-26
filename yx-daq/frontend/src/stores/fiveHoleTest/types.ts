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
  iterationCount: number; converged: boolean; valid: boolean; errorMsg?: string
}

// ==================== 数据点 & 状态 ====================

export interface FiveHoleTraversalDataPoint {
  pointId: string; probeId: string; x: number; y: number
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
  startX: number; startY: number; endX: number; endY: number
  xSteps: StepSegment[]; ySteps: StepSegment[]
}

export interface RectangleLayout {
  xMin: number; xMax: number; yMin: number; yMax: number
  xSteps: StepSegment[]; ySteps: StepSegment[]
}

export interface TraversalLayout {
  pattern: TraversalPatternValue
  line?: LineLayout
  rectangle?: RectangleLayout
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

export interface FiveHoleCalibFileInfo {
  filePath: string; fileName: string; cMa: number
}

export interface FiveHoleProbeConfig {
  probeId: string // probe1/probe2/probe3
  enabled: boolean // 配几根跑几根
  probeChannels: FiveHoleProbeChannelConfig[]
  motionAlpha: FiveHoleMotionAxisMapping
  motionBeta: FiveHoleMotionAxisMapping
  calibFiles: FiveHoleCalibFileInfo[]
}

// ==================== 全局配置 ====================

export interface FiveHoleTraversalConfig {
  name: string
  layout: TraversalLayout
  dwellTimeMs: number
  samplesPerPoint: number
  sampleIntervalMs: number
  motionTimeoutMs: number
  // PAtm/TAtm 全局共享数据源（三根共用）
  pAtmDeviceId: string
  pAtmChannel: number
  tAtmDeviceId: string
  tAtmChannel: number
  probes: FiveHoleProbeConfig[] // 1-3 根探针
  savePath: string
  saveFileName: string
}
