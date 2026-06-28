<template>
  <router-view />
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useCalibrationStore } from './stores/calibration'
import { useDeviceStore } from './stores/device'
import { useMotionStore } from './stores/motion'

onMounted(() => {
  const deviceStore = useDeviceStore()
  const motionStore = useMotionStore()
  const calibStore = useCalibrationStore()

  deviceStore.startListening()
  motionStore.startListening()
  calibStore.startListening()
})
</script>

<style lang="scss">
#app {
  width: 100%;
  height: 100%;
}
</style>
