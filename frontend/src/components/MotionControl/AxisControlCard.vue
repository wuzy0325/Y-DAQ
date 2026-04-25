<template>
  <div class="axis-card" :class="{ 'is-running': isRunning, 'is-error': isError }">
    <!-- 头部 -->
    <div class="axis-header">
      <div class="axis-title">
        <span class="axis-name">{{ axis.name }}轴</span>
        <el-tag :type="axis.kind === 'LINEAR' ? 'primary' : 'success'" size="small">
          {{ axis.kind === 'LINEAR' ? '平移' : '旋转' }}
        </el-tag>
      </div>
      <div class="axis-actions">
        <div class="limit-indicators">
          <span class="limit-dot" :class="{ active: axis.negLimitActive }" title="负限位">-</span>
          <span class="limit-dot" :class="{ active: axis.posLimitActive }" title="正限位">+</span>
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
      <div class="position-value">
        <span class="number" :class="{ 'is-homed': axis.isHomed }">
          {{ axis.currentPosition.toFixed(2) }}
        </span>
        <span class="unit">{{ unit }}</span>
      </div>
      <!-- 简化位置条 -->
      <div class="position-bar">
        <div class="bar-track">
          <div class="bar-indicator" :style="{ left: `${positionPercent}%` }"></div>
        </div>
        <div class="bar-labels">
          <span>-{{ maxPosition }}</span>
          <span>0</span>
          <span>+{{ maxPosition }}</span>
        </div>
      </div>
    </div>

    <!-- 输入控制 -->
    <div class="control-row">
      <div class="input-group">
        <span class="input-label">目标</span>
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
      </div>
      <div class="input-group">
        <span class="input-label">步距</span>
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
      </div>
    </div>

    <!-- 控制按钮 -->
    <div class="button-row">
      <el-button size="small" :disabled="!canJog" @click="onJog('minus')">
        <el-icon><ArrowLeft /></el-icon>
      </el-button>
      <el-button type="success" size="small" :disabled="!canRun" :loading="isRunning" @click="onMoveToTarget">
        运行
      </el-button>
      <el-button size="small" :disabled="!canJog" @click="onJog('plus')">
        <el-icon><ArrowRight /></el-icon>
      </el-button>
      <el-button type="danger" size="small" :disabled="!isRunning && !isJogging" @click="onStop">
        停止
      </el-button>
    </div>

    <el-button class="home-btn" type="info" size="small" plain @click="onHome">
      <el-icon><Aim /></el-icon> 置零
    </el-button>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Setting, ArrowLeft, ArrowRight, Aim } from '@element-plus/icons-vue'
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
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  transition: all 0.2s;

  &:hover {
    border-color: rgba(0, 245, 255, 0.2);
  }

  &.is-running {
    border-color: rgba(0, 255, 136, 0.3);
    background: rgba(0, 255, 136, 0.03);
  }

  &.is-error {
    border-color: rgba(255, 77, 79, 0.3);
    background: rgba(255, 77, 79, 0.03);
  }
}

.axis-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.axis-title {
  display: flex;
  align-items: center;
  gap: 8px;

  .axis-name {
    font-size: 14px;
    font-weight: 600;
    color: rgba(255, 255, 255, 0.9);
  }
}

.axis-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.limit-indicators {
  display: flex;
  gap: 4px;

  .limit-dot {
    width: 16px;
    height: 16px;
    border-radius: 4px;
    background: rgba(255, 255, 255, 0.06);
    border: 1px solid rgba(255, 255, 255, 0.1);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 10px;
    color: rgba(255, 255, 255, 0.3);
    font-weight: 600;

    &.active {
      background: rgba(255, 77, 79, 0.2);
      border-color: rgba(255, 77, 79, 0.5);
      color: #ff4d4f;
    }
  }
}

.position-display {
  .position-value {
    text-align: center;
    margin-bottom: 8px;

    .number {
      font-size: 24px;
      font-weight: 600;
      color: #00f5ff;
      font-family: 'Courier New', monospace;

      &.is-homed {
        color: #00ff88;
      }
    }

    .unit {
      font-size: 12px;
      color: rgba(255, 255, 255, 0.4);
      margin-left: 4px;
    }
  }
}

.position-bar {
  .bar-track {
    position: relative;
    height: 4px;
    background: rgba(255, 255, 255, 0.06);
    border-radius: 2px;

    .bar-indicator {
      position: absolute;
      top: 50%;
      width: 8px;
      height: 8px;
      background: #00f5ff;
      border-radius: 50%;
      transform: translate(-50%, -50%);
      transition: left 0.1s;
    }
  }

  .bar-labels {
    display: flex;
    justify-content: space-between;
    margin-top: 4px;
    font-size: 10px;
    color: rgba(255, 255, 255, 0.3);
  }
}

.control-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}

.input-group {
  display: flex;
  flex-direction: column;
  gap: 4px;

  .input-label {
    font-size: 11px;
    color: rgba(255, 255, 255, 0.4);
  }

  :deep(.el-input-number) {
    width: 100%;
  }
}

.button-row {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 6px;
}

.home-btn {
  width: 100%;
}
</style>
