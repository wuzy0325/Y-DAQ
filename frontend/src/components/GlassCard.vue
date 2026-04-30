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
  background: $glass-bg;
  border: 1px solid $glass-border;
  border-radius: $border-radius-md;
  padding: $spacing-lg;
  backdrop-filter: $blur-md;
  box-shadow: $shadow-glass;
  transition: all $transition-base;

  &.elevated {
    background: $glass-bg-elevated;
    box-shadow: $shadow-glass-hover;
  }

  &:hover {
    border-color: rgba($color-primary, 0.2);
  }
}

.card-header {
  display: flex;
  align-items: center;
  gap: $spacing-sm;
  margin-bottom: $spacing-md;
  padding-bottom: $spacing-sm;
  border-bottom: 1px solid $glass-border-light;
}

.card-icon { font-size: $font-size-xl; }
.card-title { font-size: $font-size-md; font-weight: 600; color: $text-secondary; flex: 1; }
.card-actions { display: flex; gap: $spacing-sm; }
</style>
