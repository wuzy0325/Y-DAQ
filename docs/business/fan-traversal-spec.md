# Spec: 五孔移位测试扇形布点模式

## 1. Objective

为五孔移位测试模块新增一种"扇形"（fan）布点模式，使用户能够按极坐标生成扫描点位：

- 半径方向（R）由线性轴驱动；
- 角度方向（θ）由旋转轴驱动；
- 第一个目标点位即当前位置（相对偏移 0,0），后续点位相对于第一点递增；
- 预览图仍以笛卡尔坐标 X/Y 绘制扇形点位；
- CSV 导出保持现有列格式，不额外增加 R/θ 列。

**用户故事**

- 作为五孔探针实验人员，我需要在以某参考点为圆心的扇面上采集压力分布，从而获取不同攻角/侧滑角方向下的气动参数。
- 作为五孔测试配置人员，我希望在选择角度轴时只显示已标记为"旋转轴"的轴，避免误选线性轴。

**成功标准**

- 五孔测试配置 UI 支持选择"扇形"布点模式。
- 用户可配置 R 方向步进（StepSegment）和 θ 方向步进（StepSegment）。
- 用户在配置 θ 方向轴映射时，UI 列出当前运动控制器 profile 中 `Kind == ROTARY` 的轴供选择（仅作提示，最终仍由用户确认）。
- 后端 `five_hole/point_generator.go` 新增 `generateFanPoints`，将极坐标 (R, θ) 转换为笛卡尔坐标 (x, y)。
- 第一点位为相对原点（0, 0），即从当前位置直接采样；后续点位为 `x = (R_i - R_start)·cos(θ_j - θ_start)`、`y = (R_i - R_start)·sin(θ_j - θ_start)`（以数学极坐标为准：0° 沿 +X，逆时针为正）。
- 预览 Canvas 正确显示扇形点位，颜色按整体完成状态着色。
- 配置验证（`FiveHoleTraversalConfig.Validate`）覆盖扇形模式：R/θ 步进非空、R/θ 轴非空且互不相同、半径轴和角度轴必须被启用探针使用。
- 单测覆盖扇形生成器至少 3 种典型配置（单半径单角度、多半径单角度、多半径多角度）。
- `go build ./...`、`go test ./internal/five_hole/...`、`cd src/frontend && npm run build` 全部通过。

## 2. Tech Stack

- 后端：Go 1.23 + Wails v3
- 前端：Vue 3 + TypeScript + Vite 3 + Element Plus
- 测试：Go `testing` / `testify`（后端）、Vitest + happy-dom（前端）
- 状态管理：Pinia

## 3. Commands

```bat
:: Go 编译检查（在 src/ 目录执行）
cd src && go build ./...

:: Go 单元测试
cd src && go test ./internal/five_hole/... ./internal/types/...

:: 前端类型检查与构建
cd src/frontend && npm run build

:: 前端测试
cd src/frontend && npm run test

:: 开发模式热重载
cd src && wails3 dev

:: 完整构建
cd src && wails3 task build
```

## 4. Project Structure

```
src/
├── internal/
│   ├── types/
│   │   ├── motion.go                      # AxisKind 已存在（LINEAR/ROTARY）
│   │   ├── three_hole_traversal.go        # LineLayout / RectangleLayout / TraversalLayout
│   │   └── five_hole_traversal.go         # 五孔验证逻辑（新增 FanLayout 校验分支）
│   └── five_hole/
│       ├── point_generator.go             # 新增 generateFanPoints
│       └── point_generator_test.go        # 新增扇形单测
├── frontend/src/
│   ├── views/
│   │   ├── MotionView.vue                 # 运动控制器配置 UI（轴类型显示/编辑）
│   │   └── FiveHoleTestView.vue           # 五孔测试配置 UI（扇形参数 + 轴映射筛选）
│   ├── composables/
│   │   └── usePointPreviewCanvas.ts       # 布点预览 Canvas（新增扇形极坐标→笛卡尔转换）
│   └── stores/
│       ├── fiveHoleTest.ts                # 五孔测试状态管理（默认配置、事件监听、CSV 导出）
│       └── fiveHoleTest/types.ts          # 五孔测试 TypeScript 类型定义
└── ...
```

## 5. Code Style

**Go 示例：扇形生成器**

```go
// generateFanPoints 扇形布点：R 方向线性，θ 方向旋转
// 第一点位为相对原点 (0,0)，后续点位按 (R_i - R_start, θ_j - θ_start) 偏移
// 角度使用数学极坐标：0° 沿 +X，逆时针为正
func generateFanPoints(fan *types.FanLayout) []types.TraversalPoint {
    if fan == nil {
        return nil
    }
    rValues := expandStepSegments(fan.RSteps)
    thetaValues := expandStepSegments(fan.ThetaSteps)
    if len(rValues) == 0 || len(thetaValues) == 0 {
        return nil
    }
    points := make([]types.TraversalPoint, 0, len(rValues)*len(thetaValues))
    id := 0
    for _, r := range rValues {
        for _, thetaDeg := range thetaValues {
            theta := (thetaDeg - fan.ThetaStart) * math.Pi / 180
            dr := r - fan.RStart
            points = append(points, types.TraversalPoint{
                ID: fmt.Sprintf("pt-%d", id),
                X:  dr * math.Cos(theta),
                Y:  dr * math.Sin(theta),
            })
            id++
        }
    }
    return points
}
```

**关键约定**

- Go 错误包装使用 `%w`，日志使用 `log/slog` 结构化输出。
- 所有新增字段 JSON tag 使用 camelCase。
- 前端样式统一使用 `<style lang="scss" scoped>`。
- 前端 UI 文本使用中文。

## 6. Testing Strategy

| 层级 | 范围 | 方式 |
|------|------|------|
| 单元测试 | `internal/five_hole/point_generator_test.go` | 验证扇形点位数量、坐标值、相对原点行为 |
| 单元测试 | `internal/types/validate_test.go` | 验证扇形配置校验通过/失败场景 |
| 单元测试 | `src/frontend/src/api/__tests__/enums.test.ts` | 验证 TraversalPattern 新增 `fan` 模式及标签 |
| 单元测试 | `src/frontend/src/composables/__tests__/expandSteps.test.ts` | 验证步长展开与扇形预览一致性 |
| 类型检查 | 前端构建 | `vue-tsc --noEmit` 验证新增类型 |
| 手工验证 | 开发模式 | 配置一个扇形测试，观察预览图和运动行为 |

## 7. Boundaries

- **Always:**
  - 在修改 schema 前先补充单测；
  - 所有外部错误信息使用中文；
  - 组件卸载时清理新增的定时器/监听器；
  - 遵循项目现有目录结构和命名规范。

- **Ask first:**
  - 新增第三方依赖；
  - 改动三孔模块的布点模式；
  - 修改 CSV 列格式；
  - 改变 TraversalPoint schema（本 spec 明确不改）。

- **Never:**
  - 在扇形实现中硬编码某固定轴为旋转轴（必须通过 `AxisKind` 筛选）；
  - 修改现有 line/rectangle/custom 三种模式的语义；
  - 在扇形模式下扩展 TraversalPoint 字段；
  - 提交未通过 `go test` 和 `npm run build` 的代码。

## 8. Open Questions

1. 运动控制器 profile 中 AxisKind 默认是否已正确配置？现有 `DefaultAxisConfigs()` 已将 U 轴设为 `ROTARY`，X/Y/Z 为 `LINEAR`。如果用户已有旧配置文件，是否需要迁移逻辑将未设置 Kind 的轴默认为 `LINEAR`？
   - **已解决**：UI 端通过 `availableRotaryAxisOptions` 过滤 `Kind == ROTARY` 的轴，未配置 Kind 的轴不会出现在 θ 方向候选列表中；后端校验不强制检查 Kind。
2. 扇形预览 Canvas 是否需要在图上标注圆心/角度参考线，还是仅显示离散点位？本 spec 按"仅显示离散点位"实现，如需参考线可在后续迭代补充。
   - **已解决**：按"仅显示离散点位"实现，与矩形/直线模式保持一致。
3. 角度轴若选择 U 轴，其位置单位在现有运动控制协议中是否按"度"解释？本 spec 假设控制器接受的角度指令单位为度，无需额外换算。
   - **已解决**：实现中直接将角度值（度）作为目标位置下发，不做额外换算。
