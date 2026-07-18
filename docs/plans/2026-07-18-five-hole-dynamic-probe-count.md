# Plan: 五孔探针数量动态化

> 来源 spec：[docs/specs/five-hole-dynamic-probe-count.md](../specs/five-hole-dynamic-probe-count.md)
> 阶段：Phase 2 — Plan（待人工审阅）
> 日期：2026-07-18

## 1. 范围回顾

| 项 | 决策 |
|----|------|
| 探针上限 | **3**（保持后端 `Validate()` 不变） |
| 默认探针数 | **1**（新用户首次进入） |
| ProbeID 生成 | 顺序 `probeN`，删除中间探针保留剩余 ID，下次添加复用最小未用编号 |
| 后端改动 | 零（仅注释更新） |
| UI 布局 | 现有 3 栏；增删按钮在设置弹窗与每张卡片头部 |

## 2. 组件与依赖关系

```
┌─────────────────────────────────────────────────────────┐
│ A. 常量与辅助函数                                        │
│    - MAX_PROBES = 3                                       │
│    - probeIdFor(n) / nextProbeNumber(ids)                 │
│    （纯函数，无副作用，可独立单测）                       │
└────────────────┬────────────────────────────────────────┘
                 ↓
┌─────────────────────────────────────────────────────────┐
│ B. Store action：addProbe / removeProbe                  │
│    依赖 A 的辅助函数                                      │
│    修改 config.value.probes / calibLoadedMap /          │
│        calibFilesMap                                     │
└────────────────┬────────────────────────────────────────┘
                 ↓
┌─────────────────────────────────────────────────────────┐
│ C. defaultConfig() 默认 1 根 + 初始化 calib map 为 {}   │
│    依赖 A 的常量                                          │
└────────────────┬────────────────────────────────────────┘
                 ↓
   ┌─────────────┴─────────────┐
   ↓                           ↓
┌─────────────────┐     ┌────────────────────────────────┐
│ D. 测试用例      │     │ E. View：增删按钮 +           │
│  (Vitest)        │     │    probeLabel(id) 计算属性    │
│  依赖 B/C        │     │  依赖 B 暴露的 action         │
└─────────────────┘     └────────────────────────────────┘
                 ↓
┌─────────────────────────────────────────────────────────┐
│ F. 类型注释更新（Go 后端，零功能改动）                  │
│    独立任务，可与 D/E 并行                                │
└─────────────────────────────────────────────────────────┘
```

## 3. 实施顺序（顺序依赖）

| 顺序 | 任务 | 依赖 | 并行？ |
|------|------|------|--------|
| 1 | A. 常量 + 辅助函数 | 无 | 是（与 F 并行） |
| 2 | C. defaultConfig() 默认 1 根 + 初始化 map 为 `{}` | A | 否 |
| 3 | B. addProbe / removeProbe action | A, C | 否 |
| 4 | D. Vitest 测试用例（含 A/B/C 回归） | A, B, C | 否 |
| 5 | E. View 增删按钮 + probeLabel 计算属性 | B | 否 |
| 6 | F. Go 注释更新 | 无 | 是（早期可并行） |
| 7 | 验证：vue-tsc / npm run test / npm run build / go test | 全部 | 否 |

**可并行任务**：A+F 可同时进行；其余顺序执行。

## 4. 风险与缓解

| 风险 | 概率 | 影响 | 缓解 |
|------|------|------|------|
| 旧配置含 3 根 → 加载后用户看到 3 张卡片而非 1 | 中 | 低 | **预期行为**：保留旧配置不裁剪；仅新用户得到 1 根默认。spec Open Question Q5 已确认 |
| 删除 probe2 后 `enabledProbes.every(p => calibLoadedMap.value[p.probeId])` 在添加新 probe2 时返回 false | 高 | 低 | **预期行为**：新探针默认未加载 calib，`allCalibLoaded=false`，用户须重新加载。这是正确的安全行为 |
| 现有测试断言 `probes.length === 3` 会失败 | 高 | 中 | 提前 grep 现有断言并更新为新默认（`=== 1`）或显式构造 3 根配置 |
| `removeProbe` 在 `isRunning=true` 时被外部调用绕过禁用 | 低 | 中 | action 内部双重检查 `if (isRunning.value) return`；UI 按钮也禁用 |
| `delete calibLoadedMap.value[probeId]` 在 Vue 3 中可能不触发响应式更新 | 中 | 中 | `calibLoadedMap` 是 `ref<Record>`，`delete` 操作 Vue 3 默认 reactive 但 `ref` 包裹的 object 需测试；备选用 `{ ...calibLoadedMap.value, [probeId]: undefined }` 或 `Object.assign` |
| 后端 `motion_coordinator.go` 假设最多 3 根 | 低 | 高 | 后端代码已用切片 `[]FiveHoleProbeConfig` 与 `map[probeID]*Interpolator`，无字面量 3；本 spec 上限仍为 3，无回归风险 |
| 五孔测试 `probeLabels` 在 View 中两处使用（行 118, 474） | 低 | 低 | 全部替换为 `probeLabel(probe.probeId)` 函数调用 |

## 5. 验证检查点

| 检查点 | 命令 | 通过条件 |
|--------|------|----------|
| CP-1 | `cd src/frontend && npx vue-tsc --noEmit` | 0 errors |
| CP-2 | `cd src/frontend && npm run test -- --run fiveHoleTest` | 新增用例全过 + 既有用例无回归 |
| CP-3 | `cd src/frontend && npm run build` | Vite 构建成功 |
| CP-4 | `cd src/frontend && npm run lint` | 0 errors（warning 可接受） |
| CP-5 | `cd src && go build ./...` | 编译成功 |
| CP-6 | `cd src && go test ./internal/...` | 现有用例全过（零回归） |
| CP-7 | 手动 `wails3 dev` 端到端 8 步 | spec 成功条件全部满足 |

## 6. 并行 vs 顺序总结

- **顺序链**（关键路径）：A → C → B → D → 验证
- **可并行**：
  - F（Go 注释）可与 A 同步进行
  - E（View）可在 B 完成后与 D 并行（不同文件，互不冲突）
- **不并行**：C 与 B 必须串行（B 依赖 C 的 store 结构）；D 必须在 A/B/C 完成后

## 7. 关键技术决策（PLAN 阶段澄清）

### 7.1 `calibLoadedMap` / `calibFilesMap` 初始值

**决策**：从 `{ probe1: false, probe2: false, probe3: false }` 改为 `{}`（空对象）。

**理由**：
- 现有字面量耦合了 3 根假设
- 空对象 + 动态读写更符合「按需扩展」语义
- 现有代码读取 `calibLoadedMap.value[probe.probeId]` 在 key 不存在时返回 `undefined`，与 `false` 在 `if` 中等效

**风险**：`enabledProbes.value.every(p => calibLoadedMap.value[p.probeId])` 中 `undefined` 是 falsy，行为正确。

### 7.2 `removeProbe` 不删除后端已加载的 calib

**决策**：前端 `removeProbe` 仅清理前端 `calibLoadedMap` / `calibFilesMap`，不调用后端 API 卸载 calib。

**理由**：
- 后端 `FiveHoleTraversalService.interpolators` 是 `map[probeID]*Interpolator`，被删除的 probeID 留在 map 中无害（下次 `LoadCalibFiles(probeID, ...)` 时复用）
- 添加回同 ID 的探针时，后端 calib 仍可用（避免用户重新选文件）
- 减少前后端往返调用

**风险**：若用户删除 probe2 后又添加新的 probe2，会直接显示「已加载」状态。这是 **预期行为**（用户友好，避免重复操作）。

### 7.3 配置持久化时机

**决策**：`addProbe` / `removeProbe` 不主动调用 `saveConfig()`。沿用现有自动保存机制（`useAutoSaveConfig` composable 在 watch config 变化时触发）。

**理由**：与现有 `probe.enabled = !probe.enabled` 等内联修改一致的语义。

### 7.4 View 中两处 `probeLabels` 替换

**位置**：
- [FiveHoleTestView.vue:118](../../src/frontend/src/views/FiveHoleTestView.vue#L118) — `<GlassCard :title="probeLabels[probe.probeId] || probe.probeId"`
- [FiveHoleTestView.vue:474](../../src/frontend/src/views/FiveHoleTestView.vue#L474) — `:label="probeLabels[probe.probeId] || probe.probeId"`

**替换为**：`probeLabel(probe.probeId)`（函数调用，已包含 fallback 逻辑）

### 7.5 滚动容器

**决策**：在探针卡片外层加 `<div class="probe-cards-scroll" style="overflow-x: auto">`。

**理由**：spec Open Question Q1 已确认上限为 3，本不需要滚动；但加入容器作为防御性措施，未来放开到 6 时无需改结构。

## 8. 不在 PLAN 范围

- 后端 `Validate()` 修改（保持 `> 3` 拒绝）
- `defaultConfig()` 之外的默认值变更（如 `dwellTimeMs` 等保持现状）
- `migrateLegacyConfig()` 修改（保留旧字段迁移逻辑不变）
- `completeProbeDataPoints` 类型签名变更（已支持任意 probeID map）
- 五孔校准模块（`src/internal/calibration/`，与移位测试模块解耦）
