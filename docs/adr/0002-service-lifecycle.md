# ADR-0002: 采集/测试服务统一实现 Start → Pause → Resume → Stop 生命周期

所有长时间运行的服务（三孔移位插值测试、五孔探针校准）共享同一个生命周期模式。

选用此模式是因为：
1. 前端 UI 需要统一的启动/暂停/恢复/停止控制
2. 所有服务都涉及 goroutine 管理，统一生命周期降低心智负担
3. 状态转换明确（不允许从 Paused 直接 Start 等），用 `atomic.Bool` + channel 组合实现

替代方案（直接 goroutine + context.WithCancel）被否决，因为暂停/恢复需要双向信号通信，单纯 context 无法优雅处理。
