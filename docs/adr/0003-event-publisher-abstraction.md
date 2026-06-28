# ADR-0003: 事件发布器通过接口注入而非直接依赖 Wails Runtime

业务服务（calibration、three_hole）不直接调用 `wailsRuntime.EventsEmit`，而是通过注入的 `EventPublisher` 接口发布事件。

接口在业务服务侧定义（使用方定义），实现在 `internal/app/event_publishers.go` 中持有 `*App.ctx` 调用 Wails Runtime。

这样做是为了：
1. 业务服务可单元测试 — 注入 mock publisher 即可验证事件发布
2. 避免 `package main` 的循环导入 — 业务包不依赖 Wails 包
3. 接口小而聚焦（1-2 个方法），符合接口隔离原则

_Status: accepted_ | _Considered Options_: 直接调用 wailsRuntime（否决：无法测试）、通过回调函数（否决：多个事件类型时参数膨胀）
