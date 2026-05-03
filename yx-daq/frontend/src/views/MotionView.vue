<template>
  <div class="motion-view">
    <!-- 页面头部 -->
    <div class="page-header">
      <div class="header-left">
        <h2 class="page-title">运动控制器</h2>
        <span v-if="activeProfile" class="controller-name">{{ activeProfile.name }}</span>
      </div>
      <div class="header-right">
        <!-- 连接状态 -->
        <el-tag :type="connectionStatusType" size="large" effect="dark">
          {{ connectionStatusText }}
        </el-tag>

        <!-- 连接/断开按钮 -->
        <div class="header-action-group">
          <el-button
            v-if="!motionStore.isConnected"
            type="success"
            :loading="motionStore.connectionStatus === 'connecting'"
            @click="connectController"
          >
            连接
          </el-button>
          <el-button
            v-else
            type="danger"
            plain
            @click="disconnectController"
          >
            断开
          </el-button>
        </div>

        <!-- 紧急停止 -->
        <el-button
          type="danger"
          class="estop-btn"
          :disabled="!motionStore.isAnyAxisRunning"
          @click="onEmergencyStop"
        >
          紧急停止
        </el-button>
      </div>
    </div>

    <!-- 主内容区 -->
    <div class="main-content">
      <!-- 左侧：轴控制面板 -->
      <GlassCard title="轴控制面板" icon="🎮" class="control-panel">
        <div class="axes-grid">
          <AxisControlCard
            v-for="axis in motionStore.allAxes"
            :key="axis.name"
            :axis="axis"
            @configure="onConfigureAxis"
          />
        </div>
      </GlassCard>

      <!-- 右侧：系统状态 -->
      <div class="status-panel">
        <!-- 连接信息 -->
        <div class="status-section">
          <div class="section-title">连接信息</div>
          <div class="info-list">
            <div class="info-item">
              <span class="label">IP地址</span>
              <el-input
                v-model="editableAddress"
                size="small"
                style="width: 140px"
                :disabled="motionStore.isConnected"
                @change="onConnectionInfoChange"
              />
            </div>
            <div class="info-item">
              <span class="label">端口号</span>
              <el-input-number
                v-model="editablePort"
                size="small"
                :min="1"
                :max="65535"
                style="width: 120px"
                :disabled="motionStore.isConnected"
                @change="onConnectionInfoChange"
              />
            </div>
            <div class="info-item">
              <span class="label">控制器类型</span>
              <span class="value">{{ activeProfile?.type === 'B140-MC' ? 'B140' : (activeProfile?.type || 'B140') }}</span>
            </div>
            <div class="info-item">
              <span class="label">连接状态</span>
              <span class="value" :class="motionStore.connectionStatus">{{ connectionStatusText }}</span>
            </div>
          </div>
        </div>

        <!-- 各轴位置 -->
        <div class="status-section">
          <div class="section-title">各轴位置</div>
          <div class="position-list">
            <div
              v-for="axis in motionStore.allAxes"
              :key="axis.name"
              class="position-item"
              :class="{ 'is-selected': motionStore.selectedAxis === axis.name }"
              @click="motionStore.selectAxis(axis.name)"
            >
              <div class="axis-info">
                <span class="axis-name">{{ axis.name }}轴</span>
                <el-tag :type="axis.kind === 'LINEAR' ? 'primary' : 'success'" size="small">
                  {{ axis.kind === 'LINEAR' ? '平移' : '旋转' }}
                </el-tag>
              </div>
              <div class="position-value">
                <span class="value" :class="{ 'is-homed': axis.isHomed }">
                  {{ axis.currentPosition.toFixed(2) }}
                </span>
                <span class="unit">{{ motionStore.getAxisUnit(axis.kind as any) }}</span>
              </div>
              <div class="state-badge" :class="axis.runState">
                {{ motionStore.getRunStateText(axis.runState as any) }}
              </div>
            </div>
          </div>
        </div>

        <!-- 运行日志 -->
        <div class="status-section log-section">
          <div class="section-title-row">
            <div class="section-title">运行日志</div>
            <el-button type="primary" link size="small" @click="motionStore.clearLogs">清空</el-button>
          </div>
          <div class="log-container">
            <div v-for="(log, index) in motionStore.logs" :key="index" class="log-item">{{ log }}</div>
            <div v-if="motionStore.logs.length === 0" class="log-empty">暂无日志</div>
          </div>
        </div>
      </div>
    </div>

    <!-- 轴配置对话框 -->
    <AxisConfigDialog ref="configDialog" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { useMotionStore } from '../stores/motion'
import GlassCard from '../components/GlassCard.vue'
import AxisConfigDialog from '../components/MotionControl/AxisConfigDialog.vue'
import AxisControlCard from '../components/MotionControl/AxisControlCard.vue'

const motionStore = useMotionStore()
const configDialog = ref<InstanceType<typeof AxisConfigDialog>>()

const activeProfile = computed(() => {
  // 优先选择B140控制器（真实硬件）
  const b140 = motionStore.profiles.find(p => p.type === 'B140-MC')
  const id = motionStore.activeControllerId
  if (id) {
    const found = motionStore.profiles.find(p => p.id === id)
    // 如果当前选中的是B140，直接返回
    if (found && found.type === 'B140-MC') return found
    // 如果当前选中的是模拟控制器，但存在B140，优先返回B140
    if (b140) return b140
    if (found) return found
  }
  // 没有activeControllerId时，优先B140
  if (b140) return b140
  return motionStore.profiles[0] || null
})

// 可编辑的连接信息（默认值对应B140控制器）
const editableAddress = ref('192.168.1.101')
const editablePort = ref(5000)

// 同步 profile 的 address/port 到可编辑字段
watch(activeProfile, (profile) => {
  if (profile) {
    editableAddress.value = profile.address
    editablePort.value = profile.port
  }
}, { immediate: true })

// 修改 IP/端口后保存到后端
async function onConnectionInfoChange() {
  const profile = activeProfile.value
  if (!profile) return
  if (editableAddress.value === profile.address && editablePort.value === profile.port) return

  try {
    const { UpdateMotionProfile } = await import('../../wailsjs/go/main/App')
    const { types } = await import('../../wailsjs/go/models')
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

const connectionStatusType = computed(() => {
  switch (motionStore.connectionStatus) {
    case 'connected': return 'success'
    case 'connecting': return 'warning'
    case 'error': return 'danger'
    default: return 'info'
  }
})

async function connectController() {
  const profile = activeProfile.value
  if (!profile) {
    ElMessage.warning('请先添加控制器')
    return
  }
  // 连接前先同步最新的IP/端口到后端profile，确保连接使用最新地址
  if (editableAddress.value !== profile.address || editablePort.value !== profile.port) {
    try {
      const { UpdateMotionProfile } = await import('../../wailsjs/go/main/App')
      const { types } = await import('../../wailsjs/go/models')
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
  // 确保profiles已加载
  await motionStore.fetchProfiles()
  // 如果存在B140控制器，优先选中它
  if (motionStore.profiles.length > 0) {
    const b140Profile = motionStore.profiles.find(p => p.type === 'B140-MC')
    if (b140Profile) {
      // 设置activeControllerId为B140
      if (motionStore.activeControllerId !== b140Profile.id) {
        motionStore.activeControllerId = b140Profile.id
      }
      // 如果未连接，尝试连接B140
      if (!motionStore.isConnected) {
        motionStore.connectController(b140Profile.id)
      }
    }
  }
})
</script>

<style lang="scss" scoped>
.motion-view {
  padding: 12px 16px;
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
  padding-bottom: 10px;
  border-bottom: 1px solid $glass-border-light;
  flex-shrink: 0;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
  .page-title {
    font-size: 20px;
    font-weight: 600;
    color: $color-accent;
    margin: 0;
  }
  .controller-name {
    font-size: 13px;
    color: rgba(255,255,255,0.4);
    padding: 3px 10px;
    background: $glass-bg;
    border-radius: 4px;
  }
}

.header-right {
  display: flex;
  align-items: center;
  gap: 12px;

  .header-action-group {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 0 12px;
    border-left: 1px solid $glass-border-light;
    border-right: 1px solid $glass-border-light;
  }

  .estop-btn {
    font-weight: 700;
    letter-spacing: 1px;
    box-shadow: 0 0 15px rgba(239, 68, 68, 0.4);
    animation: estop-pulse 2s infinite;
    &:hover { box-shadow: 0 0 25px rgba(239, 68, 68, 0.6); }
  }
}

@keyframes estop-pulse {
  0% { transform: scale(1); }
  50% { transform: scale(1.02); box-shadow: 0 0 20px rgba(239, 68, 68, 0.6); }
  100% { transform: scale(1); }
}

.main-content {
  flex: 1;
  display: grid;
  grid-template-columns: 1fr 260px;
  gap: 12px;
  min-height: 0;
  overflow: hidden;
}

.control-panel {
  :deep(.card-body) {
    flex: 1;
    min-height: 0;
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }
}

.axes-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  grid-template-rows: repeat(2, 1fr);
  gap: 12px;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}

.status-panel {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-height: 0;
  overflow-y: auto;
}

.status-section {
  background: $glass-bg;
  border: 1px solid $glass-border;
  border-radius: 8px;
  padding: 10px;
  flex-shrink: 0;

  .section-title {
    font-size: 12px;
    font-weight: 600;
    color: rgba(255, 255, 255, 0.8);
    margin-bottom: 8px;
    padding-left: 6px;
    border-left: 2px solid rgba($color-accent, 0.5);
  }

  .section-title-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 8px;
    .section-title { margin-bottom: 0; }
  }
}

.info-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  .info-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 6px 10px;
    background: rgba(255,255,255,0.03);
    border-radius: 6px;

    .label { 
      font-size: 11px; 
      color: rgba(255,255,255,0.4);
    }
    .value {
      font-size: 12px;
      font-weight: 500;
      color: rgba(255,255,255,0.8);

      &.connected { color: $color-success; }
      &.disconnected { color: rgba(255,255,255,0.3); }
      &.error { color: $color-danger; }
    }
  }
}

.position-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  .position-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 10px;
    background: rgba(255,255,255,0.03);
    border-radius: 6px;
    cursor: pointer;
    transition: all 0.2s;
    border: 1px solid transparent;

    &:hover { 
      background: rgba(255,255,255,0.05);
    }

    &.is-selected {
      border-color: rgba($color-accent, 0.3);
      background: rgba($color-accent, 0.05);
    }

    .axis-info {
      display: flex;
      align-items: center;
      gap: 6px;
      .axis-name { 
        font-size: 13px; 
        font-weight: 600; 
        color: rgba(255,255,255,0.85);
        width: 24px;
      }
    }

    .position-value {
      display: flex;
      align-items: baseline;
      gap: 3px;
      .value {
        font-size: 14px;
        font-weight: 600;
        color: $color-accent;
        font-family: 'Courier New', monospace;

        &.is-homed { color: $color-success; }
      }
      .unit { font-size: 11px; color: rgba(255,255,255,0.4); }
    }

    .state-badge {
      font-size: 10px;
      padding: 2px 6px;
      border-radius: 9px;
      font-weight: 500;

      &.idle { 
        background: rgba(255,255,255,0.05); 
        color: rgba(255,255,255,0.4); 
      }
      &.running {
        background: rgba($color-success, 0.1);
        color: $color-success;
      }
      &.jogging_minus, &.jogging_plus {
        background: rgba($color-warning, 0.1);
        color: $color-warning;
      }
      &.error {
        background: rgba($color-danger, 0.1);
        color: $color-danger;
      }
    }
  }
}

.log-section {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;

  .log-container {
    flex: 1;
    overflow-y: auto;
    background: rgba(255,255,255,0.02);
    border-radius: 6px;
    padding: 8px;
    font-family: 'Courier New', monospace;
    font-size: 11px;
    line-height: 1.5;

    .log-item {
      color: rgba(255,255,255,0.4);
      padding: 1px 0;
      border-bottom: 1px solid rgba(255,255,255,0.03);
    }

    .log-empty {
      color: rgba(255,255,255,0.2);
      text-align: center;
      padding: 12px;
    }
  }
}

@media (max-width: 1200px) {
  .main-content {
    grid-template-columns: 1fr;
    .control-panel { min-height: 400px; }
  }
}

@media (max-width: 768px) {
  .axes-grid { grid-template-columns: 1fr; }
  .header-right { flex-wrap: wrap; justify-content: flex-end; }
}
</style>
