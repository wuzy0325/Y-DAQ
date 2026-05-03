export type AxisKind = 'LINEAR' | 'ROTARY'
export type AxisRunState = 'idle' | 'running' | 'jogging_minus' | 'jogging_plus' | 'error'

export interface AxisStatus {
  name: string
  position: number
  moving: boolean
  homed: boolean
  posLimit: boolean
  negLimit: boolean
  compensating: boolean
}

export interface MotionControllerStatus {
  id: string
  name: string
  type: string
  status: string
  axes: AxisStatus[]
  lastError: string
}

export interface AxisConfig {
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

export interface MotionControllerProfile {
  id: string
  name: string
  type: string
  address: string
  port: number
  timeoutMs: number
  axes: AxisConfig[]
}

export interface AxisUIState {
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

export type ConnectionStatus = 'disconnected' | 'connecting' | 'connected' | 'error'
