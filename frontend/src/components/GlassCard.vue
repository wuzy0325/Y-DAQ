<template>
  <div class="glass-card" :class="{ elevated: elevated }">
    <div v-if="title" class="card-header">
      <span v-if="icon" class="card-icon">{{ icon }}</span>
      <span class="card-title">{{ title }}</span>
      <div class="card-actions">
        <slot name="actions" />
      </div>
    </div>
    <div class="card-body">
      <slot />
    </div>
  </div>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
  title?: string
  icon?: string
  elevated?: boolean
}>(), {
  elevated: false,
})
</script>

<style lang="scss" scoped>
.glass-card {
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 12px;
  padding: 16px;
  backdrop-filter: blur(16px);
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4), inset 0 1px 0 rgba(255, 255, 255, 0.1);
  transition: all 250ms cubic-bezier(0.4, 0, 0.2, 1);

  &.elevated {
    background: rgba(255, 255, 255, 0.08);
    box-shadow: 0 12px 48px rgba(0, 0, 0, 0.5), inset 0 1px 0 rgba(255, 255, 255, 0.15);
  }

  &:hover {
    border-color: rgba(184, 41, 255, 0.2);
  }
}

.card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
  padding-bottom: 8px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}

.card-icon { font-size: 18px; }
.card-title { font-size: 14px; font-weight: 600; color: rgba(255,255,255,0.9); flex: 1; }
.card-actions { display: flex; gap: 8px; }
</style>
