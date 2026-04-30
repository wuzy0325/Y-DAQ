import { ref, shallowRef, watch, onUnmounted, type Ref } from 'vue'

// ==================== 类型 ====================

interface InterpResult {
  ptProbe: number; psProbe: number; machProbe: number; alphaProbe: number
}

interface RealtimeEvent {
  pointId?: string
  interpResult: InterpResult
}

// ==================== 波形历史 Composable ====================

const MAX_WAVE_POINTS = 200

export function useWaveHistory(
  realtime: Ref<RealtimeEvent | null>,
  isRunning: Ref<boolean>,
  isPaused: Ref<boolean>,
) {
  const ptHistory = ref<number[]>([])
  const psHistory = ref<number[]>([])
  const maHistory = ref<number[]>([])
  const alphaHistory = ref<number[]>([])
  const waveLabels = ref<number[]>([])

  const ptChartOption = shallowRef(makeWaveOption([], '#ffaa00'))
  const psChartOption = shallowRef(makeWaveOption([], '#00ff88'))
  const maChartOption = shallowRef(makeWaveOption([], '#b829ff'))
  const alphaChartOption = shallowRef(makeWaveOption([], '#00f5ff'))

  // 去重
  let lastWavePointId = ''

  // 监听实时数据
  watch(realtime, (rt) => {
    if (!rt || !isRunning.value || isPaused.value) return
    if (rt.pointId && rt.pointId === lastWavePointId) return
    lastWavePointId = rt.pointId || ''

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

  // 运行状态变化时重置
  watch(isRunning, (running) => {
    if (!running) lastWavePointId = ''
    if (running) {
      ptHistory.value = []
      psHistory.value = []
      maHistory.value = []
      alphaHistory.value = []
      waveLabels.value = []
    }
  })

  // 节流更新
  let waveUpdateTimer: number | null = null
  let waveDirty = false

  function scheduleWaveUpdate() {
    waveDirty = true
    if (waveUpdateTimer) return
    waveUpdateTimer = window.setTimeout(() => {
      waveUpdateTimer = null
      if (!waveDirty) return
      waveDirty = false
      const labels = waveLabels.value
      ptChartOption.value = {
        ...ptChartOption.value,
        xAxis: { ...ptChartOption.value.xAxis, data: labels },
        series: [{ ...ptChartOption.value.series[0], data: ptHistory.value }],
      }
      psChartOption.value = {
        ...psChartOption.value,
        xAxis: { ...psChartOption.value.xAxis, data: labels },
        series: [{ ...psChartOption.value.series[0], data: psHistory.value }],
      }
      maChartOption.value = {
        ...maChartOption.value,
        xAxis: { ...maChartOption.value.xAxis, data: labels },
        series: [{ ...maChartOption.value.series[0], data: maHistory.value }],
      }
      alphaChartOption.value = {
        ...alphaChartOption.value,
        xAxis: { ...alphaChartOption.value.xAxis, data: labels },
        series: [{ ...alphaChartOption.value.series[0], data: alphaHistory.value }],
      }
    }, 200)
  }

  onUnmounted(() => {
    if (waveUpdateTimer) {
      clearTimeout(waveUpdateTimer)
      waveUpdateTimer = null
    }
  })

  return {
    ptChartOption, psChartOption, maChartOption, alphaChartOption,
    waveLabels,
  }
}

function makeWaveOption(data: number[], color: string) {
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
      data: [] as number[],
      axisLine: { lineStyle: { color: 'rgba(255,255,255,0.1)' } },
      axisLabel: { color: 'rgba(255,255,255,0.3)', fontSize: 9 },
    },
    yAxis: {
      type: 'value',
      scale: true,
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
