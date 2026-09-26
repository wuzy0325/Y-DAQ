import { LogService } from '@bindings/yx-daq/internal/app'

/**
 * 前端错误上报：将未捕获异常 / Vue 组件错误写入后端日志系统（source=frontend），
 * 与设备通信日志统一落盘，便于排查前后端问题。
 * 相同错误在去重窗口内只上报一次，避免错误风暴刷爆日志。
 */
const DEDUP_WINDOW_MS = 5000
const MAX_DEDUP_ENTRIES = 200

export type FrontendLogLevel = 'debug' | 'info' | 'warn' | 'error'

const reported = new Map<string, number>()

/** 上报一条前端错误（异步执行，不阻塞调用方） */
export function reportFrontendError(level: FrontendLogLevel, message: string, stack = ''): void {
  if (!message && !stack) return

  const now = Date.now()
  const signature = `${level}|${message}`
  const last = reported.get(signature)
  if (last !== undefined && now - last < DEDUP_WINDOW_MS) return
  reported.set(signature, now)

  if (reported.size > MAX_DEDUP_ENTRIES) {
    for (const [key, ts] of reported) {
      if (now - ts > DEDUP_WINDOW_MS) reported.delete(key)
    }
  }

  void send(level, message, stack)
}

async function send(level: FrontendLogLevel, message: string, stack: string): Promise<void> {
  try {
    await LogService.WriteFrontendError(level, message.slice(0, 2000), stack.slice(0, 4000))
  } catch {
    // Wails 未就绪（启动早期）或日志服务不可用时静默忽略，避免上报本身产生错误循环
  }
}

/** 重置去重状态（测试用） */
export function resetFrontendLogDedup(): void {
  reported.clear()
}
