import { describe, it, expect } from 'vitest'
import {
  DeviceType,
  DeviceTypeLabels,
  deviceTypeRegistry,
  getDeviceInfo,
  getTotalChannelCount,
  ThreeHoleChannelRole,
  ThreeHoleChannelRoleLabels,
  FiveHoleChannelRole,
  FiveHoleChannelRoleLabels,
  TraversalPattern,
  TraversalPatternLabels,
  AxisName,
  ThermocoupleType,
  ThermocoupleTypeLabels,
  thermocoupleTypeOptions,
} from '../enums'

describe('enums', () => {
  describe('DeviceType', () => {
    it('应有 4 种设备类型', () => {
      expect(Object.keys(DeviceType)).toHaveLength(4)
      expect(DeviceType.SIMULATED).toBe('SIMULATED')
      expect(DeviceType.EA2508A).toBe('EA2508A')
      expect(DeviceType.EA2516A).toBe('EA2516A')
      expect(DeviceType.EA2516T).toBe('EA2516T')
    })

    it('DeviceTypeLabels 应覆盖所有类型', () => {
      for (const val of Object.values(DeviceType)) {
        expect(DeviceTypeLabels[val]).toBeTruthy()
      }
    })

    it('deviceTypeRegistry 应覆盖所有类型且字段完整', () => {
      for (const val of Object.values(DeviceType)) {
        const info = deviceTypeRegistry[val]
        expect(info).toBeDefined()
        expect(info.type).toBe(val)
        expect(info.label).toBeTruthy()
        expect(info.pressureChCount).toBeGreaterThan(0)
        expect(info.totalChCount).toBeGreaterThanOrEqual(info.pressureChCount)
        expect(info.defaultHost).toBeTruthy()
        expect(info.defaultPort).toBeGreaterThan(0)
        expect(info.defaultUnit).toBeTruthy()
      }
    })

    it('EA2516T 应为温度设备（isTemperature=true, defaultUnit=°C）', () => {
      expect(deviceTypeRegistry[DeviceType.EA2516T].isTemperature).toBe(true)
      expect(deviceTypeRegistry[DeviceType.EA2516T].defaultUnit).toBe('°C')
    })

    it('非 DAQT 设备应为压力设备（isTemperature=false）', () => {
      expect(deviceTypeRegistry[DeviceType.EA2508A].isTemperature).toBe(false)
      expect(deviceTypeRegistry[DeviceType.EA2516A].isTemperature).toBe(false)
      expect(deviceTypeRegistry[DeviceType.SIMULATED].isTemperature).toBe(false)
    })
  })

  describe('getDeviceInfo', () => {
    it('已知类型返回对应信息', () => {
      const info = getDeviceInfo(DeviceType.EA2508A)
      expect(info.type).toBe('EA2508A')
      expect(info.pressureChCount).toBe(8)
      expect(info.totalChCount).toBe(10)
    })

    it('未知类型回退到 EA2516A', () => {
      const info = getDeviceInfo('unknown' as any)
      expect(info.type).toBe('EA2516A')
    })
  })

  describe('getTotalChannelCount', () => {
    it('EA2508A 总通道数 = 10', () => {
      expect(getTotalChannelCount(DeviceType.EA2508A)).toBe(10)
    })

    it('EA2516A 总通道数 = 18（16 压力 + 大气压 + 大气温度）', () => {
      expect(getTotalChannelCount(DeviceType.EA2516A)).toBe(18)
    })

    it('EA2516T 总通道数 = 16', () => {
      expect(getTotalChannelCount(DeviceType.EA2516T)).toBe(16)
    })
  })

  describe('ThreeHoleChannelRole', () => {
    it('应有 5 个角色（P1/P2/P3/P_ATM/T_ATM）', () => {
      expect(Object.keys(ThreeHoleChannelRole)).toHaveLength(5)
      expect(ThreeHoleChannelRole.P1).toBe('threeHole.p1')
      expect(ThreeHoleChannelRole.P_ATM).toBe('threeHole.pAtm')
      expect(ThreeHoleChannelRole.T_ATM).toBe('threeHole.tAtm')
    })

    it('Labels 应覆盖所有角色', () => {
      for (const val of Object.values(ThreeHoleChannelRole)) {
        expect(ThreeHoleChannelRoleLabels[val]).toBeTruthy()
      }
    })
  })

  describe('FiveHoleChannelRole', () => {
    it('应有 7 个角色（P1-P5 + P_ATM + T_ATM）', () => {
      expect(Object.keys(FiveHoleChannelRole)).toHaveLength(7)
      expect(FiveHoleChannelRole.P5).toBe('fiveHole.p5')
    })

    it('Labels 应覆盖所有角色', () => {
      for (const val of Object.values(FiveHoleChannelRole)) {
        expect(FiveHoleChannelRoleLabels[val]).toBeTruthy()
      }
    })
  })

  describe('TraversalPattern', () => {
    it('应有 4 种模式', () => {
      expect(Object.keys(TraversalPattern)).toHaveLength(4)
      expect(TraversalPattern.LINE).toBe('line')
      expect(TraversalPattern.RECTANGLE).toBe('rectangle')
      expect(TraversalPattern.CUSTOM).toBe('custom')
      expect(TraversalPattern.FAN).toBe('fan')
    })

    it('Labels 应覆盖所有模式', () => {
      for (const val of Object.values(TraversalPattern)) {
        expect(TraversalPatternLabels[val]).toBeTruthy()
      }
    })
  })

  describe('AxisName', () => {
    it('应有 X/Y/Z/U 4 轴', () => {
      expect(Object.keys(AxisName)).toHaveLength(4)
      expect(AxisName.X).toBe('X')
      expect(AxisName.U).toBe('U')
    })
  })

  describe('ThermocoupleType', () => {
    it('应有 9 种热电偶类型（K/J/T/E/N/S/R/B/C）', () => {
      expect(Object.keys(ThermocoupleType)).toHaveLength(9)
    })

    it('Labels 应覆盖所有类型', () => {
      for (const val of Object.values(ThermocoupleType)) {
        expect(ThermocoupleTypeLabels[val]).toBeTruthy()
      }
    })

    it('thermocoupleTypeOptions 应由 Labels 派生且长度一致', () => {
      expect(thermocoupleTypeOptions).toHaveLength(9)
      for (const opt of thermocoupleTypeOptions) {
        expect(opt.value).toBeTruthy()
        expect(opt.label).toBeTruthy()
        expect(ThermocoupleTypeLabels[opt.value]).toBe(opt.label)
      }
    })
  })
})
