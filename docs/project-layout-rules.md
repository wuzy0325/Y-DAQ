# 项目目录与文件组织规则

## 一、顶层结构

```
yx-daq/
├── main.go                   # Wails 入口
├── app.go                    # App 结构体 + Wails 绑定
├── AGENTS.md                 # Agent 指令（AI 开发用）
├── wails.json                # Wails v2 项目配置
├── go.mod / go.sum           # Go 模块依赖
├── build.bat                 # Windows 构建（exe / nsis / clean）
├── dev.bat                   # Windows 开发（wails dev / build+run）
├── .gitignore
│
├── internal/                 # Go 后端（业务逻辑、驱动、存储）
├── frontend/                 # Vue 3 前端
├── build/                    # Wails 构建资源（图标、安装脚本等）
├── docs/                     # 设计文档、审查报告、规范（仅 .md）
└── docs/                     # 所有 .md 文档
```

---

## 二、Go 后端（`internal/`）

### 2.1 包划分原则

每个包一个职责，按**业务领域**而非技术分层划分：

```
internal/
├── types/                    # 共享类型、常量、枚举（纯定义，无业务逻辑）
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
├── manager/                  # 管理器（运行时状态、协调）
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

### 2.2 命名规则

| 类别 | 规则 | 示例 |
|------|------|------|
| 包名 | 全小写，单数 | `driver`, `storage`, `calibration` |
| 文件 | 小写加下划线 | `config_store.go`, `pdf_report.go` |
| 导出类型 | PascalCase | `ThreeHoleTraversalService` |
| 导出函数 | PascalCase | `NewThreeHoleTraversalService()` |
| 接口 | `-er` 后缀 或 明确动词 | `ThreeHoleEventPublisher`, `BatchGetter` |
| 测试文件 | `_test.go` 后缀 | `formulas_test.go` |

### 2.3 包间依赖规则

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

- `types/` 是唯一无内部依赖的包
- `app.go` 是依赖注入的汇聚点（`SetBatchGetter`、`SetMotionController`）
- 不允许循环依赖
- `manager/` 不直接依赖 `calibration/` 或 `three_hole/`（通过 `app.go` 回调注入）

### 2.4 每包最大行数

| 包 | 建议上限 | 超限处理 |
|----|---------|---------|
| `types/` 单文件 | 300 行 | 按领域拆文件 |
| `driver/` 单文件 | 500 行 | 按协议版本/功能拆 |
| `three_hole/service.go` | 800 行 | 拆为 `controller.go` / `planner.go` |
| 其他 service | 500 行 | 拆辅助逻辑到子文件 |

---

## 三、前端（`frontend/src/`）

### 3.1 目录结构

```
frontend/src/
├── api/                      # 枚举、常量、API 类型（与 Wails 无关的）
│   └── enums.ts
│
├── types/                    # 前端领域类型/接口（非 Wails 生成，手动维护）
│   └── index.ts              # 可自行拆分
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
├── layouts/                  # 布局组件
│   └── MainLayout.vue         # 主导航 + 页面容器
│
├── router/                   # [推荐新增] 路由配置
│   └── index.ts              # 从 main.ts 中抽离
│
├── utils/                    # [推荐新增] 工具函数
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
├── main.ts                   # Vue 应用入口 + 路由注册（推荐逐步拆分）
├── App.vue
└── vite-env.d.ts
```

### 3.2 命名规则

| 类别 | 规则 | 示例 |
|------|------|------|
| Vue 组件 | PascalCase | `GlassCard.vue`, `ThreeHoleTestView.vue` |
| Store 文件 | camelCase | `device.ts`, `threeHoleTest.ts` |
| 类型/接口 | PascalCase，接口无 `I` 前缀 | `TraversalConfig`, `DeviceProfile` |
| 枚举 | PascalCase | `TraversalPattern`, `DeviceType` |
| 工具函数 | camelCase | `formatTimestamp()` |
| 目录名 | PascalCase（组件目录）/ camelCase（非组件） | `MotionControl/`, `stores/` |
| 测试文件 | `<Component>.test.ts` | `GlassCard.test.ts` |

### 3.3 组件分层

```
views/          → 页面级，对应路由，包含完整业务逻辑
components/     → 通用可复用，无页面级依赖
stores/         → 状态管理，可被 views 和 components 引用
api/ types/     → 纯定义，无运行时依赖
```

- `views/` 中的组件可以引用 `stores/`、`components/`、`api/`、`types/`
- `components/` 中的组件可以引用 `stores/`、`api/`、`types/`，但**不引用** `views/`
- 组件目录名反映组件用途：`MotionControl/` 内的组件只处理运动控制相关 UI
- `node_modules` 和 `wailsjs/` 不提交 git（已在 `.gitignore`）

### 3.4 导入顺序规范

每个 TS/TSVue 文件中的 `import` 按以下顺序分组（每组空行分隔）：

1. 外部库（`vue`, `pinia`, `element-plus`, `echarts` 等）
2. Wails 运行时（`wailsjs/runtime`, `wailsjs/go/main/App`）
3. 本地绝对路径（`@/stores/...`, `@/components/...`, `@/api/...`）
4. 相对路径（`../../components/...`）

### 3.5 事件命名

Wails 事件通道命名规则：`<domain>:<action>`

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

## 四、根目录文档

### 4.1 文档目录

```
docs/                         # 所有 .md 文档统一放在此
├── review-three-hole.md       # 审查报告
├── improvement-plan-three-hole.md
├── project-layout-rules.md    # 本文件
└── ...                        # 后续所有 .md 文档
```

### 4.2 根目录只保留

| 文件 | 理由 |
|------|------|
| `README.md` | GitHub/仓库介绍 |
| `AGENTS.md` | AI 开发助手指令（开发流程必需） |
| `DEV_GUIDE.md` | 新手开发指引（可选,或移入 docs/） |
| `PERF_SPEC.md` | 性能规格（可选,或移入 docs/） |
| `THREE_HOLE_BUSINESS.md` | 三孔业务说明（建议移入 docs/） |

> 注：新建文档一律放入 `docs/`，根目录非必要不新增 `.md` 文件。

---

## 五、测试文件

| 层次 | 位置 | 命名 | 运行方式 |
|------|------|------|---------|
| Go 后端 | 与被测文件同目录 | `<name>_test.go` | `go test ./internal/...` |
| Vue/TS 组件 | `components/__tests__/` | `<Component>.test.ts` | `npm run test` |
| Store 测试 | [未来] `stores/__tests__/` | `<store>.test.ts` | `npm run test` |

Go 测试约定：
- 采用标准 `testing` 包
- 测试文件与被测文件放在同一包内
- 命名：`formulas_test.go` 测试 `formulas.go`

前端测试约定：
- 使用 Vitest + happy-dom
- 测试文件放在目标组件旁的 `__tests__/` 目录
- 命名：`GlassCard.test.ts` 测试 `GlassCard.vue`

---

## 六、Git 提交 & 分支

| 类别 | 规则 |
|------|------|
| 提交信息 | 中文，概述原因（不要"修改xx文件"而要"修复xx问题"） |
| 分支名 | `feature/xxx` `fix/xxx` `refactor/xxx` |
| 提交粒度 | 一个逻辑改动一次提交，不混入无关修改 |
| 禁止提交 | `.env`、凭据、`node_modules/`、`frontend/dist/` |

---

## 七、违反示例（反模式）

| 反模式 | 问题 | 正确处理 |
|--------|------|---------|
| `internal/utils/` 通用工具包 | 变成垃圾桶，职责不清 | 按业务拆分到各包 |
| 前端 `components/` 中放页面组件 | 组件层次混乱 | 页面组件放入 `views/` |
| 一个 Go 文件超过 1000 行 | 难以维护 | 按功能拆分子文件 |
| 前端类型定义分散在各 `.vue` 中 | 类型不可复用 | 提取到 `types/` 或 `stores/` |
| `internal/` 外的 Go 文件（除 `app.go`, `main.go`） | 违反 Go 惯例 | 全部放入 `internal/` |
| 在 `main.ts` 中写大量路由配置 | 模块不清晰 | 抽离到 `router/index.ts` |
