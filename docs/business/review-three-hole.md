# 三孔移位测试 — 业务逻辑审查报告

## 一、严重 Bug

### 1.1 直线布点前端预览与后端严重不一致

**位置**: `frontend/src/views/ThreeHoleTestView.vue:419-428` vs `internal/three_hole/service.go:695-704`

前端 `previewPoints` 对 `line` 模式使用单层循环：

```ts
// ThreeHoleTestView.vue
const n = Math.max(xValues.length, yValues.length)
for (let i = 0; i < n; i++) {
  const x = i < xValues.length ? xValues[i] : xValues[xValues.length - 1]
  const y = i < yValues.length ? yValues[i] : yValues[yValues.length - 1]
  points.push({ x, y, state: 'pending' })
}
```

后端 `generateLinePoints` 使用**双重循环**（X/Y 全组合）：

```go
// service.go
for _, x := range xValues {
    for _, y := range yValues {
        points = append(points, types.TraversalPoint{...})
    }
}
```

**影响**: 前端预览只生成对角线上的点，后端实际执行生成全网格。用户看到的布点预览与实际执行的完全不符。

---

### 1.2 直线布点 UI 缺少步长配置，只能生成 2 个点

**位置**: `frontend/src/views/ThreeHoleTestView.vue:167-184`

直线模式的 UI 只有**起点**和**终点**输入框，没有任何步长输入入口。

后端 `generateLinePoints` 当 `xSteps` 和 `ySteps` 均为空时的行为（`service.go:680-686`）：

```go
if len(xValues) == 0 && len(yValues) == 0 {
    // 只生成起止两个点
    points = append(points, StartX, StartY)
    points = append(points, EndX, EndY)
    return points
}
```

**影响**: 直线模式实质上只能生成起点和终点 2 个数据点，中间插值布点功能完全无法使用。

---

### 1.3 矩形布点前端预览忽略多分段步长

**位置**: `frontend/src/views/ThreeHoleTestView.vue:412` vs `internal/three_hole/service.go:718-719`

前端 `previewPoints` 使用单一步长展开：

```ts
// ThreeHoleTestView.vue
const xValues = expandRange(r.xMin, r.xMax, xStep.value)
const yValues = expandRange(r.yMin, r.yMax, yStep.value)
```

后端使用 `StepSegment` 数组（支持多段）：

```go
// service.go
xValues := expandStepSegments(rect.XSteps)
yValues := expandStepSegments(rect.YSteps)
```

前端的 `expandRange` 等价于 `[{start: min, end: max, step: singleStep}]`。如果配置包含多个段（如 `[{start:-20,end:0,step:5}, {start:0,end:20,step:2}]`），后端生成更多点，但前端始终使用单个步长显示预览。

**影响**: 多段步长配置下预览不准，用户看到的点数和位置与实际执行不一致。

---

### 1.4 CSV 保存路径无 UI 配置入口

**位置**: `frontend/src/views/ThreeHoleTestView.vue` 设置弹窗

`ThreeHoleTraversalConfig.SavePath` 默认值为空字符串。设置弹窗中**没有保存路径和文件名的输入项**。

后端 CSV 初始化（`service.go:276`）：

```go
if err := csvWriter.Initialize(s.config.SavePath, s.config.SaveFileName); err != nil {
```

当 `SavePath` 为空时，`os.MkdirAll("", 0755)` 行为不可控 —— 在某些系统上失败，在某些系统上在当前工作目录创建文件。`SaveFileName` 虽有默认值，但用户无法自定义。

**影响**: CSV 写入可能静默失败，用户无法控制数据存储位置。

---

## 二、中等严重问题

### 2.1 完成事件后未清理前端 progress

**位置**: `frontend/src/stores/threeHoleTest.ts:313-316`

```ts
EventsOn('three-hole:complete', (data: ThreeHoleTraversalCompleteEvent) => {
  isRunning.value = false
  isPaused.value = false
  fetchStatus()
  // progress 未被清理，进度条仍显示上次数据的进度
})
```

`stopTest()` 中清理了 `progress` 和 `realtime`，但 complete 事件到达时未清理。用户会看到过时的进度信息。

---

### 2.2 后端 resumeCh 通道多消费者竞争

**位置**: `internal/three_hole/service.go:173,217,304,409,459`

`resumeCh` 是无缓冲通道，`Resume()` 只发出一个信号：

```go
// service.go:216-219
select {
case s.resumeCh <- struct{}{}:
default:
}
```

但 `runTestLoop` 中**3 处**同时监听 `resumeCh`：

1. 外层暂停循环（line 304）
2. `dwellWithRealtimeUpdate` 内暂停循环（line 459）
3. `runSinglePoint` 移动后暂停循环（line 409）

信号只被一个 consumer 接收，其他 consumer 依赖 `paused` atomic flag 配合 100ms 超时轮询退出。

**影响**: 不会死锁，但恢复有最多 100ms 延迟，且恢复瞬间有多个 goroutine 并发发送实时事件。

---

### 2.3 Pause / Stop 间的并发竞争

**位置**: `internal/three_hole/service.go:195-250`

`Pause()` 设置 `paused.Store(true)` 后立即返回。如果用户快速点击 Pause → Stop：

- `Stop()` 发送 `cancelCh` 信号
- `Stop()` 设置 `paused.Store(false)`
- `runTestLoop` 可能在检查 `for s.paused.Load()`、正在执行 dwell、或正在采集

---

### 2.4 Stop() 状态设置时序

**位置**: `internal/three_hole/service.go:233,343-344`

```go
// Stop() 先设 Idle
s.status.Status = types.TraversalStatusIdle  // line 233
s.mu.Unlock()
// 然后等 doneCh
select {
case <-doneCh:
case <-time.After(5 * time.Second):
}

// runTestLoop 结束时也可能设 Completed
s.status.Status = types.TraversalStatusCompleted  // line 344
```

两者锁竞争，最终 status 取决于执行时序。建议在 goroutine 退出清理后再统一设 status。

---

## 三、设计 / 体验问题

### 3.1 直线布点 UI 缺少 Offset 输入

前端设置中 `motionX.offset` 和 `motionY.offset` 没有 UI 输入，但后端 `resolveTargetPosition` 使用了它。目前 UI 只有 Axis 选择和 Scale 输入（`ThreeHoleTestView.vue:190-208`），Offset 使用默认值 0。

### 3.2 isPaused 状态在 startTest 中硬编码

```ts
// threeHoleTest.ts:223
isPaused.value = false
```

如果后端 start 失败（catch 中设 `isRunning = false`），但 `isPaused` 已提前被设为 false，实际没问题。但如果 start 失败前另一个地方依赖了 `isPaused` 的旧值，存在窗口问题。

### 3.3 前端 Error 事件的异步 fetchStatus 竞争

```ts
// threeHoleTest.ts:318-327
EventsOn('three-hole:error', (data) => {
  lastError.value = data.error
  fetchStatus().then(() => {
    if (taskStatus.value?.status === 'error') {
      isRunning.value = false  // 仅当 status===error 时停止
    }
  })
})
```

非致命错误（`emitPointError`）不改变 status，前端保持 `isRunning = true`，行为正确。但如果 fetchStatus 异步返回时状态已改变，可能导致状态不一致。

### 3.4 dpZeroThreshold 通用常量

`interpolator.go:20` — `dpZeroThreshold = 1e-6` 用于三种不同物理含义：

| 用途 | 物理量纲 | 位置 |
|------|---------|------|
| deltaP 判零 | 压力 (Pa) | `interpolator.go:137` |
| CMa 差判零 | 马赫数 | `interpolator.go:291` |
| Kb 差判零 | 无量纲 | `interpolator.go:344` |

建议按物理量纲拆分为独立常量。

### 3.5 App 关闭时未清理测试 goroutine

如果测试运行中直接关闭窗口，`runTestLoop` goroutine 无退出信号。`doneCh` 的 `select` 在 `Stop()` 中设置了 5 秒超时。但 Wails 关闭时若未调用 `Stop()`，goroutine 可能残留。

---

## 四、状态机验证

### 状态转换表

| 操作 | idle | running | paused | completed | error |
|------|------|---------|--------|-----------|-------|
| `Start()` | → running | reject | reject | → running | → running |
| `Pause()` | noop | → paused | noop | noop | noop |
| `Resume()` | noop | noop | → running | noop | noop |
| `Stop()` | noop | → idle | → idle | noop | → idle |
| 自动完成 | - | → completed | - | - | - |
| 致命错误 | - | → error | → error | - | - |

### 前端 isRunning / isPaused 一致性

| 后端状态 | idle | running | paused | completed | error |
|---------|------|---------|--------|-----------|-------|
| `isRunning` | false | true | true | false (complete 事件) | false (error 事件判断) |
| `isPaused` | false | false | true (pauseTest fetch) | false | false |

### 状态机问题

- `completed` 或 `error` 状态下调用 `Start()` 会重新启动（后端允许，前端 `isRunning` guard 阻止 UI 重复点击）
- 非致命点位错误不改变 Status，仅更新 `LastError` + 发送 error 事件，`isRunning` 保持 true ✓
- `Stop()` 对 `completed` 状态 noop：但前端此时 `isRunning=false`，用户无法点击 stop ✓

---

## 五、数据流检查

```
用户 UI (Vue) → Wails Binding → App.go → Service → Interpolator + CSV Writer
                   ↑                               ↓
                   └──── Wails EventsEmit ← Publisher ←┘
```

### 事件通道

| 事件名 | 触发时机 | 接收端 | 频率 |
|-------|---------|-------|------|
| `three-hole:progress` | 每点完成 | 进度条 + 布点预览 | 逐点 |
| `three-hole:realtime` | 每 100ms | 实时数据面板 + 波形 | 100ms |
| `three-hole:complete` | 全部完成 | 状态重置 | 一次 |
| `three-hole:error` | 任何错误 | 错误提示 | 不定 |

### 并发路径

| Goroutine | 访问资源 | 同步方式 |
|-----------|---------|---------|
| `runTestLoop` | status, config, csv, paused | sync.Mutex + atomic.Bool |
| `runRealtimeMonitor` | config, batchGetter | monitorRunning guard |
| `runSinglePoint` | config, motionCtrl, paused | cancelCh + atomic.Bool |
| `dwellWithRealtimeUpdate` | config, eventPublisher | cancelCh + resumeCh |

---

## 六、总结优先级

| 级别 | 描述 | 数量 |
|------|------|------|
| **P0 - 严重** | 直线模式不可用、预览一致性问题、保存路径缺失 | 4 |
| **P1 - 中等** | 并发竞争、状态清理缺失、通道竞争延迟 | 4 |
| **P2 - 体验** | UI 配置缺失、常量拆分、关闭清理 | 4 |
