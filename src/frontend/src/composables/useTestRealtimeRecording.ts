import { ref } from 'vue'

export interface UseTestRealtimeRecordingOptions {
  // 启动录制，返回文件路径（用户取消时返回 undefined 或空字符串）
  startRecording: () => Promise<string | undefined | null>
  // 停止录制
  stopRecording: () => Promise<void>
}

/**
 * 实时录制控制 composable
 * 三孔/五孔测试视图共享的「实时保存」按钮交互逻辑。
 */
export function useTestRealtimeRecording(options: UseTestRealtimeRecordingOptions) {
  const isRecording = ref(false)

  async function handleStartRecording() {
    try {
      const filePath = await options.startRecording()
      if (!filePath) return // 用户取消
      isRecording.value = true
      ElMessage.success(`已开始保存: ${filePath}`)
    } catch (e: any) {
      ElMessage.error(`开始保存失败: ${e?.message || e}`)
    }
  }

  async function handleStopRecording() {
    try {
      await options.stopRecording()
      isRecording.value = false
      ElMessage.success('已停止保存')
    } catch (e: any) {
      ElMessage.error(`停止保存失败: ${e?.message || e}`)
    }
  }

  return { isRecording, handleStartRecording, handleStopRecording }
}
