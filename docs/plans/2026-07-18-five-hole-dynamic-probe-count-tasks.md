# Tasks: 五孔探针数量动态化

> 来源 plan：[docs/plans/2026-07-18-five-hole-dynamic-probe-count.md](2026-07-18-five-hole-dynamic-probe-count.md)
> 阶段：Phase 3 — Tasks（待人工审阅）

## 执行原则

- 每个任务可在一次专注会话内完成
- 任务严格按依赖顺序执行
- 每个任务完成后立即执行 Verify 步骤；失败不得推进下一任务
- 单个任务涉及文件 ≤ 5
- 完成后勾选 `[x]` 并记录实际验证结果

---

## Task 1: 添加常量与辅助函数

- [ ] **Task**: 在 `fiveHoleTest.ts` 顶部新增 `MAX_PROBES = 3` 常量及 `probeIdFor(n)` / `nextProbeNumber(existingIds)` 纯函数。删除（或改造）现有 `PROBE_IDS` 字面量数组。
- **Acceptance**:
  - `MAX_PROBES` 常量定义为 `3`
  - `probeIdFor(1) === 'probe1'`，`probeIdFor(3) === 'probe3'`
  - `nextProbeNumber(['probe1', 'probe3']) === 2`（找最小未用）
  - `nextProbeNumber(['probe1', 'probe2', 'probe3']) === 4`（满员时返回 MAX_PROBES+1）
  - 不再导出或引用 `PROBE_IDS` 字面量
  - `vue-tsc --noEmit` 通过
- **Verify**: `cd src/frontend && npx vue-tsc --noEmit`
- **Files**: `src/frontend/src/stores/fiveHoleTest.ts`

---

## Task 2: 修改 defaultConfig 与 calibMap 初始化

- [ ] **Task**: 修改 `defaultConfig()` 使其仅创建 1 根探针（probe1，enabled=true）。将 `calibLoadedMap` / `calibFilesMap` 的初始值从硬编码 `{ probe1: false, probe2: false, probe3: false }` 改为空对象 `{}`。
- **Acceptance**:
  - `defaultConfig().probes.length === 1`
  - `defaultConfig().probes[0].probeId === 'probe1'`
  - `defaultConfig().probes[0].enabled === true`
  - `defaultConfig()` 其余字段（layout/dwellTimeMs/samplesPerPoint 等）保持不变
  - `calibLoadedMap` / `calibFilesMap` 初始为 `{}`
  - 既有读取代码 `calibLoadedMap.value[probe.probeId]` 在 key 不存在时返回 `undefined`，与原 `false` 在 `if` 上下文中行为等效（无需修改读取逻辑）
- **Verify**: `cd src/frontend && npx vue-tsc --noEmit`
- **Files**: `src/frontend/src/stores/fiveHoleTest.ts`

---

## Task 3: 实现 addProbe / removeProbe action

- [ ] **Task**: 在 store 内新增 `addProbe()` 和 `removeProbe(probeId: string)` 两个 action，并在 store return 中暴露。
- **Acceptance**:
  - `addProbe()` 行为：
    - `config.probes.length >= MAX_PROBES` 时 no-op
    - `isRunning.value === true` 时 no-op
    - 否则调用 `nextProbeNumber(existingIds)` 计算下一个序号，构造 `defaultProbe(probeIdFor(n))` 并 push 到 `config.value.probes`
  - `removeProbe(probeId)` 行为：
    - `config.probes.length <= 1` 时 no-op（至少保留 1 根）
    - `isRunning.value === true` 时 no-op
    - `probeId` 不存在时 no-op
    - 否则 splice 删除该探针，并 `delete calibLoadedMap.value[probeId]` / `delete calibFilesMap.value[probeId]`
    - **不**调用后端 API 卸载 calib（保留后端 interpolators map 条目，便于同 ID 复用）
  - 两个 action 在 store return 中导出
  - `vue-tsc --noEmit` 通过
- **Verify**: `cd src/frontend && npx vue-tsc --noEmit`
- **Files**: `src/frontend/src/stores/fiveHoleTest.ts`

---

## Task 4: 新增 Vitest 测试用例

- [ ] **Task**: 在 `src/frontend/src/stores/__tests__/fiveHoleTest.test.ts` 新增 `describe('addProbe / removeProbe 动态化', ...)` 测试组，覆盖 spec「Testing Strategy」表中的 9 个场景。同步更新任何断言 `probes.length === 3` 的既有用例（改为显式构造 3 根配置或断言 `=== 1`）。
- **Acceptance**:
  - 新增至少 9 个测试用例，覆盖：
    1. `defaultConfig()` 返回 1 根 probe1
    2. `addProbe()` 从 1 根扩展到 2 根（probe1, probe2）
    3. `addProbe()` 在 3 根时 no-op
    4. `removeProbe('probe2')` 在 [probe1, probe2, probe3] 时 → [probe1, probe3]
    5. `removeProbe` 在 1 根时 no-op
    6. `removeProbe` 在 `isRunning=true` 时 no-op
    7. 删除 probe2 后 `addProbe()` → 复用 probe2 ID
    8. `removeProbe` 后 `calibLoadedMap` / `calibFilesMap` 对应 key 已删除
    9. 配置持久化：保存 2 根配置 → 重新加载 → 仍是 2 根
  - 既有用例无回归（grep `probes.length === 3` 等硬编码断言并修正）
  - 全部测试通过
- **Verify**: `cd src/frontend && npm run test -- --run fiveHoleTest`
- **Files**: `src/frontend/src/stores/__tests__/fiveHoleTest.test.ts`

---

## Task 5: 修改 View 增删按钮与 probeLabel

- [ ] **Task**: 修改 `FiveHoleTestView.vue`：(a) 将 `probeLabels` 字面量改为 `probeLabel(probeId)` 函数并替换两处使用点（行 118、474）；(b) 在探针配置区头部加「+ 添加探针」按钮（满 3 或运行中禁用）；(c) 在每张探针卡片头部加「删除」按钮（仅剩 1 根或运行中禁用）；(d) 在探针卡片容器外层加 `<div style="overflow-x: auto">` 防御性滚动容器。
- **Acceptance**:
  - `probeLabels` 字面量已删除
  - 两处使用点改为 `probeLabel(probe.probeId)` 函数调用
  - `probeLabel('probe3') === '探针 3'`，`probeLabel('unknown') === 'unknown'`（fallback）
  - 「+ 添加探针」按钮在 `probes.length >= 3` 或 `isRunning` 时 `:disabled=true`
  - 「删除」按钮在 `probes.length <= 1` 或 `isRunning` 时 `:disabled=true`
  - 删除按钮触发 `store.removeProbe(probe.probeId)`
  - 添加按钮触发 `store.addProbe()`
  - 探针卡片容器有水平滚动样式
  - `vue-tsc --noEmit` 通过
  - `npm run build` 成功
- **Verify**: `cd src/frontend && npx vue-tsc --noEmit && npm run build`
- **Files**: `src/frontend/src/views/FiveHoleTestView.vue`

---

## Task 6: 更新后端类型注释

- [ ] **Task**: 修改 `src/internal/types/five_hole_traversal.go` 中 `FiveHoleProbeConfig.ProbeID` 字段注释，从 `// probe1/probe2/probe3` 改为 `// probe1..probeN (N≤3)`。无功能改动。
- **Acceptance**:
  - 注释已更新
  - Go 编译通过
  - Go 测试全部通过（零回归）
- **Verify**: `cd src && go build ./... && go test ./internal/...`
- **Files**: `src/internal/types/five_hole_traversal.go`

---

## Task 7: 全量验证与端到端检查

- [ ] **Task**: 运行 spec「Success Criteria」与 plan「CP-1 ~ CP-7」全部验证步骤；记录结果。
- **Acceptance**:
  - CP-1 vue-tsc: 0 errors
  - CP-2 npm run test: 全部通过
  - CP-3 npm run build: 成功
  - CP-4 npm run lint: 0 errors
  - CP-5 go build: 成功
  - CP-6 go test: 现有用例全过
  - CP-7 手动端到端 8 步全部通过（可选，需用户配合）
- **Verify**: 依次执行上述命令
- **Files**: 无（仅运行验证）

---

## 任务依赖图

```
Task 1 (常量+辅助函数)
  ↓
Task 2 (defaultConfig + calibMap init)
  ↓
Task 3 (addProbe / removeProbe)
  ↓
Task 4 (Vitest 测试)  ←─── 也可与 Task 5 并行
                          ↘
Task 5 (View 修改)       ←─┘
  ↓
Task 6 (Go 注释)  ←─ 可与 Task 1-5 任意时刻并行
  ↓
Task 7 (全量验证)
```

## 总文件清单

| 文件 | 改动类型 |
|------|---------|
| `src/frontend/src/stores/fiveHoleTest.ts` | 修改（Task 1, 2, 3） |
| `src/frontend/src/stores/__tests__/fiveHoleTest.test.ts` | 修改（Task 4） |
| `src/frontend/src/views/FiveHoleTestView.vue` | 修改（Task 5） |
| `src/internal/types/five_hole_traversal.go` | 修改（Task 6，仅注释） |

总计 4 个文件改动，0 个新增文件。
