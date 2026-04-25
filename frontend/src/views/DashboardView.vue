<template>
  <div class="dashboard">
    <!-- 左侧：设备列表 -->
    <aside class="device-sidebar">
      <div class="sidebar-title">
        <span class="sidebar-title-icon">📡</span>
        <span>设备列表</span>
      </div>
      <div class="acq-controls">
        <el-button
          :type="deviceStore.isAcquiring ? 'warning' : 'success'"
          size="small"
          :disabled="!hasConnectedDevice && !deviceStore.isAcquiring"
          @click="deviceStore.isAcquiring ? handleStopAcqAll() : handleStartAcqAll()"
        >{{ deviceStore.isAcquiring ? '停止采集' : '开始采集' }}</el-button>
        <el-button
          :type="isRecording ? 'danger' : 'primary'"
          size="small"
          :disabled="!deviceStore.isAcquiring && !isRecording"
          @click="isRecording ? handleStopRecording() : handleStartRecording()"
        >{{ isRecording ? '停止记录' : '开始记录' }}</el-button>
      </div>
      <div class="device-list">
        <div
          v-for="s in deviceStore.statuses"
          :key="s.id"
          class="device-item"
          :class="{ active: selectedDeviceId === s.id }"
          @click="selectedDeviceId = s.id"
        >
          <span
            class="device-light"
            :class="{
              connected: s.status === 'Connected',
              disconnected: s.status !== 'Connected',
              acquiring: s.acquiring,
            }"
          ></span>
          <div class="device-info">
            <span class="device-name">{{ s.name }}</span>
            <span class="device-type">{{ s.type }}</span>
          </div>
          <span v-if="s.acquiring" class="acq-tag">采集中</span>
          <span v-if="s.acquiring && isRecording" class="rec-tag">记录中</span>
        </div>
        <div v-if="deviceStore.statuses.length === 0" class="no-device">
          暂无设备，请先在设备管理中添加设备
        </div>
      </div>
    </aside>

    <!-- 右侧：选中设备的数据展示 -->
    <div class="data-area">
      <div v-if="!selectedDeviceId" class="no-data-hint">请从左侧选择一个设备查看实时数据</div>
      <template v-else>
        <!-- 实时压力数据 -->
        <GlassCard title="实时压力数据" icon="📊" class="chart-card">
          <template #actions>
            <el-popover
              placement="bottom-end"
              :width="200"
              trigger="click"
              v-model:visible="channelSelectorVisible"
            >
              <template #reference>
                <el-button size="small">通道选择</el-button>
              </template>
              <div class="channel-selector">
                <div class="selector-header">
                  <el-checkbox
                    :model-value="allChannelsSelected"
                    :indeterminate="!allChannelsSelected && visibleChannels.size > 0"
                    @change="toggleAllChannels"
                  >全选</el-checkbox>
                </div>
                <div class="selector-list">
                  <el-checkbox
                    v-for="ch in channelOptions"
                    :key="ch.index"
                    :model-value="visibleChannels.has(ch.index)"
                    @change="toggleChannel(ch.index)"
                  >{{ ch.label }}</el-checkbox>
                </div>
              </div>
            </el-popover>
            <el-button size="small" @click="clearHistory">清空</el-button>
          </template>
          <ChartPanel :option="chartOption" height="100%" class="chart-panel" />
        </GlassCard>

        <!-- 实时通道值 -->
        <GlassCard title="实时通道值" icon="📋" class="channel-card">
          <div class="channel-grid">
            <div class="device-channels">
              <div v-for="ch in selectedChannelConfigs" :key="ch.index" class="channel-item">
                <div class="ch-name">{{ ch.name }}</div>
                <ValueDisplay :value="getChannelValue(ch.index)" :precision="ch.precision" :unit="ch.unit" />
              </div>
            </div>
          </div>
        </GlassCard>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, shallowRef, triggerRef } from 'vue'
import { ElMessage } from 'element-plus'
import { useDeviceStore } from '../stores/device'
import GlassCard from '../components/GlassCard.vue'
import ChartPanel from '../components/ChartPanel.vue'
import ValueDisplay from '../components/ValueDisplay.vue'

// keep-alive 需要组件名匹配
defineOptions({ name: 'DashboardView' })

const deviceStore = useDeviceStore()

// ==================== 采集控制（全局） ====================
const hasConnectedDevice = computed(() => deviceStore.statuses.some(s => s.status === 'Connected'))

async function handleStartAcqAll() {
  try {
    const { StartAcquisitionAll } = await import('../../wailsjs/go/main/App')
    const count = await StartAcquisitionAll()
    await deviceStore.fetchStatuses()
    if (count > 0) {
      ElMessage.success(`已启动 ${count} 个设备采集`)
    } else {
      ElMessage.warning('没有已连接的设备可启动采集')
    }
  } catch (e: any) {
    ElMessage.error(`启动采集失败: ${e?.message || e}`)
  }
}

async function handleStopAcqAll() {
  try {
    const { StopAcquisitionAll } = await import('../../wailsjs/go/main/App')
    await StopAcquisitionAll()
    await deviceStore.fetchStatuses()
    ElMessage.success('已停止所有设备采集')
  } catch (e: any) {
    ElMessage.error(`停止采集失败: ${e?.message || e}`)
  }
}

// ==================== 录制控制 ====================
const isRecording = ref(false)

async function handleStartRecording() {
  try {
    const { StartRecording } = await import('../../wailsjs/go/main/App')
    await StartRecording()
    isRecording.value = true
    ElMessage.success('已开始记录数据')
  } catch (e: any) {
    ElMessage.error(`开始记录失败: ${e?.message || e}`)
  }
}

async function handleStopRecording() {
  try {
    const { StopRecording } = await import('../../wailsjs/go/main/App')
    await StopRecording()
    isRecording.value = false
    ElMessage.success('已停止记录')
  } catch (e: any) {
    ElMessage.error(`停止记录失败: ${e?.message || e}`)
  }
}

// 选中的设备ID
const selectedDeviceId = ref<string>('')

// 组件挂载时初始化选中的设备和通道
onMounted(() => {
  if (deviceStore.statuses.length > 0 && !selectedDeviceId.value) {
    selectedDeviceId.value = deviceStore.statuses[0].id
  }
  // 初始化通道选择（排除大气压力和大气温度）
  if (selectedDeviceId.value) {
    const profile = deviceStore.profiles.find(p => p.id === selectedDeviceId.value)
    if (profile) {
      const total = profile.channels.length
      visibleChannels.value = new Set(profile.channels.filter(ch => ch.enabled && !isAtmosphericChannel(ch, total)).map(ch => ch.index))
    }
  }
})

// 自动选中第一个设备（设备列表变化时）
watch(() => deviceStore.statuses, (statuses) => {
  if (statuses.length > 0 && !selectedDeviceId.value) {
    selectedDeviceId.value = statuses[0].id
  }
  // 如果选中的设备被删除了，重新选第一个
  if (selectedDeviceId.value && !statuses.find(s => s.id === selectedDeviceId.value)) {
    selectedDeviceId.value = statuses.length > 0 ? statuses[0].id : ''
  }
})

// 选中设备的快照数据
const selectedSnapshots = computed(() => {
  if (!selectedDeviceId.value) return []
  return deviceStore.snapshots.filter(s => s.deviceId === selectedDeviceId.value)
})

// 选中设备的通道配置（始终显示，无论是否有数据）
const selectedChannelConfigs = computed(() => {
  if (!selectedDeviceId.value) return []
  const profile = deviceStore.profiles.find(p => p.id === selectedDeviceId.value)
  if (!profile) return []
  return profile.channels.filter(ch => ch.enabled)
})

// 获取通道实时值（无数据时返回 undefined，ValueDisplay 会显示 --）
function getChannelValue(channelIndex: number): number | undefined {
  if (!selectedDeviceId.value) return undefined
  const snap = deviceStore.snapshots.find(s => s.deviceId === selectedDeviceId.value)
  if (!snap) return undefined
  const idx = snap.channelIndices.indexOf(channelIndex)
  if (idx < 0) return undefined
  return snap.channels[idx]
}

// 从profile中获取通道配置
function getChannelConfig(deviceId: string, channelIndex: number) {
  const profile = deviceStore.profiles.find(p => p.id === deviceId)
  if (profile && channelIndex >= 0 && channelIndex < profile.channels.length) {
    return profile.channels[channelIndex]
  }
  return null
}
function getChannelName(deviceId: string, channelIndex: number): string {
  const ch = getChannelConfig(deviceId, channelIndex)
  return ch ? ch.name : `CH${channelIndex + 1}`
}
function getChannelUnit(deviceId: string, channelIndex: number): string {
  const ch = getChannelConfig(deviceId, channelIndex)
  return ch ? ch.unit : 'kPa'
}
function getChannelPrecision(deviceId: string, channelIndex: number): number {
  const ch = getChannelConfig(deviceId, channelIndex)
  return ch ? ch.precision : 3
}

// 实时折线图数据历史
const MAX_POINTS = 100
const historyData = shallowRef<Record<string, number[]>>({})
const historyLabels = ref<string[]>([])

// 判断通道是否为大气压力/大气温度（不显示在波形图上）
function isAtmosphericChannel(ch: { name: string; index: number }, total: number): boolean {
  return ch.index >= total - 2
}

// 通道选择：记录哪些通道在波形图中显示
const visibleChannels = ref<Set<number>>(new Set())

// 通道选择下拉框开关
const channelSelectorVisible = ref(false)

// 所有可选通道列表（用通道配置中的名称）
const channelOptions = computed(() => {
  return selectedChannelConfigs.value.map(ch => ({
    index: ch.index,
    label: ch.name || `CH${ch.index + 1}`,
  }))
})

// 全选/全不选
const allChannelsSelected = computed(() => {
  if (channelOptions.value.length === 0) return false
  return channelOptions.value.every(ch => visibleChannels.value.has(ch.index))
})

function toggleAllChannels() {
  if (allChannelsSelected.value) {
    visibleChannels.value = new Set()
  } else {
    visibleChannels.value = new Set(channelOptions.value.map(ch => ch.index))
  }
  scheduleChartUpdate()
}

function toggleChannel(index: number) {
  const newSet = new Set(visibleChannels.value)
  if (newSet.has(index)) {
    newSet.delete(index)
  } else {
    newSet.add(index)
  }
  visibleChannels.value = newSet
  scheduleChartUpdate()
}

// 追加数据 - 只处理选中设备，且仅在采集中追加
let lastSnapTime = 0
watch(() => deviceStore.snapshots, (snaps) => {
  if (!selectedDeviceId.value) return
  // 未采集时不追加数据
  if (!deviceStore.isAcquiring) return
  const snap = snaps.find(s => s.deviceId === selectedDeviceId.value)
  if (!snap) return

  // 防止同一秒内重复追加数据点
  const now = Date.now()
  if (now - lastSnapTime < 500) return
  lastSnapTime = now

  const date = new Date(now)
  const label = `${date.getMinutes().toString().padStart(2,'0')}:${date.getSeconds().toString().padStart(2,'0')}`
  historyLabels.value.push(label)
  if (historyLabels.value.length > MAX_POINTS) {
    historyLabels.value.shift()
  }

  const data = historyData.value
  for (let i = 0; i < snap.channels.length; i++) {
    const chIndex = snap.channelIndices[i]
    const name = getChannelName(selectedDeviceId.value, chIndex)
    if (!data[name]) {
      data[name] = []
    }
    data[name].push(snap.channels[i])
    if (data[name].length > MAX_POINTS) {
      data[name].shift()
    }
  }
  triggerRef(historyData)

  // 标记图表需要更新（延迟批量刷新）
  scheduleChartUpdate()
})

const NEON_COLORS = ['#b829ff', '#00f5ff', '#00ff88', '#ffaa00', '#ff3366', '#00aaff', '#d966ff', '#66faff']

// 使用 shallowRef 避免深层响应式追踪，减少不必要的触发
const chartOption = shallowRef<Record<string, any>>({
  backgroundColor: 'transparent',
  tooltip: {
    trigger: 'axis',
    backgroundColor: 'rgba(10,10,26,0.9)',
    borderColor: 'rgba(184,41,255,0.3)',
    textStyle: { color: '#fff' },
  },
  legend: {
    data: [],
    textStyle: { color: 'rgba(255,255,255,0.7)', fontSize: 11 },
    top: 0,
  },
  grid: { left: 50, right: 20, top: 30, bottom: 30 },
  xAxis: {
    type: 'category',
    data: [],
    axisLine: { lineStyle: { color: 'rgba(255,255,255,0.1)' } },
    axisLabel: { color: 'rgba(255,255,255,0.4)', fontSize: 10 },
  },
  yAxis: {
    type: 'value',
    scale: true,
    axisLine: { lineStyle: { color: 'rgba(255,255,255,0.1)' } },
    axisLabel: { color: 'rgba(255,255,255,0.4)', fontSize: 10 },
    splitLine: { lineStyle: { color: 'rgba(255,255,255,0.05)' } },
  },
  series: [],
})

// 定时批量更新图表配置，避免高频重绘
let chartUpdateTimer: number | null = null
let chartDirty = false

function scheduleChartUpdate() {
  chartDirty = true
  if (chartUpdateTimer) return
  chartUpdateTimer = window.setTimeout(() => {
    chartUpdateTimer = null
    if (!chartDirty) return
    chartDirty = false
    updateChartOption()
  }, 200)
}

function updateChartOption() {
  const allEntries = Object.entries(historyData.value)
  const filteredEntries = allEntries.filter(([name]) => {
    const ch = selectedChannelConfigs.value.find(c => (c.name || `CH${c.index + 1}`) === name)
    return ch ? visibleChannels.value.has(ch.index) : true
  })

  const series = filteredEntries.map(([name, data], idx) => ({
    name,
    type: 'line',
    data,
    smooth: true,
    symbol: 'none',
    lineStyle: {
      width: 2,
      color: NEON_COLORS[idx % NEON_COLORS.length],
      shadowColor: NEON_COLORS[idx % NEON_COLORS.length],
      shadowBlur: 4,
    },
    itemStyle: {
      color: NEON_COLORS[idx % NEON_COLORS.length],
    },
  }))

  // 增量更新：只更新变化的数据部分，避免完整重建 option 导致闪烁
  chartOption.value = {
    legend: {
      data: filteredEntries.map(([name]) => name),
      textStyle: { color: 'rgba(255,255,255,0.7)', fontSize: 11 },
      top: 0,
    },
    xAxis: {
      data: historyLabels.value,
    },
    series,
  }
}

function clearHistory() {
  historyData.value = {}
  historyLabels.value = []
  // 清空时需要完整重建 option
  chartOption.value = {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      backgroundColor: 'rgba(10,10,26,0.9)',
      borderColor: 'rgba(184,41,255,0.3)',
      textStyle: { color: '#fff' },
    },
    legend: {
      data: [],
      textStyle: { color: 'rgba(255,255,255,0.7)', fontSize: 11 },
      top: 0,
    },
    grid: { left: 50, right: 20, top: 30, bottom: 30 },
    xAxis: {
      type: 'category',
      data: [],
      axisLine: { lineStyle: { color: 'rgba(255,255,255,0.1)' } },
      axisLabel: { color: 'rgba(255,255,255,0.4)', fontSize: 10 },
    },
    yAxis: {
      type: 'value',
      scale: true,
      axisLine: { lineStyle: { color: 'rgba(255,255,255,0.1)' } },
      axisLabel: { color: 'rgba(255,255,255,0.4)', fontSize: 10 },
      splitLine: { lineStyle: { color: 'rgba(255,255,255,0.05)' } },
    },
    series: [],
  }
}

// 当通道配置加载完成但 visibleChannels 尚未初始化时，自动选中所有通道（排除大气压力和大气温度）
watch(selectedChannelConfigs, (configs) => {
  if (configs.length > 0 && visibleChannels.value.size === 0) {
    const profile = deviceStore.profiles.find(p => p.id === selectedDeviceId.value)
    const total = profile ? profile.channels.length : 0
    visibleChannels.value = new Set(configs.filter(ch => !isAtmosphericChannel(ch, total)).map(ch => ch.index))
    scheduleChartUpdate()
  }
})

// 切换设备时清空历史并重置通道选择
watch(selectedDeviceId, (newId, oldId) => {
  // 只在用户主动切换设备时清空（oldId 有值说明是切换而非初始化）
  if (!oldId) return
  clearHistory()
  // 默认选中所有通道（排除大气压力和大气温度）
  const profile = deviceStore.profiles.find(p => p.id === newId)
  if (profile) {
    const total = profile.channels.length
    visibleChannels.value = new Set(profile.channels.filter(ch => ch.enabled && !isAtmosphericChannel(ch, total)).map(ch => ch.index))
  } else {
    visibleChannels.value = new Set()
  }
})

// 停止采集时清空波形图历史数据
watch(() => deviceStore.isAcquiring, (acquiring) => {
  if (!acquiring) {
    clearHistory()
  }
})
</script>

<style lang="scss" scoped>
.dashboard {
  display: flex;
  height: calc(100vh - 48px - 32px);
  gap: 16px;
}

.device-sidebar {
  width: 220px;
  min-width: 220px;
  background: var(--bg-secondary, rgba(255,255,255,0.06));
  border: 1px solid var(--border-color, rgba(255,255,255,0.12));
  border-radius: 12px;
  backdrop-filter: blur(16px);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.sidebar-title {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 14px 16px;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary, #ffffff);
  border-bottom: 1px solid var(--border-color, rgba(255,255,255,0.12));
}

.sidebar-title-icon {
  font-size: 18px;
}

.device-list {
  flex: 1;
  overflow-y: auto;
  padding: 8px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.device-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  border-radius: 8px;
  background: rgba(255,255,255,0.04);
  border: 1px solid rgba(255,255,255,0.08);
  cursor: pointer;
  transition: all 0.2s;

  &:hover {
    background: rgba(255,255,255,0.08);
    border-color: rgba(0,245,255,0.3);
  }

  &.active {
    background: rgba(0,245,255,0.1);
    border-color: rgba(0,245,255,0.5);
    box-shadow: 0 0 12px rgba(0,245,255,0.15);
  }
}

.device-light {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;

  &.connected {
    background: #00ff88;
    box-shadow: 0 0 8px rgba(0,255,136,0.6);
  }

  &.disconnected {
    background: #666;
  }

  &.acquiring {
    background: #00f5ff;
    box-shadow: 0 0 8px rgba(0,245,255,0.6);
    animation: lightPulse 1.5s ease-in-out infinite;
  }
}

@keyframes lightPulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.5; transform: scale(0.8); }
}

.device-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
}

.device-name {
  font-size: 13px;
  color: rgba(255,255,255,0.85);
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.device-type {
  font-size: 11px;
  color: rgba(255,255,255,0.4);
}

.acq-tag {
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 10px;
  background: rgba(0,245,255,0.15);
  color: #00f5ff;
  flex-shrink: 0;
}

.rec-tag {
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 10px;
  background: rgba(255,51,102,0.15);
  color: #ff3366;
  flex-shrink: 0;
}

.acq-controls {
  display: flex;
  gap: 8px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--border-color, rgba(255,255,255,0.12));
}

.no-device {
  color: rgba(255,255,255,0.3);
  text-align: center;
  padding: 20px;
}

.data-area {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 16px;
  overflow: hidden;
}

.chart-card {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;

  :deep(.card-body) {
    flex: 1;
    min-height: 0;
    overflow: hidden;
  }
}

.chart-panel {
  width: 100%;
  height: 100%;
}

.channel-card {
  flex-shrink: 0;
}

.no-data-hint {
  color: rgba(255,255,255,0.3);
  text-align: center;
  padding: 20px;
}

.channel-grid { display: flex; gap: 16px; flex-wrap: wrap; }
.device-channels { display: flex; flex-wrap: wrap; gap: 8px; }
.channel-item {
  background: rgba(255,255,255,0.04); border: 1px solid rgba(255,255,255,0.08);
  border-radius: 8px; padding: 8px 12px; width: 120px; text-align: center;
  flex-shrink: 0;
}
.ch-name { font-size: 11px; color: rgba(255,255,255,0.5); margin-bottom: 4px; height: 16px; line-height: 16px; }

.channel-selector {
  .selector-header {
    padding-bottom: 8px;
    border-bottom: 1px solid rgba(255,255,255,0.1);
    margin-bottom: 8px;
  }
  .selector-list {
    display: flex;
    flex-direction: column;
    gap: 4px;
    max-height: 240px;
    overflow-y: auto;
  }
}
</style>
