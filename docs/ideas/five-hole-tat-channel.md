# 五孔测试通道配置新增总温 (TAT)

## Problem Statement
How might we 在五孔测试配置中新增一个物理语义正确的总温 (TAT) 通道，
使 SAT 计算在有总温探头时使用真实总温源，在无总温探头时回退为用现有
TAtm（大气温度）字段参与计算——保持完全向后兼容。

## Recommended Direction
**方向 A+4：最小增量 + 有效性校验 + 配置回退**

在 `FiveHoleTraversalConfig` 新增全局共享的 `TTotalDeviceID`/`TTotalChannel` 字段，
UI 完全镜像现有 TAtm 行（设备下拉 + 通道索引）。

**核心规则（回退策略）：**
- 配置了 TTotal → `CalculateSAT` 用 TTotal
- 未配置 TTotal → `CalculateSAT` 用 TAtm（保旧行为，完全向后兼容）

`collectInputWarnings` 中追加 TAT 范围校验（有限值、合理量程），
失败时 warning 不中断流程，SAT 置零。

**density 计算保持不变**——继续用 TAtm。理由：
- 新配置下：TAtm 作为静温参与 density 计算（语义正确）
- 旧配置回退路径下：TAtm 实际被当 TAT 用，density 仍按旧行为计算（保持数值一致）
  注：旧配置下 density 语义略有偏差（小马赫数下可接受），新配置下 density 语义正确。

不重构 `AtmosphericConfig`，不跨模块共享，不动恢复系数 r，不改 JSON 字段名。
新字段在旧配置加载时默认为空，自动走回退路径，保证旧配置升级后输出与升级前一致。

## Key Assumptions to Validate
- [ ] 回退策略可接受——验证：找 1 个现有配置文件加载测试，未配置 TTotal 时
      SAT/density 输出与升级前数值完全一致。
- [ ] 独立总温探头接入采集设备后，通道返回的是温度值（℃）而非原始电压——验证：
      现场探头+采集设备组合是否输出工程单位。若是电压，需 Phase 2 增加标定曲线。
- [ ] TAT 范围校验阈值（-50~200℃）覆盖现场工况——验证：与用户确认风洞工况范围，
      避免误报。
- [ ] TAtm 在新配置下作为静温参与 density 计算的语义可接受——若用户现场实际只有
      总温探头而无静温传感器，density 应改为用计算所得 SAT。

## MVP Scope
**In:**
- 后端：`FiveHoleTraversalConfig` 新增 `TTotalDeviceID`/`TTotalChannel` 字段 + `Validate` 校验
  （`TTotalDeviceID` 为空时跳过校验，即该字段可选）
- 后端：`runtimeInput` 新增 `TTotal` 字段（用于回退判断）；`calculateVelocity`/
  `toInterpolationResult` 中 `CalculateSAT` 输入按规则选择：
  `TTotal` 已配置且有效 → 用 TTotal；否则 → 用 TAtm（保旧行为）
- 后端：`collectInputWarnings` 追加 TAT 有效性校验（仅在 TTotal 已配置时触发）
- 后端：CSV writer 追加 `TTotal(℃)` 列（位于 TAtm 列之后；未配置时填空或"--"）
- 后端：配置加载无需显式迁移——`TTotalDeviceID` 默认零值（空串）即触发回退
- 前端：`FiveHoleTestView` 大气参数区追加"总温 T0 设备"行，镜像 TAtm UI（可选清空）
- 前端：实时数据卡区追加 TTotal 显示（可选，若布局允许）
- 测试：5H 配置 `Validate` 单测（含"未配置 TTotal 通过"用例）、CSV writer 单测、
  回退路径单测（旧配置 SAT 输出与升级前一致）、TAT 校验单测

**Out:**
- 恢复系数 r 可配置（Phase 2）
- `AtmosphericConfig` 嵌套重构
- 跨模块（三孔/校准）大气参数共享
- TAT/SAT 交叉一致性诊断
- 每探针 TAT 独立覆盖
- 原始电压→温度标定曲线
- density 改用计算所得 SAT（保留现状，待 TAtm 语义明确后再决策）

## Not Doing (and Why)
- 不重构 `AtmosphericConfig` —— 与"自动迁移 + 向后兼容"目标冲突，扁平字段更易迁移
- 不动恢复系数 r —— 扩大改动面，且 r 的合理默认值需用户确认，独立决策更合适
- 不跨模块共享大气参数 —— 三孔/校准模块当前无 TAT 需求，避免提前抽象
- 不改 JSON 字段名（`tAtmDeviceId` 等）—— 破坏所有旧配置文件，收益仅是语义清晰
- 不加 TAT/SAT 交叉校验 —— 锦上添花，MVP 应聚焦"有 TAT 时能正确计算 SAT"
- 不改 density 公式 —— 避免破坏旧配置的数值一致性，density 语义问题留待 TAtm
  可选化时一并处理

## Resolved Decisions
- TAT 量程校验阈值：-50~200℃
- UI 显示 TTotal 卡片：显示。未配置时显示"-"
- CSV TTotal 列：有值就显示数值，未配置时显示"-"

## Open Questions
- （已全部解决，进入实现阶段）
