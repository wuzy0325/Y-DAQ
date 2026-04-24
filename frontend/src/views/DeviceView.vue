<template>
  <div class="device-view">
    <GlassCard title="设备管理" icon="📡">
      <template #actions>
        <el-button type="primary" size="small" @click="openAddDialog">添加设备</el-button>
        <el-button size="small" @click="scanDevices">扫描设备</el-button>
      </template>
      <el-table :data="deviceStore.statuses" style="width: 100%" dark>
        <el-table-column prop="name" label="名称" width="150" />
        <el-table-column prop="type" label="类型" width="120" />
        <el-table-column prop="status" label="状态" width="120">
          <template #default="{ row }">
            <StatusIndicator
              :status="row.status === 'Connected' ? 'connected' : 'disconnected'"
              :label="row.status"
            />
          </template>
        </el-table-column>
        <el-table-column prop="acquiring" label="采集" width="80">
          <template #default="{ row }">
            <StatusIndicator v-if="row.acquiring" status="running" label="采集中" :animated="true" />
            <span v-else class="idle-text">停止</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="280">
          <template #default="{ row }">
            <el-button size="small" @click="openEditDialog(row.id)">编辑</el-button>
            <el-button v-if="row.status !== 'Connected'" type="primary" size="small" @click="handleConnect(row.id)">连接</el-button>
            <el-button v-else type="danger" size="small" @click="handleDisconnect(row.id)">断开</el-button>
            <el-button size="small" type="danger" @click="removeDevice(row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </GlassCard>

    <!-- 添加设备对话框 -->
    <el-dialog v-model="showAddDialog" title="添加设备" width="460px" :append-to-body="true">
      <el-form :model="newDevice" label-width="64px" size="small" class="compact-form">
        <el-form-item label="名称">
          <el-input v-model="newDevice.name" />
        </el-form-item>
        <el-form-item label="类型">
          <el-select v-model="newDevice.type" style="width: 100%">
            <el-option label="XY-DAQ16" value="XY-DAQ16" />
            <el-option label="模拟设备" value="SIMULATED" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="newDevice.type === 'XY-DAQ16'" label="IP地址">
          <el-input v-model="newDevice.host" placeholder="192.168.3.101" />
        </el-form-item>
        <el-form-item v-if="newDevice.type === 'XY-DAQ16'" label="端口">
          <el-input-number v-model="newDevice.port" :min="1" :max="65535" style="width:100%" />
        </el-form-item>
        <el-form-item label="采样频率">
          <div class="inline-field">
            <el-input-number v-model="newDevice.publishRate" :min="1" :max="100" :step="1" style="flex:1" />
            <span class="form-hint">Hz</span>
          </div>
        </el-form-item>
        <el-form-item label="单位">
          <div class="inline-field">
            <el-select v-model="newDevice.unit" filterable allow-create style="width:120px">
              <el-option v-for="u in unitOptions" :key="u" :label="u" :value="u" />
            </el-select>
            <span class="form-hint">CH1-16统一</span>
          </div>
        </el-form-item>
        <el-form-item label="精度">
          <div class="inline-field">
            <el-input-number v-model="newDevice.precision" :min="0" :max="6" style="width:90px" />
            <span class="form-hint">所有通道统一</span>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddDialog = false">取消</el-button>
        <el-button type="primary" :loading="adding" @click="addDevice">确定</el-button>
      </template>
    </el-dialog>

    <!-- 编辑设备对话框 -->
    <el-dialog v-model="showEditDialog" title="编辑设备" width="780px" :append-to-body="true">
      <!-- 基础信息：双列紧凑布局 -->
      <div class="edit-basic-grid">
        <el-form :model="editForm" label-width="64px" size="small" class="compact-form">
          <el-form-item label="设备名">
            <el-input v-model="editForm.name" />
          </el-form-item>
          <el-form-item label="IP地址">
            <el-input v-model="editForm.host" />
          </el-form-item>
        </el-form>
        <el-form :model="editForm" label-width="64px" size="small" class="compact-form">
          <el-form-item label="端口号">
            <el-input-number v-model="editForm.port" :min="1" :max="65535" style="width:100%" />
          </el-form-item>
          <el-form-item label="采样频率">
            <div class="inline-field">
              <el-input-number v-model="editForm.publishRate" :min="1" :max="100" :step="1" style="flex:1" />
              <span class="form-hint">Hz</span>
            </div>
          </el-form-item>
        </el-form>
      </div>
      <div class="edit-unit-row">
        <div class="unit-field">
          <span class="field-label">单位</span>
          <el-select v-model="editForm.unit" filterable allow-create size="small" style="width:120px">
            <el-option v-for="u in unitOptions" :key="u" :label="u" :value="u" />
          </el-select>
          <span class="form-hint">CH1-16统一，大气压kPa，温度°C</span>
        </div>
        <div class="precision-field">
          <span class="field-label">精度</span>
          <el-input-number v-model="editForm.precision" :min="0" :max="6" size="small" style="width:90px" />
          <span class="form-hint">所有通道统一</span>
        </div>
      </div>

      <!-- 通道编辑表格 -->
      <div class="channel-section">
        <div class="channel-header">通道配置</div>
        <el-table :data="editChannels" size="small" border class="channel-table" :max-height="340">
          <el-table-column prop="index" label="#" width="36" align="center" />
          <el-table-column label="通道名" width="90">
            <template #default="{ row }">
              <el-input v-model="row.name" size="small" />
            </template>
          </el-table-column>
          <el-table-column label="启用" width="50" align="center">
            <template #default="{ row }">
              <el-switch v-model="row.enabled" size="small" />
            </template>
          </el-table-column>
          <el-table-column label="单位" width="60" align="center">
            <template #default="{ row }">
              <span class="readonly-text">{{ row.unit }}</span>
            </template>
          </el-table-column>
          <el-table-column label="精度" width="44" align="center">
            <template #default="{ row }">
              <span class="readonly-text">{{ row.precision }}</span>
            </template>
          </el-table-column>
          <el-table-column label="量程下限" width="95" align="center">
            <template #default="{ row }">
              <el-input-number v-model="row.rangeMin" size="small" controls-position="right" style="width:80px" />
            </template>
          </el-table-column>
          <el-table-column label="量程上限" width="95" align="center">
            <template #default="{ row }">
              <el-input-number v-model="row.rangeMax" size="small" controls-position="right" style="width:80px" />
            </template>
          </el-table-column>
        </el-table>
        <div class="channel-hint">
          0-15: CH1-CH16 (压力) | 16: 大气压 | 17: 大气温度
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
import { ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { useDeviceStore } from '../stores/device'
import GlassCard from '../components/GlassCard.vue'
import StatusIndicator from '../components/StatusIndicator.vue'

const deviceStore = useDeviceStore()

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
  }
  showAddDialog.value = true
}

async function addDevice() {
  adding.value = true
  const id = `dev-${Date.now()}`
  const deviceName = newDevice.value.name || '新设备'
  try {
    const { AddDeviceProfile, ConnectDevice } = await import('../../wailsjs/go/main/App')
    const { types } = await import('../../wailsjs/go/models')

    const channels = []
    for (let i = 0; i < 18; i++) {
      const isAtmPressure = i === 16
      const isAtmTemp = i === 17
      channels.push({
        index: i,
        name: i < 16 ? `CH${i+1}` : (isAtmPressure ? '大气压' : '大气温度'),
        enabled: true,
        unit: isAtmPressure ? 'kPa' : (isAtmTemp ? '°C' : newDevice.value.unit),
        precision: newDevice.value.precision,
        rangeMin: 0,
        rangeMax: 200,
      })
    }

    const profile = new types.DeviceProfile({
      id,
      name: deviceName,
      type: newDevice.value.type,
      host: newDevice.value.host,
      port: newDevice.value.port,
      streamId: 1,
      periodMs: Math.round(1000 / newDevice.value.publishRate),
      channels,
    })

    await AddDeviceProfile(profile)

    // 设置发布频率
    try {
      const { SetPublishRate } = await import('../../wailsjs/go/main/App')
      await SetPublishRate(newDevice.value.publishRate)
    } catch {}

    showAddDialog.value = false
    ElMessage.success(`设备 "${deviceName}" 添加成功`)

    // 尝试自动连接
    try {
      await ConnectDevice(id)
      ElMessage.success(`设备 "${deviceName}" 已连接`)
    } catch (connErr: any) {
      ElMessage.warning(`设备已添加，但连接失败: ${connErr?.message || connErr}`)
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
})

// 通道编辑数据（深拷贝，独立编辑）
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

// 常用单位选项
const unitOptions = ['kPa', 'Pa', 'MPa', 'bar', 'mbar', 'mmHg', 'psi', '°C', '°F']

function openEditDialog(id: string) {
  const profile = deviceStore.profiles.find(p => p.id === id)
  if (!profile) {
    ElMessage.warning('未找到设备配置')
    return
  }
  // 从 CH0 提取统一单位，从任意通道提取统一精度
  const ch0Unit = profile.channels.length > 0 ? profile.channels[0].unit : 'kPa'
  const ch0Precision = profile.channels.length > 0 ? profile.channels[0].precision : 3
  // 从 periodMs 反推采样频率
  const publishRate = profile.periodMs > 0 ? Math.round(1000 / profile.periodMs) : 20
  editForm.value = {
    id: profile.id,
    name: profile.name,
    host: profile.host,
    port: profile.port,
    publishRate,
    unit: ch0Unit,
    precision: ch0Precision,
  }
  // 深拷贝通道配置
  editChannels.value = profile.channels.map(c => ({ ...c }))

  // 异步加载当前发布频率
  import('../../wailsjs/go/main/App').then(({ GetPublishRate }) => {
    GetPublishRate().then((rate: number) => {
      editForm.value.publishRate = rate
    }).catch(() => {})
  })

  showEditDialog.value = true
}

// 当统一单位或精度变化时，同步到通道表格
function syncUnitToChannels() {
  for (const ch of editChannels.value) {
    if (ch.index < 16) {
      ch.unit = editForm.value.unit
    }
    // index 16: 大气压 固定 kPa, index 17: 大气温度 固定 °C
  }
}
function syncPrecisionToChannels() {
  for (const ch of editChannels.value) {
    ch.precision = editForm.value.precision
  }
}

// 监听 editForm.unit 和 editForm.precision 变化，同步到通道
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

    const { types } = await import('../../wailsjs/go/models')
    // 按规则构建通道配置：CH1-CH16 用统一单位，大气压固定kPa，大气温度固定°C，精度统一
    const updatedChannels = editChannels.value.map(c => ({
      index: c.index,
      name: c.name,
      enabled: c.enabled,
      unit: c.index === 16 ? 'kPa' : (c.index === 17 ? '°C' : editForm.value.unit),
      precision: editForm.value.precision,
      rangeMin: c.rangeMin,
      rangeMax: c.rangeMax,
    }))

    const updatedProfile = new types.DeviceProfile({
      id: profile.id,
      name: editForm.value.name,
      type: profile.type,
      host: editForm.value.host,
      port: editForm.value.port,
      streamId: profile.streamId,
      periodMs: Math.round(1000 / editForm.value.publishRate),
      channels: updatedChannels,
    })

    const err = await deviceStore.updateProfile(updatedProfile as any)
    if (err) {
      ElMessage.error(`更新失败: ${err}`)
    } else {
      // 更新发布频率
      try {
        const { SetPublishRate } = await import('../../wailsjs/go/main/App')
        await SetPublishRate(editForm.value.publishRate)
      } catch {}

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
    const { ScanDevices } = await import('../../wailsjs/go/main/App')
    const devices = await ScanDevices()
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
    const { RemoveDeviceProfile } = await import('../../wailsjs/go/main/App')
    await RemoveDeviceProfile(id)
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
.idle-text { font-size: 12px; color: rgba(255,255,255,0.4); }

.form-hint {
  margin-left: 6px;
  font-size: 11px;
  color: rgba(255,255,255,0.4);
  white-space: nowrap;
}

// 编辑对话框：双列基础信息
.edit-basic-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0 16px;
  margin-bottom: 8px;
}

.compact-form {
  :deep(.el-form-item) { margin-bottom: 8px; }
  :deep(.el-form-item__label) { font-size: 12px; padding-right: 6px; }
}

.inline-field {
  display: flex;
  align-items: center;
  gap: 4px;
  width: 100%;
}

// 单位/精度行
.edit-unit-row {
  display: flex;
  align-items: center;
  gap: 20px;
  padding: 6px 0 8px;
  border-bottom: 1px solid rgba(255,255,255,0.06);
  margin-bottom: 8px;

  .unit-field, .precision-field {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .field-label {
    font-size: 12px;
    color: rgba(255,255,255,0.5);
    white-space: nowrap;
  }
}

.channel-section {
  margin-top: 4px;
}
.channel-header {
  font-size: 12px;
  font-weight: 600;
  margin-bottom: 6px;
  color: rgba(255,255,255,0.7);
}
.channel-table {
  width: 100%;
  :deep(.el-table__cell) { padding: 2px 0; }
}
.channel-hint {
  margin-top: 4px;
  font-size: 11px;
  color: rgba(255,255,255,0.35);
}
.readonly-text {
  font-size: 11px;
  color: rgba(255,255,255,0.55);
}
</style>
