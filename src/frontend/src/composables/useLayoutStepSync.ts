import { ref, watch, type Ref } from 'vue'

interface StepSegment {
  start: number
  end: number
  step: number
}

interface RectangleLike {
  xMin: number
  xMax: number
  yMin: number
  yMax: number
  xSteps: StepSegment[]
  ySteps: StepSegment[]
}

interface LineLike {
  startX: number
  startY: number
  endX: number
  endY: number
  xSteps: StepSegment[]
  ySteps: StepSegment[]
}

interface LayoutLike {
  pattern: string
  rectangle?: RectangleLike | null
  line?: LineLike | null
}

/**
 * 布点步长快捷同步 composable
 * 三孔/五孔测试视图共享的步长快捷设置逻辑：用户在 UI 上设置 X/Y 步长，
 * 自动同步到 store.config.layout.{rectangle|line}.{xSteps|ySteps} 分段。
 *
 * 暴露 4 个 ref（rectXStep / rectYStep / lineXStep / lineYStep），
 * 与 <el-input-number v-model="rectXStep"> 等模板控件双向绑定。
 */
export function useLayoutStepSync(layout: Ref<LayoutLike>) {
  const rectXStep = ref(5)
  const rectYStep = ref(5)
  const lineXStep = ref(5)
  const lineYStep = ref(5)

  watch(
    () => {
      const r = layout.value.rectangle
      if (!r) return null
      return { xMin: r.xMin, xMax: r.xMax, yMin: r.yMin, yMax: r.yMax, xs: rectXStep.value, ys: rectYStep.value }
    },
    (val) => {
      if (!val) return
      const r = layout.value.rectangle
      if (!r) return
      r.xSteps = [{ start: val.xMin, end: val.xMax, step: val.xs }]
      r.ySteps = [{ start: val.yMin, end: val.yMax, step: val.ys }]
    },
    { immediate: true },
  )

  watch(
    () => {
      const l = layout.value.line
      if (!l) return null
      return { startX: l.startX, startY: l.startY, endX: l.endX, endY: l.endY, xs: lineXStep.value, ys: lineYStep.value }
    },
    (val) => {
      if (!val) return
      const l = layout.value.line
      if (!l) return
      l.xSteps = [{ start: l.startX, end: l.endX, step: val.xs }]
      l.ySteps = [{ start: l.startY, end: l.endY, step: val.ys }]
    },
    { immediate: true },
  )

  return { rectXStep, rectYStep, lineXStep, lineYStep }
}
