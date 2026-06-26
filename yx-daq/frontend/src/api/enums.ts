// ==================== 三孔插值移位测试枚举定义 ====================

// 设备类型
export const DeviceType = {
  SIMULATED: 'SIMULATED',
  XY_DAQ8: 'XY-DAQ8',
  XY_DAQ16: 'XY-DAQ16',
  YX_DAQT: 'YX-DAQ-T',
} as const

export type DeviceTypeValue = typeof DeviceType[keyof typeof DeviceType]

// 设备类型中文标签
export const DeviceTypeLabels: Record<DeviceTypeValue, string> = {
  [DeviceType.SIMULATED]: '模拟设备',
  [DeviceType.XY_DAQ8]: 'XY-DAQ8',
  [DeviceType.XY_DAQ16]: 'XY-DAQ16',
  [DeviceType.YX_DAQT]: 'DAQ-T-1603',
}

export interface DeviceTypeInfo {
  type: DeviceTypeValue
  label: string
  pressureChCount: number
  totalChCount: number
  isTemperature: boolean
  defaultHost: string
  defaultPort: number
  defaultUnit: string
}

export const deviceTypeRegistry: Record<DeviceTypeValue, DeviceTypeInfo> = {
  [DeviceType.XY_DAQ8]: {
    type: 'XY-DAQ8', label: 'XY-DAQ8',
    pressureChCount: 8, totalChCount: 10, isTemperature: false,
    defaultHost: '192.168.3.101', defaultPort: 9000, defaultUnit: 'kPa',
  },
  [DeviceType.XY_DAQ16]: {
    type: 'XY-DAQ16', label: 'XY-DAQ16',
    pressureChCount: 16, totalChCount: 18, isTemperature: false,
    defaultHost: '192.168.3.101', defaultPort: 9000, defaultUnit: 'kPa',
  },
  [DeviceType.YX_DAQT]: {
    type: 'YX-DAQ-T', label: 'DAQ-T-1603',
    pressureChCount: 16, totalChCount: 16, isTemperature: true,
    defaultHost: '192.168.1.7', defaultPort: 9000, defaultUnit: '°C',
  },
  [DeviceType.SIMULATED]: {
    type: 'SIMULATED', label: '模拟设备',
    pressureChCount: 16, totalChCount: 18, isTemperature: false,
    defaultHost: '127.0.0.1', defaultPort: 9000, defaultUnit: 'kPa',
  },
}

export function getDeviceInfo(type: DeviceTypeValue): DeviceTypeInfo {
  return deviceTypeRegistry[type] || deviceTypeRegistry[DeviceType.XY_DAQ16]
}

// 设备类型对应的总通道数（压力 + 大气压 + 大气温度）
export function getTotalChannelCount(type: DeviceTypeValue): number {
  return getDeviceInfo(type).totalChCount
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

// 五孔通道角色（与后端 FiveHoleChannelRole 枚举对齐）
export const FiveHoleChannelRole = {
  P1: 'fiveHole.p1',
  P2: 'fiveHole.p2',
  P3: 'fiveHole.p3',
  P4: 'fiveHole.p4',
  P5: 'fiveHole.p5',
  P_ATM: 'fiveHole.pAtm',
  T_ATM: 'fiveHole.tAtm',
} as const

export type FiveHoleChannelRoleValue = typeof FiveHoleChannelRole[keyof typeof FiveHoleChannelRole]

// 五孔通道角色中文标签
export const FiveHoleChannelRoleLabels: Record<FiveHoleChannelRoleValue, string> = {
  [FiveHoleChannelRole.P1]: '1号孔压力',
  [FiveHoleChannelRole.P2]: '2号孔压力(中心)',
  [FiveHoleChannelRole.P3]: '3号孔压力',
  [FiveHoleChannelRole.P4]: '4号孔压力',
  [FiveHoleChannelRole.P5]: '5号孔压力',
  [FiveHoleChannelRole.P_ATM]: '大气压',
  [FiveHoleChannelRole.T_ATM]: '大气温度',
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

// 热电偶类型（@f3 命令每个通道用单字符编码，仅支持以下单字符类型）
export const ThermocoupleType = {
  K: 'K',
  J: 'J',
  T: 'T',
  E: 'E',
  N: 'N',
  S: 'S',
  R: 'R',
  B: 'B',
  C: 'C',
} as const

export type ThermocoupleTypeValue = typeof ThermocoupleType[keyof typeof ThermocoupleType]

// 热电偶类型中文标签
export const ThermocoupleTypeLabels: Record<string, string> = {
  [ThermocoupleType.K]: 'K 型',
  [ThermocoupleType.J]: 'J 型',
  [ThermocoupleType.T]: 'T 型',
  [ThermocoupleType.E]: 'E 型',
  [ThermocoupleType.N]: 'N 型',
  [ThermocoupleType.S]: 'S 型',
  [ThermocoupleType.R]: 'R 型',
  [ThermocoupleType.B]: 'B 型',
  [ThermocoupleType.C]: 'C 型',
}

// 热电偶类型选项（用于 el-select）
export const thermocoupleTypeOptions: { value: string; label: string }[] =
  Object.entries(ThermocoupleTypeLabels).map(([value, label]) => ({
    value,
    label,
  }))
