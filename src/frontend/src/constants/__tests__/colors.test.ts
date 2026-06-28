import { describe, it, expect } from 'vitest'
import {
  COLOR_PRIMARY,
  COLOR_PRIMARY_LIGHT,
  COLOR_ACCENT,
  COLOR_ACCENT_LIGHT,
  COLOR_SUCCESS,
  COLOR_WARNING,
  COLOR_DANGER,
  COLOR_INFO,
  NEON_COLORS,
} from '../colors'

const ALL_COLORS = [
  COLOR_PRIMARY,
  COLOR_PRIMARY_LIGHT,
  COLOR_ACCENT,
  COLOR_ACCENT_LIGHT,
  COLOR_SUCCESS,
  COLOR_WARNING,
  COLOR_DANGER,
  COLOR_INFO,
]

describe('constants/colors', () => {
  it('所有颜色常量应为 #rrggbb 格式', () => {
    for (const c of ALL_COLORS) {
      expect(c).toMatch(/^#[0-9a-f]{6}$/i)
    }
  })

  it('NEON_COLORS 应有 8 个颜色且与常量顺序一致', () => {
    expect(NEON_COLORS).toHaveLength(8)
    expect(NEON_COLORS[0]).toBe(COLOR_PRIMARY)
    expect(NEON_COLORS[1]).toBe(COLOR_ACCENT)
    expect(NEON_COLORS[2]).toBe(COLOR_SUCCESS)
    expect(NEON_COLORS[3]).toBe(COLOR_WARNING)
    expect(NEON_COLORS[4]).toBe(COLOR_DANGER)
    expect(NEON_COLORS[5]).toBe(COLOR_INFO)
    expect(NEON_COLORS[6]).toBe(COLOR_PRIMARY_LIGHT)
    expect(NEON_COLORS[7]).toBe(COLOR_ACCENT_LIGHT)
  })

  it('PRIMARY 与 PRIMARY_LIGHT 应不同', () => {
    expect(COLOR_PRIMARY).not.toBe(COLOR_PRIMARY_LIGHT)
  })

  it('ACCENT 与 ACCENT_LIGHT 应不同', () => {
    expect(COLOR_ACCENT).not.toBe(COLOR_ACCENT_LIGHT)
  })
})
