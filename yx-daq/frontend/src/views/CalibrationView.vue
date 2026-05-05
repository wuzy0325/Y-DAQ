<template>
  <div class="calibration-view">
    <div class="grid-row">
      <GlassCard title="校准配置" icon="🔬" class="left-panel">
        <el-form label-width="80px" size="small">
          <el-form-item label="校准类型">
            <el-select v-model="config.type" style="width: 100%">
              <el-option label="五孔探针" value="five-hole" />
            </el-select>
          </el-form-item>
          <el-form-item label="α轴">
            <el-select v-model="config.alphaAxis" style="width: 100%">
              <el-option label="X" value="X" />
              <el-option label="Y" value="Y" />
            </el-select>
          </el-form-item>
          <el-form-item label="β轴">
            <el-select v-model="config.betaAxis" style="width: 100%">
              <el-option label="X" value="X" />
              <el-option label="Y" value="Y" />
            </el-select>
          </el-form-item>
          <el-form-item label="α范围">
            <div class="form-row">
              <el-input-number v-model="config.alphaMin" :step="5" size="small" style="width:100px" />
              <span>~</span>
              <el-input-number v-model="config.alphaMax" :step="5" size="small" style="width:100px" />
            </div>
          </el-form-item>
          <el-form-item label="β范围">
            <div class="form-row">
              <el-input-number v-model="config.betaMin" :step="5" size="small" style="width:100px" />
              <span>~</span>
              <el-input-number v-model="config.betaMax" :step="5" size="small" style="width:100px" />
            </div>
          </el-form-item>
          <el-form-item label="步数">
            <el-input-number v-model="config.steps" :min="2" :max="20" size="small" />
          </el-form-item>
          <el-form-item label="驻留时间">
            <el-input-number v-model="config.dwellTimeMs" :min="100" :step="100" size="small" /> ms
          </el-form-item>
          <el-form-item label="采样次数">
            <el-input-number v-model="config.samplesPerPoint" :min="1" :max="100" size="small" />
          </el-form-item>
        </el-form>

        <div class="calib-controls">
          <el-button v-if="!calibStore.isRunning" type="primary" @click="startCalib">启动校准</el-button>
          <el-button v-if="calibStore.isRunning" type="warning" @click="calibStore.pauseCalibration()">暂停</el-button>
          <el-button v-if="calibStore.isRunning" type="success" @click="calibStore.resumeCalibration()">恢复</el-button>
          <el-button v-if="calibStore.isRunning" type="danger" @click="calibStore.stopCalibration()">停止</el-button>
        </div>

        <div class="point-editor-section">
          <CalibPointEditor
            v-model="calibPoints"
            :alpha-min="config.alphaMin"
            :alpha-max="config.alphaMax"
            :beta-min="config.betaMin"
            :beta-max="config.betaMax"
          />
        </div>
      </GlassCard>

      <div class="right-panel">
        <GlassCard title="实时数据" icon="⚡">
          <div class="realtime-panel">
            <div class="section-label">原始压力</div>
            <div class="raw-data">
              <div class="data-item"><span class="label">P1</span><ValueDisplay :value="calibStore.realtime?.rawData.p1" :precision="3" color="#b829ff" /></div>
              <div class="data-item"><span class="label">P2</span><ValueDisplay :value="calibStore.realtime?.rawData.p2" :precision="3" color="#00f5ff" /></div>
              <div class="data-item"><span class="label">P3</span><ValueDisplay :value="calibStore.realtime?.rawData.p3" :precision="3" color="#00ff88" /></div>
              <div class="data-item"><span class="label">P4</span><ValueDisplay :value="calibStore.realtime?.rawData.p4" :precision="3" color="#ffaa00" /></div>
              <div class="data-item"><span class="label">P5</span><ValueDisplay :value="calibStore.realtime?.rawData.p5" :precision="3" color="#ff3366" /></div>
              <div class="data-item"><span class="label">P∞</span><ValueDisplay :value="calibStore.realtime?.rawData.pAtm" :precision="3" /></div>
              <div class="data-item"><span class="label">T∞</span><ValueDisplay :value="calibStore.realtime?.rawData.tAtm" :precision="2" unit="°C" /></div>
            </div>
            <div class="section-label">校准系数</div>
            <div class="coefficients">
              <div class="coeff-item kalpha"><span class="label">Kα</span><ValueDisplay :value="calibStore.realtime?.coefficients.Kalpha" :precision="4" color="#b829ff" /></div>
              <div class="coeff-item kbeta"><span class="label">Kβ</span><ValueDisplay :value="calibStore.realtime?.coefficients.Kbeta" :precision="4" color="#00f5ff" /></div>
              <div class="coeff-item cpt"><span class="label">CPT</span><ValueDisplay :value="calibStore.realtime?.coefficients.CPT" :precision="4" color="#ffaa00" /></div>
              <div class="coeff-item cps"><span class="label">CPS</span><ValueDisplay :value="calibStore.realtime?.coefficients.CPS" :precision="4" color="#00ff88" /></div>
            </div>
          </div>

          <div v-if="calibStore.progress" class="progress-section">
            <el-progress :percentage="calibStore.progress.progress" :stroke-width="10" color="#b829ff" />
            <div class="progress-text">
              {{ calibStore.progress.completedPoints }} / {{ calibStore.progress.totalPoints }}
              (α={{ calibStore.progress.currentAlpha.toFixed(1) }}°, β={{ calibStore.progress.currentBeta.toFixed(1) }}°)
            </div>
          </div>
        </GlassCard>

        <GlassCard title="系数等值线图" icon="🗺️">
          <div class="chart-controls">
            <el-radio-group v-model="contourField" size="small">
              <el-radio-button label="Kalpha">Kα</el-radio-button>
              <el-radio-button label="Kbeta">Kβ</el-radio-button>
              <el-radio-button label="CPT">CPT</el-radio-button>
              <el-radio-button label="CPS">CPS</el-radio-button>
            </el-radio-group>
          </div>
          <ChartPanel :option="contourOption" height="280px" />
        </GlassCard>
      </div>
    </div>

    <GlassCard title="校准结果" icon="📋" class="mt-lg">
      <template #actions>
        <el-button v-if="hasResults" type="primary" size="small" @click="exportCSV">导出CSV</el-button>
        <el-button v-if="hasResults" type="success" size="small" @click="exportPDF">导出PDF</el-button>
      </template>
      <el-table v-if="hasResults" :data="calibStore.taskStatus?.dataPoints" size="small" dark max-height="300">
        <el-table-column prop="alpha" label="α" width="80" />
        <el-table-column prop="beta" label="β" width="80" />
        <el-table-column label="P1" width="90"><template #default="{ row }">{{ row.rawData.p1.toFixed(3) }}</template></el-table-column>
        <el-table-column label="P2" width="90"><template #default="{ row }">{{ row.rawData.p2.toFixed(3) }}</template></el-table-column>
        <el-table-column label="P3" width="90"><template #default="{ row }">{{ row.rawData.p3.toFixed(3) }}</template></el-table-column>
        <el-table-column label="P4" width="90"><template #default="{ row }">{{ row.rawData.p4.toFixed(3) }}</template></el-table-column>
        <el-table-column label="P5" width="90"><template #default="{ row }">{{ row.rawData.p5.toFixed(3) }}</template></el-table-column>
        <el-table-column label="Kα" width="90"><template #default="{ row }">{{ row.coefficients.Kalpha.toFixed(4) }}</template></el-table-column>
        <el-table-column label="Kβ" width="90"><template #default="{ row }">{{ row.coefficients.Kbeta.toFixed(4) }}</template></el-table-column>
        <el-table-column label="CPT" width="90"><template #default="{ row }">{{ row.coefficients.CPT.toFixed(4) }}</template></el-table-column>
        <el-table-column label="CPS" width="90"><template #default="{ row }">{{ row.coefficients.CPS.toFixed(4) }}</template></el-table-column>
        <el-table-column prop="sampleCount" label="采样数" width="70" />
        <el-table-column label="标准差" width="80"><template #default="{ row }">{{ row.stdDev.toFixed(4) }}</template></el-table-column>
      </el-table>
      <div v-else class="no-data">暂无校准结果</div>
    </GlassCard>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, computed } from 'vue'
import { useCalibrationStore } from '../stores/calibration'
import CalibPointEditor from '../components/CalibPointEditor.vue'
import ChartPanel from '../components/ChartPanel.vue'
import GlassCard from '../components/GlassCard.vue'
import ValueDisplay from '../components/ValueDisplay.vue'
import { CalibrationService, DataService } from '../../bindings/yx-daq/internal/app'

const calibStore = useCalibrationStore()

const config = reactive({
  type: 'five-hole',
  alphaAxis: 'X',
  betaAxis: 'Y',
  alphaMin: -20,
  alphaMax: 20,
  betaMin: -20,
  betaMax: 20,
  steps: 5,
  dwellTimeMs: 500,
  samplesPerPoint: 10,
})

const contourField = ref<'Kalpha' | 'Kbeta' | 'CPT' | 'CPS'>('Kalpha')

// 校准点 (可视化编辑)
const calibPoints = ref(generateCalibPoints(-20, 20, -20, 20, 5))

const hasResults = computed(() => (calibStore.taskStatus?.dataPoints?.length ?? 0) > 0)

async function startCalib() {
  const calibConfig = {
    type: config.type,
    deviceId: 'sim-1',
    controllerId: 'sim-mc-1',
    alphaAxis: config.alphaAxis,
    betaAxis: config.betaAxis,
    dwellTimeMs: config.dwellTimeMs,
    samplesPerPoint: config.samplesPerPoint,
    probeChannels: [
      { name: 'P1', role: 'fiveHole.p1', channel: 0, enabled: true },
      { name: 'P2', role: 'fiveHole.p2', channel: 1, enabled: true },
      { name: 'P3', role: 'fiveHole.p3', channel: 2, enabled: true },
      { name: 'P4', role: 'fiveHole.p4', channel: 3, enabled: true },
      { name: 'P5', role: 'fiveHole.p5', channel: 4, enabled: true },
      { name: '大气压', role: 'fiveHole.pAtm', channel: 16, enabled: true },
      { name: '大气温度', role: 'fiveHole.tAtm', channel: 17, enabled: true },
    ],
    points: calibPoints.value,
    sphereTankGate: { enabled: false, channelIndex: 0, thresholdRate: 0.01, stableTimeMs: 1000 },
  }
  await calibStore.startCalibration(calibConfig)
}

function generateCalibPoints(alphaMin: number, alphaMax: number, betaMin: number, betaMax: number, steps: number) {
  const points = []
  const alphaStep = steps > 1 ? (alphaMax - alphaMin) / (steps - 1) : 0
  const betaStep = steps > 1 ? (betaMax - betaMin) / (steps - 1) : 0
  let id = 0
  for (let i = 0; i < steps; i++) {
    for (let j = 0; j < steps; j++) {
      points.push({
        id: `pt-${id++}`,
        alpha: alphaMin + i * alphaStep,
        beta: betaMin + j * betaStep,
      })
    }
  }
  return points
}

// 等值线图
const FIELD_COLORS: Record<string, string[]> = {
  Kalpha: ['#2e0053', '#5b008a', '#8b00c2', '#b829ff', '#d966ff', '#f0a6ff'],
  Kbeta:  ['#003344', '#006688', '#0099bb', '#00ccee', '#00f5ff', '#66faff'],
  CPT:    ['#332200', '#664400', '#996600', '#cc8800', '#ffaa00', '#ffcc33'],
  CPS:    ['#003322', '#006644', '#009966', '#00cc88', '#00ffaa', '#66ffcc'],
}

const contourOption = computed(() => {
  const dataPoints = calibStore.taskStatus?.dataPoints ?? []
  if (dataPoints.length < 3) {
    return {
      backgroundColor: 'transparent',
      title: { text: '等待数据...', left: 'center', top: 'center', textStyle: { color: 'rgba(255,255,255,0.3)' } },
    }
  }

  // 构造散点数据用于等值线
  const scatterData = dataPoints.map(p => [p.alpha, p.beta, p.coefficients[contourField.value]])

  return {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'item',
      backgroundColor: 'rgba(10,10,26,0.9)',
      borderColor: 'rgba(184,41,255,0.3)',
      textStyle: { color: '#fff' },
      formatter: (p: any) => `α=${p.data[0].toFixed(1)}° β=${p.data[1].toFixed(1)}°<br/>${contourField.value}=${p.data[2].toFixed(4)}`,
    },
    grid: { left: 50, right: 30, top: 20, bottom: 40 },
    xAxis: {
      name: 'α (°)',
      nameTextStyle: { color: 'rgba(255,255,255,0.5)' },
      axisLine: { lineStyle: { color: 'rgba(255,255,255,0.1)' } },
      axisLabel: { color: 'rgba(255,255,255,0.4)' },
    },
    yAxis: {
      name: 'β (°)',
      nameTextStyle: { color: 'rgba(255,255,255,0.5)' },
      axisLine: { lineStyle: { color: 'rgba(255,255,255,0.1)' } },
      axisLabel: { color: 'rgba(255,255,255,0.4)' },
      splitLine: { lineStyle: { color: 'rgba(255,255,255,0.05)' } },
    },
    visualMap: {
      show: true,
      min: Math.min(...scatterData.map(d => d[2])),
      max: Math.max(...scatterData.map(d => d[2])),
      inRange: { color: FIELD_COLORS[contourField.value] || FIELD_COLORS.Kalpha },
      textStyle: { color: 'rgba(255,255,255,0.6)' },
      right: 0,
      top: 'center',
      itemWidth: 10,
      itemHeight: 100,
    },
    series: [{
      type: 'scatter',
      data: scatterData,
      symbolSize: 12,
      itemStyle: { borderColor: 'rgba(255,255,255,0.3)', borderWidth: 1 },
    }],
  }
})

// CSV 导出
function exportCSV() {
  const dataPoints = calibStore.taskStatus?.dataPoints ?? []
  if (dataPoints.length === 0) return

  const BOM = '\uFEFF'
  const headers = ['α', 'β', 'P1', 'P2', 'P3', 'P4', 'P5', 'P∞', 'T∞', 'Kα', 'Kβ', 'CPT', 'CPS', '采样数', '标准差']
  const rows = dataPoints.map(p => [
    p.alpha, p.beta,
    p.rawData.p1, p.rawData.p2, p.rawData.p3, p.rawData.p4, p.rawData.p5,
    p.rawData.pAtm, p.rawData.tAtm,
    p.coefficients.Kalpha, p.coefficients.Kbeta, p.coefficients.CPT, p.coefficients.CPS,
    p.sampleCount, p.stdDev,
  ].join(','))

  const csv = BOM + headers.join(',') + '\n' + rows.join('\n')
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `calibration-${new Date().toISOString().slice(0,19).replace(/:/g,'-')}.csv`
  a.click()
  URL.revokeObjectURL(url)
}

// PDF 导出
async function exportPDF() {
  try {
    await DataService.ExportCalibrationPDF()
  } catch (e) {
    console.error('exportPDF failed:', e)
  }
}
</script>

<style lang="scss" scoped>
.calibration-view { display: flex; flex-direction: column; gap: $spacing-lg; }
.grid-row { display: flex; gap: $spacing-lg; }

.left-panel { flex: 0 0 360px; }
.right-panel { flex: 1; display: flex; flex-direction: column; gap: $spacing-lg; }
.form-row { display: flex; gap: $spacing-sm; align-items: center; }
.chart-controls { display: flex; gap: $spacing-sm; margin-bottom: $spacing-sm; }
.mt-lg { margin-top: $spacing-lg; }

.calib-controls { display: flex; gap: $spacing-sm; margin-top: $spacing-md; }
.point-editor-section { margin-top: $spacing-md; }

.realtime-panel { display: flex; flex-direction: column; gap: $spacing-sm; }
.section-label { font-size: $font-size-xs; color: $text-muted; text-transform: uppercase; letter-spacing: 1px; }
.raw-data, .coefficients { display: flex; flex-wrap: wrap; gap: $spacing-sm; }
.data-item, .coeff-item {
  background: $glass-bg; border-radius: 6px; padding: 6px 10px;
  display: flex; flex-direction: column; align-items: center; min-width: 70px;
}
.data-item .label, .coeff-item .label { font-size: 10px; color: rgba(255,255,255,0.5); margin-bottom: 2px; }

.no-data { color: rgba(255,255,255,0.3); text-align: center; padding: 20px; }
.progress-section { margin-top: $spacing-md; }
.progress-text { font-size: $font-size-sm; color: $text-tertiary; margin-top: 4px; }
</style>
