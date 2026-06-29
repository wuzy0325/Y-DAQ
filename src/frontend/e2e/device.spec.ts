import { test, expect } from './fixtures'

/**
 * 设备管理 E2E 测试。
 *
 * 覆盖功能：添加 / 编辑 / 连接 / 断开 / 删除 / 扫描 / 批量采集。
 * 每个测试独立运行（page load 重置 mock 状态），需要预置设备的测试通过 UI 添加。
 */

/** 通过 UI 添加一个设备（取消自动连接，避免 1s 等待），返回设备名 */
async function addDeviceViaUI(page: import('@playwright/test').Page, name: string, type = 'XY-DAQ16') {
  await page.getByRole('button', { name: '添加设备' }).click()
  await expect(page.locator('.el-dialog').filter({ hasText: '添加设备' })).toBeVisible()
  await page.locator('.device-dialog .el-input__inner').first().fill(name)
  await page.locator('.device-dialog .el-select').first().click()
  await page.locator('.el-select-dropdown__item', { hasText: type }).click()
  // 取消自动连接（el-switch 是 div，用 click 切换）
  const autoConnectSwitch = page.locator('.device-dialog .auto-connect-row .el-switch').first()
  if (await autoConnectSwitch.evaluate((el: HTMLElement) => el.classList.contains('is-checked'))) {
    await autoConnectSwitch.click()
  }
  await page.locator('.device-dialog').getByRole('button', { name: '确定' }).click()
  await expect(page.locator('.el-message--success')).toBeVisible({ timeout: 5000 })
  await expect(page.locator('.el-dialog').filter({ hasText: '添加设备' })).toBeHidden({ timeout: 5000 })
}

test.describe('设备管理', () => {
  test.beforeEach(async ({ page, mock }) => {
    await mock.goto('/device')
    await expect(page).toHaveURL(/#\/device/)
  })

  test('添加设备（XY-DAQ16）', async ({ page }) => {
    await page.getByRole('button', { name: '添加设备' }).click()
    await expect(page.locator('.el-dialog').filter({ hasText: '添加设备' })).toBeVisible()

    // 填写名称
    await page.locator('.device-dialog .el-input__inner').first().fill('测试设备A')

    // 类型默认 XY-DAQ16，确认 host/port 已自动填充
    await expect(page.locator('.device-dialog input[placeholder="192.168.3.101"]')).toHaveValue('192.168.3.101')

    // 取消自动连接
    const autoSwitch = page.locator('.device-dialog .auto-connect-row .el-switch').first()
    if (await autoSwitch.evaluate((el: HTMLElement) => el.classList.contains('is-checked'))) {
      await autoSwitch.click()
    }

    // 提交
    await page.locator('.device-dialog').getByRole('button', { name: '确定' }).click()

    // 验证：成功消息 + 设备出现在表格
    await expect(page.locator('.el-message--success')).toBeVisible()
    await expect(page.locator('.device-table')).toContainText('测试设备A')
    await expect(page.locator('.device-table')).toContainText('XY-DAQ16')
  })

  test('添加设备（模拟设备 SIMULATED）', async ({ page }) => {
    await page.getByRole('button', { name: '添加设备' }).click()
    await page.locator('.device-dialog .el-input__inner').first().fill('模拟设备')
    await page.locator('.device-dialog .el-select').first().click()
    await page.locator('.el-select-dropdown__item', { hasText: '模拟设备' }).click()
    // 模拟设备不显示网络配置
    await expect(page.locator('.device-dialog')).not.toContainText('网络配置')
    const simSwitch = page.locator('.device-dialog .auto-connect-row .el-switch').first()
    if (await simSwitch.evaluate((el: HTMLElement) => el.classList.contains('is-checked'))) {
      await simSwitch.click()
    }
    await page.locator('.device-dialog').getByRole('button', { name: '确定' }).click()
    await expect(page.locator('.device-table')).toContainText('模拟设备')
    await expect(page.locator('.device-table')).toContainText('SIMULATED')
  })

  test('扫描设备', async ({ page }) => {
    await page.getByRole('button', { name: '扫描设备' }).click()
    // mock 返回 2 个设备
    await expect(page.locator('.el-message--success')).toBeVisible()
    await expect(page.locator('.el-message--success')).toContainText('发现 2 个设备')
  })

  test('连接 / 断开设备', async ({ page }) => {
    await addDeviceViaUI(page, '连接测试设备')
    // 初始状态：未连接
    await expect(page.locator('.device-table')).toContainText('未连接')

    // 点击连接按钮（第二个按钮，primary 类型）
    const row = page.locator('.el-table__row').filter({ hasText: '连接测试设备' })
    await row.locator('button').nth(1).click()
    // connectDevice 内部有 1s setTimeout，等待"设备已连接"成功消息出现（标志 connectDevice 完成）
    await expect(page.locator('.el-message--success').filter({ hasText: '设备已连接' })).toBeVisible({ timeout: 5000 })

    // 连接完成后，断开按钮才可用（disconnectDevice 会在 connectingIds 仍含 id 时直接返回）
    await row.locator('button').nth(1).click()
    await expect(page.locator('.el-message--success').filter({ hasText: '设备已断开' })).toBeVisible({ timeout: 5000 })
    await expect(page.locator('.device-table')).toContainText('未连接', { timeout: 5000 })
  })

  test('编辑设备配置', async ({ page }) => {
    await addDeviceViaUI(page, '编辑前设备')
    const row = page.locator('.el-table__row').filter({ hasText: '编辑前设备' })

    // 点击编辑按钮（第一个按钮）
    await row.locator('button').nth(0).click()
    const editDialog = page.locator('.el-dialog').filter({ hasText: '编辑设备' })
    await expect(editDialog).toBeVisible()

    // 修改设备名（必须用 :visible 限定，避免选中隐藏的添加设备弹窗中的 input）
    await editDialog.locator('.el-input__inner').first().fill('编辑后设备')
    // 保存
    await editDialog.getByRole('button', { name: '保存' }).click()

    // 用文本过滤避免与 addDeviceViaUI 残留的"添加成功"消息冲突（strict mode violation）
    await expect(page.locator('.el-message--success').filter({ hasText: '设备配置已更新' })).toBeVisible()
    await expect(page.locator('.device-table')).toContainText('编辑后设备')
  })

  test('删除设备', async ({ page }) => {
    await addDeviceViaUI(page, '待删除设备')
    await expect(page.locator('.device-table')).toContainText('待删除设备')

    const row = page.locator('.el-table__row').filter({ hasText: '待删除设备' })
    // 点击删除按钮（第三个按钮，danger 类型）— DeviceView 直接删除，无二次确认弹窗
    await row.locator('button').nth(2).click()

    await expect(page.locator('.el-message--success').filter({ hasText: '设备已删除' })).toBeVisible()
    await expect(page.locator('.device-table')).not.toContainText('待删除设备')
  })

  test('采集状态显示（采集中 / 空闲）', async ({ page, mock }) => {
    await addDeviceViaUI(page, '采集设备')
    // 连接设备
    const row = page.locator('.el-table__row').filter({ hasText: '采集设备' })
    await row.locator('button').nth(1).click()
    await expect(page.locator('.device-table')).toContainText('已连接', { timeout: 5000 })

    // 初始：采集列显示空闲（--）
    await expect(row).toContainText('--')

    // 模拟后端推送 acquiring=true 的状态更新
    const statuses = await mock.getState<any[]>('deviceStatuses')
    const updated = statuses.map(s => ({ ...s, acquiring: s.name === '采集设备' }))
    await mock.emitEvent('device:status-updated', updated)

    // 采集列应显示"采集中"
    await expect(row).toContainText('采集中', { timeout: 5000 })
  })
})
