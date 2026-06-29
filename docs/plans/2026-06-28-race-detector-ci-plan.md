# 计划：将 `go test -race ./internal/...` 纳入 CI

> 创建日期：2026-06-28
> 触发背景：刚完成 `MotionControllerManager.StartPolling` 竞态修复（CompareAndSwap + `context.WithCancel`），需固化保护，避免回归。
> 范围：仅后端 Go 代码（`src/internal/...`）。前端 Vitest 不涉及 race detector。

---

## 1. 目标

1. **修复已识别的真实竞态**（不是仅靠 `-race` 跑出来，而是代码审查已确认）。
2. **补齐并发测试用例**，让 `-race` 在 CI 上有实际内容可跑、能暴露问题。
3. **本地一键运行**：通过 `wails3 task test:race` 触发，与现有 Taskfile 工作流一致。
4. **CI 自动化**：在 GitHub Actions（或等价物）上对每个 PR / push 自动执行 race 检测。
5. **零误报**：所有 race 报告必须可稳定复现并修复，禁止用 `// nolint` 或跳过机制掩盖。

非目标：性能基准测试、E2E 桌面应用测试、前端测试纳入 CI（后续单独计划）。

---

## 2. 现状评估

| 维度 | 现状 | 评估 |
|------|------|------|
| CI 配置 | 仓库根无 `.github/workflows/`，无 `.gitlab-ci.yml` | **缺失** |
| 本地 race 任务 | `Taskfile.yml` 仅有 build/package/dev/run | **缺失** |
| 现有 Go 测试 | `calibration/formulas_test.go`、`storage/config_store_test.go`、`manager/*_test.go`、`driver/*_test.go`、`three_hole/*_test.go`、`five_hole/*_test.go` | 覆盖以纯函数/CRUD 为主 |
| 并发测试 | `acquisition_hub_test.go`、`motion_manager_test.go`、`tcp_base_test.go` 均无 `go func` 并发场景 | **空白** |
| `StartPolling` 竞态 | 已修复（[motion_manager.go:357-392](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/manager/motion_manager.go#L357-L392)） | 需固化测试 |
| `three_hole` 测试 | topics.md 记录"three_hole 包测试失败为重构前已存在问题" | **阻塞项**，需先修 |

---

## 3. 已识别的竞态风险点（代码审查证据）

### R1: `AcquisitionHub.SetOnSnapshot` 无锁 🔴

[acquisition_hub.go:90-92](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/manager/acquisition_hub.go#L90-L92)
```go
func (h *AcquisitionHub) SetOnSnapshot(cb func(snapshots []types.DataPayload)) {
    h.onSnapshot = cb   // 无锁写
}
```
而 `StartPublishing` goroutine 在 [acquisition_hub.go:119](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/manager/acquisition_hub.go#L119) 直接读 `h.onSnapshot`。**`-race` 必报**。

### R2: `TCPDriverBase.SetDataCallback` 无锁 🔴

[tcp_base.go:46-48](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/driver/tcp_base.go#L46-L48)
```go
func (b *TCPDriverBase) SetDataCallback(cb types.DataCallback) {
    b.onData = cb   // 无锁写
}
```
而 `EmitData` 在 [tcp_base.go:237-238](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/driver/tcp_base.go#L237-L238) 由 `receiveLoop` goroutine 调用读 `b.onData`。**`-race` 必报**。

### R3: `MotionControllerManager.StartPolling` 修复未固化 🟡

[motion_manager.go:357-392](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/manager/motion_manager.go#L357-L392) 已用 `CompareAndSwap` + `context.WithCancel`，但无对应并发测试，回归无防护。

### R4: `MotionControllerManager.onStatusChange` 读写不对称 🟡

[motion_manager.go:86-90](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/manager/motion_manager.go#L86-L90) 用 `m.Lock()`（写锁），[motion_manager.go:95-97](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/manager/motion_manager.go#L95-L97) 用 `m.RLock()`（读锁）——这是正确的。但 `onStatusChange` 字段访问未与 `runtimeStatus`/`instances` 的访问同步，需在并发测试下确认 `emitStatusChange` 在 `Connect`/`RemoveProfile` 期间不产生数据竞争。

### R5: 三孔/五孔 TestManager 的 `ctx`/`cancel` 字段并发访问 🟡

[three_hole/test_manager.go:14-26](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/three_hole/test_manager.go#L14-L26) 中 `ctx`/`cancel` 字段在 `mu` 锁外可能被读取（需进一步审查）。`five_hole` 结构平行。

---

## 4. 实施计划（分阶段）

### 阶段 P0：修复已知竞态（前置阻塞）

> 必须先完成，否则 `-race` 在 CI 上跑起来就是红的，毫无意义。

| ID | 任务 | 文件 | 验证 |
|----|------|------|------|
| P0-1 | 修复 R1：`SetOnSnapshot` 加 `h.mu.Lock()`，`StartPublishing` 读取时也加 `RLock` | `src/internal/manager/acquisition_hub.go` | `go test -race ./internal/manager/...` |
| P0-2 | 修复 R2：`SetDataCallback` 改用 `b.mu.Lock()`，`EmitData` 改用 `getOnData()` 加锁读取（与 `getOnStatusChange` 风格一致） | `src/internal/driver/tcp_base.go` | `go test -race ./internal/driver/...` |
| P0-3 | 修复 three_hole 已失败的测试（topics.md 记录的"重构前已存在问题"） | `src/internal/three_hole/` | `go test ./internal/three_hole/...` 通过 |
| P0-4 | 审查 R5：确认 `ctx`/`cancel` 字段访问均在 `mu` 锁内，或改用 `atomic.Pointer` | `three_hole/test_manager.go`、`five_hole/test_manager.go` | 代码审查 + race 测试 |

### 阶段 P1：补充并发测试用例

> 让 `-race` 有内容可跑，且能暴露未来回归。

| ID | 测试文件 | 测试内容 | 覆盖目标 |
|----|---------|---------|---------|
| P1-1 | `acquisition_hub_test.go` 新增 `TestAcquisitionHub_Concurrent_OnData_And_GetSnapshot` | N 个 goroutine 并发 `OnData` + M 个 goroutine 并发 `GetSnapshot`/`GetLatestValue`，运行 1s | R1 回归 |
| P1-2 | `acquisition_hub_test.go` 新增 `TestAcquisitionHub_Concurrent_SetOnSnapshot_DuringPublish` | 启动 `StartPublishing`，并发 `SetOnSnapshot` 切换回调，1s 后 cancel | R1 回归 |
| P1-3 | `motion_manager_test.go` 新增 `TestMotionManager_StartPolling_Concurrent_StartStop` | 并发调用 `StartPolling()`×10 + `StopPolling()`×10，断言不 panic、不重复启动 | R3 回归 |
| P1-4 | `motion_manager_test.go` 新增 `TestMotionManager_PollStatus_During_Connect_Remove` | 启动 `StartPolling`，并发 `Connect`/`RemoveProfile`/`GetStatusAll` | R4 |
| P1-5 | `tcp_base_test.go` 新增 `TestTCPDriverBase_Concurrent_SetDataCallback_And_EmitData` | 并发 `SetDataCallback` + `EmitData` | R2 回归 |
| P1-6 | `three_hole/test_manager_test.go` 新增 `TestTestManager_Concurrent_Start_Pause_Stop` | 并发 `Start`/`Pause`/`Resume`/`Stop`，断言状态机一致性 | R5 |

> 编写规范：
> - 并发测试用 `sync.WaitGroup` 同步，运行时长用 `time.After` 控制（建议 200ms-1s）。
> - 必须用 `-race` 在本地验证通过后再提交。
> - 不要在测试中 `time.Sleep` 等待固定时长做同步（flaky），用 channel 或 `Eventually` 模式。

### 阶段 P2：本地 Taskfile 任务

在 `src/Taskfile.yml` 新增任务（与现有 `build`/`package` 风格一致）：

```yaml
  test:
    summary: Run Go unit tests
    dir: "{{.ROOT_DIR}}"
    cmds:
      - go test ./internal/...

  test:race:
    summary: Run Go tests with race detector (slow, ~30s)
    dir: "{{.ROOT_DIR}}"
    cmds:
      - go test -race -count=1 ./internal/...

  test:verbose:
    summary: Run Go tests with verbose output
    dir: "{{.ROOT_DIR}}"
    cmds:
      - go test -v -race -count=1 ./internal/...
```

> 注：`-count=1` 禁用测试结果缓存，确保每次都真跑。
> `dir: "{{.ROOT_DIR}}"` 即 `src/`（Taskfile 所在目录），与 `wails3 task build` 工作目录一致。

### 阶段 P3：CI 工作流

> 项目无现成 CI，本阶段同时建立最小可用的 GitHub Actions。

新建 `.github/workflows/go-test.yml`：

```yaml
name: Go Tests

on:
  push:
    branches: [main, master]
  pull_request:
    branches: [main, master]
  # 支持手动触发，便于排查
  workflow_dispatch:

jobs:
  race-test:
    runs-on: windows-latest   # 项目为 Windows-only Wails 应用
    defaults:
      run:
        working-directory: src
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.23'
          cache: true
          cache-dependency-path: src/go.sum

      - name: Go vet
        run: go vet ./internal/...

      - name: Build check
        run: go build ./...

      - name: Unit tests
        run: go test -count=1 ./internal/...

      - name: Race detector tests
        run: go test -race -count=1 ./internal/...
        # race detector 较慢，单独 step 便于在 Actions 日志中定位
```

> 决策说明：
> - **为何选 `windows-latest`**：AGENTS.md 明确"Windows-only"，驱动代码（`b140.go`/`xy_daq16.go`）依赖 Windows 网络栈行为，模拟器测试在 Windows 上跑更贴近生产。
> - **为何不跑 `wails3 task build`**：CI 仅验证后端逻辑，避免 NSIS/WebView2 等重依赖。
> - **为何不跑前端 `npm run test`**：本计划仅针对 race detector；前端 CI 后续单独规划。
> - **缓存策略**：`actions/setup-go` 的 `cache: true` 会按 `go.sum` 缓存模块，二次运行提速明显。

### 阶段 P4：文档与规范固化

| ID | 任务 | 文件 |
|----|------|------|
| P4-1 | 更新 `AGENTS.md` 的 Commands 表，加入 `wails3 task test` / `test:race` | `AGENTS.md` |
| P4-2 | 更新 `docs/engineering/coding-standards.md`，新增"并发规范"小节：回调字段必须加锁、goroutine 取消统一用 `context.Context`、状态标志统一用 `atomic.Bool` | `docs/engineering/coding-standards.md` |
| P4-3 | 在 `project_memory.md` 的"Engineering Conventions"补一条：所有回调字段（`onXxx`）必须用 `mu` 保护读写 | memory（用户侧） |

---

## 5. 验收标准

- [ ] P0 全部完成：`go test -race -count=1 ./internal/...` 在本地零 race 报告、零失败。
- [ ] P1 全部完成：6 个并发测试用例存在且通过。
- [ ] P2 完成：`wails3 task test:race` 在 `src/` 可一键执行。
- [ ] P3 完成：PR 推送后 GitHub Actions 自动跑 race 测试并通过（绿勾）。
- [ ] P4 完成：规范文档更新，新写代码有据可依。

---

## 6. 风险与回退

| 风险 | 影响 | 缓解 |
|------|------|------|
| `windows-latest` runner 慢/排队 | CI 反馈延迟 | 可降级到 `ubuntu-latest` 跑 race（仅后端逻辑测试，不依赖 Windows API），但生产仍以 Windows 验证为准 |
| three_hole 已失败测试修复难度大 | 阻塞 P0-3 | 单独排查根因，必要时回滚相关重构 commit；不阻塞 P0-1/P0-2 |
| race detector 内存占用高（10x） | 大型测试 OOM | 分包跑：`go test -race ./internal/manager/...` + `./internal/driver/...` 分步执行 |
| 修复 R1/R2 引入死锁 | 测试 hang | 严格按"先读锁再回调、回调在锁外执行"模式（参考 `emitStatusChange` 实现） |

---

## 7. 执行顺序建议

```
P0-1 → P0-2 → P1-1 → P1-2 → P1-5     # acquisition_hub + tcp_base 闭环
        ↓
P0-3 (独立分支) → P1-6                 # three_hole 闭环
        ↓
P1-3 → P1-4                            # motion_manager 闭环
        ↓
P2 → P3 → P4                           # 工程化收尾
```

P0-1/P0-2/P1-1/P1-2/P1-5 可作为一个 PR 提交；P0-3 视修复难度独立 PR；motion_manager 部分独立 PR；P2/P3/P4 作为工程化 PR。

---

## 8. 不做什么（防 scope creep）

- 不引入 `golangci-lint`（已存在独立命令，CI 接入是另一个计划）。
- 不重写 `AcquisitionHub` 架构（仅加锁修复，不改"最新帧缓存"设计）。
- 不补前端测试到 CI（前端测试体系另立计划）。
- 不增加性能 benchmark（`-bench` 后续按需）。
