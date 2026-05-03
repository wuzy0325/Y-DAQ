# YX-DAQ 架构与设计规范

> 本文档定义项目目录结构、设计原则与架构约束。编码细节见 `coding-standards.md`，性能规格见 `perf-spec.md`。

---

## 一、核心设计原则

### 1.1 职责单一（Single Responsibility）

每个模块、结构体、函数只做一件事，只有一个变更理由。

**Go 后端**：
- 每个 `internal/` 子包对应一个业务领域（types / driver / manager / calibration / three_hole / storage / scanner）
- 每个 struct 只承担一个角色：`Service` 负责业务编排，`Driver` 负责硬件通信，`Manager` 负责生命周期管理，`Store` 负责持久化
- Handler 方法（`app.go` / `handlers_*.go`）只做参数校验 + 调用 service + 返回结果，不含业务逻辑
- 文件行数上限：单文件 500 行，service 主文件 800 行（超出则按功能拆分子文件）

**前端**：
- `views/` 只做页面编排，不写可复用组件逻辑
- `components/` 只做 UI 展示与交互，不包含业务语义
- `stores/` 只做状态管理与 API 调用，不包含 DOM 操作
- `composables/` 只做逻辑复用，不引用 store，不包含 UI

**判断标准**：如果一个 struct 的方法列表无法用一个短语概括，它就承担了过多职责。

### 1.2 模块化（Modularity）

高内聚、低耦合。模块间通过窄接口通信，不暴露内部细节。

- Go 包间依赖方向：`types → driver → manager → calibration/three_hole/storage`，**禁止循环依赖**
- 跨包调用只通过接口，不直接依赖具体实现
- `app.go` 是唯一的依赖注入汇聚点，所有跨层回调在此连接
- 前端组件分层：`views → components`（单向），`stores` 被两者引用，`components` 不引用 `views`

### 1.3 前后端分离

Go 后端与 Vue 前端职责明确，通过 Wails 绑定层通信。

| 层 | 职责 | 禁止 |
|----|------|------|
| Go `internal/` | 业务逻辑、硬件通信、数据持久化 | 不包含 UI 逻辑、不返回 HTML/样式 |
| Go `app.go` / `handlers_*.go` | Wails 绑定层，参数校验 + 调用 service | 不含业务逻辑 |
| Vue `stores/` | 状态管理、调用 Wails API、事件监听 | 不直接操作 DOM |
| Vue `views/` + `components/` | UI 渲染与用户交互 | 不包含业务算法 |

- 后端通过 `EventsEmit` 推送数据，前端通过 `EventsOn` 监听
- 事件通道命名：`<domain>:<action>`（如 `daq:data-snapshot`、`three-hole:progress`）
- 前端不假设后端内部结构，只依赖 Wails 生成的类型（`wailsjs/go/models`）

---

## 二、顶层目录结构

```
yx-daq/
├── main.go                   # Wails 入口
├── app.go                    # App 结构体 + startup/shutdown + DI
├── handlers_device.go        # Wails 绑定：设备管理
├── handlers_motion.go        # Wails 绑定：运动控制
├── handlers_calib.go         # Wails 绑定：五孔校准
├── handlers_3h.go            # Wails 绑定：三孔测试
├── handlers_data.go          # Wails 绑定：录制/回放/数据
├── handlers_config.go        # Wails 绑定：配置/路径
├── AGENTS.md                 # Agent 指令（AI 开发用）
├── CLAUDE.md                 # AI 行为准则 + 规范引用
├── wails.json                # Wails v2 项目配置
├── go.mod / go.sum           # Go 模块依赖
├── build.bat                 # Windows 构建（exe / nsis / clean）
├── dev.bat                   # Windows 开发（wails dev / build+run）
├── .gitignore
│
├── internal/                 # Go 后端（业务逻辑、驱动、存储）
├── frontend/                 # Vue 3 前端
├── build/                    # Wails 构建资源（图标、安装脚本等）
└── docs/                     # 所有 .md 文档（设计文档、审查报告、规范）
```

根目录只保留：

| 文件 | 理由 |
|------|------|
| `README.md` | GitHub/仓库介绍 |
| `AGENTS.md` | AI 开发助手指令（开发流程必需） |
| `CLAUDE.md` | AI 行为准则 + 规范引用（开发流程必需） |

> 注：新建文档一律放入 `docs/`，根目录非必要不新增 `.md` 文件。

---

## 三、Go 后端（`internal/`）

### 3.1 包划分

每个包一个职责，按**业务领域**而非技术分层划分：

```
internal/
├── types/                    # 共享类型、常量、枚举（纯定义，无业务逻辑，零依赖）
│   ├── device.go
│   ├── motion.go
│   ├── calibration.go
│   ├── three_hole_traversal.go
│   └── constants.go
│
├── driver/                   # 硬件驱动（采集设备 + 运动控制器）
│   ├── xy_daq16.go            # DAQ-16 TCP 驱动
│   ├── b140.go                # B140 运动控制器 TCP 驱动
│   ├── simulated_device.go    # 模拟采集设备
│   └── simulated_motion.go   # 模拟运动控制器
│
├── manager/                  # 管理器（运行时状态、协调），定义接口
│   ├── device_manager.go      # 采集设备生命周期管理
│   ├── motion_manager.go      # 运动控制器生命周期管理
│   └── acquisition_hub.go     # 数据采集汇总与分发
│
├── scanner/                  # UDP 设备扫描发现
│   └── daq_scanner.go
│
├── storage/                  # 数据持久化（配置、记录、报表）
│   ├── config_store.go        # JSON 配置读写
│   ├── data_storage.go        # CSV 实时记录
│   └── pdf_report.go          # PDF 报告生成
│
├── calibration/              # 五孔探针校准业务
│   ├── service.go             # 校准服务
│   ├── formulas.go            # 计算公式
│   ├── encoder_compensation.go# 编码器补偿状态机
│   └── sphere_tank_gate.go    # 球罐闸门控制
│
└── three_hole/               # 三孔探针移位测试业务
    ├── service.go             # 测试生命周期 + 主循环
    ├── interpolator.go        # 插值算法
    └── csv_writer.go          # CSV 导出
```

- `types/` 是唯一无内部依赖的包
- `manager` 包定义接口（`DeviceDriver`, `MotionController`），`driver` 包实现它们
- `calibration` 和 `three_hole` 结构平行，模式相同
- `app.go` 是依赖注入的汇聚点，`manager/` 不直接依赖 `calibration/` 或 `three_hole/`（通过 `app.go` 回调注入）

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
app.go → 所有 internal 包（注入依赖）
```

- 不允许循环依赖

### 3.3 每包最大行数

| 包 | 建议上限 | 超限处理 |
|----|---------|---------|
| `types/` 单文件 | 300 行 | 按领域拆文件 |
| `driver/` 单文件 | 500 行 | 按协议版本/功能拆 |
| `three_hole/service.go` | 800 行 | 拆为 `controller.go` / `planner.go` |
| 其他 service | 500 行 | 拆辅助逻辑到子文件 |
| `package main` 单文件 | 500 行 | 拆为 `handlers_xxx.go` |

---

## 四、前端（`frontend/src/`）

### 4.1 目录结构

```
frontend/src/
├── api/                      # 枚举、常量、API 类型（与 Wails 无关的）
│   └── enums.ts
│
├── stores/                   # Pinia 状态管理
│   ├── device.ts              # 采集设备 store
│   ├── motion.ts              # 运动控制器 store
│   ├── calibration.ts         # 五孔校准 store
│   └── threeHoleTest.ts       # 三孔测试 store
│
├── views/                    # 页面级组件（对应路由）
│   ├── DashboardView.vue      # 仪表盘
│   ├── DeviceView.vue         # 设备管理
│   ├── MotionView.vue         # 运动控制
│   ├── ThreeHoleTestView.vue  # 三孔测试
│   ├── CalibrationView.vue    # 五孔校准（当前隐藏）
│   ├── DataView.vue           # 数据查看
│   └── SettingsView.vue       # 设置
│
├── components/               # 通用可复用组件
│   ├── GlassCard.vue
│   ├── ChartPanel.vue
│   ├── ValueDisplay.vue
│   ├── StatusIndicator.vue
│   ├── CalibPointEditor.vue
│   └── MotionControl/         # 运动控制相关子组件
│       ├── AxisConfigDialog.vue
│       └── AxisControlCard.vue
│
├── composables/              # 逻辑复用（不引用 store，不含 UI）
│
├── layouts/                  # 布局组件
│   └── MainLayout.vue         # 主导航 + 页面容器
│
├── router/                   # 路由配置
│   └── index.ts
│
├── utils/                    # 工具函数
│   └── format.ts
│
├── assets/                   # 静态资源
│   ├── styles/
│   │   ├── variables.scss     # 全局变量（Vite 自动注入）
│   │   ├── global.scss        # 全局样式
│   │   └── themes/
│   │       └── theme-variables.scss
│   ├── fonts/
│   └── images/
│
├── main.ts                   # Vue 应用入口
├── App.vue
└── vite-env.d.ts
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
- `node_modules` 和 `wailsjs/` 不提交 git（已在 `.gitignore`）

### 4.3 Wails 事件命名

`<domain>:<action>`

```
采集:      daq:data-snapshot
运动:      motion:status-updated
三孔:      three-hole:progress
           three-hole:realtime
           three-hole:complete
           three-hole:error
五孔:      calibration:* （沿用现有，保持不变）
```

---

## 五、类与接口设计

### 5.1 结构体设计原则

- 每个导出 struct 必须有 `NewXxx()` 构造函数，返回指针
- 必要依赖通过构造函数注入，可选/循环依赖通过 Setter 注入
- struct 字段按可见性分组：导出字段在前，非导出字段在后
- 状态字段与配置字段分离：配置通过构造函数/setter 一次性设置，状态通过方法修改

```go
type CalibrationService struct {
    // 依赖（构造注入）
    eventPublisher ThreeHoleEventPublisher

    // 状态（内部管理）
    mu     sync.Mutex
    status Status
    running atomic.Bool

    // 通道（生命周期控制）
    cancelCh  chan struct{}
    pauseCh   chan struct{}
    resumeCh  chan struct{}
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

### 5.3 嵌入与继承

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
| 接口隐式实现 | 驱动多态 | `DeviceDriver` / `SimulatedDevice` | Go 鸐式实现，零声明成本 |
| 事件发布/订阅 | 后端→前端数据推送 | `threeHoleEventPublisher` | Wails 事件机制，解耦前后端 |
| 服务生命周期 | 长运行任务 | Start/Pause/Resume/Stop | 统一模式，易理解 |
| 状态机 | 编码器补偿 | `encoder_compensation.go` | 状态转换明确，防非法状态 |
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
| 禁止提交 | `.env`、凭据、`node_modules/`、`frontend/dist/` |

---

## 九、反模式速查

| 反模式 | 问题 | 正确做法 |
|--------|------|---------|
| `internal/utils/` 通用工具包 | 变垃圾桶 | 按业务拆分到各包 |
| 循环依赖 | 编译失败、逻辑纠缠 | 通过接口 + app.go 注入解耦 |
| Handler 含业务逻辑 | 职责不清、难测试 | Handler 只校验 + 调用 + 返回 |
| `components/` 引用 `views/` | 分层违规 | views 引用 components |
| Go 文件超过 500/800 行 | 难维护 | 按领域拆分 handlers_xxx.go / 子文件 |
| 在 `main.ts` 中写大量路由配置 | 模块不清晰 | 抽离到 `router/index.ts` |
| 前端类型定义分散在各 `.vue` 中 | 类型不可复用 | 提取到 store 或就地声明 |
| `internal/` 外的 Go 文件（非 `package main`） | 违反 Go 惯例 | 放入 `internal/` |
| 手动编辑 `frontend/wailsjs/` | 自动生成，下次构建覆盖 | 通过 wails 命令重新生成 |
| 超过 2 层间接调用 | 难追踪、难调试 | 扁平化调用链 |
| `fmt.Errorf("...: %v", err)` | 断错误链 | 用 `%w` |
| `interface{}` 替代泛型 | 类型不安全 | 用 `ConfigStore[T]` 泛型 |
| 持有锁时调用外部函数 | 死锁风险 | 先释放锁再调用 |
| `panic` / `log.Fatal`（非 main） | 无法优雅恢复 | 返回 error |
| 前端 `mixins` / `extends` | 已废弃 | 用 composables |
