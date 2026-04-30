<template>
  <div class="data-view">
    <div class="grid-row">
      <!-- 录制控制 -->
      <GlassCard title="数据管理" icon="💾" class="sidebar-card">
        <div class="recording-status">
          <span class="status-dot" :class="recording ? 'recording' : ''" />
          <span>{{ recording ? '录制中...' : '未录制' }}</span>
        </div>
        <div class="data-info">
          <div class="info-item">
            <span class="info-label">数据快照频率</span>
            <span class="info-value">{{ publishRate }} Hz</span>
          </div>
          <div class="info-item">
            <span class="info-label">当前快照数</span>
            <span class="info-value">{{ deviceStore.snapshots.length }}</span>
          </div>
        </div>
        <div class="record-controls">
          <el-button type="primary" size="small" @click="startRecording">开始录制</el-button>
          <el-button type="danger" size="small" @click="stopRecording">停止录制</el-button>
        </div>
      </GlassCard>

      <!-- 录制文件列表 -->
      <GlassCard title="录制文件" icon="📁" class="flex-card">
        <template #actions>
          <el-button type="primary" size="small" @click="loadExternalCSV">加载CSV</el-button>
          <el-button size="small" @click="refreshFiles">刷新</el-button>
        </template>
        <el-table v-if="recordingFiles.length > 0" :data="recordingFiles" size="small" max-height="200">
          <el-table-column prop="name" label="文件名" />
          <el-table-column label="操作" width="100">
            <template #default="{ row }">
              <el-button
                type="primary"
                size="small"
                :disabled="loadingFile === row.name"
                :loading="loadingFile === row.name"
                @click="loadFileForPlayback(row.name)"
              >
                回放
              </el-button>
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
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { useDeviceStore } from '../stores/device'
import { usePlayback } from '../composables/usePlayback'
import ChartPanel from '../components/ChartPanel.vue'
import GlassCard from '../components/GlassCard.vue'
import { ListRecordingFiles, LoadCSVFile, ReadRecordingFile } from '../../wailsjs/go/main/App'

const deviceStore = useDeviceStore()

const {
  playbackData, playbackIndex, isPlaying, playbackSpeed,
  parseAndLoadCSV, togglePlayback, resetPlayback,
  playbackProgress, currentTimeLabel, playbackChartOption,
} = usePlayback()

const recording = ref(false)
const publishRate = ref(20)
const recordingFiles = ref<{ name: string }[]>([])
const loadingFile = ref('')

function startRecording() { recording.value = true }
function stopRecording() { recording.value = false }

async function refreshFiles() {
  try {
    const files = await ListRecordingFiles() as string[]
    recordingFiles.value = files.map(f => ({ name: f }))
  } catch (e) {
    console.error('refreshFiles failed:', e)
  }
}

async function loadExternalCSV() {
  try {
    const content = await LoadCSVFile() as string
    if (content) {
      parseAndLoadCSV(content)
    }
  } catch (e) {
    console.error('loadExternalCSV failed:', e)
  }
}

async function loadFileForPlayback(fileName: string) {
  loadingFile.value = fileName
  try {
    const content = await ReadRecordingFile(fileName) as string
    if (content) {
      parseAndLoadCSV(content)
      ElMessage.success(`已加载: ${fileName}`)
    } else {
      ElMessage.warning('文件内容为空')
    }
  } catch (e: any) {
    console.error('loadFileForPlayback failed:', e)
    ElMessage.error(`加载失败: ${e?.message || e}`)
  } finally {
    loadingFile.value = ''
  }
}

onMounted(() => {
  refreshFiles()
})
</script>

<style lang="scss" scoped>
.data-view { display: flex; flex-direction: column; gap: 16px; }
.grid-row { display: flex; gap: 16px; }
.sidebar-card { flex: 0 0 360px; }
.flex-card { flex: 1; }

.record-controls { display: flex; gap: 8px; margin-top: 12px; }

.recording-status {
  display: flex; align-items: center; gap: 8px; margin-bottom: 12px;
  font-size: 13px; color: rgba(255,255,255,0.7);
}

.status-dot {
  width: 8px; height: 8px; border-radius: 50%; background: rgba(255,255,255,0.3);
  &.recording { background: $color-danger; box-shadow: 0 0 8px $color-danger-glow; animation: pulse 1s infinite; }
}

@keyframes pulse { 0%,100%{opacity:1} 50%{opacity:0.5} }

.data-info { display: flex; gap: 24px; }
.info-item { display: flex; flex-direction: column; gap: 2px; }
.info-label { font-size: 11px; color: rgba(255,255,255,0.4); }
.info-value { font-size: 16px; color: $color-accent; font-family: monospace; }

.playback-content { display: flex; flex-direction: column; gap: 8px; }
.playback-info {
  display: flex; gap: 16px; font-size: 12px; color: rgba(255,255,255,0.6);
}

.speed-control {
  display: flex; align-items: center; gap: 6px; margin-left: 8px;
}
.speed-label { font-size: 11px; color: rgba(255,255,255,0.5); }
.speed-value { font-size: 11px; color: $color-accent; min-width: 30px; }

.no-data { color: rgba(255,255,255,0.3); text-align: center; padding: 20px; }
</style>
