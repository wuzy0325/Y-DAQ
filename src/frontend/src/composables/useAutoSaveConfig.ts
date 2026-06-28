import { ref, watch, onUnmounted, type Ref } from 'vue'

export interface UseAutoSaveConfigOptions<TConfig> {
  // 响应式配置对象，深度变更时触发防抖保存
  config: Ref<TConfig>
  // 是否正在运行测试（运行中跳过实时监控重启）
  isRunning: Ref<boolean>
  // 保存配置到 localStorage / 后端
  saveConfig: () => void | Promise<void>
  // 停止实时监控（用于重启监控以应用最新配置）
  stopRealtimeMonitor: () => void | Promise<void>
  // 启动实时监控
  startRealtimeMonitor: () => void | Promise<void>
  // 防抖延迟（ms），默认 500
  debounceMs?: number
}

/**
 * 配置自动保存 composable
 * 三孔/五孔测试视图共享的配置防抖保存 + 实时监控重启逻辑。
 *
 * 使用：
 *   const { isInitializing, markInitialized } = useAutoSaveConfig({...})
 *   onMounted(async () => { ...; markInitialized() })
 *
 * 注意：composable 内部已注册 onUnmounted 清理防抖定时器，
 *       调用方无需在 onUnmounted 中重复清理 configSaveTimer。
 */
export function useAutoSaveConfig<TConfig>(options: UseAutoSaveConfigOptions<TConfig>) {
  const isInitializing = ref(true)
  let configSaveTimer: number | null = null
  const debounceMs = options.debounceMs ?? 500

  watch(
    () => options.config.value,
    () => {
      if (isInitializing.value) return
      if (configSaveTimer) clearTimeout(configSaveTimer)
      configSaveTimer = window.setTimeout(() => {
        options.saveConfig()
        // 重启实时监控以应用最新配置（通道映射等）
        if (!options.isRunning.value) {
          options.stopRealtimeMonitor()
          options.startRealtimeMonitor()
        }
      }, debounceMs)
    },
    { deep: true },
  )

  function markInitialized() {
    isInitializing.value = false
  }

  onUnmounted(() => {
    if (configSaveTimer) {
      clearTimeout(configSaveTimer)
      configSaveTimer = null
    }
  })

  return { isInitializing, markInitialized }
}
