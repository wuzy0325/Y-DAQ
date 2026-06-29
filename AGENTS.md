# YX-DAQ —— Agent instructions

Windows-only Wails v3 desktop app (Go 1.23 + Vue 3 + TypeScript + Vite 3 + Element Plus + ECharts 6).
多采集设备数据采集、显示、保存，设备管理与配置；连接运动控制器结合采集设备进行五孔探针、三孔探针等移位布点插值测试及数据导出。

> **渐进式披露**：本文件是 AI Agent 的唯一主入口，仅含每次必读的核心规则。详细说明按需读取子文档（见末尾"详细参考"）。

## Prerequisites

| Dependency | Install | Verify |
|------------|---------|--------|
| Go 1.23+ | https://go.dev/dl/ | `go version` |
| Node.js 18+ | https://nodejs.org/ | `node --version` |
| Wails CLI v3 alpha | `go install github.com/wailsapp/wails/v3/cmd/wails3@latest` | `wails3 version` |
| NSIS 3.x | https://nsis.sourceforge.io/Download → 安装后将 `C:\Program Files (x86)\NSIS` 加入系统 PATH | `makensis /VERSION` |

> **Wails v3 注意**: 本项目使用 Wails v3（`wails3`），不是 PATH 中的 `wails`(v2)。用错命令会报 `Unable to find Wails in go.mod`。
> **NSIS PATH 注意**: `wails3 task package` 需要 `makensis` 在系统 PATH 中；若未配置 PATH，可手动调用 `C:\Program Files (x86)\NSIS\makensis.exe`。

## Build Procedure

> 所有 `wails3` / `go` 命令必须在 `src/` 目录运行（Wails v3 工程根 + Go 模块根）。

### 1. 仅构建 exe（生产版）
```bat
cd src
wails3 task build
```
产物: `bin\yx-daq.exe`（直接运行，需目标机已装 WebView2）

### 2. 构建 exe（调试版，含 DevTools）
```bat
cd src
wails3 task build DEV=true
```

### 3. 开发模式（热重载）
```bat
cd src
wails3 dev
```

### 4. 构建 exe + NSIS 安装包
```bat
cd src
wails3 task package
```
产物:
- `bin\yx-daq.exe` — 程序本体
- `bin\yx-daq-amd64-installer.exe` — NSIS 安装包

### 5. 清理
```bat
rmdir /s /q bin
rmdir /s /q src\frontend\dist
```

## Commands

| What | How |
|------|------|
| Dev mode (hot reload) | `cd src && wails3 dev` |
| Build exe (production) | `cd src && wails3 task build` |
| Build exe (debug) | `cd src && wails3 task build DEV=true` |
| Build + installer | `cd src && wails3 task package` |
| Clean artifacts | `rmdir /s /q bin && rmdir /s /q src\frontend\dist` |
| Build bindings | `cd src && wails3 generate bindings -clean=true -ts` |
| Go compile check | `cd src && go build ./...` |
| Go linter | `cd src && golangci-lint run ./internal/...` |
| Frontend typecheck + build | `cd src/frontend && npm run build` (vue-tsc --noEmit then vite build) |
| Frontend lint | `cd src/frontend && npm run lint` |
| Frontend tests (happy-dom) | `cd src/frontend && npm run test` (vitest) |
| Frontend tests (watch) | `cd src/frontend && npm run test:watch` |
| Go tests | `cd src && go test ./internal/...` |
| Go tests (race detector, 需 CGO_ENABLED=1 + gcc) | `cd src && wails3 task test:race` |
| Go tests (verbose + race) | `cd src && wails3 task test:verbose` |

Test files (pattern: `src/**/*.{test,spec}.{js,ts}`): 按 `src/internal/` 和 `src/frontend/src/` 目录分布，含 `_test.go`（Go 单元/并发/状态机测试）和 `__tests__/`（Vitest 组件测试）

## Architecture（速查）

> 所有源码（Go + 前端 + Wails 工程文件）统一在 `src/` 下。`wails3`/`go` 命令在 `src/` 运行，构建产物输出到仓库根 `bin/`。

- `src/main.go` — entrypoint, creates `Core`, embeds `frontend/dist` via `//go:embed`, Wails v3 `application.New()` with 8 Services
- `src/internal/app/` — Wails v3 service layer: `Core` (lifecycle/DI), `CoreService`, `DeviceService`, `MotionService`, `ThreeHoleService`, `FiveHoleService`, `CalibrationService`, `DataService`, `ConfigService`. Events: `daq:data-snapshot`, `device:status-updated`, `motion:status-updated`, `calibration:*`, `three-hole:*`, `five-hole:*`
- `src/internal/types/` — shared types and constants (包括五孔和三孔探针类型，零依赖)
- `src/internal/driver/` — hardware drivers: `xy_daq16.go` / `yx_daqt.go` (TCP 采集), `b140.go` (motion TCP), `simulated_device.go`, `simulated_motion.go`
- `src/internal/manager/` — `DeviceManager`, `MotionControllerManager` (10 Hz poll), `AcquisitionHub` (20 Hz publish)
- `src/internal/calibration/` — **五孔探针校准**业务（`service.go` + `formulas.go`，当前隐藏）
- `src/internal/three_hole/` — **三孔移位插值测试**业务（单探针，编排者+协作者结构：service/test_manager/data_processor/interpolator/point_generator/realtime_recorder/event_handler/csv_writer）
- `src/internal/five_hole/` — **五孔移位插值测试**业务（1-3 探针，结构与 three_hole 平行，多 `motion_coordinator.go`）
- `src/internal/storage/` — JSON config persistence (`~/.yx-daq/`), CSV recording, PDF report (go-pdf/fpdf)
- `src/internal/scanner/` — UDP device scanner
- `src/internal/logger/` — 结构化日志（log/slog）
- Frontend (`src/frontend/`): views (Dashboard, Device, Motion, ThreeHoleTest, FiveHoleTest, Settings, Calibration[隐藏]), Pinia stores, hash-based routing via `vue-router`. Wails v3 绑定直接调用：Store/View 静态 import `@bindings/yx-daq/internal/app`（Service 对象）+ `@wailsio/runtime`（Events）

## Conventions（核心约束）

**必须遵守以下文档**：
- `docs/engineering/architecture.md` —— 架构与设计规范（目录结构、设计原则、接口设计、设计模式、反模式）
- `docs/engineering/coding-standards.md` —— 编码规范（Go + 前端，权威）

核心速查：
- Config/recordings stored in `~/.yx-daq/` (user home directory)
- `//go:embed all:frontend/dist` in `src/main.go` — frontend must be built before Go build（embed 路径相对 `src/main.go`，即 `src/frontend/dist/`）
- SCSS: Vite auto-injects `@use "@/assets/styles/variables.scss" as *;` globally
- TS path alias: `@` → `/src`, `@bindings` → `bindings` (configured in `src/frontend/vite.config.ts`)
- Vitest uses `happy-dom` environment (not jsdom)
- Wails v3 auto-generates `src/frontend/bindings/` (TypeScript, `@wailsio/runtime`) — **禁止手动编辑**
- All external strings in Chinese (UI labels, error messages, file dialogs)
- Axes: X=purple, Y=cyan, Z=green, U=orange (UI convention)
- Go 错误包装用 `%w`，日志用 `log/slog`（结构化：`slog.Error("msg", "err", err)`）
- 前端 Store/View 中 Wails 绑定必须静态 import `@bindings/yx-daq/internal/app`，事件用 `@wailsio/runtime` 的 `Events.On/Off`（回调参数 `{data: T}`，需 `event.data` 解包）
- 前端样式必须 `<style lang="scss" scoped>`

## Directory & File Layout Rules

**必须遵守 `docs/engineering/architecture.md`**，核心摘要如下：

- 源码统一在 `src/` 下（`src/main.go` + `src/internal/` + `src/frontend/` + `src/build/` + `src/wails.json` + `src/Taskfile.yml` + `src/go.mod`）
- `src/internal/` 按业务领域分包（types → driver → manager → calibration/three_hole/five_hole/storage），`src/internal/app/core.go` 为依赖注入汇聚点，禁止循环依赖
- 业务服务采用编排者 + 协作者模式（Service 持有 TestManager/DataProcessor 等指针，自身只编排）
- 前端 `src/frontend/src/views/` 放页面组件，`components/` 放通用组件，`stores/` 放 Pinia 状态，组件不引用 `views/`
- Go 文件小写加下划线，Vue 组件 PascalCase，store 文件 camelCase
- 所有 `.md` 文档放入 `docs/`，根目录不新增
- 事件通道命名 `<domain>:<action>`
- Wails v3 绑定直接调用：`src/frontend/bindings/`（v3 自动生成，禁止手动编辑）→ Store/View 静态 import

## MCP 工具核心规则

> 详细使用说明见 [docs/agents/mcp-tools.md](docs/agents/mcp-tools.md)。

- **修改符号前必须运行** `gitnexus_impact` 分析影响范围，HIGH/CRITICAL 风险必须警告用户
- **提交前必须运行** `gitnexus_detect_changes()` 验证改动范围
- **探索代码优先用 graph 工具**（`semantic_search_nodes` / `query_graph`），而非 Grep/Glob/Read
- **重命名用** `gitnexus_rename`，禁止 find-and-replace

## 详细参考（按需读取）

| 主题 | 文档 | 何时读取 |
|------|------|---------|
| AI 行为准则 | [docs/agents/ai-behavior.md](docs/agents/ai-behavior.md) | 开始任务前需要思考框架 |
| MCP 工具详细说明 | [docs/agents/mcp-tools.md](docs/agents/mcp-tools.md) | 需要用 GitNexus / code-review-graph 时 |
| ECC Skills & Agents | [docs/agents/ecc-skills.md](docs/agents/ecc-skills.md) | 需要 `/plan`、`/verify`、`/code-review` 等 agent 时 |
| 架构与设计规范 | [docs/engineering/architecture.md](docs/engineering/architecture.md) | 设计模块接口、查找目录结构规则 |
| 编码规范 | [docs/engineering/coding-standards.md](docs/engineering/coding-standards.md) | 写代码前查命名/分层/反模式 |
| 性能规格 | [docs/engineering/perf-spec.md](docs/engineering/perf-spec.md) | 涉及性能优化时 |
| UI 规范 | [docs/engineering/ui-guidelines.md](docs/engineering/ui-guidelines.md) | 修改前端 UI 时 |
| 领域语言 | [CONTEXT.md](CONTEXT.md) | 理解业务术语（五孔/三孔/探针/布点等） |
