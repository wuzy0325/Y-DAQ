<template>
  <span class="status-indicator" :class="[status, { pulse: animated }]">
    <span class="dot" />
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

.connected .dot { background: $color-success; box-shadow: 0 0 8px $color-success-glow; }
.disconnected .dot { background: rgba(255,255,255,0.3); }
.error .dot { background: $color-danger; box-shadow: 0 0 8px $color-danger-glow; }
.running .dot { background: $color-accent; box-shadow: 0 0 8px $color-accent-glow; }
.warning .dot { background: $color-warning; box-shadow: 0 0 8px $color-warning-glow; }

.label { color: $text-secondary; }

.pulse .dot {
  animation: statusPulse 1.5s ease-in-out infinite;
}

@keyframes statusPulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.5; transform: scale(0.8); }
}
</style>
