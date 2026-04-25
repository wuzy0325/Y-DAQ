<template>
  <div class="settings-view">
    <!-- 数据保存路径 -->
    <div class="settings-section">
      <div class="section-header">
        <span class="section-icon">📂</span>
        <span class="section-title">数据保存路径</span>
      </div>
      <div class="path-setting">
        <div class="path-display">
          <el-icon class="path-icon"><Folder /></el-icon>
          <span class="path-value">{{ dataSavePath || '使用默认路径' }}</span>
        </div>
        <div class="path-actions">
          <el-button type="primary" size="small" @click="selectPath">更改</el-button>
          <el-button size="small" @click="resetPath">重置</el-button>
        </div>
      </div>
    </div>

    <div class="settings-grid">
      <!-- 录制文件 -->
      <div class="settings-section file-section">
        <div class="section-header">
          <span class="section-icon">📁</span>
          <span class="section-title">录制文件</span>
          <div class="section-actions">
            <el-button size="small" @click="refreshFiles">
              <el-icon><Refresh /></el-icon>
            </el-button>
            <el-button type="primary" size="small" @click="loadExternalCSV">
              <el-icon><Upload /></el-icon>加载
            </el-button>
          </div>
        </div>
        
        <div class="file-list" v-if="recordingFiles.length > 0">
          <div 
            v-for="file in recordingFiles" 
            :key="file.name" 
            class="file-item"
            @click="loadFileForPlayback(file.name)"
          >
            <div class="file-info">
              <el-icon class="file-icon"><Document /></el-icon>
              <span class="file-name">{{ file.name }}</span>
            </div>
            <el-button type="primary" size="small" link @click.stop="loadFileForPlayback(file.name)">
              <el-icon><VideoPlay /></el-icon>
            </el-button>
          </div>
        </div>
        <div v-else class="empty-state">
          <el-icon class="empty-icon"><FolderOpened /></el-icon>
          <span>暂无录制文件</span>
        </div>
      </div>

      <!-- 数据回放 -->
      <div class="settings-section playback-section">
        <div class="section-header">
          <span class="section-icon">▶️</span>
          <span class="section-title">数据回放</span>
          <div class="section-actions" v-if="playbackData.length > 0">
            <div class="speed-control">
              <span class="speed-label">{{ playbackSpeed }}x</span>
              <el-slider v-model="playbackSpeed" :min="0.25" :max="4" :step="0.25" :show-tooltip="false" style="width: 80px" />
            </div>
            <el-button size="small" @click="resetPlayback">
              <el-icon><RefreshLeft /></el-icon>
            </el-button>
            <el-button :type="isPlaying ? 'warning' : 'primary'" size="small" @click="togglePlayback">
              <el-icon><component :is="isPlaying ? 'VideoPause' : 'VideoPlay'" /></el-icon>
              {{ isPlaying ? '暂停' : '播放' }}
            </el-button>
          </div>
        </div>

        <div v-if="playbackData.length > 0" class="playback-content">
          <div class="playback-stats">
            <div class="stat-item">
              <span class="stat-label">数据点</span>
              <span class="stat-value">{{ playbackData.length.toLocaleString() }}</span>
            </div>
            <div class="stat-item">
              <span class="stat-label">进度</span>
              <span class="stat-value">{{ playbackIndex + 1 }} / {{ playbackData.length }}</span>
            </div>
            <div class="stat-item">
              <span class="stat-label">时间</span>
              <span class="stat-value time">{{ currentTimeLabel }}</span>
            </div>
          </div>
          <el-progress :percentage="playbackProgress" :stroke-width="4" color="#00f5ff" :show-text="false" />
          <ChartPanel :option="playbackChartOption" height="260px" />
        </div>
        <div v-else class="empty-state">
          <el-icon class="empty-icon"><VideoPlay /></el-icon>
          <span>选择文件进行回放</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Folder, Refresh, Upload, Document, VideoPlay, FolderOpened, RefreshLeft, VideoPause } from '@element-plus/icons-vue'
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
  console.log('[loadFileForPlayback] clicked, fileName:', fileName)
  try {
    console.log('[loadFileForPlayback] importing ReadRecordingFile...')
    const { ReadRecordingFile } = await import('../../wailsjs/go/main/App')
    console.log('[loadFileForPlayback] ReadRecordingFile:', ReadRecordingFile)
    const content = await ReadRecordingFile(fileName) as string
    console.log('[loadFileForPlayback] content length:', content?.length)
    if (content) {
      console.log('[loadFileForPlayback] calling parseAndLoadCSV...')
      parseAndLoadCSV(content)
    } else {
      console.warn('[loadFileForPlayback] content is empty')
    }
  } catch (e: any) {
    console.error('[loadFileForPlayback] failed:', e)
    ElMessage.error(`加载文件失败: ${e?.message || e}`)
  }
}

// 解析CSV内容
function parseAndLoadCSV(content: string) {
  console.log('[parseAndLoadCSV] start, content length:', content.length)
  // 去除BOM
  if (content.charCodeAt(0) === 0xFEFF) {
    console.log('[parseAndLoadCSV] detected BOM, removing')
    content = content.slice(1)
  }

  const lines = content.split('\n').filter(l => l.trim())
  console.log('[parseAndLoadCSV] lines count:', lines.length)
  if (lines.length < 2) {
    console.warn('[parseAndLoadCSV] not enough lines, returning')
    return
  }

  console.log('[parseAndLoadCSV] header:', lines[0])
  // 跳过表头
  const dataRows: PlaybackRow[] = []
  for (let i = 1; i < lines.length; i++) {
    const cols = lines[i].split(',')
    console.log(`[parseAndLoadCSV] row ${i} cols=${cols.length}:`, cols)
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

  console.log('[parseAndLoadCSV] dataRows parsed:', dataRows.length)
  if (dataRows.length > 0) {
    playbackData.value = dataRows
    playbackIndex.value = 0
    isPlaying.value = false
    console.log('[parseAndLoadCSV] playbackData updated, rows:', dataRows.length)
  } else {
    console.warn('[parseAndLoadCSV] no valid data rows found')
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
      scale: true,
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
.settings-view {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 4px;
}

// 通用区块样式
.settings-section {
  background: rgba(255,255,255,0.03);
  border: 1px solid rgba(255,255,255,0.06);
  border-radius: 10px;
  padding: 16px;
}

.section-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid rgba(255,255,255,0.06);

  .section-icon {
    font-size: 16px;
  }

  .section-title {
    font-size: 13px;
    font-weight: 600;
    color: rgba(255,255,255,0.9);
    flex: 1;
  }

  .section-actions {
    display: flex;
    align-items: center;
    gap: 8px;
  }
}

// 路径设置
.path-setting {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.path-display {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 1;
  min-width: 0;

  .path-icon {
    font-size: 18px;
    color: rgba(255,255,255,0.4);
  }

  .path-value {
    font-size: 13px;
    color: rgba(255,255,255,0.8);
    font-family: monospace;
    word-break: break-all;
    background: rgba(0,0,0,0.2);
    padding: 6px 12px;
    border-radius: 6px;
    flex: 1;
  }
}

.path-actions {
  display: flex;
  gap: 8px;
}

// 网格布局
.settings-grid {
  display: grid;
  grid-template-columns: 320px 1fr;
  gap: 16px;
}

// 文件列表
.file-section {
  min-height: 400px;
}

.file-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.file-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 10px;
  background: rgba(255,255,255,0.03);
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s ease;

  &:hover {
    background: rgba(255,255,255,0.06);
  }
}

.file-info {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;

  .file-icon {
    font-size: 16px;
    color: rgba(255,255,255,0.4);
    flex-shrink: 0;
  }

  .file-name {
    font-size: 12px;
    color: rgba(255,255,255,0.8);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
}

// 回放区域
.playback-content {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.playback-stats {
  display: flex;
  gap: 24px;
  padding: 10px 12px;
  background: rgba(0,0,0,0.15);
  border-radius: 8px;
}

.stat-item {
  display: flex;
  flex-direction: column;
  gap: 2px;

  .stat-label {
    font-size: 10px;
    color: rgba(255,255,255,0.4);
  }

  .stat-value {
    font-size: 13px;
    color: rgba(255,255,255,0.9);
    font-weight: 500;

    &.time {
      font-family: monospace;
      color: #00f5ff;
    }
  }
}

.speed-control {
  display: flex;
  align-items: center;
  gap: 8px;
  background: rgba(0,0,0,0.15);
  padding: 4px 10px;
  border-radius: 6px;

  .speed-label {
    font-size: 11px;
    color: #00f5ff;
    font-weight: 500;
    min-width: 30px;
  }
}

// 空状态
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 40px 20px;
  color: rgba(255,255,255,0.35);

  .empty-icon {
    font-size: 32px;
    opacity: 0.5;
  }

  span {
    font-size: 12px;
  }
}

// 滑块样式覆盖
:deep(.el-slider__runway) {
  background-color: rgba(255,255,255,0.1);
}
:deep(.el-slider__bar) {
  background-color: #00f5ff;
}
:deep(.el-slider__button) {
  border-color: #00f5ff;
}
</style>
