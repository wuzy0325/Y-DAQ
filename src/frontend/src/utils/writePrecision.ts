// ==================== 数值落盘（CSV 导出）小数位约定 ====================
//
// 约定：导出文件的小数位与界面显示一致 —— 界面显示几位，文件就写几位。
//   - 原始采集列（P1..P5 / P∞ / T∞）按对应设备通道配置的 precision 落盘
//   - 插值列按界面显示的固定位数落盘
//   - 界面未显示的列（CAS/SAT/Qc/ρ/Vx/Vy/Vz）与点位坐标保持历史精度，避免无谓丢精度
//
// 后端对应实现：src/internal/types/write_precision.go + 各领域的 write_precision.go
// （两侧规则必须同步修改）

import type { FiveHoleAtmSource, FiveHoleProbeConfig } from '../stores/fiveHoleTest/types'
import type { ThreeHoleProbeChannelConfig } from '../stores/threeHoleTest/types'
import { FiveHoleChannelRole, ThreeHoleChannelRole } from '../api/enums'
import type { FiveHoleChannelRoleValue, ThreeHoleChannelRoleValue } from '../api/enums'

/** 通道精度解析不到时的回退小数位（与后端 types.FallbackChannelPrecision 一致） */
export const FALLBACK_CHANNEL_PRECISION = 3

/** 设备通道精度上限（设备页 precision el-input-number :max=6） */
const MAX_CHANNEL_PRECISION = 6

/** 插值列/坐标列落盘小数位（与界面显示一致；界面未显示的列保持历史精度） */
export const WRITE_PRECISION = {
  pt: 3,               // 总压 Pt（界面 3 位）
  ps: 3,               // 静压 Ps（界面 3 位）
  mach: 4,             // 马赫数 Ma（界面 4 位）
  alpha: 2,            // 攻角 α（界面 2 位）
  beta: 2,             // 侧滑角 β（界面 2 位）
  velocity: 2,         // 速度 V（界面 2 位）
  cas: 4,              // 校正空速 CAS（界面未显示，保持历史精度）
  sat: 4,              // 静温 SAT（界面未显示，保持历史精度）
  dynamicPressure: 6,  // 动压 Qc（界面未显示，保持历史精度）
  density: 6,          // 密度 ρ（界面未显示，保持历史精度）
  vx: 4, vy: 4, vz: 4, // 三向速度分量（界面未显示，保持历史精度）
  position: 4,         // 点位坐标 X/Y（界面无对应数值列）
} as const

/** 设备通道精度查询所需的最小 profile 结构（DeviceProfile 的子集，便于测试注入） */
export interface ChannelPrecisionProfile {
  id: string
  channels: { precision: number }[]
}

/** 解析设备通道配置的采集精度：设备/通道未配置或配置越界时回退 FALLBACK_CHANNEL_PRECISION */
export function channelPrecision(
  profiles: readonly ChannelPrecisionProfile[],
  deviceId: string,
  channel: number,
): number {
  if (!deviceId || channel < 0) return FALLBACK_CHANNEL_PRECISION
  const precision = profiles.find(p => p.id === deviceId)?.channels?.[channel]?.precision
  if (typeof precision !== 'number' || precision < 0 || precision > MAX_CHANNEL_PRECISION) {
    return FALLBACK_CHANNEL_PRECISION
  }
  return precision
}

/** 五孔探针原始采集列的落盘小数位（与后端 five_hole.ProbePrecision 对应） */
export interface FiveHoleProbePrecision {
  p1: number; p2: number; p3: number; p4: number; p5: number
  pAtm: number; tAtm: number
}

/** 五孔：按单个探针的通道角色配置 + P∞/T∞ 数据源解析落盘小数位 */
export function fiveHoleProbePrecision(
  profiles: readonly ChannelPrecisionProfile[],
  probe: FiveHoleProbeConfig | undefined,
): FiveHoleProbePrecision {
  const prec: FiveHoleProbePrecision = {
    p1: FALLBACK_CHANNEL_PRECISION, p2: FALLBACK_CHANNEL_PRECISION, p3: FALLBACK_CHANNEL_PRECISION,
    p4: FALLBACK_CHANNEL_PRECISION, p5: FALLBACK_CHANNEL_PRECISION,
    pAtm: FALLBACK_CHANNEL_PRECISION, tAtm: FALLBACK_CHANNEL_PRECISION,
  }
  if (!probe) return prec
  const byRole: Record<string, keyof FiveHoleProbePrecision> = {
    [FiveHoleChannelRole.P1]: 'p1',
    [FiveHoleChannelRole.P2]: 'p2',
    [FiveHoleChannelRole.P3]: 'p3',
    [FiveHoleChannelRole.P4]: 'p4',
    [FiveHoleChannelRole.P5]: 'p5',
  }
  for (const ch of probe.probeChannels) {
    const key = byRole[ch.role]
    if (key) prec[key] = channelPrecision(profiles, ch.deviceId, ch.channel)
  }
  prec.pAtm = atmSourcePrecision(profiles, probe.pAtmSource)
  prec.tAtm = atmSourcePrecision(profiles, probe.tAtmSource)
  return prec
}

/** 五孔：P∞/T∞ 数据源的落盘小数位（manual 模式用回退精度，与界面显示一致） */
export function atmSourcePrecision(
  profiles: readonly ChannelPrecisionProfile[],
  src: FiveHoleAtmSource | undefined,
): number {
  // 'device' 即 ATM_SOURCE_MODE.DEVICE；此处用字面量避免 utils 依赖 store 运行时模块
  if (!src || src.mode !== 'device' || !src.deviceId) return FALLBACK_CHANNEL_PRECISION
  return channelPrecision(profiles, src.deviceId, src.channel)
}

/** 五孔：按探针 + 通道角色解析采集精度（实时显示用） */
export function fiveHoleRolePrecision(
  profiles: readonly ChannelPrecisionProfile[],
  probe: FiveHoleProbeConfig | undefined,
  role: FiveHoleChannelRoleValue,
): number {
  const ch = probe?.probeChannels.find(c => c.role === role)
  if (!ch) return FALLBACK_CHANNEL_PRECISION
  return channelPrecision(profiles, ch.deviceId, ch.channel)
}

/** 三孔探针原始采集列的落盘小数位（与后端 three_hole.ProbePrecision 对应） */
export interface ThreeHoleProbePrecision {
  p1: number; p2: number; p3: number
  pAtm: number; tAtm: number
}

/** 三孔：按角色通道配置解析落盘小数位（三孔所有通道共用同一采集设备） */
export function threeHoleProbePrecision(
  profiles: readonly ChannelPrecisionProfile[],
  deviceId: string,
  probeChannels: readonly ThreeHoleProbeChannelConfig[],
): ThreeHoleProbePrecision {
  const prec: ThreeHoleProbePrecision = {
    p1: FALLBACK_CHANNEL_PRECISION, p2: FALLBACK_CHANNEL_PRECISION, p3: FALLBACK_CHANNEL_PRECISION,
    pAtm: FALLBACK_CHANNEL_PRECISION, tAtm: FALLBACK_CHANNEL_PRECISION,
  }
  const byRole: Record<string, keyof ThreeHoleProbePrecision> = {
    [ThreeHoleChannelRole.P1]: 'p1',
    [ThreeHoleChannelRole.P2]: 'p2',
    [ThreeHoleChannelRole.P3]: 'p3',
    [ThreeHoleChannelRole.P_ATM]: 'pAtm',
    [ThreeHoleChannelRole.T_ATM]: 'tAtm',
  }
  for (const ch of probeChannels) {
    const key = byRole[ch.role]
    if (key) prec[key] = channelPrecision(profiles, deviceId, ch.channel)
  }
  return prec
}

/** 三孔：按通道角色解析采集精度（实时显示用） */
export function threeHoleRolePrecision(
  profiles: readonly ChannelPrecisionProfile[],
  deviceId: string,
  probeChannels: readonly ThreeHoleProbeChannelConfig[],
  role: ThreeHoleChannelRoleValue,
): number {
  const ch = probeChannels.find(c => c.role === role)
  if (!ch) return FALLBACK_CHANNEL_PRECISION
  return channelPrecision(profiles, deviceId, ch.channel)
}
