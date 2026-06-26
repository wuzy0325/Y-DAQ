# Checklist

## 后端
- [x] `MotionControllerManager` 持有 `configStore` 字段，AddProfile/UpdateProfile/RemoveProfile 自动调用 `saveProfiles`
- [x] `Core.saveMotionConfig()` 及其在 `service_motion.go` 中的 3 处手动调用已移除
- [x] `MotionControllerManager.Init` 从 `configStore` 加载 profiles，无配置时创建单个 `sim-mc-default`
- [x] `RemoveProfile` 执行完整清理：断开实例 → 删 instances → 删 statuses → 删 profiles → saveProfiles → emitStatusChange
- [x] `MotionControllerManager` 持有 `onStatusChange` 回调，`emitStatusChange(id)` 在 Connect/Disconnect/错误/运动完成等节点被调用
- [x] `core.go` 将 `onStatusChange` 桥接为 Wails 事件 `motion:status-updated`（对齐 `device:status-updated`）
- [x] `broadcastMotionStatus` 10Hz 轮询作为兼容降级保留
- [x] 控制器类型注册表 `controllerFactories` 已建立，`Connect`/`ensureInstance` 不再使用 switch-case 硬编码
- [x] `go build ./...` 通过

## 前端 Store
- [x] `motion.ts` 的 `addController` 已接入并刷新 profiles/statuses
- [x] `motion.ts` 新增 `removeController(id)` 方法
- [x] `motion.ts` 新增 `updateControllerProfile(profile)` 方法
- [x] `motion.ts` 新增 `connectingIds: ref<Set<string>>`，连接动作加入/移出
- [x] `startListening` 订阅 `motion:status-updated` 事件并更新对应 status

## 前端 UI
- [x] `MotionView.vue` 顶部工具栏含「添加控制器」与「急停全部」按钮
- [x] `MotionView.vue` 使用 `<el-table>` 展示所有 profiles，列含名称/类型/地址:端口/状态/轴数量/操作组
- [x] 操作按钮组含编辑 / 连接或断开（loading 由 connectingIds 控制）/ 删除（danger）
- [x] 空表格展示空状态提示与「添加控制器」按钮
- [x] 原 `activeProfile` 的 B140 偏好硬编码逻辑已移除
- [x] `onMounted` 不再自动选中 B140，仅 fetch + startListening
- [x] 添加弹窗表单字段完整（name/type/address/port/timeoutMs），含校验规则
- [x] 编辑弹窗含基础信息区 + 轴配置区（复用 AxisConfigDialog 逻辑）
- [x] 删除按钮触发 `ElMessageBox.confirm` 二次确认，提示名称与连接状态
- [x] 保留独立窗口接入方式（`OpenMotionWindow` + `/#/motion?window=1` 未改动）
- [x] `npm run build` 通过

## 端到端（代码路径已核验，建议运行时手动复测）
- [x] 添加 → 列表新增 → `motion.json` 自动更新（AddProfile→saveProfiles 持久化路径已核验）
- [x] 编辑已连接控制器 → 驱动实例同步新轴配置（UpdateProfile→axisConfigUpdater.UpdateAxes 路径已核验）
- [x] 删除已连接控制器 → 列表移除 → 后端 instances/statuses/profiles 无残留（RemoveProfile 完整清理已核验）
- [x] 删除取消 → 不执行任何操作（ElMessageBox.confirm 取消分支已核验）
- [x] 连接成功后 < 1s 收到 `motion:status-updated` 事件，列表状态实时刷新（emitStatusChange 即时推送 + 10Hz 兜底已核验）
- [x] 重启应用后配置与连接状态正确恢复（Init 从 configStore 加载 + 自动连接已核验）
