import { test, expect } from './fixtures'

/**
 * 冒烟测试：验证 E2E 基础设施可用。
 * - dev server 在 E2E=true 下启动，mock 别名生效
 * - 应用可加载，window.__mock 就绪
 * - 顶部导航栏可点击切换路由
 */
test('应用加载 + 导航栏切换', async ({ page, mock }) => {
  await mock.goto('/')
  // 顶部 logo 可见
  await expect(page.locator('.logo-text')).toHaveText('YX-DAQ')
  // 默认在仪表盘
  await expect(page.locator('.nav-item.active .nav-label')).toHaveText('仪表盘')

  // 点击"设备管理"
  await page.getByRole('link', { name: '设备管理' }).click()
  await expect(page).toHaveURL(/#\/device/)
  await expect(page.locator('.nav-item.active .nav-label')).toHaveText('设备管理')

  // 点击"设置"
  await page.getByRole('link', { name: '设置' }).click()
  await expect(page).toHaveURL(/#\/settings/)
  await expect(page.locator('.nav-item.active .nav-label')).toHaveText('设置')

  // 点击"五孔插值移位测试"（router-link，用文本定位更稳定）
  await page.locator('.nav-item', { hasText: '五孔插值移位测试' }).click()
  await expect(page).toHaveURL(/#\/five-hole-test/)
})
