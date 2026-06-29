/**
 * E2E 测试用 @wailsio/runtime mock。
 *
 * 真实运行时：bindings 通过 Call.ByID(id, ...args) 调用 Go 后端方法；
 * 事件通过 Events.On/Off 订阅由 Go 端 emit 的广播。
 *
 * Mock 策略：
 * - Call.ByID 不分发到后端，直接返回 resolved promise（实际逻辑由各 service mock 实现，
 *   service mock 自身不调用 Call.ByID，所以此函数仅在意外路径下作为兜底）。
 * - Events 维护内存订阅表，测试通过 __emitEvent 模拟后端推送。
 * - Create.Array / Create.Any 直接透传，因为 mock service 已返回正确类型。
 */
type EventHandler = (event: { data: any }) => void

const subscribers = new Map<string, Set<EventHandler>>()

export const Events = {
  On(channel: string, handler: EventHandler): void {
    if (!subscribers.has(channel)) subscribers.set(channel, new Set())
    subscribers.get(channel)!.add(handler)
  },
  Off(channel: string): void {
    subscribers.delete(channel)
  },
}

/** 测试 helper：模拟后端在 channel 上广播事件（回调签名与真实 Wails v3 一致：{ data }） */
export function __emitEvent(channel: string, data: any): void {
  const handlers = subscribers.get(channel)
  if (!handlers) return
  for (const h of handlers) {
    try {
      h({ data })
    } catch (e) {
      console.error(`[mock] event handler error on ${channel}:`, e)
    }
  }
}

/** 测试 helper：清除所有事件订阅（每个测试 reset 时调用） */
export function __clearEventSubscriptions(): void {
  subscribers.clear()
}

export const Call = {
  ByID(_id: number, ..._args: any[]): Promise<any> {
    // service mock 应直接实现逻辑而非走 Call.ByID；这里仅作兜底
    return Promise.resolve(undefined)
  },
}

// CancellablePromise 在类型上是 Promise 的子类；mock 用普通 Promise
export const CancellablePromise = Promise as any

export const Create = {
  Array: (_fn: any) => (value: any) => value,
  Any: (value: any) => value,
}

// ==================== 测试桥接 ====================
// 将 helper 暴露到 window.__mock，供 Playwright 测试通过 page.evaluate 调用。
// 必须在模块加载时同步设置（应用 import mock 后立即可用）。
;(globalThis as any).__mock = {
  ...(globalThis as any).__mock,
  emitEvent: __emitEvent,
  clearEventSubscriptions: __clearEventSubscriptions,
}
