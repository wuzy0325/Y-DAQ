import { describe, it, expect } from 'vitest'
import { expandSteps } from '../usePointPreviewCanvas'

describe('expandSteps', () => {
  it('空数组返回空数组', () => {
    expect(expandSteps([])).toEqual([])
  })

  it('start > end 跳过', () => {
    expect(expandSteps([{ start: 10, end: 5, step: 1 }])).toEqual([])
  })

  it('step < 0 跳过', () => {
    expect(expandSteps([{ start: 0, end: 10, step: -1 }])).toEqual([])
  })

  it('step == 0 时 push (start, end)', () => {
    expect(expandSteps([{ start: 0, end: 10, step: 0 }])).toEqual([0, 10])
  })

  it('正常步进：start=0, end=10, step=2 → [0,2,4,6,8,10]', () => {
    expect(expandSteps([{ start: 0, end: 10, step: 2 }])).toEqual([0, 2, 4, 6, 8, 10])
  })

  it('单点：start=5, end=5, step=1 → [5]（n=0）', () => {
    expect(expandSteps([{ start: 5, end: 5, step: 1 }])).toEqual([5])
  })

  it('浮点步进：start=0, end=1, step=0.25 → [0, 0.25, 0.5, 0.75, 1]', () => {
    expect(expandSteps([{ start: 0, end: 1, step: 0.25 }])).toEqual([0, 0.25, 0.5, 0.75, 1])
  })

  it('n 计算：start=0, end=10, step=3 → n=floor(10/3+0.5)=3 → [0,3,6,9]', () => {
    // 注意：n=3 意味着循环 i=0..3 共 4 个值，最后一个是 9（不是 10）
    expect(expandSteps([{ start: 0, end: 10, step: 3 }])).toEqual([0, 3, 6, 9])
  })

  it('多段拼接', () => {
    const result = expandSteps([
      { start: 0, end: 4, step: 2 },  // [0, 2, 4]
      { start: 10, end: 12, step: 1 }, // [10, 11, 12]
    ])
    expect(result).toEqual([0, 2, 4, 10, 11, 12])
  })

  it('n > 50000 跳过（防止无限循环）', () => {
    // start=0, end=100000, step=1 → n=100000 > 50000 → 跳过
    expect(expandSteps([{ start: 0, end: 100000, step: 1 }])).toEqual([])
  })

  it('混合：跳过段 + 正常段', () => {
    const result = expandSteps([
      { start: 10, end: 5, step: 1 },  // 跳过（start>end）
      { start: 0, end: 4, step: 2 },   // [0, 2, 4]
      { start: 0, end: 10, step: -1 }, // 跳过（step<0）
    ])
    expect(result).toEqual([0, 2, 4])
  })
})
