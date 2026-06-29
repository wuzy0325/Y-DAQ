import { test, expect } from './fixtures'
import type { Page } from '@playwright/test'

/**
 * 三孔测试 E2E 测试。
 *
 * 覆盖功能：加载校准文件 / 启动 / 暂停 / 恢复 / 停止 / 进度显示 / 完成导出 / 实时录制。
 *
 * 注意：
 * - mock 自动 emit 的 `three-hole:status-updated-${pid}` 通道 store 不监听；
 *   测试需主动用 `mock.emitEvent('three-hole:probe1:progress', {...})` 推送进度。
 * - mock 返回的 status 缺少 `dataPoints` 字段，需通过 `mock.setState` 补充以让「导出CSV」按钮可见。
 */

/** 通过 UI 加载校准文件（设置弹窗 → 选择文件） */
async function loadCalibViaUI(page: Page) {
  // 打开设置弹窗
  await page.getByRole('button', { name: '设置' }).click()
  const dialog = page.locator('.el-dialog').filter({ hasText: '测试设置' })
  await expect(dialog).toBeVisible()
  // 点击"选择文件"按钮（mock 自动返回 ['C:/data/probe1.cal'] 并设置 calibLoaded=true）
  await dialog.getByRole('button', { name: '选择文件' }).click()
  // 等待弹窗内显示已加载状态（.calib-status.loaded）
  await expect(dialog.locator('.calib-status.loaded')).toBeVisible({ timeout: 5000 })
  // 保存设置
  await dialog.getByRole('button', { name: '保存' }).click()
  await expect(dialog).toBeHidden({ timeout: 5000 })
  // 工具栏应显示"校准文件已加载"
  await expect(page.locator('.calib-ok')).toContainText('校准文件已加载')
}

/** 通过 UI 启动测试（前置：calibLoaded=true） */
async function startTestViaUI(page: Page) {
  await page.getByRole('button', { name: '启动测试' }).click()
  // 按钮切换为"暂停"+"停止"
  await expect(page.getByRole('button', { name: '暂停' })).toBeVisible({ timeout: 5000 })
  await expect(page.getByRole('button', { name: '停止' })).toBeVisible({ timeout: 5000 })
}

test.describe('三孔测试', () => {
  test.beforeEach(async ({ page, mock }) => {
    await mock.goto('/three-hole-test?probe=probe1')
    await expect(page).toHaveURL(/#\/three-hole-test/)
  })

  test('加载校准文件', async ({ page }) => {
    // 初始状态：未加载
    await expect(page.locator('.calib-no')).toContainText('未加载校准文件')
    await loadCalibViaUI(page)
    // 验证校准文件已加载
    await expect(page.locator('.calib-ok')).toContainText('校准文件已加载')
    await expect(page.locator('.calib-ok')).toContainText('1')
  })

  test('启动测试（需先加载校准）', async ({ page }) => {
    await loadCalibViaUI(page)
    await startTestViaUI(page)
  })

  test('未加载校准时启动按钮禁用', async ({ page }) => {
    const startBtn = page.getByRole('button', { name: '启动测试' })
    await expect(startBtn).toBeDisabled()
  })

  test('暂停 / 恢复', async ({ page }) => {
    await loadCalibViaUI(page)
    await startTestViaUI(page)

    // 点击暂停
    await page.getByRole('button', { name: '暂停' }).click()
    // 暂停后显示"恢复"按钮
    await expect(page.getByRole('button', { name: '恢复' })).toBeVisible({ timeout: 5000 })
    await expect(page.getByRole('button', { name: '暂停' })).toBeHidden()

    // 点击恢复
    await page.getByRole('button', { name: '恢复' }).click()
    // 恢复后显示"暂停"按钮
    await expect(page.getByRole('button', { name: '暂停' })).toBeVisible({ timeout: 5000 })
    await expect(page.getByRole('button', { name: '恢复' })).toBeHidden()
  })

  test('停止测试', async ({ page }) => {
    await loadCalibViaUI(page)
    await startTestViaUI(page)

    // 点击停止
    await page.getByRole('button', { name: '停止' }).click()
    // 停止后回到初始状态：显示"启动测试"按钮
    await expect(page.getByRole('button', { name: '启动测试' })).toBeVisible({ timeout: 5000 })
    await expect(page.getByRole('button', { name: '停止' })).toBeHidden()
  })

  test('进度显示（emit progress 事件）', async ({ page, mock }) => {
    await loadCalibViaUI(page)
    await startTestViaUI(page)

    // 推送进度事件（store 监听 three-hole:probe1:progress 通道）
    await mock.emitEvent('three-hole:probe1:progress', {
      taskId: 'test-task',
      totalPoints: 9,
      completedPoints: 3,
      progress: 33.3,
      currentX: 10.5,
      currentY: -5.0,
      phase: 'acquiring',
    })

    // 进度条应显示
    await expect(page.locator('.toolbar-progress')).toBeVisible({ timeout: 5000 })
    await expect(page.locator('.toolbar-progress')).toContainText('3 / 9 点')
    await expect(page.locator('.toolbar-progress')).toContainText('10.5')
    await expect(page.locator('.toolbar-progress')).toContainText('-5')
  })

  test('完成态 + 导出CSV按钮显示', async ({ page, mock }) => {
    await loadCalibViaUI(page)
    await startTestViaUI(page)

    // 先在 mockState 中设置带 dataPoints 的 status（用于 complete 事件后 fetchStatus 读取）
    await mock.setState({
      threeHoleStatus: {
        probe1: {
          taskId: 'test-task',
          status: 'completed',
          totalPoints: 9,
          completedPoints: 9,
          progress: 100,
          currentPoint: null,
          dataPoints: [
            { pointId: 1, x: 10, y: 10, rawData: {}, interpResult: {} },
            { pointId: 2, x: 20, y: 20, rawData: {}, interpResult: {} },
          ],
          lastError: '',
        },
      },
    })

    // 推送 complete 事件（store 收到后会调用 fetchStatus 读取上面的 status）
    await mock.emitEvent('three-hole:probe1:complete', { taskId: 'test-task' })

    // 导出CSV按钮应出现
    await expect(page.getByRole('button', { name: '导出CSV' })).toBeVisible({ timeout: 5000 })
  })

  test('错误事件显示', async ({ page, mock }) => {
    await loadCalibViaUI(page)
    await startTestViaUI(page)

    // 推送 error 事件
    await mock.emitEvent('three-hole:probe1:error', {
      taskId: 'test-task',
      error: '设备通信中断',
      isFatal: true,
    })

    // 错误栏应显示
    await expect(page.locator('.error-bar')).toBeVisible({ timeout: 5000 })
    await expect(page.locator('.error-bar')).toContainText('设备通信中断')
  })

  test('实时录制（开始 / 停止）', async ({ page, mock }) => {
    // 实时保存按钮需要 config.deviceId 非空，先添加一个设备
    // 导航到设备页，添加设备（deviceStore 是单例，跨路由保留）
    await mock.goto('/device')
    await page.getByRole('button', { name: '添加设备' }).click()
    await expect(page.locator('.el-dialog').filter({ hasText: '添加设备' })).toBeVisible()
    await page.locator('.device-dialog .el-input__inner').first().fill('三孔测试设备')
    await page.locator('.device-dialog .el-select').first().click()
    await page.locator('.el-select-dropdown__item', { hasText: 'XY-DAQ16' }).click()
    // 取消自动连接
    const autoSwitch = page.locator('.device-dialog .auto-connect-row .el-switch').first()
    if (await autoSwitch.evaluate((el: HTMLElement) => el.classList.contains('is-checked'))) {
      await autoSwitch.click()
    }
    await page.locator('.device-dialog').getByRole('button', { name: '确定' }).click()
    await expect(page.locator('.el-message--success')).toBeVisible({ timeout: 5000 })

    // 导航回三孔测试页
    await mock.goto('/three-hole-test?probe=probe1')

    // 打开设置 → 通道映射 tab → 选择设备 → 保存
    await page.getByRole('button', { name: '设置' }).click()
    const settingsDialog = page.locator('.el-dialog').filter({ hasText: '测试设置' })
    await expect(settingsDialog).toBeVisible()
    // 切换到"通道映射"tab
    await settingsDialog.locator('.el-tabs__item', { hasText: '通道映射' }).click()
    // 选择采集设备（.device-group 下有"采集设备"和"运动控制器"两个 select，取第一个）
    await settingsDialog.locator('.device-group .el-select').first().click()
    await page.locator('.el-select-dropdown__item', { hasText: '三孔测试设备' }).click()
    // 保存
    await settingsDialog.getByRole('button', { name: '保存' }).click()
    await expect(settingsDialog).toBeHidden({ timeout: 5000 })

    // 实时保存按钮应可用
    const recordBtn = page.getByRole('button', { name: '实时保存' })
    await expect(recordBtn).toBeEnabled({ timeout: 5000 })

    // 点击开始录制
    await recordBtn.click()
    // 按钮变为"停止保存"
    await expect(page.getByRole('button', { name: '停止保存' })).toBeVisible({ timeout: 5000 })

    // 点击停止录制
    await page.getByRole('button', { name: '停止保存' }).click()
    // 按钮变回"实时保存"
    await expect(page.getByRole('button', { name: '实时保存' })).toBeVisible({ timeout: 5000 })
  })
})
