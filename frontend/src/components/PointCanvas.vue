<template>
  <div class="point-canvas-wrapper">
    <div class="point-legend">
      <span class="legend-item"><span class="legend-dot pending" />待测</span>
      <span class="legend-item"><span class="legend-dot moving" />移动</span>
      <span class="legend-item"><span class="legend-dot acquiring" />采集</span>
      <span class="legend-item"><span class="legend-dot waiting" />等待</span>
      <span class="legend-item"><span class="legend-dot completed" />完成</span>
    </div>
    <canvas ref="canvasRef" class="point-canvas" :width="canvasWidth" :height="canvasHeight" />
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, nextTick } from 'vue'
import { POINT_STATE_COLORS, POINT_STATE_GLOW, POINT_STATE_BORDER } from '../constants/colors'

export type PointState = 'pending' | 'moving' | 'acquiring' | 'waiting' | 'completed'

export interface PointItem {
  x: number; y: number; state: PointState
}

const props = withDefaults(defineProps<{
  points?: PointItem[]
  width?: number
  height?: number
}>(), {
  points: () => [],
  width: 400,
  height: 400,
})

const canvasRef = ref<HTMLCanvasElement>()
const canvasWidth = props.width
const canvasHeight = props.height

// 状态颜色映射（来源于 constants/colors.ts，与 SCSS 变量保持一致）
const stateColors = POINT_STATE_COLORS as Record<PointState, string>
const stateGlow = POINT_STATE_GLOW as Record<PointState, string>
const stateBorder = POINT_STATE_BORDER as Record<PointState, string>

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

function draw() {
  const canvas = canvasRef.value
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  if (!ctx) return

  const dpr = window.devicePixelRatio || 1
  const w = props.width
  const h = props.height
  canvas.width = w * dpr
  canvas.height = h * dpr
  canvas.style.width = w + 'px'
  canvas.style.height = h + 'px'
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)

  ctx.clearRect(0, 0, w, h)

  const points = props.points
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

  const padLeft = 48; const padRight = 16; const padTop = 16; const padBottom = 40
  const plotW = w - padLeft - padRight; const plotH = h - padTop - padBottom

  const toCanvasX = (x: number) => padLeft + ((x - xMin) / xRange) * plotW
  const toCanvasY = (y: number) => padTop + plotH - ((y - yMin) / yRange) * plotH

  const xStep = niceStep(xRange, 6)
  const yStep = niceStep(yRange, 6)

  // 网格
  ctx.strokeStyle = 'rgba(255,255,255,0.06)'; ctx.lineWidth = 1
  for (let x = Math.ceil(xMin / xStep) * xStep; x <= xMax + xStep * 0.01; x += xStep) {
    const cx = toCanvasX(x)
    ctx.beginPath(); ctx.moveTo(cx, padTop); ctx.lineTo(cx, h - padBottom); ctx.stroke()
  }
  for (let y = Math.ceil(yMin / yStep) * yStep; y <= yMax + yStep * 0.01; y += yStep) {
    const cy = toCanvasY(y)
    ctx.beginPath(); ctx.moveTo(padLeft, cy); ctx.lineTo(w - padRight, cy); ctx.stroke()
  }

  // 坐标轴边框
  ctx.strokeStyle = 'rgba(255,255,255,0.15)'; ctx.lineWidth = 1
  ctx.beginPath(); ctx.moveTo(padLeft, padTop); ctx.lineTo(padLeft, h - padBottom)
  ctx.lineTo(w - padRight, h - padBottom); ctx.stroke()

  // 标签
  ctx.fillStyle = 'rgba(255,255,255,0.55)'; ctx.font = '10px sans-serif'
  ctx.textAlign = 'center'; ctx.textBaseline = 'top'
  for (let x = Math.ceil(xMin / xStep) * xStep; x <= xMax + xStep * 0.01; x += xStep) {
    ctx.fillText(formatTick(x, xStep), toCanvasX(x), h - padBottom + 5)
  }
  ctx.textAlign = 'right'; ctx.textBaseline = 'middle'
  for (let y = Math.ceil(yMin / yStep) * yStep; y <= yMax + yStep * 0.01; y += yStep) {
    ctx.fillText(formatTick(y, yStep), padLeft - 6, toCanvasY(y))
  }

  // 坐标轴标题
  ctx.fillStyle = 'rgba(255,255,255,0.45)'; ctx.font = '11px sans-serif'
  ctx.textAlign = 'center'; ctx.textBaseline = 'top'
  ctx.fillText('X (mm)', padLeft + plotW / 2, h - padBottom + 18)
  ctx.save(); ctx.translate(12, padTop + plotH / 2); ctx.rotate(-Math.PI / 2)
  ctx.textAlign = 'center'; ctx.textBaseline = 'top'
  ctx.fillText('Y (mm)', 0, 0); ctx.restore()

  // 点位
  const pointRadius = Math.max(2.5, Math.min(5, 140 / Math.sqrt(points.length)))
  for (const pt of points) {
    const cx = toCanvasX(pt.x); const cy = toCanvasY(pt.y)
    const color = stateColors[pt.state]; const glow = stateGlow[pt.state]
    const border = stateBorder[pt.state]

    ctx.shadowColor = glow !== 'transparent' ? glow : 'transparent'
    ctx.shadowBlur = glow !== 'transparent' ? 8 : 0

    ctx.beginPath(); ctx.arc(cx, cy, pointRadius, 0, Math.PI * 2)
    ctx.fillStyle = color; ctx.fill()
    ctx.strokeStyle = border; ctx.lineWidth = 1; ctx.stroke()

    ctx.shadowColor = 'transparent'; ctx.shadowBlur = 0
  }
}

watch(() => props.points, () => nextTick(draw), { deep: true })
onMounted(() => nextTick(draw))
</script>

<style lang="scss" scoped>
.point-canvas-wrapper {
  display: flex;
  flex-direction: column;
}

.point-legend {
  display: flex;
  gap: 10px;
  margin-bottom: 6px;
  flex-wrap: wrap;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 3px;
  font-size: 10px;
  color: $text-tertiary;
}

.legend-dot {
  width: 6px; height: 6px; border-radius: 50%; display: inline-block;

  &.pending { background: rgba(255,255,255,0.35); border: 1px solid rgba(255,255,255,0.2); }
  &.moving { background: $color-warning; box-shadow: 0 0 3px $color-warning-glow; }
  &.acquiring { background: $color-accent; box-shadow: 0 0 3px $color-accent-glow; }
  &.waiting { background: $color-danger; box-shadow: 0 0 3px $color-danger-glow; }
  &.completed { background: $color-success; box-shadow: 0 0 3px $color-success-glow; }
}

.point-canvas {
  width: 400px;
  height: 400px;
  border-radius: $border-radius-sm;
  background: rgba(0,0,0,0.15);
}
</style>
