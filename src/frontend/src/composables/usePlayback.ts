import { ref, computed, onUnmounted } from 'vue'

export interface PlaybackRow {
  timestamp: string
  deviceId: string
  channelIndex: number
  channelName: string
  value: number
  unit: string
}

const CHANNEL_COLORS = ['#b829ff', '#00f5ff', '#00ff88', '#ffaa00', '#ff3366', '#00aaff', '#d966ff', '#66faff']

export function usePlayback() {
  const playbackData = ref<PlaybackRow[]>([])
  const playbackIndex = ref(0)
  const isPlaying = ref(false)
  const playbackSpeed = ref(1)
  let playbackTimer: number | null = null

  function parseAndLoadCSV(content: string) {
    if (content.charCodeAt(0) === 0xFEFF) {
      content = content.slice(1)
    }

    const lines = content.split('\n').filter(l => l.trim())
    if (lines.length < 2) return

    const dataRows: PlaybackRow[] = []
    for (let i = 1; i < lines.length; i++) {
      const cols = lines[i].split(',')
      if (cols.length >= 6) {
        dataRows.push({
          timestamp: cols[0].trim(),
          deviceId: cols[1].trim(),
          channelIndex: parseInt(cols[2].trim()) || 0,
          channelName: cols[3].trim(),
          value: parseFloat(cols[4].trim()) || 0,
          unit: cols[5].trim(),
        })
      }
    }

    if (dataRows.length > 0) {
      playbackData.value = dataRows
      playbackIndex.value = 0
      isPlaying.value = false
    }
  }

  function startPlayback() {
    if (playbackData.value.length === 0) return
    isPlaying.value = true
    const intervalMs = 50 / playbackSpeed.value
    playbackTimer = window.setInterval(() => {
      if (playbackIndex.value < playbackData.value.length - 1) {
        playbackIndex.value++
      } else {
        pausePlayback()
      }
    }, intervalMs)
  }

  function pausePlayback() {
    isPlaying.value = false
    if (playbackTimer !== null) {
      clearInterval(playbackTimer)
      playbackTimer = null
    }
  }

  function resetPlayback() {
    pausePlayback()
    playbackIndex.value = 0
  }

  function togglePlayback() {
    if (isPlaying.value) {
      pausePlayback()
    } else {
      startPlayback()
    }
  }

  onUnmounted(() => {
    if (playbackTimer !== null) {
      clearInterval(playbackTimer)
    }
  })

  const playbackProgress = computed(() => {
    if (playbackData.value.length === 0) return 0
    return Math.round((playbackIndex.value / (playbackData.value.length - 1)) * 100)
  })

  const currentTimeLabel = computed(() => {
    if (playbackData.value.length === 0) return '--'
    return playbackData.value[playbackIndex.value]?.timestamp ?? '--'
  })

  const playbackChartOption = computed(() => {
    if (playbackData.value.length === 0) {
      return {
        backgroundColor: 'transparent',
        title: { text: '等待回放数据...', left: 'center', top: 'center', textStyle: { color: 'rgba(255,255,255,0.3)' } },
      }
    }

    const channelMap = new Map<string, { sampleIndex: number; value: number }[]>()
    const data = playbackData.value
    const endIdx = playbackIndex.value + 1

    let currentTimestamp = ''
    let sampleIndex = -1
    const xAxisData: number[] = []

    for (let i = 0; i < endIdx && i < data.length; i++) {
      const row = data[i]
      if (row.timestamp !== currentTimestamp) {
        currentTimestamp = row.timestamp
        sampleIndex++
        xAxisData.push(sampleIndex)
      }
      const key = `${row.channelName}`
      if (!channelMap.has(key)) {
        channelMap.set(key, [])
      }
      channelMap.get(key)!.push({ sampleIndex, value: row.value })
    }

    const series: any[] = []
    let chIdx = 0
    channelMap.forEach((points, name) => {
      series.push({
        name,
        type: 'line',
        data: points.map(p => [p.sampleIndex, p.value]),
        smooth: true,
        symbol: 'none',
        lineStyle: { width: 1.5, color: CHANNEL_COLORS[chIdx % CHANNEL_COLORS.length] },
        itemStyle: { color: CHANNEL_COLORS[chIdx % CHANNEL_COLORS.length] },
      })
      chIdx++
    })

    return {
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'axis',
        backgroundColor: 'rgba(10,10,26,0.9)',
        borderColor: 'rgba(0,245,255,0.3)',
        textStyle: { color: '#fff' },
      },
      legend: {
        top: 0,
        textStyle: { color: 'rgba(255,255,255,0.6)', fontSize: 10 },
      },
      grid: { left: 60, right: 20, top: 30, bottom: 30 },
      xAxis: {
        type: 'category',
        data: xAxisData,
        axisLine: { lineStyle: { color: 'rgba(255,255,255,0.1)' } },
        axisLabel: { color: 'rgba(255,255,255,0.4)', fontSize: 10 },
      },
      yAxis: {
        type: 'value',
        scale: true,
        name: 'Pa',
        nameTextStyle: { color: 'rgba(255,255,255,0.5)' },
        axisLine: { lineStyle: { color: 'rgba(255,255,255,0.1)' } },
        axisLabel: { color: 'rgba(255,255,255,0.4)' },
        splitLine: { lineStyle: { color: 'rgba(255,255,255,0.05)' } },
      },
      series,
    }
  })

  return {
    playbackData, playbackIndex, isPlaying, playbackSpeed,
    parseAndLoadCSV, togglePlayback, startPlayback, pausePlayback, resetPlayback,
    playbackProgress, currentTimeLabel, playbackChartOption,
  }
}
