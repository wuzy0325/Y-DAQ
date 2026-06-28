# Plan & Tasks: 五孔插值算法补足

> Phase 2 Plan + Phase 3 Tasks（基于已批准的 `five-hole-interpolation-spec.md`）
> Open Questions 结论：
> 1. 前端保留既有 6 个数值卡，新字段仅进 CSV。
> 2. `V` 与 `Velocity` 在 `toInterpolationResult` 中均赋值 `result.V`（TAS），二者等价 → `VelocityProbe` 用 `Velocity`。
> 3. 测试用合成数据（参考仓库 `syntheticCalLines`）。

## 实现顺序（依赖驱动）

```
T1 vendor 算法包 ──┐
                   ├─→ T3 适配器 ──┐
T2 更新 types ─────┘                ├─→ T5 测试 ──→ T7 重生成绑定+前端 ──→ T8 验证
                   T4 CSV writer ──┘
                   T6 service(若需)
```

## Tasks

- [ ] **T1 vendor 算法包**（src/internal/five_hole/interpolation/）
  - Acceptance: `go build ./internal/five_hole/interpolation/...` 通过；删除未引入的 `five_hole_new_interpolator.go`；裁剪测试文件去掉 New 插值器用例
  - Verify: `cd src && go build ./internal/five_hole/interpolation/... && go test ./internal/five_hole/interpolation/...`
  - Files: 删 `five_hole_new_interpolator.go`、`five_hole_new_interpolator_test.go`；新建 `cal_interpolator_test.go`（裁剪后用例）

- [ ] **T2 更新类型**（src/internal/types/five_hole_traversal.go）
  - Acceptance: `FiveHoleInterpolationResult` 去 IterationCount/Converged，加 CASProbe/SATProbe/DynamicPressure/Density/VxProbe/VyProbe/VzProbe；`FiveHoleCalibFileInfo` 加 ValidRange 字段；删 `FiveHoleCalibEntry`/`FiveHoleCalibData`（grep 确认无外部引用）
  - Verify: `cd src && go build ./...`
  - Files: five_hole_traversal.go（可能波及 calibration.go 若有 Entry/Data 引用）

- [ ] **T3 重写适配器**（src/internal/five_hole/interpolator.go）
  - Acceptance: LoadCalibFiles 读 .cal → MultiCalInterpolator.LoadCalData；Calculate 映射 InterpolationResult→FiveHoleInterpolationResult；GetCalibInfo 返回新结构
  - Verify: `cd src && go build ./internal/five_hole/...`
  - Files: interpolator.go

- [ ] **T4 更新 CSV writer**（src/internal/five_hole/csv_writer.go）
  - Acceptance: 表头去"迭代次数/收敛"，加 CAS/SAT/动压/密度/Vx/Vy/Vz
  - Verify: `cd src && go build ./internal/five_hole/...`
  - Files: csv_writer.go

- [ ] **T5 重写测试**（interpolator_test.go, csv_writer_test.go）
  - Acceptance: 适配器测试（13×13 合成 .cal，单/多文件，异常）；CSV 测试新列；全绿
  - Verify: `cd src && go test ./internal/five_hole/...`
  - Files: interpolator_test.go, csv_writer_test.go

- [ ] **T6 service 层适配**（src/internal/five_hole/service.go 等，按需）
  - Acceptance: GetCalibInfo/LoadCalibFiles 编译通过；无遗留占位引用
  - Verify: `cd src && go build ./internal/five_hole/...`
  - Files: service.go, event_handler.go（如有 GetCalibInfo 透传）

- [ ] **T7 重生成绑定 + 前端同步**
  - Acceptance: types.ts 接口同步；fiveHoleTest.ts CSV 列同步；FiveHoleTestView.vue 去无效卡片；`npm run build` 通过
  - Verify: `cd src && wails3 generate bindings -clean=true -ts` → `cd src/frontend && npm run build`
  - Files: bindings/(自动), stores/fiveHoleTest/types.ts, stores/fiveHoleTest.ts, views/FiveHoleTestView.vue

- [ ] **T8 最终验证**
  - Verify: `cd src && go build ./... && go test ./internal/...`；`cd src/frontend && npm run build`；`cd src && wails3 task build DEV=true`
