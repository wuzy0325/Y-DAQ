<template>
  <div
    class="axis-card"
    :class="cardClasses"
    :style="cardStyle"
  >
    <!-- 顶部条 + 头部 -->
    <div class="axis-header">
      <div class="axis-brand">
        <div class="axis-icon" :style="iconStyle">
          <span class="axis-letter">{{ axis.name }}</span>
        </div>
        <div class="axis-meta">
          <span class="axis-name">{{ axis.name }}轴</span>
          <span class="axis-kind">{{ axis.kind === 'LINEAR' ? '平移轴' : '旋转轴' }}</span>
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

    <!-- 位置显示区：大号数字 + 置零按钮 -->
    <div class="position-section">
      <div class="position-header">
        <div class="position-value">
          <span class="number" :class="{ 'is-homed': axis.isHomed }">
            {{ formatPosition(axis.currentPosition) }}
          </span>
          <span class="unit">{{ unit }}</span>
        </div>
        <button
          class="home-btn-inline"
          :disabled="!store.isConnected"
          @click="onHome"
        >
          <el-icon><Aim /></el-icon>
          <span>置零</span>
        </button>
      </div>
      <div class="position-track">
        <span class="track-end">-{{ formatPosition(maxPosition) }}</span>
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
        <span class="track-end">+{{ formatPosition(maxPosition) }}</span>
      </div>
      <div v-if="axis.negLimitActive || axis.posLimitActive" class="limit-warning">
        <el-icon><WarningFilled /></el-icon>
        <span>{{ axis.posLimitActive ? '已触发正限位' : '已触发负限位' }}，请反向点动退出</span>
      </div>
    </div>

    <!-- 控制区：点动（独立分区） -->
    <div class="control-block jog-block">
      <div class="block-header">
        <span class="block-title">点动步距</span>
        <span class="block-hint">相对移动</span>
      </div>
      <div class="block-body">
        <el-input-number
          v-model="localRelative"
          :precision="2"
          :step="stepValue"
          :min="0.01"
          :max="maxPosition"
          size="small"
          controls-position="right"
          class="step-input"
          @change="onRelativeChange"
        />
        <div class="block-actions">
          <button
            class="jog-btn jog-minus"
            :disabled="!canRun"
            @click="onJogMove('minus')"
          >
            <el-icon><Minus /></el-icon>
            <span>负向</span>
          </button>
          <button
            class="jog-btn jog-plus"
            :disabled="!canRun"
            @click="onJogMove('plus')"
          >
            <el-icon><Plus /></el-icon>
            <span>正向</span>
          </button>
        </div>
      </div>
    </div>

    <!-- 控制区：目标位置（独立分区） -->
    <div class="control-block target-block">
      <div class="block-header">
        <span class="block-title">目标位置</span>
        <span class="block-hint">绝对定位</span>
      </div>
      <div class="block-body">
        <el-input-number
          v-model="localTarget"
          :precision="2"
          :step="stepValue"
          :min="-maxPosition"
          :max="maxPosition"
          size="small"
          controls-position="right"
          class="target-input"
          @change="onTargetChange"
        />
        <div class="block-actions">
          <button
            class="run-btn"
            :class="{ running: isRunning }"
            :disabled="!canRun"
            @click="onMoveToTarget"
          >
            <el-icon v-if="isRunning" class="spin-icon"><Loading /></el-icon>
            <el-icon v-else><VideoPlay /></el-icon>
            <span>运行</span>
          </button>
          <button
            class="stop-btn"
            :disabled="!isRunning && !isJogging"
            @click="onStop"
          >
            <el-icon><VideoPause /></el-icon>
            <span>停止</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import {
  Setting, Aim,
  VideoPlay, VideoPause, Loading, WarningFilled,
  Minus, Plus,
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
const hasLimit = computed(() => props.axis.posLimitActive || props.axis.negLimitActive)
const canRun = computed(() => store.isConnected && props.axis.runState === 'idle')

const cardClasses = computed(() => ({
  'is-running': isRunning.value,
  'is-error': isError.value,
  'is-connected': store.isConnected,
  'has-limit': hasLimit.value,
}))

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

async function onJogMove(direction: 'minus' | 'plus') {
  const delta = direction === 'minus' ? -localRelative.value : localRelative.value
  const result = await store.moveBy(props.axis.name, delta)
  if (!result.success) ElMessage.warning(result.error || `${props.axis.name}轴移动失败`)
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
  padding: 12px 14px 14px;
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
    opacity: 0.35;
    transition: opacity 0.3s;
  }

  &:hover {
    border-color: var(--axis-color-glow);
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.3), inset 0 1px 0 rgba(255, 255, 255, 0.05);

    &::before {
      opacity: 0.7;
    }
  }

  &.is-running {
    border-color: var(--axis-color-glow);
    background: linear-gradient(180deg, var(--axis-color-dim) 0%, $glass-bg 60%);
    box-shadow: 0 0 30px var(--axis-color-dim), inset 0 1px 0 rgba(255, 255, 255, 0.05);
    animation: card-breathe 3s ease-in-out infinite;

    &::before {
      opacity: 0.9;
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

  &.has-limit {
    border-color: rgba($color-danger, 0.4);
    animation: limit-flash 1s ease-in-out infinite;

    &::before {
      background: $color-danger;
      opacity: 0.7;
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

@keyframes limit-flash {
  0%, 100% { border-color: rgba($color-danger, 0.4); }
  50% { border-color: rgba($color-danger, 0.7); }
}

/* ========== 头部 ========== */
.axis-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.axis-brand {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.axis-icon {
  width: 30px;
  height: 30px;
  border-radius: 9px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid;
  transition: all 0.3s;
  flex-shrink: 0;

  .axis-letter {
    font-size: 16px;
    font-weight: 700;
    line-height: 1;
  }
}

.axis-meta {
  display: flex;
  flex-direction: column;
  gap: 1px;
  min-width: 0;

  .axis-name {
    font-size: 14px;
    font-weight: 600;
    color: $text-primary;
    line-height: 1.2;
  }

  .axis-kind {
    font-size: 10px;
    color: $text-muted;
  }
}

.axis-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.limit-indicators {
  display: flex;
  gap: 3px;

  .limit-dot {
    width: 18px;
    height: 18px;
    border-radius: 4px;
    background: rgba(255, 255, 255, 0.04);
    border: 1px solid rgba(255, 255, 255, 0.08);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 10px;
    color: rgba(255, 255, 255, 0.2);
    font-weight: 700;
    transition: all 0.2s;

    &.active {
      background: rgba($color-danger, 0.2);
      border-color: rgba($color-danger, 0.5);
      color: $color-danger;
      box-shadow: 0 0 10px rgba($color-danger, 0.4);
      animation: limit-dot-pulse 0.8s ease-in-out infinite;
    }
  }
}

@keyframes limit-dot-pulse {
  0%, 100% { box-shadow: 0 0 10px rgba($color-danger, 0.4); }
  50% { box-shadow: 0 0 18px rgba($color-danger, 0.7); }
}

.state-badge {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 2px 7px;
  border-radius: 6px;
  font-size: 10px;
  font-weight: 500;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.08);
  transition: all 0.3s;
  white-space: nowrap;

  .state-dot {
    width: 5px;
    height: 5px;
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
  padding: 5px;
  color: $text-muted;

  &:hover {
    color: var(--axis-color);
  }
}

/* ========== 位置显示区 ========== */
.position-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.position-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 10px;
}

.position-value {
  display: flex;
  align-items: baseline;
  gap: 6px;
  min-width: 0;

  .number {
    font-size: clamp(24px, 2.6vw, 30px);
    font-weight: 700;
    color: var(--axis-color);
    font-family: 'SF Mono', Monaco, 'Cascadia Code', Consolas, 'Courier New', monospace;
    line-height: 1;
    text-shadow: 0 0 20px var(--axis-color-glow);
    transition: all 0.3s;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .unit {
    font-size: 12px;
    color: $text-muted;
    font-weight: 500;
    flex-shrink: 0;
  }
}

.home-btn-inline {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 10px;
  border-radius: 6px;
  border: 1px solid rgba($color-primary, 0.2);
  background: rgba($color-primary, 0.06);
  color: $color-primary;
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s;
  white-space: nowrap;
  margin-bottom: 3px;
  flex-shrink: 0;

  &:hover:not(:disabled) {
    background: rgba($color-primary, 0.12);
    border-color: rgba($color-primary, 0.35);
    box-shadow: 0 0 10px rgba($color-primary, 0.15);
  }

  &:disabled {
    opacity: 0.3;
    cursor: not-allowed;
  }
}

.position-track {
  display: flex;
  align-items: center;
  gap: 8px;

  .track-bg {
    position: relative;
    flex: 1;
    height: 5px;
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
    width: 12px;
    height: 12px;
    border-radius: 50%;
    border: 2px solid;
    background: var(--axis-color);
    transform: translate(-50%, -50%);
    box-shadow: 0 0 10px var(--axis-color-glow);
    transition: left 0.2s ease;
    z-index: 2;
  }

  .track-end {
    font-size: 9px;
    color: $text-muted;
    font-family: 'SF Mono', Monaco, 'Cascadia Code', Consolas, 'Courier New', monospace;
    white-space: nowrap;
  }
}

.limit-warning {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 5px 9px;
  border-radius: 6px;
  background: rgba($color-danger, 0.08);
  border: 1px solid rgba($color-danger, 0.2);
  color: $color-danger;
  font-size: 10px;
  font-weight: 500;
  animation: limit-warn-fade 2s ease-in-out infinite;
}

@keyframes limit-warn-fade {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.7; }
}

/* ========== 控制分区（目标 / 点动） ========== */
.control-block {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 8px 10px;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid rgba(255, 255, 255, 0.04);
  border-radius: 8px;
  transition: all 0.2s;

  &:hover {
    background: rgba(255, 255, 255, 0.04);
    border-color: rgba(255, 255, 255, 0.08);
  }
}

.block-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;

  .block-title {
    font-size: 11px;
    font-weight: 600;
    color: $text-secondary;
    letter-spacing: 0.3px;
  }

  .block-hint {
    font-size: 9px;
    color: $text-muted;
    opacity: 0.7;
  }
}

.block-body {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.block-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}

/* 输入框：占满整行 */
.target-input,
.step-input {
  width: 100% !important;

  :deep(.el-input__wrapper) {
    background: rgba(0, 0, 0, 0.3);
    box-shadow: inset 0 1px 3px rgba(0, 0, 0, 0.3);
    height: 30px;
  }

  :deep(.el-input__inner) {
    color: $text-primary;
    font-family: 'SF Mono', Monaco, 'Cascadia Code', Consolas, 'Courier New', monospace;
    font-weight: 500;
    font-size: 13px;
  }
}

/* ========== 按钮样式 ========== */
.run-btn,
.stop-btn,
.jog-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  height: 30px;
  border-radius: 7px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s;
  user-select: none;
  white-space: nowrap;
  flex: 1;

  &:active:not(:disabled) {
    transform: scale(0.97);
  }

  &:disabled {
    opacity: 0.3;
    cursor: not-allowed;
  }

  .el-icon {
    font-size: 13px;
  }
}

.run-btn {
  border: 1px solid rgba($color-success, 0.25);
  background: rgba($color-success, 0.12);
  color: $color-success;

  &:hover:not(:disabled) {
    background: rgba($color-success, 0.22);
    border-color: rgba($color-success, 0.4);
    box-shadow: 0 0 18px rgba($color-success, 0.2);
  }

  &.running {
    background: rgba($color-success, 0.22);
    border-color: rgba($color-success, 0.45);
    box-shadow: 0 0 22px rgba($color-success, 0.25);
  }
}

.stop-btn {
  border: 1px solid rgba($color-danger, 0.2);
  background: rgba($color-danger, 0.08);
  color: $color-danger;

  &:hover:not(:disabled) {
    background: rgba($color-danger, 0.16);
    border-color: rgba($color-danger, 0.35);
    box-shadow: 0 0 15px rgba($color-danger, 0.15);
  }
}

.jog-btn {
  border: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(255, 255, 255, 0.04);
  color: $text-secondary;

  &:hover:not(:disabled) {
    background: rgba(255, 255, 255, 0.08);
    border-color: var(--axis-color-glow);
    color: var(--axis-color);
  }

  &:active:not(:disabled) {
    background: var(--axis-color-dim);
  }
}

.jog-minus:hover:not(:disabled) {
  border-color: rgba($color-danger, 0.3);
  color: $color-danger;
  background: rgba($color-danger, 0.08);
}

.jog-plus:hover:not(:disabled) {
  border-color: rgba($color-success, 0.3);
  color: $color-success;
  background: rgba($color-success, 0.08);
}

.spin-icon {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

@media (max-width: 560px) {
  .axis-header {
    align-items: flex-start;
  }
}
</style>
