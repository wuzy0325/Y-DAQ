import { test, expect } from './fixtures'

/**
 * 运动控制器 E2E 测试。
 *
 * 覆盖功能：添加 / 编辑 / 连接 / 断开 / 删除 / 轴移动 / 轴回零 / 急停全部。
 * MotionView 使用卡片列表（非表格），删除使用自定义弹窗（非 ElMessageBox）。
 */

/** 通过 UI 添加一个 B140 控制器 */
async function addControllerViaUI(page: import('@playwright/test').Page, name: string, type = 'B140 运动控制器') {
  await page.locator('.controller-sidebar').getByRole('button', { name: '添加', exact: true }).click()
  const dialog = page.locator('.el-dialog').filter({ hasText: '添加控制器' })
  await expect(dialog).toBeVisible()
  await dialog.locator('.el-input__inner').first().fill(name)
  // 选择类型
  await dialog.locator('.el-select').first().click()
  await page.locator('.el-select-dropdown__item', { hasText: type }).click()
  // 确定
  await dialog.getByRole('button', { name: '确定' }).click()
  // 用 .last() 避免多次添加时残留的成功消息导致 strict mode violation
  await expect(page.locator('.el-message--success').filter({ hasText: '控制器添加成功' }).last()).toBeVisible({ timeout: 5000 })
  await expect(dialog).toBeHidden({ timeout: 5000 })
}

/** 获取指定名称的控制器卡片 */
function controllerCard(page: import('@playwright/test').Page, name: string) {
  return page.locator('.controller-card-item').filter({ hasText: name })
}

test.describe('运动控制器', () => {
  test.beforeEach(async ({ page, mock }) => {
    await mock.goto('/motion')
    await expect(page).toHaveURL(/#\/motion/)
  })

  test('添加控制器（B140）', async ({ page }) => {
    await addControllerViaUI(page, '测试控制器A')
    // 验证卡片出现
    const card = controllerCard(page, '测试控制器A')
    await expect(card).toBeVisible()
    await expect(card).toContainText('B140')
    await expect(card).toContainText('未连接')
  })

  test('添加控制器（模拟控制器）', async ({ page }) => {
    await page.locator('.controller-sidebar').getByRole('button', { name: '添加', exact: true }).click()
    const dialog = page.locator('.el-dialog').filter({ hasText: '添加控制器' })
    await expect(dialog).toBeVisible()
    await dialog.locator('.el-input__inner').first().fill('模拟控制器')
    await dialog.locator('.el-select').first().click()
    await page.locator('.el-select-dropdown__item', { hasText: '模拟控制器' }).click()
    // 模拟控制器禁用地址/端口
    await expect(dialog.locator('input[placeholder="192.168.1.101"]')).toBeDisabled()
    await dialog.getByRole('button', { name: '确定' }).click()
    await expect(page.locator('.el-message--success').filter({ hasText: '控制器添加成功' })).toBeVisible({ timeout: 5000 })
    await expect(controllerCard(page, '模拟控制器')).toBeVisible()
    await expect(controllerCard(page, '模拟控制器')).toContainText('模拟')
  })

  test('连接 / 断开控制器', async ({ page }) => {
    await addControllerViaUI(page, '连接测试')
    const card = controllerCard(page, '连接测试')
    await expect(card).toContainText('未连接')

    // 点击连接按钮（第二个按钮）
    await card.locator('.card-actions button').nth(1).click()
    await expect(card).toContainText('已连接', { timeout: 5000 })

    // 断开
    await card.locator('.card-actions button').nth(1).click()
    await expect(card).toContainText('未连接', { timeout: 5000 })
  })

  test('编辑控制器配置', async ({ page }) => {
    await addControllerViaUI(page, '编辑前')
    const card = controllerCard(page, '编辑前')

    // 点击编辑按钮（第一个按钮）
    await card.locator('.card-actions button').nth(0).click()
    const editDialog = page.locator('.el-dialog').filter({ hasText: '编辑控制器' })
    await expect(editDialog).toBeVisible()

    // 修改名称
    await editDialog.locator('.el-input__inner').first().fill('编辑后')
    await editDialog.getByRole('button', { name: '保存' }).click()

    await expect(page.locator('.el-message--success').filter({ hasText: '控制器配置已更新' })).toBeVisible({ timeout: 5000 })
    await expect(controllerCard(page, '编辑后')).toBeVisible()
  })

  test('删除控制器', async ({ page }) => {
    await addControllerViaUI(page, '待删除')
    const card = controllerCard(page, '待删除')
    await expect(card).toBeVisible()

    // 点击删除按钮（第三个按钮，danger 类型）
    await card.locator('.card-actions button').nth(2).click()

    // 自定义删除确认弹窗（非 ElMessageBox）
    const deleteDialog = page.locator('.el-dialog').filter({ hasText: '删除控制器' })
    await expect(deleteDialog).toBeVisible()
    // 点击"删除"按钮（自定义 HTML button，class=confirm-delete）
    await deleteDialog.locator('button.confirm-delete').click()

    await expect(page.locator('.el-message--success').filter({ hasText: '控制器已删除' })).toBeVisible({ timeout: 5000 })
    await expect(card).toBeHidden({ timeout: 5000 })
  })

  test('轴移动（运行 / 停止）', async ({ page }) => {
    await addControllerViaUI(page, '轴移动测试')
    const card = controllerCard(page, '轴移动测试')
    // 选中控制器（点击卡片本身，非操作按钮）
    await card.locator('.card-header, .controller-name, .card-info').first().click()

    // 连接控制器
    await card.locator('.card-actions button').nth(1).click()
    await expect(card).toContainText('已连接', { timeout: 5000 })

    // 等待轴控制卡片渲染
    const xAxisCard = page.locator('.axis-card').filter({ hasText: 'X轴' }).first()
    await expect(xAxisCard).toBeVisible({ timeout: 5000 })

    // 设置目标位置（el-input-number 需 Enter 提交 v-model）
    const targetInput = xAxisCard.locator('.target-input input')
    await targetInput.fill('10.5')
    await targetInput.press('Enter')

    // 点击运行按钮（axis-card 内有 pointer-events 拦截，用 dispatchEvent 直接触发 click）
    await xAxisCard.locator('.run-btn').dispatchEvent('click')

    // mock 在 100ms 后更新位置，等待位置更新
    await expect(xAxisCard.locator('.position-value .number')).toContainText('10.5', { timeout: 3000 })
  })

  test('轴回零（置零）', async ({ page, mock }) => {
    await addControllerViaUI(page, '回零测试')
    const card = controllerCard(page, '回零测试')
    await card.locator('.card-header, .controller-name, .card-info').first().click()
    await card.locator('.card-actions button').nth(1).click()
    await expect(card).toContainText('已连接', { timeout: 5000 })

    const xAxisCard = page.locator('.axis-card').filter({ hasText: 'X轴' }).first()
    await expect(xAxisCard).toBeVisible({ timeout: 5000 })

    // 先通过 mock 设置一个非零位置（axes 为数组，按 name 索引）
    await mock.emitEvent('motion:status-updated', (await mock.getState<any[]>('motionStatuses')).map(s => ({
      ...s,
      axes: s.axes.map((ax: any) => ax.name === 'X' ? { ...ax, position: 5 } : ax),
    })))
    await expect(xAxisCard.locator('.position-value .number')).toContainText('5.00', { timeout: 3000 })

    // 点击置零按钮（.home-btn-inline 调用 definePosition(name, 0)）
    await xAxisCard.locator('.home-btn-inline').click({ force: true })
    // 位置应变为 0
    await expect(xAxisCard.locator('.position-value .number')).toContainText('0.00', { timeout: 3000 })
  })

  test('急停全部', async ({ page }) => {
    await addControllerViaUI(page, '控制器1')
    await addControllerViaUI(page, '控制器2')

    // 连接两个控制器
    for (const name of ['控制器1', '控制器2']) {
      const card = controllerCard(page, name)
      await card.locator('.card-actions button').nth(1).click()
      await expect(card).toContainText('已连接', { timeout: 5000 })
    }

    // 急停全部按钮在 sidebar 底部
    const estopBtn = page.locator('.estop-btn')
    await expect(estopBtn).toBeEnabled()
    await estopBtn.click()

    // 验证所有控制器仍为已连接（急停只停止运动，不断开连接）
    await expect(controllerCard(page, '控制器1')).toContainText('已连接')
    await expect(controllerCard(page, '控制器2')).toContainText('已连接')
  })
})
