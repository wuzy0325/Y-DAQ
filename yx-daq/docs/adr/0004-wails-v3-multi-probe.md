# ADR-0004: Wails v3 迁移以支持多探针独立窗口

**Status**: accepted

## 背景

现需支持两个三孔探针（探针1、探针2）同时进行移位插值测试。两个探针物理独立，各自连接采集设备和运动控制器（可共享也可独立），需各自独立的控制窗口。

Wails v2 仅支持单窗口，无法实现独立 OS 窗口。Wails v3 原生支持多窗口管理（`app.Window.NewWithOptions()`），是唯一能满足需求的技术路径。

## 决策

1. **框架升级**：Wails v2.12 → v3，涉及 `main.go` 重写、Service 模式迁移、事件系统和前端运行时 API 全面更新
2. **多窗口**：主窗口保持 Dashboard 不变；探针1和探针2各自为独立纯功能窗口，通过加载同一前端 bundle + `?probe=1|2` URL 参数区分
3. **后端架构**：`ThreeHoleService` 维护 `map[string]*TestRunner`，每个探针各自独立的 `TestManager`/`DataProcessor`/`Interpolator`/`CsvWriter`/`EventHandler`
4. **事件路由**：事件通道加探针前缀区分（`three-hole:probe1:progress`、`three-hole:probe2:progress`）
5. **迁移策略**：两阶段 —— 阶段1：纯 Wails v3 迁移（零功能变更，只重构现有探针1）；阶段2：新增探针2 功能和窗口

## 考虑的替代方案

- **应用内分屏**：Wails v2 内用拖动分隔栏模拟左右面板。弃用原因：面板无法独立为 OS 窗口（不能拖到不同显示器），且需要 v3 升级才能实现真正的独立窗口
- **标签页切换**：单窗口内切换标签。弃用原因：无法同时查看两个测试

## 影响

- 全项目重构：`App` 单体约 50 个方法需拆分为按业务领域的 Service；前端 `wailsjs/` 导入路径全面更新；`wails.json` 配置格式变更
- 事件系统从 `runtime.EventsEmit(ctx, ...)` 迁移至 `app.Event.Emit(...)`
- Service 模式消除 `ctx context.Context` 在 Go 方法签名中的传递
