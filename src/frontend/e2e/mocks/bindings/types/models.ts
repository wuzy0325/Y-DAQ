/**
 * E2E 测试用 @bindings/yx-daq/internal/types mock。
 *
 * 真实 bindings 的类型层是 Wails v3 自动生成的：每个 class 有构造器（设默认值 + Object.assign）
 * 和静态 createFrom（JSON.parse 后调用构造器）；enum 是字符串枚举。
 *
 * Mock 策略：用通用工厂 createMockClass 生成行为一致的类；enum 直接复制真实值。
 * 这样前端 `new types.DeviceProfile({...})` / `types.AxisConfig.createFrom({...})`
 * / `types.AxisName.AxisX` 都能正常工作，无需为 53 个类型各写一份样板。
 */

/** 通用 mock class 工厂：构造器 Object.assign，静态 createFrom 透传 */
function createMockClass(name: string): any {
  const cls = class {
    constructor(source: Record<string, any> = {}) {
      Object.assign(this, source)
    }
    static createFrom(source: any = {}): any {
      const parsed = typeof source === 'string' ? JSON.parse(source) : source
      return new (this as any)(parsed)
    }
  }
  Object.defineProperty(cls, 'name', { value: name })
  return cls
}

// ==================== 枚举（值与真实 bindings 一致） ====================
export enum AxisKind {
  $zero = '',
  AxisKindLinear = 'LINEAR',
  AxisKindRotary = 'ROTARY',
}

export enum AxisName {
  $zero = '',
  AxisX = 'X',
  AxisY = 'Y',
  AxisZ = 'Z',
  AxisU = 'U',
}

export enum CalibrationStatus {
  $zero = '',
  CalibStatusIdle = 'idle',
  CalibStatusConfiguring = 'configuring',
  CalibStatusRunning = 'running',
  CalibStatusPaused = 'paused',
  CalibStatusCompleted = 'completed',
  CalibStatusError = 'error',
}

export enum CalibrationType {
  $zero = '',
  CalibrationTypeFiveHole = 'five-hole',
}

export enum ConnectionStatus {
  $zero = '',
  StatusDisconnected = 'Disconnected',
  StatusConnecting = 'Connecting',
  StatusConnected = 'Connected',
  StatusError = 'Error',
}

export enum DeviceType {
  $zero = '',
  DeviceTypeSimulated = 'SIMULATED',
  DeviceTypeEA2508A = 'EA2508A',
  DeviceTypeEA2516A = 'EA2516A',
  DeviceTypeEA2516T = 'EA2516T',
}

export enum FiveHoleChannelRole {
  $zero = '',
  Role5H_P1 = 'fiveHole.p1',
  Role5H_P2 = 'fiveHole.p2',
  Role5H_P3 = 'fiveHole.p3',
  Role5H_P4 = 'fiveHole.p4',
  Role5H_P5 = 'fiveHole.p5',
  Role5H_PAtm = 'fiveHole.pAtm',
  Role5H_TAtm = 'fiveHole.tAtm',
}

export enum MotionControllerType {
  $zero = '',
  MotionTypeSimulated = 'SIMULATED-MC',
  MotionTypeEA25MC04 = 'EA25MC04',
}

export enum ProbeChannelRole {
  $zero = '',
  RoleP1 = 'fiveHole.p1',
  RoleP2 = 'fiveHole.p2',
  RoleP3 = 'fiveHole.p3',
  RoleP4 = 'fiveHole.p4',
  RoleP5 = 'fiveHole.p5',
  RolePAtm = 'fiveHole.pAtm',
  RoleTAtm = 'fiveHole.tAtm',
  RolePTotal = 'fiveHole.pTotal',
}

export enum ThreeHoleChannelRole {
  $zero = '',
  Role3H_P1 = 'threeHole.p1',
  Role3H_P2 = 'threeHole.p2',
  Role3H_P3 = 'threeHole.p3',
  Role3H_PAtm = 'threeHole.pAtm',
  Role3H_TAtm = 'threeHole.tAtm',
}

export enum TraversalPattern {
  $zero = '',
  TraversalPatternLine = 'line',
  TraversalPatternRectangle = 'rectangle',
  TraversalPatternCustom = 'custom',
}

export enum TraversalTestStatus {
  $zero = '',
  TraversalStatusIdle = 'idle',
  TraversalStatusRunning = 'running',
  TraversalStatusPaused = 'paused',
  TraversalStatusCompleted = 'completed',
  TraversalStatusError = 'error',
}

// ==================== 类（通用工厂生成） ====================
export const AxisConfig = createMockClass('AxisConfig')
export const AxisStatus = createMockClass('AxisStatus')
export const CalibrationConfig = createMockClass('CalibrationConfig')
export const CalibrationDataPoint = createMockClass('CalibrationDataPoint')
export const CalibrationPoint = createMockClass('CalibrationPoint')
export const CalibrationTaskStatus = createMockClass('CalibrationTaskStatus')
export const ChannelConfig = createMockClass('ChannelConfig')
export const DataPayload = createMockClass('DataPayload')
export const DeviceProfile = createMockClass('DeviceProfile')
export const DeviceStatus = createMockClass('DeviceStatus')
export const DiscoveredDevice = createMockClass('DiscoveredDevice')
export const EncoderCompensationConfig = createMockClass('EncoderCompensationConfig')
export const FiveHoleCalibFileInfo = createMockClass('FiveHoleCalibFileInfo')
export const FiveHoleCalibRange = createMockClass('FiveHoleCalibRange')
export const FiveHoleCoefficients = createMockClass('FiveHoleCoefficients')
export const FiveHoleInterpolationResult = createMockClass('FiveHoleInterpolationResult')
export const FiveHoleMotionAxisMapping = createMockClass('FiveHoleMotionAxisMapping')
export const FiveHoleProbeChannelConfig = createMockClass('FiveHoleProbeChannelConfig')
export const FiveHoleProbeConfig = createMockClass('FiveHoleProbeConfig')
export const FiveHoleProbeStatus = createMockClass('FiveHoleProbeStatus')
export const FiveHoleRawData = createMockClass('FiveHoleRawData')
export const FiveHoleTraversalConfig = createMockClass('FiveHoleTraversalConfig')
export const FiveHoleTraversalTaskStatus = createMockClass('FiveHoleTraversalTaskStatus')
export const LimitStatus = createMockClass('LimitStatus')
export const LineLayout = createMockClass('LineLayout')
export const MotionAxisMapping = createMockClass('MotionAxisMapping')
export const MotionControllerProfile = createMockClass('MotionControllerProfile')
export const MotionControllerStatus = createMockClass('MotionControllerStatus')
export const ProbeChannelConfig = createMockClass('ProbeChannelConfig')
export const RectangleLayout = createMockClass('RectangleLayout')
export const SphereTankGateConfig = createMockClass('SphereTankGateConfig')
export const StepSegment = createMockClass('StepSegment')
export const ThreeHoleCalibFileInfo = createMockClass('ThreeHoleCalibFileInfo')
export const ThreeHoleInterpolationResult = createMockClass('ThreeHoleInterpolationResult')
export const ThreeHoleProbeChannelConfig = createMockClass('ThreeHoleProbeChannelConfig')
export const ThreeHoleRawData = createMockClass('ThreeHoleRawData')
export const ThreeHoleTraversalConfig = createMockClass('ThreeHoleTraversalConfig')
export const ThreeHoleTraversalDataPoint = createMockClass('ThreeHoleTraversalDataPoint')
export const ThreeHoleTraversalTaskStatus = createMockClass('ThreeHoleTraversalTaskStatus')
export const TraversalLayout = createMockClass('TraversalLayout')
export const TraversalPoint = createMockClass('TraversalPoint')
