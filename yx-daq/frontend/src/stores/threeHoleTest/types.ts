import type { ThreeHoleChannelRoleValue, TraversalPatternValue } from '../../api/enums'

export interface ThreeHoleRawData {
  p1: number; p2: number; p3: number; pAtm: number; tAtm: number
}

export interface ThreeHoleInterpolationResult {
  ptProbe: number; psProbe: number; machProbe: number; alphaProbe: number; velocityProbe: number
  iterationCount: number; converged: boolean; valid: boolean; errorMsg?: string
}

export interface ThreeHoleTraversalDataPoint {
  pointId: string; x: number; y: number
  rawData: ThreeHoleRawData; interpResult: ThreeHoleInterpolationResult
  sampleCount: number; timestamp: number
}

export interface ThreeHoleTraversalTaskStatus {
  taskId: string; status: string
  totalPoints: number; completedPoints: number; progress: number
  currentPoint: { id: string; x: number; y: number } | null
  dataPoints: ThreeHoleTraversalDataPoint[]
  lastError: string
}

export interface ThreeHoleTraversalProgressEvent {
  taskId: string; totalPoints: number; completedPoints: number
  progress: number; currentX: number; currentY: number; phase?: string
}

export interface ThreeHoleTraversalRealtimeEvent {
  taskId: string; pointId: string
  rawData: ThreeHoleRawData; interpResult: ThreeHoleInterpolationResult
}

export interface ThreeHoleTraversalCompleteEvent {
  taskId: string; status: string
  dataPoints: ThreeHoleTraversalDataPoint[]
}

export interface ThreeHoleTraversalErrorEvent {
  taskId: string; error: string; isFatal: boolean
}

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

export interface ThreeHoleProbeChannelConfig {
  name: string; role: ThreeHoleChannelRoleValue; channel: number; enabled: boolean
}

export interface MotionAxisMapping {
  axis: string
}

export interface ThreeHoleCalibFileInfo {
  filePath: string; fileName: string; cMa: number
}

export interface ThreeHoleTraversalConfig {
  name: string
  deviceId: string
  motionControllerId: string
  layout: TraversalLayout
  probeChannels: ThreeHoleProbeChannelConfig[]
  motionAlpha: MotionAxisMapping
  motionBeta: MotionAxisMapping
  calibFiles: ThreeHoleCalibFileInfo[]
  dwellTimeMs: number
  samplesPerPoint: number
  sampleIntervalMs: number
  motionTimeoutMs: number
  savePath: string
  saveFileName: string
}
