# YX-DAQ —— Agent instructions

Windows-only Wails v2 desktop app (Go 1.23 + Vue 3 + TypeScript + Vite 3 + Element Plus + ECharts 6).
多采集设备数据采集、显示、保存，设备管理与配置；连接运动控制器结合采集设备进行五孔探针、三孔探针等移位布点插值测试及数据导出。

## Commands

| What | How |
|------|------|
| Dev mode (hot reload) | `wails dev` |
| Build exe | `build.bat` (runs `go build ./...` then `wails build`) |
| Build + installer | `build.bat nsis` |
| Clean artifacts | `build.bat clean` |
| Go compile check | `go build ./...` |
| Frontend typecheck + build | `cd frontend && npm run build` (vue-tsc --noEmit then vite build) |
| Frontend tests (happy-dom) | `cd frontend && npm run test` (vitest) |
| Frontend tests (watch) | `cd frontend && npm run test:watch` |
| Go tests | `go test ./internal/...` |

Test files (pattern: `src/**/*.{test,spec}.{js,ts}`):
- Go: `internal/calibration/formulas_test.go`, `internal/storage/config_store_test.go`
- Frontend: `frontend/src/components/__tests__/GlassCard.test.ts`, `StatusIndicator.test.ts`, `ValueDisplay.test.ts`

## Architecture

- `main.go` — entrypoint, creates `App`, embeds `frontend/dist` via `//go:embed`
- `app.go` — Wails bindings (~50 methods), event publishing (`daq:data-snapshot`, `motion:status-updated`, `calibration:*`, `three-hole:*`)
- `internal/types/` — shared types and constants (包括五孔和三孔探针类型)
- `internal/driver/` — hardware drivers: `xy_daq16.go` (TCP), `b140.go` (motion TCP), `simulated_device.go`, `simulated_motion.go`
- `internal/manager/` — `DeviceManager`, `MotionControllerManager` (10 Hz poll), `AcquisitionHub` (20 Hz publish)
- `internal/calibration/` — **五孔移位插值测试**服务（当前隐藏），含 formulas、encoder compensation 状态机、sphere tank gate
- `internal/three_hole/` — **三孔移位插值测试**服务，含 interpolator、CSV writer
- `internal/storage/` — JSON config persistence (`~/.yx-daq/`), CSV recording, PDF report (go-pdf/fpdf)
- `internal/scanner/` — UDP device scanner
- Frontend: 6 active views (Dashboard, Device, Motion, ThreeHoleTest, Settings, Data), Pinia stores, hash-based routing via `vue-router`. `CalibrationView` 路由存在但当前隐藏.

## Conventions

- Config/recordings stored in `~/.yx-daq/` (user home directory)
- `//go:embed all:frontend/dist` in `main.go` — frontend must be built before Go build
- SCSS: Vite auto-injects `@use "@/assets/styles/variables.scss" as *;` globally
- TS path alias: `@` → `/src` (configured in `vite.config.ts`)
- Vitest uses `happy-dom` environment (not jsdom)
- Wails auto-generates `frontend/wailsjs/` bindings (do not edit manually)
- All external strings in Chinese (UI labels, error messages, file dialogs)
- Axes: X=purple, Y=cyan, Z=green, U=orange (UI convention)
- PDF export, CSV export, CSV replay features exist

## Directory & File Layout Rules

**必须遵守 `docs/project-layout-rules.md`**，核心摘要如下：

- `internal/` 按业务领域分包（types → driver → manager → calibration/three_hole/storage），`app.go` 为依赖注入汇聚点，禁止循环依赖
- 前端 `views/` 放页面组件，`components/` 放通用组件，`stores/` 放 Pinia 状态，组件不引用 `views/`
- Go 文件小写加下划线，Vue 组件 PascalCase，store 文件 camelCase
- 所有 `.md` 文档放入 `docs/`，根目录不新增
- 事件通道命名 `<domain>:<action>`
- Wails 绑定文件 `frontend/wailsjs/` 为自动生成，**禁止手动编辑**
