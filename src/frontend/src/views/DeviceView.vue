<template>
  <div class="device-view">
    <GlassCard title="设备管理" icon="📡">
      <template #actions>
        <el-button type="primary" size="small" @click="openAddDialog">添加设备</el-button>
        <el-button size="small" @click="scanDevices">扫描设备</el-button>
        <el-button
          type="warning"
          size="small"
          :loading="batchZeroCalibrating"
          :disabled="!batchZeroCalibrating && !canBatchZeroCalibrate"
          @click="handleBatchZeroCalibrate"
        >批量校零</el-button>
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
        <el-table-column prop="status" label="连接状态" width="120" align="center">
          <template #default="{ row }">
            <div class="status-badge" :class="statusClass(row.status)">
              <span class="status-dot" :class="{ pulse: row.status === 'Connecting' }" />
              <span class="status-text">{{ statusLabel(row.status) }}</span>
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
        <el-table-column label="阀位" width="120" align="center">
          <template #default="{ row }">
            <el-select
              v-if="isValveSupported(row.type)"
              :model-value="valveStates[row.id]"
              :loading="valveLoading[row.id]"
              :disabled="!!valveDisabledReason(row)"
              size="small"
              style="width: 95px"
              :placeholder="valvePlaceholder(row.id)"
              @change="(val) => handleSetValve(row, val)"
            >
              <el-option
                v-for="opt in valveOptions"
                :key="opt.value"
                :label="opt.label"
                :value="opt.value"
              />
            </el-select>
            <span v-else class="readonly-text">--</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="260" align="right">
          <template #default="{ row }">
            <el-button-group class="action-group">
              <el-button title="编辑设备" size="small" @click="openEditDialog(row.id)">
                <el-icon><Edit /></el-icon>
              </el-button>
              <el-button v-if="row.status !== 'Connected'" title="连接设备" type="primary" size="small" :loading="deviceStore.isDeviceConnecting(row.id)" @click="handleConnect(row.id)">
                <el-icon v-if="!deviceStore.isDeviceConnecting(row.id)"><Link /></el-icon>
              </el-button>
              <el-button v-else title="断开连接" type="warning" size="small" @click="handleDisconnect(row.id)">
                <el-icon><CircleClose /></el-icon>
              </el-button>
              <el-button
                v-if="row.status === 'Connected' || row.acquiring"
                :title="row.acquiring ? '停止采集' : '启动采集'"
                :type="row.acquiring ? 'warning' : 'success'"
                size="small"
                :loading="deviceStore.isAcqBusy(row.id)"
                :disabled="deviceStore.isAcqBusy(row.id)"
                @click="handleToggleAcq(row.id)"
              >
                <el-icon v-if="!deviceStore.isAcqBusy(row.id)">
                  <VideoPause v-if="row.acquiring" />
                  <VideoPlay v-else />
                </el-icon>
              </el-button>
              <el-button title="删除设备" size="small" type="danger" :disabled="deviceStore.isDeviceConnecting(row.id)" @click="removeDevice(row.id)">
                <el-icon><Delete /></el-icon>
              </el-button>
            </el-button-group>
          </template>
        </el-table-column>
      </el-table>
    </GlassCard>

    <!-- 添加设备对话框 -->
    <el-dialog v-model="showAddDialog" title="添加设备" width="420px" :append-to-body="true" class="device-dialog">
      <div class="dialog-section">
        <div class="section-title">📡 基础信息</div>
        <el-form :model="newDevice" label-width="60px" size="small">
          <el-form-item label="名称">
            <el-input v-model="newDevice.name" placeholder="请输入设备名称" />
          </el-form-item>
          <el-form-item label="类型">
            <el-select v-model="newDevice.type" style="width: 100%">
              <el-option label="EA2508A" value="EA2508A" />
              <el-option label="EA2516A" value="EA2516A" />
              <el-option label="EA2516T (热电偶)" value="EA2516T" />
              <el-option label="模拟设备" value="SIMULATED" />
            </el-select>
          </el-form-item>
        </el-form>
      </div>

      <div v-if="newDevice.type !== 'SIMULATED'" class="dialog-section">
        <div class="section-title">🔗 网络配置</div>
        <div class="form-row">
          <div class="form-group">
            <label class="group-label">IP地址</label>
            <el-input v-model="newDevice.host" placeholder="192.168.3.101" size="small" style="width: 140px" />
          </div>
          <div class="form-group">
            <label class="group-label">端口</label>
            <el-input-number v-model="newDevice.port" :min="1" :max="65535" size="small" style="width: 100px" controls-position="right" />
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
              <el-input-number v-model="newDevice.publishRate" :min="1" :max="1000" :step="1" size="small" style="width: 90px" controls-position="right" />
              <span class="unit">Hz</span>
            </div>
          </div>
          <div class="form-group">
            <label class="group-label">单位</label>
            <el-select v-model="newDevice.unit" filterable allow-create size="small" style="width: 90px">
              <el-option v-for="u in unitOptions" :key="u" :label="u" :value="u" />
            </el-select>
          </div>
          <div class="form-group">
            <label class="group-label">精度</label>
            <el-input-number v-model="newDevice.precision" :min="0" :max="6" size="small" style="width: 70px" controls-position="right" />
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
    <el-dialog v-model="showEditDialog" title="编辑设备" width="720px" :append-to-body="true" class="device-dialog">
      <div class="dialog-section">
        <div class="section-title">📡 基础信息</div>
        <div class="form-row">
          <div class="form-group">
            <label class="group-label">设备名</label>
            <el-input v-model="editForm.name" size="small" style="width: 160px" />
          </div>
          <div class="form-group">
            <label class="group-label">IP地址</label>
            <el-input v-model="editForm.host" size="small" style="width: 140px" />
          </div>
          <div class="form-group">
            <label class="group-label">端口</label>
            <el-input-number v-model="editForm.port" :min="1" :max="65535" size="small" style="width: 90px" controls-position="right" />
          </div>
          <div class="form-group">
            <label class="group-label">采样频率</label>
            <div class="input-with-unit">
              <el-input-number v-model="editForm.publishRate" :min="1" :max="1000" :step="1" size="small" style="width: 90px" controls-position="right" />
              <span class="unit">Hz</span>
            </div>
          </div>
        </div>
        <div class="form-row" style="margin-top: 12px">
          <div class="form-group auto-connect-row">
            <span class="auto-connect-label">自动连接</span>
            <el-switch v-model="editForm.autoConnect" size="small" />
          </div>
        </div>
      </div>

      <div class="dialog-section">
        <div class="section-title">⚙️ 通道参数</div>
        <div class="form-row">
          <div v-if="editProfileType !== 'EA2516T'" class="form-group">
            <label class="group-label">压力单位</label>
            <el-select v-model="editForm.unit" filterable allow-create size="small" style="width: 100px">
              <el-option v-for="u in unitOptions" :key="u" :label="u" :value="u" />
            </el-select>
            <span class="hint-text">CH1-CH{{ editPressureCount }}</span>
          </div>
          <div v-else class="form-group">
            <label class="group-label">热电偶类型</label>
            <el-select v-model="editForm.thermocoupleType" size="small" style="width: 120px" @change="syncThermocoupleTypeToChannels">
              <el-option v-for="opt in thermocoupleTypeOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
            </el-select>
            <span class="hint-text">统一设置所有通道</span>
          </div>
          <div class="form-group">
            <label class="group-label">精度</label>
            <el-input-number v-model="editForm.precision" :min="0" :max="6" size="small" style="width: 70px" controls-position="right" />
            <span class="hint-text">所有通道</span>
          </div>
          <div class="form-group">
            <label class="group-label">特殊通道</label>
            <span class="special-channels" v-if="editProfileType !== 'EA2516T'">CH{{ editPressureCount + 1 }}: 大气压 | CH{{ editPressureCount + 2 }}: 大气温度</span>
            <span class="special-channels" v-else>16 通道热电偶温度</span>
          </div>
        </div>
      </div>

      <!-- 通道编辑表格 -->
      <div class="channel-section">
        <div class="channel-section-header">
          <div class="section-title">📋 通道配置</div>
          <div class="batch-zero-actions" v-if="isPressureDevice(editProfileType)">
            <el-button
              size="small"
              type="primary"
              :loading="zeroCalibrating"
              :disabled="!canZeroCalibrate"
              @click="handleZeroCalibrateAll"
            >批量校零</el-button>
            <el-button
              size="small"
              type="warning"
              :disabled="!hasAnyZeroOffset"
              @click="handleClearAllZeroOffsets"
            >批量去校零</el-button>
          </div>
        </div>
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
          <el-table-column v-if="editProfileType === 'EA2516T'" label="热电偶" width="100" align="center">
            <template #default="{ row }">
              <el-select v-model="row.thermocoupleType" size="small" style="width: 80px" @change="onChannelThermocoupleChange(row)">
                <el-option v-for="opt in thermocoupleTypeOptions" :key="opt.value" :label="opt.value" :value="opt.value" />
              </el-select>
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
          <el-table-column v-if="isPressureDevice(editProfileType)" label="零位" width="130" align="center">
            <template #default="{ row }">
              <template v-if="row.index < editPressureCount">
                <div v-if="row.zeroCalibratedAt" class="zero-info">
                  <span class="zero-value">{{ formatZeroValue(row.zeroOffset) }} {{ row.zeroOffsetUnit }}</span>
                  <span class="zero-time">{{ formatZeroTime(row.zeroCalibratedAt) }}</span>
                </div>
                <span v-else class="readonly-text">未校准</span>
              </template>
              <span v-else class="readonly-text">--</span>
            </template>
          </el-table-column>
          <el-table-column v-if="isPressureDevice(editProfileType)" label="校零" width="80" align="center">
            <template #default="{ row }">
              <template v-if="row.index < editPressureCount">
                <el-button
                  v-if="!row.zeroCalibratedAt"
                  size="small"
                  type="primary"
                  link
                  :loading="zeroCalibrating"
                  :disabled="!canZeroCalibrate"
                  @click="handleZeroCalibrateChannel(row.index)"
                >校零</el-button>
                <el-button
                  v-else
                  size="small"
                  type="warning"
                  link
                  :disabled="zeroCalibrating"
                  @click="handleClearZeroOffset(row.index)"
                >去校零</el-button>
              </template>
            </template>
          </el-table-column>
        </el-table>
        <div class="channel-hint" v-if="isPressureDevice(editProfileType)">
          0-{{ editPressureCount - 1 }}: 压力通道 | {{ editPressureCount }}: 大气压 | {{ editPressureCount + 1 }}: 大气温度
        </div>
        <div class="channel-hint" v-else>
          0-15: 温度通道（热电偶）
        </div>
      </div>

      <template #footer>
        <el-button @click="showEditDialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveEdit">保存</el-button>
      </template>
    </el-dialog>

    <!-- 批量校零结果对话框 -->
    <el-dialog v-model="showBatchZeroResult" title="批量校零结果" width="720px" :append-to-body="true" class="device-dialog">
      <div class="batch-zero-summary">
        <span class="summary-item success">成功 {{ batchZeroSummary.success }}</span>
        <span class="summary-item failed">失败 {{ batchZeroSummary.failed }}</span>
      </div>
      <el-table :data="batchZeroResults" size="small" class="channel-table" :max-height="320">
        <el-table-column prop="deviceName" label="设备" min-width="140" />
        <el-table-column label="结果" width="100" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.success" type="success" size="small">成功</el-tag>
            <el-tag v-else type="danger" size="small">失败</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="通道数" width="80" align="center">
          <template #default="{ row }">
            <span v-if="row.success">{{ row.channels }}</span>
            <span v-else class="readonly-text">--</span>
          </template>
        </el-table-column>
        <el-table-column prop="error" label="错误信息" min-width="180">
          <template #default="{ row }">
            <span v-if="row.error" class="error-text">{{ row.error }}</span>
            <span v-else class="readonly-text">--</span>
          </template>
        </el-table-column>
      </el-table>
      <template #footer>
        <el-button type="primary" @click="showBatchZeroResult = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, computed, reactive } from 'vue'
import { Edit, Link, CircleClose, Delete, VideoPlay, VideoPause } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useDeviceStore } from '../stores/device'
import type { ZeroCalibrateResult } from '../stores/device'
import { getDeviceInfo, thermocoupleTypeOptions, getThermocoupleRange } from '../api/enums'
import type { DeviceTypeValue } from '../api/enums'
import GlassCard from '../components/GlassCard.vue'
import { DeviceService, DataService } from '@bindings/yx-daq/internal/app'
import * as types from '@bindings/yx-daq/internal/types'

const deviceStore = useDeviceStore()

// isPressureDevice 是否为压力设备（含 SIMULATED，排除温度设备 EA2516T）。
// 用于零位校准/零位偏移 UI 可见性判断：EA2508A/EA2516A/SIMULATED 均属压力设备。
// 区别于 isValveSupported（仅真实 DAQ，排除 SIMULATED）。
function isPressureDevice(type: string): boolean {
  const info = getDeviceInfo(type as DeviceTypeValue)
  return !info.isTemperature
}

// ==================== 阀位控制 ====================
// 阀位状态按需查询：设备已连接时从硬件读取，未连接或读取失败显示"未知"
// 阀位不持久化到本地，每次重新连接后需重新查询
// 仅压力设备（EA2508A/EA2516A/SIMULATED）支持阀控，EA2516T 温度设备不支持
const valveStates = reactive<Record<string, string>>({}) // id -> 'Calibration' | 'Measurement' | 'Unknown'
const valveLoading = reactive<Record<string, boolean>>({})

// 可选阀位枚举（Unknown 不作为可选项，仅作为查询失败/未初始化的占位显示）
const valveOptions = [
  { label: 'A-测量位', value: 'Measurement' },
  { label: 'B-校准吹扫位', value: 'Calibration' },
]

function isValveSupported(type: string): boolean {
  // 仅真实压力设备支持阀控（EA2508A/EA2516A），排除温度设备和模拟设备
  const info = getDeviceInfo(type as DeviceTypeValue)
  return info.isRealDAQ && !info.isTemperature
}

// placeholder：阀位已知时为空（select 显示已选值），未知时显示"未知"
function valvePlaceholder(id: string): string {
  return valveStates[id] ? '' : '未知'
}

// 禁用原因：返回非空字符串时 select 禁用
function valveDisabledReason(row: { id: string; status: string; acquiring: boolean }): string {
  if (row.status !== 'Connected') return '设备未连接'
  if (row.acquiring) return '采集进行中，不允许切换阀位'
  return ''
}

// 选择阀位（来自下拉枚举）
async function handleSetValve(row: { id: string; status: string; acquiring: boolean }, target: string) {
  const reason = valveDisabledReason(row)
  if (reason) {
    ElMessage.warning(reason)
    return
  }
  if (target !== 'Calibration' && target !== 'Measurement') return
  valveLoading[row.id] = true
  try {
    await DeviceService.SetValveState(row.id, target as any)
    valveStates[row.id] = target
    ElMessage.success(`已切换到${target === 'Calibration' ? 'B-校准吹扫位' : 'A-测量位'}`)
  } catch (e: any) {
    ElMessage.error(`切换阀位失败: ${e?.message || e}`)
    // 切换失败时重新查询真实状态
    refreshValveState(row.id).catch(() => {})
  } finally {
    valveLoading[row.id] = false
  }
}

// 查询单个设备阀位（设备已连接时调用）
async function refreshValveState(id: string) {
  try {
    const state = await DeviceService.ReadValveState(id) as string
    valveStates[id] = state || 'Unknown'
  } catch (e) {
    valveStates[id] = 'Unknown'
  }
}

// 监听设备状态变化，已连接的设备自动查询阀位
watch(
  () => deviceStore.statuses,
  (statuses) => {
    for (const s of statuses) {
      if (s.status === 'Connected' && isValveSupported(s.type) && !valveStates[s.id]) {
        refreshValveState(s.id)
      }
    }
  },
  { deep: true },
)

// 设备断开连接时清除缓存的阀位状态，下次重连时重新查询
watch(
  () => deviceStore.statuses.map(s => `${s.id}:${s.status}`).join(','),
  () => {
    for (const [id] of Object.entries(valveStates)) {
      const ds = deviceStore.statuses.find(s => s.id === id)
      if (!ds || ds.status !== 'Connected') {
        delete valveStates[id]
      }
    }
  },
)

// 连接状态映射
function statusClass(status: string): string {
  switch (status) {
    case 'Connected': return 'connected'
    case 'Connecting': return 'connecting'
    case 'Error': return 'error'
    default: return 'disconnected'
  }
}

function statusLabel(status: string): string {
  switch (status) {
    case 'Connected': return '已连接'
    case 'Connecting': return '连接中'
    case 'Error': return '连接错误'
    default: return '未连接'
  }
}

// 根据设备类型获取压力通道数
function getPressureCount(type: string): number {
  return getDeviceInfo(type as DeviceTypeValue).pressureChCount
}

// 根据设备类型获取总通道数（压力+大气压+大气温度）
function getTotalChannels(type: string): number {
  return getDeviceInfo(type as DeviceTypeValue).totalChCount
}

function isTemperatureDevice(type: string): boolean {
  return getDeviceInfo(type as DeviceTypeValue).isTemperature
}

// ==================== 添加设备 ====================
const showAddDialog = ref(false)
const adding = ref(false)
const newDevice = ref({
  name: '',
  type: 'EA2516A',
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
    type: 'EA2516A',
    host: '192.168.3.101',
    port: 9000,
    publishRate: 20,
    unit: 'kPa',
    precision: 3,
    autoConnect: true,
  }
  showAddDialog.value = true
}

watch(() => newDevice.value.type, (newType) => {
  const info = getDeviceInfo(newType as DeviceTypeValue)
  newDevice.value.host = info.defaultHost
  newDevice.value.port = info.defaultPort
  newDevice.value.unit = info.defaultUnit
})

async function addDevice() {
  adding.value = true
  const id = `dev-${Date.now()}`
  const deviceName = newDevice.value.name || '新设备'
  try {
    const channels = []
    const info = getDeviceInfo(newDevice.value.type as DeviceTypeValue)
    const totalCh = info.totalChCount
    for (let i = 0; i < totalCh; i++) {
      if (info.isTemperature) {
        const tcRange = getThermocoupleRange('K')
        channels.push({
          index: i,
          name: `CH${i+1}`,
          enabled: true,
          unit: '°C',
          precision: newDevice.value.precision,
          rangeMin: tcRange.min,
          rangeMax: tcRange.max,
          thermocoupleType: 'K',
        })
      } else {
        const isAtmPressure = i === info.pressureChCount
        const isAtmTemp = i === info.pressureChCount + 1
        channels.push({
          index: i,
          name: i < info.pressureChCount ? `CH${i+1}` : (isAtmPressure ? '大气压' : '大气温度'),
          enabled: true,
          unit: isAtmPressure ? 'Pa' : (isAtmTemp ? '°C' : newDevice.value.unit),
          precision: newDevice.value.precision,
          rangeMin: 0,
          rangeMax: 200,
        })
      }
    }

    const profile = new types.DeviceProfile({
      id,
      name: deviceName,
      type: newDevice.value.type as any,
      host: newDevice.value.host,
      port: newDevice.value.port,
      streamId: 1,
      periodMs: Math.round(1000 / newDevice.value.publishRate),
      autoConnect: newDevice.value.autoConnect,
      channels,
    })

    await DeviceService.AddDeviceProfile(profile)

    // 设置发布频率
    try {
      await DataService.SetPublishRate(newDevice.value.publishRate)
    } catch {}

    showAddDialog.value = false
    ElMessage.success(`设备 "${deviceName}" 添加成功`)

    if (newDevice.value.autoConnect) {
      const connErr = await deviceStore.connectDevice(id)
      if (connErr) {
        ElMessage.warning(`设备已添加，但连接失败: ${connErr}`)
      } else {
        ElMessage.success(`设备 "${deviceName}" 已连接`)
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
  thermocoupleType: 'K',
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
  thermocoupleType: string
  zeroOffset?: number
  zeroOffsetUnit?: string
  zeroCalibratedAt?: number
}
const editChannels = ref<EditChannel[]>([])
const editProfileType = ref('')

// 编辑中的设备压力通道数（从通道配置推断）
const editPressureCount = computed(() => {
  if (editProfileType.value === 'EA2516T') return 16
  return Math.max(editChannels.value.length - 2, 8)
})

// 常用压力单位选项（仅零位校准白名单内 6 种，删除 mmHg/atm/mbar）
const unitOptions = ['psi', 'kgf/cm²', 'bar', 'kPa', 'MPa', 'Pa']

function openEditDialog(id: string) {
  const profile = deviceStore.profiles.find(p => p.id === id)
  if (!profile) {
    ElMessage.warning('未找到设备配置')
    return
  }
  // 从 CH0 提取统一单位，从任意通道提取统一精度
  const ch0Unit = profile.channels.length > 0 ? profile.channels[0].unit : 'Pa'
  const ch0TcType = profile.channels.length > 0 && profile.channels[0].thermocoupleType
    ? profile.channels[0].thermocoupleType
    : 'K'
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
    autoConnect: (profile as any).autoConnect !== false,
    thermocoupleType: ch0TcType,
  }
  editProfileType.value = profile.type
  // 深拷贝通道配置
  editChannels.value = profile.channels.map(c => ({ ...c, thermocoupleType: c.thermocoupleType || 'K' }))

  // publishRate 已从 profile.periodMs 反推得到正确值，不再调用 DataService.GetPublishRate() 异步覆盖。
  // GetPublishRate 返回的是 AcquisitionHub 的全局发布频率（所有设备的最小值，且受 100Hz 上限截断），
  // 会覆盖当前设备的真实配置，导致"采样频率"输入框在对话框打开后可见地闪烁跳变。
  showEditDialog.value = true
}

// 当统一单位或精度变化时，同步到通道表格
function syncUnitToChannels() {
  if (editProfileType.value === 'EA2516T') {
    for (const ch of editChannels.value) {
      ch.unit = '°C'
    }
    return
  }
  const pc = editPressureCount.value
  for (const ch of editChannels.value) {
    if (ch.index < pc) {
      ch.unit = editForm.value.unit
    }
    // 大气压通道固定 Pa, 大气温度通道固定 °C
  }
}
function syncPrecisionToChannels() {
  for (const ch of editChannels.value) {
    ch.precision = editForm.value.precision
  }
}

// 当统一热电偶类型变化时，同步到通道表格（含温度量程）
function syncThermocoupleTypeToChannels() {
  if (editProfileType.value !== 'EA2516T') return
  const range = getThermocoupleRange(editForm.value.thermocoupleType)
  for (const ch of editChannels.value) {
    ch.thermocoupleType = editForm.value.thermocoupleType
    ch.rangeMin = range.min
    ch.rangeMax = range.max
  }
}

// 单个通道热电偶类型变化（同步更新该通道温度量程）
function onChannelThermocoupleChange(row: EditChannel) {
  const range = getThermocoupleRange(row.thermocoupleType)
  row.rangeMin = range.min
  row.rangeMax = range.max
  const types = new Set(editChannels.value.map(c => c.thermocoupleType))
  if (types.size === 1) {
    editForm.value.thermocoupleType = editChannels.value[0].thermocoupleType
  }
}

// 监听 editForm.unit 和 editForm.precision 变化，同步到通道
watch(() => editForm.value.unit, () => syncUnitToChannels())
watch(() => editForm.value.precision, () => syncPrecisionToChannels())

// ==================== 零位校准 ====================
const zeroCalibrating = ref(false)

// 当前编辑设备是否已连接且正在采集（校零前置条件）
const canZeroCalibrate = computed(() => {
  const status = deviceStore.statuses.find(s => s.id === editForm.value.id)
  return status?.status === 'Connected' && status?.acquiring
})

// 当前编辑设备是否存在任何已校零的压力通道（批量去校零按钮启用条件）
// 仅当至少一个压力通道已校零时才允许批量清除，避免无意义操作。
// isPressureDevice 守卫由按钮组 v-if 保证，此处无需重复判断。
const hasAnyZeroOffset = computed(() => {
  const pc = editPressureCount.value
  return editChannels.value.some(c => c.index < pc && c.zeroCalibratedAt)
})

function formatZeroValue(value: number | undefined): string {
  if (!value) return '0'
  return Math.abs(value) < 0.001 ? '0' : value.toFixed(3)
}

function formatZeroTime(ts: number | undefined): string {
  if (!ts) return ''
  const d = new Date(ts)
  const hh = String(d.getHours()).padStart(2, '0')
  const mm = String(d.getMinutes()).padStart(2, '0')
  return `${hh}:${mm}校准`
}

// 校零后从后端 profile 同步零位字段到 editChannels（保留其他未保存编辑）
async function syncZeroOffsetsFromProfile() {
  await deviceStore.fetchProfiles()
  const profile = deviceStore.profiles.find(p => p.id === editForm.value.id)
  if (!profile) return
  for (const ec of editChannels.value) {
    const pc = profile.channels.find(c => c.index === ec.index)
    if (pc) {
      ec.zeroOffset = pc.zeroOffset
      ec.zeroOffsetUnit = pc.zeroOffsetUnit
      ec.zeroCalibratedAt = pc.zeroCalibratedAt
    }
  }
}

async function handleZeroCalibrateAll() {
  ElMessage({ message: '请保持设备静止，正在采样...', type: 'warning', duration: 1200 })
  zeroCalibrating.value = true
  try {
    const err = await deviceStore.zeroCalibrate(editForm.value.id)
    if (err) {
      ElMessage.error(`批量校零失败: ${err}`)
    } else {
      ElMessage.success('批量校零完成')
      await syncZeroOffsetsFromProfile()
    }
  } finally {
    zeroCalibrating.value = false
  }
}

async function handleZeroCalibrateChannel(channelIndex: number) {
  ElMessage({ message: '请保持设备静止，正在采样...', type: 'warning', duration: 1200 })
  zeroCalibrating.value = true
  try {
    const err = await deviceStore.zeroCalibrateChannel(editForm.value.id, channelIndex)
    if (err) {
      ElMessage.error(`通道 ${channelIndex} 校零失败: ${err}`)
    } else {
      ElMessage.success(`通道 ${channelIndex} 校零完成`)
      await syncZeroOffsetsFromProfile()
    }
  } finally {
    zeroCalibrating.value = false
  }
}

async function handleClearZeroOffset(channelIndex: number) {
  const err = await deviceStore.clearZeroOffset(editForm.value.id, channelIndex)
  if (err) {
    ElMessage.error(`清除零位失败: ${err}`)
  } else {
    ElMessage.success('零位已清除')
    await syncZeroOffsetsFromProfile()
  }
}

// 批量去校零：清除当前编辑设备所有压力通道的零位偏移。
// 后端 ClearAllZeroOffsets 会清除所有通道（含大气压/大气温度通道，它们本就无零位）。
// 按钮已通过 :disabled="!hasAnyZeroOffset" 禁用无校零项场景，此处无需重复判断。
async function handleClearAllZeroOffsets() {
  try {
    await ElMessageBox.confirm(
      '将清除该设备所有压力通道的零位偏移，此操作不可恢复。是否继续？',
      '批量去校零确认',
      { confirmButtonText: '清除', cancelButtonText: '取消', type: 'warning' },
    )
  } catch {
    return // 用户取消
  }
  const err = await deviceStore.clearAllZeroOffsets(editForm.value.id)
  if (err) {
    ElMessage.error(`批量去校零失败: ${err}`)
  } else {
    ElMessage.success('批量去校零完成')
    await syncZeroOffsetsFromProfile()
  }
}

// ==================== 跨设备批量校零 ====================
const batchZeroCalibrating = ref(false)
const showBatchZeroResult = ref(false)
const batchZeroResults = ref<ZeroCalibrateResult[]>([])

// 批量校零结果汇总（成功/失败）
const batchZeroSummary = computed(() => {
  let success = 0, failed = 0
  for (const r of batchZeroResults.value) {
    if (r.success) success++
    else failed++
  }
  return { success, failed }
})

// 是否存在可批量校零的设备（已连接 + 采集中 + 压力设备）
const canBatchZeroCalibrate = computed(() => {
  return deviceStore.statuses.some(s => {
    if (s.status !== 'Connected' || !s.acquiring) return false
    return isPressureDevice(s.type)
  })
})

async function handleBatchZeroCalibrate() {
  if (!canBatchZeroCalibrate.value) {
    ElMessage.warning('没有可批量校零的设备（需已连接且正在采集的压力设备）')
    return
  }
  try {
    await ElMessageBox.confirm(
      '将对所有已连接且正在采集的压力设备执行零位校准，请确保所有设备静止并处于A-测量位。是否继续？',
      '批量校零确认',
      { confirmButtonText: '开始校零', cancelButtonText: '取消', type: 'warning' },
    )
  } catch {
    return // 用户取消
  }

  batchZeroCalibrating.value = true
  ElMessage({ message: '请保持所有设备静止，正在并行采样...', type: 'warning', duration: 1500 })
  try {
    const ret = await deviceStore.zeroCalibrateAll()
    // 整体调用失败（后端异常等）：明确提示错误，不与"无符合条件设备"混淆
    if (!ret.success) {
      ElMessage.error(`批量校零调用失败: ${ret.error || '未知错误'}`)
      return
    }
    const results = ret.results || []
    batchZeroResults.value = results
    if (results.length === 0) {
      ElMessage.warning('批量校零未执行：没有符合条件的设备（需已连接且正在采集的压力设备）')
      return
    }
    const summary = batchZeroSummary.value
    if (summary.failed === 0) {
      ElMessage.success(`批量校零完成：全部 ${summary.success} 台设备成功`)
    } else if (summary.success === 0) {
      ElMessage.error(`批量校零失败：全部 ${summary.failed} 台设备失败`)
    } else {
      ElMessage.warning(`批量校零完成：成功 ${summary.success} 台，失败 ${summary.failed} 台`)
    }
    showBatchZeroResult.value = true
  } finally {
    batchZeroCalibrating.value = false
  }
}

async function saveEdit() {
  saving.value = true
  try {
    const profile = deviceStore.profiles.find(p => p.id === editForm.value.id)
    if (!profile) {
      ElMessage.error('设备配置不存在')
      return
    }

    const formSnapshot = { ...editForm.value }
    const channelsSnapshot = editChannels.value.map(c => ({ ...c }))
    const pc = editPressureCount.value
    const isTempDevice = editProfileType.value === 'EA2516T'
    const updatedChannels = channelsSnapshot.map(c => ({
      index: c.index,
      name: c.name,
      enabled: c.enabled,
      unit: isTempDevice ? '°C' : (c.index === pc ? 'Pa' : (c.index === pc + 1 ? '°C' : formSnapshot.unit)),
      precision: formSnapshot.precision,
      rangeMin: c.rangeMin,
      rangeMax: c.rangeMax,
      thermocoupleType: isTempDevice ? (c.thermocoupleType || 'K') : undefined,
      // 保留零位校准字段（校零由运行时操作设置，编辑配置时不能丢失）
      zeroOffset: c.zeroOffset,
      zeroOffsetUnit: c.zeroOffsetUnit,
      zeroCalibratedAt: c.zeroCalibratedAt,
    }))

    const updatedProfile = new types.DeviceProfile({
      id: profile.id,
      name: formSnapshot.name,
      type: profile.type as any,
      host: formSnapshot.host,
      port: formSnapshot.port,
      streamId: profile.streamId,
      periodMs: Math.round(1000 / formSnapshot.publishRate),
      autoConnect: formSnapshot.autoConnect,
      channels: updatedChannels,
    })

    const err = await deviceStore.updateProfile(updatedProfile as any)
    if (err) {
      ElMessage.error(`更新失败: ${err}`)
    } else {
      const oldUnit = profile.channels.length > 0 ? profile.channels[0].unit : 'Pa'
      const oldStatus = deviceStore.statuses.find(s => s.id === formSnapshot.id)

      if (oldStatus?.status === 'Connected' && formSnapshot.unit !== oldUnit) {
        const unitErr = await deviceStore.setUnit(formSnapshot.id, formSnapshot.unit)
        if (unitErr) {
          ElMessage.error(`设置硬件单位失败: ${unitErr}`)
          return
        }
      }

      // DAQ-T 设备：发送热电偶类型命令到硬件
      if (isTempDevice && oldStatus?.status === 'Connected') {
        const tcTypes = channelsSnapshot.map(c => c.thermocoupleType || 'K').join('').padEnd(16, 'K').substring(0, 16)
        const tcErr = await deviceStore.setThermocoupleType(formSnapshot.id, tcTypes)
        if (tcErr) {
          ElMessage.error(`设置热电偶类型失败: ${tcErr}`)
          return
        }
      }

      try {
        await DataService.SetPublishRate(formSnapshot.publishRate)
      } catch {}

      if (formSnapshot.autoConnect) {
        if (!oldStatus || oldStatus.status !== 'Connected') {
          const connErr = await deviceStore.connectDevice(formSnapshot.id)
          if (connErr) {
            ElMessage.warning(`自动连接失败: ${connErr}`)
          } else {
            ElMessage.success('设备已自动连接')
          }
        }
      } else {
        await deviceStore.disconnectDevice(formSnapshot.id)
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

// 单设备采集启停：busy 状态由 store 统一管理，view 只负责提示
async function handleToggleAcq(id: string) {
  const { error, name, wasAcquiring } = await deviceStore.toggleAcquisition(id)
  if (error) {
    ElMessage.error(`${wasAcquiring ? '停止' : '开始'}采集失败：${name} - ${error}`)
  } else {
    ElMessage.success(`${wasAcquiring ? '已停止' : '已开始'}采集：${name}`)
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
  white-space: nowrap;

  &.connected {
    background: rgba($color-success, 0.1);
    color: $color-success;
    .status-dot {
      background: $color-success;
      box-shadow: 0 0 4px rgba($color-success, 0.5);
    }
  }
  &.connecting {
    background: rgba($color-accent, 0.1);
    color: $color-accent;
    .status-dot {
      background: $color-accent;
      box-shadow: 0 0 4px rgba($color-accent, 0.5);
      animation: statusDotPulse 1.2s ease-in-out infinite;
    }
  }
  &.error {
    background: rgba($color-danger, 0.1);
    color: $color-danger;
    .status-dot {
      background: $color-danger;
      box-shadow: 0 0 4px rgba($color-danger, 0.5);
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

@keyframes statusDotPulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.4; transform: scale(0.7); }
}

.acquiring-badge {
  display: inline-flex;
  align-items: center;
  gap: $spacing-xs;
  padding: $spacing-xs $spacing-sm;
  border-radius: $border-radius-md;
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
  // 与 acquiring-badge 保持相同的盒模型，避免采集状态切换时行高变化引起表格跳动
  display: inline-flex;
  align-items: center;
  padding: $spacing-xs $spacing-sm;
  border-radius: $border-radius-md;
  font-size: 11px;
  color: rgba(255,255,255,0.3);
}

.action-group {
  .el-button {
    padding: 6px 10px;
  }
}

// ==================== 弹窗通用样式 ====================
:deep(.device-dialog) {
  .el-dialog {
    max-height: 88vh;
    display: flex;
    flex-direction: column;
  }
  .el-dialog__header {
    margin-right: 0;
    padding: 16px 20px;
    border-bottom: 1px solid rgba(255,255,255,0.08);
    flex-shrink: 0;
  }
  .el-dialog__title {
    font-size: 14px;
    font-weight: 600;
    color: rgba(255,255,255,0.9);
  }
  .el-dialog__body {
    padding: 16px 20px;
    overflow-y: auto;
    flex: 1;
    min-height: 0;
  }
  .el-dialog__footer {
    flex-shrink: 0;
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

.form-row {
  display: flex;
  gap: 16px;
  align-items: flex-end;

  &.three-col {
    gap: 12px;
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
}

.param-hint {
  margin-top: 8px;
  font-size: 10px;
  color: rgba(255,255,255,0.35);
}

.special-channels {
  font-size: 10px;
  color: rgba(255,255,255,0.45);
}

.auto-connect-row {
  display: flex;
  flex-direction: row !important;
  align-items: center;
  gap: 8px;
}
.auto-connect-label {
  font-size: 12px;
  color: rgba(255,255,255,0.75);
  font-weight: 500;
}

// ==================== 通道表格 ====================
.channel-section {
  .section-title {
    margin-bottom: 10px;
  }
}

.channel-section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
  .section-title {
    margin-bottom: 0;
  }
}

// 批量校零/去校零按钮组：紧邻排列，统一间距
.batch-zero-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}

.zero-info {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  .zero-value {
    font-family: $font-family-mono;
    font-size: 11px;
    color: rgba(255,255,255,0.85);
    white-space: nowrap;
  }
  .zero-time {
    font-size: $font-size-xs;
    color: rgba(255,255,255,0.4);
  }
}

// ==================== 批量校零结果 ====================
.batch-zero-summary {
  display: flex;
  gap: $spacing-lg;
  margin-bottom: $spacing-md;
  padding: $spacing-sm $spacing-lg;
  background: $bg-tertiary;
  border-radius: 6px;
  border: 1px solid $glass-border-light;

  .summary-item {
    font-size: 13px;
    font-weight: 600;
    &.success { color: $color-success; }
    &.failed { color: $color-danger; }
  }
}

.error-text {
  font-size: 11px;
  color: rgba($color-danger, 0.85);
  word-break: break-all;
}

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

// 表格内输入框样式统一
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
    background-color: var(--accent-color) !important;
    border-color: var(--accent-color) !important;
  }
}
</style>
