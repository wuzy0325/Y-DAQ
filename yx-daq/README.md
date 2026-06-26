# YX-DAQ

多采集设备数据采集、显示、保存桌面应用；支持运动控制器结合采集设备进行五孔探针、三孔探针等移位布点插值测试及数据导出。

基于 Wails v3（Go 1.23 + Vue 3 + TypeScript + Vite 3 + Element Plus + ECharts 6）。

## 技术栈

| 层 | 技术 |
|---|------|
| 桌面框架 | Wails v3.0.0-alpha.84 |
| 后端 | Go 1.23+ |
| 前端 | Vue 3 + TypeScript + Vite 3 |
| UI | Element Plus + Sass |
| 状态管理 | Pinia 3 |
| 图表 | ECharts 6 |
| 路由 | Vue Router 4（hash 模式） |
| PDF | go-pdf/fpdf |
| 前端测试 | Vitest + happy-dom |

## 环境要求

- Go 1.23+
- Node.js 18+
- Wails CLI v3：`go install github.com/wailsapp/wails/v3/cmd/wails3@latest`
- Windows 10/11（当前仅支持 Windows）

## 快速开始

```bash
# 开发模式（热重载）
wails3 dev

# 构建 exe
build.bat

# 构建 + NSIS 安装包
build.bat nsis

# 清理产物
build.bat clean
```

构建产物输出到 `bin\yx-daq.exe`；安装包输出到 `bin\yx-daq-amd64-installer.exe`。

## 功能

- **多设备采集** — 同时连接最多 10 台 XY-DAQ8/16 采集设备，每设备 1000 Hz，互不干扰
- **设备管理** — 添加/编辑/删除设备配置，UDP 自动扫描，支持模拟设备调试
- **运动控制** — 连接 B140 运动控制器，4 轴（X/Y/Z/U）点动、定位、回零、急停
- **移位插值测试** — 五孔探针和三孔探针的移位布点插值测试，配置布点模式（直线/矩形/自定义），自动遍历采集
- **实时显示** — UI 刷新频率 1–10 Hz 可调，ECharts 实时折线图（霓虹暗色主题），通道数据面板
- **数据存储** — 支持录制全部设备全通道数据，CSV 导出
- **数据回放** — 加载历史录制文件，播放/暂停/调速（0.25x–4x）
- **PDF 报告** — 测试结果导出为 PDF 报告

## 性能目标

| 指标 | 规格 |
|------|------|
| 单设备采样率 | 1000 Hz |
| 最大设备数 | 10 台 |
| 总吞吐量 | 10,000 帧/秒（~2 MB/s） |
| UI 刷新 | 1–10 Hz 可调 |
| 运行时长 | 7×24 小时设计 |

详见 [docs/perf-spec.md](./docs/perf-spec.md)。

## 项目结构

```
yx-daq/
├── main.go              # Go 入口，//go:embed all:frontend/dist
├── app/                 # 核心应用包（重构后）
│   ├── app.go          # Wails 绑定（~50 API 方法），主应用结构
│   ├── event_publishers.go  # 事件发布器实现
│   └── handlers/       # API 处理器（作为 App struct 方法）
│       ├── handlers_3h.go      # 三孔测试 API
│       ├── handlers_motion.go  # 运动控制 API
│       ├── handlers_device.go  # 设备管理 API
│       ├── handlers_calib.go   # 校准 API
│       ├── handlers_config.go  # 配置管理 API
│       └── handlers_data.go   # 数据管理 API
├── build.bat           # 一键构建脚本
├── wails.json          # Wails 配置
├── internal/           # 内部业务包（按领域划分）
│   ├── types/          # 共享类型定义
│   ├── driver/         # 硬件驱动（XY-DAQ16 TCP、B140 TCP、模拟设备）
│   ├── manager/        # 管理器（DeviceManager、MotionControllerManager、AcquisitionHub）
│   ├── calibration/    # 五孔探针校准服务
│   ├── three_hole/     # 三孔移位测试服务
│   ├── storage/        # JSON 配置持久化、CSV 录制、PDF 报告
│   └── scanner/        # UDP 设备扫描
└── frontend/
    ├── src/
    │   ├── main.ts     # 前端入口
    │   ├── stores/     # Pinia 状态管理
    │   ├── views/      # 页面（Dashboard、Device、Motion、ThreeHoleTest、Settings、Data）
    │   ├── components/ # 通用组件（GlassCard、ChartPanel、StatusIndicator、ValueDisplay）
    │   └── layouts/    # MainLayout（侧边栏导航）
    └── wailsjs/        # Wails 自动生成绑定（勿手动编辑）
```

### 架构特点

- **领域驱动设计** - 按业务功能分包，职责清晰
- **Wails v3 最佳实践** - 所有处理器作为 App struct 的方法
- **低耦合高内聚** - 消除了跨包依赖，提高可维护性
- **事件驱动架构** - 统一的事件发布机制，前后端解耦

## 命令速查

| 操作 | 命令 |
|------|------|
| 开发模式（热重载） | `wails3 dev` |
| Go 编译检查 | `go build ./...` |
| Go 测试 | `go test ./internal/...` |
| 前端类型检查 + 构建 | `cd frontend && npm run build` |
| 前端测试 | `cd frontend && npm run test` |
| 前端测试（监听） | `cd frontend && npm run test:watch` |
| 构建 exe | `build.bat` |
| 构建 + 安装包 | `build.bat nsis` |
| 清理 | `build.bat clean` |

## 配置存储

所有配置和录制文件存储在 `~/.yx-daq/`（用户 home 目录），JSON 格式原子写入。

## 开发规范

### 代码组织原则

1. **单一职责** - 每个包专注于特定业务领域
2. **包边界** - `internal/` 包只被内部引用，避免循环依赖
3. **API 设计** - 所有公开方法必须通过 App struct 的方法暴露
4. **事件驱动** - 使用统一的事件发布器进行前后端通信

### 添加新功能

1. 在 `internal/{domain}/` 创建相应的服务包
2. 在 `app/handlers_*.go` 中添加对应的 API 方法
3. 在 `app/event_publishers.go` 中添加事件发布支持
4. 更新前端调用

### 构建验证

```bash
# 验证后端编译
go build ./internal/...
go build main.go

# 验证前端构建
cd frontend && npm run build

# 运行测试
go test ./internal/...
cd frontend && npm run test
```

详见 [docs/dev-guide.md](./docs/dev-guide.md)。
