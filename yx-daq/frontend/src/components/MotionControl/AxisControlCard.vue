<template>
  <div
    class="axis-card"
    :class="{ 'is-running': isRunning, 'is-error': isError, 'is-connected': store.isConnected }"
    :style="cardStyle"
  >
    <!-- 头部：轴名 + 状态 -->
    <div class="axis-header">
      <div class="axis-brand">
        <div class="axis-icon" :style="iconStyle">
          <span class="axis-letter">{{ axis.name }}</span>
        </div>
        <div class="axis-meta">
          <span class="axis-name">{{ axis.name }}轴</span>
          <span class="axis-kind">{{ axis.kind === 'LINEAR' ? '平移' : '旋转' }}</span>
        </div>
      </div>
      <div class="axis-actions">
        <div class="limit-indicators">
          <span
            class="limit-dot"
            :class="{ active: axis.negLimitActive }"
            title="负限位"
          >-</span>
          <span
            class="limit-dot"
            :class="{ active: axis.posLimitActive }"
            title="正限位"
          >+</span>
        </div>
        <div class="state-badge" :class="runStateClass">
          <span class="state-dot" />
          <span class="state-text">{{ runStateLabel }}</span>
        </div>
        <el-button
          type="primary"
          size="small"
          text
          class="config-btn"
          @click="$emit('configure', axis.name)"
        >
          <el-icon><Setting /></el-icon>
        </el-button>
      </div>
    </div>

    <!-- 位置显示区 -->
    <div class="position-section">
      <div class="section-label">当前位置</div>
      <div class="position-value">
        <span class="number" :class="{ 'is-homed': axis.isHomed }">
          {{ formatPosition(axis.currentPosition) }}
        </span>
        <span class="unit">{{ unit }}</span>
      </div>
      <!-- 位置进度条 -->
      <div class="position-track">
        <div class="track-bg">
          <div
            class="track-fill"
            :style="{ width: `${positionPercent}%`, background: axisColor }"
          />
          <div
            class="track-indicator"
            :style="{ left: `${positionPercent}%`, borderColor: axisColor, background: axisColor }"
          />
        </div>
        <div class="track-labels">
          <span>-{{ formatPosition(maxPosition) }}</span>
          <span class="center">0</span>
          <span>+{{ formatPosition(maxPosition) }}</span>
        </div>
      </div>
    </div>

    <!-- 控制输入区 -->
    <div class="control-section">
      <div class="section-label">运动参数</div>
      <div class="input-row">
        <div class="input-group target-group">
          <span class="input-label">目标位置</span>
          <el-input-number
            v-model="localTarget"
            :precision="2"
            :step="stepValue"
            :min="-maxPosition"
            :max="maxPosition"
            size="small"
            controls-position="right"
            class="motion-input"
            @change="onTargetChange"
          />
        </div>
        <div class="input-group step-group">
          <span class="input-label">步距</span>
          <el-input-number
            v-model="localRelative"
            :precision="2"
            :step="stepValue"
            :min="0.01"
            :max="maxPosition"
            size="small"
            controls-position="right"
            class="motion-input"
            @change="onRelativeChange"
          />
        </div>
      </div>

      <!-- 控制按钮区 -->
      <div class="movement-section">
        <div class="jog-pair">
          <button
            class="jog-btn jog-minus"
            :disabled="!canJog"
            @mousedown="startJog('minus')"
            @mouseup="stopJog('minus')"
            @mouseleave="stopJog('minus')"
            @touchstart.prevent="startJog('minus')"
            @touchend.prevent="stopJog('minus')"
          >
            <el-icon><ArrowLeft /></el-icon>
            <span class="jog-label">负向点动</span>
          </button>

          <button
            class="jog-btn jog-plus"
            :disabled="!canJog"
            @mousedown="startJog('plus')"
            @mouseup="stopJog('plus')"
            @mouseleave="stopJog('plus')"
            @touchstart.prevent="startJog('plus')"
            @touchend.prevent="stopJog('plus')"
          >
            <span class="jog-label">正向点动</span>
            <el-icon><ArrowRight /></el-icon>
          </button>
        </div>

        <div class="action-pair">
          <button
            class="action-btn run-btn"
            :class="{ running: isRunning }"
            :disabled="!canRun"
            @click="onMoveToTarget"
          >
            <el-icon v-if="isRunning" class="spin-icon"><Loading /></el-icon>
            <el-icon v-else><VideoPlay /></el-icon>
            <span>{{ isRunning ? '运行中' : '运行到目标' }}</span>
          </button>

          <button
            class="action-btn stop-btn"
            :disabled="!isRunning && !isJogging"
            @click="onStop"
          >
            <el-icon><VideoPause /></el-icon>
            <span>停止</span>
          </button>
        </div>
      </div>

      <button
        class="home-btn"
        :disabled="!store.isConnected"
        @click="onHome"
      >
        <el-icon><Aim /></el-icon>
        <span>置零</span>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import {
  Setting, ArrowLeft, ArrowRight, Aim,
  VideoPlay, VideoPause, Loading,
} from '@element-plus/icons-vue'
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
  axisColor: string
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
const runStateClass = computed(() => {
  switch (props.axis.runState) {
    case 'idle': return 'state-idle'
    case 'running': return 'state-running'
    case 'jogging_minus':
    case 'jogging_plus': return 'state-jogging'
    case 'error': return 'state-error'
    default: return 'state-idle'
  }
})

const positionPercent = computed(() => {
  const pos = props.axis.currentPosition
  const max = maxPosition.value
  return Math.max(0, Math.min(100, ((pos + max) / (2 * max)) * 100))
})

const cardStyle = computed(() => ({
  '--axis-color': props.axisColor,
  '--axis-color-glow': props.axisColor + '4D',
  '--axis-color-dim': props.axisColor + '1A',
}))

const iconStyle = computed(() => ({
  background: `linear-gradient(135deg, ${props.axisColor}26, ${props.axisColor}0D)`,
  borderColor: `${props.axisColor}33`,
  color: props.axisColor,
  boxShadow: `0 0 15px ${props.axisColor}1A`,
}))

function formatPosition(val: number) {
  return val.toFixed(2)
}

function onTargetChange(val: number | undefined) {
  if (val !== undefined) store.updateAxisTarget(props.axis.name, val)
}

function onRelativeChange(val: number | undefined) {
  if (val !== undefined) store.updateAxisRelativeDistance(props.axis.name, val)
}

let jogDirection: 'minus' | 'plus' | null = null

async function startJog(direction: 'minus' | 'plus') {
  if (jogDirection) return
  jogDirection = direction
  const result = await store.startJog(props.axis.name, direction)
  if (!result.success) {
    ElMessage.warning(result.error || `${props.axis.name}轴点动失败`)
    jogDirection = null
  }
}

async function stopJog(direction: 'minus' | 'plus') {
  if (jogDirection !== direction) return
  jogDirection = null
  if (props.axis.runState === 'jogging_minus' || props.axis.runState === 'jogging_plus') {
    await store.stopJog(props.axis.name)
  }
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

<style lang="scss" scoped>
.axis-card {
  --axis-color: #00f5ff;
  --axis-color-glow: rgba(0, 245, 255, 0.3);
  --axis-color-dim: rgba(0, 245, 255, 0.1);

  background: $glass-bg;
  border: 1px solid $glass-border;
  border-radius: $border-radius-md;
  padding: 14px 16px 16px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  transition: all 0.3s ease;
  min-height: 0;
  overflow: visible;
  position: relative;

  &::before {
    content: '';
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    height: 2px;
    background: var(--axis-color);
    opacity: 0.3;
    transition: opacity 0.3s;
  }

  &:hover {
    border-color: var(--axis-color-glow);
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.3), inset 0 1px 0 rgba(255, 255, 255, 0.05);

    &::before {
      opacity: 0.6;
    }
  }

  &.is-running {
    border-color: var(--axis-color-glow);
    background: linear-gradient(180deg, var(--axis-color-dim) 0%, $glass-bg 60%);
    box-shadow: 0 0 30px var(--axis-color-dim), inset 0 1px 0 rgba(255, 255, 255, 0.05);
    animation: card-breathe 3s ease-in-out infinite;

    &::before {
      opacity: 0.8;
      box-shadow: 0 0 10px var(--axis-color);
    }
  }

  &.is-error {
    border-color: rgba($color-danger, 0.3);
    background: linear-gradient(180deg, rgba($color-danger, 0.05) 0%, $glass-bg 60%);

    &::before {
      background: $color-danger;
      opacity: 0.6;
    }
  }

  &:not(.is-connected) {
    opacity: 0.7;
  }
}

@keyframes card-breathe {
  0%, 100% { box-shadow: 0 0 20px var(--axis-color-dim), inset 0 1px 0 rgba(255, 255, 255, 0.05); }
  50% { box-shadow: 0 0 35px var(--axis-color-glow), inset 0 1px 0 rgba(255, 255, 255, 0.08); }
}

/* ========== 头部 ========== */
.axis-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.axis-brand {
  display: flex;
  align-items: center;
  gap: 10px;
}

.axis-icon {
  width: 34px;
  height: 34px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid;
  transition: all 0.3s;

  .axis-letter {
    font-size: 18px;
    font-weight: 700;
    line-height: 1;
  }
}

.axis-meta {
  display: flex;
  flex-direction: column;
  gap: 2px;

  .axis-name {
    font-size: 15px;
    font-weight: 600;
    color: $text-primary;
    line-height: 1.2;
  }

  .axis-kind {
    font-size: 11px;
    color: $text-muted;
  }
}

.axis-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.limit-indicators {
  display: flex;
  gap: 4px;

  .limit-dot {
    width: 20px;
    height: 20px;
    border-radius: 5px;
    background: rgba(255, 255, 255, 0.04);
    border: 1px solid rgba(255, 255, 255, 0.08);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 11px;
    color: rgba(255, 255, 255, 0.2);
    font-weight: 700;
    transition: all 0.2s;

    &.active {
      background: rgba($color-danger, 0.15);
      border-color: rgba($color-danger, 0.4);
      color: $color-danger;
      box-shadow: 0 0 8px rgba($color-danger, 0.3);
    }
  }
}

.state-badge {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 4px 10px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 500;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.08);
  transition: all 0.3s;

  .state-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: $text-muted;
    transition: all 0.3s;
  }

  .state-text {
    color: $text-muted;
  }

  &.state-idle {
    .state-dot { background: $text-muted; }
    .state-text { color: $text-muted; }
  }

  &.state-running {
    background: rgba($color-success, 0.08);
    border-color: rgba($color-success, 0.2);

    .state-dot {
      background: $color-success;
      box-shadow: 0 0 6px rgba($color-success, 0.5);
      animation: pulse-glow 1.5s infinite;
    }
    .state-text { color: $color-success; }
  }

  &.state-jogging {
    background: rgba($color-warning, 0.08);
    border-color: rgba($color-warning, 0.2);

    .state-dot {
      background: $color-warning;
      box-shadow: 0 0 6px rgba($color-warning, 0.5);
      animation: pulse-glow 1s infinite;
    }
    .state-text { color: $color-warning; }
  }

  &.state-error {
    background: rgba($color-danger, 0.08);
    border-color: rgba($color-danger, 0.2);

    .state-dot {
      background: $color-danger;
      box-shadow: 0 0 6px rgba($color-danger, 0.5);
    }
    .state-text { color: $color-danger; }
  }
}

@keyframes pulse-glow {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}

.config-btn {
  padding: 6px;
  color: $text-muted;

  &:hover {
    color: var(--axis-color);
  }
}

/* ========== 位置显示 ========== */
.position-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px 12px;
  border-radius: 12px;
  background: linear-gradient(135deg, var(--axis-color-dim), rgba(255, 255, 255, 0.025));
  border: 1px solid rgba(255, 255, 255, 0.06);
}

.section-label {
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.5px;
  color: $text-muted;
  line-height: 1;
}

.position-value {
  display: flex;
  align-items: baseline;
  justify-content: flex-start;
  gap: 6px;

  .number {
    font-size: clamp(26px, 2.8vw, 32px);
    font-weight: 700;
    color: var(--axis-color);
    font-family: 'SF Mono', Monaco, 'Cascadia Code', Consolas, 'Courier New', monospace;
    line-height: 1;
    text-shadow: 0 0 20px var(--axis-color-glow);
    transition: all 0.3s;

    &.is-homed {
      color: $color-success;
      text-shadow: 0 0 20px rgba($color-success, 0.3);
    }
  }

  .unit {
    font-size: 13px;
    color: $text-muted;
    font-weight: 500;
  }
}

.position-track {
  .track-bg {
    position: relative;
    height: 6px;
    background: rgba(255, 255, 255, 0.04);
    border-radius: 4px;
    overflow: visible;
  }

  .track-fill {
    position: absolute;
    left: 0;
    top: 0;
    bottom: 0;
    border-radius: 4px;
    opacity: 0.25;
    transition: width 0.2s ease;
  }

  .track-indicator {
    position: absolute;
    top: 50%;
    width: 14px;
    height: 14px;
    border-radius: 50%;
    border: 2px solid;
    background: var(--axis-color);
    transform: translate(-50%, -50%);
    box-shadow: 0 0 10px var(--axis-color-glow);
    transition: left 0.2s ease;
    z-index: 2;
  }

  .track-labels {
    display: flex;
    justify-content: space-between;
    margin-top: 6px;
    font-size: 10px;
    color: $text-muted;
    font-family: 'SF Mono', Monaco, 'Cascadia Code', Consolas, 'Courier New', monospace;

    .center {
      color: rgba(255, 255, 255, 0.25);
    }
  }
}

/* ========== 控制区 ========== */
.control-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.input-row {
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
    color: $text-muted;
    font-weight: 500;
  }

  :deep(.motion-input) {
    width: 100%;

    .el-input__wrapper {
      background: rgba(0, 0, 0, 0.25);
      box-shadow: inset 0 1px 3px rgba(0, 0, 0, 0.3);
    }

    .el-input__inner {
      color: $text-primary;
      font-family: 'SF Mono', Monaco, 'Cascadia Code', Consolas, 'Courier New', monospace;
      font-weight: 500;
    }
  }
}

/* ========== 操作按钮 ========== */
.movement-section {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}

.jog-pair,
.action-pair {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}

.jog-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  min-height: 34px;
  padding: 7px 8px;
  border-radius: 8px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(255, 255, 255, 0.04);
  color: $text-secondary;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s;
  user-select: none;

  &:hover:not(:disabled) {
    background: rgba(255, 255, 255, 0.08);
    border-color: var(--axis-color-glow);
    color: var(--axis-color);
  }

  &:active:not(:disabled) {
    transform: scale(0.96);
    background: var(--axis-color-dim);
  }

  &:disabled {
    opacity: 0.3;
    cursor: not-allowed;
  }

  .jog-label {
    font-weight: 700;
    font-size: 12px;
    white-space: nowrap;
  }
}

.action-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  min-height: 34px;
  padding: 7px 8px;
  border-radius: 8px;
  border: none;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s;
  user-select: none;

  &:disabled {
    opacity: 0.3;
    cursor: not-allowed;
  }
}

.run-btn {
  background: rgba($color-success, 0.12);
  color: $color-success;
  border: 1px solid rgba($color-success, 0.2);

  &:hover:not(:disabled) {
    background: rgba($color-success, 0.2);
    border-color: rgba($color-success, 0.35);
    box-shadow: 0 0 15px rgba($color-success, 0.15);
  }

  &:active:not(:disabled) {
    transform: scale(0.96);
  }

  &.running {
    background: rgba($color-success, 0.2);
    border-color: rgba($color-success, 0.4);
    box-shadow: 0 0 20px rgba($color-success, 0.2);
  }
}

.stop-btn {
  background: rgba($color-danger, 0.12);
  color: $color-danger;
  border: 1px solid rgba($color-danger, 0.2);

  &:hover:not(:disabled) {
    background: rgba($color-danger, 0.2);
    border-color: rgba($color-danger, 0.35);
    box-shadow: 0 0 15px rgba($color-danger, 0.15);
  }

  &:active:not(:disabled) {
    transform: scale(0.96);
  }
}

.spin-icon {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.home-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  min-height: 32px;
  padding: 6px 0;
  border-radius: 8px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(255, 255, 255, 0.03);
  color: $text-muted;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s;

  &:hover:not(:disabled) {
    background: rgba(255, 255, 255, 0.06);
    border-color: rgba(255, 255, 255, 0.12);
    color: $text-secondary;
  }

  &:disabled {
    opacity: 0.3;
    cursor: not-allowed;
  }
}

@media (max-width: 1180px) {
  .movement-section {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 560px) {
  .axis-header {
    align-items: flex-start;
  }

  .input-row,
  .jog-pair,
  .action-pair {
    grid-template-columns: 1fr;
  }
}
</style>
