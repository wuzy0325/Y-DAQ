# 五孔模块实现计划与任务拆解

> 基于 [five-hole-module-spec.md](./five-hole-module-spec.md)。
> 架构蓝本：三孔模块（`internal/three_hole/` + `internal/types/three_hole_traversal.go` + `frontend/src/views/ThreeHoleTestView.vue`）。

## Phase 2: Plan

### 主要组件及依赖

```
                       ┌──────────────────────────────────┐
                       │ internal/types/five_hole_traversal│ (T1)
                       └──────────────┬───────────────────┘
                                      │ 依赖 types.ThreeHole* 共享类型（TraversalLayout 等）
                    ┌─────────────────┼─────────────────────┐
                    ▼                 ▼                     ▼
         ┌─────────────────┐ ┌──────────────────┐  ┌─────────────────┐
         │ five_hole/      │ │ five_hole/       │  │ five_hole/      │
         │ interpolator    │ │ point_generator  │  │ csv_writer       │ (T2-T4)
         │ (占位骨架)       │ │ (复用三孔逻辑)   │  │ (加 β 列)        │
         └────────┬────────┘ └────────┬─────────┘  └────────┬────────┘
                  │                   │                     │
                  └───────────┬───────┴─────────────────────┘
                              ▼
                ┌─────────────────────────────┐
                │ five_hole/                   │
                │ motion_coordinator          │ (T5) 多探针并行移动
                │ + data_processor            │      多设备并行采集
                └────────────┬────────────────┘
                             │
                ┌────────────▼────────────────┐
                │ five_hole/                   │
                │ service + test_manager      │ (T6) 测试生命周期 + 主循环
                │ + event_handler             │
                └────────────┬────────────────┘
                             │
                ┌────────────▼────────────────┐
                │ app/service_five_hole.go    │ (T7) Wails 绑定层
                │ + core.go 注册              │
                └────────────┬────────────────┘
                             │ wails3 generate bindings
                ┌────────────▼────────────────┐
                │ frontend/stores/fiveHoleTest│ (T8) Pinia store
                │ + wails-compat 映射         │
                └────────────┬────────────────┘
                             │
                ┌────────────▼────────────────┐
                │ frontend/views/              │
                │ FiveHoleTestView.vue         │ (T9) 单窗口三栏 UI
                └─────────────────────────────┘
```

### 实现顺序

**顺序约束**：
1. T1（类型层）必须最先——所有其他任务依赖类型
2. T2-T4（插值器/布点/CSV）可并行，依赖 T1
3. T5（运动协调/采集）依赖 T1+T2
4. T6（service/生命周期）依赖 T2-T5
5. T7（Wails 绑定）依赖 T6，且需要 `wails3 generate bindings` 生成前端绑定
6. T8（前端 store）依赖 T7 的 bindings
7. T9（前端视图）依赖 T8

**可并行**：T2、T3、T4 在 T1 完成后可并行。

### 风险与缓解

| 风险 | 缓解 |
|------|------|
| 修改 `core.go` 影响 DI 汇聚点 | 最小化改动：仅新增 `FiveHoleServices map` + `initFiveHole()`，不动现有字段。修改前跑 GitNexus impact |
| 修改 `enums.ts` 影响三孔 | 五孔新增独立 `FiveHoleChannelRole`，不动 `ThreeHoleChannelRole` |
| 多设备并行采集性能 | 参考 `AcquisitionHub.GetSnapshot()` 已有批量读取，按 DeviceID 分组后并行 |
| 直线模式单轴识别 | `motion_coordinator` 检查 LineLayout 端点坐标，单维变化时跳过对应轴 |
| .prb 格式未定 | 占位 `FiveHoleCalibEntry`，解析器先按参考文档六列实现，待算法确认后调整 |
| Wails bindings 生成失败 | T7 后立即 `wails3 generate bindings` 验证，失败则回查 service 方法签名 |

### 验证检查点

| 检查点 | 命令 | 时机 |
|--------|------|------|
| Go 编译 | `go build ./...` | T1, T6, T7 后 |
| Go lint | `golangci-lint run ./internal/...` | T7 后 |
| Go 测试 | `go test ./internal/five_hole/... ./internal/types/...` | T2-T6 后 |
| Wails bindings 生成 | `wails3 generate bindings -clean=true -ts` | T7 后 |
| 前端构建 | `cd frontend && npm run build` | T8, T9 后 |
| 前端 lint | `cd frontend && npm run lint` | T9 后 |
| 集成验证 | `wails3 dev` 手动走流程 | T9 后 |

---

## Phase 3: Tasks

### T1: 类型层 `five_hole_traversal.go`

- **描述**：在 `internal/types/` 新建五孔全部类型定义，照三孔组织
- **内容**：
  - `FiveHoleChannelRole`（P1-P5, PAtm, TAtm）
  - `FiveHoleProbeChannelConfig`（含 DeviceID，每通道独立设备）
  - `FiveHoleProbeConfig`（含 ProbeID/Enabled/Channels/MotionAlpha/MotionBeta/CalibFiles）
  - `MotionAxisMapping` 扩展（含 ControllerID）—— 注意：三孔已有 `MotionAxisMapping` 只有 `Axis`。**五孔新建独立类型 `FiveHoleMotionAxisMapping`** 避免影响三孔
  - `FiveHoleTraversalConfig`（全局配置：Layout/Dwell/Samples/PAtm/TAtm source/Probes[]）
  - `FiveHoleRawData`（P1-P5 + PAtm + TAtm）
  - `FiveHoleInterpolationResult`（Pt/Ps/Ma/α/β/V/IterCount/Converged/Valid/ErrorMsg）
  - `FiveHoleCalibData` + `FiveHoleCalibEntry`（占位，六列 ka/kb/cpt/cps/alpha/beta）
  - `FiveHoleCalibFileInfo`
  - `FiveHoleProbeStatus`（ProbeID/Phase/X/Y/RawData/InterpResult）
  - `FiveHoleTraversalTaskStatus`（统一进度 + ProbeStatuses[]）
  - `FiveHoleRealtimeEvent` / `FiveHoleProgressEvent` / `FiveHoleCompleteEvent` / `FiveHoleErrorEvent`
  - `Validate()` 方法
- **验收**：`go build ./...` 通过；类型可 JSON 序列化
- **验证**：`go build ./internal/types/...`
- **文件**：`yx-daq/internal/types/five_hole_traversal.go`（新建）

### T2: 插值器占位骨架

- **描述**：五孔插值器，`Calculate` 返回 NotImplemented，校准文件解析按占位格式
- **内容**：
  - `FiveHoleInterpolator` 结构体（calibData / loaded）
  - `LoadCalibFiles(filePaths []string) error`（解析 .prb，按占位六列格式，校验所有文件 α/β 序列一致）
  - `Calculate(rawData FiveHoleRawData) (FiveHoleInterpolationResult, error)`（占位：未载入返回错误，已载入返回 NotImplemented）
  - `IsLoaded() bool` / `GetCalibInfo() []FiveHoleCalibFileInfo`
  - `parsePrbFile(path string) (*FiveHoleCalibData, error)`（占位解析器）
  - 单元测试：未载入返回错误、载入后 Calculate 返回 NotImplemented、解析空文件报错
- **验收**：`go test ./internal/five_hole/...` 通过
- **验证**：`go test -v ./internal/five_hole/ -run TestFiveHoleInterpolator`
- **文件**：`yx-daq/internal/five_hole/interpolator.go`、`interpolator_test.go`（新建）

### T3: 布点坐标生成器

- **描述**：复用三孔 `point_generator.go` 逻辑（TraversalLayout 是共享类型）
- **内容**：
  - `generatePoints(layout TraversalLayout) ([]TraversalPoint, error)`（照三孔，支持 line/rectangle/custom）
  - `expandStepSegments(segments []StepSegment) ([]float64, error)`（照三孔，整数步数防浮点误差）
  - `maxTraversalPoints = 50000` 上限保护
  - 单元测试：矩形/直线/自定义、分段步长、单轴直线（StartX==EndX 或 StartY==EndY）、超上限报错
- **验收**：测试覆盖三孔已有场景 + 单轴直线场景
- **验证**：`go test -v ./internal/five_hole/ -run TestPointGenerator`
- **文件**：`yx-daq/internal/five_hole/point_generator.go`、`point_generator_test.go`（新建，可从三孔复制改造）

### T4: CSV 写入器

- **描述**：每探针独立 CSV，表头加 β 列
- **内容**：
  - `FiveHoleCsvWriter` 结构体
  - `NewCsvWriter(filePath string, probeID string) (*FiveHoleCsvWriter, error)`
  - `WriteHeader() error`（表头：`X,Y,P1,P2,P3,P4,P5,P∞,T∞,Pt,Ps,Ma,α,β,V,迭代次数,收敛,时间戳`）
  - `WriteDataPoint(point FiveHoleTraversalDataPoint) error`（UTF-8 BOM，每 50 点 flush）
  - `Close() error`
  - 单元测试：写入表头正确、数据行字段顺序、β 列存在、多文件独立
- **验收**：CSV 表头含 β 列，数据行 17 列对齐
- **验证**：`go test -v ./internal/five_hole/ -run TestCsvWriter`
- **文件**：`yx-daq/internal/five_hole/csv_writer.go`、`csv_writer_test.go`（新建）

### T5: 运动协调器 + 数据处理器

- **描述**：多探针并行移动 + 多设备并行采集 + 3σ 滤波 + 实时监控
- **内容**：
  - `types_local.go`：函数类型定义
    - `FiveHoleMultiDeviceBatchGetter func(deviceID string, channels []int) (map[int]float64, error)`
    - `FiveHoleProbeAxisMover func(controllerID string, axis string, position float64) error`
    - `FiveHoleProbeAxisWaiter func(controllerID string, axis string, timeoutMs int) error`
    - `FiveHoleEventPublisher` 接口
  - `motion_coordinator.go`：
    - `MoveAllProbesToPoint(point TraversalPoint, probes []FiveHoleProbeConfig, layout TraversalLayout) error`
    - 识别直线模式单轴场景（LineLayout 端点单维变化时跳过对应轴）
    - 6 轴并行 goroutine + `sync.WaitGroup` + MotionTimeoutMs 超时
  - `data_processor.go`：
    - `ReadAllProbesRawData(probes, pAtmSrc, tAtmSrc) ([]FiveHoleRawData, error)`（全局 PAtm/TAtm 一次读取，各探针 P1-P5 按设备分组并行）
    - `outlierFilteredAvg(values []float64) float64`（3σ 滤波，照三孔）
    - `runRealtimeMonitor` 100ms ticker（测试未运行时）
    - `DwellWithRealtimeUpdate` / `AcquireAndInterpolate`
  - 单元测试：多探针并行移动等待最慢、单轴直线跳过对应轴、多设备并行采集、3σ 滤波
- **验收**：多探针并行移动正确等待、单轴直线只驱动一轴、多设备采集按 DeviceID 分组
- **验证**：`go test -v ./internal/five_hole/ -run "TestMotionCoordinator|TestDataProcessor"`
- **文件**：`yx-daq/internal/five_hole/types_local.go`、`motion_coordinator.go`、`data_processor.go`、`*_test.go`（新建）

### T6: 服务层 + 测试管理器 + 事件处理器

- **描述**：测试生命周期 + 主循环 + 事件路由
- **内容**：
  - `service.go`：
    - `FiveHoleTraversalService` 结构体（持有多探针 interpolator map、motion_coordinator、data_processor、csv_writers map）
    - `Start(config FiveHoleTraversalConfig) (string, error)`（校验 1-3 探针启用、校准已载入、PAtm/TAtm 已配、布点非空；为每个启用探针初始化 interpolator + csv_writer；生成共享布点；主循环）
    - `Pause() / Resume() / Stop() / GetStatus() / GetConfig()`
    - `LoadCalibFiles(probeID string, filePaths []string) error`（每探针独立载入）
    - `IsCalibLoaded(probeID string) bool` / `GetCalibInfo(probeID string) []FiveHoleCalibFileInfo`
    - `StartRealtimeMonitor(config) / StopRealtimeMonitor()`
    - `SetMultiDeviceBatchGetter / SetProbeAxisMover / SetProbeAxisWaiter / SetEventPublisher`
    - 主循环 `runTestLoop`：每点 = MoveAll → Dwell → Acquire（并行读+每探针滤波+插值）→ 每探针写 CSV → 统一进度+1
  - `test_manager.go`：状态机 + 暂停/停止协调（照三孔，扩展为多探针 phase 跟踪）
  - `event_handler.go`：事件发射路由（realtime/progress/complete/error）
  - `realtime_recorder.go`：实时数据录制（照三孔）
  - 单元测试：Start 校验失败场景（无探针启用/未载入校准/PAtm 未配/布点为空）
- **验收**：Start 校验逻辑正确、主循环结构完整（占位算法时插值返回 NotImplemented 但流程走通）
- **验证**：`go test -v ./internal/five_hole/ -run TestService`
- **文件**：`yx-daq/internal/five_hole/service.go`、`test_manager.go`、`event_handler.go`、`realtime_recorder.go`、`*_test.go`（新建）

### T7: Wails 绑定层 + Core 注册

- **描述**：`FiveHoleService` Wails 绑定 + 注册到 Core + main.go
- **内容**：
  - `internal/app/service_five_hole.go`：
    - `FiveHoleService struct { Core *Core }`
    - `OpenTestWindow() string`（单窗口，无 probeID；URL `/#/five-hole-test`）
    - `LoadFiveHoleCalibFiles(probeID string, filePaths []string) error`
    - `IsFiveHoleCalibLoaded(probeID string) bool`
    - `GetFiveHoleCalibInfo(probeID string) []types.FiveHoleCalibFileInfo`
    - `StartFiveHoleTraversal(config types.FiveHoleTraversalConfig) (string, error)`（含多设备冲突检查、自动启动采集设备）
    - `PauseFiveHoleTraversal() / ResumeFiveHoleTraversal() / StopFiveHoleTraversal()`（无 probeID，统一控制）
    - `StartFiveHoleRealtimeMonitor(config) / StopFiveHoleRealtimeMonitor()`
    - `SelectAndStartFiveHoleRealtimeRecording() (string, error) / StopFiveHoleRealtimeRecording() / IsFiveHoleRealtimeRecording() bool`
    - `GetFiveHoleTraversalStatus() types.FiveHoleTraversalTaskStatus`
    - `SelectFiveHoleCalibFiles() []string`（文件选择对话框，过滤 *.prb）
    - `CheckFiveHoleMotionConflict(config) error`（检查多位移机构轴冲突）
    - `CheckFiveHoleDeviceChannelOverlap(config) string`（检查多采集设备通道冲突）
    - `SaveFiveHoleConfig(config) error / LoadFiveHoleConfig() (types.FiveHoleTraversalConfig, error)`
  - `internal/app/core.go`：
    - `Core` 新增 `FiveHoleServices *five_hole.FiveHoleTraversalService`（单实例，非 map）
    - `NewCore()` 初始化
    - `Startup()` 调用 `c.initFiveHole()`
    - `initFiveHole()`：创建单实例 service，设置 multi-device batch getter / probe axis mover / waiter（参考三孔，扩展为多设备/多控制器）
    - `Shutdown()` 停止五孔 service
    - `CheckFiveHoleMotionConflict` / `CheckFiveHoleDeviceChannelOverlap` 辅助方法
  - `main.go`：`Services` 数组新增 `application.NewService(&app.FiveHoleService{Core: core})`
  - 运行 `wails3 generate bindings -clean=true -ts` 生成前端绑定
- **验收**：`go build ./...` + `golangci-lint run ./internal/...` 通过；bindings 生成成功
- **验证**：`go build ./... && golangci-lint run ./internal/... && wails3 generate bindings -clean=true -ts`
- **文件**：`yx-daq/internal/app/service_five_hole.go`（新建）、`core.go`（修改）、`main.go`（修改）

### T8: 前端 Pinia Store + Wails 兼容层

- **描述**：五孔 store + wails-compat 映射
- **内容**：
  - `frontend/src/stores/fiveHoleTest.ts`：
    - state：config（全局 + 3 探针）、taskStatus、realtimeData（3 探针）、calibInfo（3 探针）、isRunning/isPaused
    - actions：init/loadConfig/saveConfig/startTest/pause/resume/stop/loadCalibFiles/startRealtimeMonitor/stopRealtimeMonitor
    - 事件监听：`five-hole:realtime` / `five-hole:progress` / `five-hole:complete` / `five-hole:error`
    - 配置持久化：localStorage key `fiveHoleTestConfig`
    - 默认配置：3 探针骨架（probe1/probe2/probe3，默认 probe1 启用、2/3 禁用）
  - `frontend/src/stores/fiveHoleTest/types.ts`：store 类型定义
  - `frontend/src/api/enums.ts`：新增 `FiveHoleChannelRole` 枚举
  - `frontend/src/wails-compat/app.ts` / `services.ts`：新增 `StartFiveHoleTraversal` / `PauseFiveHoleTraversal` 等映射到 v3 bindings
- **验收**：`cd frontend && npm run build` 通过；store 类型正确
- **验证**：`cd frontend && npm run build`
- **文件**：`frontend/src/stores/fiveHoleTest.ts`、`fiveHoleTest/types.ts`（新建）、`api/enums.ts`、`wails-compat/app.ts`、`wails-compat/services.ts`（修改）

### T9: 前端视图 FiveHoleTestView.vue

- **描述**：单窗口三栏 UI
- **内容**：
  - `frontend/src/views/FiveHoleTestView.vue`：
    - 工具栏：开始/暂停/恢复/停止 + 统一进度条 + phase 徽章 + 当前 X/Y + 设置按钮
    - 布点预览 Canvas（单 Canvas，共享布点，整体状态着色——从三孔复制 `drawPointCanvas`/`expandSteps`/`niceStep` 逻辑）
    - 三栏并排（每栏对应一根探针，禁用探针灰显）：
      - 探针标题 + phase + 当前 X/Y
      - 实时压力数值卡（P1-P5 + P∞ + T∞，用 `ValueDisplay` 组件）
      - 插值结果数值卡（α/β/Ma/V/Pt/Ps，未载入校准或占位算法时显示 `--`）
      - 校准文件载入按钮 + 状态显示
    - 设置弹窗 `el-dialog`：
      - Tab 1 全局配置：布点 + 驻留/采样参数 + PAtm/TAtm 数据源 + 保存路径
      - Tab 2 探针配置：1-3 探针启用开关 + 每探针 P1-P5 通道映射表（含设备选择）+ α/β 轴（位移机构+轴号）+ .prb 载入
  - `frontend/src/router/index.ts`：新增 `five-hole-test` 路由
  - `frontend/src/layouts/MainLayout.vue`（若需在导航栏加入口，确认是否需要）
- **验收**：`cd frontend && npm run build && npm run lint` 通过；`wails3 dev` 启动后可进入五孔界面、配置 1-3 探针、走完整流程（占位算法时插值显示"算法未实现"）
- **验证**：`cd frontend && npm run build && npm run lint`，然后 `wails3 dev` 手动验证
- **文件**：`frontend/src/views/FiveHoleTestView.vue`（新建）、`router/index.ts`（修改）

---

## 执行策略

- **顺序执行**：T1 → (T2,T3,T4 并行) → T5 → T6 → T7 → T8 → T9
- **每任务完成后**：跑对应验证命令，绿了再进下一个
- **T7 后必须**：`wails3 generate bindings` 生成前端绑定，否则 T8 无法 import
- **T9 后**：`wails3 dev` 手动验证完整流程

## 不在本计划内（Open Questions 留待算法确认后处理）

- 具体五孔插值算法实现（T2 占位）
- `.prb` 校准文件实际格式（T2 占位解析器）
- 多/单 prb 索引方式（T2 占位，待算法确认）
- 抽取共享代码到 `internal/traversal/` 公共包（本次复制，后续重构）
- `internal/calibration/` 现有代码处置（保留不动）
