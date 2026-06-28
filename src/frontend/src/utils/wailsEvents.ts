import { Events } from '@wailsio/runtime'

type EventHandler = (event: any) => void

interface EventSubscription {
  channel: string
  handler: EventHandler
}

/**
 * 创建一个 Wails 事件监听器，封装 Events.On/Off + try/catch + listening 标志位管理。
 *
 * 用于 store 中重复出现的 startListening/stopListening 模式：
 * - 多数 store 在事件注册成功后设置 listening=true，stopListening 仅在 listening 时才注销
 * - 部分失败时不回滚已注册的通道（与原手写实现一致，避免双重注销复杂度）
 *
 * @param name 用于日志识别的监听器名称（如 'motion'、'device'）
 * @param subscriptions 通道与回调列表；通道名在创建时固定，不支持动态 pid 拼接
 * @returns { start, stop, isListening } — store 可直接暴露或包装调用
 */
export function createWailsEventListener(name: string, subscriptions: EventSubscription[]) {
  let listening = false
  return {
    isListening: () => listening,
    start() {
      if (listening) return
      try {
        for (const { channel, handler } of subscriptions) {
          Events.On(channel, handler)
        }
        listening = true
      } catch (e) {
        console.warn(`${name} startListening failed:`, e)
      }
    },
    stop() {
      if (!listening) return
      try {
        for (const { channel } of subscriptions) {
          Events.Off(channel)
        }
        listening = false
      } catch (e) {
        console.warn(`${name} stopListening failed:`, e)
      }
    },
  }
}
