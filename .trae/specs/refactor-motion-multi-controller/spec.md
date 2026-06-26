# 运动控制多控制器改造 Spec

## Why
当前运动控制画面（`MotionView.vue`）是围绕单一活动控制器设计的仪表盘，没有列表、添加、删除 UI。虽然后端 `MotionControllerManager` 已用 map 支持多控制器，但前端无法管理多个控制器，且后端 `RemoveProfile` 存在清理缺陷（不清理已连接实例与状态、无内置持久化、`Init` 硬编码默认 profile）。本次改造让运动控制画面达到与设备管理一致的多控制器 CRUD 体验。

## What Changes
- 重写 `MotionView.vue` 为「列表 + 添加/编辑弹窗 + 行内操作」形态，参照 `DeviceView.vue` 的 `el-table` + `el-dialog` 模式
- 新增控制器添加弹窗（名称/类型/地址/端口/超时），编辑弹窗复用现有 `AxisConfigDialog` 配置轴参数
- 列表行内提供：编辑 / 连接或断开 / 删除按钮，删除前 `ElMessageBox.confirm` 二次确认
- `motion.ts` store 接入已存在但未使用的 `addController`，新增 `removeController`、`updateControllerProfile` 方法
- **后端 `MotionControllerManager` 修复**：
  - `RemoveProfile` 增加断开实例、清理 `statuses`、持久化、发事件逻辑（对齐 `DeviceManager.RemoveProfile`）
  - 引入 `configStore`，AddProfile/UpdateProfile/RemoveProfile 自动持久化，移除 `Core.saveMotionConfig()` 手动调用
  - `Init` 改为从 `configStore` 加载 profile，无配置才创建默认模拟控制器
  - 引入 `onStatusChange` 回调 + `emitStatusChange`，由 `core.go` 桥接为 `motion:status-updated` 事件，替代 10Hz 轮询（保留轮询作为兼容降级）
- **BREAKING**：`MotionView.vue` 顶部单控制器状态栏布局被列表布局取代，`activeProfile` 的 B140 偏好硬编码逻辑移除
- 保留独立窗口接入方式（`OpenMotionWindow` + `/#/motion?window=1` 不变）
- 添加控制器类型注册表（对齐 `deviceTypeRegistry`），为未来类型扩展留口

## Impact
- 受影响 specs：无（首次 spec）
- 受影响代码：
  - 前端：`yx-daq/frontend/src/views/MotionView.vue`（重写）、`yx-daq/frontend/src/stores/motion.ts`（补 CRUD 方法）、`yx-daq/frontend/src/components/MotionControl/AxisControlCard.vue`（列表行内轴控制适配）、`yx-daq/frontend/src/components/MotionControl/AxisConfigDialog.vue`（可能复用）
  - 前端兼容层：`yx-daq/frontend/src/wails-compat/app.ts`（已就绪，无需改动）
  - 后端：`yx-daq/internal/manager/motion_manager.go`（RemoveProfile 修复 + configStore + onStatusChange + Init 重构 + 类型注册表）、`yx-daq/internal/app/service_motion.go`（移除手动 saveMotionConfig 调用、桥接 motion:status-updated 事件）、`yx-daq/internal/app/core.go`（保留/调整 broadcastMotionStatus 兼容降级）
  - 类型：`yx-daq/internal/types/motion.go`（可能新增类型注册表辅助）
  - 配置：`yx-daq/config/motion.json`（默认 seed，可能调整）

## ADDED Requirements

### Requirement: 运动控制器列表展示
系统 SHALL 在运动控制画面顶部以 `el-table` 形式展示所有已配置的运动控制器，每行包含：名称、类型、地址:端口、连接状态、轴数量、操作按钮组（编辑/连接或断开/删除）。

#### Scenario: 列表加载
- **WHEN** 用户打开运动控制窗口（`/#/motion?window=1`）
- **THEN** 自动调用 `fetchProfiles()` + `fetchStatuses()`，表格展示所有 profile
- **AND** 若列表为空，展示空状态提示与「添加控制器」按钮

#### Scenario: 状态实时更新
- **WHEN** 后端推送 `motion:status-updated` 事件
- **THEN** 表格中对应行的连接状态实时刷新，无需手动重载

### Requirement: 添加运动控制器
系统 SHALL 提供「添加控制器」按钮，点击后弹出 `el-dialog`，表单包含：名称（必填）、类型（下拉：B140-MC / SIMULATED-MC）、地址（IP，B140-MC 时必填）、端口（数字，B140-MC 时必填）、超时毫秒数（默认 3000）。

#### Scenario: 添加成功
- **WHEN** 用户填写完整有效信息并点击确认
- **THEN** 调用 `AddMotionProfile`，列表新增一行，配置自动持久化到 `motion.json`
- **AND** 弹窗关闭，提示添加成功

#### Scenario: 校验失败
- **WHEN** 用户未填名称或 B140-MC 类型未填地址/端口
- **THEN** 表单校验失败，禁用确认按钮并显示错误提示

### Requirement: 编辑运动控制器
系统 SHALL 在列表行内提供编辑按钮，点击后弹出编辑 `el-dialog`，包含基础信息编辑区（名称/地址/端口/超时）与轴配置区（复用 `AxisConfigDialog` 的轴表格逻辑：每行可编辑 name/enabled/kind/inverted/stepAngleDeg/lead/gearRatio/maxSpeed/encoderScale/microSteps）。

#### Scenario: 编辑已连接控制器
- **WHEN** 用户编辑已连接控制器的轴参数并保存
- **THEN** 调用 `UpdateMotionProfile`，驱动实例同步新轴配置（对齐 `DeviceManager.UpdateProfile` 行为）
- **AND** 配置自动持久化

### Requirement: 删除运动控制器（带二次确认）
系统 SHALL 在列表行内提供删除按钮（danger 样式）。点击后弹出 `ElMessageBox.confirm` 二次确认对话框，提示控制器名称与连接状态。

#### Scenario: 删除已连接控制器
- **WHEN** 用户确认删除一个已连接的控制器
- **THEN** 后端 `RemoveProfile` 先断开实例连接，再清理 profiles/statuses，持久化，发事件
- **AND** 列表移除该行，提示删除成功

#### Scenario: 删除取消
- **WHEN** 用户在确认对话框点击取消
- **THEN** 不执行任何删除操作

### Requirement: 后端 RemoveProfile 完整清理
`MotionControllerManager.RemoveProfile` SHALL 执行完整清理：若该控制器已连接则先 Disconnect、删除 instances map 项、删除 statuses map 项、删除 profiles map 项、调用 `saveProfiles` 持久化、调用 `emitStatusChange` 通知。

#### Scenario: 删除已连接控制器后无残留
- **WHEN** 删除一个已连接的 B140 控制器
- **THEN** instances / statuses / profiles 三个 map 中均无该 id
- **AND** `motion.json` 中该 profile 已移除

### Requirement: 后端内置持久化
`MotionControllerManager` SHALL 持有 `configStore` 字段，AddProfile/UpdateProfile/RemoveProfile 自动调用 `saveProfiles`。`Core.saveMotionConfig()` 的手动调用 SHALL 被移除。

#### Scenario: 添加 profile 后自动持久化
- **WHEN** 调用 `AddMotionProfile`
- **THEN** `motion.json` 自动更新，无需 service 层手动触发保存

### Requirement: 后端从配置加载 Init
`MotionControllerManager.Init` SHALL 优先从 `configStore` 加载 profiles；无配置时创建一个默认 SIMULATED-MC profile。

#### Scenario: 首次启动无配置
- **WHEN** `~/.yx-daq/motion.json` 不存在
- **THEN** 创建一个默认 `sim-mc-default` profile 并持久化

### Requirement: 后端事件驱动状态广播
`MotionControllerManager` SHALL 通过 `onStatusChange` 回调 + `emitStatusChange` 推送状态变更，`core.go` 桥接为 Wails 事件 `motion:status-updated`。10Hz `broadcastMotionStatus` 轮询 SHALL 作为兼容降级保留（事件未触发的兜底）。

#### Scenario: 连接成功后状态推送
- **WHEN** 控制器连接成功
- **THEN** 后端主动推送 `motion:status-updated` 事件，前端无需等待下一次轮询

## MODIFIED Requirements

### Requirement: 运动控制器类型扩展
`MotionControllerManager` SHALL 引入类型注册表（对齐 `deviceTypeRegistry` + `driverFactories`），新增控制器类型只需注册一行，不再使用 switch-case 硬编码。

## REMOVED Requirements

### Requirement: MotionView 单控制器仪表盘布局
**Reason**: 替换为多控制器列表 + 弹窗布局，原顶部单控制器状态栏、`activeProfile` 的 B140 偏好硬编码逻辑不再适用。
**Migration**: `activeControllerId` 单值在 store 中保留用于「当前选中」概念（如轴控制卡片的默认目标），但不再决定列表展示；列表展示所有 profiles。
