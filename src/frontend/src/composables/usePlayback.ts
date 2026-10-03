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

// parseChannelHeader 解析宽表表头列，如 "CH6 (kPa)" → { name: 'CH6', unit: 'kPa' }
function parseChannelHeader(raw: string): { name: string; unit: string } {
  const m = raw.match(/^CH(\d+)(?:\s*\(([^)]*)\))?$/)
  if (m) return { name: `CH${m[1]}`, unit: m[2]?.trim() ?? '' }
  return { name: raw, unit: '' }
}

// unquoteCSVField 还原 CSV 字段的引号转义（encoding/csv 对含引号字段会包裹并双写引号），
// 例如磁盘上的时间戳字段 =\"...\" 实际存为 "=\"\"...\"\""
function unquoteCSVField(raw: string): string {
  const t = raw.trim()
  if (t.length >= 2 && t.startsWith('"') && t.endsWith('"')) {
    return t.slice(1, -1).replace(/""/g, '"')
  }
  return t
}

// unwrapTimestamp 剥离录制文件中 ="..." 公式包裹（强制 Excel 按文本显示时间用），
// 先按 CSV 规则解引号转义（新录制文件），再剥离公式包裹；兼容无包裹的旧格式
function unwrapTimestamp(raw: string): string {
  const t = unquoteCSVField(raw)
  if (t.startsWith('="') && t.endsWith('"') && t.length > 3) {
    return t.slice(2, -1)
  }
  return t
}

export function usePlayback() {
  const playbackData = ref<PlaybackRow[]>([])
  const playbackIndex = ref(0)
  const isPlaying = ref(false)
  const playbackSpeed = ref(1)
  // 文件是否包含多台设备（旧版单文件录制）：图表需按 设备+通道 拆分曲线，
  // 否则不同设备的同名列会混入同一序列（主要表现为其他设备的 0.x 值混入主设备曲线）
  const multiDevice = ref(false)
  let playbackTimer: number | null = null

  function parseAndLoadCSV(content: string) {
    if (content.charCodeAt(0) === 0xFEFF) {
      content = content.slice(1)
    }

    const lines = content.split('\n').filter(l => l.trim())
    if (lines.length < 2) return

    const headerCols = lines[0].split(',')
    const isWide = headerCols.length >= 2 && headerCols[0].trim() === 'Timestamp' && headerCols[1].trim() === 'DeviceID'
      && headerCols.length > 2 && /^CH\d+/.test(headerCols[2].trim())

    const dataRows: PlaybackRow[] = []
    // 解析时增量判定多设备：避免对超大文件再分配一份等长的 deviceId 数组（Set(map())）
    let multiDeviceFlag = false
    let firstDeviceId: string | null = null
    const trackDevice = (deviceId: string) => {
      if (firstDeviceId === null) {
        firstDeviceId = deviceId
      } else if (deviceId !== firstDeviceId) {
        multiDeviceFlag = true
      }
    }
    if (isWide) {
      // 宽表格式：Timestamp, DeviceID, CH6 (kPa), CH7 (kPa), ...（每帧一行，所有通道横排）
      const channelCols = headerCols.slice(2).map(raw => parseChannelHeader(raw.trim()))
      if (channelCols.length === 0) return
      for (let i = 1; i < lines.length; i++) {
        const cols = lines[i].split(',')
        if (cols.length < 2) continue
        const timestamp = unwrapTimestamp(cols[0])
        const deviceId = cols[1].trim()
        trackDevice(deviceId)
        for (let c = 0; c < channelCols.length; c++) {
          const rawVal = (cols[c + 2] ?? '').trim()
          if (rawVal === '') continue
          dataRows.push({
            timestamp,
            deviceId,
            channelIndex: c,
            channelName: channelCols[c].name,
            value: parseFloat(rawVal) || 0,
            unit: channelCols[c].unit,
          })
        }
      }
    } else {
      // 旧版长表格式：Timestamp, DeviceID, ChannelIndex, ChannelName, Value, Unit
      for (let i = 1; i < lines.length; i++) {
        const cols = lines[i].split(',')
        if (cols.length >= 6) {
          const deviceId = cols[1].trim()
          trackDevice(deviceId)
          dataRows.push({
            timestamp: unwrapTimestamp(cols[0]),
            deviceId,
            channelIndex: parseInt(cols[2].trim()) || 0,
            channelName: cols[3].trim(),
            value: parseFloat(cols[4].trim()) || 0,
            unit: cols[5].trim(),
          })
        }
      }
    }

    if (dataRows.length > 0) {
      playbackData.value = dataRows
      playbackIndex.value = 0
      isPlaying.value = false
      multiDevice.value = multiDeviceFlag
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
      const key = multiDevice.value ? `${row.deviceId || '未知设备'}·${row.channelName}` : row.channelName
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
