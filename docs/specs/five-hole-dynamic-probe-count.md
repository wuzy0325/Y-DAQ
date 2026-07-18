# Spec: 五孔探针数量动态化（Dynamic Probe Count）

> 来源：[docs/ideas/five-hole-dynamic-probe-count.md](../ideas/five-hole-dynamic-probe-count.md)
> 阶段：Phase 1 — Specify（待人工审阅）

## Objective

让五孔移位测试模块从「默认创建 3 根探针 + 仅启用/禁用」演化为「默认 1 根，用户按需增减，上限 3 根」。

**用户故事**：
- 作为风洞测试工程师，我希望默认只看到 1 根探针的配置卡片，避免单探针场景下 2 张空卡片干扰视线
- 作为多探针同步测试工程师，我希望点击「添加探针」即可在 1→2→3 之间按需扩展，超过 3 根时按钮自动禁用
- 作为调试人员，我希望可以删除任意一根探针（包括中间的 probe2），剩余探针配置不受影响

**成功条件（Reframed Success Criteria）**：
- 默认 `loadConfig()` 在无既有配置时创建 **1** 根启用探针（probe1），而非 3 根
- 用户可在「设置弹窗」中点击「+ 添加探针」添加到最多 3 根；满 3 时按钮禁用
- 用户可点击每根探针卡片上的「删除」按钮移除该探针；至少保留 1 根（即不可删除到 0）
- 测试运行中禁用增删按钮，避免破坏状态机
- 后端 `Validate()` 保持 `enabledCount > 3` 拒绝逻辑不变
- 4-6 根上限的硬件场景（见 idea 文档）不在本 spec 范围；如未来放开，仅需调整 `MAX_PROBES` 常量

## Tech Stack

- 后端：Go 1.23（`yx-daq/internal/types`、`yx-daq/internal/five_hole`）—— **本 spec 范围内零改动**
- 前端：Vue 3 + TypeScript + Pinia + Element Plus
- 测试：Vitest（happy-dom）、Go test
- 关键文件：
  - [src/frontend/src/stores/fiveHoleTest.ts](../../src/frontend/src/stores/fiveHoleTest.ts) — `PROBE_IDS` / `defaultProbe` / `defaultConfig` / `calibLoadedMap` / `calibFilesMap`
  - [src/frontend/src/views/FiveHoleTestView.vue](../../src/frontend/src/views/FiveHoleTestView.vue) — `probeLabels` 字面量、探针配置区 v-for、删除/添加按钮
  - [src/internal/types/five_hole_traversal.go](../../src/internal/types/five_hole_traversal.go) — `Validate()` 上限校验（保持不变，仅注释更新）

## Commands

| What | How |
|------|------|
| 前端类型检查 | `cd src/frontend && npx vue-tsc --noEmit` |
| 前端构建 | `cd src/frontend && npm run build` |
| 前端单元测试 | `cd src/frontend && npm run test` |
| 前端 lint | `cd src/frontend && npm run lint` |
| Go 编译检查 | `cd src && go build ./...` |
| Go 测试 | `cd src && go test ./internal/...` |
| 端到端验证 | `cd src && wails3 dev` → 进入五孔测试界面手动验证 |

## Project Structure

```
src/frontend/src/
  stores/
    fiveHoleTest.ts          ← 修改：常量化、addProbe/removeProbe action
  views/
    FiveHoleTestView.vue     ← 修改：probeLabels 计算属性、增删按钮、滚动容器
  stores/fiveHoleTest/
    __tests__/               ← 新增/更新测试：4-6 探针场景替换为 1-3 探针动态化用例
      fiveHoleTest.test.ts
src/internal/types/
  five_hole_traversal.go    ← 仅注释更新（ProbeID 注释从「probe1/probe2/probe3」→「probe1..probeN (N≤3)」）
src/internal/five_hole/      ← 零改动
docs/specs/
  five-hole-dynamic-probe-count.md  ← 本文件
```

## Code Style

遵循 [docs/engineering/coding-standards.md](../engineering/coding-standards.md)。关键约定示例：

```typescript
// stores/fiveHoleTest.ts — 新增常量与 action

/** 最大探针数（与后端 Validate() 保持一致） */
const MAX_PROBES = 3

/** 生成 probeN 字符串（N 从 1 开始） */
function probeIdFor(n: number): string {
  return `probe${n}`
}

/** 找出当前未使用的最小探针序号（1-based） */
function nextProbeNumber(existingIds: string[]): number {
  const used = new Set(existingIds)
  for (let n = 1; n <= MAX_PROBES; n++) {
    if (!used.has(probeIdFor(n))) return n
  }
  return MAX_PROBES + 1 // 调用方应预先检查 length < MAX_PROBES
}

// action 示例
function addProbe() {
  if (config.value.probes.length >= MAX_PROBES) return
  const next = nextProbeNumber(config.value.probes.map(p => p.probeId))
  config.value.probes.push(defaultProbe(probeIdFor(next)))
}

function removeProbe(probeId: string) {
  if (config.value.probes.length <= 1) return // 至少保留 1 根
  if (isRunning.value) return                   // 运行中禁用
  const idx = config.value.probes.findIndex(p => p.probeId === probeId)
  if (idx < 0) return
  config.value.probes.splice(idx, 1)
  // 清理该 probeId 对应的 calib 缓存（后端 LoadCalibFiles 仍保留其内存中的条目，
  // 但前端 store 中的 calibLoadedMap/calibFilesMap 应同步清理）
  delete calibLoadedMap.value[probeId]
  delete calibFilesMap.value[probeId]
}
```

```vue
<!-- FiveHoleTestView.vue — 探针配置区头部 -->
<template>
  <div class="probe-list-header">
    <span>探针配置</span>
    <el-button
      type="primary"
      size="small"
      :disabled="store.isRunning || store.config.probes.length >= 3"
      @click="store.addProbe"
    >
      + 添加探针
    </el-button>
  </div>
  <div class="probe-cards-scroll">
    <el-card
      v-for="probe in store.config.probes"
      :key="probe.probeId"
      class="probe-card"
    >
      <template #header>
        <div class="probe-card-header">
          <span>{{ probeLabel(probe.probeId) }}</span>
          <el-button
            type="danger"
            size="small"
            :disabled="store.isRunning || store.config.probes.length <= 1"
            @click="store.removeProbe(probe.probeId)"
          >
            删除
          </el-button>
        </div>
      </template>
      <!-- 既有通道/位移机构/校准文件配置 -->
    </el-card>
  </div>
</template>

<script setup lang="ts">
function probeLabel(probeId: string): string {
  const match = /^probe(\d+)$/.exec(probeId)
  return match ? `探针 ${match[1]}` : probeId
}
</script>
```

## Testing Strategy

**前端测试（Vitest）** — `src/frontend/src/stores/fiveHoleTest/__tests__/`：

| 测试场景 | 期望行为 |
|---------|---------|
| `defaultConfig()` 在空配置时返回 1 根 probe1 | `probes.length === 1 && probes[0].probeId === 'probe1'` |
| `addProbe()` 在 1 根时 → 2 根（probe1, probe2） | 顺序生成，enabled=true |
| `addProbe()` 在 3 根时 → 不变（按钮禁用 + action no-op） | `probes.length === 3` |
| `removeProbe('probe2')` 在 [probe1, probe2, probe3] 时 → [probe1, probe3] | 保留剩余 ID，不重命名 |
| `removeProbe` 在 1 根时 → 不变 | 至少保留 1 根 |
| `removeProbe` 在 `isRunning=true` 时 → 不变 | 运行中禁用 |
| 删除 probe2 后 `addProbe()` → 复用 probe2 | `nextProbeNumber` 找最小未用 |
| `calibLoadedMap` 在 removeProbe 后清理对应 key | `!(probeId in calibLoadedMap)` |
| 配置持久化：保存 2 根配置 → 重新加载 → 仍是 2 根 | localStorage + 后端 ConfigStore |

**回归测试** — 现有 1-3 根测试用例继续通过：
- `motion_and_data_test.go`、`csv_writer_test.go`、`service_realtime_test.go` 中 `probe1/probe2/probe3` 字面量保持兼容
- 后端 `Validate()` 拒绝 `enabledCount > 3` 行为不变

**手动验证（端到端）**：
1. `wails3 dev` 启动，进入五孔测试界面
2. 默认看到 1 张探针卡片
3. 点击「+ 添加探针」→ 出现第 2 张
4. 添加第 3 张后按钮变灰
5. 删除中间一张 → 剩余 2 张配置保留
6. 再次添加 → 复用被删除的 ID
7. 启动测试 → 增删按钮全部禁用
8. 停止测试 → 重新可操作

## Boundaries

**Always do**:
- 修改前先运行 `cd src/frontend && npx vue-tsc --noEmit` 验证类型
- 修改后运行 `npm run test` 和 `npm run build` 双重验证
- 前端事件监听 / 定时器在 `onUnmounted` 中清理（沿用现有约定）
- 后端 `Validate()` 行为保持不变（`enabledCount > 3` 仍拒绝）

**Ask first**:
- 调整 `MAX_PROBES` 常量值（如果未来要放开到 6）
- 修改后端 `Validate()` 的探针数量上限逻辑
- 修改 `defaultConfig()` 中默认探针数为非 1 的值
- 引入新的 store state 字段（除现有 `calibLoadedMap` / `calibFilesMap`）

**Never do**:
- 修改后端 `five_hole/` 任何业务文件（service.go / data_processor.go / motion_coordinator.go / csv_writer.go 等）
- 修改 `Validate()` 中的 `enabledCount > 3` 拒绝逻辑
- 重命名/重编号现有 probeID（保持 ID 稳定，不强制连续）
- 引入「Probe Rake 探针排架」抽象或「探针托管实体」（见 idea 文档 Not Doing 部分）
- 在测试运行中允许增删探针
- 删除最后一根探针（保证至少 1 根）

## Success Criteria

- [ ] `defaultConfig()` 在无既有配置时返回 1 根 `probe1`
- [ ] 用户可在设置弹窗点击「+ 添加探针」从 1 根扩展到 3 根
- [ ] 满 3 根时添加按钮禁用（视觉 + 行为双重）
- [ ] 用户可删除任意探针；删除后剩余探针的 `probeId`、`calibFiles`、通道配置均不受影响
- [ ] 至少保留 1 根：1 根时删除按钮禁用 + `removeProbe` no-op
- [ ] 测试运行中（`isRunning=true`）增删按钮全部禁用
- [ ] 前端 `vue-tsc --noEmit` / `npm run build` / `npm run test` 全部通过
- [ ] 后端 `go test ./internal/...` 全部通过（零改动预期）
- [ ] 手动端到端验证 8 步全部通过
- [ ] 旧配置（保存了 3 根的）加载后仍可用，不被裁剪

## Open Questions

1. **3 vs 6 上限冲突**：原始 idea 文档（`docs/ideas/five-hole-dynamic-probe-count.md`）明确为「上限 6」，本次 spec 中用户最新澄清为「上限 3」。**请确认采用 3 还是 6**。
   - 若选 6：仅需将 `MAX_PROBES = 6` 并放开后端 `Validate()` 中的 `enabledCount > 3` → `> 6`；同步更新注释。
   - 若选 3：本 spec 保持不变。

2. **删除中间探针后 ProbeID 重用策略**：本 spec 采用「保留剩余 ID，下次添加复用最小未用编号」。备选方案为「重命名使 ID 连续」（probe1, probe2, probe3 删除 probe2 → probe1, probe2 ← 原 probe3）。重命名会破坏后端已加载的 calib 关联（map[probeID]*Interpolator），需要联动迁移。本 spec 选择前者（保留 ID 稳定），更安全。**请确认此策略**。

3. **probeLabel 是否需要持久化**：当前用 `probeLabel(probeId)` 计算属性从 ID 推断显示名（`probe3` → `探针 3`）。若未来需要用户自定义探针名（如「左侧探针」「校准用探针」），需新增 `displayName` 字段。本 spec 不引入，保持 ID 与显示名强绑定。**请确认不需要自定义名**。

4. **`probeLabels` 旧字面量清理**：[FiveHoleTestView.vue:606-610](../../src/frontend/src/views/FiveHoleTestView.vue#L606-L610) 现有 `probeLabels` 字面量映射 `{ probe1: '探针 1', probe2: '探针 2', probe3: '探针 3' }`。本 spec 改为计算属性 `probeLabel(id)`，删除字面量。**请确认**。

5. **配置迁移**：用户旧配置中 `probes` 数组长度可能为 3。加载时是否裁剪到 1？本 spec **不裁剪**，保持用户既有配置原样加载（仅 `migrateLegacyConfig` 不变）。新用户才得到默认 1 根。**请确认**。

## Out of Scope（明确不在本次范围）

依据 idea 文档「Not Doing」部分，以下项目刻意不做：

- **Probe Rake 探针排架抽象** —— 共享布点网格语义已足够
- **探针托管实体化** —— 无跨测试复用需求
- **紧凑视图双轨制** —— 1-3 根用现有大卡片，超过 3 根才考虑（本 spec 上限 3 不触发）
- **探针模板预设** —— 不保存「N 探针配置」为可复用模板
- **后端架构改动** —— 切片无上限、`motion_coordinator` 同步语义已支持 N 根、CSV 按探针独立导出已就绪
- **单探针故障隔离机制** —— 同步语义下整测停止（保持现状）
