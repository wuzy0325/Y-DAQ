# 计划：three_hole / five_hole Service 层状态机测试

> 创建日期：2026-06-28
> 触发背景：用户在 race detector 计划之外，要求单独为三孔/五孔 service 层状态机补测试。当前 service 层几乎裸奔（三孔 1 个测试、五孔 0 个生命周期测试），是回归风险最高的区域。
> 范围：仅 `src/internal/three_hole/` 与 `src/internal/five_hole/` 两个包的 Service + TestManager 层状态机。`DataProcessor` / `Interpolator` / `CsvWriter` 等协作者不在本计划范围内（已有独立测试）。

---

## 1. 目标

1. **状态机全覆盖**：把 `Idle → Running → Paused → Running → Completed/Error → Idle` 每条合法迁移路径都钉死。
2. **非法迁移拒绝**：所有非法路径（如 Idle 下 Resume、Running 下再 Start）必须返回错误或静默 no-op，且有断言。
3. **并发安全固化**：Start/Pause/Resume/Stop 在交错调用下不 panic、不死锁、不泄露 goroutine、不产生 race。
4. **生命周期副作用正确**：`taskID` 唯一性、`testGen` 代际隔离、`doneCh` 关闭、`ProbeStatuses` 初始化、CSV 回滚 —— 这些"状态机附带的副作用"必须有断言。
5. **零外部依赖**：所有测试用 mock（`MockEventPublisher` + mock 运动函数 + mock batchGetter），可在 CI 无硬件环境运行。

非目标：插值算法正确性、CSV 文件格式、运动协调器并行度（已有独立测试）、realtime 监控细节（五孔已有 9 个测试覆盖）。

---

## 2. 现状评估

| 维度 | three_hole | five_hole |
|------|-----------|-----------|
| Service 生命周期测试 | [service_test.go](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/three_hole/service_test.go) **仅 1 个**（`TestGenerationCheckSimple`，覆盖双 Start 拒绝） | [service_test.go](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/five_hole/service_test.go) **0 个生命周期测试**（3 个仅覆盖 Validate/LoadCalib） |
| TestManager 单元测试 | [test_manager_test.go](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/three_hole/test_manager_test.go) 19 个，覆盖 Pause/Resume/Stop/SetStatus 等基本迁移 | **无 `test_manager_test.go` 文件** |
| Stop→Restart 测试 | [test_stop_restart_test.go](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/three_hole/test_stop_restart_test.go) 3 个，含 10 次循环的 race 测试 | 无 |
| Service 层并发测试 | `TestConcurrentAccess`（仅 Start×5 并发） | 无 |
| 五孔多探针状态机测试 | N/A | **完全空白** |
| 致命错误路径测试 | 无 | 无 |
| runTestLoop 取消语义 | 无 | 无 |

### 已识别的覆盖空白（按风险排序）

| ID | 空白 | 风险 | 证据 |
|----|------|------|------|
| G1 | five_hole TestManager 完全无测试 | 🔴 极高 | [test_manager.go](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/five_hole/test_manager.go) 有 350+ 行逻辑，`ProbeStatuses` 初始化、`CloseDoneCh`、`EmitFatalError` 均无测试 |
| G2 | Service.Start 失败回滚路径 | 🔴 高 | [three_hole/service.go:148-160](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/three_hole/service.go#L148-L160)、[five_hole/service.go:286-300](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/five_hole/service.go#L286-L300) `eventHandler.OnTestStart`/`testManager.Start` 失败时的 `testRunning`/CSV 回滚未被验证 |
| G3 | EmitFatalError 状态迁移 | 🟠 中 | [three_hole/test_manager.go:248-262](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/three_hole/test_manager.go#L248-L262) 应切到 `TraversalStatusError` 并清零 running/paused，无测试 |
| G4 | testGen 代际隔离 | 🟠 中 | 仅 [service_test.go](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/three_hole/service_test.go) 有简化版测试，未验证"残留 goroutine 不干扰新测试"的真实场景 |
| G5 | five_hole ProbeStatuses 副作用 | 🟠 中 | [test_manager.go:74-83](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/five_hole/test_manager.go#L74-L83) 启用探针初始化、`UpdateProbeStatus`/`UpdateProbeData` 均无测试 |
| G6 | Stop 在 Paused 状态 | 🟡 低 | three_hole 已有 `TestStop_CompletedTask`，但未覆盖 `Stop_PausedTask` |
| G7 | Pause 在非 Running 状态的 no-op | 🟡 低 | three_hole 有 `TestPause_NotRunning`，但未覆盖 `Pause_FromError`/`Pause_FromCompleted` |
| G8 | runTestLoop 中途取消 | 🟠 中 | 测试循环中 `ctx.Err()` 检查点 ([three_hole/service.go:221](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/three_hole/service.go#L221), [five_hole/service.go:382](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/five_hole/service.go#L382)) 无覆盖 |
| G9 | five_hole 数据停滞自动暂停 | 🟠 中 | [service.go:485-498](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/five_hole/service.go#L485-L498) `ErrDataStagnant` → 自动 Pause → 等待恢复 → 重新采样，无测试 |
| G10 | Service.Stop 100ms sleep 的时序假设 | 🟡 低 | [three_hole/service.go:190](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/three_hole/service.go#L190)、[five_hole/service.go:328](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/five_hole/service.go#L328) 用 `time.Sleep(100ms)` 等待 goroutine 退出，脆弱 |

---

## 3. 状态机迁移图（测试覆盖矩阵）

```
                          ┌──────────┐
                          │   Idle   │ ◄─────────────────────┐
                          └────┬─────┘                       │
                               │ Start(config)               │ Stop
                               ▼                             │
                          ┌──────────┐  Pause  ┌──────────┐   │
              ┌──────────►│ Running  ├────────►│  Paused  │   │
              │           └─┬──┬─────┘ ◄───────┴────┬─────┘   │
              │             │  │      Resume       │         │
              │             │  │                   │ Stop    │
              │             │  │ ErrDataStagnant   │         │
              │             │  ▼ (5H only)        │         │
              │             │ auto-Pause ─────────┘         │
              │             │                                │
              │             │ OnTestComplete (loop ends)     │
              │             ▼                                │
              │      ┌────────────┐                          │
              │      │ Completed  │ ──── Stop ──────────────►│
              │      └────────────┘                          │
              │                                                │
              │ EmitFatalError                                 │
              ▼                                                │
        ┌────────────┐                                        │
        │   Error    │ ──── Stop ────────────────────────────►│
        └────────────┘
```

### 迁移矩阵（✓=合法，✗=非法/no-op）

| From \ Event | Start | Pause | Resume | Stop | FatalErr | Complete |
|--------------|:-----:|:-----:|:------:|:----:|:--------:|:--------:|
| Idle         |   ✓   |   ✗   |   ✗    |  ✗   |    ✗     |    ✗     |
| Running      |   ✗   |   ✓   |   ✗    |  ✓   |    ✓     |    ✓     |
| Paused       |   ✗   |   ✗   |   ✓    |  ✓   |    ✓     |    ✗     |
| Completed    |   ✓*  |   ✗   |   ✗    |  ✓   |    ✗     |    ✗     |
| Error        |   ✓*  |   ✗   |   ✗    |  ✓   |    ✗     |    ✗     |

> ✓* = `Start` 在 `Completed`/`Error` 状态下应允许（重新开始新测试），需断言 `taskID` 变化、`testGen` 递增。

---

## 4. 实施计划（分阶段）

### 阶段 P0：five_hole TestManager 单元测试（补 G1）

> 优先级最高。five_hole TestManager 是当前最大盲区。新建 `src/internal/five_hole/test_manager_test.go`。

| ID | 测试函数 | 覆盖 |
|----|---------|------|
| P0-1 | `TestNewTestManager_InitialState` | 初始 status=Idle、ProbeStatuses=空切片、ctx 已取消 |
| P0-2 | `TestStart_InitializesProbeStatuses` | Start 后 ProbeStatuses 按启用探针初始化（Phase="idle"），禁用探针不出现 |
| P0-3 | `TestStart_AlreadyRunning` | Running 下 Start 返回 "test already running" 错误 |
| P0-4 | `TestStart_GeneratesUniqueTaskID` | 两次 Start（中间 Stop）taskID 不同，前缀 "5h-traversal-" |
| P0-5 | `TestStart_GeneratesPointsError` | 故意构造无法生成布点的 Layout，断言错误信息 |
| P0-6 | `TestPause_RunningTask` | Pause 后 status=Paused、paused.Load()=true |
| P0-7 | `TestPause_NotRunning_NoOp` | Idle 下 Pause 不改状态 |
| P0-8 | `TestResume_PausedTask` | Resume 后 status=Running、paused.Load()=false |
| P0-9 | `TestResume_NotPaused_NoOp` | Idle/Running 下 Resume 不改状态 |
| P0-10 | `TestStop_FromRunning` | Stop 后 status=Idle、running=false、paused=false |
| P0-11 | `TestStop_FromPaused` | G6 |
| P0-12 | `TestStop_FromCompleted` | 验证 TaskID 保留 |
| P0-13 | `TestStop_FromError` | Error 状态下 Stop 合法 |
| P0-14 | `TestStop_Idle_NoOp` | Idle 下 Stop 不 panic |
| P0-15 | `TestUpdateProgress_UpdatesFields` | CompletedPoints/Progress/CurrentPoint |
| P0-16 | `TestUpdateProbeStatus_UpdatesPhaseAndCoords` | 指定 probeID 的 Phase/CurrentX/CurrentY 更新 |
| P0-17 | `TestUpdateProbeStatus_UnknownProbe_NoOp` | 不存在的 probeID 不 panic |
| P0-18 | `TestUpdateProbeData_UpdatesRawAndInterp` | RawData/InterpResult 指针更新 |
| P0-19 | `TestEmitFatalError_TransitionsToError` | G3：status=Error、running=false、paused=false、发布 isFatal=true 事件 |
| P0-20 | `TestCloseDoneCh_IdempotentAndNilSafe` | 多次 CloseDoneCh 不 panic；doneCh 关闭后 waitForTestComplete 能退出 |
| P0-21 | `TestConcurrent_Start_OnlyOneSucceeds` | 5 个 goroutine 并发 Start，仅 1 个成功 |
| P0-22 | `TestConcurrent_PauseResumeStop_NoPanic` | 测试运行期间并发 Pause/Resume/Stop，1s 后无 panic |

> five_hole TestManager 与 three_hole 平行，可大量复用 three_hole [test_manager_test.go](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/three_hole/test_manager_test.go) 的测试模式，但需追加 ProbeStatuses 相关断言。

### 阶段 P1：three_hole TestManager 补强（补 G3、G6、G7）

> three_hole 已有 19 个测试，仅补关键缺口。修改/追加到 `src/internal/three_hole/test_manager_test.go` 与 `test_stop_restart_test.go`。

| ID | 测试函数 | 覆盖 |
|----|---------|------|
| P1-1 | `TestEmitFatalError_TransitionsToError` | G3：status=Error、running=false、paused=false、errorEvents 含 isFatal=true |
| P1-2 | `TestEmitFatalError_FromIdle_NoStateCorruption` | Idle 下调用 EmitFatalError 不应让 status 卡在非 Idle |
| P1-3 | `TestStop_FromPaused` | G6：Paused → Stop → Idle |
| P1-4 | `TestPause_FromCompleted_NoOp` | G7：Completed 下 Pause 不改状态 |
| P1-5 | `TestPause_FromError_NoOp` | G7：Error 下 Pause 不改状态 |
| P1-6 | `TestStart_AfterCompleted_SucceedsWithNewTaskID` | Completed → Start 应成功，taskID 不同，testGen+1 |
| P1-7 | `TestStart_AfterError_SucceedsWithNewTaskID` | Error → Start 应成功 |
| P1-8 | `TestConcurrent_PauseResumeStop_DuringRun` | 启动测试循环后并发 Pause/Resume/Stop，1s 内不 panic 不死锁 |

### 阶段 P2：Service 层状态机测试（补 G2、G4、G8）

> 这是本计划的核心。三孔 [service_test.go](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/three_hole/service_test.go) 与五孔 [service_test.go](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/five_hole/service_test.go) 各自扩展。

#### 2.1 共享测试模式（两包平行实现）

| ID | 测试函数 | 覆盖 |
|----|---------|------|
| P2-1 | `TestService_Start_ReturnsError_WhenConfigInvalid` | 配置未通过 Validate，Start 返回错误，testRunning 仍为 false |
| P2-2 | `TestService_Start_Rollback_OnTestManagerStartFail` | G2：注入会失败的 testManager（构造能通过 Validate 但 generatePoints 失败的 config），断言 `testRunning=false`、CSV writer 已关闭（五孔）/未创建文件（三孔） |
| P2-3 | `TestService_Stop_DuringRun_StopsLoop` | G8：Start → 等 100ms → Stop → 500ms 内 status=Idle、runTestLoop goroutine 退出（用 goroutine 计数器或 doneCh 验证） |
| P2-4 | `TestService_Stop_DuringPause_StopsLoop` | G8：Start → Pause → Stop → status=Idle |
| P2-5 | `TestService_PauseResume_DuringRun_LoopContinues` | Start → 立即 Pause → 等 200ms → Resume → 验证 progress 事件序列含 "moving"→无→"moving" |
| P2-6 | `TestService_Start_RejectsDoubleStart` | 第二次 Start 返回错误，原 taskID 不变 |
| P2-7 | `TestService_Start_AfterStop_SucceedsWithNewTaskID` | Stop → Start，新 taskID，testGen 递增 |
| P2-8 | `TestService_Start_AfterComplete_Succeeds` | 等测试自然完成（用 1 个点位 + mock 立即返回的依赖）→ Start 新测试成功 |
| P2-9 | `TestService_FatalError_StopsTest` | 注入 DataProcessor 让 RunSinglePoint 返回致命错误 → status=Error、发布 isFatal 事件 |
| P2-10 | `TestService_NonFatalError_ContinuesTest` | 注入让某点位失败但非致命 → status 仍 Running、继续下一点位、错误事件 isFatal=false |
| P2-11 | `TestService_GenerationIsolation_OldGoroutineDoesNotInterfere` | G4：Start → 等 50ms → Stop → 立即 Start → 验证旧 goroutine 不再发布 progress 事件（用 publisher 事件 TaskID 区分） |
| P2-12 | `TestService_Concurrent_StartPauseStop_NoRace` | 3 个 goroutine 分别循环 Start/Pause/Stop，运行 2s，`-race` 通过 |

#### 2.2 five_hole 专属测试

| ID | 测试函数 | 覆盖 |
|----|---------|------|
| P2-13 | `TestService_Start_Rollback_OnCSVInitFail` | G2：注入会失败的 csvWriter（用 SavePath 为只读目录），断言 testRunning=false、testManager 未进入 Running |
| P2-14 | `TestService_Start_InitializesProbeStatuses_ForEnabledProbesOnly` | G5：3 个探针（2 启用 1 禁用），Start 后 ProbeStatuses 仅含 2 个 |
| P2-15 | `TestService_AutoPause_OnDataStagnant_ThenResume` | G9：构造 batchGetter 返回 `ErrDataStagnant` → 验证 status=Paused、error 事件含 "数据停滞" → Resume 后重新采样当前点位 |
| P2-16 | `TestService_MultiProbe_ProbeStatuses_UpdateIndependently` | 多探针场景下，UpdateProbeStatus 对 probe1 不影响 probe2 |
| P2-17 | `TestService_Stop_ClosesAllProbeCSVWriters` | Stop 后所有启用探针的 CSV 文件句柄关闭（用文件锁/重命名测试） |

#### 2.3 three_hole 专属测试

| ID | 测试函数 | 覆盖 |
|----|---------|------|
| P2-18 | `TestService_RealtimeMonitor_Stops_OnTestStart` | Start 测试后，realtime monitor 不再推送（testRunning=true 阻断） |
| P2-19 | `TestService_RealtimeMonitor_Restarts_OnTestStop` | Stop 后 monitor 可继续推送 |

### 阶段 P3：并发与压力测试（补 G10）

| ID | 测试函数 | 覆盖 |
|----|---------|------|
| P3-1 | `TestService_RapidStopStart_100Cycles_NoLeak` | 100 次 Stop→Start 循环，goroutine 数不增长（用 `runtime.NumGoroutine()` 前后对比） |
| P3-2 | `TestService_RapidStopStart_100Cycles_NoRace` | 同上 + `-race` |
| P3-3 | `TestService_Stop_ThenImmediateStart_NoDeadlock` | G10：针对 100ms sleep 的脆弱性，Stop 后立即 Start（不等 sleep 完成）应不死锁 |
| P3-4 | `TestTestManager_WaitForTestComplete_DoesNotLeakOnCancel` | Start → Stop → 验证 waitForTestComplete goroutine 在 1s 内退出 |

---

## 5. 测试基础设施

### 5.1 Mock 工具

两包已有 `MockEventPublisher`（[three_hole/test_helpers.go](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/three_hole/test_helpers.go)、[five_hole/service_test.go](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/five_hole/service_test.go#L14)），均已加 `sync.Mutex`。需新增：

```go
// three_hole/test_helpers.go 追加
func makeMockMotionController() (ThreeHoleMotionController, *int32) {
    var calls int32
    return func(axis types.AxisName, position float64) error {
        atomic.AddInt32(&calls, 1)
        return nil
    }, &calls
}

// 类似 makeMockMotionWaiter、makeMockBatchGetter
```

### 5.2 five_hole 测试辅助

five_hole 已有 [service_test.go](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/five_hole/service_test.go#L72) 中的 `makeValidFiveHoleConfig`/`makeEnabledProbe`/`writeTestCalFile`，复用即可。需新增：

```go
// five_hole/test_helpers.go（新建）
func makeMockMover() (FiveHoleProbeAxisMover, *int32) { ... }
func makeMockWaiter() (FiveHoleProbeAxisWaiter, *int32) { ... }
func makeStagnantBatchGetter() FiveHoleMultiDeviceBatchGetter { ... }  // 用于 G9
```

### 5.3 测试规范

- **不要 `time.Sleep` 做同步**：用 channel + `select` + 超时（1s）做"Eventually"模式。
- **goroutine 泄漏检测**：用 `runtime.NumGoroutine()` 在测试开始和结束对比，差值 > 0 时 `t.Errorf`。
- **事件断言**：用 `MockEventPublisher.GetXxxEvents()` 取快照，不要在事件回调中断言（会持锁）。
- **并发测试**：`-race` 必须通过；用 `t.Run` 子测试隔离场景。
- **测试名约定**：`TestService_<方法>_<场景>`、`TestTestManager_<方法>_<场景>`，场景用中文便于对应业务。

---

## 6. 验收标准

- [ ] P0 完成：five_hole 新增 `test_manager_test.go`，22 个测试全部通过。
- [ ] P1 完成：three_hole TestManager 补 8 个测试，全部通过。
- [ ] P2.1 完成：两包各 12 个 Service 层测试通过。
- [ ] P2.2 完成：five_hole 5 个专属测试通过。
- [ ] P2.3 完成：three_hole 2 个专属测试通过。
- [ ] P3 完成：4 个并发/压力测试通过，`-race` 零报告。
- [ ] 整体：`go test -race -count=1 ./internal/three_hole/... ./internal/five_hole/...` 在本地零失败、零 race。
- [ ] 覆盖率：Service 与 TestManager 文件行覆盖率 ≥ 80%（用 `go test -cover` 验证）。

---

## 7. 风险与回退

| 风险 | 影响 | 缓解 |
|------|------|------|
| Service.Stop 的 100ms sleep 导致测试慢 | 测试套件总时长 > 5s | 用 `testing.Short()` 跳过慢测试；或重构 Stop 接受 `context.WithTimeout` |
| runTestLoop 无法注入取消点 | P2-3/P2-4 难以验证 goroutine 退出 | 用 mock 让 RunSinglePoint 阻塞在 channel 上，Stop 后验证 channel 已被放弃 |
| five_hole 自动暂停路径涉及多协程 | P2-15 测试 flaky | 用受控的 batchGetter（按调用次数切换返回值）而非 time.Sleep |
| 现有 three_hole 测试失败（topics.md 记录） | 阻塞 P1 | 先修现有失败测试（race detector 计划 P0-3 已涵盖），本计划假定其已修复 |
| EmitFatalError 在 Idle 下调用导致状态污染 | P1-2 暴露新 bug | 这正是测试价值所在；如确认是 bug，单独修后再合并测试 |

---

## 8. 执行顺序建议

```
P0 (five_hole TestManager)        ← 独立 PR，无依赖
        ↓
P1 (three_hole TestManager 补强)  ← 依赖 race detector 计划 P0-3 修好现有失败
        ↓
P2.1 (两包 Service 平行测试)      ← 依赖 P0/P1 的 mock 工具
        ↓
P2.2 + P2.3 (各包专属测试)        ← 可并行
        ↓
P3 (并发/压力测试)                ← 最后跑，验证整体
```

> 与 [race-detector-ci-plan.md](file:///d:/work/wuli/YX_DAQ/YX-DAQ/docs/plans/2026-06-28-race-detector-ci-plan.md) 的关系：
> - 依赖该计划 P0-3（修复 three_hole 已失败测试）。
> - 本计划的 P3 测试将自动被该计划 P3 的 CI 工作流覆盖。
> - 两计划可并行启动，P0 互不阻塞。

---

## 9. 不做什么（防 scope creep）

- 不重写 Service/TestManager 架构（仅测试，不改实现；发现的 bug 单独 issue 跟踪）。
- 不测试 DataProcessor 的采样逻辑（已有 [data_processor_test.go](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/three_hole/data_processor_test.go)）。
- 不测试 Interpolator 算法（已有 [interpolator_test.go](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/three_hole/interpolator_test.go)）。
- 不测试 CsvWriter 文件格式（已有 [csv_writer_test.go](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/three_hole/csv_writer_test.go)）。
- 不测试 RealtimeRecorder（已有 [realtime_recorder_test.go](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/three_hole/realtime_recorder_test.go)）。
- 不测试 EventPublisher 的 Wails 事件桥接（属于 app 层，非本包职责）。
- 不引入 testify/testify 等断言库（项目现有测试用标准库 `testing`，保持一致）。
- 不增加 benchmark（性能测试另立计划）。
