<template>
  <div class="calib-point-editor">
    <div class="editor-toolbar">
      <el-button type="primary" size="small" @click="addPoint">添加点</el-button>
      <el-button type="danger" size="small" :disabled="selectedIdx < 0" @click="removeSelectedPoint">删除选中</el-button>
      <el-button size="small" @click="resetPoints">重置网格</el-button>
      <span class="point-count">共 {{ points.length }} 个点</span>
    </div>
    <div ref="wrapperRef" class="canvas-wrapper">
      <canvas
        ref="canvasRef"
        class="point-canvas"
        @mousedown="onMouseDown"
        @mousemove="onMouseMove"
        @mouseup="onMouseUp"
        @mouseleave="onMouseUp"
      />
    </div>
    <div v-if="selectedIdx >= 0" class="selected-info">
      <span>选中点 #{{ selectedIdx + 1 }}</span>
      <span>α = {{ points[selectedIdx]?.alpha.toFixed(2) }}°</span>
      <span>β = {{ points[selectedIdx]?.beta.toFixed(2) }}°</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue'

interface CalibPoint {
  id: string
  alpha: number
  beta: number
}

const props = withDefaults(defineProps<{
  modelValue: CalibPoint[]
  alphaMin?: number
  alphaMax?: number
  betaMin?: number
  betaMax?: number
}>(), {
  alphaMin: -20,
  alphaMax: 20,
  betaMin: -20,
  betaMax: 20,
})

const emit = defineEmits<{
  'update:modelValue': [points: CalibPoint[]]
}>()

const points = ref<CalibPoint[]>([...props.modelValue])
const selectedIdx = ref(-1)
const draggingIdx = ref(-1)

const canvasRef = ref<HTMLCanvasElement>()
const wrapperRef = ref<HTMLDivElement>()

// 坐标映射
const PADDING = 40
let canvasW = 400
let canvasH = 400

function getPlotArea() {
  return {
    x: PADDING,
    y: PADDING,
    w: canvasW - 2 * PADDING,
    h: canvasH - 2 * PADDING,
  }
}

function dataToCanvas(alpha: number, beta: number) {
  const area = getPlotArea()
  const x = area.x + ((alpha - props.alphaMin) / (props.alphaMax - props.alphaMin)) * area.w
  const y = area.y + area.h - ((beta - props.betaMin) / (props.betaMax - props.betaMin)) * area.h
  return { x, y }
}

function canvasToData(cx: number, cy: number) {
  const area = getPlotArea()
  const alpha = props.alphaMin + ((cx - area.x) / area.w) * (props.alphaMax - props.alphaMin)
  const beta = props.betaMin + ((area.y + area.h - cy) / area.h) * (props.betaMax - props.betaMin)
  return { alpha, beta }
}

// 绘制
function draw() {
  const canvas = canvasRef.value
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  if (!ctx) return

  const dpr = window.devicePixelRatio || 1
  canvas.width = canvasW * dpr
  canvas.height = canvasH * dpr
  ctx.scale(dpr, dpr)

  // 背景
  ctx.fillStyle = 'rgba(10, 10, 26, 0.8)'
  ctx.fillRect(0, 0, canvasW, canvasH)

  const area = getPlotArea()

  // 网格
  ctx.strokeStyle = 'rgba(255, 255, 255, 0.06)'
  ctx.lineWidth = 1
  const gridCount = 8
  for (let i = 0; i <= gridCount; i++) {
    const x = area.x + (i / gridCount) * area.w
    const y = area.y + (i / gridCount) * area.h
    ctx.beginPath(); ctx.moveTo(x, area.y); ctx.lineTo(x, area.y + area.h); ctx.stroke()
    ctx.beginPath(); ctx.moveTo(area.x, y); ctx.lineTo(area.x + area.w, y); ctx.stroke()
  }

  // 坐标轴
  ctx.strokeStyle = 'rgba(255, 255, 255, 0.2)'
  ctx.lineWidth = 1
  // α轴 (底部)
  ctx.beginPath(); ctx.moveTo(area.x, area.y + area.h); ctx.lineTo(area.x + area.w, area.y + area.h); ctx.stroke()
  // β轴 (左侧)
  ctx.beginPath(); ctx.moveTo(area.x, area.y); ctx.lineTo(area.x, area.y + area.h); ctx.stroke()

  // 轴标签
  ctx.fillStyle = 'rgba(255, 255, 255, 0.5)'
  ctx.font = '11px sans-serif'
  ctx.textAlign = 'center'
  // α刻度
  const alphaStep = (props.alphaMax - props.alphaMin) / 4
  for (let i = 0; i <= 4; i++) {
    const val = props.alphaMin + i * alphaStep
    const pos = dataToCanvas(val, props.betaMin)
    ctx.fillText(val.toFixed(0) + '°', pos.x, pos.y + 14)
  }
  // β刻度
  ctx.textAlign = 'right'
  const betaStep = (props.betaMax - props.betaMin) / 4
  for (let i = 0; i <= 4; i++) {
    const val = props.betaMin + i * betaStep
    const pos = dataToCanvas(props.alphaMin, val)
    ctx.fillText(val.toFixed(0) + '°', pos.x - 6, pos.y + 4)
  }

  // 轴名称
  ctx.fillStyle = 'rgba(255, 255, 255, 0.6)'
  ctx.font = '12px sans-serif'
  ctx.textAlign = 'center'
  ctx.fillText('α (°)', area.x + area.w / 2, canvasH - 4)
  ctx.save()
  ctx.translate(10, area.y + area.h / 2)
  ctx.rotate(-Math.PI / 2)
  ctx.fillText('β (°)', 0, 0)
  ctx.restore()

  // 绘制校准点
  const POINT_RADIUS = 6
  points.value.forEach((pt, idx) => {
    const { x, y } = dataToCanvas(pt.alpha, pt.beta)
    const isSelected = idx === selectedIdx.value
    const isDragging = idx === draggingIdx.value

    // 发光效果
    if (isSelected || isDragging) {
      const gradient = ctx.createRadialGradient(x, y, 0, x, y, 20)
      gradient.addColorStop(0, 'rgba(184, 41, 255, 0.4)')
      gradient.addColorStop(1, 'rgba(184, 41, 255, 0)')
      ctx.fillStyle = gradient
      ctx.beginPath()
      ctx.arc(x, y, 20, 0, Math.PI * 2)
      ctx.fill()
    }

    // 点
    ctx.beginPath()
    ctx.arc(x, y, POINT_RADIUS, 0, Math.PI * 2)
    if (isSelected) {
      ctx.fillStyle = '#b829ff'
      ctx.strokeStyle = '#d966ff'
    } else {
      ctx.fillStyle = '#00f5ff'
      ctx.strokeStyle = '#66faff'
    }
    ctx.lineWidth = 2
    ctx.fill()
    ctx.stroke()

    // 序号
    ctx.fillStyle = 'rgba(255, 255, 255, 0.8)'
    ctx.font = '9px sans-serif'
    ctx.textAlign = 'center'
    ctx.fillText(`${idx + 1}`, x, y - POINT_RADIUS - 4)
  })
}

// 鼠标交互
function findPointAt(cx: number, cy: number): number {
  const HIT_RADIUS = 12
  for (let i = points.value.length - 1; i >= 0; i--) {
    const { x, y } = dataToCanvas(points.value[i].alpha, points.value[i].beta)
    const dist = Math.sqrt((cx - x) ** 2 + (cy - y) ** 2)
    if (dist <= HIT_RADIUS) return i
  }
  return -1
}

function getCanvasPos(e: MouseEvent) {
  const canvas = canvasRef.value!
  const rect = canvas.getBoundingClientRect()
  return {
    x: e.clientX - rect.left,
    y: e.clientY - rect.top,
  }
}

function onMouseDown(e: MouseEvent) {
  const { x, y } = getCanvasPos(e)
  const idx = findPointAt(x, y)
  if (idx >= 0) {
    selectedIdx.value = idx
    draggingIdx.value = idx
  } else {
    selectedIdx.value = -1
  }
  draw()
}

function onMouseMove(e: MouseEvent) {
  if (draggingIdx.value < 0) return
  const { x, y } = getCanvasPos(e)
  const { alpha, beta } = canvasToData(x, y)

  // 限制在范围内
  const clampedAlpha = Math.max(props.alphaMin, Math.min(props.alphaMax, alpha))
  const clampedBeta = Math.max(props.betaMin, Math.min(props.betaMax, beta))

  points.value[draggingIdx.value].alpha = Math.round(clampedAlpha * 100) / 100
  points.value[draggingIdx.value].beta = Math.round(clampedBeta * 100) / 100
  emitUpdate()
  draw()
}

function onMouseUp() {
  draggingIdx.value = -1
}

function emitUpdate() {
  emit('update:modelValue', [...points.value])
}

// 操作
let nextId = 0

function addPoint() {
  const alpha = (props.alphaMin + props.alphaMax) / 2
  const beta = (props.betaMin + props.betaMax) / 2
  points.value.push({ id: `pt-${nextId++}`, alpha, beta })
  selectedIdx.value = points.value.length - 1
  emitUpdate()
  draw()
}

function removeSelectedPoint() {
  if (selectedIdx.value < 0) return
  points.value.splice(selectedIdx.value, 1)
  selectedIdx.value = -1
  emitUpdate()
  draw()
}

function resetPoints() {
  const steps = 5
  const alphaStep = steps > 1 ? (props.alphaMax - props.alphaMin) / (steps - 1) : 0
  const betaStep = steps > 1 ? (props.betaMax - props.betaMin) / (steps - 1) : 0
  const newPoints: CalibPoint[] = []
  let id = 0
  for (let i = 0; i < steps; i++) {
    for (let j = 0; j < steps; j++) {
      newPoints.push({
        id: `pt-${id++}`,
        alpha: Math.round((props.alphaMin + i * alphaStep) * 100) / 100,
        beta: Math.round((props.betaMin + j * betaStep) * 100) / 100,
      })
    }
  }
  points.value = newPoints
  selectedIdx.value = -1
  nextId = id
  emitUpdate()
  draw()
}

// 监听外部变化
watch(() => props.modelValue, (newVal) => {
  points.value = [...newVal]
  draw()
}, { deep: true })

// 尺寸自适应
function resizeCanvas() {
  if (wrapperRef.value) {
    canvasW = Math.min(wrapperRef.value.clientWidth, 500)
    canvasH = canvasW
    draw()
  }
}

let resizeObserver: ResizeObserver | null = null

onMounted(() => {
  resizeCanvas()
  if (wrapperRef.value) {
    resizeObserver = new ResizeObserver(() => resizeCanvas())
    resizeObserver.observe(wrapperRef.value)
  }
})

onUnmounted(() => {
  resizeObserver?.disconnect()
})
</script>

<style lang="scss" scoped>
.calib-point-editor {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.editor-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
}

.point-count {
  margin-left: auto;
  font-size: 12px;
  color: rgba(255, 255, 255, 0.5);
}

.canvas-wrapper {
  display: flex;
  justify-content: center;
}

.point-canvas {
  cursor: crosshair;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 8px;
}

.selected-info {
  display: flex;
  gap: 12px;
  font-size: 12px;
  color: #b829ff;
  padding: 4px 8px;
  background: rgba(184, 41, 255, 0.1);
  border-radius: 4px;
}
</style>
