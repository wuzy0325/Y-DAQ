<template>
  <router-view />
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useDeviceStore } from './stores/device'
import { useMotionStore } from './stores/motion'
import { useCalibrationStore } from './stores/calibration'

onMounted(() => {
  const deviceStore = useDeviceStore()
  const motionStore = useMotionStore()
  const calibStore = useCalibrationStore()

  deviceStore.startListening()
  motionStore.startListening()
  calibStore.startListening()
})
</script>

<style>
#app {
  width: 100%;
  height: 100%;
}
</style>
