# ADR-0001: 应用层分层 — internal/app/ 作为 DI 汇聚点

Wails v2 应用将所有 handler 方法绑定在 `*App` 上。我们将 App 定义、依赖注入、Wails 生命周期和所有 handler 方法集中放在 `internal/app/` 包，而非 `package main`。

选择此结构是因为 Wails 的绑定机制要求所有对外暴露的方法在同一接收器类型上。将 handlers 按业务领域拆分到独立文件（`handlers_device.go`、`handlers_motion.go` 等），既能满足 Wails 的约束，又能避免单个文件膨胀到不可维护。
