import { describe, it, expect } from 'vitest'
import { ref, nextTick } from 'vue'
import { useLayoutStepSync } from '../useLayoutStepSync'

describe('composables/useLayoutStepSync', () => {
  it('默认值：rectXStep / rectYStep = 5', () => {
    const layout = ref({
      pattern: 'rectangle',
      rectangle: { xMin: 0, xMax: 10, yMin: 0, yMax: 5, xSteps: [], ySteps: [] },
      line: null,
    })
    const { rectXStep, rectYStep } = useLayoutStepSync(layout)
    expect(rectXStep.value).toBe(5)
    expect(rectYStep.value).toBe(5)
  })

  it('immediate=true：rectangle 存在时立即同步 xSteps/ySteps', () => {
    const layout = ref({
      pattern: 'rectangle',
      rectangle: { xMin: 0, xMax: 10, yMin: 0, yMax: 5, xSteps: [], ySteps: [] },
      line: null,
    })
    const { rectXStep, rectYStep } = useLayoutStepSync(layout)
    // immediate watch 已触发
    expect(layout.value.rectangle!.xSteps).toEqual([{ start: 0, end: 10, step: 5 }])
    expect(layout.value.rectangle!.ySteps).toEqual([{ start: 0, end: 5, step: 5 }])
    // rectXStep/rectYStep 仍是默认 5
    expect(rectXStep.value).toBe(5)
    expect(rectYStep.value).toBe(5)
  })

  it('rectangle 为 null 时不报错且不写入', () => {
    const layout = ref({
      pattern: 'line',
      rectangle: null,
      line: null,
    })
    expect(() => useLayoutStepSync(layout)).not.toThrow()
    // 无 rectangle 可写入，无副作用
  })

  it('修改 rectXStep 触发 xSteps 同步', async () => {
    const layout = ref({
      pattern: 'rectangle',
      rectangle: { xMin: 0, xMax: 10, yMin: 0, yMax: 5, xSteps: [], ySteps: [] },
      line: null,
    })
    const { rectXStep } = useLayoutStepSync(layout)
    rectXStep.value = 2
    await nextTick()
    expect(layout.value.rectangle!.xSteps).toEqual([{ start: 0, end: 10, step: 2 }])
    // ySteps 不变
    expect(layout.value.rectangle!.ySteps).toEqual([{ start: 0, end: 5, step: 5 }])
  })

  it('修改 rectYStep 触发 ySteps 同步', async () => {
    const layout = ref({
      pattern: 'rectangle',
      rectangle: { xMin: 0, xMax: 10, yMin: 0, yMax: 5, xSteps: [], ySteps: [] },
      line: null,
    })
    const { rectYStep } = useLayoutStepSync(layout)
    rectYStep.value = 1
    await nextTick()
    expect(layout.value.rectangle!.ySteps).toEqual([{ start: 0, end: 5, step: 1 }])
    // xSteps 不变（仍是 immediate 时的值）
    expect(layout.value.rectangle!.xSteps).toEqual([{ start: 0, end: 10, step: 5 }])
  })

  it('修改 rectangle 边界触发 xSteps/ySteps 重算（用当前 step 值）', async () => {
    const layout = ref({
      pattern: 'rectangle',
      rectangle: { xMin: 0, xMax: 10, yMin: 0, yMax: 5, xSteps: [], ySteps: [] },
      line: null,
    })
    const { rectXStep } = useLayoutStepSync(layout)
    rectXStep.value = 3
    await nextTick()
    // 修改边界
    layout.value.rectangle!.xMin = -5
    layout.value.rectangle!.xMax = 15
    await nextTick()
    expect(layout.value.rectangle!.xSteps).toEqual([{ start: -5, end: 15, step: 3 }])
  })
})
