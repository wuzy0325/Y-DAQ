<template>
  <div class="settings-view">
    <!-- 数据保存路径设置 -->
    <GlassCard title="数据保存路径" icon="📂">
      <div class="path-setting">
        <div class="path-info">
          <span class="path-label">录制数据保存目录：</span>
          <span class="path-value">{{ dataSavePath || '未设置（使用默认路径）' }}</span>
        </div>
        <div class="path-controls">
          <el-button type="primary" size="small" @click="selectPath">选择路径</el-button>
          <el-button size="small" @click="resetPath">恢复默认</el-button>
        </div>
      </div>
    </GlassCard>

    <div class="grid-row">
      <!-- 录制文件列表 -->
      <GlassCard title="录制文件" icon="📁" style="flex: 1">
        <template #actions>
          <el-button type="primary" size="small" @click="loadExternalCSV">加载CSV</el-button>
          <el-button size="small" @click="refreshFiles">刷新</el-button>
        </template>
        <el-table v-if="recordingFiles.length > 0" :data="recordingFiles" size="small" dark max-height="200">
          <el-table-column prop="name" label="文件名" />
          <el-table-column label="操作" width="100">
            <template #default="{ row }">
              <el-button type="primary" size="small" link @click="loadFileForPlayback(row.name)">回放</el-button>
            </template>
          </el-table-column>
        </el-table>
        <div v-else class="no-data">暂无录制文件</div>
      </GlassCard>
    </div>

    <!-- 数据回放 -->
    <GlassCard title="数据回放" icon="▶️">
      <template #actions>
        <template v-if="playbackData.length > 0">
          <el-button type="primary" size="small" @click="togglePlayback">
            {{ isPlaying ? '暂停' : '播放' }}
          </el-button>
          <el-button size="small" @click="resetPlayback">重置</el-button>
          <div class="speed-control">
            <span class="speed-label">速度:</span>
            <el-slider v-model="playbackSpeed" :min="0.25" :max="4" :step="0.25" :show-tooltip="false" style="width: 100px" />
            <span class="speed-value">{{ playbackSpeed }}x</span>
          </div>
        </template>
      </template>

      <div v-if="playbackData.length > 0" class="playback-content">
        <div class="playback-info">
          <span>数据点: {{ playbackData.length }}</span>
          <span>当前: {{ playbackIndex + 1 }} / {{ playbackData.length }}</span>
          <span>时间: {{ currentTimeLabel }}</span>
        </div>
        <el-progress :percentage="playbackProgress" :stroke-width="6" color="#00f5ff" />
        <ChartPanel :option="playbackChartOption" height="300px" />
      </div>
      <div v-else class="no-data">选择文件进行回放</div>
    </GlassCard>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import GlassCard from '../components/GlassCard.vue'
import ChartPanel from '../components/ChartPanel.vue'

// 数据保存路径
const dataSavePath = ref('')

// 录制文件
const recordingFiles = ref<{ name: string }[]>([])

// 回放状态
const playbackData = ref<PlaybackRow[]>([])
const playbackIndex = ref(0)
const isPlaying = ref(false)
const playbackSpeed = ref(1)
let playbackTimer: number | null = null

interface PlaybackRow {
  timestamp: string
  deviceId: string
  channelIndex: number
  channelName: string
  value: number
  unit: string
}

// 加载数据保存路径
async function loadDataSavePath() {
  try {
    const { GetDataDir } = await import('../../wailsjs/go/main/App')
    dataSavePath.value = await GetDataDir() as string
  } catch (e) {
    console.error('loadDataSavePath failed:', e)
  }
}

// 选择数据保存路径
async function selectPath() {
  try {
    const { SelectDataSavePath, SetDataSavePath } = await import('../../wailsjs/go/main/App')
    const dir = await SelectDataSavePath() as string
    if (dir) {
      await SetDataSavePath(dir)
      dataSavePath.value = dir
    }
  } catch (e) {
    console.error('selectPath failed:', e)
  }
}

// 恢复默认路径
async function resetPath() {
  try {
    const { SetDataSavePath, GetDataDir } = await import('../../wailsjs/go/main/App')
    await SetDataSavePath('')
    dataSavePath.value = await GetDataDir() as string
  } catch (e) {
    console.error('resetPath failed:', e)
  }
}

// 刷新文件列表
async function refreshFiles() {
  try {
    const { ListRecordingFiles } = await import('../../wailsjs/go/main/App')
    const files = await ListRecordingFiles() as string[]
    recordingFiles.value = files.map(f => ({ name: f }))
  } catch (e) {
    console.error('refreshFiles failed:', e)
  }
}

// 加载外部CSV
async function loadExternalCSV() {
  try {
    const { LoadCSVFile } = await import('../../wailsjs/go/main/App')
    const content = await LoadCSVFile() as string
    if (content) {
      parseAndLoadCSV(content)
    }
  } catch (e) {
    console.error('loadExternalCSV failed:', e)
  }
}

// 加载录制文件回放
async function loadFileForPlayback(fileName: string) {
  try {
    const { GetDataDir } = await import('../../wailsjs/go/main/App')
    const dataDir = await GetDataDir() as string
    // 通过LoadCSVFile让用户选择，或直接读取
    // 这里简化处理：使用LoadCSVFile
    await loadExternalCSV()
  } catch (e) {
    console.error('loadFileForPlayback failed:', e)
  }
}

// 解析CSV内容
function parseAndLoadCSV(content: string) {
  // 去除BOM
  if (content.charCodeAt(0) === 0xFEFF) {
    content = content.slice(1)
  }

  const lines = content.split('\n').filter(l => l.trim())
  if (lines.length < 2) return

  // 跳过表头
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

// 回放控制
function togglePlayback() {
  if (isPlaying.value) {
    pausePlayback()
  } else {
    startPlayback()
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

onUnmounted(() => {
  if (playbackTimer !== null) {
    clearInterval(playbackTimer)
  }
})

// 回放进度
const playbackProgress = computed(() => {
  if (playbackData.value.length === 0) return 0
  return Math.round((playbackIndex.value / (playbackData.value.length - 1)) * 100)
})

const currentTimeLabel = computed(() => {
  if (playbackData.value.length === 0) return '--'
  return playbackData.value[playbackIndex.value]?.timestamp ?? '--'
})

// 回放图表 - 按通道分组显示折线
const CHANNEL_COLORS = ['#b829ff', '#00f5ff', '#00ff88', '#ffaa00', '#ff3366', '#00aaff', '#d966ff', '#66faff']

const playbackChartOption = computed(() => {
  if (playbackData.value.length === 0) {
    return {
      backgroundColor: 'transparent',
      title: { text: '等待回放数据...', left: 'center', top: 'center', textStyle: { color: 'rgba(255,255,255,0.3)' } },
    }
  }

  // 按通道分组
  const channelMap = new Map<string, { time: number; value: number }[]>()
  const data = playbackData.value
  const endIdx = playbackIndex.value + 1

  for (let i = 0; i < endIdx && i < data.length; i++) {
    const row = data[i]
    const key = `${row.channelName}`
    if (!channelMap.has(key)) {
      channelMap.set(key, [])
    }
    channelMap.get(key)!.push({ time: i, value: row.value })
  }

  const series: any[] = []
  let chIdx = 0
  channelMap.forEach((points, name) => {
    series.push({
      name,
      type: 'line',
      data: points.map(p => [p.time, p.value]),
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
      axisLine: { lineStyle: { color: 'rgba(255,255,255,0.1)' } },
      axisLabel: { color: 'rgba(255,255,255,0.4)', fontSize: 10 },
    },
    yAxis: {
      type: 'value',
      name: 'kPa',
      nameTextStyle: { color: 'rgba(255,255,255,0.5)' },
      axisLine: { lineStyle: { color: 'rgba(255,255,255,0.1)' } },
      axisLabel: { color: 'rgba(255,255,255,0.4)' },
      splitLine: { lineStyle: { color: 'rgba(255,255,255,0.05)' } },
    },
    series,
  }
})

// 初始化
onMounted(() => {
  loadDataSavePath()
  refreshFiles()
})
</script>

<style lang="scss" scoped>
.settings-view { display: flex; flex-direction: column; gap: 16px; }
.grid-row { display: flex; gap: 16px; }

.path-setting {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.path-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.path-label {
  font-size: 12px;
  color: rgba(255,255,255,0.5);
}

.path-value {
  font-size: 13px;
  color: #00f5ff;
  font-family: monospace;
  word-break: break-all;
}

.path-controls { display: flex; gap: 8px; }

.playback-content { display: flex; flex-direction: column; gap: 8px; }
.playback-info {
  display: flex; gap: 16px; font-size: 12px; color: rgba(255,255,255,0.6);
}

.speed-control {
  display: flex; align-items: center; gap: 6px; margin-left: 8px;
}
.speed-label { font-size: 11px; color: rgba(255,255,255,0.5); }
.speed-value { font-size: 11px; color: #00f5ff; min-width: 30px; }

.no-data { color: rgba(255,255,255,0.3); text-align: center; padding: 20px; }
</style>
