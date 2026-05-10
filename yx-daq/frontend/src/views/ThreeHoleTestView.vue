<template>
  <div class="three-hole-test-view">
    <!-- 顶部工具栏 -->
    <div class="toolbar">
      <div class="toolbar-left">
        <el-button type="primary" :disabled="store.isRunning" @click="showSettingsDialog = true">
          <el-icon><Setting /></el-icon> 设置
        </el-button>
        <el-tooltip v-if="!store.isRunning" :content="!store.calibLoaded ? '请先加载校准文件' : ''" :disabled="store.calibLoaded" placement="bottom">
          <el-button type="success" :disabled="!store.calibLoaded" @click="store.startTest()">
            启动测试
          </el-button>
        </el-tooltip>
        <el-button v-if="store.isRunning && !store.isPaused" type="warning" @click="store.pauseTest()">暂停</el-button>
        <el-button v-if="store.isPaused" type="success" @click="store.resumeTest()">恢复</el-button>
        <el-button v-if="store.isRunning" type="danger" @click="store.stopTest()">停止</el-button>
      </div>
      <!-- 测试进度条 -->
      <div v-if="store.isRunning && store.progress" class="toolbar-progress">
        <el-progress
          :percentage="Math.round(store.progress.progress)"
          :stroke-width="10"
          color="#00f5ff"
        />
        <div class="test-progress-meta">
          <span class="meta-item">{{ store.progress.completedPoints }} / {{ store.progress.totalPoints }} 点</span>
          <span class="meta-item phase-badge" :class="store.progress.phase || 'acquiring'">{{ phaseLabel }}</span>
          <span class="meta-item">X={{ store.progress.currentX.toFixed(1) }}°  Y={{ store.progress.currentY.toFixed(1) }}°</span>
        </div>
      </div>
      <div class="toolbar-right">
        <span v-if="store.calibLoaded" class="calib-ok">校准文件已加载 ({{ store.calibFiles.length }})</span>
        <span v-else class="calib-no">未加载校准文件</span>
        <el-button v-if="store.hasResults" type="primary" size="small" @click="store.exportCSV()">导出CSV</el-button>
      </div>
    </div>

    <!-- 错误信息 -->
    <div v-if="store.lastError" class="error-bar">
      <el-alert :title="store.lastError" type="error" :closable="true" show-icon @close="store.clearError()" />
    </div>

    <!-- 主内容区 -->
    <div class="main-content">
      <!-- 左侧：布点预览 + 实时数据 -->
      <div class="left-panel">
        <!-- 布点预览图 -->
        <GlassCard title="布点预览" icon="🗺️">
          <div class="point-legend">
            <span class="legend-item"><span class="legend-dot pending" />待测</span>
            <span class="legend-item"><span class="legend-dot moving" />移动</span>
            <span class="legend-item"><span class="legend-dot acquiring" />采集</span>
            <span class="legend-item"><span class="legend-dot waiting" />等待</span>
            <span class="legend-item"><span class="legend-dot completed" />完成</span>
          </div>
          <canvas ref="pointCanvasRef" class="point-canvas" width="400" height="400" />
          <div v-if="previewPoints.length > 10000" class="point-count-warning">
            ⚠ 布点数量 {{ previewPoints.length }} 较大，测试耗时可能很长
          </div>
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
            <div class="data-item"><span class="label">速度 V</span><ValueDisplay :value="store.realtime?.interpResult.velocityProbe" :precision="2" color="#ff6b6b" unit="m/s" /></div>
            <div class="data-item"><span class="label">迭代次数</span><ValueDisplay :value="store.realtime?.interpResult.iterationCount" :precision="0" color="rgba(255,255,255,0.6)" /></div>
          </div>
        </GlassCard>
      </div>
    </div>

    <!-- ==================== 设置弹窗 ==================== -->
    <el-dialog v-model="showSettingsDialog" title="测试设置" width="600px" :append-to-body="true" top="5vh" class="settings-dialog">
      <el-tabs>
        <!-- 测试配置 -->
        <el-tab-pane label="测试配置">
          <div class="settings-section">
            <div class="section-title">📍 布点配置</div>
            <el-form label-width="70px" size="small" class="compact-form">
              <el-form-item label="布点模式">
                <el-select v-model="store.config.layout.pattern" style="width: 160px">
                  <el-option
                    v-for="(label, key) in TraversalPatternLabels"
                    :key="key"
                    :label="label"
                    :value="key"
                  />
                </el-select>
              </el-form-item>
            </el-form>

            <!-- 矩形布点参数 -->
            <template v-if="store.config.layout.pattern === TraversalPattern.RECTANGLE && store.config.layout.rectangle">
              <div class="form-row">
                <div class="form-group">
                  <label class="group-label">X范围</label>
                  <div class="range-inputs">
                    <el-input-number v-model="store.config.layout.rectangle.xMin" :step="5" size="small" style="width:90px" />
                    <span class="range-separator">~</span>
                    <el-input-number v-model="store.config.layout.rectangle.xMax" :step="5" size="small" style="width:90px" />
                  </div>
                </div>
                <div class="form-group">
                  <label class="group-label">Y范围</label>
                  <div class="range-inputs">
                    <el-input-number v-model="store.config.layout.rectangle.yMin" :step="5" size="small" style="width:90px" />
                    <span class="range-separator">~</span>
                    <el-input-number v-model="store.config.layout.rectangle.yMax" :step="5" size="small" style="width:90px" />
                  </div>
                </div>
              </div>
              <div class="form-row">
                <div class="form-group">
                  <label class="group-label">X步长</label>
                  <el-input-number v-model="xStep" :min="1" :step="1" size="small" style="width:100px" />
                </div>
                <div class="form-group">
                  <label class="group-label">Y步长</label>
                  <el-input-number v-model="yStep" :min="1" :step="1" size="small" style="width:100px" />
                </div>
              </div>
            </template>

            <!-- 直线布点参数 -->
            <template v-if="store.config.layout.pattern === TraversalPattern.LINE && store.config.layout.line">
              <div class="form-row">
                <div class="form-group">
                  <label class="group-label">起点</label>
                  <div class="point-inputs">
                    <el-input-number v-model="store.config.layout.line.startX" :step="5" size="small" style="width:90px" />
                    <el-input-number v-model="store.config.layout.line.startY" :step="5" size="small" style="width:90px" />
                  </div>
                </div>
                <div class="form-group">
                  <label class="group-label">终点</label>
                  <div class="point-inputs">
                    <el-input-number v-model="store.config.layout.line.endX" :step="5" size="small" style="width:90px" />
                    <el-input-number v-model="store.config.layout.line.endY" :step="5" size="small" style="width:90px" />
                  </div>
                </div>
              </div>
              <div class="form-row">
                <div class="form-group">
                  <label class="group-label">X步长</label>
                  <el-input-number v-model="lineXStep" :min="1" :step="1" size="small" style="width:100px" />
                </div>
                <div class="form-group">
                  <label class="group-label">Y步长</label>
                  <el-input-number v-model="lineYStep" :min="1" :step="1" size="small" style="width:100px" />
                </div>
              </div>
            </template>
          </div>

          <div class="settings-section">
            <div class="section-title">⚙️ 硬件参数</div>
            <div class="form-row">
              <div class="form-group">
                <label class="group-label">α方向</label>
                <div class="axis-inputs">
                  <el-select v-model="store.config.motionAlpha.axis" style="width: 60px">
                    <el-option v-for="axis in axisOptions" :key="axis" :label="axis" :value="axis" />
                  </el-select>
                </div>
              </div>
              <div class="form-group">
                <label class="group-label">β方向</label>
                <div class="axis-inputs">
                  <el-select v-model="store.config.motionBeta.axis" style="width: 60px">
                    <el-option v-for="axis in axisOptions" :key="axis" :label="axis" :value="axis" />
                  </el-select>
                </div>
              </div>
            </div>
          </div>

          <div class="settings-section">
            <div class="section-title">📊 采集参数</div>
            <div class="form-row">
              <div class="form-group">
                <label class="group-label">驻留时间</label>
                <el-input-number v-model="store.config.dwellTimeMs" :min="100" :step="100" size="small" style="width:100px" />
                <span class="unit-label">ms</span>
              </div>
              <div class="form-group">
                <label class="group-label">采样次数</label>
                <el-input-number v-model="store.config.samplesPerPoint" :min="1" :max="100" size="small" style="width:100px" />
              </div>
              <div class="form-group">
                <label class="group-label">采样间隔</label>
                <el-input-number v-model="store.config.sampleIntervalMs" :min="10" :step="10" size="small" style="width:100px" />
                <span class="unit-label">ms</span>
              </div>
              <div class="form-group">
                <label class="group-label">运动超时</label>
                <el-input-number v-model="store.config.motionTimeoutMs" :min="1000" :step="1000" size="small" style="width:100px" />
                <span class="unit-label">ms</span>
                <div class="form-hint">轴移动等待超时时间</div>
              </div>
            </div>
            <div class="form-row" style="margin-top: 8px">
              <div class="form-group" style="flex: 1">
                <label class="group-label">保存路径</label>
                <el-input v-model="store.config.savePath" placeholder="默认 ~/.yx-daq/recordings/" size="small" clearable>
                  <template #append>
                    <el-button :icon="FolderOpened" @click="browseSavePath" />
                  </template>
                </el-input>
              </div>
              <div class="form-group" style="flex: 1">
                <label class="group-label">文件名</label>
                <el-input v-model="store.config.saveFileName" placeholder="ThreeHoleTraversal-xxx.csv" size="small" clearable />
              </div>
            </div>
          </div>

          <!-- 校准文件 -->
          <div class="calib-section">
            <div class="calib-header">
              <div class="calib-title">
                <span class="calib-icon">📄</span>
                <span class="calib-label">校准文件</span>
              </div>
              <el-button size="small" type="primary" plain @click="store.selectCalibFiles()">选择文件</el-button>
            </div>
            <div v-if="store.calibLoaded" class="calib-status loaded">
              <span class="status-dot" />已加载 {{ store.calibFiles.length }} 个文件
            </div>
            <div v-else class="calib-status not-loaded">
              <span class="status-dot" />未加载校准文件
            </div>
            <div v-if="store.calibFiles.length > 0" class="calib-file-list">
              <div v-for="(f, i) in store.calibFiles" :key="i" class="calib-file-item" :title="f">{{ f.split(/[/\\]/).pop() }}</div>
            </div>
          </div>
        </el-tab-pane>

        <!-- 通道映射 -->
        <el-tab-pane label="通道映射">
          <div class="settings-section">
            <div class="section-title">🔌 设备选择</div>
            <div class="form-row device-row">
              <div class="form-group device-group">
                <label class="group-label">采集设备</label>
                <el-select v-model="store.config.deviceId" placeholder="请选择采集设备" style="width: 220px" clearable size="small">
                  <el-option
                    v-for="dev in deviceStore.profiles"
                    :key="dev.id"
                    :label="`${dev.name} (${dev.type})`"
                    :value="dev.id"
                  />
                </el-select>
              </div>
              <div class="form-group device-group">
                <label class="group-label">运动控制器</label>
                <el-select v-model="store.config.motionControllerId" placeholder="请选择运动控制器" style="width: 220px" clearable size="small">
                  <el-option
                    v-for="mc in motionStore.profiles"
                    :key="mc.id"
                    :label="`${mc.name} (${mc.type})`"
                    :value="mc.id"
                  />
                </el-select>
              </div>
            </div>
          </div>

          <div class="settings-section">
            <div class="section-title">📋 通道映射</div>
            <el-table :data="store.config.probeChannels" size="small" class="channel-table" :header-cell-style="{background:'rgba(255,255,255,0.05)'}">
              <el-table-column prop="name" label="通道" width="90" />
              <el-table-column label="角色" width="130">
                <template #default="{ row }">
                  {{ ThreeHoleChannelRoleLabels[row.role as keyof typeof ThreeHoleChannelRoleLabels] || row.role }}
                </template>
              </el-table-column>
              <el-table-column label="通道号" width="110">
                <template #default="{ row }">
                  <el-input-number v-model="row.channel" :min="0" :max="maxChannelIndex" size="small" style="width:85px" controls-position="right" />
                </template>
              </el-table-column>
              <el-table-column label="启用" width="70" align="center">
                <template #default="{ row }">
                  <el-switch v-model="row.enabled" size="small" />
                </template>
              </el-table-column>
            </el-table>
            <div class="channel-hint">
              通道号范围：0 ~ {{ maxChannelIndex }} ({{ selectedDeviceTypeName }})
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>

      <template #footer>
        <el-button type="primary" @click="store.saveConfig(); showSettingsDialog = false">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick, shallowRef } from 'vue'
import { Setting, FolderOpened } from '@element-plus/icons-vue'
import { useDeviceStore } from '../stores/device'
import { useMotionStore } from '../stores/motion'
import { useThreeHoleTestStore } from '../stores/threeHoleTest'
import { SelectDataSavePath } from '../../wailsjs/go/main/App'
import {
  TraversalPattern,
  TraversalPatternLabels,
  ThreeHoleChannelRoleLabels,
  DeviceTypeLabels,
  AxisName,
  getTotalChannelCount,
  type TraversalPatternValue,
} from '../api/enums'
import ChartPanel from '../components/ChartPanel.vue'
import GlassCard from '../components/GlassCard.vue'
import ValueDisplay from '../components/ValueDisplay.vue'

const store = useThreeHoleTestStore()
const deviceStore = useDeviceStore()
const motionStore = useMotionStore()

async function browseSavePath() {
  try {
    const dir = await SelectDataSavePath() as string
    if (dir) {
      store.config.savePath = dir
    }
  } catch (e) {
    console.error('browseSavePath failed:', e)
  }
}

// ==================== 设置弹窗 ====================
const showSettingsDialog = ref(false)

// 当前选中设备的类型
const selectedDeviceType = computed(() => {
  if (!store.config.deviceId) return ''
  const profile = deviceStore.profiles.find(p => p.id === store.config.deviceId)
  return profile?.type || ''
})

// 当前选中设备名称
const selectedDeviceTypeName = computed(() => {
  if (!selectedDeviceType.value) return '未选择'
  return DeviceTypeLabels[selectedDeviceType.value as keyof typeof DeviceTypeLabels] || selectedDeviceType.value
})

// 通道号最大值（根据设备类型动态调整）
const maxChannelIndex = computed(() => {
  if (!selectedDeviceType.value) return 17 // 默认 DAQ16
  return getTotalChannelCount(selectedDeviceType.value as any) - 1
})

// 运动轴选项（从已连接的运动控制器获取可用轴）
const axisOptions = computed(() => {
  const mcId = store.config.motionControllerId
  if (!mcId) return [AxisName.X, AxisName.Y]
  const profile = motionStore.profiles.find(p => p.id === mcId)
  if (profile?.axes?.length) {
    return profile.axes.filter(a => a.enabled).map(a => a.name)
  }
  return [AxisName.X, AxisName.Y]
})


// 阶段标签
const phaseLabel = computed(() => {
  const map: Record<string, string> = {
    starting: '启动中',
    moving: '移动中',
    waiting: '等待中',
    acquiring: '采集中',
    acquired: '已采集',
  }
  return map[store.progress?.phase || ''] || '采集中'
})

// 步长快捷设置
const xStep = ref(5)
const yStep = ref(5)
const lineXStep = ref(5)
const lineYStep = ref(5)

// 同步矩形步长配置
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

// 同步直线步长配置
watch(
  () => {
    const l = store.config.layout.line
    if (!l) return null
    return { startX: l.startX, startY: l.startY, endX: l.endX, endY: l.endY, xs: lineXStep.value, ys: lineYStep.value }
  },
  (val) => {
    if (!val) return
    const l = store.config.layout.line
    if (!l) return
    l.xSteps = [{ start: l.startX, end: l.endX, step: val.xs }]
    l.ySteps = [{ start: l.startY, end: l.endY, step: val.ys }]
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
    // 使用 expandSteps 展开多分段步长，与后端 expandStepSegments 行为一致
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
    const xValues = expandSteps(l.xSteps)
    const yValues = expandSteps(l.ySteps)
    if (xValues.length === 0 && yValues.length === 0) {
      points.push({ x: l.startX, y: l.startY, state: 'pending' })
      points.push({ x: l.endX, y: l.endY, state: 'pending' })
    } else {
      if (yValues.length === 0) { yValues.push(l.startY) }
      if (xValues.length === 0) { xValues.push(l.startX) }
      for (const x of xValues) {
        for (const y of yValues) {
          points.push({ x, y, state: 'pending' })
        }
      }
    }
  }

  // 从进度事件获取已完成点数（运行时可靠更新，无需等待 complete 事件）
  const completedCount = store.progress?.completedPoints ?? 0

  // 按序号标记已完成点（与后端遍历顺序一致）
  for (let i = 0; i < points.length && i < completedCount; i++) {
    points[i].state = 'completed'
  }

  // 标记当前正在处理的点（按索引定位，与后端遍历顺序一致）
  const currentPoint = store.progress
  if (currentPoint && store.isRunning && completedCount < points.length) {
    const phase = currentPoint.phase
    points[completedCount].state = (phase === 'moving' || phase === 'waiting' || phase === 'acquiring') ? phase : 'acquiring'
  }

  return points
})

function expandSteps(steps: { start: number; end: number; step: number }[]): number[] {
  const values: number[] = []
  for (const seg of steps) {
    if (seg.step === 0) {
      values.push(seg.start, seg.end)
      continue
    }
    if (seg.start > seg.end || seg.step < 0) continue
    // 使用整数步数计算，避免浮点累加精度问题（与后端 expandStepSegments 一致）
    const n = Math.round((seg.end - seg.start) / seg.step)
    for (let i = 0; i <= n; i++) {
      values.push(Math.round((seg.start + i * seg.step) * 10000) / 10000)
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

  // 更紧凑的边距设置，给绘图区域留出更多空间
  const padLeft = 48
  const padRight = 16
  const padTop = 16
  const padBottom = 40
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
  ctx.font = '10px sans-serif'
  ctx.textAlign = 'center'
  ctx.textBaseline = 'top'
  for (let x = Math.ceil(xMin / xStep) * xStep; x <= xMax + xStep * 0.01; x += xStep) {
    ctx.fillText(formatTick(x, xStep), toCanvasX(x), h - padBottom + 5)
  }
  ctx.textAlign = 'right'
  ctx.textBaseline = 'middle'
  for (let y = Math.ceil(yMin / yStep) * yStep; y <= yMax + yStep * 0.01; y += yStep) {
    ctx.fillText(formatTick(y, yStep), padLeft - 6, toCanvasY(y))
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

  // 绘制点位 - 使用更小的点确保网格密集时也能清晰显示
  const pointRadius = Math.max(2.5, Math.min(5, 140 / Math.sqrt(points.length)))
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
const waveLabels = ref<string[]>([])

// 记录已追加到波形的pointId，防止同一布点重复追加
let lastWavePointId = ''

// 监听实时数据追加波形
watch(() => store.realtime, (rt) => {
  if (!rt) return

  // 测试运行时且未暂停：每个布点添加一次，使用实际坐标作为标签
  if (store.isRunning && !store.isPaused) {
    // 同一个布点只追加一次波形数据
    if (rt.pointId && rt.pointId === lastWavePointId) return
    lastWavePointId = rt.pointId || ''

    // 添加数据，使用点位坐标作为标签
    ptHistory.value.push(rt.interpResult.ptProbe)
    psHistory.value.push(rt.interpResult.psProbe)
    maHistory.value.push(rt.interpResult.machProbe)
    alphaHistory.value.push(rt.interpResult.alphaProbe)
    // 使用点位坐标作为x轴标签
    waveLabels.value.push(rt.pointId || '')

    // 限制数据点数量
    if (ptHistory.value.length > MAX_WAVE_POINTS) {
      ptHistory.value.shift()
      psHistory.value.shift()
      maHistory.value.shift()
      alphaHistory.value.shift()
      waveLabels.value.shift()
    }

    scheduleWaveUpdate()
  } else {
    // 测试未运行时，重置lastWavePointId，避免下次启动时数据重复
    if (rt.pointId && rt.pointId !== lastWavePointId) {
      lastWavePointId = ''
    }
  }
})

// 测试状态变化时的处理
watch(() => store.isRunning, (running) => {
  if (!running) {
    // 测试停止时清除历史数据，避免下次启动时数据重复
    ptHistory.value = []
    psHistory.value = []
    maHistory.value = []
    alphaHistory.value = []
    waveLabels.value = []
    lastWavePointId = ''
  } else {
    // 测试开始时清除历史数据
    ptHistory.value = []
    psHistory.value = []
    maHistory.value = []
    alphaHistory.value = []
    waveLabels.value = []
    lastWavePointId = ''
  }
})

function makeWaveOption(data: number[], color: string, unit?: string) {
  return {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      backgroundColor: 'rgba(10,10,26,0.9)',
      borderColor: `${color}44`,
      textStyle: { color: '#fff', fontSize: 11 },
      formatter: function(params: any) {
        const dataIndex = params[0].dataIndex
        const label = waveLabels.value[dataIndex]
        const value = params[0].value
        return `${label}<br/>${unit ? value.toFixed(3) + unit : value.toFixed(4)}`
      }
    },
    grid: { left: 50, right: 10, top: 8, bottom: 40 },
    xAxis: {
      type: 'category',
      data: waveLabels.value,
      axisLine: { lineStyle: { color: 'rgba(255,255,255,0.1)' } },
      axisLabel: {
        color: 'rgba(255,255,255,0.3)',
        fontSize: 10,
        rotate: 45,
      },
    },
    yAxis: {
      type: 'value',
      scale: true,
      axisLine: { lineStyle: { color: 'rgba(255,255,255,0.1)' } },
      axisLabel: {
        color: 'rgba(255,255,255,0.3)',
        fontSize: 9,
        formatter: unit ? `{value} ${unit}` : '{value}'
      },
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

    // 批量更新所有图表选项
    const labels = waveLabels.value
    const updateChart = (chartOption: any, data: number[]) => {
      return {
        ...chartOption.value,
        xAxis: { ...chartOption.value.xAxis, data: labels },
        series: [{ ...chartOption.value.series[0], data: data }]
      }
    }

    // 同时更新所有图表
    ptChartOption.value = updateChart(ptChartOption, ptHistory.value)
    psChartOption.value = updateChart(psChartOption, psHistory.value)
    maChartOption.value = updateChart(maChartOption, maHistory.value)
    alphaChartOption.value = updateChart(alphaChartOption, alphaHistory.value)
  }, 100) // 减少延迟到100ms，提高响应速度
}


onMounted(() => {
  store.startListening()
  store.loadConfig()
  store.startRealtimeMonitor()
  deviceStore.fetchProfiles()
  deviceStore.fetchStatuses()
  motionStore.fetchProfiles()
  motionStore.fetchStatuses()
  nextTick(drawPointCanvas)
})

onUnmounted(() => {
  store.stopListening()
  store.stopRealtimeMonitor()
})

// 配置变更时自动保存（防抖500ms，避免输入时频繁写后端）
// 同时重启实时监控以使用最新配置
let configSaveTimer: number | null = null
watch(() => store.config, () => {
  if (configSaveTimer) clearTimeout(configSaveTimer)
  configSaveTimer = window.setTimeout(() => {
    store.saveConfig()
    // 重启实时监控以应用最新配置（通道映射等）
    if (!store.isRunning) {
      store.stopRealtimeMonitor()
      store.startRealtimeMonitor()
    }
  }, 500)
}, { deep: true })
</script>

<style lang="scss" scoped>
.three-hole-test-view {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  background: rgba(255,255,255,0.04);
  border-radius: 8px;
  border: 1px solid rgba(255,255,255,0.08);
  gap: 12px;
}

.toolbar-left, .toolbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

// 工具栏中的进度条
.toolbar-progress {
  flex: 1;
  max-width: 400px;
  background: rgba(0, 0, 0, 0.2);
  border: 1px solid rgba(255,255,255,0.06);
  border-radius: 6px;
  padding: 6px 10px;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.toolbar-progress :deep(.el-progress-bar__outer) {
  border-radius: 3px;
  background-color: rgba(255,255,255,0.06) !important;
}

.toolbar-progress :deep(.el-progress__text) {
  font-size: 11px;
  color: #fff;
  min-width: 32px;
}

.calib-ok { font-size: 12px; color: #00ff88; }
.calib-no { font-size: 12px; color: rgba(255,255,255,0.4); }

.error-bar { margin: 0; }

.test-progress-meta {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 11px;
  color: rgba(255,255,255,0.5);
}

.test-progress-meta .meta-item {
  display: flex;
  align-items: center;
  gap: 4px;
}

.phase-badge {
  padding: 1px 6px;
  border-radius: 3px;
  font-size: 10px;
  font-weight: 600;
  color: #fff;
}
.phase-badge.moving   { background: rgba(255,170,0,0.35); color: #ffcc66; }
.phase-badge.waiting  { background: rgba(255,51,102,0.35); color: #ff7799; }
.phase-badge.acquiring{ background: rgba(0,245,255,0.25); color: #66f5ff; }

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
  gap: 10px;
  margin-bottom: 6px;
  flex-wrap: wrap;
}

.point-count-warning {
  margin-top: 6px;
  font-size: 11px;
  color: #ff9800;
  text-align: center;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 3px;
  font-size: 10px;
  color: rgba(255,255,255,0.55);
}

.legend-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  display: inline-block;

  &.pending { background: rgba(255,255,255,0.35); border: 1px solid rgba(255,255,255,0.2); }
  &.moving { background: #ffaa00; box-shadow: 0 0 3px rgba(255,170,0,0.5); }
  &.acquiring { background: #00f5ff; box-shadow: 0 0 3px rgba(0,245,255,0.5); }
  &.waiting { background: #ff3366; box-shadow: 0 0 3px rgba(255,51,102,0.5); }
  &.completed { background: #00ff88; box-shadow: 0 0 3px rgba(0,255,136,0.4); }
}

.point-canvas {
  width: 400px;
  height: 400px;
  border-radius: 8px;
  background: rgba(0,0,0,0.15);
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

// 设置弹窗样式
.settings-section {
  margin-bottom: 16px;
  padding: 12px;
  background: rgba(255,255,255,0.03);
  border-radius: 8px;
  border: 1px solid rgba(255,255,255,0.06);
}

.section-title {
  font-size: 12px;
  font-weight: 600;
  color: rgba(255,255,255,0.85);
  margin-bottom: 12px;
  padding-bottom: 8px;
  border-bottom: 1px solid rgba(255,255,255,0.08);
}

.form-row {
  display: flex;
  gap: 24px;
  margin-bottom: 12px;
  &:last-child { margin-bottom: 0; }
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.group-label {
  font-size: 11px;
  color: rgba(255,255,255,0.55);
  font-weight: 500;
}

.range-inputs, .point-inputs, .axis-inputs {
  display: flex;
  align-items: center;
  gap: 6px;
}

.range-separator {
  color: rgba(255,255,255,0.4);
  font-size: 12px;
}

.unit-label {
  font-size: 11px;
  color: rgba(255,255,255,0.4);
  margin-left: 4px;
}

.form-hint {
  font-size: 10px;
  color: rgba(255,255,255,0.3);
  margin-top: 2px;
  line-height: 1.2;
}

.compact-form :deep(.el-form-item) {
  margin-bottom: 10px;
}

// 校准文件区域
.calib-section {
  padding: 12px;
  background: rgba(255,255,255,0.04);
  border-radius: 8px;
  border: 1px solid rgba(255,255,255,0.06);
}

.calib-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.calib-title {
  display: flex;
  align-items: center;
  gap: 6px;
}

.calib-icon {
  font-size: 14px;
}

.calib-label {
  font-size: 12px;
  font-weight: 600;
  color: rgba(255,255,255,0.85);
}

.calib-status {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  margin-bottom: 8px;

  .status-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
  }

  &.loaded {
    color: #00ff88;
    .status-dot { background: #00ff88; box-shadow: 0 0 4px rgba(0,255,136,0.4); }
  }
  &.not-loaded {
    color: rgba(255,255,255,0.4);
    .status-dot { background: rgba(255,255,255,0.3); }
  }
}

.calib-file-list {
  max-height: 80px;
  overflow-y: auto;
  padding: 6px 8px;
  background: rgba(0,0,0,0.2);
  border-radius: 4px;
}
.calib-file-item {
  font-size: 10px;
  color: rgba(255,255,255,0.55);
  padding: 2px 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

// 通道映射样式
.device-row {
  gap: 20px;
}
.device-group {
  flex: 1;
}

.channel-table {
  :deep(th) {
    font-size: 11px;
    color: rgba(255,255,255,0.7) !important;
    font-weight: 600;
    padding: 8px 4px !important;
  }
  :deep(td) {
    font-size: 11px;
    color: rgba(255,255,255,0.6);
    padding: 6px 4px !important;
  }
}

.channel-hint {
  margin-top: 10px;
  font-size: 10px;
  color: rgba(255,255,255,0.35);
  text-align: right;
}

// 弹窗全局样式优化
:deep(.settings-dialog) {
  .el-dialog__header {
    margin-right: 0;
    padding: 16px 20px;
    border-bottom: 1px solid rgba(255,255,255,0.08);
  }
  .el-dialog__title {
    font-size: 14px;
    font-weight: 600;
    color: rgba(255,255,255,0.9);
  }
  .el-dialog__body {
    padding: 16px 20px;
  }
  .el-tabs__nav-wrap::after {
    background: rgba(255,255,255,0.08);
  }
  .el-tabs__item {
    font-size: 12px;
    color: rgba(255,255,255,0.55);
    &.is-active {
      color: #00f5ff;
    }
  }
  .el-tabs__active-bar {
    background: #00f5ff;
  }
}
</style>
