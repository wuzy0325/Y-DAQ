import { describe, it, expect } from 'vitest'
import { FiveHoleChannelRole, ThreeHoleChannelRole } from '../../api/enums'
import type { FiveHoleProbeConfig } from '../../stores/fiveHoleTest/types'
import {
  FALLBACK_CHANNEL_PRECISION,
  WRITE_PRECISION,
  channelPrecision,
  atmSourcePrecision,
  fiveHoleProbePrecision,
  fiveHoleRolePrecision,
  threeHoleProbePrecision,
  threeHoleRolePrecision,
  type ChannelPrecisionProfile,
} from '../writePrecision'

describe('utils/writePrecision', () => {
  const profiles: ChannelPrecisionProfile[] = [
    { id: 'dev1', channels: [{ precision: 1 }, { precision: 0 }, { precision: 2 }, { precision: 6 }] },
    { id: 'dev2', channels: [] },
  ]

  describe('channelPrecision', () => {
    it('按设备+通道返回配置精度', () => {
      expect(channelPrecision(profiles, 'dev1', 0)).toBe(1)
      expect(channelPrecision(profiles, 'dev1', 1)).toBe(0)
      expect(channelPrecision(profiles, 'dev1', 3)).toBe(6)
    })

    it('设备未知/通道越界/通道列表为空 → 回退默认精度', () => {
      expect(channelPrecision(profiles, 'unknown', 0)).toBe(FALLBACK_CHANNEL_PRECISION)
      expect(channelPrecision(profiles, 'dev1', 9)).toBe(FALLBACK_CHANNEL_PRECISION)
      expect(channelPrecision(profiles, 'dev2', 0)).toBe(FALLBACK_CHANNEL_PRECISION)
    })

    it('设备ID为空或通道号为负 → 回退默认精度', () => {
      expect(channelPrecision(profiles, '', 0)).toBe(FALLBACK_CHANNEL_PRECISION)
      expect(channelPrecision(profiles, 'dev1', -1)).toBe(FALLBACK_CHANNEL_PRECISION)
    })

    it('配置值越界（<0 或 >6）→ 回退默认精度', () => {
      const bad: ChannelPrecisionProfile[] = [{ id: 'dev', channels: [{ precision: -1 }, { precision: 7 }] }]
      expect(channelPrecision(bad, 'dev', 0)).toBe(FALLBACK_CHANNEL_PRECISION)
      expect(channelPrecision(bad, 'dev', 1)).toBe(FALLBACK_CHANNEL_PRECISION)
    })
  })

  describe('atmSourcePrecision', () => {
    it('device 模式且已选设备 → 取该通道精度', () => {
      expect(atmSourcePrecision(profiles, { mode: 'device', deviceId: 'dev1', channel: 2, manualValue: 0 })).toBe(2)
    })

    it('manual 模式 / 未选设备 / 未配置 → 回退默认精度（与界面显示一致）', () => {
      expect(atmSourcePrecision(profiles, { mode: 'manual', deviceId: 'dev1', channel: 2, manualValue: 0 })).toBe(FALLBACK_CHANNEL_PRECISION)
      expect(atmSourcePrecision(profiles, { mode: 'device', deviceId: '', channel: 2, manualValue: 0 })).toBe(FALLBACK_CHANNEL_PRECISION)
      expect(atmSourcePrecision(profiles, undefined)).toBe(FALLBACK_CHANNEL_PRECISION)
    })
  })

  describe('fiveHoleProbePrecision', () => {
    const probe = {
      probeId: 'probe1',
      probeChannels: [
        { name: 'P1', role: FiveHoleChannelRole.P1, deviceId: 'dev1', channel: 0, enabled: true },
        { name: 'P2', role: FiveHoleChannelRole.P2, deviceId: 'dev1', channel: 1, enabled: true },
        { name: 'P3', role: FiveHoleChannelRole.P3, deviceId: 'dev1', channel: 2, enabled: true },
        { name: 'P4', role: FiveHoleChannelRole.P4, deviceId: 'dev1', channel: 3, enabled: true },
        { name: 'P5', role: FiveHoleChannelRole.P5, deviceId: 'dev2', channel: 0, enabled: true },
      ],
      pAtmSource: { mode: 'device', deviceId: 'dev1', channel: 2, manualValue: 0 },
      tAtmSource: { mode: 'manual', deviceId: '', channel: 0, manualValue: 25 },
    } as FiveHoleProbeConfig

    it('P1..P5 取各自通道精度，P∞ 取数据源通道精度，T∞（手动）回退默认', () => {
      expect(fiveHoleProbePrecision(profiles, probe)).toEqual({
        p1: 1, p2: 0, p3: 2, p4: 6, p5: FALLBACK_CHANNEL_PRECISION,
        pAtm: 2, tAtm: FALLBACK_CHANNEL_PRECISION,
      })
    })

    it('探针未配置 → 全部回退默认精度', () => {
      expect(fiveHoleProbePrecision(profiles, undefined)).toEqual({
        p1: FALLBACK_CHANNEL_PRECISION, p2: FALLBACK_CHANNEL_PRECISION, p3: FALLBACK_CHANNEL_PRECISION,
        p4: FALLBACK_CHANNEL_PRECISION, p5: FALLBACK_CHANNEL_PRECISION,
        pAtm: FALLBACK_CHANNEL_PRECISION, tAtm: FALLBACK_CHANNEL_PRECISION,
      })
    })
  })

  describe('fiveHoleRolePrecision', () => {
    it('按角色解析；角色未配置或探针缺失 → 回退默认精度', () => {
      const probe = {
        probeChannels: [{ name: 'P1', role: FiveHoleChannelRole.P1, deviceId: 'dev1', channel: 0, enabled: true }],
      } as FiveHoleProbeConfig
      expect(fiveHoleRolePrecision(profiles, probe, FiveHoleChannelRole.P1)).toBe(1)
      expect(fiveHoleRolePrecision(profiles, probe, FiveHoleChannelRole.P4)).toBe(FALLBACK_CHANNEL_PRECISION)
      expect(fiveHoleRolePrecision(profiles, undefined, FiveHoleChannelRole.P1)).toBe(FALLBACK_CHANNEL_PRECISION)
    })
  })

  describe('threeHoleProbePrecision', () => {
    it('按角色通道解析（三孔共用同一采集设备）', () => {
      expect(threeHoleProbePrecision(profiles, 'dev1', [
        { name: 'P1', role: ThreeHoleChannelRole.P1, channel: 0, enabled: true },
        { name: 'P2', role: ThreeHoleChannelRole.P2, channel: 1, enabled: true },
        { name: 'P3', role: ThreeHoleChannelRole.P3, channel: 2, enabled: true },
        { name: 'P∞', role: ThreeHoleChannelRole.P_ATM, channel: 3, enabled: true },
        { name: 'T∞', role: ThreeHoleChannelRole.T_ATM, channel: 0, enabled: true },
      ])).toEqual({ p1: 1, p2: 0, p3: 2, pAtm: 6, tAtm: 1 })
    })

    it('无通道配置 → 全部回退默认精度', () => {
      expect(threeHoleProbePrecision(profiles, 'dev1', [])).toEqual({
        p1: FALLBACK_CHANNEL_PRECISION, p2: FALLBACK_CHANNEL_PRECISION, p3: FALLBACK_CHANNEL_PRECISION,
        pAtm: FALLBACK_CHANNEL_PRECISION, tAtm: FALLBACK_CHANNEL_PRECISION,
      })
    })
  })

  describe('threeHoleRolePrecision', () => {
    it('按角色解析；角色未配置 → 回退默认精度', () => {
      const channels = [{ name: 'P1', role: ThreeHoleChannelRole.P1, channel: 0, enabled: true }]
      expect(threeHoleRolePrecision(profiles, 'dev1', channels, ThreeHoleChannelRole.P1)).toBe(1)
      expect(threeHoleRolePrecision(profiles, 'dev1', channels, ThreeHoleChannelRole.T_ATM)).toBe(FALLBACK_CHANNEL_PRECISION)
    })
  })

  describe('WRITE_PRECISION 与界面显示一致', () => {
    it('插值列位数：Pt/Ps 3、Ma 4、α/β 2、V 2；未显示列保持历史精度', () => {
      expect(WRITE_PRECISION).toEqual({
        pt: 3, ps: 3, mach: 4, alpha: 2, beta: 2, velocity: 2,
        cas: 4, sat: 4, dynamicPressure: 6, density: 6,
        vx: 4, vy: 4, vz: 4, position: 4,
      })
    })
  })
})
