<template>
  <div class="value-display">
    <div class="value" :style="{ color: color }">{{ formattedValue }}</div>
    <div v-if="unit" class="unit">{{ unit }}</div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  value?: number
  precision?: number
  unit?: string
  color?: string
}>(), {
  precision: 3,
  color: '#00f5ff',
})

const formattedValue = computed(() => {
  if (typeof props.value !== 'number' || isNaN(props.value)) return '--'
  return props.value.toFixed(props.precision)
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
}

.unit {
  font-size: 11px;
  color: rgba(255,255,255,0.4);
}
</style>
