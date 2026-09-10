import { ref, computed, watch, nextTick, type Ref, type ComputedRef } from 'vue'

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
  axis: string // 物理轴名（如 X/Y/Z/U）或三孔的 'x'/'y'
  start: number
  end: number
  step: number
  fixed: number
}

interface CustomPointLike {
  id: string
  x: number
  y: number
}

interface FanLayoutLike {
  rSteps: StepSegment[]
  thetaSteps: StepSegment[]
}

interface LayoutLike {
  pattern: string
  rectangle?: RectangleLike | null
  line?: LineLike | null
  fan?: FanLayoutLike | null
  customPoints?: CustomPointLike[]
}

interface ProgressLike {
  completedPoints?: number
  phase?: string
}

export type PointState = 'pending' | 'moving' | 'acquiring' | 'waiting' | 'completed'

export interface PreviewPoint {
  x: number
  y: number
  state: PointState
}

export interface UsePointPreviewCanvasOptions {
  layout: Ref<LayoutLike>
  progress: Ref<ProgressLike | null>
  isRunning: Ref<boolean>
  // 画布尺寸模式：
  //   'parent' —— 跟随父容器尺寸（响应式，默认）
  //   { w, h } —— 固定尺寸
  canvasSize?: 'parent' | { w: number; h: number }
  // 画布内边距
  padding?: { left: number; right: number; top: number; bottom: number }
  // 点半径缩放因子：pointRadius = clamp(2.5, 5, factor / sqrt(points.length))
  pointRadiusFactor?: number
  // 是否支持 'custom' 布点模式（五孔支持，三孔不支持）
  supportCustomPattern?: boolean
  // 直线布点 axis 为物理轴名时，调用此函数判断是 X 方向（返回 true）还是 Y 方向（返回 false）
  // 不传时按三孔语义：'x' -> true, 'y' -> false，其他兜底 true
  lineAxisResolver?: (axis: string) => boolean
}

/**
 * expandSteps 展开分段步长为具体数值列表
 * 与后端 point_generator.go::expandStepSegments 算法保持一致：
 *   - skip start>end 或 step<0
 *   - step==0 时 push (start, end)
 *   - n = floor((end-start)/step + 0.5)，避免浮点累加误差
 *   - 不再额外四舍五入数值，与后端 seg.Start+float64(i)*seg.Step 完全对齐
 */
export function expandSteps(steps: StepSegment[]): number[] {
  const values: number[] = []
  const MAX_POINTS = 50000
  for (const seg of steps) {
    if (seg.start > seg.end) continue
    if (seg.step === 0) {
      values.push(seg.start, seg.end)
      continue
    }
    if (seg.step < 0) continue
    const n = Math.floor((seg.end - seg.start) / seg.step + 0.5)
    if (n < 0 || n > MAX_POINTS) continue
    for (let i = 0; i <= n; i++) {
      values.push(seg.start + i * seg.step)
    }
  }
  return values
}

/**
 * 沿单轴方向展开点位坐标（与后端 expandLineAxisValues 逻辑一致）
 * - 支持 start>end 反向（步长恒为正，方向自动跟随 start→end）
 * - 步长不整除时强制包含 end（最后一点用 end 替代，避免浮点漂移）
 */
export function expandLineAxisValues(start: number, end: number, step: number): number[] {
  if (step <= 0) return [start]
  if (start === end) return [start]

  const direction = end < start ? -1 : 1
  const absStep = step
  let absDelta = end - start
  if (absDelta < 0) absDelta = -absDelta

  // n = floor(absDelta/absStep)，整数步数
  const n = Math.floor(absDelta / absStep)
  const values: number[] = [start]
  for (let i = 1; i <= n; i++) {
    values.push(start + direction * i * absStep)
  }
  // 若 n 步尚未抵达 end，追加 end 作为终点
  if (n * absStep < absDelta) {
    values.push(end)
  }
  return values
}

/**
 * 布点预览 Canvas composable
 * 三孔/五孔测试视图共享的布点预览逻辑：previewPoints 计算属性 + drawPointCanvas 绘制函数。
 *
 * previewPoints 根据布局（rectangle/line/custom）展开分段步长，
 * 并根据进度事件标记 pending/moving/acquiring/waiting/completed 状态。
 *
 * drawPointCanvas 在 canvas 上绘制带状态颜色、网格、坐标轴的布点预览图。
 * 调用方需在 <canvas ref="pointCanvasRef"> 上绑定返回的 pointCanvasRef，
 * 并通过 watch(previewPoints, () => nextTick(drawPointCanvas)) 触发重绘
 * （或自行调用 drawPointCanvas）。
 */
export function usePointPreviewCanvas(options: UsePointPreviewCanvasOptions): {
  pointCanvasRef: Ref<HTMLCanvasElement | undefined>
  previewPoints: ComputedRef<PreviewPoint[]>
  drawPointCanvas: () => void
} {
  const canvasSize = options.canvasSize ?? 'parent'
  const padding = options.padding ?? { left: 44, right: 16, top: 12, bottom: 36 }
  const pointRadiusFactor = options.pointRadiusFactor ?? 120
  const supportCustomPattern = options.supportCustomPattern ?? false

  const pointCanvasRef = ref<HTMLCanvasElement>()

  const previewPoints = computed<PreviewPoint[]>(() => {
    const layout = options.layout.value
    const points: PreviewPoint[] = []

    if (layout.pattern === 'rectangle' && layout.rectangle) {
      const r = layout.rectangle
      const xValues = expandSteps(r.xSteps)
      const yValues = expandSteps(r.ySteps)
      if (xValues.length === 0) { xValues.push(r.xMin, r.xMax) }
      if (yValues.length === 0) { yValues.push(r.yMin, r.yMax) }
      for (const x of xValues) {
        for (const y of yValues) {
          points.push({ x, y, state: 'pending' })
        }
      }
    } else if (layout.pattern === 'line' && layout.line) {
      const l = layout.line
      const values = expandLineAxisValues(l.start, l.end, l.step)
      const isX = l.axis === 'x' || (l.axis !== 'y' && options.lineAxisResolver?.(l.axis) !== false)
      for (const v of values) {
        if (isX) {
          points.push({ x: v, y: l.fixed, state: 'pending' })
        } else {
          points.push({ x: l.fixed, y: v, state: 'pending' })
        }
      }
    } else if (layout.pattern === 'fan' && layout.fan) {
      // 点位为轴坐标（r=半径轴绝对位置，thetaDeg=角度轴绝对位置），
      // 绘制时转为笛卡尔坐标（数学极坐标：0° 沿 +X，逆时针为正）呈现真实扇面形状
      const f = layout.fan
      const rValues = expandSteps(f.rSteps)
      const thetaValues = expandSteps(f.thetaSteps)
      for (const r of rValues) {
        for (const thetaDeg of thetaValues) {
          const theta = thetaDeg * Math.PI / 180
          points.push({ x: r * Math.cos(theta), y: r * Math.sin(theta), state: 'pending' })
        }
      }
    } else if (supportCustomPattern && layout.pattern === 'custom' && layout.customPoints) {
      for (const pt of layout.customPoints) {
        points.push({ x: pt.x, y: pt.y, state: 'pending' })
      }
    }

    // 按序号标记已完成点（与后端遍历顺序一致）
    const completedCount = options.progress.value?.completedPoints ?? 0
    for (let i = 0; i < points.length && i < completedCount; i++) {
      points[i].state = 'completed'
    }

    // 标记当前正在处理的点
    const currentPoint = options.progress.value
    if (currentPoint && options.isRunning.value && completedCount < points.length) {
      const phase = currentPoint.phase
      points[completedCount].state = (phase === 'moving' || phase === 'waiting' || phase === 'acquiring') ? phase : 'acquiring'
    }

    return points
  })

  function drawPointCanvas() {
    const canvas = pointCanvasRef.value
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    const dpr = window.devicePixelRatio || 1
    let w: number
    let h: number
    if (canvasSize === 'parent') {
      const parent = canvas.parentElement
      // 默认 fallback 尺寸（与 FiveHole 原值一致）
      w = parent ? parent.clientWidth - 20 : 440
      h = parent ? parent.clientHeight - 20 : 320
    } else {
      w = canvasSize.w
      h = canvasSize.h
    }
    canvas.width = w * dpr
    canvas.height = h * dpr
    canvas.style.width = w + 'px'
    canvas.style.height = h + 'px'
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0)

    ctx.clearRect(0, 0, w, h)

    const points = previewPoints.value
    if (points.length === 0) {
      ctx.fillStyle = 'rgba(255,255,255,0.3)'
      ctx.font = '14px sans-serif'
      ctx.textAlign = 'center'
      ctx.fillText('请先配置布点参数', w / 2, h / 2)
      return
    }

    const xs = points.map(p => p.x)
    const ys = points.map(p => p.y)
    const xMin = Math.min(...xs)
    const xMax = Math.max(...xs)
    const yMin = Math.min(...ys)
    const yMax = Math.max(...ys)
    const xRange = xMax - xMin || 1
    const yRange = yMax - yMin || 1

    const padLeft = padding.left
    const padRight = padding.right
    const padTop = padding.top
    const padBottom = padding.bottom
    const plotW = w - padLeft - padRight
    const plotH = h - padTop - padBottom

    const toCanvasX = (x: number) => padLeft + ((x - xMin) / xRange) * plotW
    const toCanvasY = (y: number) => padTop + plotH - ((y - yMin) / yRange) * plotH

    function niceStep(range: number, targetTicks: number): number {
      const rough = range / targetTicks
      const mag = Math.pow(10, Math.floor(Math.log10(rough)))
      const norm = rough / mag
      let nice: number
      if (norm <= 1.5) nice = 1
      else if (norm <= 3.5) nice = 2
      else if (norm <= 7.5) nice = 5
      else nice = 10
      return nice * mag
    }
    function formatTick(value: number, step: number): string {
      if (step >= 1) return Math.round(value).toString()
      const decimals = Math.max(0, -Math.floor(Math.log10(step)))
      return value.toFixed(decimals)
    }

    const xTickStep = niceStep(xRange, 6)
    const yTickStep = niceStep(yRange, 6)

    // 网格
    ctx.strokeStyle = 'rgba(255,255,255,0.06)'
    ctx.lineWidth = 1
    for (let x = Math.ceil(xMin / xTickStep) * xTickStep; x <= xMax + xTickStep * 0.01; x += xTickStep) {
      const cx = toCanvasX(x)
      ctx.beginPath()
      ctx.moveTo(cx, padTop)
      ctx.lineTo(cx, h - padBottom)
      ctx.stroke()
    }
    for (let y = Math.ceil(yMin / yTickStep) * yTickStep; y <= yMax + yTickStep * 0.01; y += yTickStep) {
      const cy = toCanvasY(y)
      ctx.beginPath()
      ctx.moveTo(padLeft, cy)
      ctx.lineTo(w - padRight, cy)
      ctx.stroke()
    }

    // 坐标轴边框
    ctx.strokeStyle = 'rgba(255,255,255,0.15)'
    ctx.lineWidth = 1
    ctx.beginPath()
    ctx.moveTo(padLeft, padTop)
    ctx.lineTo(padLeft, h - padBottom)
    ctx.lineTo(w - padRight, h - padBottom)
    ctx.stroke()

    // 刻度标签
    ctx.fillStyle = 'rgba(255,255,255,0.55)'
    ctx.font = '10px sans-serif'
    ctx.textAlign = 'center'
    ctx.textBaseline = 'top'
    for (let x = Math.ceil(xMin / xTickStep) * xTickStep; x <= xMax + xTickStep * 0.01; x += xTickStep) {
      ctx.fillText(formatTick(x, xTickStep), toCanvasX(x), h - padBottom + 5)
    }
    ctx.textAlign = 'right'
    ctx.textBaseline = 'middle'
    for (let y = Math.ceil(yMin / yTickStep) * yTickStep; y <= yMax + yTickStep * 0.01; y += yTickStep) {
      ctx.fillText(formatTick(y, yTickStep), padLeft - 6, toCanvasY(y))
    }

    // 坐标轴标题
    ctx.fillStyle = 'rgba(255,255,255,0.45)'
    ctx.font = '11px sans-serif'
    ctx.textAlign = 'center'
    ctx.textBaseline = 'top'
    ctx.fillText('X (mm)', padLeft + plotW / 2, h - padBottom + 18)
    ctx.save()
    ctx.translate(12, padTop + plotH / 2)
    ctx.rotate(-Math.PI / 2)
    ctx.textAlign = 'center'
    ctx.textBaseline = 'top'
    ctx.fillText('Y (mm)', 0, 0)
    ctx.restore()

    // 状态颜色映射
    const stateColors: Record<PointState, string> = {
      pending: 'rgba(255,255,255,0.35)',
      moving: '#ffaa00',
      acquiring: '#00f5ff',
      waiting: '#ff3366',
      completed: '#00ff88',
    }
    const stateGlow: Record<PointState, string> = {
      pending: 'transparent',
      moving: 'rgba(255,170,0,0.4)',
      acquiring: 'rgba(0,245,255,0.4)',
      waiting: 'rgba(255,51,102,0.4)',
      completed: 'rgba(0,255,136,0.3)',
    }
    const stateBorder: Record<PointState, string> = {
      pending: 'rgba(255,255,255,0.2)',
      moving: 'rgba(255,170,0,0.6)',
      acquiring: 'rgba(0,245,255,0.6)',
      waiting: 'rgba(255,51,102,0.6)',
      completed: 'rgba(0,255,136,0.6)',
    }

    const pointRadius = Math.max(2.5, Math.min(5, pointRadiusFactor / Math.sqrt(points.length)))
    for (const pt of points) {
      const cx = toCanvasX(pt.x)
      const cy = toCanvasY(pt.y)
      const color = stateColors[pt.state]
      const glow = stateGlow[pt.state]
      const border = stateBorder[pt.state]

      if (glow !== 'transparent') {
        ctx.shadowColor = glow
        ctx.shadowBlur = 8
      } else {
        ctx.shadowColor = 'transparent'
        ctx.shadowBlur = 0
      }

      ctx.beginPath()
      ctx.arc(cx, cy, pointRadius, 0, Math.PI * 2)
      ctx.fillStyle = color
      ctx.fill()

      ctx.strokeStyle = border
      ctx.lineWidth = 1
      ctx.stroke()

      ctx.shadowColor = 'transparent'
      ctx.shadowBlur = 0
    }
  }

  return { pointCanvasRef, previewPoints, drawPointCanvas }
}
