// ==================== JS/TS 颜色常量 ====================
// 与 SCSS variables.scss 保持同步，供 <script setup> 中使用
// 来源: frontend/src/assets/styles/variables.scss

export const COLOR_PRIMARY = '#b829ff'
export const COLOR_PRIMARY_DARK = '#a820f0'
export const COLOR_PRIMARY_LIGHT = '#d966ff'

export const COLOR_ACCENT = '#00f5ff'
export const COLOR_ACCENT_DARK = '#00d4e0'
export const COLOR_ACCENT_LIGHT = '#66faff'

export const COLOR_SUCCESS = '#00ff88'
export const COLOR_SUCCESS_DARK = '#00e07a'

export const COLOR_WARNING = '#ffaa00'
export const COLOR_WARNING_DARK = '#e69900'

export const COLOR_DANGER = '#ff3366'
export const COLOR_DANGER_DARK = '#e62e5c'

export const COLOR_INFO = '#00aaff'
export const COLOR_INFO_DARK = '#0099e6'

/** 图表默认色序（与 $chart-line-* SCSS 变量对应） */
export const CHART_COLORS = [
  COLOR_PRIMARY,  // 系列1 / Kα
  COLOR_ACCENT,   // 系列2 / Kβ
  COLOR_SUCCESS,  // 系列3 / Cps
  COLOR_WARNING,  // 系列4 / Cpt
  COLOR_DANGER,
  COLOR_INFO,
  COLOR_PRIMARY_LIGHT,
  COLOR_ACCENT_LIGHT,
]

/** ECharts / Canvas 等需要数组形式的图表配色 */
export const NEON_COLORS = CHART_COLORS

/** 点状态颜色（Canvas/JS 使用） */
export const POINT_STATE_COLORS: Record<string, string> = {
  pending: 'rgba(255,255,255,0.35)',
  moving: COLOR_WARNING,
  acquiring: COLOR_ACCENT,
  waiting: COLOR_DANGER,
  completed: COLOR_SUCCESS,
}

export const POINT_STATE_GLOW: Record<string, string> = {
  pending: 'transparent',
  moving: 'rgba(255,170,0,0.4)',
  acquiring: 'rgba(0,245,255,0.4)',
  waiting: 'rgba(255,51,102,0.4)',
  completed: 'rgba(0,255,136,0.3)',
}

export const POINT_STATE_BORDER: Record<string, string> = {
  pending: 'rgba(255,255,255,0.2)',
  moving: 'rgba(255,170,0,0.6)',
  acquiring: 'rgba(0,245,255,0.6)',
  waiting: 'rgba(255,51,102,0.6)',
  completed: 'rgba(0,255,136,0.6)',
}

/** 轴色（X/Y/Z/U） */
export const AXIS_COLORS: Record<string, string> = {
  X: COLOR_PRIMARY,
  Y: COLOR_ACCENT,
  Z: COLOR_SUCCESS,
  U: COLOR_WARNING,
}
