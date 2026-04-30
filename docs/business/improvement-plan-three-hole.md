# 三孔移位测试 — 改善计划

> 基于 `docs/review-three-hole.md` 审查结果制定。按优先级分组，每组内按依赖顺序排列。

---

## P0 — 严重问题（4 项）

### P0.1 直线布点前端预览与后端不一致

| 项 | 内容 |
|------|--------|
| **问题** | 前端 `previewPoints` 对 `line` 模式使用单层循环（取 `max(lenX, lenY)` 逐对），后端 `generateLinePoints` 使用双重循环（X×Y 全组合），预览和结果完全不符 |
| **改动文件** | `frontend/src/views/ThreeHoleTestView.vue:419-428` |
| **修复方案** | 将前端 `line` 分支改为与后端一致的双重循环；同时补充 P0.2 的步长 UI 后，预览应与执行一致 |
| **依赖** | 需要 P0.2 的步长 UI 提供 `xStep`/`yStep` 值 |
| **验证** | 前端预览布点数 = 后端 `generateLinePoints` 计算数 |

---

### P0.2 直线布点 UI 增加步长配置

| 项 | 内容 |
|------|--------|
| **问题** | 直线模式 UI 只有起点/终点输入框，无步长输入。后端当 `xSteps`/`ySteps` 为空时只生成 2 个点 |
| **改动文件** | `frontend/src/views/ThreeHoleTestView.vue:167-184` |
| **修复方案** | 在直线布点参数区增加 X 步长和 Y 步长 `el-input-number` 控件；通过 `watch` 同步到 `store.config.layout.line.xSteps`/`ySteps` |
| **参考** | 矩形模式的步长实现（`ThreeHoleTestView.vue:155-163`、`382-395`），可复用相同模式 |
| **验证** | 设置起点(0,0) 终点(20,20) 步长 5 → 预览应显示 5×5=25 个点 |

---

### P0.3 矩形布点预览支持多分段步长

| 项 | 内容 |
|------|--------|
| **问题** | 前端始终用 `expandRange(min, max, singleStep)` 预览，后端支持多段 `StepSegment[]`，多段配置下预览不准 |
| **改动文件** | `frontend/src/views/ThreeHoleTestView.vue:412-418` |
| **修复方案** | 前端 `previewPoints` 中矩形分支改用 `expandSteps(layout.rectangle.xSteps)`/`ySteps`，与后端 `expandStepSegments` 对齐 |
| **细节** | 前端已有 `expandSteps` 函数（`ThreeHoleTestView.vue:458-467`），行为与后端一致。只需替换 `expandRange` 调用即可 |
| **验证** | 设置 `xSteps=[{start:-20,end:0,step:5},{start:0,end:20,step:2}]` → 预览 4+10=14 个 X 值，非单一步长的 9 个 |

---

### P0.4 CSV 保存路径增加 UI 配置入口

| 项 | 内容 |
|------|--------|
| **问题** | 设置弹窗无保存路径和文件名输入，`SavePath` 默认空字符串导致 `os.MkdirAll("")` 行为不可控 |
| **改动文件** | `frontend/src/views/ThreeHoleTestView.vue` 设置弹窗 + `frontend/src/stores/threeHoleTest.ts` 默认值 |
| **修复方案 A（推荐）** | 在"采集参数"区域增加"保存路径"文件夹选择器（`el-button` + 调用 Wails 目录对话框）和"文件名"输入框。前端缺少 Wails 目录对话框绑定，需要先在 `app.go` 新增 `SelectDirectory` 绑定 |
| **修复方案 B（最小改动）** | 设置弹窗增加两个 `el-input`：`savePath`（文本输入）和 `saveFileName`。用户手动输入路径和文件名 |
| **额外** | 默认 `savePath` 改为 `~/.yx-daq/recordings/` |
| **验证** | 配置保存后检查后端 CSV 文件创建在指定路径 |

---

## P1 — 中等问题（4 项）

### P1.1 完成事件后清理 progress

| 项 | 内容 |
|------|--------|
| **问题** | `three-hole:complete` 事件到达时 `progress` ref 未清理，进度条显示过时数据 |
| **改动文件** | `frontend/src/stores/threeHoleTest.ts:313-316` |
| **修复方案** | 在 complete 回调中增加 `progress.value = null` |
| **代码** |  |
| **验证** | 测试完成后进度条应消失或显示空状态 |

---

### P1.2 resumeCh 多消费者 → 统一恢复路径

| 项 | 内容 |
|------|--------|
| **问题** | 3 处独立暂停循环监听同一 `resumeCh`，信号只被一个 consumer 接收，其余依赖 100ms 超时轮询 |
| **改动文件** | `internal/three_hole/service.go` |
| **修复方案 A（推荐）** | 消除重复的暂停检查：`runSinglePoint` 中的暂停循环（line 406-413, 421-428）可移除。只保留 `runTestLoop` 外层（line 290-308）和 `dwellWithRealtimeUpdate`（line 451-467）的暂停检查。`runSinglePoint` 移动后和驻留后的暂停检查是冗余的———`runTestLoop` 的下一个点循环会检查，dwell 内部也已处理暂停 |
| **修复方案 B（保守）** | 将 `resumeCh` 改为带缓冲（`make(chan struct{}, 3)`），并在 `Resume()` 中发送多次信号 |
| **验证** | 暂停 → 恢复 → 测试应立刻继续，无明显延迟 |

---

### P1.3 Stop() 中统一状态设置时序

| 项 | 内容 |
|------|--------|
| **问题** | `Stop()` 先设 status=Idle 后等待 doneCh，`runTestLoop` goroutine 退出前可能设 status=Completed，两处竞争写 |
| **改动文件** | `internal/three_hole/service.go:233,269-354` |
| **修复方案** | `Stop()` 中不设 status，只发 cancelCh 并等待 doneCh。等 doneCh 返回后（goroutine 已退出），在锁中统一设 status=Idle。`runTestLoop` 中收到 cancelCh 直接 return 不设 status |
| **细节** | `runTestLoop` 正常完成时仍设 `Completed`。`Stop()` 在 doneCh 返回后设 `Idle`。这确保了谁最后写谁决定最终值 |
| **验证** | Stop() 后 `GetStatus()` 始终返回 Idle；测试自然完成后始终返回 Completed |

---

### P1.4 Pause/Stop 并发竞争处理

| 项 | 内容 |
|------|--------|
| **问题** | Pause 和 Stop 可并发调用，atomic 操作间无协调。Stop 设置 paused=false 的同时 runTestLoop 可能在执行暂停逻辑 |
| **改动文件** | `internal/three_hole/service.go` |
| **修复方案** | `Stop()` 中先获取 `mu.Lock()` 再设 status + 发 cancelCh，确保与 `Pause()` 互斥。同时 `Pause()` 在 lock 内判断 status 是否是 running，避免在 Stop 过程中暂停 |
| **验证** | 快速点击暂停→停止，不触发 panic，goroutine 正常退出 |

---

## P2 — 体验问题（4 项）

### P2.1 运动轴映射增加 Offset UI 输入

| 项 | 内容 |
|------|--------|
| **问题** | `motionX.offset`/`motionY.offset` 在硬件参数区无输入控件，默认 0 |
| **改动文件** | `frontend/src/views/ThreeHoleTestView.vue:190-208` |
| **修复方案** | 在 Scale 旁边增加 Offfset `el-input-number`。布局参考矩形范围输入（范围输入+分隔符+范围输入的模式） |
| **验证** | 配置带 offset 保存 → 后端 `resolveTargetPosition` 输出正确目标位置 |

---

### P2.2 前端 Error 事件改为同步判断

| 项 | 内容 |
|------|--------|
| **问题** | error 回调中 `fetchStatus()` 后异步判断 `status === 'error'` 才设 `isRunning=false`。异步返回时状态可能已变 |
| **改动文件** | `frontend/src/stores/threeHoleTest.ts:318-327` |
| **修复方案** | error 回调中直接判断错误类型：致命 error 事件（`emitFatalError`）同步设 `isRunning=false`；非致命错误仅显示 lastError。可通过后端在事件中增加 `isFatal` 字段，或前端约定错误消息前缀 |
| **替代方案** | 最小改动：直接同步设 `isRunning=false`（保守些，非致命到完成时的窗口期很短） |
| **验证** | 致命错误 → isRunning 立即 false；点位错误 → isRunning 保持 true |

---

### P2.3 dpZeroThreshold 拆分为独立常量

| 项 | 内容 |
|------|--------|
| **问题** | `1e-6` 用于压力差分、马赫数差、Kb 差三种不同物理含义 |
| **改动文件** | `internal/three_hole/interpolator.go:20` |
| **修复方案** | 拆分为三个常量： |
| | `deltaPZeroThreshold = 1e-6` — 压力差分 (Pa) |
| | `maDiffThreshold = 1e-6` — 马赫数差 |
| | `kbDiffThreshold = 1e-6` — Kb 系数差 |
| **验证** | 编译通过，插值结果不变 |

---

### P2.4 App 关闭时清理测试 goroutine

| 项 | 内容 |
|------|--------|
| **问题** | Wails 关闭窗口时若未调 `Stop()`，`runTestLoop` goroutine 残留 |
| **改动文件** | `app.go` — `OnShutdown` 或 `OnBeforeClose` 生命周期方法 |
| **修复方案** | 在 `App` 结构体上实现 `OnShutdown()` 方法，在其中调用 `threeHoleService.Stop()` |
| **验证** | 测试运行时关闭窗口 → 无 goroutine 泄漏（`go test -race` / pprof） |

---

## 实施顺序建议

```
P0.1 ──→ P0.2     P0.4
   ↕        │
P0.3 ──────┘
                ↓
         P1.1 P1.2
         P1.3 P1.4
                ↓
         P2.1 P2.2 P2.3 P2.4
```

**建议分批**：

| 批次 | 内容 | 预估工时 |
|------|------|---------|
| 第 1 批 | P0.1 + P0.3 预览修正（同一文件同一函数，一起改最合理） | 1h |
| 第 2 批 | P0.2 直线步长 UI | 0.5h |
| 第 3 批 | P0.4 CSV 保存路径 UI | 1h |
| 第 4 批 | P1.1 + P1.3 + P1.4 状态与并发修复 | 2h |
| 第 5 批 | P1.2 resumeCh 优化 | 1h |
| 第 6 批 | P2.x 体验项 | 2h |

---

## 涉及文件清单

| 文件 | 改动项 |
|------|--------|
| `frontend/src/views/ThreeHoleTestView.vue` | P0.1, P0.2, P0.3, P0.4, P2.1 |
| `frontend/src/stores/threeHoleTest.ts` | P0.4, P1.1, P2.2 |
| `internal/three_hole/service.go` | P1.2, P1.3, P1.4 |
| `internal/three_hole/interpolator.go` | P2.3 |
| `app.go` | P0.4（目录选择器绑定）, P2.4（OnShutdown） |
