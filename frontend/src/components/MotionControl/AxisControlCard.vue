<template>
  <div class="axis-card" :class="{ 'is-running': isRunning, 'is-error': isError }">
    <!-- 头部 -->
    <div class="axis-header">
      <div class="axis-title">
        <span class="axis-name">{{ axis.name }}轴</span>
        <el-tag :type="axis.kind === 'LINEAR' ? 'primary' : 'success'" size="small">
          {{ axis.kind === 'LINEAR' ? '平移轴' : '旋转轴' }}
        </el-tag>
      </div>
      <div class="axis-actions">
        <!-- 限位指示灯 -->
        <div class="limit-indicators">
          <div class="limit-item">
            <span class="dot" :class="{ active: axis.negLimitActive }"></span>
            <span class="label">-限</span>
          </div>
          <div class="limit-item">
            <span class="dot" :class="{ active: axis.posLimitActive }"></span>
            <span class="label">+限</span>
          </div>
        </div>
        <el-tag :type="runStateTagType" size="small" effect="dark">
          {{ runStateLabel }}
        </el-tag>
        <el-button type="primary" size="small" text @click="$emit('configure', axis.name)">
          <el-icon><Setting /></el-icon>
        </el-button>
      </div>
    </div>

    <!-- 位置显示 -->
    <div class="position-display">
      <div class="position-row">
        <span class="label">当前位置</span>
        <span class="value" :class="{ 'is-homed': axis.isHomed }">
          {{ axis.currentPosition.toFixed(2) }}
          <span class="unit">{{ unit }}</span>
        </span>
      </div>
      <!-- 位置进度条 -->
      <div class="position-bar">
        <div class="bar-track">
          <div class="bar-center"></div>
          <div class="bar-indicator" :style="{ left: `${positionPercent}%` }"></div>
        </div>
        <div class="bar-labels">
          <span>-{{ maxPosition }}</span>
          <span>0</span>
          <span>+{{ maxPosition }}</span>
        </div>
      </div>
    </div>

    <!-- 目标位置 & 相对距离 并列 -->
    <div class="input-row-group">
      <div class="input-group">
        <span class="input-label">目标位置</span>
        <div class="input-row">
          <el-input-number
            v-model="localTarget"
            :precision="2"
            :step="stepValue"
            :min="-maxPosition"
            :max="maxPosition"
            size="small"
            controls-position="right"
            @change="onTargetChange"
          />
          <span class="input-unit">{{ unit }}</span>
        </div>
      </div>
      <div class="input-group">
        <span class="input-label">相对距离 (Jog)</span>
        <div class="input-row">
          <el-input-number
            v-model="localRelative"
            :precision="2"
            :step="stepValue"
            :min="0.01"
            :max="maxPosition"
            size="small"
            controls-position="right"
            @change="onRelativeChange"
          />
          <span class="input-unit">{{ unit }}</span>
        </div>
      </div>
    </div>

    <!-- 控制按钮 -->
    <div class="control-buttons">
      <el-button type="primary" size="small" :disabled="!canJog" @click="onJog('minus')">
        <el-icon><ArrowLeft /></el-icon> Jog-
      </el-button>
      <el-button type="success" size="small" :disabled="!canRun" :loading="isRunning" @click="onMoveToTarget">
        <el-icon><VideoPlay /></el-icon> 运行
      </el-button>
      <el-button type="primary" size="small" :disabled="!canJog" @click="onJog('plus')">
        Jog+ <el-icon><ArrowRight /></el-icon>
      </el-button>
      <el-button type="danger" size="small" :disabled="!isRunning && !isJogging" @click="onStop">
        <el-icon><CircleClose /></el-icon> 停止
      </el-button>
    </div>

    <!-- 置零按钮 -->
    <el-button class="home-button" type="info" size="small" plain @click="onHome">
      <el-icon><Aim /></el-icon> 置零 (当前位置设为0)
    </el-button>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Setting, ArrowLeft, ArrowRight, VideoPlay, CircleClose, Aim } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useMotionStore } from '../../stores/motion'

const props = defineProps<{
  axis: {
    name: string
    kind: string
    currentPosition: number
    targetPosition: number
    relativeDistance: number
    runState: string
    isHomed: boolean
    posLimitActive: boolean
    negLimitActive: boolean
  }
}>()

defineEmits<{
  configure: [axisName: string]
}>()

const store = useMotionStore()

const localTarget = ref(props.axis.targetPosition)
const localRelative = ref(props.axis.relativeDistance)

watch(() => props.axis.targetPosition, (val) => { localTarget.value = val })
watch(() => props.axis.relativeDistance, (val) => { localRelative.value = val })

const unit = computed(() => store.getAxisUnit(props.axis.kind as any))
const maxPosition = computed(() => props.axis.kind === 'LINEAR' ? 200 : 180)
const stepValue = computed(() => props.axis.kind === 'LINEAR' ? 1 : 0.5)

const isRunning = computed(() => props.axis.runState === 'running')
const isJogging = computed(() => props.axis.runState === 'jogging_minus' || props.axis.runState === 'jogging_plus')
const isError = computed(() => props.axis.runState === 'error')
const canJog = computed(() => store.isConnected && (props.axis.runState === 'idle' || isJogging.value))
const canRun = computed(() => store.isConnected && props.axis.runState === 'idle')

const runStateLabel = computed(() => store.getRunStateText(props.axis.runState as any))
const runStateTagType = computed(() => {
  switch (props.axis.runState) {
    case 'idle': return 'info'
    case 'running': return 'success'
    case 'jogging_minus':
    case 'jogging_plus': return 'warning'
    case 'error': return 'danger'
    default: return 'info'
  }
})

const positionPercent = computed(() => {
  const pos = props.axis.currentPosition
  const max = maxPosition.value
  return Math.max(0, Math.min(100, ((pos + max) / (2 * max)) * 100))
})

function onTargetChange(val: number | undefined) {
  if (val !== undefined) store.updateAxisTarget(props.axis.name, val)
}

function onRelativeChange(val: number | undefined) {
  if (val !== undefined) store.updateAxisRelativeDistance(props.axis.name, val)
}

async function onJog(direction: 'minus' | 'plus') {
  const result = await store.startJog(props.axis.name, direction)
  if (!result.success) ElMessage.warning(result.error || `${props.axis.name}轴点动失败`)
}

async function onMoveToTarget() {
  const result = await store.moveTo(props.axis.name, localTarget.value)
  if (!result.success) ElMessage.warning(result.error || `${props.axis.name}轴运动失败`)
}

async function onStop() {
  const result = await store.stopAxis(props.axis.name)
  if (!result.success) ElMessage.warning(result.error || `${props.axis.name}轴停止失败`)
}

async function onHome() {
  const result = await store.definePosition(props.axis.name, 0)
  if (!result.success) ElMessage.warning(result.error || `${props.axis.name}轴置零失败`)
}
</script>

<style scoped lang="scss">
.axis-card {
  background: rgba(255,255,255,0.04);
  border: 1px solid rgba(255,255,255,0.08);
  border-radius: 10px;
  padding: 14px;
  display: flex;
  flex-direction: column;
  transition: all 0.3s ease;

  &:hover {
    border-color: rgba(0, 245, 255, 0.3);
    box-shadow: 0 2px 12px rgba(0, 245, 255, 0.1);
  }

  &.is-running {
    border-color: rgba(0, 255, 136, 0.4);
    box-shadow: 0 0 0 2px rgba(0, 255, 136, 0.15);
  }

  &.is-error {
    border-color: rgba(255, 51, 102, 0.4);
    box-shadow: 0 0 0 2px rgba(255, 51, 102, 0.15);
  }
}

.axis-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
  padding-bottom: 8px;
  border-bottom: 1px solid rgba(255,255,255,0.06);
}

.axis-title {
  display: flex;
  align-items: center;
  gap: 6px;
  .axis-name {
    font-size: 14px;
    font-weight: 600;
    color: #b829ff;
  }
}

.axis-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.limit-indicators {
  display: flex;
  gap: 6px;
  align-items: center;
  padding-right: 8px;
  border-right: 1px solid rgba(255,255,255,0.06);

  .limit-item {
    display: flex;
    align-items: center;
    gap: 3px;
    font-size: 10px;
    color: rgba(255,255,255,0.4);

    .dot {
      width: 8px;
      height: 8px;
      border-radius: 50%;
      background: #111;
      border: 1px solid #444;
      transition: all 0.2s;

      &.active {
        background: #ff4d4f;
        border-color: #ff4d4f;
        box-shadow: 0 0 5px rgba(255, 77, 79, 0.5);
      }
    }
  }
}

.position-display {
  margin-bottom: 12px;

  .position-row {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    margin-bottom: 8px;

    .label { font-size: 13px; color: rgba(255,255,255,0.5); }
    .value {
      font-size: 20px;
      font-weight: 600;
      color: #00f5ff;
      font-family: 'Courier New', monospace;

      &.is-homed { color: #00ff88; }
      .unit { font-size: 12px; font-weight: 400; color: rgba(255,255,255,0.4); margin-left: 3px; }
    }
  }
}

.position-bar {
  .bar-track {
    position: relative;
    height: 6px;
    background: rgba(255,255,255,0.06);
    border-radius: 3px;
    overflow: visible;

    .bar-center {
      position: absolute;
      left: 50%;
      width: 2px;
      height: 100%;
      background: rgba(255,255,255,0.2);
      transform: translateX(-50%);
      z-index: 1;
    }

    .bar-indicator {
      position: absolute;
      top: 50%;
      width: 12px;
      height: 12px;
      background: linear-gradient(135deg, #3b82f6, #10b981);
      border-radius: 50%;
      transform: translate(-50%, -50%);
      transition: left 0.1s ease;
      box-shadow: 0 0 4px rgba(59, 130, 246, 0.5);
      z-index: 2;
    }
  }

  .bar-labels {
    display: flex;
    justify-content: space-between;
    margin-top: 4px;
    font-size: 11px;
    color: rgba(255,255,255,0.25);
  }
}

.input-row-group {
  display: flex;
  gap: 10px;
  margin-bottom: 10px;
}

.input-group {
  flex: 1;
  min-width: 0;

  .input-label {
    display: block;
    font-size: 12px;
    color: rgba(255,255,255,0.5);
    margin-bottom: 5px;
  }

  .input-row {
    display: flex;
    align-items: center;
    gap: 6px;
    .el-input-number { flex: 1; }
    .input-unit { font-size: 12px; color: rgba(255,255,255,0.4); min-width: 20px; }
  }
}

.control-buttons {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr 1fr;
  gap: 8px;
  margin-bottom: 10px;

  .el-button {
    padding: 8px 4px;
    font-size: 12px;
    height: 32px;
    .el-icon { font-size: 13px; }
  }
}

.home-button {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  height: 32px;
  font-size: 12px;
}
</style>
