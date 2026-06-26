# YX-DAQ —— Agent instructions

Windows-only Wails v3 desktop app (Go 1.23 + Vue 3 + TypeScript + Vite 3 + Element Plus + ECharts 6).
多采集设备数据采集、显示、保存，设备管理与配置；连接运动控制器结合采集设备进行五孔探针、三孔探针等移位布点插值测试及数据导出。

## Prerequisites

| Dependency | Install | Verify |
|------------|---------|--------|
| Go 1.23+ | https://go.dev/dl/ | `go version` |
| Node.js 18+ | https://nodejs.org/ | `node --version` |
| Wails CLI v3 alpha | `go install github.com/wailsapp/wails/v3/cmd/wails3@latest` | `wails3 version` |
| NSIS 3.x | https://nsis.sourceforge.io/Download → 安装后将 `C:\Program Files (x86)\NSIS` 加入系统 PATH | `makensis /VERSION` |

> **Wails v3 注意**: 本项目使用 Wails v3（`wails3`），不是 PATH 中的 `wails`(v2)。用错命令会报 `Unable to find Wails in go.mod`。
> **NSIS PATH 注意**: `build.bat nsis` 需要 `makensis` 在系统 PATH 中；若未配置 PATH，可手动调用 `C:\Program Files (x86)\NSIS\makensis.exe`。

## Build Procedure

### 1. 仅构建 exe（生产版）
```bat
cd yx-daq
wails3 task build
```
产物: `bin\yx-daq.exe`（直接运行，需目标机已装 WebView2）

### 2. 构建 exe（调试版，含 DevTools）
```bat
cd yx-daq
wails3 task build DEV=true
```

### 3. 开发模式（热重载）
```bat
cd yx-daq
wails3 dev
```

### 4. 构建 exe + NSIS 安装包
```bat
cd yx-daq
build.bat nsis
```
产物:
- `bin\yx-daq.exe` — 程序本体
- `bin\yx-daq-amd64-installer.exe` — NSIS 安装包

### 5. 清理
```bat
build.bat clean
```

## Commands

| What | How |
|------|------|
| Dev mode (hot reload) | `wails3 dev` |
| Build exe (production) | `wails3 task build` |
| Build exe (debug) | `wails3 task build DEV=true` |
| Build + installer | `build.bat nsis` |
| Clean artifacts | `build.bat clean` |
| Build bindings | `wails3 generate bindings -clean=true -ts` |
| Go compile check | `go build ./...` |
| Go linter | `golangci-lint run ./internal/...` |
| Frontend typecheck + build | `cd frontend && npm run build` (vue-tsc --noEmit then vite build) |
| Frontend lint | `cd frontend && npm run lint` |
| Frontend tests (happy-dom) | `cd frontend && npm run test` (vitest) |
| Frontend tests (watch) | `cd frontend && npm run test:watch` |
| Go tests | `go test ./internal/...` |

Test files (pattern: `src/**/*.{test,spec}.{js,ts}`):
- Go: `internal/calibration/formulas_test.go`, `internal/storage/config_store_test.go`
- Frontend: `frontend/src/components/__tests__/GlassCard.test.ts`, `StatusIndicator.test.ts`, `ValueDisplay.test.ts`

## ECC Skills & Agents

ECC agents and skills installed at system level (in `~/.claude/` and `~/.opencode/`).

**可用 agents**（通过 Task 工具调用）：
- `/plan` — 新功能实现规划
- `/architect` — 架构设计决策  
- `/code-review` — 代码质量审查
- `/go-review` — Go 代码专项审查
- `/go-build` — Go 构建错误修复
- `/go-test` — Go 测试编写/执行
- `/rust-review`, `/rust-build`, `/rust-test` — Rust 代码审查/构建/测试
- `/python-review` — Python 代码审查
- `/typescript-review` — TypeScript 专项审查
- `/database-review` — 数据库/存储设计审查
- `/tdd` — 测试驱动开发流程
- `/verify` — 验证循环（lint → test → build）
- `/refactor-clean` — 死代码清理
- `/security` — 安全审查
- `/checkpoint` — 保存检查点
- `/save-session` — 保存 session 摘要

**验证规范**：
- 修改 Go 后：`golangci-lint run ./internal/...` + `go build ./...`
- 修改前端后：`cd frontend && npm run lint` + `npm run build`
- 代码提交前：运行 `/verify`
- 复杂功能开发前：运行 `/plan`

## Architecture

- `main.go` — entrypoint, creates `Core`, embeds `frontend/dist` via `//go:embed`, Wails v3 `application.New()` with Services
- `internal/app/` — Wails v3 service layer: `Core` (lifecycle/DI), `CoreService`, `DeviceService`, `MotionService`, `ThreeHoleService`, `CalibrationService`, `DataService`, `ConfigService`. Events: `daq:data-snapshot`, `motion:status-updated`, `calibration:*`, `three-hole:*`
- `internal/types/` — shared types and constants (包括五孔和三孔探针类型)
- `internal/driver/` — hardware drivers: `xy_daq16.go` (TCP), `b140.go` (motion TCP), `simulated_device.go`, `simulated_motion.go`
- `internal/manager/` — `DeviceManager`, `MotionControllerManager` (10 Hz poll), `AcquisitionHub` (20 Hz publish)
- `internal/calibration/` — **五孔移位插值测试**服务（当前隐藏），含 formulas、encoder compensation 状态机、sphere tank gate
- `internal/three_hole/` — **三孔移位插值测试**服务，含 interpolator、CSV writer
- `internal/storage/` — JSON config persistence (`~/.yx-daq/`), CSV recording, PDF report (go-pdf/fpdf)
- `internal/scanner/` — UDP device scanner
- Frontend: 6 active views (Dashboard, Device, Motion, ThreeHoleTest, Settings, Data), Pinia stores, hash-based routing via `vue-router`. `CalibrationView` 路由存在但当前隐藏.

## Conventions

**必须遵守以下文档**：
- `docs/engineering/architecture.md` —— 架构与设计规范（目录结构、设计原则、接口设计、设计模式、反模式）
- `docs/engineering/coding-standards.md` —— 编码规范（Go + 前端，权威）

核心速查：
- Config/recordings stored in `~/.yx-daq/` (user home directory)
- `//go:embed all:frontend/dist` in `main.go` — frontend must be built before Go build
- SCSS: Vite auto-injects `@use "@/assets/styles/variables.scss" as *;` globally
- TS path alias: `@` → `/src` (configured in `vite.config.ts`)
- Vitest uses `happy-dom` environment (not jsdom)
- Wails v3 auto-generates `frontend/bindings/` (TypeScript, `@wailsio/runtime`) — **禁止手动编辑**
- 旧版 `frontend/wailsjs/` (Wails v2) 已废弃，通过 `frontend/src/wails-compat/` 兼容层映射到 v3 bindings
- All external strings in Chinese (UI labels, error messages, file dialogs)
- Axes: X=purple, Y=cyan, Z=green, U=orange (UI convention)
- Go 错误包装用 `%w`，日志用 `log.Printf`
- 前端 Store 中 Wails 绑定必须静态 import
- 前端样式必须 `<style lang="scss" scoped>`

## Directory & File Layout Rules

**必须遵守 `docs/engineering/architecture.md`**，核心摘要如下：

- `internal/` 按业务领域分包（types → driver → manager → calibration/three_hole/storage），`app.go` 为依赖注入汇聚点，禁止循环依赖
- 前端 `views/` 放页面组件，`components/` 放通用组件，`stores/` 放 Pinia 状态，组件不引用 `views/`
- Go 文件小写加下划线，Vue 组件 PascalCase，store 文件 camelCase
- 所有 `.md` 文档放入 `docs/`，根目录不新增
- 事件通道命名 `<domain>:<action>`
- Wails 绑定文件 `frontend/bindings/` 为 Wails v3 自动生成，**禁止手动编辑**
- `frontend/wailsjs/` 为 Wails v2 旧绑定，通过 `frontend/src/wails-compat/` 兼容层映射到 v3

<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **y-daq** (3494 symbols, 6905 relationships, 154 execution flows). Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

> If any GitNexus tool warns the index is stale, run `npx gitnexus analyze` in terminal first.

## Always Do

- **MUST run impact analysis before editing any symbol.** Before modifying a function, class, or method, run `gitnexus_impact({target: "symbolName", direction: "upstream"})` and report the blast radius (direct callers, affected processes, risk level) to the user.
- **MUST run `gitnexus_detect_changes()` before committing** to verify your changes only affect expected symbols and execution flows.
- **MUST warn the user** if impact analysis returns HIGH or CRITICAL risk before proceeding with edits.
- When exploring unfamiliar code, use `gitnexus_query({query: "concept"})` to find execution flows instead of grepping. It returns process-grouped results ranked by relevance.
- When you need full context on a specific symbol — callers, callees, which execution flows it participates in — use `gitnexus_context({name: "symbolName"})`.

## Never Do

- NEVER edit a function, class, or method without first running `gitnexus_impact` on it.
- NEVER ignore HIGH or CRITICAL risk warnings from impact analysis.
- NEVER rename symbols with find-and-replace — use `gitnexus_rename` which understands the call graph.
- NEVER commit changes without running `gitnexus_detect_changes()` to check affected scope.

## Resources

| Resource | Use for |
|----------|---------|
| `gitnexus://repo/y-daq/context` | Codebase overview, check index freshness |
| `gitnexus://repo/y-daq/clusters` | All functional areas |
| `gitnexus://repo/y-daq/processes` | All execution flows |
| `gitnexus://repo/y-daq/process/{name}` | Step-by-step execution trace |

## CLI

| Task | Read this skill file |
|------|---------------------|
| Understand architecture / "How does X work?" | `.claude/skills/gitnexus/gitnexus-exploring/SKILL.md` |
| Blast radius / "What breaks if I change X?" | `.claude/skills/gitnexus/gitnexus-impact-analysis/SKILL.md` |
| Trace bugs / "Why is X failing?" | `.claude/skills/gitnexus/gitnexus-debugging/SKILL.md` |
| Rename / extract / split / refactor | `.claude/skills/gitnexus/gitnexus-refactoring/SKILL.md` |
| Tools, resources, schema reference | `.claude/skills/gitnexus/gitnexus-guide/SKILL.md` |
| Index, status, clean, wiki CLI commands | `.claude/skills/gitnexus/gitnexus-cli/SKILL.md` |
| Work in the Three_hole area (156 symbols) | `.claude/skills/generated/three-hole/SKILL.md` |
| Work in the Go area (90 symbols) | `.claude/skills/generated/go/SKILL.md` |
| Work in the Stores area (82 symbols) | `.claude/skills/generated/stores/SKILL.md` |
| Work in the Manager area (74 symbols) | `.claude/skills/generated/manager/SKILL.md` |
| Work in the Views area (60 symbols) | `.claude/skills/generated/views/SKILL.md` |
| Work in the Driver area (48 symbols) | `.claude/skills/generated/driver/SKILL.md` |
| Work in the Calibration area (29 symbols) | `.claude/skills/generated/calibration/SKILL.md` |
| Work in the Storage area (18 symbols) | `.claude/skills/generated/storage/SKILL.md` |
| Work in the App area (16 symbols) | `.claude/skills/generated/app/SKILL.md` |
| Work in the Components area (14 symbols) | `.claude/skills/generated/components/SKILL.md` |
| Work in the Types area (7 symbols) | `.claude/skills/generated/types/SKILL.md` |
| Work in the Main area (7 symbols) | `.claude/skills/generated/main/SKILL.md` |
| Work in the Scanner area (4 symbols) | `.claude/skills/generated/scanner/SKILL.md` |
| Work in the Runtime area (4 symbols) | `.claude/skills/generated/runtime/SKILL.md` |
| Work in the Composables area (4 symbols) | `.claude/skills/generated/composables/SKILL.md` |

<!-- gitnexus:end -->
