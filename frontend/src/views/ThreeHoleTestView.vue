<template>
  <div class="three-hole-test-view">
    <!-- 顶部工具栏 -->
    <div class="toolbar">
      <div class="toolbar-left">
        <el-button type="primary" @click="showSettingsDialog = true" :disabled="store.isRunning">
          <el-icon><Setting /></el-icon> 设置
        </el-button>
        <el-tooltip v-if="!store.isRunning" :content="!store.calibLoaded ? '请先加载校准文件' : ''" :disabled="store.calibLoaded" placement="bottom">
          <el-button type="success" @click="store.startTest()" :disabled="!store.calibLoaded">
            启动测试
          </el-button>
        </el-tooltip>
        <el-button v-if="store.isRunning" type="warning" @click="store.pauseTest()">暂停</el-button>
        <el-button v-if="store.isRunning" type="success" @click="store.resumeTest()">恢复</el-button>
        <el-button v-if="store.isRunning" type="danger" @click="store.stopTest()">停止</el-button>
      </div>
      <div class="toolbar-right">
        <span v-if="store.calibLoaded" class="calib-ok">校准文件已加载 ({{ store.calibFiles.length }})</span>
        <span v-else class="calib-no">未加载校准文件</span>
        <el-button v-if="store.hasResults" type="primary" size="small" @click="store.exportCSV()">导出CSV</el-button>
      </div>
    </div>

    <!-- 错误信息 -->
    <div v-if="store.lastError" class="error-bar">
      <el-alert :title="store.lastError" type="error" :closable="false" show-icon />
    </div>

    <!-- 主内容区 -->
    <div class="main-content">
      <!-- 左侧：布点预览 + 实时数据 -->
      <div class="left-panel">
        <!-- 布点预览图 -->
        <GlassCard title="布点预览" icon="🗺️">
          <div class="point-legend">
            <span class="legend-item"><span class="legend-dot pending"></span>未走</span>
            <span class="legend-item"><span class="legend-dot moving"></span>移动</span>
            <span class="legend-item"><span class="legend-dot acquiring"></span>采集</span>
            <span class="legend-item"><span class="legend-dot waiting"></span>等待</span>
            <span class="legend-item"><span class="legend-dot completed"></span>完成</span>
          </div>
          <canvas ref="pointCanvasRef" class="point-canvas" width="400" height="400"></canvas>
        </GlassCard>

        <!-- 实时数据 -->
        <GlassCard title="实时数据" icon="⚡" style="margin-top: 12px">
          <div class="realtime-panel">
            <div class="section-label">原始压力</div>
            <div class="raw-data">
              <div class="data-item"><span class="label">P1</span><ValueDisplay :value="store.realtime?.rawData.p1" :precision="3" color="#b829ff" /></div>
              <div class="data-item"><span class="label">P2(中心)</span><ValueDisplay :value="store.realtime?.rawData.p2" :precision="3" color="#00f5ff" /></div>
              <div class="data-item"><span class="label">P3</span><ValueDisplay :value="store.realtime?.rawData.p3" :precision="3" color="#00ff88" /></div>
              <div class="data-item"><span class="label">P∞</span><ValueDisplay :value="store.realtime?.rawData.pAtm" :precision="3" /></div>
              <div class="data-item"><span class="label">T∞</span><ValueDisplay :value="store.realtime?.rawData.tAtm" :precision="2" unit="°C" /></div>
            </div>
          </div>

          <!-- 进度 -->
          <div v-if="store.progress" class="progress-section">
            <el-progress :percentage="store.progress.progress" :stroke-width="10" color="#00f5ff" />
            <div class="progress-text">
              {{ store.progress.completedPoints }} / {{ store.progress.totalPoints }}
              (X={{ store.progress.currentX.toFixed(1) }}°, Y={{ store.progress.currentY.toFixed(1) }}°)
            </div>
          </div>
        </GlassCard>
      </div>

      <!-- 右侧：插值结果波形图 -->
      <div class="right-panel">
        <GlassCard title="插值结果波形" icon="📈">
          <div class="wave-charts">
            <div class="wave-chart-item">
              <div class="wave-title">总压 Pt (Pa)</div>
              <ChartPanel :option="ptChartOption" height="180px" />
            </div>
            <div class="wave-chart-item">
              <div class="wave-title">静压 Ps (Pa)</div>
              <ChartPanel :option="psChartOption" height="180px" />
            </div>
            <div class="wave-chart-item">
              <div class="wave-title">马赫数 Ma</div>
              <ChartPanel :option="maChartOption" height="180px" />
            </div>
            <div class="wave-chart-item">
              <div class="wave-title">攻角 α (°)</div>
              <ChartPanel :option="alphaChartOption" height="180px" />
            </div>
          </div>
        </GlassCard>

        <!-- 插值结果数值 -->
        <GlassCard title="插值结果" icon="📊" style="margin-top: 12px">
          <div class="raw-data">
            <div class="data-item"><span class="label">总压 Pt</span><ValueDisplay :value="store.realtime?.interpResult.ptProbe" :precision="3" color="#ffaa00" unit="Pa" /></div>
            <div class="data-item"><span class="label">静压 Ps</span><ValueDisplay :value="store.realtime?.interpResult.psProbe" :precision="3" color="#00ff88" unit="Pa" /></div>
            <div class="data-item"><span class="label">马赫数 Ma</span><ValueDisplay :value="store.realtime?.interpResult.machProbe" :precision="4" color="#b829ff" /></div>
            <div class="data-item"><span class="label">攻角 α</span><ValueDisplay :value="store.realtime?.interpResult.alphaProbe" :precision="2" color="#00f5ff" unit="°" /></div>
            <div class="data-item"><span class="label">迭代次数</span><ValueDisplay :value="store.realtime?.interpResult.iterationCount" :precision="0" color="rgba(255,255,255,0.6)" /></div>
          </div>
        </GlassCard>


      </div>
    </div>

    <!-- ==================== 设置弹窗 ==================== -->
    <el-dialog v-model="showSettingsDialog" title="三孔插值移位测试 - 设置" width="700px" :append-to-body="true" top="5vh">
      <el-tabs>
        <!-- 测试配置 -->
        <el-tab-pane label="测试配置">
          <el-form label-width="90px" size="small">
            <el-form-item label="布点模式">
              <el-select v-model="store.config.layout.pattern" style="width: 100%">
                <el-option label="矩形" value="rectangle" />
                <el-option label="直线" value="line" />
                <el-option label="自定义" value="custom" />
              </el-select>
            </el-form-item>

            <!-- 矩形布点参数 -->
            <template v-if="store.config.layout.pattern === 'rectangle' && store.config.layout.rectangle">
              <el-form-item label="X范围">
                <div style="display:flex;gap:8px;align-items:center">
                  <el-input-number v-model="store.config.layout.rectangle.xMin" :step="5" size="small" style="width:100px" />
                  <span>~</span>
                  <el-input-number v-model="store.config.layout.rectangle.xMax" :step="5" size="small" style="width:100px" />
                </div>
              </el-form-item>
              <el-form-item label="Y范围">
                <div style="display:flex;gap:8px;align-items:center">
                  <el-input-number v-model="store.config.layout.rectangle.yMin" :step="5" size="small" style="width:100px" />
                  <span>~</span>
                  <el-input-number v-model="store.config.layout.rectangle.yMax" :step="5" size="small" style="width:100px" />
                </div>
              </el-form-item>
              <el-form-item label="X步长">
                <el-input-number v-model="xStep" :min="1" :step="1" size="small" style="width:100px" /> °
              </el-form-item>
              <el-form-item label="Y步长">
                <el-input-number v-model="yStep" :min="1" :step="1" size="small" style="width:100px" /> °
              </el-form-item>
            </template>

            <!-- 直线布点参数 -->
            <template v-if="store.config.layout.pattern === 'line' && store.config.layout.line">
              <el-form-item label="起点">
                <div style="display:flex;gap:8px;align-items:center">
                  <el-input-number v-model="store.config.layout.line.startX" :step="5" size="small" style="width:80px" />
                  <el-input-number v-model="store.config.layout.line.startY" :step="5" size="small" style="width:80px" />
                </div>
              </el-form-item>
              <el-form-item label="终点">
                <div style="display:flex;gap:8px;align-items:center">
                  <el-input-number v-model="store.config.layout.line.endX" :step="5" size="small" style="width:80px" />
                  <el-input-number v-model="store.config.layout.line.endY" :step="5" size="small" style="width:80px" />
                </div>
              </el-form-item>
            </template>

            <el-divider content-position="left">硬件参数</el-divider>

            <el-form-item label="X轴">
              <el-select v-model="store.config.motionX.axis" style="width: 80px">
                <el-option label="X" value="X" />
                <el-option label="Y" value="Y" />
              </el-select>
              <span style="margin-left:8px;color:rgba(255,255,255,0.5);font-size:12px">缩放</span>
              <el-input-number v-model="store.config.motionX.scale" :step="0.1" size="small" style="width:80px" />
            </el-form-item>
            <el-form-item label="Y轴">
              <el-select v-model="store.config.motionY.axis" style="width: 80px">
                <el-option label="X" value="X" />
                <el-option label="Y" value="Y" />
              </el-select>
              <span style="margin-left:8px;color:rgba(255,255,255,0.5);font-size:12px">缩放</span>
              <el-input-number v-model="store.config.motionY.scale" :step="0.1" size="small" style="width:80px" />
            </el-form-item>

            <el-divider content-position="left">采集参数</el-divider>

            <el-form-item label="驻留时间">
              <el-input-number v-model="store.config.dwellTimeMs" :min="100" :step="100" size="small" /> ms
            </el-form-item>
            <el-form-item label="采样次数">
              <el-input-number v-model="store.config.samplesPerPoint" :min="1" :max="100" size="small" />
            </el-form-item>
          </el-form>

          <!-- 校准文件 -->
          <div class="calib-section">
            <div class="calib-header">
              <span class="calib-label">校准文件</span>
              <el-button size="small" @click="store.selectCalibFiles()">选择文件</el-button>
            </div>
            <div v-if="store.calibLoaded" class="calib-status loaded">已加载 {{ store.calibFiles.length }} 个校准文件</div>
            <div v-else class="calib-status not-loaded">未加载校准文件</div>
            <div v-if="store.calibFiles.length > 0" class="calib-file-list">
              <div v-for="(f, i) in store.calibFiles" :key="i" class="calib-file-item">{{ f.split(/[/\\]/).pop() }}</div>
            </div>
          </div>
        </el-tab-pane>

        <!-- 通道映射 -->
        <el-tab-pane label="通道映射">
          <el-table :data="store.config.probeChannels" size="small" dark>
            <el-table-column prop="name" label="通道名" width="100" />
            <el-table-column prop="role" label="角色" width="140" />
            <el-table-column label="通道号" width="120">
              <template #default="{ row }">
                <el-input-number v-model="row.channel" :min="0" :max="17" size="small" style="width:100px" />
              </template>
            </el-table-column>
            <el-table-column label="启用" width="80">
              <template #default="{ row }">
                <el-switch v-model="row.enabled" size="small" />
              </template>
            </el-table-column>
          </el-table>
          <div class="channel-hint" style="margin-top:8px;font-size:11px;color:rgba(255,255,255,0.4)">
            通道号对应采集设备的通道索引 (0-15: CH1-CH16, 16: 大气压, 17: 大气温度)
          </div>
        </el-tab-pane>
      </el-tabs>

      <template #footer>
        <el-button @click="showSettingsDialog = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick, shallowRef } from 'vue'
import { Setting } from '@element-plus/icons-vue'
import { useThreeHoleTestStore } from '../stores/threeHoleTest'
import GlassCard from '../components/GlassCard.vue'
import ChartPanel from '../components/ChartPanel.vue'
import ValueDisplay from '../components/ValueDisplay.vue'

const store = useThreeHoleTestStore()

// ==================== 设置弹窗 ====================
const showSettingsDialog = ref(false)


// 步长快捷设置
const xStep = ref(5)
const yStep = ref(5)

// 同步步长配置：当步长或范围变化时，更新配置中的分段步长
watch(
  () => {
    const r = store.config.layout.rectangle
    if (!r) return null
    return { xMin: r.xMin, xMax: r.xMax, yMin: r.yMin, yMax: r.yMax, xs: xStep.value, ys: yStep.value }
  },
  (val) => {
    if (!val) return
    store.config.layout.rectangle!.xSteps = [{ start: val.xMin, end: val.xMax, step: val.xs }]
    store.config.layout.rectangle!.ySteps = [{ start: val.yMin, end: val.yMax, step: val.ys }]
  },
  { immediate: true }
)

// ==================== 布点预览 Canvas ====================
const pointCanvasRef = ref<HTMLCanvasElement>()

// 点位状态枚举
type PointState = 'pending' | 'moving' | 'acquiring' | 'waiting' | 'completed'

// 生成预览点位（直接从范围+步长计算，确保响应式追踪完整）
const previewPoints = computed(() => {
  const layout = store.config.layout
  const points: { x: number; y: number; state: PointState }[] = []

  if (layout.pattern === 'rectangle' && layout.rectangle) {
    const r = layout.rectangle
    // 直接从 xMin/xMax/yMin/yMax + xStep/yStep 计算点位，确保所有依赖都被追踪
    const xValues = expandRange(r.xMin, r.xMax, xStep.value)
    const yValues = expandRange(r.yMin, r.yMax, yStep.value)
    for (const y of yValues) {
      for (const x of xValues) {
        points.push({ x, y, state: 'pending' })
      }
    }
  } else if (layout.pattern === 'line' && layout.line) {
    const l = layout.line
    const xValues = expandSteps(l.xSteps)
    const yValues = expandSteps(l.ySteps)
    const n = Math.max(xValues.length, yValues.length)
    for (let i = 0; i < n; i++) {
      const x = i < xValues.length ? xValues[i] : xValues[xValues.length - 1]
      const y = i < yValues.length ? yValues[i] : yValues[yValues.length - 1]
      points.push({ x, y, state: 'pending' })
    }
  }

  // 根据已完成数据点更新状态
  const completedPoints = store.taskStatus?.dataPoints ?? []
  const currentPoint = store.progress
  const completedSet = new Set(completedPoints.map(p => `${p.x.toFixed(4)},${p.y.toFixed(4)}`))

  for (const pt of points) {
    const key = `${pt.x.toFixed(4)},${pt.y.toFixed(4)}`
    if (completedSet.has(key)) {
      pt.state = 'completed'
    }
  }

  // 标记当前正在处理的点
  if (currentPoint && store.isRunning) {
    const cx = currentPoint.currentX
    const cy = currentPoint.currentY
    for (const pt of points) {
      if (pt.state === 'pending') {
        const dist = Math.sqrt((pt.x - cx) ** 2 + (pt.y - cy) ** 2)
        if (dist < 0.01) {
          pt.state = store.realtime ? 'acquiring' : 'moving'
          break
        }
      }
    }
  }

  return points
})

function expandSteps(steps: { start: number; end: number; step: number }[]): number[] {
  const values: number[] = []
  for (const seg of steps) {
    const step = seg.step || 1
    for (let v = seg.start; v <= seg.end + step * 0.01; v += step) {
      values.push(Math.round(v * 10000) / 10000)
    }
  }
  return values
}

function expandRange(min: number, max: number, step: number): number[] {
  const values: number[] = []
  const s = step || 1
  for (let v = min; v <= max + s * 0.01; v += s) {
    values.push(Math.round(v * 10000) / 10000)
  }
  return values
}

// 绘制布点预览
const CANVAS_W = 400
const CANVAS_H = 400

function drawPointCanvas() {
  const canvas = pointCanvasRef.value
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  if (!ctx) return

  const dpr = window.devicePixelRatio || 1
  const w = CANVAS_W
  const h = CANVAS_H
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

  // 计算范围
  const xs = points.map(p => p.x)
  const ys = points.map(p => p.y)
  const xMin = Math.min(...xs)
  const xMax = Math.max(...xs)
  const yMin = Math.min(...ys)
  const yMax = Math.max(...ys)
  const xRange = xMax - xMin || 1
  const yRange = yMax - yMin || 1

  // 左侧留更多空间给Y轴标签，底部留更多空间给X轴标签
  const padLeft = 55
  const padRight = 20
  const padTop = 20
  const padBottom = 45
  const plotW = w - padLeft - padRight
  const plotH = h - padTop - padBottom

  const toCanvasX = (x: number) => padLeft + ((x - xMin) / xRange) * plotW
  const toCanvasY = (y: number) => padTop + plotH - ((y - yMin) / yRange) * plotH

  // 计算合适的刻度步长（1-2-5 序列），目标约 4-8 个刻度
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

  // 智能格式化刻度值：去除多余小数位
  function formatTick(value: number, step: number): string {
    if (step >= 1) return Math.round(value).toString()
    const decimals = Math.max(0, -Math.floor(Math.log10(step)))
    return value.toFixed(decimals)
  }

  const xStep = niceStep(xRange, 6)
  const yStep = niceStep(yRange, 6)

  // 绘制网格
  ctx.strokeStyle = 'rgba(255,255,255,0.06)'
  ctx.lineWidth = 1
  for (let x = Math.ceil(xMin / xStep) * xStep; x <= xMax + xStep * 0.01; x += xStep) {
    const cx = toCanvasX(x)
    ctx.beginPath()
    ctx.moveTo(cx, padTop)
    ctx.lineTo(cx, h - padBottom)
    ctx.stroke()
  }
  for (let y = Math.ceil(yMin / yStep) * yStep; y <= yMax + yStep * 0.01; y += yStep) {
    const cy = toCanvasY(y)
    ctx.beginPath()
    ctx.moveTo(padLeft, cy)
    ctx.lineTo(w - padRight, cy)
    ctx.stroke()
  }

  // 绘制坐标轴边框线
  ctx.strokeStyle = 'rgba(255,255,255,0.15)'
  ctx.lineWidth = 1
  ctx.beginPath()
  ctx.moveTo(padLeft, padTop)
  ctx.lineTo(padLeft, h - padBottom)
  ctx.lineTo(w - padRight, h - padBottom)
  ctx.stroke()

  // 绘制坐标轴标签
  ctx.fillStyle = 'rgba(255,255,255,0.55)'
  ctx.font = '11px sans-serif'
  ctx.textAlign = 'center'
  ctx.textBaseline = 'top'
  for (let x = Math.ceil(xMin / xStep) * xStep; x <= xMax + xStep * 0.01; x += xStep) {
    ctx.fillText(formatTick(x, xStep), toCanvasX(x), h - padBottom + 6)
  }
  ctx.textAlign = 'right'
  ctx.textBaseline = 'middle'
  for (let y = Math.ceil(yMin / yStep) * yStep; y <= yMax + yStep * 0.01; y += yStep) {
    ctx.fillText(formatTick(y, yStep), padLeft - 8, toCanvasY(y))
  }

  // 坐标轴标题
  ctx.fillStyle = 'rgba(255,255,255,0.45)'
  ctx.font = '12px sans-serif'
  ctx.textAlign = 'center'
  ctx.textBaseline = 'top'
  ctx.fillText('X (mm)', padLeft + plotW / 2, h - padBottom + 22)
  ctx.save()
  ctx.translate(14, padTop + plotH / 2)
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

  // 绘制点位
  const pointRadius = Math.max(3, Math.min(8, 200 / Math.sqrt(points.length)))
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

    // 所有状态都绘制边框，提升点的可辨识度
    ctx.strokeStyle = border
    ctx.lineWidth = 1
    ctx.stroke()

    ctx.shadowColor = 'transparent'
    ctx.shadowBlur = 0
  }
}

// 监听布点配置或点位变化重绘
watch(
  [previewPoints, pointCanvasRef, () => store.config.layout],
  () => {
    nextTick(drawPointCanvas)
  },
  { deep: true }
)

// ==================== 插值结果波形图 ====================
const MAX_WAVE_POINTS = 200

// 各参数的历史数据
const ptHistory = ref<number[]>([])
const psHistory = ref<number[]>([])
const maHistory = ref<number[]>([])
const alphaHistory = ref<number[]>([])
const waveLabels = ref<number[]>([])

// 监听实时数据追加波形
watch(() => store.realtime, (rt) => {
  if (!rt) return
  ptHistory.value.push(rt.interpResult.ptProbe)
  psHistory.value.push(rt.interpResult.psProbe)
  maHistory.value.push(rt.interpResult.machProbe)
  alphaHistory.value.push(rt.interpResult.alphaProbe)
  waveLabels.value.push(waveLabels.value.length + 1)

  if (ptHistory.value.length > MAX_WAVE_POINTS) {
    ptHistory.value.shift()
    psHistory.value.shift()
    maHistory.value.shift()
    alphaHistory.value.shift()
    waveLabels.value.shift()
  }

  scheduleWaveUpdate()
})

function makeWaveOption(data: number[], color: string, unit?: string) {
  return {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      backgroundColor: 'rgba(10,10,26,0.9)',
      borderColor: `${color}44`,
      textStyle: { color: '#fff', fontSize: 11 },
    },
    grid: { left: 50, right: 10, top: 8, bottom: 24 },
    xAxis: {
      type: 'category',
      data: waveLabels.value,
      axisLine: { lineStyle: { color: 'rgba(255,255,255,0.1)' } },
      axisLabel: { color: 'rgba(255,255,255,0.3)', fontSize: 9 },
    },
    yAxis: {
      type: 'value',
      axisLine: { lineStyle: { color: 'rgba(255,255,255,0.1)' } },
      axisLabel: { color: 'rgba(255,255,255,0.3)', fontSize: 9 },
      splitLine: { lineStyle: { color: 'rgba(255,255,255,0.04)' } },
    },
    series: [{
      type: 'line',
      data,
      smooth: true,
      symbol: 'none',
      lineStyle: { width: 2, color, shadowColor: color, shadowBlur: 4 },
      itemStyle: { color },
    }],
  }
}

// 使用 shallowRef + 节流更新，避免 computed 每次数据变化都重建完整 option
const ptChartOption = shallowRef(makeWaveOption([], '#ffaa00', 'Pa'))
const psChartOption = shallowRef(makeWaveOption([], '#00ff88', 'Pa'))
const maChartOption = shallowRef(makeWaveOption([], '#b829ff'))
const alphaChartOption = shallowRef(makeWaveOption([], '#00f5ff', '°'))

let waveUpdateTimer: number | null = null
let waveDirty = false

function scheduleWaveUpdate() {
  waveDirty = true
  if (waveUpdateTimer) return
  waveUpdateTimer = window.setTimeout(() => {
    waveUpdateTimer = null
    if (!waveDirty) return
    waveDirty = false
    // 增量更新：只更新数据和 xAxis，不重建 tooltip/grid/yAxis 等静态配置
    const labels = waveLabels.value
    ptChartOption.value = {
      ...ptChartOption.value,
      xAxis: { ...ptChartOption.value.xAxis, data: labels },
      series: [{ ...ptChartOption.value.series[0], data: ptHistory.value }]
    }
    psChartOption.value = {
      ...psChartOption.value,
      xAxis: { ...psChartOption.value.xAxis, data: labels },
      series: [{ ...psChartOption.value.series[0], data: psHistory.value }]
    }
    maChartOption.value = {
      ...maChartOption.value,
      xAxis: { ...maChartOption.value.xAxis, data: labels },
      series: [{ ...maChartOption.value.series[0], data: maHistory.value }]
    }
    alphaChartOption.value = {
      ...alphaChartOption.value,
      xAxis: { ...alphaChartOption.value.xAxis, data: labels },
      series: [{ ...alphaChartOption.value.series[0], data: alphaHistory.value }]
    }
  }, 200)
}


onMounted(() => {
  store.startListening()
  nextTick(drawPointCanvas)
})
</script>

<style lang="scss" scoped>
.three-hole-test-view {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  background: rgba(255,255,255,0.04);
  border-radius: 8px;
  border: 1px solid rgba(255,255,255,0.08);
}

.toolbar-left, .toolbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.calib-ok { font-size: 12px; color: #00ff88; }
.calib-no { font-size: 12px; color: rgba(255,255,255,0.4); }

.error-bar { margin: 0; }

.main-content {
  display: flex;
  gap: 12px;
}

.left-panel {
  flex: 0 0 420px;
  display: flex;
  flex-direction: column;
}

.right-panel {
  flex: 1;
  display: flex;
  flex-direction: column;
}

// 布点预览
.point-legend {
  display: flex;
  gap: 12px;
  margin-bottom: 8px;
  flex-wrap: wrap;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  color: rgba(255,255,255,0.6);
}

.legend-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  display: inline-block;

  &.pending { background: rgba(255,255,255,0.35); border: 1px solid rgba(255,255,255,0.2); }
  &.moving { background: #ffaa00; box-shadow: 0 0 4px rgba(255,170,0,0.4); }
  &.acquiring { background: #00f5ff; box-shadow: 0 0 4px rgba(0,245,255,0.4); }
  &.waiting { background: #ff3366; box-shadow: 0 0 4px rgba(255,51,102,0.4); }
  &.completed { background: #00ff88; box-shadow: 0 0 4px rgba(0,255,136,0.3); }
}

.point-canvas {
  width: 400px;
  height: 400px;
  border-radius: 6px;
  background: rgba(0,0,0,0.2);
}

// 实时数据
.realtime-panel {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.section-label {
  font-size: 11px;
  color: rgba(255,255,255,0.4);
  text-transform: uppercase;
  letter-spacing: 1px;
}

.raw-data, .interp-results {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}


.data-item {
  background: rgba(255,255,255,0.04);
  border-radius: 6px;
  padding: 6px 10px;
  display: flex;
  flex-direction: column;
  align-items: center;
  min-width: 80px;
}

.data-item .label {
  font-size: 10px;
  color: rgba(255,255,255,0.5);
  margin-bottom: 2px;
}

.no-data {
  color: rgba(255,255,255,0.3);
  text-align: center;
  padding: 20px;
}

.progress-section { margin-top: 12px; }
.progress-text {
  font-size: 12px;
  color: rgba(255,255,255,0.6);
  margin-top: 4px;
}

// 波形图
.wave-charts {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}

.wave-chart-item {
  background: rgba(255,255,255,0.02);
  border-radius: 6px;
  padding: 6px;
}

.wave-title {
  font-size: 11px;
  color: rgba(255,255,255,0.5);
  margin-bottom: 4px;
}

// 设置弹窗
.calib-section {
  margin-top: 12px;
  padding: 8px;
  background: rgba(255,255,255,0.04);
  border-radius: 6px;
}

.calib-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 6px;
}

.calib-label {
  font-size: 13px;
  font-weight: 500;
  color: rgba(255,255,255,0.8);
}

.calib-status {
  font-size: 12px;
  margin-bottom: 4px;
  &.loaded { color: #00ff88; }
  &.not-loaded { color: rgba(255,255,255,0.4); }
}

.calib-file-list { max-height: 80px; overflow-y: auto; }
.calib-file-item {
  font-size: 11px;
  color: rgba(255,255,255,0.5);
  padding: 2px 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>
