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
        
        <div v-if="recordingFiles.length > 0" class="file-list">
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
          <div v-if="playbackData.length > 0" class="section-actions">
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
import { ref, onMounted } from 'vue'
import { Folder, Refresh, Upload, Document, VideoPlay, FolderOpened, RefreshLeft, VideoPause } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { usePlayback } from '../composables/usePlayback'
import ChartPanel from '../components/ChartPanel.vue'
import { GetDataDir, SetDataSavePath, SelectDataSavePath, ListRecordingFiles, LoadCSVFile, ReadRecordingFile } from '../../wailsjs/go/main/App'

const {
  playbackData, playbackIndex, isPlaying, playbackSpeed,
  parseAndLoadCSV, togglePlayback, resetPlayback,
  playbackProgress, currentTimeLabel, playbackChartOption,
} = usePlayback()

const dataSavePath = ref('')
const recordingFiles = ref<{ name: string }[]>([])

async function loadDataSavePath() {
  try {
    dataSavePath.value = await GetDataDir() as string
  } catch (e) {
    console.error('loadDataSavePath failed:', e)
  }
}

async function selectPath() {
  try {
    const dir = await SelectDataSavePath() as string
    if (dir) {
      await SetDataSavePath(dir)
      dataSavePath.value = dir
    }
  } catch (e) {
    console.error('selectPath failed:', e)
  }
}

async function resetPath() {
  try {
    await SetDataSavePath('')
    dataSavePath.value = await GetDataDir() as string
  } catch (e) {
    console.error('resetPath failed:', e)
  }
}

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
  try {
    const content = await ReadRecordingFile(fileName) as string
    if (content) {
      parseAndLoadCSV(content)
    } else {
      ElMessage.warning('文件内容为空')
    }
  } catch (e: any) {
    console.error('loadFileForPlayback failed:', e)
    ElMessage.error(`加载文件失败: ${e?.message || e}`)
  }
}

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
  background: $bg-tertiary;
  border: 1px solid $glass-bg;
  border-radius: 10px;
  padding: 16px;
}

.section-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid $glass-bg;

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
  background: $bg-tertiary;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s ease;

  &:hover {
    background: $glass-bg;
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
      color: $color-accent;
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
    color: $color-accent;
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
  background-color: $color-accent;
}
:deep(.el-slider__button) {
  border-color: $color-accent;
}
</style>
