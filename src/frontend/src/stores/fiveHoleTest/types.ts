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

// ==================== 耗时预估 vs 实际 ====================

// 预估口径：移动 = 点位间距离 / 轴最大速度（同相位并行取最大，扇面 θ/R 两相相加）
//           驻留 = 配置驻留时间
//           采样 = 采样次数 × 最慢采集设备出帧周期（采样频率 = 1000/PeriodMs）
// 实际口径：移动/驻留/采样三段分别实测（暂停时长不计入），总耗时为测试开始至今扣除暂停
export interface FiveHoleTraversalTiming {
  estimatedTotalMs: number  // 预估总时长
  estMoveTotalMs: number    // 预估移动耗时合计
  estDwellTotalMs: number   // 预估驻留耗时合计
  estSampleTotalMs: number  // 预估采样耗时合计
  estMoveMsPerPoint: number   // 预估单点移动耗时
  estDwellMsPerPoint: number  // 预估单点驻留耗时
  estSampleMsPerPoint: number // 预估单点采样耗时
  sampleHz: number          // 参与预估的最慢设备采样频率（Hz）
  actualElapsedMs: number   // 实际已用总时长（不含暂停）
  actualMoveMs: number      // 实际移动耗时累计
  actualDwellMs: number     // 实际驻留耗时累计
  actualSampleMs: number    // 实际采样耗时累计
  actualPoints: number      // 已完成点位数量
}

export interface FiveHoleTraversalTaskStatus {
  taskId: string; status: string
  totalPoints: number; completedPoints: number; progress: number
  currentPoint?: { id: string; x: number; y: number } | null
  probeStatuses: FiveHoleProbeStatus[]
  lastError?: string
  timing?: FiveHoleTraversalTiming | null
}

// ==================== 事件类型 ====================

export interface FiveHoleTraversalProgressEvent {
  taskId: string; totalPoints: number; completedPoints: number
  progress: number; currentX: number; currentY: number
  phase?: string
  probeStatuses: FiveHoleProbeStatus[]
  timing?: FiveHoleTraversalTiming | null
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

// ==================== 数据保存模式 ====================

// 每个点位多次采样后的保存方式
export const FIVE_HOLE_DATA_MODE = {
  AVERAGE: 'average', // 默认：3σ 滤波平均后每点位保存一个数据点
  RAW: 'raw',         // 保存每个原始采样帧及其插值结果
} as const

export type FiveHoleDataMode = typeof FIVE_HOLE_DATA_MODE[keyof typeof FIVE_HOLE_DATA_MODE]

// ==================== 全局配置 ====================

export interface FiveHoleTraversalConfig {
  name: string
  layout: TraversalLayout
  dwellTimeMs: number
  samplesPerPoint: number
  motionTimeoutMs: number
  // 数据保存模式：默认 average（3σ 滤波平均）；raw 保存每个原始采样帧及其插值结果
  dataMode: FiveHoleDataMode
  // 共用轴位：true 时所有启用探针统一使用 sharedMotionX/Y（多探针装在同一位移机构场景），
  // 各探针独立 motionX/motionY 被忽略（配置保留，切回独立模式时仍可用）
  sharedMotion: boolean
  sharedMotionX: FiveHoleMotionAxisMapping
  sharedMotionY: FiveHoleMotionAxisMapping
  probes: FiveHoleProbeConfig[] // 1-8 根探针
  savePath: string
  saveFileName: string
}
