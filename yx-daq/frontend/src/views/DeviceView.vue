<template>
  <div class="device-view">
    <GlassCard title="设备管理" icon="📡">
      <template #actions>
        <el-button type="primary" size="small" @click="openAddDialog">添加设备</el-button>
        <el-button size="small" @click="scanDevices">扫描设备</el-button>
      </template>
      <el-table :data="deviceStore.statuses" class="device-table">
        <el-table-column prop="name" label="设备名称" min-width="140">
          <template #default="{ row }">
            <div class="device-name">
              <span class="name-text">{{ row.name }}</span>
              <el-tag size="small" type="info" class="device-type">{{ row.type }}</el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="连接状态" width="110" align="center">
          <template #default="{ row }">
            <div class="status-badge" :class="row.status === 'Connected' ? 'connected' : 'disconnected'">
              <span class="status-dot" />
              <span class="status-text">{{ row.status === 'Connected' ? '已连接' : '未连接' }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="acquiring" label="采集" min-width="100" align="center">
          <template #default="{ row }">
            <div v-if="row.acquiring" class="acquiring-badge">
              <span class="pulse-dot" />
              <span>采集中</span>
            </div>
            <span v-else class="idle-badge">--</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="220" align="right">
          <template #default="{ row }">
            <el-button-group class="action-group">
              <el-button size="small" @click="openEditDialog(row.id)">
                <el-icon><Edit /></el-icon>
              </el-button>
              <el-button v-if="row.status !== 'Connected'" type="primary" size="small" @click="handleConnect(row.id)">
                <el-icon><Link /></el-icon>
              </el-button>
              <el-button v-else type="warning" size="small" @click="handleDisconnect(row.id)">
                <el-icon><CircleClose /></el-icon>
              </el-button>
              <el-button size="small" type="danger" @click="removeDevice(row.id)">
                <el-icon><Delete /></el-icon>
              </el-button>
            </el-button-group>
          </template>
        </el-table-column>
      </el-table>
    </GlassCard>

    <!-- 添加设备对话框 -->
    <el-dialog v-model="showAddDialog" title="添加设备" width="420px" class="device-dialog">
      <div class="dialog-section">
        <div class="section-title">📡 基础信息</div>
        <el-form :model="newDevice" label-width="60px" size="small">
          <el-form-item label="名称">
            <el-input v-model="newDevice.name" placeholder="请输入设备名称" />
          </el-form-item>
          <el-form-item label="类型">
            <el-select v-model="newDevice.type" style="width: 100%">
              <el-option label="XY-DAQ8" value="XY-DAQ8" />
              <el-option label="XY-DAQ16" value="XY-DAQ16" />
              <el-option label="模拟设备" value="SIMULATED" />
            </el-select>
          </el-form-item>
        </el-form>
      </div>

      <div v-if="newDevice.type !== 'SIMULATED'" class="dialog-section">
        <div class="section-title">🔗 网络配置</div>
        <div class="form-row two-col">
          <div class="form-group">
            <label class="group-label">IP地址</label>
            <el-input v-model="newDevice.host" placeholder="192.168.3.101" size="small" />
          </div>
          <div class="form-group">
            <label class="group-label">端口</label>
            <el-input-number v-model="newDevice.port" :min="1" :max="65535" size="small" controls-position="right" class="num-md" />
          </div>
        </div>
      </div>

      <div class="dialog-section">
        <div class="section-title">🔌 连接选项</div>
        <div class="form-row">
          <div class="form-group auto-connect-row">
            <span class="auto-connect-label">添加后自动连接</span>
            <el-switch v-model="newDevice.autoConnect" size="small" />
          </div>
        </div>
      </div>

      <div class="dialog-section">
        <div class="section-title">⚙️ 采集参数</div>
        <div class="form-row three-col">
          <div class="form-group">
            <label class="group-label">采样频率</label>
            <div class="input-with-unit">
              <el-input-number v-model="newDevice.publishRate" :min="1" :max="100" :step="1" size="small" controls-position="right" class="num-sm" />
              <span class="unit">Hz</span>
            </div>
          </div>
          <div class="form-group">
            <label class="group-label">单位</label>
            <el-select v-model="newDevice.unit" filterable allow-create size="small" class="sel-sm">
              <el-option v-for="u in unitOptions" :key="u" :label="u" :value="u" />
            </el-select>
          </div>
          <div class="form-group">
            <label class="group-label">精度</label>
            <el-input-number v-model="newDevice.precision" :min="0" :max="6" size="small" controls-position="right" class="num-xs" />
          </div>
        </div>
        <div class="param-hint">
          单位/精度适用于 CH1-CH{{ getPressureCount(newDevice.type) }}
        </div>
      </div>

      <template #footer>
        <el-button @click="showAddDialog = false">取消</el-button>
        <el-button type="primary" :loading="adding" @click="addDevice">确定</el-button>
      </template>
    </el-dialog>

    <!-- 编辑设备对话框 -->
    <el-dialog v-model="showEditDialog" title="编辑设备" width="720px" class="device-dialog">
      <div class="dialog-section">
        <div class="section-title">📡 基础信息</div>
        <div class="form-row three-col">
          <div class="form-group">
            <label class="group-label">设备名</label>
            <el-input v-model="editForm.name" size="small" />
          </div>
          <div class="form-group">
            <label class="group-label">IP地址</label>
            <el-input v-model="editForm.host" size="small" />
          </div>
          <div class="form-group">
            <label class="group-label">端口</label>
            <el-input-number v-model="editForm.port" :min="1" :max="65535" size="small" controls-position="right" class="num-md" />
          </div>
        </div>
        <div class="form-row two-col-top">
          <div class="form-group">
            <label class="group-label">采样频率</label>
            <div class="input-with-unit">
              <el-input-number v-model="editForm.publishRate" :min="1" :max="100" :step="1" size="small" controls-position="right" class="num-sm" />
              <span class="unit">Hz</span>
            </div>
          </div>
          <div class="form-group auto-connect-row">
            <span class="auto-connect-label">自动连接</span>
            <el-switch v-model="editForm.autoConnect" size="small" />
          </div>
        </div>
      </div>

      <div class="dialog-section">
        <div class="section-title">⚙️ 通道参数</div>
        <div class="form-row three-col">
          <div class="form-group">
            <label class="group-label">压力单位</label>
            <div class="input-with-unit">
              <el-select v-model="editForm.unit" filterable allow-create size="small" class="sel-sm">
                <el-option v-for="u in unitOptions" :key="u" :label="u" :value="u" />
              </el-select>
              <span class="hint-text">CH1-CH{{ editPressureCount }}</span>
            </div>
          </div>
          <div class="form-group">
            <label class="group-label">精度</label>
            <div class="input-with-unit">
              <el-input-number v-model="editForm.precision" :min="0" :max="6" size="small" controls-position="right" class="num-xs" />
              <span class="hint-text">所有通道</span>
            </div>
          </div>
          <div class="form-group">
            <label class="group-label">特殊通道</label>
            <span class="special-channels">CH{{ editPressureCount + 1 }}: 大气压 | CH{{ editPressureCount + 2 }}: 大气温度</span>
          </div>
        </div>
      </div>

      <!-- 通道编辑表格 -->
      <div class="dialog-section">
        <div class="section-title">📋 通道配置</div>
        <el-table :data="editChannels" size="small" class="channel-table" :max-height="320">
          <el-table-column prop="index" label="#" width="45" align="center">
            <template #default="{ row }">
              <span class="channel-index">{{ row.index }}</span>
            </template>
          </el-table-column>
          <el-table-column label="通道名" width="100">
            <template #default="{ row }">
              <el-input v-model="row.name" size="small" />
            </template>
          </el-table-column>
          <el-table-column label="启用" width="65" align="center">
            <template #default="{ row }">
              <el-switch v-model="row.enabled" size="small" />
            </template>
          </el-table-column>
          <el-table-column label="单位" width="65" align="center">
            <template #default="{ row }">
              <span class="readonly-text">{{ row.unit }}</span>
            </template>
          </el-table-column>
          <el-table-column label="精度" width="50" align="center">
            <template #default="{ row }">
              <span class="readonly-text">{{ row.precision }}</span>
            </template>
          </el-table-column>
          <el-table-column label="量程下限" width="105" align="center">
            <template #default="{ row }">
              <el-input-number v-model="row.rangeMin" size="small" controls-position="right" style="width: 85px" />
            </template>
          </el-table-column>
          <el-table-column label="量程上限" width="105" align="center">
            <template #default="{ row }">
              <el-input-number v-model="row.rangeMax" size="small" controls-position="right" style="width: 85px" />
            </template>
          </el-table-column>
        </el-table>
        <div class="channel-hint">
          0-{{ editPressureCount - 1 }}: 压力通道 | {{ editPressureCount }}: 大气压 | {{ editPressureCount + 1 }}: 大气温度
        </div>
      </div>

      <template #footer>
        <el-button @click="showEditDialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveEdit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { Edit, Link, CircleClose, Delete } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useDeviceStore } from '../stores/device'
import GlassCard from '../components/GlassCard.vue'
import { DeviceService, DataService } from '../../bindings/yx-daq/internal/app'
import * as models from '../../bindings/yx-daq/internal/types'

const deviceStore = useDeviceStore()

function getPressureCount(type: string): number {
  if (type === 'XY-DAQ8') return 8
  return 16
}

function getTotalChannels(type: string): number {
  return getPressureCount(type) + 2
}

// ==================== 添加设备 ====================
const showAddDialog = ref(false)
const adding = ref(false)
const newDevice = ref({
  name: '',
  type: 'XY-DAQ16',
  host: '192.168.3.101',
  port: 9000,
  publishRate: 20,
  unit: 'kPa',
  precision: 3,
  autoConnect: true,
})

function openAddDialog() {
  newDevice.value = {
    name: '',
    type: 'XY-DAQ16',
    host: '192.168.3.101',
    port: 9000,
    publishRate: 20,
    unit: 'kPa',
    precision: 3,
    autoConnect: true,
  }
  showAddDialog.value = true
}

async function addDevice() {
  adding.value = true
  const id = `dev-${Date.now()}`
  const deviceName = newDevice.value.name || '新设备'
  try {
    const channels = []
    const pressureCount = getPressureCount(newDevice.value.type)
    const totalCh = getTotalChannels(newDevice.value.type)
    for (let i = 0; i < totalCh; i++) {
      const isAtmPressure = i === pressureCount
      const isAtmTemp = i === pressureCount + 1
      channels.push({
        index: i,
        name: i < pressureCount ? `CH${i+1}` : (isAtmPressure ? '大气压' : '大气温度'),
        enabled: true,
        unit: isAtmPressure ? 'kPa' : (isAtmTemp ? '°C' : newDevice.value.unit),
        precision: newDevice.value.precision,
        rangeMin: 0,
        rangeMax: 200,
      })
    }

    const profile = new models.DeviceProfile({
      id,
      name: deviceName,
      type: newDevice.value.type,
      host: newDevice.value.host,
      port: newDevice.value.port,
      streamId: 1,
      periodMs: Math.round(1000 / newDevice.value.publishRate),
      autoConnect: newDevice.value.autoConnect,
      channels,
    })

    await DeviceService.AddDeviceProfile(profile)

    try {
      await DataService.SetPublishRate(newDevice.value.publishRate)
    } catch {}

    showAddDialog.value = false
    ElMessage.success(`设备 "${deviceName}" 添加成功`)

    if (newDevice.value.autoConnect) {
      try {
        await DeviceService.ConnectDevice(id)
        ElMessage.success(`设备 "${deviceName}" 已连接`)
      } catch (connErr: any) {
        ElMessage.warning(`设备已添加，但连接失败: ${connErr?.message || connErr}`)
      }
    }

    await deviceStore.fetchProfiles()
    await deviceStore.fetchStatuses()
  } catch (e: any) {
    ElMessage.error(`添加设备失败: ${e?.message || e}`)
  } finally {
    adding.value = false
  }
}

// ==================== 编辑设备 ====================
const showEditDialog = ref(false)
const saving = ref(false)
const editForm = ref({
  id: '',
  name: '',
  host: '',
  port: 9000,
  publishRate: 20,
  unit: 'kPa',
  precision: 3,
  autoConnect: true,
})

interface EditChannel {
  index: number
  name: string
  enabled: boolean
  unit: string
  precision: number
  rangeMin: number
  rangeMax: number
}
const editChannels = ref<EditChannel[]>([])

const editPressureCount = computed(() => {
  return Math.max(editChannels.value.length - 2, 8)
})

const unitOptions = ['kPa', 'Pa', 'MPa', 'bar', 'mbar', 'mmHg', 'psi', '°C', '°F']

function openEditDialog(id: string) {
  const profile = deviceStore.profiles.find(p => p.id === id)
  if (!profile) {
    ElMessage.warning('未找到设备配置')
    return
  }
  const ch0Unit = profile.channels.length > 0 ? profile.channels[0].unit : 'kPa'
  const ch0Precision = profile.channels.length > 0 ? profile.channels[0].precision : 3
  const publishRate = profile.periodMs > 0 ? Math.round(1000 / profile.periodMs) : 20
  editForm.value = {
    id: profile.id,
    name: profile.name,
    host: profile.host,
    port: profile.port,
    publishRate,
    unit: ch0Unit,
    precision: ch0Precision,
    autoConnect: (profile as any).autoConnect !== false,
  }
  editChannels.value = profile.channels.map(c => ({ ...c }))

    DataService.GetPublishRate().then((rate: number) => {
      editForm.value.publishRate = rate
    }).catch(() => {})

  showEditDialog.value = true
}

function syncUnitToChannels() {
  const pc = editPressureCount.value
  for (const ch of editChannels.value) {
    if (ch.index < pc) {
      ch.unit = editForm.value.unit
    }
  }
}
function syncPrecisionToChannels() {
  for (const ch of editChannels.value) {
    ch.precision = editForm.value.precision
  }
}

watch(() => editForm.value.unit, () => syncUnitToChannels())
watch(() => editForm.value.precision, () => syncPrecisionToChannels())

async function saveEdit() {
  saving.value = true
  try {
    const profile = deviceStore.profiles.find(p => p.id === editForm.value.id)
    if (!profile) {
      ElMessage.error('设备配置不存在')
      return
    }

    const pc = editPressureCount.value
    const updatedChannels = editChannels.value.map(c => ({
      index: c.index,
      name: c.name,
      enabled: c.enabled,
      unit: c.index === pc ? 'kPa' : (c.index === pc + 1 ? '°C' : editForm.value.unit),
      precision: editForm.value.precision,
      rangeMin: c.rangeMin,
      rangeMax: c.rangeMax,
    }))

    const updatedProfile = new models.DeviceProfile({
      id: profile.id,
      name: editForm.value.name,
      type: profile.type,
      host: editForm.value.host,
      port: editForm.value.port,
      streamId: profile.streamId,
      periodMs: Math.round(1000 / editForm.value.publishRate),
      autoConnect: editForm.value.autoConnect,
      channels: updatedChannels,
    })

    const err = await deviceStore.updateProfile(updatedProfile as any)
    if (err) {
      ElMessage.error(`更新失败: ${err}`)
    } else {
      try {
        await DataService.SetPublishRate(editForm.value.publishRate)
      } catch {}

      if (editForm.value.autoConnect) {
        const oldStatus = deviceStore.statuses.find(s => s.id === editForm.value.id)
        if (!oldStatus || oldStatus.status !== 'Connected') {
          try {
            await DeviceService.ConnectDevice(editForm.value.id)
            ElMessage.success('设备已自动连接')
          } catch (connErr: any) {
            ElMessage.warning(`自动连接失败: ${connErr?.message || connErr}`)
          }
        }
      } else {
        try {
          await DeviceService.DisconnectDevice(editForm.value.id)
        } catch {}
      }

      showEditDialog.value = false
      ElMessage.success('设备配置已更新')
      await deviceStore.fetchStatuses()
    }
  } catch (e: any) {
    ElMessage.error(`更新失败: ${e?.message || e}`)
  } finally {
    saving.value = false
  }
}

// ==================== 设备操作 ====================
async function handleConnect(id: string) {
  const err = await deviceStore.connectDevice(id)
  if (err) {
    ElMessage.error(`连接失败: ${err}`)
  } else {
    ElMessage.success('设备已连接')
  }
}

async function handleDisconnect(id: string) {
  const err = await deviceStore.disconnectDevice(id)
  if (err) {
    ElMessage.error(`断开失败: ${err}`)
  } else {
    ElMessage.success('设备已断开')
  }
}

async function scanDevices() {
  try {
    const devices = await DeviceService.ScanDevices()
    if (devices && devices.length > 0) {
      ElMessage.success(`发现 ${devices.length} 个设备`)
    } else {
      ElMessage.info('未发现设备')
    }
  } catch (e: any) {
    ElMessage.error(`扫描失败: ${e?.message || e}`)
  }
}

async function removeDevice(id: string) {
  try {
    await DeviceService.RemoveDeviceProfile(id)
    await deviceStore.fetchProfiles()
    await deviceStore.fetchStatuses()
    ElMessage.success('设备已删除')
  } catch (e: any) {
    ElMessage.error(`删除失败: ${e?.message || e}`)
  }
}
</script>

<style lang="scss" scoped>
.device-view { display: flex; flex-direction: column; }

// ==================== 设备列表表格 ====================
.device-table {
  :deep(th) {
    font-size: 12px;
    font-weight: 600;
    color: rgba(255,255,255,0.7) !important;
    background: rgba(255,255,255,0.04) !important;
    padding: 10px 8px !important;
  }
  :deep(td) {
    font-size: 12px;
    padding: 10px 8px !important;
    color: rgba(255,255,255,0.8) !important;
  }
  :deep(tr) {
    background: transparent !important;
  }
  :deep(tr:hover) {
    background: rgba(255,255,255,0.04) !important;
  }
  :deep(.el-table__row--striped) {
    background: transparent !important;
  }
}

.device-name {
  display: flex;
  flex-direction: column;
  gap: 2px;
  .name-text {
    font-weight: 500;
    color: rgba(255,255,255,0.9);
  }
  .device-type {
    align-self: flex-start;
    font-size: 10px;
    height: 18px;
    padding: 0 6px;
  }
}

.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 3px 10px;
  border-radius: 12px;
  font-size: 11px;
  font-weight: 500;

  &.connected {
    background: rgba($color-success, 0.1);
    color: $color-success;
    .status-dot {
      background: $color-success;
      box-shadow: 0 0 4px rgba($color-success, 0.5);
    }
  }
  &.disconnected {
    background: $glass-bg;
    color: $text-tertiary;
    .status-dot {
      background: $text-muted;
    }
  }

  .status-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
  }
}

.acquiring-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 3px 10px;
  border-radius: 12px;
  background: rgba($color-accent, 0.1);
  color: $color-accent;
  font-size: 11px;
  font-weight: 500;
  white-space: nowrap;

  .pulse-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: $color-accent;
    animation: pulse 1.5s infinite;
  }
}

@keyframes pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.5; transform: scale(0.8); }
}

.idle-badge {
  color: rgba(255,255,255,0.3);
  font-size: 11px;
}

.action-group {
  .el-button {
    padding: 6px 10px;
  }
}

// ==================== 弹窗通用样式 ====================
:deep(.device-dialog) {
  .el-dialog__header {
    margin-right: 0;
    padding: 16px 20px;
    border-bottom: 1px solid rgba(255,255,255,0.08);
  }
  .el-dialog__title {
    font-size: 14px;
    font-weight: 600;
    color: rgba(255,255,255,0.9);
  }
  .el-dialog__body {
    padding: 16px 20px;
  }
}

.dialog-section {
  margin-bottom: 16px;
  padding: 12px;
  background: rgba(255,255,255,0.03);
  border-radius: 8px;
  border: 1px solid rgba(255,255,255,0.05);

  &:last-of-type {
    margin-bottom: 0;
  }
}

.section-title {
  font-size: 12px;
  font-weight: 600;
  color: rgba(255,255,255,0.85);
  margin-bottom: 12px;
}

// ==================== 表单布局 — 统一对齐 ====================
.form-row {
  display: flex;
  gap: 12px;

  &.two-col {
    > .form-group { flex: 1; }
  }

  &.two-col-top {
    > .form-group { flex: 1; }
  }

  &.three-col {
    > .form-group { flex: 1; }
  }
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.group-label {
  font-size: 11px;
  color: rgba(255,255,255,0.55);
  font-weight: 500;
}

.unit {
  font-size: 11px;
  color: rgba(255,255,255,0.4);
  margin-left: 4px;
}

.input-with-unit {
  display: flex;
  align-items: center;
}

.hint-text {
  font-size: 10px;
  color: rgba(255,255,255,0.35);
  margin-left: 6px;
  white-space: nowrap;
}

.param-hint {
  margin-top: 8px;
  font-size: 10px;
  color: rgba(255,255,255,0.35);
}

.special-channels {
  font-size: 10px;
  color: rgba(255,255,255,0.45);
  line-height: 28px;
}

.auto-connect-row {
  flex-direction: row;
  align-items: center;
  gap: 8px;
}

.auto-connect-label {
  font-size: 12px;
  color: rgba(255,255,255,0.75);
  font-weight: 500;
}

// ==================== 控件尺寸 ====================
.num-xs { width: 70px; }
.num-sm { width: 90px; }
.num-md { width: 100px; }
.sel-sm { width: 90px; }

// ==================== 通道表格 ====================
.channel-table {
  width: 100%;
  border-radius: 6px;
  overflow: hidden;

  :deep(th) {
    font-size: 11px;
    font-weight: 600;
    color: rgba(255,255,255,0.7) !important;
    background: rgba(255,255,255,0.08) !important;
    padding: 8px 4px !important;
  }
  :deep(td) {
    font-size: 11px;
    padding: 6px 4px !important;
    color: rgba(255,255,255,0.85) !important;
  }
  :deep(tr) {
    background: transparent !important;
  }
  :deep(tr:hover) {
    background: rgba(255,255,255,0.04) !important;
  }
  :deep(.el-table__row--striped) {
    background: transparent !important;
  }
}

.channel-index {
  font-family: monospace;
  font-size: 11px;
  color: rgba(255,255,255,0.5);
}

.channel-hint {
  margin-top: 8px;
  font-size: 10px;
  color: rgba(255,255,255,0.35);
}

.readonly-text {
  font-size: 11px;
  color: rgba(255,255,255,0.55);
}

.channel-table {
  :deep(.el-input__wrapper) {
    background-color: rgba(0, 0, 0, 0.3) !important;
  }
  :deep(.el-input-number__decrease),
  :deep(.el-input-number__increase) {
    background: rgba(255,255,255,0.08) !important;
    border-color: rgba(255,255,255,0.1) !important;
    color: rgba(255,255,255,0.6) !important;
  }
  :deep(.el-switch__core) {
    background-color: rgba(255,255,255,0.15) !important;
    border-color: rgba(255,255,255,0.1) !important;
  }
  :deep(.el-switch.is-checked .el-switch__core) {
    background-color: var(--color-accent) !important;
    border-color: var(--color-accent) !important;
  }
}
</style>
