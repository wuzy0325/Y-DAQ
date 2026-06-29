import { defineConfig, devices } from '@playwright/test'

/**
 * E2E 测试配置。
 *
 * 策略：
 * - webServer 用 env E2E=true 启动 Vite dev server（vite.config.ts 检测此 env 切换到 mock 别名）
 * - 单 worker 串行执行：mock 状态在浏览器内存中共享，并行会互相干扰
 * - 每个测试通过 fixture 重新 page.goto 拿到干净状态
 * - 端口 5174 避免与正常 wails3 dev (5173) 冲突
 */
export default defineConfig({
  testDir: './e2e',
  outputDir: './e2e/test-results',
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: [['list']],
  timeout: 30_000,
  expect: { timeout: 5_000 },
  use: {
    baseURL: 'http://localhost:5174',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    actionTimeout: 8_000,
  },
  projects: [
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        // 使用系统已安装的 Edge，避免下载 Chromium（镜像缺失 / 沙箱限制 AppData）
        channel: 'msedge',
      },
    },
  ],
  webServer: {
    command: 'npm run dev:e2e',
    env: { E2E: 'true' } as Record<string, string>,
    url: 'http://localhost:5174',
    reuseExistingServer: false,
    timeout: 60_000,
    stdout: 'ignore',
    stderr: 'pipe',
  },
})
