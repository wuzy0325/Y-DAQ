// ==================== JS/TS 颜色常量 ====================
// 与 SCSS variables.scss 保持同步，供 <script setup> 中使用
// 来源: frontend/src/assets/styles/variables.scss

export const COLOR_PRIMARY = '#b829ff'
export const COLOR_PRIMARY_LIGHT = '#d966ff'
export const COLOR_ACCENT = '#00f5ff'
export const COLOR_ACCENT_LIGHT = '#66faff'
export const COLOR_SUCCESS = '#00ff88'
export const COLOR_WARNING = '#ffaa00'
export const COLOR_DANGER = '#ff3366'
export const COLOR_INFO = '#00aaff'

/** 图表默认色序（与 $chart-line-* SCSS 变量对应） */
const CHART_COLORS = [
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
