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

interface LayoutLike {
  pattern: string
  rectangle?: RectangleLike | null
}

/**
 * 取持久化分段里的步长作为输入框初始值（无有效分段时回退默认 5）。
 *
 * 若硬编码 5，页面每次挂载（含应用重启后异步载入配置）都会用 5 覆盖
 * 配置里已保存的步长，表现为"步长改完又跳回 5"。
 */
function initialStep(segments: StepSegment[] | undefined | null, fallback = 5): number {
  const step = segments?.[0]?.step
  return typeof step === 'number' && Number.isFinite(step) && step > 0 ? step : fallback
}

/**
 * 布点步长快捷同步 composable
 * 三孔/五孔测试视图共享的步长快捷设置逻辑：用户在 UI 上设置矩形布点 X/Y 步长，
 * 自动同步到 store.config.layout.rectangle.{xSteps|ySteps} 分段。
 *
 * 直线布点已改为单轴模型（axis/start/end/step/fixed），
 * step 字段直接双向绑定，无需本 composable 介入。
 *
 * 暴露 2 个 ref（rectXStep / rectYStep），与矩形布点的
 * <el-input-number v-model="rectXStep"> 模板控件双向绑定。
 */
export function useLayoutStepSync(layout: Ref<LayoutLike>) {
  const rectXStep = ref(initialStep(layout.value?.rectangle?.xSteps))
  const rectYStep = ref(initialStep(layout.value?.rectangle?.ySteps))

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

  return { rectXStep, rectYStep }
}
