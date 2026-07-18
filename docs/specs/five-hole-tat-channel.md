# Spec: 五孔测试通道配置新增总温（TAT）

> 来源：[docs/ideas/five-hole-tat-channel.md](../ideas/five-hole-tat-channel.md)
> 阶段：Phase 1 — Specify（事后补写，对应已实现代码）

## Objective

在五孔移位测试配置中新增一个物理语义正确的**总温（TAT / TTotal）通道**，使 SAT（静温）计算在有总温探头时使用真实总温源，在无总温探头时回退为用现有 TAtm（大气温度）字段参与计算——保持完全向后兼容。

**用户故事**：
- 作为风洞测试工程师，我希望接入独立的总温探头时，五孔测试用真实总温计算 SAT，提升高速工况下的马赫数/速度精度
- 作为现场调试人员，我希望未接入总温探头时五孔测试行为与升级前完全一致（无需修改旧配置）
- 作为数据复核人员，我希望 CSV 导出能体现是否使用了总温源，便于追溯

**成功条件（Reframed Success Criteria）**：
- 配置 `TTotalDeviceID` 时：`CalculateSAT` 使用 TTotal 值
- 未配置 `TTotalDeviceID` 时：`CalculateSAT` 回退用 TAtm（与升级前数值完全一致）
- 旧配置文件加载后 `TTotalDeviceID` 为空串（零值），自动走回退路径，无需显式迁移
- CSV 列新增 `TTotal(℃)`：有值显示数值，未配置显示 `-`
- UI 大气参数区新增「总温 T0 设备 / 通道」行，可清空
- `Validate()` 在 `TTotalDeviceID` 非空时校验 `TTotalChannel ≥ 0`，为空时跳过校验
- `collectInputWarnings` 仅在 TTotal 已配置时追加 TAT 范围校验（-50~200℃）

## Tech Stack

- 后端：Go 1.23（`yx-daq/internal/types`、`yx-daq/internal/five_hole`、`yx-daq/internal/five_hole/interpolation`）
- 前端：Vue 3 + TypeScript + Pinia + Element Plus
- 关键文件：
  - [src/internal/types/five_hole_traversal.go](../../src/internal/types/five_hole_traversal.go) — `FiveHoleTraversalConfig.TTotalDeviceID` / `TTotalChannel` 字段 + `Validate()` 校验
  - [src/internal/five_hole/data_processor.go](../../src/internal/five_hole/data_processor.go) — `ReadAllProbesRawData` 新增 TTotal 参数，读取并填充到 `FiveHoleRawData.TTotal`
  - [src/internal/five_hole/interpolation/types.go](../../src/internal/five_hole/interpolation/types.go) — `runtimeInput.TTotal *float64` 字段（nil 表示未配置）
  - [src/internal/five_hole/interpolation/cal_interpolator.go](../../src/internal/five_hole/interpolation/cal_interpolator.go) — `tatToKelvin`、`collectInputWarnings` TAT 校验
  - [src/internal/five_hole/interpolator.go](../../src/internal/five_hole/interpolator.go) — `toInterpolationResult` 按 TTotal/TAtm 规则选择输入
  - [src/internal/five_hole/service.go](../../src/internal/five_hole/service.go) — `samplePoint` / `emitRealtimeForAllProbes` / `aggregateProbeData` / `CollectDeviceIDs` 透传 TTotal
  - [src/internal/five_hole/csv_writer.go](../../src/internal/five_hole/csv_writer.go) — `TTotal(℃)` 列 + `formatTTotal`
  - [src/frontend/src/stores/fiveHoleTest/types.ts](../../src/frontend/src/stores/fiveHoleTest/types.ts) — `tTotalDeviceId` / `tTotalChannel` 字段
  - [src/frontend/src/views/FiveHoleTestView.vue](../../src/frontend/src/views/FiveHoleTestView.vue) — 大气参数区 T0 设备/通道 UI

## Commands

| What | How |
|------|------|
| 前端类型检查 | `cd src/frontend && npx vue-tsc --noEmit` |
| 前端构建 | `cd src/frontend && npm run build` |
| 前端单元测试 | `cd src/frontend && npm run test` |
| Go 编译检查 | `cd src && go build ./...` |
| Go 测试 | `cd src && go test ./internal/...` |

## Core Rules（回退策略）

| 场景 | `TTotalDeviceID` | `runtimeInput.TTotal` | `CalculateSAT` 输入 |
|------|------------------|----------------------|---------------------|
| 已配置且读取成功 | 非空 | `*float64(value)` | TTotal |
| 已配置但读取失败 | 非空 | `nil` | 回退用 TAtm |
| 未配置 | 空串 | `nil` | 回退用 TAtm（旧行为） |

**density 计算保持不变**——继续用 TAtm：
- 新配置下：TAtm 作为静温参与 density 计算（语义正确）
- 旧配置回退路径下：TAtm 实际被当 TAT 用，density 仍按旧行为计算（保持数值一致）

## Project Structure

```
src/internal/
  types/
    five_hole_traversal.go        ← 新增 TTotalDeviceID/Channel 字段 + Validate 校验
  five_hole/
    data_processor.go             ← ReadAllProbesRawData 新增 TTotal 参数与读取
    interpolator.go               ← toInterpolationResult 选择 TTotal/TAtm
    service.go                    ← samplePoint/emitRealtime/aggregateProbeData/CollectDeviceIDs 透传
    csv_writer.go                 ← TTotal(℃) 列 + formatTTotal
    interpolation/
      types.go                    ← runtimeInput.TTotal *float64
      cal_interpolator.go         ← tatToKelvin / collectInputWarnings TAT 校验
src/frontend/src/
  stores/fiveHoleTest/types.ts    ← tTotalDeviceId/tTotalChannel 字段
  views/FiveHoleTestView.vue      ← 大气参数区 T0 设备/通道 UI
docs/specs/
  five-hole-tat-channel.md        ← 本文件
```

## Testing Strategy

**Go 测试** — 覆盖以下场景：

| 测试场景 | 期望行为 |
|---------|---------|
| `Validate()` 未配置 TTotal → 通过 | `TTotalDeviceID == ""` 时不报错 |
| `Validate()` TTotal 已配置但 `Channel < 0` → 报错 | 返回「总温通道号必须≥0」 |
| `tatToKelvin` TTotal=nil → 用 AtmT | 与旧行为数值一致 |
| `tatToKelvin` TTotal=*v → 用 v | 返回 `v + 273.15` |
| `collectInputWarnings` TTotal 未配置 → 无 TAT 警告 | 跳过校验 |
| `collectInputWarnings` TTotal 超出 -50~200℃ → 警告 | 不中断流程，SAT 置零 |
| `collectInputWarnings` TTotal 在量程内 → 无警告 | 正常计算 |
| 回退路径数值一致性：未配置 TTotal 的插值结果 == 显式 TTotal=TAtm（同值）的插值结果 | 完全一致 |
| CSV writer：TTotal 有值 → 数值列；nil → `-` | 列位于 TAtm 之后 |
| `samplePoint` 已配置 TTotal 时 `FiveHoleRawData.TTotal != nil` | 透传成功 |

**前端测试**：
- `defaultConfig()` 含 `tTotalDeviceId: ''`、`tTotalChannel: 0`
- 配置持久化：保存含 TTotal 的配置 → 重新加载 → 字段保留

**手动验证**：
1. `wails3 dev` 启动，进入五孔测试界面
2. 大气参数区出现「总温 T0 设备 / 通道」行，可清空
3. 不选 T0 设备 → 启动测试 → CSV 中 TTotal 列为 `-`
4. 选择 T0 设备 + 通道 → 启动测试 → CSV 中 TTotal 列为数值
5. T0 通道填 -1 → 保存配置 → 报「总温通道号必须≥0」

## Boundaries

**Always do**:
- `TTotalDeviceID` 为空串即触发回退路径，无需显式迁移标志
- `runtimeInput.TTotal` 为 `*float64`，nil 表示未配置/读取失败
- CSV TTotal 列位于 TAtm 列之后
- 错误包装用 `%w`，日志用 `log/slog` 结构化

**Never do**:
- 重构 `AtmosphericConfig` 嵌套结构（保持扁平字段，便于迁移）
- 跨模块（三孔/校准）共享 TAT 配置
- 修改恢复系数 r
- 改 JSON 字段名（破坏所有旧配置文件）
- 改 density 公式（保持现状，density 语义问题留待 TAtm 可选化时一并处理）
- TAT/SAT 交叉一致性诊断（MVP 不做）
- 每探针 TAT 独立覆盖（全局共享）
- 原始电压→温度标定曲线（假设采集设备已输出工程单位 ℃）

## Success Criteria

- [x] `FiveHoleTraversalConfig` 含 `TTotalDeviceID` / `TTotalChannel` 字段
- [x] `Validate()` 在 TTotal 已配置时校验 `Channel ≥ 0`
- [x] `runtimeInput.TTotal` 为 `*float64`，nil 触发回退
- [x] `tatToKelvin` 按规则选择 TTotal 或 TAtm
- [x] `collectInputWarnings` 仅在 TTotal 已配置时追加 TAT 范围校验
- [x] CSV writer 输出 `TTotal(℃)` 列，未配置时为 `-`
- [x] 前端 types 含 `tTotalDeviceId` / `tTotalChannel`
- [x] 五孔测试界面大气参数区含 T0 设备/通道 UI
- [x] `go test ./internal/...` 全部通过
- [x] `vue-tsc --noEmit` / `npm run build` 通过
- [x] 旧配置加载后行为与升级前完全一致（回退路径）

## Out of Scope

依据 idea 文档「Not Doing」部分，以下项目刻意不做：

- **恢复系数 r 可配置** —— Phase 2
- **`AtmosphericConfig` 嵌套重构** —— 与「自动迁移 + 向后兼容」目标冲突
- **跨模块（三孔/校准）大气参数共享** —— 三孔/校准模块当前无 TAT 需求
- **TAT/SAT 交叉一致性诊断** —— 锦上添花，MVP 应聚焦「有 TAT 时能正确计算 SAT」
- **每探针 TAT 独立覆盖** —— 全局共享语义已足够
- **原始电压→温度标定曲线** —— 假设采集设备已输出工程单位
- **density 改用计算所得 SAT** —— 保留现状，待 TAtm 语义明确后再决策

## Resolved Decisions

- TAT 量程校验阈值：**-50~200℃**（覆盖风洞工况范围）
- UI 显示 TTotal 卡片：**显示**。未配置时显示 `-`
- CSV TTotal 列：有值显示数值，未配置时显示 `-`
- TTotal 字段 JSON 名：`tTotalDeviceId` / `tTotalChannel`（与现有 `tAtmDeviceId` 风格一致）
- 回退策略：`TTotal` nil → 用 `TAtm`（不区分「未配置」与「读取失败」，均走回退）
