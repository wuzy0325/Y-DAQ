# Tasks

## 后端改造（优先，前端依赖 API 行为）

- [x] Task 1: 为 `MotionControllerManager` 引入 `configStore` 与 `saveProfiles` 自动持久化
  - [ ] SubTask 1.1: 在 `motion_manager.go` 的 `MotionControllerManager` 结构体增加 `configStore *storage.JSONConfigStore` 字段
  - [ ] SubTask 1.2: 实现 `saveProfiles()` 私有方法（对齐 `DeviceManager.saveProfiles`，序列化 profiles map 到 `motion.json`）
  - [ ] SubTask 1.3: 在 `AddProfile`/`UpdateProfile`/`RemoveProfile` 末尾调用 `saveProfiles()`
  - [ ] SubTask 1.4: 在 `core.go` 装配处传入 `configStore` 给 `MotionControllerManager`，移除 `Core.saveMotionConfig()` 及其在 `service_motion.go` 中的 3 处手动调用
  - Verify: `go build ./...` 通过；手动添加一个 profile 后检查 `~/.yx-daq/motion.json` 自动更新

- [x] Task 2: 重构 `MotionControllerManager.Init` 从配置加载
  - [ ] SubTask 2.1: 删除 `Init` 中硬编码的 `b140-mc-1` / `sim-mc-1` 默认 profile 创建逻辑
  - [ ] SubTask 2.2: 改为优先从 `configStore` 加载 profiles；若配置为空则创建单个默认 `sim-mc-default` profile 并持久化
  - Verify: 删除 `~/.yx-daq/motion.json` 后重启应用，应自动创建 `sim-mc-default`

- [x] Task 3: 修复 `RemoveProfile` 完整清理逻辑
  - [ ] SubTask 3.1: 在 `RemoveProfile` 中判断该 id 是否存在于 `instances` map，若存在则先调用对应控制器的 Disconnect/Close
  - [ ] SubTask 3.2: 删除 `instances[id]`、`statuses[id]`、`profiles[id]`
  - [ ] SubTask 3.3: 调用 `saveProfiles()` 持久化
  - [ ] SubTask 3.4: 调用 `emitStatusChange(id)` 通知前端（事件机制在 Task 4 后可用；本任务先留调用点，事件未就绪时降级为 no-op）
  - Verify: 连接一个 B140 控制器后调用 `RemoveMotionProfile`，检查 instances/statuses/profiles 三个 map 均无残留，`motion.json` 已更新

- [x] Task 4: 引入 `onStatusChange` 回调 + `emitStatusChange` 事件驱动广播
  - [ ] SubTask 4.1: 在 `MotionControllerManager` 增加 `onStatusChange func(string, MotionControllerStatus)` 字段与 setter
  - [ ] SubTask 4.2: 实现 `emitStatusChange(id)` 方法：从 statuses 取最新状态调用回调
  - [ ] SubTask 4.3: 在 Connect/Disconnect 成功/失败、运动指令完成、错误发生等关键节点调用 `emitStatusChange`
  - [ ] SubTask 4.4: 在 `core.go` 装配处注入回调，桥接为 Wails 事件 `motion:status-updated`（对齐 `device:status-updated` 桥接方式）
  - [ ] SubTask 4.5: 保留 `broadcastMotionStatus` 10Hz 轮询作为兼容降级（不删除）
  - Verify: 连接控制器成功后前端能在 < 1s 内收到 `motion:status-updated` 事件

- [x] Task 5: 引入控制器类型注册表
  - [ ] SubTask 5.1: 在 `motion_manager.go` 顶部定义 `controllerFactories map[MotionControllerType]func(...) MotionController` 注册表
  - [ ] SubTask 5.2: 在 `init()` 或包初始化处注册 `B140-MC` 和 `SIMULATED-MC` 两个工厂
  - [ ] SubTask 5.3: 将 `Connect`/`ensureInstance` 中的 switch-case 硬编码改为查注册表
  - Verify: `go build ./...` 通过；连接 B140 与 SIMULATED 行为与改造前一致

## 前端 Store 改造

- [x] Task 6: 在 `motion.ts` store 补齐 CRUD 方法并接入事件
  - [ ] SubTask 6.1: 确认 `addController` 方法（已存在）调用 `AddMotionProfile` 后刷新 profiles/statuses，并加入 `connectingIds` Set 模式（对齐 `device.ts`）
  - [ ] SubTask 6.2: 新增 `removeController(id)` 方法：调用 `RemoveMotionProfile`，刷新 profiles/statuses
  - [ ] SubTask 6.3: 新增 `updateControllerProfile(profile)` 方法：调用 `UpdateMotionProfile`，刷新 profiles（已连接则同步刷新 statuses）
  - [ ] SubTask 6.4: 在 `startListening` 中订阅 `motion:status-updated` 事件，更新对应 status
  - [ ] SubTask 6.5: 新增 `connectingIds: ref<Set<string>>` 用于按钮 loading 状态
  - Verify: `npm run build` 通过；单元调用 store 方法不报错

## 前端 UI 改造

- [x] Task 7: 重写 `MotionView.vue` 为列表布局
  - [ ] SubTask 7.1: 顶部工具栏：标题 + 「添加控制器」按钮 + 急停全部按钮
  - [ ] SubTask 7.2: `<el-table :data="motionStore.profiles">` 列：名称 / 类型 / 地址:端口 / 连接状态（el-tag 着色）/ 轴数量 / 操作按钮组
  - [ ] SubTask 7.3: 操作按钮组：编辑（Edit）/ 连接或断开（Link/CircleClose，loading 态由 connectingIds 控制）/ 删除（Delete，danger）
  - [ ] SubTask 7.4: 空状态：表格为空时展示「暂无控制器，点击添加」与添加按钮
  - [ ] SubTask 7.5: 移除原 `activeProfile` 计算属性的 B140 偏好逻辑，列表直接渲染 `motionStore.profiles`
  - [ ] SubTask 7.6: `onMounted` 改为仅调用 `fetchProfiles` + `fetchStatuses` + `startListening`，移除自动选中 B140 逻辑
  - Verify: 打开运动控制窗口看到控制器列表，状态实时刷新

- [x] Task 8: 实现添加控制器弹窗
  - [ ] SubTask 8.1: `<el-dialog v-model="showAddDialog">` + `el-form` 表单：name（必填）/ type（下拉 B140-MC、SIMULATED-MC）/ address（B140 时必填）/ port（B140 时必填，默认 5000）/ timeoutMs（默认 3000）
  - [ ] SubTask 8.2: 表单校验规则（对齐 DeviceView 添加弹窗的校验风格）
  - [ ] SubTask 8.3: 确认按钮调用 `motionStore.addController(profile)`，成功后关闭弹窗 + `ElMessage.success`
  - Verify: 添加成功后列表新增一行，`motion.json` 自动更新

- [x] Task 9: 实现编辑控制器弹窗
  - [ ] SubTask 9.1: `<el-dialog v-model="showEditDialog">`，基础信息区（name/address/port/timeoutMs 可编辑）
  - [ ] SubTask 9.2: 轴配置区：复用 `AxisConfigDialog.vue` 的轴表格逻辑（每行可编辑 name/enabled/kind/inverted/stepAngleDeg/lead/gearRatio/maxSpeed/encoderScale/microSteps）
  - [ ] SubTask 9.3: 保存按钮调用 `motionStore.updateControllerProfile(updatedProfile)`，成功后 `ElMessage.success`
  - Verify: 编辑已连接控制器的轴参数后保存，后端驱动实例同步新配置

- [x] Task 10: 实现删除二次确认
  - [ ] SubTask 10.1: 删除按钮点击调用 `ElMessageBox.confirm`，提示控制器名称与连接状态
  - [ ] SubTask 10.2: 确认后调用 `motionStore.removeController(id)`，`ElMessage.success`
  - [ ] SubTask 10.3: 取消则不执行
  - Verify: 删除已连接控制器后列表移除该行且后端无残留

## 验证

- [x] Task 11: 端到端验证
  - [ ] SubTask 11.1: 添加 / 编辑 / 删除 / 连接 / 断开 全流程跑通
  - [ ] SubTask 11.2: 重启应用后配置与连接状态正确恢复
  - [ ] SubTask 11.3: 删除已连接控制器无孤儿实例/状态残留
  - [ ] SubTask 11.4: 状态变更通过事件实时推送（< 1s）
  - Verify: `npm run build` + `go build ./...` 通过；上述场景手动验证通过

# Task Dependencies
- Task 3 依赖 Task 4（emitStatusChange 调用点）—— Task 4 可先完成
- Task 6 依赖 Task 4（前端订阅 motion:status-updated 事件）
- Task 7-10 依赖 Task 6（store 方法）
- Task 11 依赖所有前置任务
- Task 1、2、5 可并行（后端独立改动）
- Task 8、9、10 在 Task 7 完成后可并行（前端弹窗）
