<template>
  <div class="motion-view">
    <!-- 顶部状态栏 -->
    <div class="status-bar">
      <div class="status-left">
        <div class="page-brand">
          <div class="brand-icon">
            <el-icon :size="22"><Cpu /></el-icon>
          </div>
          <div class="brand-text">
            <h2 class="page-title">运动控制</h2>
            <span v-if="activeProfile" class="controller-name">{{ activeProfile.name }}</span>
          </div>
        </div>
        <div class="connection-pill" :class="connectionStatus">
          <span class="pulse-dot" />
          <span class="conn-text">{{ connectionStatusText }}</span>
        </div>
      </div>

      <div class="status-center">
        <div class="conn-group-title">
          <span class="field-label">控制器连接</span>
          <span class="field-hint">修改后自动保存</span>
        </div>
        <div class="conn-field">
          <span class="field-label">IP 地址</span>
          <el-input
            v-model="editableAddress"
            size="small"
            class="ip-input"
            :disabled="motionStore.isConnected"
            @change="onConnectionInfoChange"
          />
        </div>
        <div class="conn-field">
          <span class="field-label">端口</span>
          <el-input-number
            v-model="editablePort"
            size="small"
            :min="1"
            :max="65535"
            class="port-input"
            :disabled="motionStore.isConnected"
            @change="onConnectionInfoChange"
          />
        </div>
        <div class="conn-field type-field">
          <span class="field-label">类型</span>
          <span class="type-badge">{{ activeProfile?.type === 'B140-MC' ? 'B140' : (activeProfile?.type || 'B140') }}</span>
        </div>
      </div>

      <div class="status-right">
        <div class="connection-actions">
          <el-button
            v-if="!motionStore.isConnected"
            type="success"
            class="conn-btn"
            :loading="motionStore.connectionStatus === 'connecting'"
            @click="connectController"
          >
            <el-icon><Link /></el-icon>
            连接控制器
          </el-button>
          <el-button
            v-else
            type="info"
            class="conn-btn disconnect"
            plain
            @click="disconnectController"
          >
            <el-icon><CircleClose /></el-icon>
            断开连接
          </el-button>
        </div>
        <el-button
          type="danger"
          class="estop-btn"
          :disabled="!motionStore.isConnected"
          @click="onEmergencyStop"
        >
          <el-icon><WarningFilled /></el-icon>
          急停
        </el-button>
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

    <!-- 轴配置对话框 -->
    <AxisConfigDialog ref="configDialog" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { Cpu, Link, CircleClose, WarningFilled } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useMotionStore } from '../stores/motion'
import { UpdateMotionProfile } from '../wails-compat/app'
import { types } from '../wails-compat/models'
import AxisConfigDialog from '../components/MotionControl/AxisConfigDialog.vue'
import AxisControlCard from '../components/MotionControl/AxisControlCard.vue'

const AXIS_COLORS = ['#b829ff', '#00f5ff', '#00ff88', '#ffaa00'] as const

const motionStore = useMotionStore()
const configDialog = ref<InstanceType<typeof AxisConfigDialog>>()

const activeProfile = computed(() => {
  const b140 = motionStore.profiles.find(p => p.type === 'B140-MC')
  const id = motionStore.activeControllerId
  if (id) {
    const found = motionStore.profiles.find(p => p.id === id)
    if (found && found.type === 'B140-MC') return found
    if (b140) return b140
    if (found) return found
  }
  if (b140) return b140
  return motionStore.profiles[0] || null
})

const editableAddress = ref('192.168.1.101')
const editablePort = ref(23)

watch(activeProfile, (profile) => {
  if (profile) {
    editableAddress.value = profile.address
    editablePort.value = profile.port
  }
}, { immediate: true })

async function onConnectionInfoChange() {
  const profile = activeProfile.value
  if (!profile) return
  if (editableAddress.value === profile.address && editablePort.value === profile.port) return

  try {
    const updated = types.MotionControllerProfile.createFrom({ ...profile, address: editableAddress.value, port: editablePort.value })
    await UpdateMotionProfile(updated)
    await motionStore.fetchProfiles()
    ElMessage.success('连接信息已保存')
  } catch (e: any) {
    ElMessage.error(`保存失败: ${e?.message || e}`)
  }
}

const connectionStatusText = computed(() => {
  switch (motionStore.connectionStatus) {
    case 'connected': return '已连接'
    case 'connecting': return '连接中'
    case 'error': return '连接错误'
    default: return '未连接'
  }
})

const connectionStatus = computed(() => motionStore.connectionStatus)

async function connectController() {
  const profile = activeProfile.value
  if (!profile) {
    ElMessage.warning('请先添加控制器')
    return
  }
  if (editableAddress.value !== profile.address || editablePort.value !== profile.port) {
    try {
      const updated = types.MotionControllerProfile.createFrom({ ...profile, address: editableAddress.value, port: editablePort.value })
      await UpdateMotionProfile(updated)
      await motionStore.fetchProfiles()
    } catch (e: any) {
      ElMessage.error(`保存连接信息失败: ${e?.message || e}`)
      return
    }
  }
  const result = await motionStore.connectController(profile.id)
  if (!result.success) {
    ElMessage.error(result.error || '连接失败')
  }
}

async function disconnectController() {
  const result = await motionStore.disconnectController()
  if (!result.success) {
    ElMessage.error(result.error || '断开失败')
  }
}

async function onEmergencyStop() {
  await motionStore.emergencyStop()
  ElMessage.warning('已触发急停，所有轴已停止')
}

function onConfigureAxis(axisName: string) {
  motionStore.selectAxis(axisName)
  openConfigDialog(axisName)
}

function openConfigDialog(axisName?: string) {
  configDialog.value?.open(axisName)
}

onMounted(async () => {
  await motionStore.fetchProfiles()
  const b140Profile = motionStore.profiles.find(p => p.type === 'B140-MC')
  if (b140Profile) {
    motionStore.activeControllerId = b140Profile.id
  }
})
</script>

<style lang="scss" scoped>
.motion-view {
  padding: 16px 20px 14px;
  height: 100%;
  display: flex;
  flex-direction: column;
  gap: 14px;
  overflow: hidden;
}

/* ========== 顶部状态栏 ========== */
.status-bar {
  display: grid;
  grid-template-columns: minmax(220px, 0.9fr) minmax(360px, 1.35fr) minmax(260px, auto);
  align-items: stretch;
  gap: 14px;
  padding: 14px 16px;
  background: $glass-bg;
  border: 1px solid $glass-border;
  border-radius: $border-radius-md;
  flex-shrink: 0;
  backdrop-filter: blur(16px);
}

.status-left {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  min-width: 0;
}

.page-brand {
  display: flex;
  align-items: center;
  gap: 10px;

  .brand-icon {
    width: 40px;
    height: 40px;
    border-radius: 10px;
    background: linear-gradient(135deg, rgba($color-primary, 0.15), rgba($color-accent, 0.1));
    border: 1px solid rgba($color-primary, 0.2);
    display: flex;
    align-items: center;
    justify-content: center;
    color: $color-primary;
    box-shadow: 0 0 15px rgba($color-primary, 0.15);
  }

  .brand-text {
    display: flex;
    flex-direction: column;
    gap: 2px;

    .page-title {
      font-size: 18px;
      font-weight: 600;
      color: $text-primary;
      margin: 0;
      line-height: 1.2;
    }

    .controller-name {
      font-size: 12px;
      color: $text-muted;
    }
  }
}

.connection-pill {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 14px;
  border-radius: 20px;
  font-size: 13px;
  font-weight: 500;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.08);
  transition: all 0.3s;
  white-space: nowrap;

  .pulse-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: $text-muted;
    transition: all 0.3s;
  }

  .conn-text {
    color: $text-muted;
    transition: color 0.3s;
  }

  &.connected {
    background: rgba($color-success, 0.08);
    border-color: rgba($color-success, 0.25);

    .pulse-dot {
      background: $color-success;
      box-shadow: 0 0 8px rgba($color-success, 0.6);
      animation: pulse-glow 2s infinite;
    }

    .conn-text {
      color: $color-success;
    }
  }

  &.connecting {
    background: rgba($color-warning, 0.08);
    border-color: rgba($color-warning, 0.25);

    .pulse-dot {
      background: $color-warning;
      animation: pulse-glow 1s infinite;
    }

    .conn-text {
      color: $color-warning;
    }
  }

  &.error {
    background: rgba($color-danger, 0.08);
    border-color: rgba($color-danger, 0.25);

    .pulse-dot {
      background: $color-danger;
      box-shadow: 0 0 8px rgba($color-danger, 0.5);
    }

    .conn-text {
      color: $color-danger;
    }
  }
}

@keyframes pulse-glow {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.5; transform: scale(1.2); }
}

.status-center {
  display: grid;
  grid-template-columns: auto minmax(140px, 1fr) minmax(100px, 0.65fr) auto;
  align-items: center;
  gap: 12px;
  padding: 0 16px;
  border-left: 1px solid $glass-border-light;
  border-right: 1px solid $glass-border-light;
}

.conn-group-title {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding-right: 4px;

  .field-label {
    color: $text-secondary;
    font-weight: 600;
  }

  .field-hint {
    font-size: 11px;
    color: $text-muted;
    white-space: nowrap;
  }
}

.conn-field {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 8px;

  .field-label {
    font-size: 12px;
    color: $text-muted;
    white-space: nowrap;
  }
}

.ip-input {
  width: 100%;
  min-width: 128px;
}

.port-input {
  width: 100%;
  min-width: 96px;
}

.type-field {
  .type-badge {
    padding: 4px 12px;
    border-radius: 6px;
    font-size: 12px;
    font-weight: 600;
    color: $color-accent;
    background: rgba($color-accent, 0.1);
    border: 1px solid rgba($color-accent, 0.2);
  }
}

.status-right {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 12px;
}

.connection-actions {
  display: flex;
  align-items: center;
}

.conn-btn {
  font-weight: 500;
  min-width: 112px;

  &.disconnect {
    opacity: 0.7;

    &:hover {
      opacity: 1;
    }
  }
}

.estop-btn {
  font-weight: 700;
  letter-spacing: 1px;
  font-size: 13px;
  min-width: 92px;
  height: 40px;
  padding: 8px 20px;
  box-shadow: 0 0 15px rgba($color-danger, 0.3);
  animation: estop-pulse 2.5s infinite;

  &:hover {
    box-shadow: 0 0 25px rgba($color-danger, 0.5);
  }

  &:disabled {
    animation: none;
    box-shadow: none;
    opacity: 0.4;
  }
}

@keyframes estop-pulse {
  0%, 100% { transform: scale(1); box-shadow: 0 0 15px rgba($color-danger, 0.3); }
  50% { transform: scale(1.02); box-shadow: 0 0 22px rgba($color-danger, 0.5); }
}

/* ========== 轴控制网格 ========== */
.axes-grid {
  flex: 1;
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  grid-template-rows: repeat(2, minmax(300px, 1fr));
  gap: 14px;
  min-height: 0;
  overflow: auto;
  padding-right: 2px;
}

@media (max-width: 900px) {
  .status-bar {
    grid-template-columns: 1fr;
  }

  .status-center {
    grid-template-columns: 1fr 1fr;
    border: none;
    padding: 10px 0 0;
  }

  .conn-group-title,
  .type-field {
    grid-column: 1 / -1;
  }

  .status-right {
    justify-content: space-between;
  }

  .axes-grid {
    grid-template-columns: 1fr;
    grid-template-rows: none;
  }
}
</style>
