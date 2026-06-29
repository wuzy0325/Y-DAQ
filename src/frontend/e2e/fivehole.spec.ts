import { test, expect } from './fixtures'
import type { Page } from '@playwright/test'

/**
 * 五孔测试 E2E 测试。
 *
 * 覆盖功能：多探针加载校准文件 / 启动 / 暂停 / 恢复 / 停止 / 进度显示 / 完成导出 / 实时录制 / 错误显示。
 *
 * 注意：
 * - 五孔事件通道为全局（five-hole:progress / five-hole:complete / five-hole:error），不按 probeID 区分。
 * - 校准文件加载需在设置弹窗"探针配置"tab 下为每个探针单独点击"选择校准文件"。
 * - 默认配置 3 个探针均启用，allCalibLoaded 要求全部加载完成。
 */

/** 通过 UI 为所有启用的探针加载校准文件 */
async function loadAllCalibViaUI(page: Page) {
  // 打开设置弹窗
  await page.getByRole('button', { name: '设置' }).click()
  const dialog = page.locator('.el-dialog').filter({ hasText: '五孔测试设置' })
  await expect(dialog).toBeVisible()

  // 切换到"探针配置"tab
  await dialog.locator('.el-tabs__item', { hasText: '探针配置' }).click()

  // 为每个探针加载校准文件（mock 自动返回 ['C:/data/probe.prb'] 并设置 calibLoaded=true）
  // 注意：不逐探针断言 .calib-ok，因为 Element Plus tabs 将所有 tab pane 保留在 DOM 中，
  // 加载第二个探针后会有 2 个 .calib-ok 元素，导致 strict mode violation。
  for (const probeLabel of ['探针 1', '探针 2', '探针 3']) {
    // 点击对应的探针 tab（在 .probe-tabs 内）
    await dialog.locator('.probe-tabs .el-tabs__item', { hasText: probeLabel }).click()
    await dialog.getByRole('button', { name: '选择校准文件' }).click()
    // 等待 store 异步操作完成（calibLoadedMap 更新触发响应式）
    await page.waitForTimeout(300)
  }

  // 保存设置
  await dialog.getByRole('button', { name: '保存' }).click()
  await expect(dialog).toBeHidden({ timeout: 5000 })
  // 工具栏应显示校准就绪（用 .toolbar 前缀避开 dialog 中残留的 .calib-ok）
  await expect(page.locator('.toolbar .calib-ok')).toContainText('校准就绪')
}

/** 通过 UI 启动测试（前置：allCalibLoaded=true） */
async function startTestViaUI(page: Page) {
  await page.getByRole('button', { name: '启动测试' }).click()
  await expect(page.getByRole('button', { name: '暂停' })).toBeVisible({ timeout: 5000 })
  await expect(page.getByRole('button', { name: '停止' })).toBeVisible({ timeout: 5000 })
}

test.describe('五孔测试', () => {
  test.beforeEach(async ({ page, mock }) => {
    await mock.goto('/five-hole-test')
    await expect(page).toHaveURL(/#\/five-hole-test/)
  })

  test('加载校准文件（多探针）', async ({ page }) => {
    // 初始状态：校准未就绪（用 .toolbar 前缀避开 dialog 残留元素）
    await expect(page.locator('.toolbar .calib-no')).toContainText('校准未就绪')
    await loadAllCalibViaUI(page)
    // 验证校准就绪状态
    await expect(page.locator('.toolbar .calib-ok')).toContainText('校准就绪')
    await expect(page.locator('.toolbar .calib-ok')).toContainText('3 探针')
  })

  test('启动测试（需先加载校准）', async ({ page }) => {
    await loadAllCalibViaUI(page)
    await startTestViaUI(page)
  })

  test('未加载校准时启动按钮禁用', async ({ page }) => {
    const startBtn = page.getByRole('button', { name: '启动测试' })
    await expect(startBtn).toBeDisabled()
  })

  test('暂停 / 恢复', async ({ page }) => {
    await loadAllCalibViaUI(page)
    await startTestViaUI(page)

    // 点击暂停
    await page.getByRole('button', { name: '暂停' }).click()
    await expect(page.getByRole('button', { name: '恢复' })).toBeVisible({ timeout: 5000 })
    await expect(page.getByRole('button', { name: '暂停' })).toBeHidden()

    // 点击恢复
    await page.getByRole('button', { name: '恢复' }).click()
    await expect(page.getByRole('button', { name: '暂停' })).toBeVisible({ timeout: 5000 })
    await expect(page.getByRole('button', { name: '恢复' })).toBeHidden()
  })

  test('停止测试', async ({ page }) => {
    await loadAllCalibViaUI(page)
    await startTestViaUI(page)

    // 点击停止
    await page.getByRole('button', { name: '停止' }).click()
    // 停止后回到初始状态：显示"启动测试"按钮（mock 的 StopFiveHoleTraversal 将 status 设为 idle）
    await expect(page.getByRole('button', { name: '启动测试' })).toBeVisible({ timeout: 5000 })
    await expect(page.getByRole('button', { name: '停止' })).toBeHidden()
  })

  test('进度显示（emit progress 事件）', async ({ page, mock }) => {
    await loadAllCalibViaUI(page)
    await startTestViaUI(page)

    // 推送五孔全局进度事件（store 监听 five-hole:progress 通道）
    await mock.emitEvent('five-hole:progress', {
      totalPoints: 81,
      completedPoints: 15,
      progress: 18.5,
      currentX: 10.5,
      currentY: -5.0,
      phase: 'acquiring',
    })

    // 进度条应显示
    await expect(page.locator('.toolbar-progress')).toBeVisible({ timeout: 5000 })
    await expect(page.locator('.toolbar-progress')).toContainText('15 / 81 点')
    await expect(page.locator('.toolbar-progress')).toContainText('10.5')
    await expect(page.locator('.toolbar-progress')).toContainText('-5')
  })

  test('完成态 + 导出CSV按钮显示', async ({ page, mock }) => {
    await loadAllCalibViaUI(page)
    await startTestViaUI(page)

    // 设置 complete 状态（store 在 complete 事件后调用 fetchStatus 读取）
    await mock.setState({
      fiveHoleStatus: {
        status: 'completed',
        currentPointIndex: 80,
        totalPoints: 81,
        completedPoints: 81,
        progress: 100,
        probeStatuses: [
          { probeId: 'probe1', probeIndex: 0, status: 'completed', completedPoints: 81 },
          { probeId: 'probe2', probeIndex: 1, status: 'completed', completedPoints: 81 },
          { probeId: 'probe3', probeIndex: 2, status: 'completed', completedPoints: 81 },
        ],
        errorMessage: '',
      },
    })

    // 推送 complete 事件（store 收到后缓存 probeDataPoints 到 completeProbeDataPoints）
    await mock.emitEvent('five-hole:complete', {
      taskId: 'test-task',
      probeDataPoints: {
        probe1: [{
          pointId: 1, probeId: 'probe1', x: 10, y: 10,
          rawData: { p1: 0, p2: 0, p3: 0, p4: 0, p5: 0, pAtm: 0, tAtm: 0 },
          interpResult: {
            alphaProbe: 0, betaProbe: 0, machProbe: 0, velocityProbe: 0,
            ptProbe: 0, psProbe: 0, casProbe: 0, satProbe: 0,
            dynamicPressure: 0, density: 0, vxProbe: 0, vyProbe: 0, vzProbe: 0,
          },
          sampleCount: 10, timestamp: 1234567890,
        }],
        probe2: [{
          pointId: 1, probeId: 'probe2', x: 10, y: 10,
          rawData: { p1: 0, p2: 0, p3: 0, p4: 0, p5: 0, pAtm: 0, tAtm: 0 },
          interpResult: {
            alphaProbe: 0, betaProbe: 0, machProbe: 0, velocityProbe: 0,
            ptProbe: 0, psProbe: 0, casProbe: 0, satProbe: 0,
            dynamicPressure: 0, density: 0, vxProbe: 0, vyProbe: 0, vzProbe: 0,
          },
          sampleCount: 10, timestamp: 1234567890,
        }],
        probe3: [{
          pointId: 1, probeId: 'probe3', x: 10, y: 10,
          rawData: { p1: 0, p2: 0, p3: 0, p4: 0, p5: 0, pAtm: 0, tAtm: 0 },
          interpResult: {
            alphaProbe: 0, betaProbe: 0, machProbe: 0, velocityProbe: 0,
            ptProbe: 0, psProbe: 0, casProbe: 0, satProbe: 0,
            dynamicPressure: 0, density: 0, vxProbe: 0, vyProbe: 0, vzProbe: 0,
          },
          sampleCount: 10, timestamp: 1234567890,
        }],
      },
    })

    // 导出CSV按钮应出现且可用
    await expect(page.getByRole('button', { name: '导出 CSV' })).toBeEnabled({ timeout: 5000 })
  })

  test('错误事件显示', async ({ page, mock }) => {
    await loadAllCalibViaUI(page)
    await startTestViaUI(page)

    // 推送 error 事件
    await mock.emitEvent('five-hole:error', {
      taskId: 'test-task',
      error: '设备通信中断',
      isFatal: true,
    })

    // 错误栏应显示
    await expect(page.locator('.error-bar')).toBeVisible({ timeout: 5000 })
    await expect(page.locator('.error-bar')).toContainText('设备通信中断')
  })

  test('实时录制（开始 / 停止）', async ({ page }) => {
    // 实时保存按钮始终可见且可用
    const recordBtn = page.getByRole('button', { name: '实时保存' })
    await expect(recordBtn).toBeVisible()

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