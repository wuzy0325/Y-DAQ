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
cd src && wails3 dev

# 构建 exe
cd src && wails3 task build

# 构建 + NSIS 安装包
cd src && wails3 task package

# 清理产物
rmdir /s /q bin && rmdir /s /q src\frontend\dist
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
YX-DAQ/
├── src/                      # 源码归拢目录（Wails v3 工程根 / Go 模块根）
│   ├── main.go               # Go 入口，//go:embed all:frontend/dist
│   ├── go.mod / wails.json / Taskfile.yml
│   ├── internal/             # 内部业务包（按领域划分）
│   │   ├── app/              # Wails v3 服务绑定层（8 个 Service + Core DI）
│   │   ├── types/            # 共享类型定义
│   │   ├── driver/           # 硬件驱动（XY-DAQ16 TCP、YX-DAQT TCP、B140 TCP、模拟设备）
│   │   ├── manager/          # 管理器（DeviceManager、MotionControllerManager、AcquisitionHub）
│   │   ├── calibration/      # 五孔探针校准服务
│   │   ├── three_hole/       # 三孔移位测试服务
│   │   ├── five_hole/        # 五孔移位测试服务（1-3 探针）
│   │   ├── storage/          # JSON 配置持久化、CSV 录制、PDF 报告
│   │   ├── scanner/          # UDP 设备扫描
│   │   └── logger/           # 结构化日志
│   ├── frontend/             # Vue 3 前端
│   │   ├── src/              # 前端源码（views/stores/components/composables）
│   │   └── bindings/         # Wails v3 自动生成绑定（勿手动编辑）
│   └── build/                # Wails 构建资源 + Taskfile 子任务
├── docs/                     # 所有文档（engineering/business/adr/hardware/...）
├── bin/                      # 构建产物（yx-daq.exe）
└── AGENTS.md / CLAUDE.md / CONTEXT.md / README.md
```

### 架构特点

- **领域驱动设计** - 按业务功能分包，职责清晰
- **Wails v3 服务层** - `internal/app/` 8 个 Service + Core 依赖注入
- **低耦合高内聚** - `internal/app/core.go` 为唯一 DI 汇聚点，避免循环依赖
- **事件驱动架构** - 统一的事件发布机制，前后端解耦

## 命令速查

> 所有 `wails3`/`go` 命令在 `src/` 目录运行。

| 操作 | 命令 |
|------|------|
| 开发模式（热重载） | `cd src && wails3 dev` |
| Go 编译检查 | `cd src && go build ./...` |
| Go 测试 | `cd src && go test ./internal/...` |
| 前端类型检查 + 构建 | `cd src/frontend && npm run build` |
| 前端测试 | `cd src/frontend && npm run test` |
| 前端测试（监听） | `cd src/frontend && npm run test:watch` |
| 构建exe | `cd src && wails3 task build` |
| 构建 + 安装包 | `cd src && wails3 task package` |
| 清理 | `rmdir /s /q bin && rmdir /s /q src\frontend\dist` |

## 配置存储

所有配置和录制文件存储在 `~/.yx-daq/`（用户 home 目录），JSON 格式原子写入。

## 开发规范

### 代码组织原则

1. **单一职责** - 每个包专注于特定业务领域
2. **包边界** - `src/internal/` 包只被内部引用，避免循环依赖
3. **服务绑定** - 所有公开方法通过 `src/internal/app/service_*.go` 暴露
4. **事件驱动** - 使用统一的事件发布器进行前后端通信

### 添加新功能

1. 在 `src/internal/{domain}/` 创建相应的服务包
2. 在 `src/internal/app/service_*.go` 中添加对应的 Service 方法
3. 在 `src/internal/app/event_publishers.go` 中添加事件发布支持
4. 更新前端调用

### 构建验证

```bash
# 验证后端编译
cd src && go build ./internal/...
cd src && go build ./...

# 验证前端构建
cd src/frontend && npm run build

# 运行测试
cd src && go test ./internal/...
cd src/frontend && npm run test
```

详见 [docs/dev-guide.md](./docs/dev-guide.md)。
