<template>
  <GlassCard title="温度校准" icon="🌡️">
    <div class="temp-calib-panel">
      <!-- 顶部工具栏：设备选择 + 通道多选 + 点数 + 采点 + 去校准 -->
      <div class="calib-toolbar">
        <div class="toolbar-left">
          <span class="toolbar-label">设备</span>
          <el-select
            :model-value="store.selectedDeviceId"
            placeholder="选择温度设备"
            size="small"
            class="device-select"
            @change="handleDeviceChange"
          >
            <el-option
              v-for="d in store.tempDevices"
              :key="d.id"
              :label="`${d.name} (${d.type})`"
              :value="d.id"
            />
          </el-select>
          <span class="toolbar-label">点数</span>
          <el-input-number
            :model-value="store.pointCount"
            :min="MIN_POINTS"
            :max="MAX_POINTS"
            :step="1"
            size="small"
            class="point-count-input"
            @update:model-value="handlePointCountChange"
          />
        </div>
        <div class="toolbar-right">
          <el-button
            type="primary"
            size="small"
            :loading="store.sampling"
            :disabled="!canSample"
            :title="sampleHint"
            @click="handleSample"
          >采点</el-button>
          <el-button
            size="small"
            :disabled="!hasAnyTempCalib"
            @click="handleClearAll"
          >全部去校准</el-button>
        </div>
      </div>

      <div v-if="store.tempDevices.length === 0" class="empty-state">
        <el-icon class="empty-icon"><Warning /></el-icon>
        <span>未发现温度设备（EA2516T），请先在设备管理中添加</span>
      </div>

      <template v-else>
        <!-- 通道多选表格 -->
        <div class="channel-table-section">
          <el-table
            :data="store.channels"
            size="small"
            height="200"
            border
            @selection-change="handleSelectionChange"
          >
            <el-table-column type="selection" width="40" :selectable="() => true" />
            <el-table-column prop="index" label="通道" width="60" />
            <el-table-column prop="name" label="名称" min-width="100" />
            <el-table-column prop="thermocoupleType" label="热电偶类型" width="120">
              <template #default="{ row }">
                <span>{{ row.thermocoupleType || '--' }}</span>
              </template>
            </el-table-column>
            <el-table-column label="已采点" width="90">
              <template #default="{ row }">
                <span>{{ store.pointsProgress(row.index).current }} / {{ store.pointCount }}</span>
              </template>
            </el-table-column>
            <el-table-column label="校准状态" width="160">
              <template #default="{ row }">
                <el-tag v-if="row.tempCalibratedAt" type="success" size="small">
                  已校准 ({{ formatR2(row.tempCalibR2) }})
                </el-tag>
                <el-tag v-else type="info" size="small">未校准</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="120">
              <template #default="{ row }">
                <el-button
                  size="small"
                  link
                  type="primary"
                  @click="store.setActiveChannel(row.index)"
                >校准</el-button>
                <el-button
                  v-if="row.tempCalibratedAt"
                  size="small"
                  link
                  type="warning"
                  @click="handleClearChannel(row.index)"
                >去校准</el-button>
              </template>
            </el-table-column>
          </el-table>
        </div>

        <!-- 通道切换 tabs + 点表格 + 拟合结果 -->
        <div v-if="store.activeChannelIndex >= 0" class="active-channel-section">
          <div class="channel-tabs">
            <div
              v-for="ch in selectedOrHasPointsChannels"
              :key="ch.index"
              class="channel-tab"
              :class="{ active: ch.index === store.activeChannelIndex }"
              @click="store.setActiveChannel(ch.index)"
            >
              <span>CH{{ ch.index }}</span>
              <span class="tab-progress">
                {{ store.pointsProgress(ch.index).current }}/{{ store.pointCount }}
              </span>
            </div>
          </div>

          <div class="point-table-wrapper">
            <div class="point-table-header">
              <span class="header-title">
                通道 {{ store.activeChannelIndex }} 校准点
              </span>
              <el-button
                type="primary"
                size="small"
                :disabled="store.activePoints.length < MIN_POINTS"
                @click="handleFit"
              >拟合</el-button>
            </div>

            <el-table :data="store.activePoints" size="small" border height="180">
              <el-table-column type="index" label="#" width="50" />
              <el-table-column label="实测均值" prop="measured">
                <template #default="{ row }">
                  {{ formatNumber(row.measured) }}
                </template>
              </el-table-column>
              <el-table-column label="参考温度" prop="reference">
                <template #default="{ row, $index }">
                  <el-input-number
                    v-model="row.reference"
                    size="small"
                    :controls="false"
                    :precision="2"
                    class="reference-input"
                    @change="onReferenceChange($index)"
                  />
                </template>
              </el-table-column>
              <el-table-column label="操作" width="80">
                <template #default="{ $index }">
                  <el-button size="small" link type="danger" @click="store.removePoint(store.activeChannelIndex, $index)">删除</el-button>
                </template>
              </el-table-column>
            </el-table>

            <!-- 拟合结果 -->
            <div class="fit-result-area">
              <div v-if="store.activeFitResult" class="fit-result-display">
                <div class="fit-formula">y = {{ formatNumber(store.activeFitResult.a) }} × x + {{ formatNumber(store.activeFitResult.b) }}</div>
                <div class="fit-meta">
                  <span>R² = {{ formatR2(store.activeFitResult.r2) }}</span>
                  <span>点数 = {{ store.activeFitResult.points }}</span>
                  <el-tag v-if="store.activeFitResult.r2 < 0.99" type="warning" size="small">R²偏低</el-tag>
                </div>
              </div>
              <div v-else-if="store.activePersisted?.tempCalibratedAt" class="fit-result-display">
                <div class="fit-formula persisted">
                  已写入: y = {{ formatNumber(store.activePersisted.tempCalibA) }} × x + {{ formatNumber(store.activePersisted.tempCalibB) }}
                </div>
                <div class="fit-meta">
                  <span>R² = {{ formatR2(store.activePersisted.tempCalibR2) }}</span>
                  <span>点数 = {{ store.activePersisted.tempCalibPoints }}</span>
                  <el-tag type="success" size="small">已生效</el-tag>
                </div>
              </div>
              <div v-else class="fit-empty">尚未拟合</div>

              <div class="fit-actions">
                <el-button
                  type="success"
                  size="small"
                  :disabled="!store.activeFitResult"
                  @click="handleWrite"
                >写入</el-button>
                <el-button
                  size="small"
                  :disabled="!store.activePersisted?.tempCalibratedAt"
                  @click="handleClearChannel(store.activeChannelIndex)"
                >去校准</el-button>
              </div>
            </div>
          </div>
        </div>
      </template>
    </div>
  </GlassCard>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Warning } from '@element-plus/icons-vue'
import { useTemperatureCalibStore, MIN_POINTS, MAX_POINTS } from '../stores/temperatureCalib'
import { useDeviceStore } from '../stores/device'
import GlassCard from './GlassCard.vue'

const store = useTemperatureCalibStore()
const deviceStore = useDeviceStore()

// 派生：当前选中设备的连接状态（用于采点前置条件判断）
const selectedDeviceStatus = computed(() => {
  if (!store.selectedDeviceId) return null
  return deviceStore.statuses.find(s => s.id === store.selectedDeviceId) ?? null
})

// 派生：是否允许采点
// 前置条件（与压力校零一致）：设备已连接 + 正在采集 + 已勾选通道 + 非采点中
// 否则后端无法获取数据帧，用户会先输入参考温度再收到错误，体验差
const canSample = computed(() => {
  if (!store.selectedDeviceId) return false
  if (store.selectedChannelIndices.length === 0) return false
  if (store.sampling) return false
  const status = selectedDeviceStatus.value
  return status?.status === 'Connected' && status?.acquiring
})

// 派生：采点按钮禁用时的 hover 提示原因
const sampleHint = computed(() => {
  if (!store.selectedDeviceId) return '请先选择温度设备'
  if (store.selectedChannelIndices.length === 0) return '请先在通道表勾选至少一个通道'
  if (store.sampling) return '采点中...'
  const status = selectedDeviceStatus.value
  if (!status) return '设备状态未知，请稍候'
  if (status.status !== 'Connected') return `设备未连接（当前: ${status.status}），请先在设备管理中连接`
  if (!status.acquiring) return '设备未启动采集，请先在设备管理中启动采集'
  return ''
})

// 派生：当前设备是否存在任何已校准通道
const hasAnyTempCalib = computed(() => {
  return store.channels.some(c => c.tempCalibratedAt)
})

// 派生：tabs 应显示的通道（已勾选或已有点的）
const selectedOrHasPointsChannels = computed(() => {
  const set = new Set<number>(store.selectedChannelIndices)
  // 也包含已有点的通道
  for (const idx of store.pointsByChannel.keys()) {
    set.add(idx)
  }
  // 也包含已校准的通道（用户可切过去查看/去校准）
  for (const c of store.channels) {
    if (c.tempCalibratedAt) set.add(c.index)
  }
  return store.channels
    .filter(c => set.has(c.index))
    .sort((a, b) => a.index - b.index)
})

function handleDeviceChange(id: string) {
  store.selectDevice(id)
}

function handlePointCountChange(n: number | undefined) {
  // el-input-number 在清空时会传 undefined，此处按 MIN_POINTS 兜底
  store.setPointCount(n ?? MIN_POINTS)
}

function handleSelectionChange(rows: { index: number }[]) {
  store.setSelectedChannels(rows.map(r => r.index))
}

async function handleSample() {
  // 弹窗输入参考温度
  let reference = 0
  try {
    const result = await ElMessageBox.prompt('请输入当前参考温度（°C）', '采点', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      inputPattern: /^-?\d+(\.\d+)?$/,
      inputErrorMessage: '请输入有效数字',
      inputValue: '0',
    })
    reference = parseFloat(result.value)
    if (Number.isNaN(reference)) {
      ElMessage.error('参考温度无效')
      return
    }
  } catch {
    return // 用户取消
  }
  ElMessage({ message: '正在采样（10 帧）...', type: 'warning', duration: 1200 })
  const r = await store.samplePoints(reference)
  if (r.success) {
    ElMessage.success(r.message)
  } else {
    ElMessage.error(r.message)
  }
}

async function handleFit() {
  const r = await store.fitActive()
  if (r.success) {
    ElMessage.success(r.message)
  } else {
    ElMessage.error(r.message)
  }
}

async function handleWrite() {
  const r = await store.writeActive()
  if (r.success) {
    ElMessage.success(r.message)
  } else {
    ElMessage.error(r.message)
  }
}

async function handleClearChannel(channelIndex: number) {
  try {
    await ElMessageBox.confirm(
      `确定要清除通道 ${channelIndex} 的温度校准吗？`,
      '去校准',
      { type: 'warning', confirmButtonText: '确定', cancelButtonText: '取消' },
    )
  } catch {
    return
  }
  const r = await store.clearChannel(channelIndex)
  if (r.success) {
    ElMessage.success(r.message)
  } else {
    ElMessage.error(r.message)
  }
}

async function handleClearAll() {
  if (!store.selectedDeviceId) return
  try {
    await ElMessageBox.confirm(
      `确定要清除当前设备所有通道的温度校准吗？`,
      '全部去校准',
      { type: 'warning', confirmButtonText: '确定', cancelButtonText: '取消' },
    )
  } catch {
    return
  }
  const r = await store.clearAll()
  if (r.success) {
    ElMessage.success(r.message)
  } else {
    ElMessage.error(r.message)
  }
}

// 用户编辑参考温度后，清除旧的拟合预演（点变了，旧结果失效）
function onReferenceChange(_index: number) {
  store.invalidateActiveFit()
}

function formatNumber(v: number | undefined): string {
  if (v === undefined || v === null || Number.isNaN(v)) return '--'
  return v.toFixed(6)
}

function formatR2(v: number | undefined): string {
  if (v === undefined || v === null || Number.isNaN(v)) return '--'
  return v.toFixed(4)
}

onMounted(() => {
  store.refreshProfiles()
})
</script>

<style lang="scss" scoped>
.temp-calib-panel {
  display: flex;
  flex-direction: column;
  gap: $spacing-md;
  padding: $spacing-xs;
}

.calib-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: $spacing-lg;
  background: $bg-tertiary;
  border: 1px solid $glass-border-light;
  border-radius: $border-radius-sm;
  padding: $spacing-sm $spacing-md;
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: $spacing-sm + 2px;
  flex: 1;
  min-width: 0;
}

.toolbar-right {
  display: flex;
  gap: $spacing-sm;
}

.toolbar-label {
  font-size: $font-size-xs;
  color: $text-tertiary;
  white-space: nowrap;
}

.device-select {
  width: 200px;
}

.point-count-input {
  width: 100px;
}

.channel-table-section {
  background: $bg-tertiary;
  border: 1px solid $glass-border-light;
  border-radius: $border-radius-sm;
  padding: $spacing-sm;
}

.active-channel-section {
  display: flex;
  flex-direction: column;
  gap: $spacing-sm;
  background: $bg-tertiary;
  border: 1px solid $glass-border-light;
  border-radius: $border-radius-sm;
  padding: $spacing-sm $spacing-md;
}

.channel-tabs {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
  padding-bottom: $spacing-sm - 2px;
  border-bottom: 1px solid $glass-border-light;
}

.channel-tab {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: $spacing-xs $spacing-sm + 2px;
  background: $glass-bg-elevated;
  border: 1px solid $glass-border-light;
  border-radius: $border-radius-sm - 2px;
  cursor: pointer;
  transition: all $transition-fast;
  min-width: 60px;

  &:hover {
    background: $glass-bg-hover;
  }

  &.active {
    background: $glass-bg-active;
    border-color: $color-accent;

    span:first-child {
      color: $color-accent;
      font-weight: 600;
    }
  }

  span:first-child {
    font-size: $font-size-xs;
    color: $text-secondary;
  }

  .tab-progress {
    font-size: $font-size-xs - 1px;
    color: $text-muted;
    margin-top: 2px;
  }
}

.point-table-wrapper {
  display: flex;
  flex-direction: column;
  gap: $spacing-sm;
}

.point-table-header {
  display: flex;
  align-items: center;
  justify-content: space-between;

  .header-title {
    font-size: $font-size-sm;
    color: $text-secondary;
    font-weight: 600;
  }
}

.reference-input {
  width: 100%;
}

.fit-result-area {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: $spacing-md;
  padding: $spacing-sm $spacing-md;
  background: $glass-bg-elevated;
  border-radius: $border-radius-sm - 2px;
}

.fit-result-display {
  display: flex;
  flex-direction: column;
  gap: $spacing-xs;
  flex: 1;
}

.fit-formula {
  font-family: $font-family-mono;
  font-size: $font-size-sm;
  color: $color-accent;
  font-weight: 600;

  &.persisted {
    color: $text-secondary;
  }
}

.fit-meta {
  display: flex;
  align-items: center;
  gap: $spacing-md;
  font-size: $font-size-xs - 1px;
  color: $text-muted;
}

.fit-empty {
  font-size: $font-size-xs;
  color: $text-muted;
  flex: 1;
}

.fit-actions {
  display: flex;
  gap: $spacing-sm;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: $spacing-sm;
  padding: $spacing-2xl + $spacing-lg $spacing-xl;
  color: $text-muted;

  .empty-icon {
    font-size: $font-size-2xl + $spacing-sm;
    opacity: 0.5;
  }

  span {
    font-size: $font-size-xs;
  }
}
</style>
