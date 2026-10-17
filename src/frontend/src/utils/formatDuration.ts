/**
 * 耗时展示格式化（五孔/三孔移位测试的"预估 vs 实际"看板复用）
 *
 * 约定：
 * - 总时长：<1 分钟显示"N秒"，<1 小时显示"M分SS秒"，否则"H时MM分SS秒"
 * - 分量时长：1 分钟内保留 1 位小数（如 0.5s），更长时复用总时长格式
 * - 缺失/非法值统一返回 "--"，避免界面出现 NaN/负值
 */

/** 时长格式化（毫秒 → 1秒 / 2分05秒 / 1时02分05秒） */
export function formatDuration(ms: number | null | undefined): string {
  if (ms == null || !Number.isFinite(ms) || ms <= 0) return '--'
  const totalSeconds = Math.round(ms / 1000)
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = totalSeconds % 60
  const pad = (n: number) => n.toString().padStart(2, '0')
  if (hours > 0) return `${hours}时${pad(minutes)}分${pad(seconds)}秒`
  if (minutes > 0) return `${minutes}分${pad(seconds)}秒`
  return `${seconds}秒`
}

/** 分量时长格式化（毫秒 → 0.5s；≥1 分钟时用 formatDuration） */
export function formatSpan(ms: number | null | undefined): string {
  if (ms == null || !Number.isFinite(ms) || ms < 0) return '--'
  if (ms >= 60000) return formatDuration(ms)
  return `${(ms / 1000).toFixed(1)}s`
}

/** 采样频率格式化（整数 Hz；非整数保留 1 位小数） */
export function formatSamplingRate(hz: number | null | undefined): string {
  if (hz == null || !Number.isFinite(hz) || hz <= 0) return '--'
  return Number.isInteger(hz) ? `${hz} Hz` : `${hz.toFixed(1)} Hz`
}
