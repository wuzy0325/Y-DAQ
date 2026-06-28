# YX-DAQ 架构与设计规范

> 本文档定义项目目录结构、设计原则与架构约束。编码细节见 `coding-standards.md`，性能规格见 `perf-spec.md`。
> 本文档与实际代码同步：所有目录树、文件名均来自当前代码库，禁止描述不存在的文件。

---

## 一、核心设计原则

本项目是 Wails v3 桌面应用（Go 1.23 后端 + Vue 3 / TypeScript 前端），用于多采集设备数据采集、显示、保存，以及结合运动控制器进行三孔 / 五孔探针移位布点插值测试。所有原则围绕五个质量目标：

| 质量目标 | 含义 | 主要约束 |
|---------|------|---------|
| **模块性** | 高内聚、低耦合，模块可独立替换 | 按业务领域分包，跨包只通过窄接口通信 |
| **前后端分离** | Go 业务逻辑与 Vue UI 职责明确，仅通过 Wails 绑定层通信 | 后端不返回 UI 语义，前端不含业务算法 |
| **健壮性** | 错误显式传播，并发安全，资源可清理，失败可恢复 | `%w` 包装错误，atomic/mutex/channel 选型明确，defer 清理 |
| **可读性** | 命名自解释，分层清晰，意图直接 | 单文件行数上限，import 顺序统一，注释只解释 WHY |
| **面向对象** | 用组合 + 接口表达对象职责，封装状态，多态解耦 | 构造函数注入，接口在使用方定义，struct 内部分组 |

### 1.1 职责单一（Single Responsibility）

每个模块、结构体、函数只做一件事，只有一个变更理由。

**Go 后端**：
- 每个 `internal/` 子包对应一个业务领域（`types` / `driver` / `manager` / `calibration` / `three_hole` / `five_hole` / `storage` / `scanner` / `logger`）
- 每个 struct 只承担一个角色：
  - `Core` —— 依赖注入汇聚点，不写业务逻辑
  - `*Service`（Wails 绑定层）—— 参数校验 + 调用业务服务 + 返回结果
  - `*TraversalService`（业务服务）—— 测试生命周期编排
  - `*Driver` / `*Controller` —— 硬件通信
  - `*Manager` —— 运行时生命周期管理
  - `*Store` —— 持久化
  - `*Coordinator` / `*Processor` / `*Recorder` / `*Writer` —— 业务服务内部的单一职责协作者
- 文件行数上限：单文件 500 行，service 主文件 800 行（超出则按功能拆分子文件）

**前端**：
- `views/` 只做页面编排，不写可复用组件逻辑
- `components/` 只做 UI 展示与交互，不包含业务语义
- `stores/` 只做状态管理与 API 调用，不包含 DOM 操作
- `composables/` 只做逻辑复用，不引用 store，不包含 UI

**判断标准**：如果一个 struct 的方法列表无法用一个短语概括，它就承担了过多职责。

### 1.2 模块化（Modularity）

高内聚、低耦合。模块间通过窄接口通信，不暴露内部细节。

- Go 包间依赖方向：`types → driver → manager → calibration/three_hole/five_hole/storage`，**禁止循环依赖**
- 跨包调用只通过接口，不直接依赖具体实现
- `internal/app/core.go` 是唯一的依赖注入汇聚点，所有跨层回调在此连接
- 前端组件分层：`views → components`（单向），`stores` 被两者引用，`components` 不引用 `views`

### 1.3 前后端分离

Go 后端与 Vue 前端职责明确，通过 Wails v3 绑定层通信。

| 层 | 职责 | 禁止 |
|----|------|------|
| Go `internal/` | 业务逻辑、硬件通信、数据持久化 | 不包含 UI 逻辑、不返回 HTML/样式 |
| Go `internal/app/` | Wails v3 服务绑定层，参数校验 + 调用业务服务 | 不含业务逻辑 |
| Vue `stores/` | 状态管理、调用 Wails API、事件监听 | 不直接操作 DOM |
| Vue `views/` + `components/` | UI 渲染与用户交互 | 不包含业务算法 |

- 后端通过 `app.Event.Emit` 推送数据，前端通过 `@wailsio/runtime` 的 `Events.On` 监听
- 事件通道命名：`<domain>:<action>`（如 `daq:data-snapshot`、`three-hole:progress`、`five-hole:realtime`）
- 前端直接 import `@bindings/yx-daq/internal/app`（Service 对象）与 `@bindings/yx-daq/internal/types`（类型）

### 1.4 面向对象关键特点

Go 不是经典 OO 语言，但本项目通过以下方式表达 OO 核心特征：

| OO 特征 | 本项目实现方式 |
|--------|--------------|
| **封装** | struct 字段小写不导出，仅通过方法暴露行为；状态字段与配置字段分组 |
| **抽象** | 接口在使用方定义（如 `manager` 定义 `DeviceDriver`，`driver` 隐式实现） |
| **多态** | 接口 + 隐式实现（真实驱动 / 模拟驱动可互换）；函数类型回调注入 |
| **组合优于继承** | 业务服务内部分解为多个单一职责 struct（`TestManager` + `DataProcessor` + `RealtimeRecorder` + `MotionCoordinator` + `EventHandler`），服务持有它们的指针并编排，不嵌入带方法的 struct 模拟继承 |
| **构造函数** | 每个导出 struct 配 `NewXxx()` 构造函数，必要依赖通过参数注入 |

```go
// 五孔服务 = 编排者 + 多个单一职责协作者（组合）
type FiveHoleTraversalService struct {
    mu sync.RWMutex
    interpolators      map[string]*FiveHoleInterpolator
    motionCoordinator  *MotionCoordinator   // 运动协调
    dataProcessor      *DataProcessor       // 数据处理
    eventHandler       *EventHandler        // 事件发布
    testManager        *TestManager         // 测试生命周期
    realtimeRecorder   *RealtimeRecorder    // 实时记录
    // ...
}
```

---

## 二、顶层目录结构

```
（仓库根）
├── AGENTS.md                 # Agent 指令 + MCP 工具配置（GitNexus + code-review-graph）
├── CLAUDE.md                 # AI 行为准则 + 项目规则 + MCP 工具配置
├── CONTEXT.md                # 领域语言 / 核心概念（AI 工具约定在根）
├── README.md                 # 仓库介绍
├── .gitignore / .golangci.yml / .mcp.json / .gitnexus-rules.json / .impeccable.md
│
├── src/                      # ★ 源码归拢目录（Wails v3 工程根 / Go 模块根）
│   ├── main.go               # Wails v3 入口：NewCore + 8 个 Service 注册 + application.New()
│   ├── go.mod / go.sum       # Go 模块依赖（module 名 yx-daq）
│   ├── wails.json            # Wails v3 项目配置
│   ├── Taskfile.yml          # Wails v3 构建任务（build/dev/run，BIN_DIR="../bin"）
│   ├── internal/             # Go 后端（业务逻辑、驱动、存储）
│   ├── frontend/             # Vue 3 前端
│   └── build/                # Wails v3 构建资源 + Taskfile 子任务（windows/darwin）
│
├── docs/                     # 所有 .md 文档（engineering/business/adr/hardware/plans/reference/agents）
└── bin/                      # 构建产物（yx-daq.exe）+ 配置模板（运行时配置实际用 ~/.yx-daq/）
```

> **路径约定**：所有源码（Go + 前端 + Wails 工程文件）统一在 `src/` 下。`wails3` 命令必须在 `src/` 目录运行（它会读取当前目录的 `wails.json` + `go.mod` + `Taskfile.yml` + `build/`）。构建产物通过 `BIN_DIR="../bin"` 输出到仓库根 `bin/`。

根目录只保留：

| 文件 | 理由 |
|------|------|
| `README.md` | GitHub/仓库介绍 |
| `AGENTS.md` | AI 开发助手指令 + MCP 工具配置（开发流程必需） |
| `CLAUDE.md` | AI 行为准则 + 项目规则 + MCP 工具配置（开发流程必需） |
| `CONTEXT.md` | 领域语言 / 核心概念（被 CLAUDE.md 引用，single-context layout） |

> 注：新建文档一律放入 `docs/`，根目录非必要不新增 `.md` 文件。`ARCHITECTURE.md`/`DESIGN.md` 已归入 `docs/engineering/`。

---

## 三、Go 后端（`src/internal/`）

### 3.1 包划分

每个包一个职责，按**业务领域**而非技术分层划分：

```
src/internal/
├── app/                      # Wails v3 服务绑定层（依赖注入、绑定方法）
│   ├── core.go               # Core 生命周期 + DI（Startup/Shutdown）
│   ├── core_service.go       # CoreService → 绑定 Core.Startup/Shutdown
│   ├── service_device.go     # DeviceService → 设备管理绑定
│   ├── service_motion.go     # MotionService → 运动控制绑定
│   ├── service_three_hole.go # ThreeHoleService → 三孔测试绑定
│   ├── service_five_hole.go  # FiveHoleService → 五孔测试绑定
│   ├── service_calibration.go# CalibrationService → 五孔校准绑定
│   ├── service_data.go       # DataService → 录制/回放绑定
│   ├── service_config.go     # ConfigService → 配置/路径绑定
│   └── event_publishers.go   # 事件发布器实现（持有 *application.App 引用）
│
├── types/                    # 共享类型、常量、枚举（纯定义，无业务逻辑，零依赖）
│   ├── device.go
│   ├── motion.go
│   ├── calibration.go
│   ├── config_types.go
│   ├── three_hole_traversal.go
│   ├── five_hole_traversal.go
│   └── constants.go
│
├── driver/                   # 硬件驱动（采集设备 + 运动控制器）
│   ├── xy_daq16.go           # DAQ-16 TCP 驱动
│   ├── yx_daqt.go            # YX-DAQT TCP 驱动
│   ├── yx_daqt_config.go     # YX-DAQT 配置
│   ├── b140.go               # B140 运动控制器 TCP 驱动
│   ├── tcp_base.go           # TCP 公共基础
│   ├── frame_parser.go       # 帧解析
│   ├── pending_response.go   # 请求/响应匹配
│   ├── simulated_device.go   # 模拟采集设备
│   └── simulated_motion.go   # 模拟运动控制器
│
├── manager/                  # 管理器（运行时状态、协调），定义接口
│   ├── device_manager.go     # 采集设备生命周期管理
│   ├── motion_manager.go     # 运动控制器生命周期管理
│   └── acquisition_hub.go    # 数据采集汇总与分发
│
├── scanner/                  # UDP 设备扫描发现
│   └── daq_scanner.go
│
├── storage/                  # 数据持久化（配置、记录、报表）
│   ├── config_store.go       # 泛型 ConfigStore[T] + ConfigManager
│   ├── data_storage.go       # CSV 实时记录
│   └── pdf_report.go         # PDF 报告生成
│
├── calibration/              # 五孔探针校准业务
│   ├── service.go            # 校准服务
│   └── formulas.go           # 计算公式
│
├── three_hole/               # 三孔探针移位测试业务（单探针）
│   ├── service.go            # 测试服务（编排者）
│   ├── test_manager.go       # 测试生命周期管理
│   ├── data_processor.go     # 数据处理（3σ 滤波等）
│   ├── interpolator.go       # 插值算法
│   ├── point_generator.go    # 布点生成
│   ├── realtime_recorder.go  # 实时数据记录
│   ├── event_handler.go      # 事件发布
│   ├── csv_writer.go         # CSV 导出
│   └── types_local.go        # 包内私有类型
│
├── five_hole/                # 五孔探针移位测试业务（1-3 探针，结构与 three_hole 平行）
│   ├── service.go            # 测试服务（编排者，支持多探针）
│   ├── test_manager.go       # 测试生命周期管理
│   ├── data_processor.go     # 数据处理（按设备分组并行读取 + timestamp 去重）
│   ├── interpolator.go       # 插值算法（每探针独立校准文件）
│   ├── point_generator.go    # 共享布点生成
│   ├── motion_coordinator.go # 多探针运动协调
│   ├── realtime_recorder.go  # 实时数据记录
│   ├── event_handler.go      # 事件发布
│   ├── csv_writer.go         # CSV 导出（每探针独立文件，含 β 列）
│   └── types_local.go        # 包内私有类型
│
└── logger/                   # 结构化日志（log/slog）
    └── logger.go
```

**关键说明**：
- `types/` 是唯一无内部依赖的包
- `manager` 包定义接口（`DeviceDriver`, `MotionController`），`driver` 包实现它们
- `three_hole/` 和 `five_hole/` 结构平行，文件一一对应；`five_hole/` 多出 `motion_coordinator.go`（多探针协调）
- 业务服务内部采用**编排者 + 单一职责协作者**模式：`Service` 持有 `TestManager` / `DataProcessor` / `RealtimeRecorder` / `MotionCoordinator` / `EventHandler` 等指针，自身只做编排，不实现具体算法
- `internal/app/core.go` 是依赖注入汇聚点，`manager/` 不直接依赖 `calibration/` / `three_hole/` / `five_hole/`（通过 `core.go` 回调注入）

### 3.2 包间依赖规则

```
types ← driver
types ← scanner
types ← manager
      │
types ← storage ← manager
      │
types ← calibration ← manager
      │
types ← three_hole  ← manager
      │
types ← five_hole   ← manager
      │
internal/app/ → 所有 internal 包（依赖注入 + 服务注册）
```

- 不允许循环依赖
- `app/` 是唯一允许引用所有内部包的包

### 3.3 每包最大行数

| 包 | 建议上限 | 超限处理 |
|----|---------|---------|
| `types/` 单文件 | 300 行 | 按领域拆文件 |
| `driver/` 单文件 | 500 行 | 按协议版本/功能拆 |
| `three_hole/service.go` / `five_hole/service.go` | 800 行 | 拆为 `controller.go` / `planner.go` |
| 其他 service | 500 行 | 拆辅助逻辑到子文件 |
| `internal/app/service_*.go` | 500 行 | 拆为子文件 |

---

## 四、前端（`src/frontend/`）

### 4.1 目录结构

```
src/frontend/
├── src/                      # 前端源码（Vite 根目录）
│   ├── api/                      # 枚举、常量、API 类型（与 Wails 无关的）
│   │   └── enums.ts
│   │
│   ├── constants/                # 全局常量（颜色等）
│   │   └── colors.ts
│   │
│   ├── stores/                   # Pinia 状态管理
│   │   ├── device.ts              # 采集设备 store
│   │   ├── motion.ts              # 运动控制器 store
│   │   ├── calibration.ts         # 五孔校准 store
│   │   ├── threeHoleTest.ts       # 三孔测试 store
│   │   ├── fiveHoleTest.ts        # 五孔测试 store
│   │   ├── motion/                # store 内部辅助（types/helpers）
│   │   ├── threeHoleTest/types.ts
│   │   └── fiveHoleTest/types.ts
│   │
│   ├── views/                    # 页面级组件（对应路由）
│   │   ├── DashboardView.vue
│   │   ├── DeviceView.vue
│   │   ├── MotionView.vue
│   │   ├── ThreeHoleTestView.vue
│   │   ├── FiveHoleTestView.vue
│   │   ├── CalibrationView.vue    # 路由存在，当前在侧边栏隐藏
│   │   ├── SettingsView.vue
│   │   └── (DataView 隐藏)
│   │
│   ├── components/               # 通用可复用组件
│   │   ├── GlassCard.vue
│   │   ├── ChartPanel.vue
│   │   ├── ValueDisplay.vue
│   │   ├── StatusIndicator.vue
│   │   ├── CalibPointEditor.vue
│   │   ├── MotionControl/         # 运动控制相关子组件
│   │   │   ├── AxisConfigDialog.vue
│   │   │   └── AxisControlCard.vue
│   │   └── __tests__/             # 组件测试
│   │
│   ├── composables/              # 逻辑复用（不引用 store，不含 UI）
│   │   └── usePlayback.ts
│   │
│   ├── layouts/                  # 布局组件
│   │   └── MainLayout.vue
│   │
│   ├── router/                   # 路由配置
│   │   └── index.ts
│   │
│   ├── assets/                   # 静态资源
│   │   ├── styles/
│   │   │   ├── variables.scss     # 全局变量（Vite 自动注入）
│   │   │   ├── global.scss
│   │   │   └── themes/theme-variables.scss
│   │   ├── fonts/
│   │   └── images/
│   │
│   ├── main.ts                   # Vue 应用入口
│   ├── App.vue
│   └── vite-env.d.ts
│
├── bindings/                 # Wails v3 自动生成的 TS 绑定（禁止手动编辑）
│   └── yx-daq/internal/{app,types}/
│
├── public/                   # 静态公共资源（favicon 等）
├── index.html                # Vite 入口 HTML
├── vite.config.ts            # Vite 配置
├── tsconfig.json             # TS 配置
├── package.json              # 前端依赖
└── .eslintrc.cjs             # ESLint 配置
```

### 4.2 组件分层

```
views/          → 页面级，对应路由，包含完整业务逻辑
components/     → 通用可复用，无页面级依赖
stores/         → 状态管理，可被 views 和 components 引用
composables/    → 逻辑复用，不引用 store，不含 UI
api/            → 纯定义，无运行时依赖
```

- `views/` 中的组件可以引用 `stores/`、`components/`、`composables/`、`api/`
- `components/` 中的组件可以引用 `stores/`、`api/`，但**不引用** `views/`
- `composables/` 不引用 `stores/`，不包含 UI 逻辑
- `node_modules` 不提交 git（已在 `.gitignore`）
- `src/frontend/bindings/` 为 Wails v3 自动生成，必要时提交（前端构建需其存在）

### 4.3 Wails v3 绑定结构

| 层 | 路径 | 性质 | 是否可编辑 |
|----|------|------|-----------|
| Wails v3 绑定 | `src/frontend/bindings/yx-daq/internal/` | 自动生成（`wails3 generate bindings`） | **禁止手动编辑** |

- Store / View 直接 import `@bindings/yx-daq/internal/app`（Service 对象，如 `DeviceService`、`FiveHoleService`）与 `@bindings/yx-daq/internal/types`（类型）
- 事件监听用 `@wailsio/runtime` 的 `Events.On(name, cb)` / `Events.Off(name)`，回调参数为 `{data: T}`，需 `event.data` 解包
- Vite alias：`@bindings` → `bindings/`，`@` → `src/`
- `bindings/` 由 `wails3 generate bindings -clean=true -ts` 重新生成

### 4.4 Wails 事件命名

`<domain>:<action>`

```
采集:      daq:data-snapshot
设备:      device:status-updated
运动:      motion:status-updated
三孔:      three-hole:progress / three-hole:realtime / three-hole:complete / three-hole:error
五孔:      five-hole:progress / five-hole:realtime / five-hole:complete / five-hole:error
五孔校准:  calibration:* （沿用现有）
```

---

## 五、类与接口设计

### 5.1 结构体设计原则

- 每个导出 struct 必须有 `NewXxx()` 构造函数，返回指针
- 必要依赖通过构造函数注入，可选/循环依赖通过 Setter 注入
- struct 字段按职责分组：依赖（构造注入）→ 状态（内部管理）→ 通道（生命周期控制）
- 状态字段与配置字段分离：配置通过构造函数/setter 一次性设置，状态通过方法修改
- **封装**：小写字段不导出，外部只能通过方法访问；map 字段读写必须经 mutex 保护

```go
type CalibrationService struct {
    // 依赖（构造注入）
    eventPublisher ThreeHoleEventPublisher

    // 状态（内部管理）
    mu      sync.Mutex
    status  Status
    running atomic.Bool

    // 通道（生命周期控制）
    cancelCh chan struct{}
    pauseCh  chan struct{}
    resumeCh chan struct{}
}
```

### 5.2 接口设计原则

- **接口在使用方定义**，不在实现方定义（Go 惯例：消费者定义接口）
- 接口应小而聚焦，1-5 个方法（接口隔离原则）
- 不为每个实现单独建接口，只为需要多态/解耦的地方定义接口
- 函数类型（`type XxxFunc func(...) (...)`）用于回调注入，替代大接口

```go
// 好：使用方定义，方法少
// manager/device_manager.go
type DeviceDriver interface {
    Connect() error
    Disconnect()
    IsConnected() bool
    ReadData() ([]types.ChannelData, error)
}

// 好：函数类型用于 Setter 注入
type BatchGetter func(deviceID string) ([]types.ChannelData, error)
type MotionFunc func(axis string, position float64) error
```

### 5.3 编排者 + 协作者模式（业务服务内部结构）

业务服务（`ThreeHoleTraversalService` / `FiveHoleTraversalService`）采用此模式，是本项目面向对象设计的核心：

- `Service` 是**编排者**：持有多个协作者指针，负责协调它们的工作流，自身不实现具体算法
- 每个协作者（`TestManager` / `DataProcessor` / `RealtimeRecorder` / `MotionCoordinator` / `EventHandler` / `CsvWriter` / `Interpolator` / `PointGenerator`）是**单一职责对象**：只做一件事
- 协作者之间不直接互引，全部通过 `Service` 编排

```go
// 编排者：只协调，不实现算法
func (s *FiveHoleTraversalService) StartTest(config Config) error {
    s.testManager.Start(config)          // 委托：生命周期
    s.pointGenerator.Generate(config)    // 委托：布点
    s.motionCoordinator.MoveTo(point)    // 委托：运动
    s.dataProcessor.Process(samples)     // 委托：数据处理
    s.eventHandler.EmitProgress(...)     // 委托：事件
    return nil
}
```

### 5.4 嵌入与继承

Go 没有类继承，使用组合而非继承：

- **允许**：嵌入接口以实现接口组合（`type ReadWriter interface { Reader; Writer }`）
- **允许**：嵌入 struct 以复用字段（但嵌入层级不超过 2 层）
- **禁止**：为了"共享代码"而嵌入大型 struct，应当提取为独立 struct 或函数
- **禁止**：嵌入带方法的 struct 来模拟继承链（用接口 + 组合替代）

前端同样遵循组合优于继承：

- Vue 组件通过 `composables/` 复用逻辑，不使用 `extends` / `mixins`
- Props + Emits 是组件间通信的标准方式，不用 `provide/inject` 传递业务数据

---

## 六、设计模式使用规范

### 6.1 核心原则

- **适度使用**：只在解决真实问题时引入设计模式，不为"将来可能需要"而预埋
- **AI 可读**：选择意图明确、命名清晰的模式，避免需要多层间接才能理解意图的模式
- **项目一致性**：同一类问题在项目内使用同一种模式，不混用

### 6.2 推荐模式（已在项目中使用，新代码沿用）

| 模式 | 用途 | 位置示例 | 选择理由 |
|------|------|---------|---------|
| 构造函数注入 | 必要依赖 | `NewCalibrationService(publisher)` | Go 惯例，依赖显式 |
| Setter 注入 | 可选/循环依赖 | `SetBatchGetter(fn)` | 解耦 manager ↔ service |
| 编排者 + 协作者 | 业务服务内部分解 | `FiveHoleTraversalService` + `TestManager` 等 | 单一职责，可独立测试 |
| 接口隐式实现 | 驱动多态 | `DeviceDriver` / `SimulatedDevice` | Go 鸭式实现，零声明成本 |
| 事件发布/订阅 | 后端→前端数据推送 | `EventHandler` + `EventPublisher` | Wails 事件机制，解耦前后端 |
| 服务生命周期 | 长运行任务 | Start/Pause/Resume/Stop | 统一模式，易理解 |
| 泛型配置存储 | 类型安全配置 | `ConfigStore[T]` | 消除 interface{} + JSON 转换 |

### 6.3 允许但不主动引入的模式

| 模式 | 场景 | 注意 |
|------|------|------|
| 策略模式 | 算法族需要运行时切换 | 用函数类型实现，不建类型层次 |
| 工厂方法 | 同族对象创建逻辑复杂 | 用 `NewXxx(typeName)` 而非抽象工厂 |
| 装饰器 | 为已有接口透明添加功能 | 嵌套不超过 2 层 |

### 6.4 禁止的模式

| 模式 | 原因 |
|------|------|
| 抽象工厂 / 建造者链 | 过度间接，AI 难以追踪创建逻辑 |
| 访问者模式 | 双分派间接，Go 中不自然 |
| 代理链 / 拦截器链 | 多层间接掩盖真实行为 |
| 任何超过 2 层的间接调用 | 难以静态追踪，调试困难 |
| `internal/utils/` 通用工具包 | 职责不清，变成垃圾桶 |
| 前端 `mixins` / `extends` | Vue 3 已废弃，用 composables 替代 |

---

## 七、测试文件

| 层次 | 位置 | 命名 | 运行方式 |
|------|------|------|---------|
| Go 后端 | 与被测文件同目录 | `<name>_test.go` | `go test ./internal/...` |
| Vue/TS 组件 | `components/__tests__/` | `<Component>.test.ts` | `cd frontend && npm run test` |
| Store 测试 | `stores/__tests__/` | `<store>.test.ts` | `cd frontend && npm run test` |

Go 测试约定：
- 采用标准 `testing` 包，白盒测试（与被测文件同包）
- 表驱动测试 + `t.Run`

前端测试约定：
- 使用 Vitest + happy-dom
- 测试文件放在目标组件旁的 `__tests__/` 目录

---

## 八、Git 提交 & 分支

| 类别 | 规则 |
|------|------|
| 提交信息 | 中文，概述原因（不要"修改xx文件"而要"修复xx问题"） |
| 分支名 | `feature/xxx` `fix/xxx` `refactor/xxx` |
| 提交粒度 | 一个逻辑改动一次提交，不混入无关修改 |
| 禁止提交 | `.env`、凭据、`node_modules/`、`src/frontend/dist/` |

---

## 九、反模式速查

| 反模式 | 问题 | 正确做法 |
|--------|------|---------|
| `internal/utils/` 通用工具包 | 变垃圾桶 | 按业务拆分到各包 |
| 循环依赖 | 编译失败、逻辑纠缠 | 通过接口 + core.go 注入解耦 |
| `*Service`（绑定层）含业务逻辑 | 职责不清、难测试 | 绑定层只校验 + 调用 + 返回 |
| `components/` 引用 `views/` | 分层违规 | views 引用 components |
| Go 文件超过 500/800 行 | 难维护 | 按领域拆分子文件 |
| 在 `main.ts` 中写大量路由配置 | 模块不清晰 | 抽离到 `router/index.ts` |
| 前端类型定义分散在各 `.vue` 中 | 类型不可复用 | 提取到 store 或就地声明 |
| `internal/` 外的 Go 文件（非 `package main`） | 违反 Go 惯例 | 放入 `internal/` |
| 手动编辑 `src/frontend/bindings/` | 自动生成，下次构建覆盖 | 通过 `wails3 generate bindings` 重新生成 |
| 超过 2 层间接调用 | 难追踪、难调试 | 扁平化调用链 |
| `fmt.Errorf("...: %v", err)` | 断错误链 | 用 `%w` |
| `interface{}` 替代泛型 | 类型不安全 | 用 `ConfigStore[T]` 泛型 |
| 持有锁时调用外部函数 | 死锁风险 | 先释放锁再调用 |
| `panic` / `log.Fatal`（非 main） | 无法优雅恢复 | 返回 error |
| 前端 `mixins` / `extends` | 已废弃 | 用 composables |
| 业务服务内嵌大型 struct 复用代码 | 模拟继承，耦合 | 拆为协作者 + 编排者 |
| `delete(map, key)` 清状态后用零值判断 | 零值与未设置混淆 | 显式值设置或存在性检查 |
