# Spec: 五孔插值移位测试模块

> 本 spec 基于 interview-me 访谈结论（15 轮问答）+ 三孔模块代码分析撰写。
> 三孔模块为本模块的架构蓝本：`internal/three_hole/` + `internal/types/three_hole_traversal.go` + `frontend/src/views/ThreeHoleTestView.vue`。
> `internal/calibration/` 现有"五孔...当前隐藏"代码不沿用，那是另一套设计。

## Objective

新增五孔插值移位测试模块，支持 **1-3 根五孔探针** 协同作业：

- 3 根探针共享同一套布点坐标（网格拓扑共享，各自坐标系独立），同步移动、同步驻留、同步采样
- 统一控制（开始/暂停/停止/恢复），一个任务驱动最多 6 轴并行移动 + 最多 3 套采集设备并行采样
- 每根探针独立载入 `.prb` 校准文件、独立插值、独立 CSV 导出
- 单界面三栏并排实时显示：每栏 = 实时压力数值卡（P1-P5+P∞+T∞）+ 插值结果数值卡（α/β/Ma/V/Pt/Ps）
- 顶部统一进度条（每点等最慢探针完成才前进）+ 单 Canvas 共享布点预览（整体状态着色）
- 插值算法占位骨架（输入 P1-P5+PAtm+TAtm，输出 α/β/Ma/V/Pt/Ps），待用户提供具体算法

**用户**：流场试验操作人员，用五孔探针测量流场剖面（攻角 α、侧滑角 β、马赫数、速度、总压、静压）。

**为何现在**：三孔模块已成熟（多探针、布点、生命周期、CSV、实时显示），五孔是其自然延伸（多一个 β 维度），需在同一套 Wails v3 框架内支持。

## Tech Stack

与三孔模块完全一致：

| 层 | 技术 |
|----|------|
| 框架 | Wails v3 alpha |
| 后端 | Go 1.23，`log/slog`，`encoding/csv` |
| 前端 | Vue 3 + TypeScript + Vite 3 + Element Plus + Pinia + vue-router (hash) |
| 图表 | 本模块**不使用** ECharts（无波形图，见决策 #11） |
| 配置持久化 | `~/.yx-daq/` 目录 JSON |

## Commands

| What | How |
|------|-----|
| Dev mode | `wails3 dev` |
| Build exe | `wails3 task build` |
| Build bindings | `wails3 generate bindings -clean=true -ts` |
| Go compile check | `go build ./...` |
| Go linter | `golangci-lint run ./internal/...` |
| Go tests | `go test ./internal/five_hole/... ./internal/types/...` |
| Frontend typecheck + build | `cd frontend && npm run build` |
| Frontend lint | `cd frontend && npm run lint` |

**关键约束**：修改 Go 符号前必须先用 GitNexus MCP 跑 impact 分析；Wails 绑定文件 `frontend/bindings/` 禁止手改；修改后端后必须 `wails3 generate bindings` 重新生成绑定。

## Project Structure

### 新增后端文件

```
yx-daq/internal/
├── five_hole/                          # 五孔移位测试服务（照 three_hole 组织）
│   ├── service.go                      # FiveHoleTraversalService：测试生命周期 + 主循环
│   ├── test_manager.go                 # 状态管理、暂停/停止协调、统一进度发射
│   ├── data_processor.go                # 多探针并行采集 + 3σ 滤波 + 实时监控协程
│   ├── interpolator.go                  # 五孔插值器（占位骨架，Calculate 返回 NotImplemented）
│   ├── point_generator.go               # 布点坐标生成（直接复用三孔逻辑或共享）
│   ├── event_handler.go                 # 事件发射路由 + CSV 写入编排
│   ├── csv_writer.go                    # 结果 CSV 导出（每探针一文件，加 β 列）
│   ├── realtime_recorder.go             # 实时数据录制
│   ├── motion_coordinator.go            # 【新增】多探针同步运动协调器（6 轴并行移动 + 等待最慢）
│   ├── *_test.go                        # 单元测试（照三孔测试套件）
│   └── types_local.go                   # 包内私有类型（如函数类型 BatchGetter/MotionController）
├── types/
│   └── five_hole_traversal.go           # 五孔全部类型定义（照 three_hole_traversal.go）
└── app/
    ├── service_five_hole.go             # FiveHoleService：Wails 绑定层
    └── core.go                           # 【修改】注册 FiveHoleServices map[string]*five_hole.FiveHoleTraversalService
```

### 新增前端文件

```
yx-daq/frontend/src/
├── views/
│   └── FiveHoleTestView.vue             # 主视图（共享工具栏 + 单 Canvas + 三栏并排）
├── stores/
│   ├── fiveHoleTest.ts                  # Pinia store（状态、API 调用、事件监听、配置持久化）
│   └── fiveHoleTest/
│       └── types.ts                     # store 类型
├── api/
│   └── enums.ts                         # 【修改】新增 FiveHoleChannelRole、复用 TraversalPattern/StepSegment/AxisName
└── wails-compat/
    └── app.ts / services.ts             # 【修改】新增 StartFiveHoleTraversal 等 Wails v3 绑定映射
```

### 配置示例

```
yx-daq/bin/config/
└── five_hole_config.json                # 配置示例（全局 + 3 探针）
```

## 核心数据模型

### 通道角色（`five_hole_traversal.go`）

```go
type FiveHoleChannelRole string

const (
    Role5H_P1   FiveHoleChannelRole = "fiveHole.p1"   // 1号孔压力
    Role5H_P2   FiveHoleChannelRole = "fiveHole.p2"   // 2号孔压力（中心孔）
    Role5H_P3   FiveHoleChannelRole = "fiveHole.p3"   // 3号孔压力
    Role5H_P4   FiveHoleChannelRole = "fiveHole.p4"   // 4号孔压力
    Role5H_P5   FiveHoleChannelRole = "fiveHole.p5"   // 5号孔压力
    Role5H_PAtm FiveHoleChannelRole = "fiveHole.pAtm" // 大气压（全局共享）
    Role5H_TAtm FiveHoleChannelRole = "fiveHole.tAtm" // 大气温度（全局共享）
)
```

### 通道配置（每通道独立指定设备 + 通道号）

```go
// FiveHoleProbeChannelConfig 五孔探针通道配置（每通道独立选采集设备）
type FiveHoleProbeChannelConfig struct {
    Name     string              `json:"name"`
    Role     FiveHoleChannelRole `json:"role"`
    DeviceID string              `json:"deviceId"`  // 【新增 vs 三孔】每通道独立选设备
    Channel  int                `json:"channel"`
    Enabled  bool                `json:"enabled"`
}
```

> 三孔是"每探针 1 个 DeviceID + 通道表"，五孔改为"每通道独立 DeviceID + Channel"（决策 #5）。
> 这意味着同一探针的 P1-P5 可能来自不同采集设备。

### 探针配置（每探针 5 压力通道 + α 轴 + β 轴 + 校准文件）

```go
// FiveHoleProbeConfig 单根五孔探针配置
type FiveHoleProbeConfig struct {
    ProbeID        string                        `json:"probeId"`        // probe1/probe2/probe3
    Enabled        bool                          `json:"enabled"`        // 是否启用（决策 #8：配几根跑几根）
    ProbeChannels  []FiveHoleProbeChannelConfig   `json:"probeChannels"`  // P1-P5 各自数据源
    MotionAlpha    MotionAxisMapping             `json:"motionAlpha"`    // α 轴：位移机构 + 轴号
    MotionBeta     MotionAxisMapping              `json:"motionBeta"`     // β 轴：位移机构 + 轴号
    CalibFiles     []FiveHoleCalibFileInfo        `json:"calibFiles"`     // .prb 校准文件（每探针独立载入）
}

// MotionAxisMapping 运动轴映射（每轴独立选位移机构）
type MotionAxisMapping struct {
    ControllerID string `json:"controllerId"` // 【新增 vs 三孔】每轴独立选位移机构
    Axis         string `json:"axis"`        // 轴名（X/Y/Z/U）
}
```

> 三孔的 `MotionAxisMapping` 只有 `Axis`（共享探针的 MotionControllerID），五孔扩展为含 `ControllerID`（决策 #5：每轴独立选位移机构）。

### 全局配置（布点 + 驻留/采样 + PAtm/TAtm + 保存路径）

```go
// FiveHoleTraversalConfig 五孔移位测试配置
type FiveHoleTraversalConfig struct {
    Name             string                 `json:"name"`
    Layout           TraversalLayout        `json:"layout"`           // 布点（复用三孔 TraversalLayout）
    DwellTimeMs      int                    `json:"dwellTimeMs"`      // 驻留时间（三根共用）
    SamplesPerPoint  int                    `json:"samplesPerPoint"`  // 采样次数（三根共用）
    SampleIntervalMs int                    `json:"sampleIntervalMs"` // 采样间隔（三根共用）
    MotionTimeoutMs  int                    `json:"motionTimeoutMs"`  // 运动等待超时（三根共用）
    // PAtm/TAtm 全局共享数据源（决策 #6）
    PAtmDeviceID     string                 `json:"pAtmDeviceId"`
    PAtmChannel      int                    `json:"pAtmChannel"`
    TAtmDeviceID     string                 `json:"tAtmDeviceId"`
    TAtmChannel      int                    `json:"tAtmChannel"`
    // 1-3 根探针（决策 #8：可配置 1-3 根）
    Probes           []FiveHoleProbeConfig   `json:"probes"`
    SavePath         string                 `json:"savePath"`
    SaveFileName     string                 `json:"saveFileName"`
}
```

### 原始数据与插值结果

```go
// FiveHoleRawData 五孔原始数据（每探针一份）
type FiveHoleRawData struct {
    P1, P2, P3, P4, P5 float64 `json:"p1,p2,p3,p4,p5"`
    PAtm               float64 `json:"pAtm"` // 来自全局 PAtm 通道
    TAtm               float64 `json:"tAtm"` // 来自全局 TAtm 通道
}

// FiveHoleInterpolationResult 五孔插值结果
type FiveHoleInterpolationResult struct {
    PtProbe        float64 `json:"ptProbe"`        // 总压
    PsProbe        float64 `json:"psProbe"`        // 静压
    MachProbe      float64 `json:"machProbe"`      // 马赫数
    AlphaProbe     float64 `json:"alphaProbe"`     // 攻角（度）
    BetaProbe      float64 `json:"betaProbe"`      // 【新增 vs 三孔】侧滑角（度）
    VelocityProbe  float64 `json:"velocityProbe"`  // 速度（m/s）
    IterationCount int     `json:"iterationCount"`
    Converged      bool    `json:"converged"`
    Valid          bool    `json:"valid"`
    ErrorMsg       string  `json:"errorMsg,omitempty"`
}
```

### 校准文件（`.prb`，占位结构，待算法确认）

```go
// FiveHoleCalibData 五孔校准数据（.prb 文件解析结果，格式待算法确认）
type FiveHoleCalibData struct {
    FilePath string                `json:"filePath"`
    FileName string                `json:"fileName"`
    CMa      float64               `json:"cMa"`      // 校准马赫数
    Entries  []FiveHoleCalibEntry  `json:"entries"`  // 校准条目
}

// FiveHoleCalibEntry 五孔校准条目（占位，待算法确认实际字段）
// 参考文档提到 .prb 是 13×13 网格，含 ka kb cpt cps alpha beta 六列
type FiveHoleCalibEntry struct {
    Ka    float64 `json:"ka"`
    Kb    float64 `json:"kb"`
    Cpt   float64 `json:"cpt"`
    Cps   float64 `json:"cps"`
    Alpha float64 `json:"alpha"`
    Beta  float64 `json:"beta"`
}
```

### 任务状态（统一进度 + 各探针 phase）

```go
// FiveHoleTraversalTaskStatus 五孔测试任务状态
type FiveHoleTraversalTaskStatus struct {
    TaskID          string                    `json:"taskId"`
    Status          TraversalTestStatus       `json:"status"`          // 统一状态
    TotalPoints     int                       `json:"totalPoints"`
    CompletedPoints int                       `json:"completedPoints"` // 统一进度（等最慢探针）
    Progress        float64                   `json:"progress"`
    CurrentPoint    *TraversalPoint           `json:"currentPoint"`
    // 每探针独立 phase/坐标（决策 #10：统一进度 + 各探针 phase 指示）
    ProbeStatuses   []FiveHoleProbeStatus     `json:"probeStatuses"`
    LastError       string                    `json:"lastError,omitempty"`
}

// FiveHoleProbeStatus 单根探针实时状态
type FiveHoleProbeStatus struct {
    ProbeID     string                   `json:"probeId"`
    Phase       TraversalPhase            `json:"phase"`      // moving/waiting/acquiring/completed
    CurrentX    float64                  `json:"currentX"`
    CurrentY    float64                  `json:"currentY"`
    RawData     *FiveHoleRawData         `json:"rawData,omitempty"`
    InterpResult *FiveHoleInterpolationResult `json:"interpResult,omitempty"`
}
```

## 核心业务流程

### 测试主循环（统一控制 + 多探针并行）

```
Start(config) →
  1. 校验：启用的探针数 1-3、每根校准文件已载入、PAtm/TAtm 通道已配置、布点非空
  2. 为每个启用探针：启动采集设备（若未运行）、载入校准文件、初始化插值器
  3. 生成布点坐标（共享）
  4. 主循环（每布点）：
     a. 【并行】移动所有启用探针到 (X, Y)（各自坐标系，6 轴并行 MoveToPoint）
     b. 【等待】等所有探针运动完成（motionWaiter，超时 = MotionTimeoutMs）
     c. 【驻留】DwellTimeMs，期间每 100ms 推送实时数据（phase=waiting）
     d. 【采样】SamplesPerPoint 次，每次间隔 SampleIntervalMs：
        - 并行读取所有探针的 P1-P5 + 全局 PAtm/TAtm
        - 每探针独立 3σ 滤波 + 插值
        - 推送实时数据（phase=acquiring）
     e. 【完成该点】对每探针：3σ 滤波后均值 → 最终插值 → 写 CSV 行 → 推送完成事件
     f. 统一进度 +1，推送 progress 事件
  5. 全部完成：停止采集（若由本任务启动）、推送 complete 事件
```

### 多探针同步运动协调器（`motion_coordinator.go`，新增）

三孔是单探针双轴并行（α→X, β→Y）。五孔扩展为**多探针各自双轴并行**：

```
MoveAllProbesToPoint(point TraversalPoint, probes []FiveHoleProbeConfig):
  for each enabled probe:
    go MoveProbeAxis(probe.MotionAlpha.ControllerID, probe.MotionAlpha.Axis, point.X)  // α 轴
    go MoveProbeAxis(probe.MotionBeta.ControllerID, probe.MotionBeta.Axis, point.Y)     // β 轴
  wait all goroutines done (with MotionTimeoutMs)
  // 决策 #10：等最慢的一个结束再进行下一步
```

> 直线模式（决策 #2）：若 LineLayout 端点单维变化（StartX==EndX 或 StartY==EndY），则该方向轴不参与移动，仅驱动另一轴。这需要 `point_generator` 或 `motion_coordinator` 识别单轴场景。

### 多探针并行采集

```
ReadAllProbesRawData(probes []FiveHoleProbeConfig, pAtmSrc, tAtmSrc) []FiveHoleRawData:
  // 全局 PAtm/TAtm 一次读取（三根共用）
  pAtm = batchGet(pAtmSrc.DeviceID, pAtmSrc.Channel)
  tAtm = batchGet(tAtmSrc.DeviceID, tAtmSrc.Channel)
  for each enabled probe:
    go func(probe):
      p1 = batchGet(probe.P1.DeviceID, probe.P1.Channel)
      ... p5 = batchGet(probe.P5.DeviceID, probe.P5.Channel)
      return FiveHoleRawData{P1..P5, PAtm: pAtm, TAtm: tAtm}
  wait all goroutines
```

> 三孔的 `ThreeHoleBatchGetter` 是单设备批量读取。五孔因每通道独立 DeviceID（决策 #5），需扩展为**多设备并行读取**：对每个 DeviceID 发起一次批量读取，再按通道映射分发给各探针。

### 插值器占位骨架（`interpolator.go`）

```go
type FiveHoleInterpolator struct {
    calibData []types.FiveHoleCalibData
    loaded    bool
}

func (i *FiveHoleInterpolator) Calculate(rawData types.FiveHoleRawData) (types.FiveHoleInterpolationResult, error) {
    if !i.loaded {
        return types.FiveHoleInterpolationResult{Valid: false, ErrorMsg: "校准文件未载入"}, nil
    }
    // 【占位】算法待用户提供
    return types.FiveHoleInterpolationResult{Valid: false, ErrorMsg: "五孔插值算法未实现（待提供）"}, nil
}
```

### 事件通道（`<domain>:<action>` 命名约定）

| 事件 | 通道 | 用途 |
|------|------|------|
| 实时数据 | `five-hole:realtime` | 每 100ms 推送所有启用探针的实时原始数据 + 插值结果 |
| 进度 | `five-hole:progress` | 每布点完成推送统一进度 + 各探针 phase |
| 完成 | `five-hole:complete` | 全部布点完成 |
| 错误 | `five-hole:error` | 含 IsFatal 标志 |

> 实时事件 payload 含所有探针的实时数据（前端三栏并排展示需一次事件含三根数据）。

## 前端 UI 设计

### 单窗口三栏布局（`FiveHoleTestView.vue`）

```
┌─────────────────────────────────────────────────────────────────┐
│ 工具栏：[开始] [暂停] [恢复] [停止] | 进度条 75% (15/20) phase徽章│
│         | 当前 X=10.5 Y=20.3 | [设置]                          │
├─────────────────────────────────────────────────────────────────┤
│ 布点预览 Canvas（单 Canvas，共享布点，整体状态着色）            │
│ ┌─────────────┐                                                │
│ │  ● ● ● ○ ○ │  ●=completed ○=pending ◎=current               │
│ │  ● ◎ ● ○ ○ │  颜色：pending/moving/acquiring/waiting/done    │
│ │  ● ● ● ○ ○ │                                                │
│ └─────────────┘                                                │
├──────────────────┬──────────────────┬──────────────────────────┤
│ 探针 1           │ 探针 2           │ 探针 3                    │
│ phase: acquiring │ phase: acquiring │ phase: waiting           │
│ X=10.5 Y=20.3   │ X=10.5 Y=20.3   │ X=10.5 Y=20.3            │
│ ┌──────────────┐│ ┌──────────────┐│ ┌──────────────┐          │
│ │实时压力数值卡││ │实时压力数值卡││ │实时压力数值卡│          │
│ │P1: 101.2 kPa││ │P1: 101.2 kPa││ │P1: 101.2 kPa│          │
│ │P2: ...      ││ │P2: ...      ││ │P2: ...      │          │
│ │P3-P5        ││ │P3-P5        ││ │P3-P5        │          │
│ │P∞: 101.3 kPa││ │P∞: 101.3 kPa││ │P∞: 101.3 kPa│          │
│ │T∞: 20.5 °C  ││ │T∞: 20.5 °C  ││ │T∞: 20.5 °C  │          │
│ └──────────────┘│ └──────────────┘│ └──────────────┘          │
│ ┌──────────────┐│ ┌──────────────┐│ ┌──────────────┐          │
│ │插值结果数值卡││ │插值结果数值卡││ │插值结果数值卡│          │
│ │α: 5.2°      ││ │α: 5.1°      ││ │α: --        │          │
│ │β: 1.3°      ││ │β: 1.4°      ││ │β: --        │          │
│ │Ma: 0.45     ││ │Ma: 0.45     ││ │Ma: --       │          │
│ │V: 150 m/s   ││ │V: 151 m/s   ││ │V: --        │          │
│ │Pt: ... Ps:..││ │Pt: ... Ps:..││ │Pt: -- Ps: --│          │
│ └──────────────┘│ └──────────────┘│ └──────────────┘          │
│ [载入校准文件]   │ [载入校准文件]   │ [载入校准文件]            │
│ probe1.prb ✓    │ probe2.prb ✓    │ (未启用)                  │
└──────────────────┴──────────────────┴──────────────────────────┘
```

### 设置弹窗（`el-dialog`，两个 Tab）

**Tab 1：全局配置**
- 布点配置（模式选择 + 矩形/直线/自定义表单 + 快捷步长，照三孔）
- 驻留/采样参数（DwellTimeMs/SamplesPerPoint/SampleIntervalMs/MotionTimeoutMs，三根共用）
- PAtm 数据源（设备 `el-select` + 通道 `el-input-number`）
- TAtm 数据源（设备 `el-select` + 通道 `el-input-number`）
- 保存路径 + 文件名

**Tab 2：探针配置**
- 1-3 根探针的启用开关
- 每根启用探针：
  - P1-P5 通道映射表（每行：通道名 / 角色 / 采集设备 `el-select` / 通道号 `el-input-number`）
  - α 轴：位移机构 `el-select` + 轴号 `el-select`（X/Y/Z/U）
  - β 轴：位移机构 `el-select` + 轴号 `el-select`
  - .prb 校准文件载入（多文件选择 + 加载状态显示）

### 路由

`/#/five-hole-test`（单窗口，非三孔的 `?probe=probe1` 多窗口模式）

### 复用的前端组件

- `GlassCard`、`ValueDisplay`（数值卡）
- 布点 Canvas 绘制逻辑（`drawPointCanvas`/`expandSteps`/`niceStep`，从三孔抽取为共享 composable 或直接复制）
- **不复用** `ChartPanel`（无波形图，决策 #11）

## Code Style

照三孔模块，关键约定：

```go
// Go：错误包装用 %w，日志用 slog，事件通道命名 <domain>:<action>
// 包内私有类型用小写开头，导出类型用大写开头
// 函数类型定义在 types_local.go（如 FiveHoleBatchGetter）
type FiveHoleBatchGetter func(deviceID string, channels []int) (map[int]float64, error)
```

```vue
<!-- Vue3 <script setup lang="ts">，样式 <style lang="scss" scoped> -->
<!-- Wails 绑定静态 import，不动态 import -->
<!-- 所有外部字符串中文 -->
```

## Testing Strategy

照三孔测试套件：

| 测试文件 | 覆盖 |
|----------|------|
| `internal/five_hole/interpolator_test.go` | 占位骨架测试（未载入校准返回错误、Calculate 返回 NotImplemented） |
| `internal/five_hole/point_generator_test.go` | 布点生成（矩形/直线/自定义，单轴直线场景） |
| `internal/five_hole/motion_coordinator_test.go` | 多探针并行移动 + 等待最慢 |
| `internal/five_hole/csv_writer_test.go` | CSV 导出（含 β 列、每探针一文件） |
| `internal/five_hole/data_processor_test.go` | 多设备并行采集 + 3σ 滤波 |

- Go 测试：`go test ./internal/five_hole/... ./internal/types/...`
- 前端测试：暂不强制（三孔前端无测试）
- 手动验证：`wails3 dev` 启动，配置 1-3 根探针，走完整流程

## Boundaries

**Always**:
- 修改 Go 符号前用 GitNexus MCP 跑 impact 分析
- 修改后端后 `wails3 generate bindings` 重新生成绑定
- `golangci-lint run ./internal/...` + `go build ./...` 通过
- 前端 `cd frontend && npm run lint` + `npm run build` 通过
- 所有外部字符串中文
- 事件通道命名 `five-hole:<action>`

**Ask first**:
- 修改 `internal/types/three_hole_traversal.go`（共享类型，如 TraversalLayout）
- 修改 `internal/app/core.go`（DI 汇聚点）
- 修改三孔模块代码（若需抽取共享逻辑到 `internal/traversal/` 公共包）
- 校准文件 `.prb` 实际格式（待算法确认后调整 `FiveHoleCalibEntry`）

**Never**:
- 手动编辑 `frontend/bindings/`
- 实现具体五孔插值算法（占位，待用户提供）
- 添加波形图组件（决策 #11 明确不要）
- 实现跨探针合并 CSV（决策 #9 明确每探针独立 CSV）
- 沿用 `internal/calibration/` 现有五孔代码（那是另一套设计）

## Success Criteria

- [ ] 可在单界面配置 1-3 根五孔探针，每根探针的 P1-P5 各自选采集设备+通道号、α/β 轴各自选位移机构+轴号、独立载入 .prb 校准文件
- [ ] 全局配置 PAtm/TAtm 数据源（三根共用）
- [ ] 布点配置（矩形/直线/自定义）照三孔，直线模式为纯轴向单轴
- [ ] 统一开始/暂停/停止/恢复，三根探针同步移动、同步驻留、同步采样
- [ ] 每点等最慢探针完成才前进，统一进度条显示
- [ ] 每栏显示该探针 phase + 当前 X/Y + 实时压力数值卡 + 插值结果数值卡
- [ ] 单 Canvas 共享布点预览，整体状态着色（三根都完成才标 completed）
- [ ] 插值器占位骨架：未载入校准返回错误，已载入返回 NotImplemented（待算法）
- [ ] 每根探针独立 CSV 导出（含 β 列）
- [ ] `go build ./...` + `golangci-lint run ./internal/...` + `cd frontend && npm run build` 全部通过
- [ ] `wails3 dev` 启动后可走完整流程（配置→开始→实时显示→暂停/恢复→停止→CSV 导出）

## Open Questions

1. **`.prb` 校准文件实际格式**：参考文档提到 13×13 网格、六列 `ka kb cpt cps alpha beta`，但需等用户提供算法时确认。当前 `FiveHoleCalibEntry` 为占位。
2. **多/单 prb 索引方式**：用户说"支持多 prb 和单 prb 插值"。当前理解为类三孔多 Ma 模式（多文件按 Ma 索引），待算法确认。
3. **是否抽取共享代码到 `internal/traversal/` 公共包**：三孔与五孔的布点（TraversalLayout/StepSegment/point_generator）、运动控制抽象、采集抽象、CSV 写入模式高度重复。可在本次或后续抽取。**本次默认不抽取**（避免改动三孔模块引入风险），直接复制到 `internal/five_hole/`，后续重构。
4. **直线模式单轴识别**：`point_generator` 生成直线点位时，是否需要标记"仅 X 方向变化"或"仅 Y 方向变化"，供 `motion_coordinator` 跳过对应轴？当前设计：`motion_coordinator` 检查 LineLayout 端点坐标判断单轴场景。
5. **配置持久化 key**：`fiveHoleTestConfig`（单窗口单配置）vs `fiveHoleTestConfig_${probeID}`（多探针独立配置）。当前设计为单窗口单配置（全局 config 含 1-3 探针），key = `fiveHoleTestConfig`。
6. **`internal/calibration/` 现有五孔代码处置**：当前标注"隐藏"。本次新增 `internal/five_hole/` 后，是否删除 `internal/calibration/`？**默认保留不动**，避免影响其他可能引用。

## 访谈决策汇总（参考）

| # | 决策点 | 选择 |
|---|--------|------|
| 1 | 轴配置模型 | 每根探针配 α、β 两轴（矩形/自定义）；直线模式纯轴向单轴 |
| 2 | 直线模式 | 纯轴向（端点单维变化），单轴驱动 |
| 3 | 探针关系 | 3 根协同同一布点坐标（网格拓扑共享，各自坐标系），同步移动同步采集 |
| 4 | 测试控制 | 统一控制（一个任务驱动 6 轴并行移动 + 3 套采集设备并行采样） |
| 5 | 设备配置粒度 | 全局逐通道/逐轴指定设备（每通道/每轴独立选"设备+通道号/轴号"） |
| 6 | PAtm/TAtm | 三根共用一组，全局配置（仍要选数据源） |
| 7 | 插值算法 | 占位骨架，输入 P1-P5+PAtm+TAtm，输出 α/β/Ma/V/Pt/Ps |
| 8 | 探针数量 | 可配置 1-3 根，配几根跑几根 |
| 9 | CSV 导出 | 每根探针独立 CSV，格式照三孔加 β 列 |
| 10 | 进度显示 | 顶部统一进度条（等最慢探针完成该点才前进）+ 每栏显示该探针 phase/X-Y |
| 11 | 单栏内容 | 实时压力数值卡（P1-P5+P∞+T∞）+ 插值结果数值卡（α/β/Ma/V/Pt/Ps），无波形图 |
| 12 | 布点 Canvas | 单 Canvas 共享布点，按整体状态着色（三根都完成才标 completed） |
| 13 | 移动语义 | 3 根探针各自坐标系，共享布点网格拓扑（同一 X/Y 值在各探针坐标系含义独立） |
| 14 | 校准文件 | 每根探针一个 .prb 文件（含 α/β 联合标定），独立载入；多/单 prb 索引方式待算法确认 |
| 15 | prb 文件 | prb 是文件扩展名，每个探针配置时选自己的 prb 文件，插值时使用 |
