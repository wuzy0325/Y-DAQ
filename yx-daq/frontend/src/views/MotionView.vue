<template>
  <div class="motion-view">
    <!-- 左侧：控制器侧边栏 -->
    <aside class="controller-sidebar">
      <div class="sidebar-header">
        <span class="sidebar-title">🎯 控制器</span>
        <el-button type="primary" size="small" @click="openAddDialog">
          <el-icon><Plus /></el-icon>
          <span>添加</span>
        </el-button>
      </div>

      <div class="controller-list">
        <div
          v-for="ctrl in motionStore.profiles"
          :key="ctrl.id"
          class="controller-card-item"
          :class="{
            'is-active': ctrl.id === motionStore.activeControllerId,
            'is-connected': getStatus(ctrl.id) === 'Connected',
          }"
          @click="onRowClick(ctrl)"
        >
          <div class="card-header">
            <span class="card-name">{{ ctrl.name }}</span>
            <el-tag size="small" type="info" class="card-type">{{ typeLabel(ctrl.type) }}</el-tag>
          </div>
          <div class="card-meta">
            <span class="card-addr">
              {{ ctrl.type === 'SIMULATED-MC' ? '本地模拟' : `${ctrl.address}:${ctrl.port}` }}
            </span>
          </div>
          <div class="card-status">
            <div class="status-badge" :class="statusClass(getStatus(ctrl.id))">
              <span class="status-dot" :class="{ pulse: getStatus(ctrl.id) === 'Connecting' }" />
              <span class="status-text">{{ statusLabel(getStatus(ctrl.id)) }}</span>
            </div>
            <span class="card-axes">{{ ctrl.axes?.length || 0 }} 轴</span>
          </div>
          <div class="card-actions" @click.stop>
            <el-button size="small" @click="openEditDialog(ctrl)" title="编辑">
              <el-icon><Edit /></el-icon>
            </el-button>
            <el-button
              v-if="getStatus(ctrl.id) !== 'Connected'"
              type="primary"
              size="small"
              :loading="motionStore.isControllerConnecting(ctrl.id)"
              @click="handleConnect(ctrl.id)"
              title="连接"
            >
              <el-icon v-if="!motionStore.isControllerConnecting(ctrl.id)"><Link /></el-icon>
            </el-button>
            <el-button v-else type="warning" size="small" @click="handleDisconnect(ctrl.id)" title="断开">
              <el-icon><CircleClose /></el-icon>
            </el-button>
            <el-button
              size="small"
              type="danger"
              :disabled="motionStore.isControllerConnecting(ctrl.id)"
              @click="removeController(ctrl)"
              title="删除"
            >
              <el-icon><Delete /></el-icon>
            </el-button>
          </div>
        </div>

        <!-- 空状态 -->
        <div v-if="motionStore.profiles.length === 0" class="empty-state">
          <el-icon class="empty-icon"><Connection /></el-icon>
          <p class="empty-text">暂无控制器</p>
          <el-button type="primary" size="small" plain @click="openAddDialog">添加控制器</el-button>
        </div>
      </div>

      <div class="sidebar-footer">
        <el-button
          type="danger"
          size="small"
          :disabled="!hasConnectedController"
          @click="onEmergencyStopAll"
          class="estop-btn"
        >
          <el-icon><WarningFilled /></el-icon>
          <span>急停全部</span>
        </el-button>
      </div>
    </aside>

    <!-- 右侧：活动控制器的轴控制区 -->
    <main class="controller-main">
      <div class="main-header">
        <span class="active-name">
          {{ activeProfile ? activeProfile.name : '未选择控制器' }}
        </span>
        <span v-if="activeProfile" class="active-status" :class="statusClass(getStatus(activeProfile.id))">
          {{ statusLabel(getStatus(activeProfile.id)) }}
        </span>
      </div>

      <!-- 轴位置概览 -->
      <div class="axes-overview">
        <div
          v-for="(axis, index) in motionStore.allAxes"
          :key="axis.name"
          class="overview-item"
          :class="{ 'has-limit': axis.posLimitActive || axis.negLimitActive }"
        >
          <span class="overview-dot" :style="{ background: AXIS_COLORS[index] }" />
          <span class="overview-name">{{ axis.name }}</span>
          <span class="overview-value" :style="{ color: AXIS_COLORS[index] }">
            {{ axis.currentPosition.toFixed(2) }}
          </span>
          <span class="overview-unit">{{ getAxisUnit(axis.kind) }}</span>
        </div>
      </div>

      <!-- 轴控制区 2x2 -->
      <div class="axes-grid">
        <AxisControlCard
          v-for="(axis, index) in motionStore.allAxes"
          :key="axis.name"
          :axis="axis"
          :axis-color="AXIS_COLORS[index]"
          @configure="onConfigureAxis"
        />
      </div>

      <!-- 未选择控制器时的占位 -->
      <div v-if="!activeProfile" class="no-active-placeholder">
        <el-icon class="placeholder-icon"><Aim /></el-icon>
        <p>请从左侧选择一个控制器</p>
      </div>
    </main>

    <!-- 添加控制器对话框 -->
    <el-dialog v-model="showAddDialog" title="添加控制器" width="420px" :append-to-body="true" class="motion-dialog">
      <div class="dialog-section">
        <div class="section-title">🎯 基础信息</div>
        <el-form :model="newController" label-width="70px" size="small">
          <el-form-item label="名称">
            <el-input v-model="newController.name" placeholder="请输入控制器名称" />
          </el-form-item>
          <el-form-item label="类型">
            <el-select v-model="newController.type" style="width: 100%">
              <el-option label="B140 运动控制器" value="B140-MC" />
              <el-option label="模拟控制器" value="SIMULATED-MC" />
            </el-select>
          </el-form-item>
        </el-form>
      </div>

      <div class="dialog-section">
        <div class="section-title">🔗 网络配置</div>
        <div class="form-row">
          <div class="form-group">
            <label class="group-label">地址</label>
            <el-input
              v-model="newController.address"
              placeholder="192.168.1.101"
              size="small"
              style="width: 150px"
              :disabled="newController.type === 'SIMULATED-MC'"
            />
          </div>
          <div class="form-group">
            <label class="group-label">端口</label>
            <el-input-number
              v-model="newController.port"
              :min="1"
              :max="65535"
              size="small"
              style="width: 110px"
              controls-position="right"
              :disabled="newController.type === 'SIMULATED-MC'"
            />
          </div>
        </div>
      </div>

      <div class="dialog-section">
        <div class="section-title">⚙️ 超时设置</div>
        <div class="form-row">
          <div class="form-group">
            <label class="group-label">超时(ms)</label>
            <el-input-number
              v-model="newController.timeoutMs"
              :min="100"
              :max="30000"
              :step="100"
              size="small"
              style="width: 130px"
              controls-position="right"
            />
          </div>
        </div>
      </div>

      <template #footer>
        <el-button @click="showAddDialog = false">取消</el-button>
        <el-button type="primary" :loading="adding" @click="addController">确定</el-button>
      </template>
    </el-dialog>

    <!-- 编辑控制器对话框 -->
    <el-dialog v-model="showEditDialog" title="编辑控制器" width="720px" :append-to-body="true" class="motion-dialog">
      <div class="dialog-section">
        <div class="section-title">📡 基础信息</div>
        <div class="form-row">
          <div class="form-group">
            <label class="group-label">名称</label>
            <el-input v-model="editForm.name" size="small" style="width: 160px" />
          </div>
          <div class="form-group">
            <label class="group-label">地址</label>
            <el-input
              v-model="editForm.address"
              size="small"
              style="width: 150px"
              :disabled="editIsSimulated"
            />
          </div>
          <div class="form-group">
            <label class="group-label">端口</label>
            <el-input-number
              v-model="editForm.port"
              :min="1"
              :max="65535"
              size="small"
              style="width: 110px"
              controls-position="right"
              :disabled="editIsSimulated"
            />
          </div>
          <div class="form-group">
            <label class="group-label">超时(ms)</label>
            <el-input-number
              v-model="editForm.timeoutMs"
              :min="100"
              :max="30000"
              :step="100"
              size="small"
              style="width: 110px"
              controls-position="right"
            />
          </div>
        </div>
      </div>

      <div class="channel-section">
        <div class="section-title">📋 轴配置</div>
        <el-table :data="editAxes" size="small" class="channel-table" :max-height="320">
          <el-table-column label="轴" width="45" align="center">
            <template #default="{ row }">
              <span class="channel-index">{{ row.name }}</span>
            </template>
          </el-table-column>
          <el-table-column label="启用" width="60" align="center">
            <template #default="{ row }">
              <el-switch v-model="row.enabled" size="small" />
            </template>
          </el-table-column>
          <el-table-column label="轴类型" width="100" align="center">
            <template #default="{ row }">
              <el-select v-model="row.kind" size="small" style="width: 85px">
                <el-option label="平移轴" value="LINEAR" />
                <el-option label="旋转轴" value="ROTARY" />
              </el-select>
            </template>
          </el-table-column>
          <el-table-column label="步距角(°)" width="90" align="center">
            <template #default="{ row }">
              <el-input-number
                v-model="row.stepAngleDeg"
                :precision="1"
                :step="0.1"
                :min="0.1"
                :max="10"
                size="small"
                controls-position="right"
                style="width: 80px"
              />
            </template>
          </el-table-column>
          <el-table-column label="细分数" width="80" align="center">
            <template #default="{ row }">
              <el-select v-model="row.microSteps" size="small" style="width: 70px">
                <el-option v-for="n in [1,2,4,8,16,32,64,128,256]" :key="n" :label="`${n}`" :value="n" />
              </el-select>
            </template>
          </el-table-column>
          <el-table-column label="丝杆导程(mm)" width="100" align="center">
            <template #default="{ row }">
              <el-input-number
                v-if="row.kind === 'LINEAR'"
                v-model="row.lead"
                :precision="2"
                :step="0.5"
                :min="0.1"
                :max="50"
                size="small"
                controls-position="right"
                style="width: 85px"
              />
              <span v-else class="readonly-text">-</span>
            </template>
          </el-table-column>
          <el-table-column label="传动比" width="90" align="center">
            <template #default="{ row }">
              <el-input-number
                v-if="row.kind === 'ROTARY'"
                v-model="row.gearRatio"
                :precision="1"
                :step="1"
                :min="1"
                size="small"
                controls-position="right"
                style="width: 80px"
              />
              <span v-else class="readonly-text">-</span>
            </template>
          </el-table-column>
          <el-table-column label="最大速度" width="110" align="center">
            <template #default="{ row }">
              <div class="input-with-unit">
                <el-input-number
                  v-model="row.maxSpeed"
                  :precision="1"
                  :step="1"
                  :min="0.1"
                  :max="500"
                  size="small"
                  controls-position="right"
                  style="width: 75px"
                />
                <span class="unit">{{ row.kind === 'LINEAR' ? 'mm/s' : '°/s' }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="方向取反" width="70" align="center">
            <template #default="{ row }">
              <el-switch v-model="row.inverted" size="small" />
            </template>
          </el-table-column>
        </el-table>
      </div>

      <template #footer>
        <el-button @click="showEditDialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveEdit">保存</el-button>
      </template>
    </el-dialog>

    <!-- 轴参数配置对话框 -->
    <AxisConfigDialog ref="configDialog" />

    <!-- 自定义删除确认弹窗 -->
    <el-dialog
      v-model="showDeleteConfirm"
      width="380px"
      :append-to-body="true"
      :show-close="false"
      class="delete-confirm-dialog"
      align-center
    >
      <div class="confirm-body">
        <div class="confirm-icon-wrap">
          <el-icon class="confirm-icon"><WarningFilled /></el-icon>
        </div>
        <div class="confirm-content">
          <div class="confirm-title">删除控制器</div>
          <div class="confirm-desc">
            确定要删除控制器「<span class="confirm-name">{{ deleteTarget?.name }}</span>」吗？
          </div>
          <div v-if="deleteTarget && getStatus(deleteTarget.id) === 'Connected'" class="confirm-warn">
            <el-icon><WarningFilled /></el-icon>
            <span>该控制器已连接，删除时将被断开</span>
          </div>
        </div>
      </div>
      <template #footer>
        <div class="confirm-actions">
          <button class="confirm-cancel" @click="cancelDelete">取消</button>
          <button class="confirm-delete" :loading="deleting" @click="confirmDelete">
            <el-icon v-if="!deleting"><Delete /></el-icon>
            <el-icon v-else class="spin-icon"><Loading /></el-icon>
            <span>删除</span>
          </button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { Edit, Link, CircleClose, Delete, Plus, Connection, WarningFilled, Aim, Loading } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useMotionStore } from '../stores/motion'
import { MotionEmergencyStop } from '../wails-compat/app'
import GlassCard from '../components/GlassCard.vue'
import AxisConfigDialog from '../components/MotionControl/AxisConfigDialog.vue'
import AxisControlCard from '../components/MotionControl/AxisControlCard.vue'

const AXIS_COLORS = ['#b829ff', '#00f5ff', '#00ff88', '#ffaa00'] as const

const motionStore = useMotionStore()
const configDialog = ref<InstanceType<typeof AxisConfigDialog>>()

// 本地类型定义（与 store 内部 MotionControllerProfile / AxisConfig 结构一致）
type AxisKind = 'LINEAR' | 'ROTARY'
interface AxisConfig {
  name: string
  enabled: boolean
  kind: AxisKind
  inverted: boolean
  stepAngleDeg: number
  microSteps: number
  lead: number
  gearRatio: number
  maxSpeed: number
  encoderScale: number
  encoderCompensation: {
    enabled: boolean
    tolerance: number
    maxCycles: number
    settleMs: number
    minStep: number
    timeoutMs: number
  }
}
interface MotionControllerProfile {
  id: string
  name: string
  type: string
  address: string
  port: number
  timeoutMs: number
  axes: AxisConfig[]
}

// 活动控制器（轴控制目标）
const activeProfile = computed(() =>
  motionStore.profiles.find(p => p.id === motionStore.activeControllerId) || null
)

// 状态映射
const statusMap = computed(() => {
  const m = new Map<string, string>()
  for (const s of motionStore.statuses) m.set(s.id, s.status)
  return m
})

function getStatus(id: string): string {
  return statusMap.value.get(id) || 'Disconnected'
}

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
    case 'Error': return '错误'
    default: return '未连接'
  }
}

function typeLabel(type: string): string {
  if (type === 'B140-MC') return 'B140'
  if (type === 'SIMULATED-MC') return '模拟'
  return type
}

const hasConnectedController = computed(() =>
  motionStore.statuses.some(s => s.status === 'Connected')
)

// ==================== 添加控制器 ====================
const showAddDialog = ref(false)
const adding = ref(false)
const newController = ref({
  name: '',
  type: 'B140-MC',
  address: '192.168.1.101',
  port: 5000,
  timeoutMs: 5000,
})

function openAddDialog() {
  newController.value = {
    name: '',
    type: 'B140-MC',
    address: '192.168.1.101',
    port: 5000,
    timeoutMs: 5000,
  }
  showAddDialog.value = true
}

watch(() => newController.value.type, (t) => {
  if (t === 'B140-MC') {
    newController.value.address = '192.168.1.101'
    newController.value.port = 5000
  } else {
    newController.value.address = ''
    newController.value.port = 0
  }
})

async function addController() {
  if (!newController.value.name.trim()) {
    ElMessage.warning('请输入控制器名称')
    return
  }
  if (newController.value.type === 'B140-MC') {
    if (!newController.value.address.trim()) {
      ElMessage.warning('请输入地址')
      return
    }
    if (!newController.value.port) {
      ElMessage.warning('请输入端口')
      return
    }
  }
  adding.value = true
  try {
    const isSimulated = newController.value.type === 'SIMULATED-MC'
    const result = await motionStore.addController({
      name: newController.value.name.trim(),
      type: newController.value.type,
      address: isSimulated ? '' : newController.value.address.trim(),
      port: isSimulated ? 0 : newController.value.port,
      timeoutMs: newController.value.timeoutMs,
    })
    if (!result.success) {
      ElMessage.error(`添加失败: ${result.error}`)
    } else {
      ElMessage.success('控制器添加成功')
      showAddDialog.value = false
    }
  } finally {
    adding.value = false
  }
}

// ==================== 编辑控制器 ====================
const showEditDialog = ref(false)
const saving = ref(false)
const editForm = ref({ id: '', name: '', address: '', port: 0, timeoutMs: 5000 })
const editAxes = ref<AxisConfig[]>([])
const editOriginal = ref<MotionControllerProfile | null>(null)

const editIsSimulated = computed(() => editOriginal.value?.type === 'SIMULATED-MC')

function openEditDialog(row: MotionControllerProfile) {
  const profile = motionStore.profiles.find(p => p.id === row.id)
  if (!profile) {
    ElMessage.warning('未找到控制器配置')
    return
  }
  editOriginal.value = profile as unknown as MotionControllerProfile
  editForm.value = {
    id: profile.id,
    name: profile.name,
    address: profile.address,
    port: profile.port,
    timeoutMs: profile.timeoutMs,
  }
  editAxes.value = (profile.axes as unknown as AxisConfig[]).map(a => ({
    ...a,
    encoderCompensation: { ...a.encoderCompensation },
  }))
  showEditDialog.value = true
}

async function saveEdit() {
  if (!editOriginal.value) return
  saving.value = true
  try {
    const orig = editOriginal.value
    const updatedAxes: AxisConfig[] = editAxes.value.map(a => {
      const origAxis = (orig.axes as unknown as AxisConfig[]).find(o => o.name === a.name)
      return {
        name: a.name,
        enabled: a.enabled,
        kind: a.kind,
        inverted: a.inverted,
        stepAngleDeg: a.stepAngleDeg,
        microSteps: a.microSteps,
        lead: a.lead,
        gearRatio: a.gearRatio,
        maxSpeed: a.maxSpeed,
        // 保留原 profile 的编码器相关字段（表格不展示）
        encoderScale: origAxis?.encoderScale ?? 0.005,
        encoderCompensation: origAxis?.encoderCompensation ?? a.encoderCompensation,
      }
    })
    const profile: MotionControllerProfile = {
      id: orig.id,
      name: editForm.value.name,
      type: orig.type,
      address: editForm.value.address,
      port: editForm.value.port,
      timeoutMs: editForm.value.timeoutMs,
      axes: updatedAxes,
    }
    const result = await motionStore.updateControllerProfile(profile as any)
    if (!result.success) {
      ElMessage.error(`更新失败: ${result.error}`)
    } else {
      ElMessage.success('控制器配置已更新')
      showEditDialog.value = false
    }
  } finally {
    saving.value = false
  }
}

// ==================== 删除控制器 ====================
const showDeleteConfirm = ref(false)
const deleteTarget = ref<MotionControllerProfile | null>(null)
const deleting = ref(false)

function removeController(row: MotionControllerProfile) {
  deleteTarget.value = row
  showDeleteConfirm.value = true
}

function cancelDelete() {
  showDeleteConfirm.value = false
  deleteTarget.value = null
}

async function confirmDelete() {
  if (!deleteTarget.value) return
  deleting.value = true
  try {
    const result = await motionStore.removeController(deleteTarget.value.id)
    if (!result.success) {
      ElMessage.error(`删除失败: ${result.error}`)
    } else {
      ElMessage.success('控制器已删除')
      showDeleteConfirm.value = false
    }
  } finally {
    deleting.value = false
    deleteTarget.value = null
  }
}

// ==================== 连接 / 断开 ====================
async function handleConnect(id: string) {
  const result = await motionStore.connectController(id)
  if (!result.success) {
    ElMessage.error(result.error || '连接失败')
  } else {
    ElMessage.success('控制器已连接')
  }
}

async function handleDisconnect(id: string) {
  // 直接断开指定控制器，不切换 activeControllerId（避免轴控制卡片目标跳变）
  const result = await motionStore.disconnectController(id)
  if (!result.success) {
    ElMessage.error(result.error || '断开失败')
  } else {
    ElMessage.success('控制器已断开')
  }
}

// ==================== 急停全部 ====================
async function onEmergencyStopAll() {
  // 并行急停所有已连接控制器，确保响应速度
  const targets = motionStore.statuses.filter(s => s.status === 'Connected')
  await Promise.allSettled(targets.map(ctrl => MotionEmergencyStop(ctrl.id)))
  ElMessage.warning('已触发急停')
}

// ==================== 行选中（活动控制器） ====================
function onRowClick(row: MotionControllerProfile) {
  motionStore.activeControllerId = row.id
}

// ==================== 轴配置 / 辅助 ====================
function onConfigureAxis(axisName: string) {
  motionStore.selectAxis(axisName)
  openConfigDialog(axisName)
}

function getAxisUnit(kind: string): string {
  return kind === 'LINEAR' ? 'mm' : '°'
}

function openConfigDialog(axisName?: string) {
  configDialog.value?.open(axisName)
}

onMounted(async () => {
  motionStore.startListening()
  await motionStore.fetchProfiles()
  await motionStore.fetchStatuses()
  // 默认选中第一个控制器作为活动目标
  if (!motionStore.activeControllerId && motionStore.profiles.length > 0) {
    motionStore.activeControllerId = motionStore.profiles[0].id
  }
})
</script>

<style lang="scss" scoped>
.motion-view {
  padding: 16px 20px 16px;
  height: 100%;
  display: flex;
  flex-direction: row;
  gap: 16px;
  overflow: hidden;
}

/* ==================== 左侧侧边栏 ==================== */
.controller-sidebar {
  width: 260px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  background: $glass-bg;
  border: 1px solid $glass-border;
  border-radius: $border-radius-md;
  backdrop-filter: blur(16px);
  overflow: hidden;
}

.sidebar-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 14px 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  flex-shrink: 0;

  .sidebar-title {
    font-size: 13px;
    font-weight: 600;
    color: rgba(255, 255, 255, 0.9);
  }

  .el-button {
    padding: 6px 10px;
    span {
      margin-left: 2px;
    }
  }
}

.controller-list {
  flex: 1;
  overflow-y: auto;
  padding: 10px;
  display: flex;
  flex-direction: column;
  gap: 8px;

  &::-webkit-scrollbar {
    width: 4px;
  }
  &::-webkit-scrollbar-thumb {
    background: rgba(255, 255, 255, 0.15);
    border-radius: 2px;
  }
}

.controller-card-item {
  padding: 10px 12px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
  position: relative;

  &:hover {
    background: rgba(255, 255, 255, 0.06);
    border-color: rgba(255, 255, 255, 0.12);
  }

  &.is-active {
    background: rgba($color-primary, 0.12);
    border-color: rgba($color-primary, 0.4);
    box-shadow: 0 0 0 1px rgba($color-primary, 0.2);
  }

  &.is-connected {
    border-left: 3px solid $color-success;
    padding-left: 10px;
  }
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 4px;

  .card-name {
    font-size: 13px;
    font-weight: 600;
    color: rgba(255, 255, 255, 0.92);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    flex: 1;
    margin-right: 6px;
  }

  .card-type {
    font-size: 10px;
    height: 18px;
    padding: 0 6px;
    flex-shrink: 0;
  }
}

.card-meta {
  margin-bottom: 8px;

  .card-addr {
    font-size: 11px;
    font-family: 'SF Mono', Monaco, 'Cascadia Code', Consolas, 'Courier New', monospace;
    color: rgba(255, 255, 255, 0.55);
  }
}

.card-status {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;

  .card-axes {
    font-size: 11px;
    color: rgba(255, 255, 255, 0.5);
  }
}

.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 2px 8px;
  border-radius: 10px;
  font-size: 10px;
  font-weight: 500;

  &.connected {
    background: rgba($color-success, 0.12);
    color: $color-success;
    .status-dot { background: $color-success; box-shadow: 0 0 4px rgba($color-success, 0.5); }
  }
  &.connecting {
    background: rgba($color-accent, 0.12);
    color: $color-accent;
    .status-dot { background: $color-accent; animation: statusDotPulse 1.2s ease-in-out infinite; }
  }
  &.error {
    background: rgba($color-danger, 0.12);
    color: $color-danger;
    .status-dot { background: $color-danger; box-shadow: 0 0 4px rgba($color-danger, 0.5); }
  }
  &.disconnected {
    background: rgba(255, 255, 255, 0.06);
    color: $text-tertiary;
    .status-dot { background: $text-muted; }
  }

  .status-dot {
    width: 5px;
    height: 5px;
    border-radius: 50%;
  }
}

@keyframes statusDotPulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.4; transform: scale(0.7); }
}

.card-actions {
  display: flex;
  gap: 4px;

  .el-button {
    padding: 5px 8px;
    margin-left: 0;
  }
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px 12px;
  gap: 10px;

  .empty-icon {
    font-size: 32px;
    color: rgba(255, 255, 255, 0.2);
  }

  .empty-text {
    font-size: 12px;
    color: rgba(255, 255, 255, 0.4);
    margin: 0;
  }
}

.sidebar-footer {
  padding: 10px 12px;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  flex-shrink: 0;

  .estop-btn {
    width: 100%;
    justify-content: center;
    span {
      margin-left: 4px;
    }
  }
}

/* ==================== 右侧主区 ==================== */
.controller-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 14px;
  min-width: 0;
  overflow: hidden;
  position: relative;
}

.main-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 4px 4px 0;
  flex-shrink: 0;

  .active-name {
    font-size: 15px;
    font-weight: 600;
    color: rgba(255, 255, 255, 0.92);
  }

  .active-status {
    font-size: 11px;
    padding: 2px 10px;
    border-radius: 10px;
    font-weight: 500;

    &.connected { background: rgba($color-success, 0.12); color: $color-success; }
    &.connecting { background: rgba($color-accent, 0.12); color: $color-accent; }
    &.error { background: rgba($color-danger, 0.12); color: $color-danger; }
    &.disconnected { background: rgba(255, 255, 255, 0.06); color: $text-tertiary; }
  }
}

/* ==================== 轴位置概览 ==================== */
.axes-overview {
  display: flex;
  gap: 4px;
  padding: 10px 18px;
  background: $glass-bg;
  border: 1px solid $glass-border;
  border-radius: $border-radius-md;
  flex-shrink: 0;
  backdrop-filter: blur(16px);
}

.overview-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 16px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.02);
  transition: all 0.3s;

  &.has-limit {
    background: rgba($color-danger, 0.08);
    .overview-value { color: $color-danger !important; }
  }
}

.overview-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.overview-name {
  font-size: 12px;
  font-weight: 700;
  color: $text-muted;
  min-width: 14px;
}

.overview-value {
  font-size: 14px;
  font-weight: 700;
  font-family: 'SF Mono', Monaco, 'Cascadia Code', Consolas, 'Courier New', monospace;
  transition: color 0.3s;
}

.overview-unit {
  font-size: 11px;
  color: $text-muted;
  font-weight: 500;
}

/* ==================== 轴控制网格 ==================== */
.axes-grid {
  flex: 1;
  display: grid;
  grid-template-columns: 1fr 1fr;
  grid-auto-rows: 1fr;
  align-content: stretch;
  gap: 14px;
  min-height: 0;
  overflow: hidden;
}

.no-active-placeholder {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  pointer-events: none;

  .placeholder-icon {
    font-size: 48px;
    color: rgba(255, 255, 255, 0.12);
  }

  p {
    font-size: 13px;
    color: rgba(255, 255, 255, 0.35);
    margin: 0;
  }
}

@media (max-width: 1100px) {
  .axes-grid {
    grid-template-columns: 1fr;
    grid-auto-rows: auto;
    align-content: start;
    overflow-y: auto;
  }
}

/* ==================== 弹窗通用样式 ==================== */
:deep(.motion-dialog) {
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

/* ==================== 轴配置表格 ==================== */
.channel-section {
  .section-title {
    margin-bottom: 10px;
  }
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

.channel-index {
  font-family: 'SF Mono', Monaco, 'Cascadia Code', Consolas, 'Courier New', monospace;
  font-size: 12px;
  font-weight: 700;
  color: rgba(255,255,255,0.7);
}

.readonly-text {
  font-size: 11px;
  color: rgba(255,255,255,0.35);
}

/* ==================== 删除确认弹窗 ==================== */
:deep(.delete-confirm-dialog) {
  .el-dialog {
    background: rgba(30, 32, 44, 0.95);
    border: 1px solid rgba($color-danger, 0.25);
    border-radius: 14px;
    backdrop-filter: blur(20px);
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.5), 0 0 0 1px rgba(255, 255, 255, 0.04);
    overflow: hidden;
  }
  .el-dialog__header {
    display: none;
  }
  .el-dialog__body {
    padding: 24px 24px 16px;
  }
  .el-dialog__footer {
    padding: 0 24px 20px;
    border: none;
  }
}

.confirm-body {
  display: flex;
  gap: 14px;
  align-items: flex-start;
}

.confirm-icon-wrap {
  flex-shrink: 0;
  width: 44px;
  height: 44px;
  border-radius: 12px;
  background: rgba($color-danger, 0.12);
  border: 1px solid rgba($color-danger, 0.25);
  display: flex;
  align-items: center;
  justify-content: center;

  .confirm-icon {
    font-size: 22px;
    color: $color-danger;
  }
}

.confirm-content {
  flex: 1;
  min-width: 0;
}

.confirm-title {
  font-size: 15px;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.92);
  margin-bottom: 6px;
}

.confirm-desc {
  font-size: 13px;
  color: rgba(255, 255, 255, 0.65);
  line-height: 1.5;

  .confirm-name {
    color: rgba(255, 255, 255, 0.95);
    font-weight: 600;
  }
}

.confirm-warn {
  display: flex;
  align-items: center;
  gap: 5px;
  margin-top: 10px;
  padding: 6px 10px;
  border-radius: 6px;
  background: rgba($color-warning, 0.08);
  border: 1px solid rgba($color-warning, 0.2);
  color: $color-warning;
  font-size: 11px;
  font-weight: 500;

  .el-icon {
    font-size: 13px;
  }
}

.confirm-actions {
  display: flex;
  gap: 10px;
  justify-content: flex-end;
}

.confirm-cancel,
.confirm-delete {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  height: 34px;
  padding: 0 18px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s;
  user-select: none;
  white-space: nowrap;

  &:active:not(:disabled) {
    transform: scale(0.97);
  }

  .el-icon {
    font-size: 14px;
  }
}

.confirm-cancel {
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(255, 255, 255, 0.04);
  color: rgba(255, 255, 255, 0.75);

  &:hover {
    background: rgba(255, 255, 255, 0.08);
    border-color: rgba(255, 255, 255, 0.2);
    color: rgba(255, 255, 255, 0.9);
  }
}

.confirm-delete {
  border: 1px solid rgba($color-danger, 0.35);
  background: rgba($color-danger, 0.15);
  color: $color-danger;

  &:hover {
    background: rgba($color-danger, 0.25);
    border-color: rgba($color-danger, 0.55);
    box-shadow: 0 0 18px rgba($color-danger, 0.25);
  }

  &:disabled,
  &.is-loading {
    opacity: 0.6;
    cursor: not-allowed;
  }
}

.spin-icon {
  animation: confirm-spin 1s linear infinite;
}

@keyframes confirm-spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
</style>
