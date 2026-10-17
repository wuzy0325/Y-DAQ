import { describe, it, expect } from 'vitest'
import { ref } from 'vue'
import { resolveAxisDomain, usePointPreviewCanvas } from '../usePointPreviewCanvas'

interface RectLike {
  xMin: number
  xMax: number
  yMin: number
  yMax: number
  xSteps: { start: number; end: number; step: number }[]
  ySteps: { start: number; end: number; step: number }[]
}

function setup(layout: { pattern: string; rectangle?: RectLike | null }) {
  return usePointPreviewCanvas({
    layout: ref(layout),
    progress: ref(null),
    isRunning: ref(false),
    canvasSize: { w: 400, h: 300 },
  })
}

describe('composables/usePointPreviewCanvas · resolveAxisDomain', () => {
  it('正常范围：原样返回并取 6 个刻度目标', () => {
    expect(resolveAxisDomain(-20, 20)).toEqual({ min: -20, max: 20, range: 40, targetTicks: 6 })
  })

  it('单值轴（min === max）：以该值居中撑开，避免点被压在画布边缘', () => {
    const d = resolveAxisDomain(30, 30)
    expect(d.min).toBeLessThan(30)
    expect(d.max).toBeGreaterThan(30)
    expect(d.range).toBe(d.max - d.min)
    // 单值轴刻度更少，避免刻度标签拥挤
    expect(d.targetTicks).toBe(3)
    expect((d.min + d.max) / 2).toBe(30)
  })

  it('单值轴为 0 时至少撑开 ±1（不出现零高度轴）', () => {
    expect(resolveAxisDomain(0, 0)).toEqual({ min: -1, max: 1, range: 2, targetTicks: 3 })
  })

  it('单值轴为大坐标时按 5% 撑开（300 → ±15）', () => {
    expect(resolveAxisDomain(300, 300)).toEqual({ min: 285, max: 315, range: 30, targetTicks: 3 })
  })
})

describe('composables/usePointPreviewCanvas · previewPoints（矩形布点）', () => {
  it('正常范围：9 × 9 = 81 点', () => {
    const { previewPoints } = setup({
      pattern: 'rectangle',
      rectangle: {
        xMin: -20, xMax: 20, yMin: -20, yMax: 20,
        xSteps: [{ start: -20, end: 20, step: 5 }],
        ySteps: [{ start: -20, end: 20, step: 5 }],
      },
    })
    expect(previewPoints.value).toHaveLength(81)
  })

  it('Y 为单值（30~30）：只生成一行点，且 Y 全部为 30', () => {
    const { previewPoints } = setup({
      pattern: 'rectangle',
      rectangle: {
        xMin: -20, xMax: 30, yMin: 30, yMax: 30,
        xSteps: [{ start: -20, end: 30, step: 5 }],
        ySteps: [{ start: 30, end: 30, step: 5 }],
      },
    })
    expect(previewPoints.value).toHaveLength(11)
    expect(previewPoints.value.every(p => p.y === 30)).toBe(true)
  })

  it('Y 范围反向（300~20）：不生成任何点位（与后端 0 点一致）', () => {
    const { previewPoints, emptyHint } = setup({
      pattern: 'rectangle',
      rectangle: {
        xMin: -20, xMax: 20, yMin: 300, yMax: 20,
        xSteps: [{ start: -20, end: 20, step: 5 }],
        ySteps: [{ start: 300, end: 20, step: 5 }],
      },
    })
    expect(previewPoints.value).toHaveLength(0)
    expect(emptyHint.value).toContain('Y 范围最小值大于最大值')
  })

  it('X 范围反向时同样不生成点位并给出 X 提示', () => {
    const { previewPoints, emptyHint } = setup({
      pattern: 'rectangle',
      rectangle: {
        xMin: 40, xMax: 10, yMin: -20, yMax: 20,
        xSteps: [{ start: 40, end: 10, step: 5 }],
        ySteps: [{ start: -20, end: 20, step: 5 }],
      },
    })
    expect(previewPoints.value).toHaveLength(0)
    expect(emptyHint.value).toContain('X 范围最小值大于最大值')
  })
})
