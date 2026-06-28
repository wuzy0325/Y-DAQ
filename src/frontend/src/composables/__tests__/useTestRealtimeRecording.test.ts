import { describe, it, expect, vi, beforeEach } from 'vitest'
import { effectScope } from 'vue'

// auto-import 将 ElMessage 解析为 from 'element-plus/es'（见 auto-imports.d.ts）
// Mock 该路径，避免加载真实模块时拉入 theme-chalk/base.css
// （vitest 默认不处理 .css 导入，会报 "Unknown file extension .css"）
vi.mock('element-plus/es', () => ({
  ElMessage: {
    success: vi.fn(),
    error: vi.fn(),
    warning: vi.fn(),
    info: vi.fn(),
  },
}))

import { ElMessage } from 'element-plus/es'
import { useTestRealtimeRecording } from '../useTestRealtimeRecording'

// 导入的 ElMessage 类型来自真实 element-plus（无 mockClear 等 mock 方法）
// 此处断言为 mock 形状以便测试中访问 mock 状态
const mockElMessage = ElMessage as unknown as {
  success: ReturnType<typeof vi.fn>
  error: ReturnType<typeof vi.fn>
  warning: ReturnType<typeof vi.fn>
  info: ReturnType<typeof vi.fn>
}

describe('composables/useTestRealtimeRecording', () => {
  beforeEach(() => {
    mockElMessage.success.mockClear()
    mockElMessage.error.mockClear()
    mockElMessage.warning.mockClear()
    mockElMessage.info.mockClear()
  })

  function setup(options: Partial<{
    startRecording: ReturnType<typeof vi.fn>
    stopRecording: ReturnType<typeof vi.fn>
  }> = {}) {
    const startRecording = options.startRecording ?? vi.fn().mockResolvedValue('/path/to/file.csv')
    const stopRecording = options.stopRecording ?? vi.fn().mockResolvedValue(undefined)

    const scope = effectScope()
    const result = scope.run(() => useTestRealtimeRecording({
      // MockInstance 可调用，但 TS 不识别其与函数签名兼容，用 any 桥接
      startRecording: startRecording as any,
      stopRecording: stopRecording as any,
    }))!

    return { ...result, startRecording, stopRecording, scope }
  }

  it('初始 isRecording=false', () => {
    const { isRecording } = setup()
    expect(isRecording.value).toBe(false)
  })

  describe('handleStartRecording', () => {
    it('成功：startRecording 返回文件路径 → isRecording=true + ElMessage.success', async () => {
      const { isRecording, handleStartRecording } = setup({
        startRecording: vi.fn().mockResolvedValue('/tmp/record.csv'),
      })
      await handleStartRecording()
      expect(isRecording.value).toBe(true)
      expect(mockElMessage.success).toHaveBeenCalledWith('已开始保存: /tmp/record.csv')
    })

    it('用户取消（返回 undefined）：isRecording 保持 false + 无 success 提示', async () => {
      const { isRecording, handleStartRecording } = setup({
        startRecording: vi.fn().mockResolvedValue(undefined),
      })
      await handleStartRecording()
      expect(isRecording.value).toBe(false)
      expect(mockElMessage.success).not.toHaveBeenCalled()
    })

    it('用户取消（返回空字符串）：isRecording 保持 false', async () => {
      const { isRecording, handleStartRecording } = setup({
        startRecording: vi.fn().mockResolvedValue(''),
      })
      await handleStartRecording()
      expect(isRecording.value).toBe(false)
      expect(mockElMessage.success).not.toHaveBeenCalled()
    })

    it('用户取消（返回 null）：isRecording 保持 false', async () => {
      const { isRecording, handleStartRecording } = setup({
        startRecording: vi.fn().mockResolvedValue(null),
      })
      await handleStartRecording()
      expect(isRecording.value).toBe(false)
    })

    it('startRecording 抛错：isRecording 保持 false + ElMessage.error', async () => {
      const { isRecording, handleStartRecording } = setup({
        startRecording: vi.fn().mockRejectedValue(new Error('设备未连接')),
      })
      await handleStartRecording()
      expect(isRecording.value).toBe(false)
      expect(mockElMessage.error).toHaveBeenCalledWith('开始保存失败: 设备未连接')
    })

    it('startRecording 抛非 Error 对象：错误信息为对象本身', async () => {
      const { handleStartRecording } = setup({
        startRecording: vi.fn().mockRejectedValue('string error'),
      })
      await handleStartRecording()
      expect(mockElMessage.error).toHaveBeenCalledWith('开始保存失败: string error')
    })
  })

  describe('handleStopRecording', () => {
    it('成功：stopRecording 解析 → isRecording=false + ElMessage.success', async () => {
      const { isRecording, handleStartRecording, handleStopRecording } = setup({
        startRecording: vi.fn().mockResolvedValue('/tmp/record.csv'),
      })
      await handleStartRecording()
      expect(isRecording.value).toBe(true)

      await handleStopRecording()
      expect(isRecording.value).toBe(false)
      expect(mockElMessage.success).toHaveBeenCalledWith('已停止保存')
    })

    it('stopRecording 抛错：isRecording 保持 true + ElMessage.error', async () => {
      const { isRecording, handleStartRecording, handleStopRecording } = setup({
        startRecording: vi.fn().mockResolvedValue('/tmp/record.csv'),
        stopRecording: vi.fn().mockRejectedValue(new Error('写入失败')),
      })
      await handleStartRecording()
      await handleStopRecording()
      expect(isRecording.value).toBe(true) // 失败时保持原状态
      expect(mockElMessage.error).toHaveBeenCalledWith('停止保存失败: 写入失败')
    })
  })
})
