// ==================== 三孔插值移位测试枚举定义 ====================

// 设备类型
export const DeviceType = {
  SIMULATED: 'SIMULATED',
  XY_DAQ8: 'XY-DAQ8',
  XY_DAQ16: 'XY-DAQ16',
} as const

export type DeviceTypeValue = typeof DeviceType[keyof typeof DeviceType]

// 设备类型中文标签
export const DeviceTypeLabels: Record<DeviceTypeValue, string> = {
  [DeviceType.SIMULATED]: '模拟设备',
  [DeviceType.XY_DAQ8]: 'XY-DAQ8',
  [DeviceType.XY_DAQ16]: 'XY-DAQ16',
}

// 设备类型对应的总通道数（压力 + 大气压 + 大气温度）
export function getTotalChannelCount(type: DeviceTypeValue): number {
  const pressureCount = type === DeviceType.XY_DAQ8 ? 8 : 16
  return pressureCount + 2
}

// 三孔通道角色
export const ThreeHoleChannelRole = {
  P1: 'threeHole.p1',
  P2: 'threeHole.p2',
  P3: 'threeHole.p3',
  P_ATM: 'threeHole.pAtm',
  T_ATM: 'threeHole.tAtm',
} as const

export type ThreeHoleChannelRoleValue = typeof ThreeHoleChannelRole[keyof typeof ThreeHoleChannelRole]

// 通道角色中文标签
export const ThreeHoleChannelRoleLabels: Record<ThreeHoleChannelRoleValue, string> = {
  [ThreeHoleChannelRole.P1]: '1号孔压力',
  [ThreeHoleChannelRole.P2]: '2号孔压力(中心)',
  [ThreeHoleChannelRole.P3]: '3号孔压力',
  [ThreeHoleChannelRole.P_ATM]: '大气压',
  [ThreeHoleChannelRole.T_ATM]: '大气温度',
}

// 布点模式
export const TraversalPattern = {
  LINE: 'line',
  RECTANGLE: 'rectangle',
  CUSTOM: 'custom',
} as const

export type TraversalPatternValue = typeof TraversalPattern[keyof typeof TraversalPattern]

// 布点模式中文标签
export const TraversalPatternLabels: Record<TraversalPatternValue, string> = {
  [TraversalPattern.LINE]: '直线',
  [TraversalPattern.RECTANGLE]: '矩形',
  [TraversalPattern.CUSTOM]: '自定义',
}

// 运动轴名
export const AxisName = {
  X: 'X',
  Y: 'Y',
  Z: 'Z',
  U: 'U',
} as const

export type AxisNameValue = typeof AxisName[keyof typeof AxisName]
