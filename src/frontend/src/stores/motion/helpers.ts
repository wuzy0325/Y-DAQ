import type { AxisKind, AxisRunState, AxisConfig, AxisUIState } from './types'

export function getAxisUnit(kind: AxisKind): string {
  return kind === 'LINEAR' ? 'mm' : '°'
}

export function getAxisKindText(kind: AxisKind): string {
  return kind === 'LINEAR' ? '平移轴' : '旋转轴'
}

export function getRunStateText(state: AxisRunState): string {
  const map: Record<AxisRunState, string> = {
    idle: '空闲',
    running: '运行中',
    jogging_minus: '反向点动',
    jogging_plus: '正向点动',
    error: '错误',
  }
  return map[state]
}

export function createDefaultAxisConfig(name: string, kind: AxisKind): AxisConfig {
  return {
    name,
    enabled: true,
    kind,
    inverted: false,
    stepAngleDeg: 1.8,
    microSteps: 16,
    lead: kind === 'LINEAR' ? 5.0 : 0,
    gearRatio: 1,
    maxSpeed: kind === 'LINEAR' ? 50 : 30,
    encoderScale: 0.005,
    encoderCompensation: {
      enabled: false,
      tolerance: 0.01,
      maxCycles: 3,
      settleMs: 100,
      minStep: 0,
      timeoutMs: 5000,
    },
  }
}

export function createDefaultAxisUIState(name: string, kind: AxisKind): AxisUIState {
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
    config: createDefaultAxisConfig(name, kind),
  }
}
