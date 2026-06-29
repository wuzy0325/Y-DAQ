/**
 * E2E 测试 fixtures。
 *
 * - test 前自动 goto baseURL，保证 mock 状态干净（每次 page load 模块状态重置）
 * - 等待 window.__mock 就绪（标志 mock 模块已加载）
 * - 提供 mock 辅助方法：emitEvent / setState / getCalls
 *
 * 用法：
 *   import { test, expect } from './fixtures'
 *   test('xxx', async ({ page, mock }) => {
 *     await mock.goto('/device')
 *     await mock.emitEvent('device:status-updated', [...])
 *   })
 */
import { test as base, expect, type Page } from '@playwright/test'

interface MockHelpers {
  /** 导航到指定 hash 路由路径并等待视图渲染 */
  goto: (path?: string) => Promise<void>
  /** 模拟后端在 channel 上广播事件（与真实 Wails v3 事件签名一致：回调收到 { data }） */
  emitEvent: (channel: string, data: any) => Promise<void>
  /** 直接修改浏览器端 mockState（合并） */
  setState: (patch: Record<string, any>) => Promise<void>
  /** 读取 mockState 的指定字段 */
  getState: <T = any>(key: string) => Promise<T>
  /** 读取方法调用日志 */
  getCalls: () => Promise<{ method: string; args: any[] }[]>
  /** 等待 mock 模块就绪（window.__mock 存在） */
  waitForMockReady: () => Promise<void>
}

async function waitForMockReady(page: Page): Promise<void> {
  await page.waitForFunction(() => !!(window as any).__mock, { timeout: 10_000 })
}

async function gotoRoute(page: Page, path?: string): Promise<void> {
  const target = path ? `/#${path}` : '/'
  await page.goto(target)
  await waitForMockReady(page)
}

export const test = base.extend<{ mock: MockHelpers }>({
  mock: async ({ page }, use) => {
    const mock: MockHelpers = {
      goto: (path?: string) => gotoRoute(page, path),
      emitEvent: async (channel: string, data: any) => {
        await page.evaluate(
          ([ch, d]) => (window as any).__mock.emitEvent(ch, d),
          [channel, data],
        )
      },
      setState: async (patch: Record<string, any>) => {
        await page.evaluate(
          (p) => Object.assign((window as any).__mock.state, p),
          patch,
        )
      },
      getState: async (key: string) => {
        return page.evaluate((k) => (window as any).__mock.state[k], key)
      },
      getCalls: async () => {
        return page.evaluate(() => (window as any).__mock.state.calls)
      },
      waitForMockReady: () => waitForMockReady(page),
    }
    await use(mock)
  },
})

export { expect }
