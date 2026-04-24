<template>
  <span class="status-indicator" :class="[status, { pulse: animated }]">
    <span class="dot"></span>
    <span v-if="label" class="label">{{ label }}</span>
  </span>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
  status: 'connected' | 'disconnected' | 'error' | 'running' | 'warning'
  label?: string
  animated?: boolean
}>(), {
  animated: false,
})
</script>

<style lang="scss" scoped>
.status-indicator {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
}

.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  display: inline-block;
}

.connected .dot { background: #00ff88; box-shadow: 0 0 8px rgba(0,255,136,0.6); }
.disconnected .dot { background: #666; }
.error .dot { background: #ff3366; box-shadow: 0 0 8px rgba(255,51,102,0.6); }
.running .dot { background: #00f5ff; box-shadow: 0 0 8px rgba(0,245,255,0.6); }
.warning .dot { background: #ffaa00; box-shadow: 0 0 8px rgba(255,170,0,0.6); }

.label { color: rgba(255,255,255,0.7); }

.pulse .dot {
  animation: statusPulse 1.5s ease-in-out infinite;
}

@keyframes statusPulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.5; transform: scale(0.8); }
}
</style>
