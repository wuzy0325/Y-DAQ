# 五孔插值移位测试 —— 布点轴配置去重

> 本文件为 interview-me 产出的 confirmed intent，作为后续 spec / plan 阶段的输入。
> 确认时间：2026-07-08

## 背景

五孔测试配置「布点」卡片与每根探针配置都包含「物理轴选择」字段，功能重复且冲突。
历史原因：五孔复用三孔 `TraversalLayout`，layout 里带轴名字段，但五孔每根探针又有独立的 `MotionX/MotionY`，导致同一件事在两处配置。

## 目标

删除测试配置侧的所有物理轴选择字段，物理轴完全由每根探针自己的 `MotionX/MotionY` 决定。
测试配置只管「测哪些点」（坐标范围 / 步长），每根探针只管「用哪根轴去走」，两者职责分离。

## 具体改动

### 1. 删除字段（测试配置侧）

- 直线模式：删 `Line.Axis`（移动轴物理名选择）
- 矩形模式：删 `Rectangle.XAxis` / `Rectangle.YAxis`
- 扇面模式：删 `Fan.RAxis` / `Fan.ThetaAxis`

### 2. 保留字段（测试配置侧）

- 直线：起点 / 终点 / 步长（**删除「固定坐标」控件**，因为只用一根轴）
- 矩形：XMin / XMax / YMin / YMax / XSteps / YSteps
- 扇面：RStart / RSteps / ThetaStart / ThetaSteps

### 3. 探针配置侧 UI 标签动态化

底层仍存 `MotionX` / `MotionY` 两个字段，仅 UI 标签随布点模式变化：

| 布点模式 | MotionX 标签 | MotionY 标签 |
|---------|-------------|-------------|
| 直线     | 「移动轴」   | UI 隐藏（字段保留，切到矩形/扇面还要用） |
| 矩形     | 保持当前标签 | 保持当前标签 |
| 扇面     | 「R方向」    | 「θ方向」    |

### 4. 方向映射规则（motion_coordinator 改造）

- 矩形：`point.X → MotionX`，`point.Y → MotionY`（强制对应，不再反查轴名）
- 扇面：`R值 → MotionX`，`θ值 → MotionY`（强制对应）
- 直线：移动值喂 `MotionX`，`MotionY` 不参与
- 删除 `findProbeAxisMapping` / `findProbeAxisMappingWithDirection` 这套反查逻辑

### 5. 校验规则调整（Validate 改造）

- 扇面模式：强制每根启用探针的 `MotionX` 是线性轴、`MotionY` 是旋转轴（利用控制器 profile 的轴类型数组）
- 矩形模式：每根启用探针的 `MotionX` / `MotionY` 不能指向同一根物理轴
- 直线模式：每根启用探针的 `MotionX` 必须配置

## Out of Scope

- 不改 `FiveHoleProbeConfig` 的数据结构（`MotionX` / `MotionY` 字段名保留，只是 UI 标签变）
- 不改 CSV 导出格式（仍按 X/Y 方向导出 ControllerID / Axis）
- 不清理 `service.go` 里 α/β 的陈旧注释（独立小瑕疵，不在本次范围）
- 不改三孔模块（三孔的 `MotionAlpha` / `MotionBeta` 是另一套结构，不碰）

## 关键代码位置（供后续 spec/plan 参考）

- 测试配置类型：`src/internal/types/five_hole_traversal.go:72-89`
- Layout 类型（复用三孔）：`src/internal/types/three_hole_traversal.go:140-147`
- 探针配置类型：`src/internal/types/five_hole_traversal.go:60-68`
- 布点生成器：`src/internal/five_hole/point_generator.go:15-146`
- 运动协调器 buildMoveTasks：`src/internal/five_hole/motion_coordinator.go:106-193`
- findProbeAxisMapping（待删）：`src/internal/five_hole/motion_coordinator.go:196-221`
- Validate：`src/internal/types/five_hole_traversal.go:205-216, 348-410`
- 前端探针配置 UI：`src/frontend/src/views/FiveHoleTestView.vue:536-565`
- 前端布点轴选择 UI：`src/frontend/src/views/FiveHoleTestView.vue:235-345`
- 前端类型定义：`src/frontend/src/stores/fiveHoleTest/types.ts:81-165`
