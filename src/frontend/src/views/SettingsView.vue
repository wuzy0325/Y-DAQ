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

    <!-- 日志 -->
    <div class="settings-section log-section">
      <div class="section-header">
        <span class="section-icon">📝</span>
        <span class="section-title">日志</span>
        <div class="section-actions">
          <div class="log-config">
            <span class="config-label">级别</span>
            <el-select v-model="logLevel" size="small" style="width: 88px" @change="saveLogConfig">
              <el-option label="调试" value="debug" />
              <el-option label="信息" value="info" />
              <el-option label="警告" value="warn" />
              <el-option label="错误" value="error" />
            </el-select>
          </div>
          <div class="log-config">
            <span class="config-label">通信日志</span>
            <el-switch v-model="commEnabled" size="small" @change="saveLogConfig" />
          </div>
          <div class="log-config">
            <span class="config-label">前端错误</span>
            <el-switch v-model="frontendErrors" size="small" @change="saveLogConfig" />
          </div>
          <el-button size="small" @click="refreshLogFiles">
            <el-icon><Refresh /></el-icon>
          </el-button>
          <el-button size="small" @click="openLogDir">
            <el-icon><FolderOpened /></el-icon>打开目录
          </el-button>
          <el-button size="small" @click="exportLogs">
            <el-icon><Download /></el-icon>导出
          </el-button>
          <el-button type="danger" plain size="small" @click="clearLogs">清空</el-button>
        </div>
      </div>

      <div class="log-content">
        <div class="log-file-list">
          <div
            v-for="file in logFiles"
            :key="file.name"
            class="log-file-item"
            :class="{ active: file.name === selectedLogFile }"
            @click="selectLogFile(file.name)"
          >
            <div class="log-file-info">
              <span class="log-category" :class="`cat-${file.category}`">{{ categoryLabel(file.category) }}</span>
              <span class="log-file-name" :title="file.name">{{ file.name }}</span>
            </div>
            <span class="log-file-meta">{{ formatSize(file.size) }}</span>
          </div>
          <div v-if="logFiles.length === 0" class="empty-state">
            <el-icon class="empty-icon"><FolderOpened /></el-icon>
            <span>暂无日志文件</span>
          </div>
        </div>
        <div class="log-tail">
          <pre v-if="logTail" class="log-tail-content">{{ logTail }}</pre>
          <div v-else class="empty-state">
            <el-icon class="empty-icon"><Document /></el-icon>
            <span>选择日志文件查看内容</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Folder, Refresh, Upload, Document, VideoPlay, FolderOpened, RefreshLeft, VideoPause, Download } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { usePlayback } from '../composables/usePlayback'
import ChartPanel from '../components/ChartPanel.vue'
import { ConfigService, DataService, LogService } from '@bindings/yx-daq/internal/app'
import * as types from '@bindings/yx-daq/internal/types'

const {
  playbackData, playbackIndex, isPlaying, playbackSpeed,
  parseAndLoadCSV, togglePlayback, resetPlayback,
  playbackProgress, currentTimeLabel, playbackChartOption,
} = usePlayback()

const dataSavePath = ref('')
const recordingFiles = ref<{ name: string }[]>([])

async function loadDataSavePath() {
  try {
    dataSavePath.value = await DataService.GetDataDir() as string
  } catch (e) {
    console.error('loadDataSavePath failed:', e)
  }
}

async function selectPath() {
  try {
    const dir = await ConfigService.SelectDataSavePath() as string
    if (dir) {
      await ConfigService.SetDataSavePath(dir)
      dataSavePath.value = dir
    }
  } catch (e) {
    console.error('selectPath failed:', e)
  }
}

async function resetPath() {
  try {
    await ConfigService.SetDataSavePath('')
    dataSavePath.value = await DataService.GetDataDir() as string
  } catch (e) {
    console.error('resetPath failed:', e)
  }
}

async function refreshFiles() {
  try {
    const files = await DataService.ListRecordingFiles() as string[] | null
    recordingFiles.value = (files ?? []).map(f => ({ name: f }))
  } catch (e) {
    console.error('refreshFiles failed:', e)
  }
}

async function loadExternalCSV() {
  try {
    const content = await DataService.LoadCSVFile() as string
    if (content) {
      parseAndLoadCSV(content)
    }
  } catch (e) {
    console.error('loadExternalCSV failed:', e)
  }
}

async function loadFileForPlayback(fileName: string) {
  try {
    const content = await DataService.ReadRecordingFile(fileName) as string
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

// ==================== 日志 ====================

const logFiles = ref<types.LogFileInfo[]>([])
const selectedLogFile = ref('')
const logTail = ref('')
const logLevel = ref('info')
const commEnabled = ref(true)
const frontendErrors = ref(true)

async function loadLogConfig() {
  try {
    const cfg = await LogService.GetLoggingConfig()
    logLevel.value = cfg.level || 'info'
    commEnabled.value = cfg.commEnabled
    frontendErrors.value = cfg.frontendErrors
  } catch (e) {
    console.error('loadLogConfig failed:', e)
  }
}

async function saveLogConfig() {
  try {
    const cfg = await LogService.GetLoggingConfig()
    cfg.level = logLevel.value
    cfg.commEnabled = commEnabled.value
    cfg.frontendErrors = frontendErrors.value
    await LogService.SetLoggingConfig(cfg)
    ElMessage.success('日志配置已保存')
  } catch (e: any) {
    console.error('saveLogConfig failed:', e)
    ElMessage.error(`保存日志配置失败: ${e?.message || e}`)
  }
}

async function refreshLogFiles() {
  try {
    const files = await LogService.ListLogFiles()
    logFiles.value = files ?? []
  } catch (e) {
    console.error('refreshLogFiles failed:', e)
  }
}

async function selectLogFile(fileName: string) {
  selectedLogFile.value = fileName
  try {
    logTail.value = await LogService.ReadLogTail(fileName, 200) as string
    if (!logTail.value) {
      ElMessage.warning('日志文件为空')
    }
  } catch (e: any) {
    console.error('selectLogFile failed:', e)
    logTail.value = ''
    ElMessage.error(`读取日志失败: ${e?.message || e}`)
  }
}

async function openLogDir() {
  try {
    await LogService.OpenLogDir()
  } catch (e: any) {
    console.error('openLogDir failed:', e)
    ElMessage.error(`打开日志目录失败: ${e?.message || e}`)
  }
}

async function exportLogs() {
  try {
    const dst = await LogService.ExportLogs() as string
    if (dst) {
      ElMessage.success(`日志已导出: ${dst}`)
    }
  } catch (e: any) {
    console.error('exportLogs failed:', e)
    ElMessage.error(`导出日志失败: ${e?.message || e}`)
  }
}

async function clearLogs() {
  try {
    await ElMessageBox.confirm(
      '将清空全部日志文件，此操作不可恢复。是否继续？',
      '清空日志确认',
      { confirmButtonText: '清空', cancelButtonText: '取消', type: 'warning' },
    )
  } catch {
    return // 用户取消
  }
  try {
    await LogService.ClearLogs()
    logTail.value = ''
    selectedLogFile.value = ''
    await refreshLogFiles()
    ElMessage.success('日志已清空')
  } catch (e: any) {
    console.error('clearLogs failed:', e)
    ElMessage.error(`清空日志失败: ${e?.message || e}`)
  }
}

function formatSize(size: number): string {
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`
  return `${(size / 1024 / 1024).toFixed(1)} MB`
}

function categoryLabel(category: string): string {
  switch (category) {
    case 'app': return '程序'
    case 'comm': return '通信'
    case 'crash': return '崩溃'
    default: return '其他'
  }
}

onMounted(() => {
  loadDataSavePath()
  refreshFiles()
  loadLogConfig()
  refreshLogFiles()
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

// 日志
.log-section {
  min-height: 320px;
}

.log-config {
  display: flex;
  align-items: center;
  gap: 6px;

  .config-label {
    font-size: 11px;
    color: rgba(255,255,255,0.5);
    white-space: nowrap;
  }
}

.log-content {
  display: grid;
  grid-template-columns: 320px 1fr;
  gap: 12px;
}

.log-file-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  max-height: 300px;
  overflow-y: auto;
}

.log-file-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 6px 10px;
  background: $bg-tertiary;
  border: 1px solid transparent;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s ease;

  &:hover {
    background: $glass-bg;
  }

  &.active {
    border-color: $color-accent;
  }
}

.log-file-info {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.log-category {
  font-size: 10px;
  padding: 1px 5px;
  border-radius: 4px;
  flex-shrink: 0;
  background: rgba(255,255,255,0.1);
  color: rgba(255,255,255,0.6);

  &.cat-app {
    color: #00f5ff;
    background: rgba(0,245,255,0.12);
  }

  &.cat-comm {
    color: #ffb74d;
    background: rgba(255,183,77,0.12);
  }

  &.cat-crash {
    color: #ff5c7a;
    background: rgba(255,92,122,0.12);
  }
}

.log-file-name {
  font-size: 12px;
  color: rgba(255,255,255,0.8);
  font-family: monospace;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.log-file-meta {
  font-size: 10px;
  color: rgba(255,255,255,0.35);
  flex-shrink: 0;
}

.log-tail {
  background: rgba(0,0,0,0.25);
  border: 1px solid $glass-bg;
  border-radius: 8px;
  min-height: 260px;
  max-height: 300px;
  overflow: auto;

  .empty-state {
    height: 260px;
  }

  .log-tail-content {
    margin: 0;
    padding: 10px 12px;
    font-size: 11px;
    line-height: 1.5;
    font-family: Consolas, monospace;
    color: rgba(255,255,255,0.75);
    white-space: pre-wrap;
    word-break: break-all;
  }
}
</style>
