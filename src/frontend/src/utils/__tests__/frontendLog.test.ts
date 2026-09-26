import { describe, it, expect, vi, beforeEach } from 'vitest'

// vi.hoisted 确保 mock 变量在 vi.mock 工厂执行时已初始化
const { mockWriteFrontendError } = vi.hoisted(() => ({
  mockWriteFrontendError: vi.fn().mockResolvedValue(undefined),
}))

vi.mock('@bindings/yx-daq/internal/app', () => ({
  LogService: {
    WriteFrontendError: mockWriteFrontendError,
  },
}))

import { reportFrontendError, resetFrontendLogDedup } from '../frontendLog'

describe('utils/frontendLog', () => {
  beforeEach(() => {
    mockWriteFrontendError.mockClear()
    mockWriteFrontendError.mockResolvedValue(undefined)
    resetFrontendLogDedup()
  })

  it('上报错误到后端日志服务', async () => {
    reportFrontendError('error', 'boom', 'stack-trace')
    await vi.waitFor(() => expect(mockWriteFrontendError).toHaveBeenCalledTimes(1))
    expect(mockWriteFrontendError).toHaveBeenCalledWith('error', 'boom', 'stack-trace')
  })

  it('去重窗口内相同错误只上报一次', async () => {
    reportFrontendError('error', 'dup')
    reportFrontendError('error', 'dup')
    await vi.waitFor(() => expect(mockWriteFrontendError).toHaveBeenCalledTimes(1))
    await new Promise(resolve => setTimeout(resolve, 10))
    expect(mockWriteFrontendError).toHaveBeenCalledTimes(1)
  })

  it('不同级别或不同消息分别上报', async () => {
    reportFrontendError('error', 'same-message')
    reportFrontendError('warn', 'same-message')
    reportFrontendError('error', 'other-message')
    await vi.waitFor(() => expect(mockWriteFrontendError).toHaveBeenCalledTimes(3))
  })

  it('空消息与空堆栈不上报', async () => {
    reportFrontendError('error', '', '')
    await new Promise(resolve => setTimeout(resolve, 10))
    expect(mockWriteFrontendError).not.toHaveBeenCalled()
  })

  it('后端不可用时静默忽略（不抛出）', async () => {
    mockWriteFrontendError.mockRejectedValueOnce(new Error('wails down'))
    reportFrontendError('error', 'should-not-throw')
    await new Promise(resolve => setTimeout(resolve, 10))
    expect(mockWriteFrontendError).toHaveBeenCalledTimes(1)
  })
})
