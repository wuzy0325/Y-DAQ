# Spec: 五孔插值移位测试 —— 布点轴配置去重

> 关联 intent: [docs/intent/five-hole-traversal-axis-cleanup.md](../intent/five-hole-traversal-axis-cleanup.md)

## 假设（Surface Assumptions）

写 spec 前先列出我做的假设，若有错请立即指出：

1. **`TraversalLayout` 是三孔/五孔共享类型**（`src/internal/types/three_hole_traversal.go:140-147`），不能删字段（intent 明确"不改三孔"）。五孔侧通过"不读取 / 不显示 / 不校验"实现去重，Go/TS 类型字段全部保留。
2. **旧配置文件兼容**：用户本地 `~/.yx-daq/` 下的旧配置 JSON 里 `layout.line.axis` / `layout.rectangle.xAxis` 等字段仍存在，加载时不报错、不清理（字段在类型里保留，只是五孔不再使用）。无需写迁移逻辑。
3. **直线模式 `point.Y = 0`**：删除 Fixed 控件后，直线布点生成的 `TraversalPoint.Y` 恒为 0。`motion_coordinator` 直线模式只发 `MotionX` 指令，不碰 `MotionY`。CSV 仍记录 Y=0 + MotionY 的 ControllerID/Axis（表示"配置了但本次未运动"）。
4. **扇面线性/旋转轴校验位置**：`Validate()` 只持有 config、无法访问 MotionService 的 profile。该校验放到 `FiveHoleTraversalService.Start()` 里做（Start 可访问 MotionService，能拿到 profile 的 `AxisConfig.Kind`）。`AxisConfig.Kind` 字段已存在（`AxisKindLinear`/`AxisKindRotary`，见 `src/internal/types/motion.go:111-114`）。
5. **前端 TS 类型保留字段**：`LineLayout.axis` / `RectangleLayout.xAxis/yAxis` / `FanLayout.rAxis/thetaAxis` 在 TS 类型里保留（可选填），仅 UI 隐藏。不写迁移逻辑。

→ 若以上假设有误，请指出后再进入 review。

---

## Objective

### 问题
五孔测试配置「布点」卡片与每根探针配置都包含「物理轴选择」字段，功能重复且冲突。
历史原因：五孔复用三孔 `TraversalLayout`，layout 里带轴名字段（`Line.Axis` / `Rectangle.XAxis,YAxis` / `Fan.RAxis,ThetaAxis`），但五孔每根探针又有独立的 `MotionX/MotionY`，导致同一件事在两处配置，用户容易配错。

### 目标
删除测试配置侧的所有物理轴选择字段，物理轴完全由每根探针自己的 `MotionX/MotionY` 决定。
- 测试配置只管「测哪些点」（坐标范围 / 步长）
- 每根探针只管「用哪根轴去走」
- 两者职责分离，不再有重复的轴选择控件

### 用户故事
作为五孔测试操作者，我希望在每根探针的配置里指定它用哪根轴，而布点配置只关心坐标网格，这样我不会在两个地方配同一件事还配冲突。

## Tech Stack

- 后端：Go 1.23，`log/slog`，`yx-daq/internal/types`、`yx-daq/internal/five_hole`
- 前端：Vue 3 + TypeScript + Element Plus + Pinia
- 测试：Go `testing`（含 `-race`），Vitest（`happy-dom`）
- 构建：Wails v3

## Commands

```bat
# Go 编译检查
cd src && go build ./...

# Go 测试（含 race）
cd src && wails3 task test:race

# Go 测试（verbose）
cd src && wails3 task test:verbose

# 前端类型检查 + 构建
cd src/frontend && npm run build

# 前端 lint
cd src/frontend && npm run lint

# 前端测试
cd src/frontend && npm run test

# 开发模式
cd src && wails3 dev
```

## Project Structure

改动涉及文件：

**后端（Go）**
- `src/internal/types/five_hole_traversal.go` — `Validate()` 删除轴名校验、调整直线/矩形/扇面校验
- `src/internal/types/three_hole_traversal.go` — **不改**（保护三孔共享类型）
- `src/internal/five_hole/motion_coordinator.go` — `buildMoveTasks` 改为强制对应，删 `findProbeAxisMapping*`，`PrePositionStationaryAxis` 直线模式改 no-op
- `src/internal/five_hole/point_generator.go` — 直线模式不再读 `line.Axis`，`lineAxisIsX` 删除，`point.Y=0`
- `src/internal/five_hole/service.go` — `Start()` 新增扇面线性/旋转轴校验

**前端（TS/Vue）**
- `src/frontend/src/stores/fiveHoleTest/types.ts` — 字段保留，类型不变
- `src/frontend/src/views/FiveHoleTestView.vue` — 删除轴选择控件、Fixed 控件；探针配置标签动态化
- `src/frontend/src/stores/fiveHoleTest.ts` — 若有默认值/校验逻辑同步调整

**测试**
- `src/internal/five_hole/*_test.go` — motion_coordinator / point_generator 单元测试更新
- `src/frontend/src/stores/fiveHoleTest/__tests__/` — 若有相关测试同步更新

## Code Style

### 后端（Go）
- 错误包装用 `%w`，日志用 `log/slog`（结构化：`slog.Error("msg", "err", err)`）
- 并发安全：回调字段读写用 mu 保护
- 错误收集合并返回，不仅返回首个

### 前端（TS/Vue）
- `<style lang="scss" scoped>`
- Store/View 静态 import `@bindings/yx-daq/internal/app`
- 组件卸载时清理定时器 / 事件监听

### 改动示例（motion_coordinator 核心改造）

```go
// buildMoveTasks 改造后：强制对应，不再反查轴名
func buildMoveTasks(point types.TraversalPoint, probes []types.FiveHoleProbeConfig, layout types.TraversalLayout) ([]moveTask, error) {
    tasks := make([]moveTask, 0, len(probes)*2)
    switch layout.Pattern {
    case types.TraversalPatternLine:
        // 直线模式：只用 MotionX，target=point.X
        for _, probe := range probes {
            if !probe.Enabled { continue }
            tasks = append(tasks, moveTask{
                probeID: probe.ProbeID, direction: "X",
                controllerID: probe.MotionX.ControllerID, axis: probe.MotionX.Axis,
                target: point.X,
            })
        }
    case types.TraversalPatternRectangle:
        // 矩形：point.X→MotionX, point.Y→MotionY（强制对应）
        for _, probe := range probes {
            if !probe.Enabled { continue }
            tasks = append(tasks, moveTask{probeID: probe.ProbeID, direction: "X",
                controllerID: probe.MotionX.ControllerID, axis: probe.MotionX.Axis, target: point.X})
            tasks = append(tasks, moveTask{probeID: probe.ProbeID, direction: "Y",
                controllerID: probe.MotionY.ControllerID, axis: probe.MotionY.Axis, target: point.Y})
        }
    case types.TraversalPatternFan:
        // 扇面：R→MotionX, θ→MotionY（强制对应）
        dr := math.Sqrt(point.X*point.X + point.Y*point.Y)
        rTarget := dr + layout.Fan.RStart
        thetaRel := math.Atan2(point.Y, point.X) * 180 / math.Pi
        thetaTarget := thetaRel + layout.Fan.ThetaStart
        for _, probe := range probes {
            if !probe.Enabled { continue }
            tasks = append(tasks, moveTask{probeID: probe.ProbeID, direction: "R",
                controllerID: probe.MotionX.ControllerID, axis: probe.MotionX.Axis, target: rTarget})
            tasks = append(tasks, moveTask{probeID: probe.ProbeID, direction: "θ",
                controllerID: probe.MotionY.ControllerID, axis: probe.MotionY.Axis, target: thetaTarget})
        }
    case types.TraversalPatternCustom:
        // 自定义：保持现状（直接用 MotionX/MotionY）
        for _, probe := range probes {
            if !probe.Enabled { continue }
            tasks = append(tasks, moveTask{probeID: probe.ProbeID, direction: "X",
                controllerID: probe.MotionX.ControllerID, axis: probe.MotionX.Axis, target: point.X})
            tasks = append(tasks, moveTask{probeID: probe.ProbeID, direction: "Y",
                controllerID: probe.MotionY.ControllerID, axis: probe.MotionY.Axis, target: point.Y})
        }
    }
    return tasks, nil
}
```

## 详细改动清单

### 1. 后端 — `motion_coordinator.go`

| 函数 | 改动 |
|------|------|
| `buildMoveTasks` rectangle | 删除读 `layout.Rectangle.XAxis/YAxis`，改为强制 `point.X→probe.MotionX`、`point.Y→probe.MotionY` |
| `buildMoveTasks` line | 删除读 `layout.Line.Axis`，改为只用 `probe.MotionX`，`target=point.X` |
| `buildMoveTasks` fan | 删除读 `layout.Fan.RAxis/ThetaAxis`，改为 `R→probe.MotionX`、`θ→probe.MotionY` |
| `findProbeAxisMapping` | **删除** |
| `findProbeAxisMappingWithDirection` | **删除** |
| `PrePositionStationaryAxis` | 直线模式改为 `return nil`（无静止轴），函数体简化或删除 |
| `ReturnProbesToInitialPositions` | **不改**（已直接用 MotionX/MotionY） |
| 类型注释 | 更新 `MotionCoordinator` 顶部注释，删除"由本协调器根据轴名反查"描述 |

### 2. 后端 — `point_generator.go`

| 函数 | 改动 |
|------|------|
| `generateLinePoints` | 不再调 `lineAxisIsX`；生成 `point{X: v, Y: 0}` |
| `lineAxisIsX` | **删除** |
| `generateRectanglePoints` | **不改**（本就不读 XAxis/YAxis） |
| `generateFanPoints` | **不改**（本就不读 RAxis/ThetaAxis） |
| 函数注释 | 删除"line.Axis 是物理轴名"相关描述 |

### 3. 后端 — `five_hole_traversal.go` Validate

| 校验项 | 改动 |
|--------|------|
| 直线 `validateFiveHoleLineAxis(c.Layout.Line.Axis, ...)` | **删除** |
| 矩形 `XAxis/YAxis == ""` 检查 | **删除** |
| 矩形 `validateFiveHoleLineAxis(XAxis/YAxis, ...)` | **删除** |
| 扇面 `RAxis/ThetaAxis == ""` 检查 | **删除** |
| 扇面 `RAxis == ThetaAxis` 检查 | **删除** |
| 扇面 `validateFiveHoleLineAxis(RAxis/ThetaAxis, ...)` | **删除** |
| `validateFiveHoleLineAxis` 函数 | **删除**（无人调用） |
| 矩形新增 | 每根启用探针 `MotionX` / `MotionY` 的 `(ControllerID, Axis)` 不能完全相同 |
| 直线新增 | （已有）每根启用探针 `MotionX` 必须配置 |
| `validateLineLayout` 调用 | **保留**（Start/End/Step 校验仍有用）；但其中的 `line.Axis == ""` 检查对五孔不再适用 → 五孔 Validate 里跳过 `validateLineLayout`，改为五孔自己校验 Start/End/Step |

### 4. 后端 — `service.go` Start

新增扇面模式线性/旋转轴校验（在 `config.Validate()` 之后）：
```go
if config.Layout.Pattern == types.TraversalPatternFan {
    if err := s.validateFanAxisKinds(config); err != nil {
        return "", fmt.Errorf("扇面轴类型校验失败: %w", err)
    }
}
```
`validateFanAxisKinds` 通过 MotionService 获取每根启用探针 `MotionX.ControllerID` 对应 profile 的 `AxisConfig.Kind`，校验 `MotionX` 必须为 `AxisKindLinear`、`MotionY` 必须为 `AxisKindRotary`。

### 5. 前端 — `FiveHoleTestView.vue`

**测试配置布点卡片**：
- 直线模式：删除「移动轴」下拉、删除「固定坐标」输入框；保留 起点/终点/步长
- 矩形模式：删除「X 方向轴」「Y 方向轴」下拉；保留 XMin/XMax/YMin/YMax/XSteps/YSteps
- 扇面模式：删除「半径轴」「角度轴」下拉；保留 RStart/RSteps/ThetaStart/ThetaSteps

**探针配置区**（MotionX/MotionY 表单）：
- 新增 computed `motionXLabel` / `motionYLabel`，根据 `config.layout.pattern` 返回：
  - `line`：MotionX 标签 =「移动轴」，MotionY 表单项 `v-if` 隐藏
  - `rectangle`：保持当前标签
  - `fan`：MotionX 标签 =「R方向」，MotionY 标签 =「θ方向」
  - `custom`：保持当前标签
- MotionY 字段在直线模式下 UI 隐藏，但数据保留（切换模式时仍可用）

### 6. 前端 — `fiveHoleTest.ts` / `types.ts`

- `types.ts`：**不改**（字段保留）
- `fiveHoleTest.ts`：检查是否有默认值初始化或前端校验引用了待删字段，若有则移除

## Testing Strategy

### Go 单元测试
- `motion_coordinator_test.go`：更新 `buildMoveTasks` 测试用例
  - 直线：断言只生成 X 方向 task，target=point.X，无 Y task
  - 矩形：断言 point.X→MotionX、point.Y→MotionY（不再依赖 layout.XAxis/YAxis）
  - 扇面：断言 R→MotionX、θ→MotionY
  - 删除涉及 `findProbeAxisMapping` 的测试
- `point_generator_test.go`：更新直线测试
  - 断言 `point.Y == 0`
  - 删除 `lineAxisIsX` 测试
- `five_hole_traversal_test.go`（若存在）：更新 Validate 测试
  - 删除 axis 字段非空的校验用例
  - 新增矩形 MotionX/MotionY 相同的校验用例
- 并发测试 `TestBuildMoveTasks_Concurrent_*` 须在 `go test -race` 下通过

### 前端测试
- 若 `FiveHoleTestView` 有组件测试：更新断言，确认轴选择控件不再渲染、Fixed 控件不渲染
- 若 store 有校验测试：同步更新

### 手动验证
- `wails3 dev` 启动，切换 line/rectangle/fan 模式，确认：
  - 布点卡片无轴选择下拉、直线无 Fixed 输入
  - 探针配置标签随模式变化
  - 直线模式探针配置无 MotionY 项
- 运行一次完整五孔测试（模拟设备 + 模拟运动），确认运动指令正确发送

## Boundaries

- **Always**:
  - 改动后跑 `go build ./...` + `wails3 task test:race` + `npm run build`
  - 错误收集合并返回
  - 并发安全用 mu 保护
  - 前端组件卸载清理定时器/事件
- **Ask first**:
  - 若发现 `validateLineLayout` 被三孔依赖且五孔改动会影响三孔 → 停下确认
  - 若 `service.go` 的 `validateFanAxisKinds` 需要 MotionService 暴露新接口 → 确认接口设计
- **Never**:
  - 改 `src/internal/types/three_hole_traversal.go` 的类型定义（三孔共享类型）
  - 改三孔模块任何代码
  - 改 `FiveHoleProbeConfig` 数据结构（MotionX/MotionY 字段名保留）
  - 改 CSV 导出格式
  - 手动编辑 `src/frontend/bindings/`（Wails 自动生成）

## Success Criteria

1. 测试配置「布点」卡片在 line/rectangle/fan 三种模式下**均无物理轴选择控件**，直线模式无「固定坐标」控件
2. 探针配置的 MotionX/MotionY 标签随布点模式动态变化（直线：移动轴/隐藏Y；矩形：保持；扇面：R方向/θ方向）
3. `motion_coordinator.buildMoveTasks` 不再读取 `layout.Line.Axis` / `Rectangle.XAxis,YAxis` / `Fan.RAxis,ThetaAxis`，强制 `point.X→MotionX`、`point.Y→MotionY`（扇面 R→MotionX、θ→MotionY）
4. `findProbeAxisMapping` / `findProbeAxisMappingWithDirection` / `lineAxisIsX` / `validateFiveHoleLineAxis` 四个函数已删除
5. `PrePositionStationaryAxis` 直线模式直接返回 nil（无静止轴预定位）
6. `Validate()` 不再校验轴名字段；矩形模式新增 MotionX/MotionY 不可完全相同校验
7. 扇面模式在 `service.Start()` 校验每根启用探针 MotionX=线性轴、MotionY=旋转轴
8. 旧配置文件（含残留 axis 字段）加载不报错
9. `go build ./...` + `wails3 task test:race` + `npm run build` + `npm run lint` 全部通过
10. 手动运行五孔测试，直线/矩形/扇面三种模式运动指令正确

## Out of Scope

- 不改 `FiveHoleProbeConfig` 数据结构（MotionX/MotionY 字段名保留）
- 不改 `TraversalLayout` / `LineLayout` / `RectangleLayout` / `FanLayout` 的 Go/TS 类型字段定义（保护三孔共享类型）
- 不改 CSV 导出格式（仍按 X/Y 方向导出 ControllerID/Axis；直线模式 Y=0）
- 不清理 `service.go` 里 α/β 的陈旧注释（独立小瑕疵）
- 不改三孔模块
- 不写旧配置迁移逻辑（字段保留，无需清理）

## Open Questions（已在 Plan 阶段解决）

1. **`validateLineLayout` 复用问题** → **已解决**：五孔 Validate 不再调 `validateLineLayout`，改为五孔自己内联校验 `line.Start/End/Step`（Step>0）。`validateLineLayout` 保留给三孔使用，不动。
2. **扇面校验接口** → **已解决**：`MotionService` 有 `GetMotionProfiles() []types.MotionControllerProfile`（`service_motion.go:38`），返回全部 profiles。FiveHoleService 用 setter 注入能力接口模式（不直接持有 MotionService）。新增 `SetAxisKindGetter(func(controllerID string, axis types.AxisName) (types.AxisKind, bool))` setter，在 app 层注入实现（内部调 `GetMotionProfiles()` 查找）。遵循项目"命名能力接口"约定。
3. **前端直线模式 MotionY 隐藏后的数据** → **已解决**：MotionY 字段 UI 隐藏但数据保留，切换模式不丢配置。用 `v-if` 控制显隐，不清空数据。

---

## Phase 2: Plan（技术实现计划）

### 组件依赖图

```
Task 1: point_generator.go ─────────────────┐
                                            ↓
Task 2: motion_coordinator.go ───────────→ Task 3: Validate ──→ Task 4: service.go 扇面校验 ──→ Task 5: app 注入
                                            ↑
Task 6: 前端 FiveHoleTestView.vue ──→ Task 7: 前端 store
                                            ↓
                                       Task 8: 集成验证
```

### 实现顺序

| 顺序 | Task | 依赖 | 可并行 |
|------|------|------|--------|
| 1 | point_generator.go | 无 | 与 Task 2 并行 |
| 2 | motion_coordinator.go | 无 | 与 Task 1 并行 |
| 3 | Validate 改造 | Task 1（约定 point.Y=0） | — |
| 4 | service.go 扇面校验 | Task 3 | — |
| 5 | app 层注入 AxisKindGetter | Task 4 | — |
| 6 | 前端 UI 改造 | 无（后端字段不变） | 与 Task 7 并行 |
| 7 | 前端 store 同步 | 无 | 与 Task 6 并行 |
| 8 | 集成验证 | 全部 | — |

### 风险与缓解

| 风险 | 缓解 |
|------|------|
| `validateLineLayout` 被三孔依赖，五孔改动误伤三孔 | 五孔 Validate 不再调它，函数本身不动；`go test ./internal/types/` 验证三孔不受影响 |
| `motion_and_data_test.go` 有 5 个 coordinator 测试需大改 | 先看测试内容再改，保持测试覆盖度不降 |
| 扇面 `AxisKind` 常量名未确认 | 已确认 `AxisKindLinear`/`AxisKindRotary` 存在（`motion.go:111-114`） |
| 前端 `FiveHoleTestView.vue` 行数多，改动范围大 | 先 Grep 定位轴选择控件和探针配置区域，精准修改 |

### 验证检查点

- **检查点 A**（Task 1-2 完成后）：`go build ./...` + `go test ./internal/five_hole/`
- **检查点 B**（Task 3-5 完成后）：`go build ./...` + `wails3 task test:race`
- **检查点 C**（Task 6-7 完成后）：`npm run build` + `npm run lint`
- **检查点 D**（Task 8）：全部命令 + 手动验证

---

## Phase 3: Tasks

### Task 1: point_generator.go 直线模式改造

- [ ] **描述**：直线布点不再读 `line.Axis`，生成 `point{X: v, Y: 0}`；删除 `lineAxisIsX`
- **Acceptance**:
  - `generateLinePoints` 生成 `TraversalPoint{X: v, Y: 0}`
  - `lineAxisIsX` 函数删除
  - `generateLinePoints` 不再接收 `probes` 参数（或保留但不使用，看签名最小改动）
- **Verify**: `cd src && go build ./... && go test ./internal/five_hole/ -run TestGenerate`
- **Files**: `src/internal/five_hole/point_generator.go`, `src/internal/five_hole/point_generator_test.go`

### Task 2: motion_coordinator.go 强制对应改造

- [ ] **描述**：`buildMoveTasks` 改为强制 `point.X→MotionX`、`point.Y→MotionY`（扇面 R→MotionX、θ→MotionY）；删除 `findProbeAxisMapping` / `findProbeAxisMappingWithDirection`；`PrePositionStationaryAxis` 直线模式 `return nil`
- **Acceptance**:
  - 矩形/扇面/自定义：直接用 `probe.MotionX/MotionY`，不调 `findProbeAxisMapping`
  - 直线：只用 `probe.MotionX`，target=point.X
  - `findProbeAxisMapping` / `findProbeAxisMappingWithDirection` 删除
  - `PrePositionStationaryAxis` 直线模式直接返回 nil
  - `ReturnProbesToInitialPositions` 不变
- **Verify**: `cd src && go build ./... && wails3 task test:race`
- **Files**: `src/internal/five_hole/motion_coordinator.go`, `src/internal/five_hole/motion_and_data_test.go`

### Task 3: Validate 改造

- [ ] **描述**：`FiveHoleTraversalConfig.Validate()` 删除所有轴名字段校验，删除 `validateFiveHoleLineAxis`；五孔自己内联校验直线 Start/End/Step；矩形新增 MotionX/MotionY 不可完全相同校验
- **Acceptance**:
  - 不再校验 `Line.Axis` / `Rectangle.XAxis,YAxis` / `Fan.RAxis,ThetaAxis`
  - `validateFiveHoleLineAxis` 函数删除
  - 直线模式内联校验 `line != nil && line.Step > 0`
  - 矩形模式校验每根启用探针 `(MotionX.ControllerID, MotionX.Axis) != (MotionY.ControllerID, MotionY.Axis)`
- **Verify**: `cd src && go build ./... && go test ./internal/types/ ./internal/five_hole/`
- **Files**: `src/internal/types/five_hole_traversal.go`, `src/internal/five_hole/service_test.go`（若有 Validate 测试）

### Task 4: service.go 扇面轴类型校验

- [ ] **描述**：新增 `SetAxisKindGetter` setter + `validateFanAxisKinds` 方法，在 `Start()` 里 `config.Validate()` 之后调用
- **Acceptance**:
  - 新增 `FiveHoleAxisKindGetter` 接口类型 `func(controllerID string, axis types.AxisName) (types.AxisKind, bool)`
  - `SetAxisKindGetter` setter 方法
  - `validateFanAxisKinds` 校验每根启用探针 MotionX=线性轴、MotionY=旋转轴
  - `Start()` 在 Validate 后、测试启动前调用该校验
  - getter 未设置时跳过校验（与 positionGetter 一致的 nil 兜底）
- **Verify**: `cd src && go build ./... && go test ./internal/five_hole/`
- **Files**: `src/internal/five_hole/service.go`, `src/internal/five_hole/service_test.go`

### Task 5: app 层注入 AxisKindGetter

- [ ] **描述**：在 `Core` 或 `CoreService` 装配处调用 `fiveHoleService.SetAxisKindGetter(...)`，实现从 `MotionService.GetMotionProfiles()` 查找轴类型
- **Acceptance**:
  - 注入的 getter 闭包：遍历 `MotionService.GetMotionProfiles()`，匹配 controllerID，再匹配 axis name，返回 `AxisConfig.Kind`
  - `go build ./...` 通过
- **Verify**: `cd src && go build ./...`
- **Files**: `src/internal/app/core.go` 或 `src/internal/app/service_five_hole.go`（装配处）

### Task 6: 前端 FiveHoleTestView.vue UI 改造

- [ ] **描述**：删除布点卡片轴选择控件 + 直线 Fixed 控件；探针配置 MotionX/MotionY 标签随 pattern 动态化
- **Acceptance**:
  - 直线模式：无移动轴下拉、无 Fixed 输入；探针配置 MotionX 标签=「移动轴」，MotionY 项 `v-if` 隐藏
  - 矩形模式：无 XAxis/YAxis 下拉
  - 扇面模式：无 RAxis/ThetaAxis 下拉；探针配置 MotionX 标签=「R方向」，MotionY=「θ方向」
  - 新增 `motionXLabel` / `motionYLabel` computed
- **Verify**: `cd src/frontend && npm run build && npm run lint`
- **Files**: `src/frontend/src/views/FiveHoleTestView.vue`

### Task 7: 前端 store 同步

- [ ] **描述**：检查 `fiveHoleTest.ts` 是否有默认值初始化或前端校验引用待删字段，若有则移除
- **Acceptance**:
  - 无引用 `layout.line.axis` / `layout.rectangle.xAxis` 等待删字段的校验逻辑
  - 默认值初始化不依赖这些字段
- **Verify**: `cd src/frontend && npm run build && npm run test`
- **Files**: `src/frontend/src/stores/fiveHoleTest.ts`

### Task 8: 集成验证

- [ ] **描述**：全量构建 + 测试 + 手动验证
- **Acceptance**:
  - `go build ./...` 通过
  - `wails3 task test:race` 通过
  - `npm run build` + `npm run lint` 通过
  - 手动：`wails3 dev`，切换 line/rectangle/fan，确认 UI 和运动指令正确
- **Verify**: 全部命令 + 手动
- **Files**: 无（验证 only）
