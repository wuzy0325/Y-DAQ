<template>
  <div class="value-display">
    <div class="value" :style="{ color: color, minWidth: minWidth }">{{ formattedValue }}</div>
    <div v-if="unit" class="unit">{{ unit }}</div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { COLOR_ACCENT } from '../constants/colors'

const props = withDefaults(defineProps<{
  value?: number
  precision?: number
  unit?: string
  color?: string
}>(), {
  precision: 3,
  color: COLOR_ACCENT,
})

const formattedValue = computed(() => {
  if (typeof props.value !== 'number' || isNaN(props.value)) return '--'
  return props.value.toFixed(props.precision)
})

// 根据精度计算最小宽度，避免数值变化时宽度跳变
// 格式: 整数部分 + 小数点 + 小数部分 + 符号位，如 "-12345.678" = 9ch
const minWidth = computed(() => {
  // 无效值时不预留固定宽度，避免 "--" 把单位推得太远
  if (typeof props.value !== 'number' || isNaN(props.value)) return 'auto'
  // precision=3 最多如 "-12345.678" ≈ 9ch，给足空间
  return `${props.precision + 6}ch`
})
</script>

<style lang="scss" scoped>
.value-display {
  display: flex;
  align-items: baseline;
  gap: 4px;
}

.value {
  font-size: 20px;
  font-family: 'JetBrains Mono', 'Fira Code', Consolas, monospace;
  font-weight: 600;
  line-height: 1;
  font-variant-numeric: tabular-nums;
  text-align: right;
}

.unit {
  font-size: 11px;
  color: rgba(255,255,255,0.4);
}
</style>
