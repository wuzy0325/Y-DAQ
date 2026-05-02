<template>
  <div ref="chartRef" class="chart-container" :style="{ width, height }" />
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, shallowRef } from 'vue'
import * as echarts from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart } from 'echarts/charts'
import {
  TitleComponent,
  TooltipComponent,
  GridComponent,
  DataZoomComponent,
  LegendComponent,
} from 'echarts/components'

echarts.use([
  CanvasRenderer,
  LineChart,
  TitleComponent,
  TooltipComponent,
  GridComponent,
  DataZoomComponent,
  LegendComponent,
])

const props = withDefaults(defineProps<{
  option: Record<string, any>
  width?: string
  height?: string
}>(), {
  width: '100%',
  height: '300px',
})

const chartRef = ref<HTMLDivElement>()
const chartInstance = shallowRef<echarts.ECharts>()

onMounted(() => {
  if (chartRef.value) {
    chartInstance.value = echarts.init(chartRef.value, 'dark', {
      renderer: 'canvas',
    })
    chartInstance.value.setOption(props.option)

    const resizeObserver = new ResizeObserver(() => {
      chartInstance.value?.resize()
    })
    resizeObserver.observe(chartRef.value)

    onUnmounted(() => {
      resizeObserver.disconnect()
      chartInstance.value?.dispose()
    })
  }
})

watch(() => props.option, (newOption) => {
  if (chartInstance.value) {
    chartInstance.value.setOption(newOption)
  }
})

defineExpose({
  getChart: () => chartInstance.value,
})
</script>

<style lang="scss" scoped>
.chart-container {
  min-height: 0;
}
</style>
