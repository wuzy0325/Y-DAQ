# Spec: 五孔插值移位测试模块 —— 插值算法补足

> 阶段：Phase 1 Specify（待人工评审）
> 算法来源：`https://github.com/wuzy0325/AI-WorkSpace/tree/master/shared/algorithms/go/fivehole/interpolation`

## Objective

将五孔移位测试模块中**占位的插值器**（`src/internal/five_hole/interpolator.go`，当前 `Calculate` 直接返回"未实现"）替换为仓库 `shared/algorithms/go/fivehole/interpolation` 中的**正式 CAL 插值算法**，并完成与 YX-DAQ 现有服务层、CSV 导出、前端展示的集成。

### 用户故事 / 验收条件

- 五孔测试运行时，每个布点采样后的 `Calculate` 返回**有效**的物理量（α、β、Ma、V、Pt、Ps 及扩展量 CAS/SAT/动压/密度/速度分量），而非"未实现"。
- 每根探针可独立载入**多个不同马赫数**的 `.cal` 校准文件；运行时按实测 Ma 在两相邻马赫数文件间线性插值（单文件时退化为直接计算）。
- 校准文件格式（首行 `13 13`，后续 169 行 `ka kb cpt cps alpha beta`），扩展名 `.cal`。
- 算法源码以**纯算法包**形式引入，不含文件 I/O；YX-DAQ 侧通过薄适配器读盘并喂入文本行。

## Tech Stack

- Go 1.23（算法 vendoring + 适配器）
- 既有：Wails v3、Vue 3 + TypeScript（前端绑定 / 类型受影响，需同步更新）
- 算法依赖：仅 Go 标准库（`math`、`sort`、`strings`、`encoding/csv`、`path/filepath`、`regexp`、`fmt`）

## Commands

> 所有 `wails3` / `go` 命令在 `src/` 目录运行。

| 用途 | 命令 |
|------|------|
| Go 编译检查 | `cd src && go build ./...` |
| Go 单测 | `cd src && go test ./internal/five_hole/...` |
| Go 全量测试 | `cd src && go test ./internal/...` |
| 前端类型检查 + 构建 | `cd src/frontend && npm run build` |
| 前端单测 | `cd src/frontend && npm run test` |
| 重新生成 Wails 绑定 | `cd src && wails3 generate bindings -clean=true -ts` |
| 构建 exe（调试） | `cd src && wails3 task build DEV=true` |

## Project Structure

### 新增（vendored 算法包，纯算法无 I/O）

```
src/internal/five_hole/interpolation/
├── types.go                          # InterpolationResult/InterpolationInput/CalValidRange/CalFileInfo/Interpolator 接口
├── atmospheric_data.go               # AtmosphericDataCalculator（Ma/SAT/CAS/TAS/动压/密度）
├── cal_interpolator.go               # CalInterpolator（13×13 网格 9 区域插值）
├── multi_cal_interpolator.go         # MultiCalInterpolator（多 Ma 文件线性/最近邻插值）
├── cal_interpolator_test.go          # 仓库自带测试
├── multi_cal_interpolator_test.go    # 仓库自带测试（如有）
└── atmospheric_data_test.go          # 仓库自带测试（如有）
```

> vendored 文件**只改 package 名**（`interpolation`，保持不变），其余照搬；不修改任何算法逻辑。

### 修改

| 文件 | 改动 |
|------|------|
| `src/internal/five_hole/interpolator.go` | 重写为**适配器**：读 `.cal` 文件 → 提取文本行 → 调 `MultiCalInterpolator.LoadCalData` → 适配 `types.FiveHoleRawData`↔`InterpolationInput`、`InterpolationResult`↔`types.FiveHoleInterpolationResult` |
| `src/internal/types/five_hole_traversal.go` | `FiveHoleInterpolationResult` 移除 `IterationCount`/`Converged`，新增 `CASProbe`/`SATProbe`/`DynamicPressure`/`Density`/`VxProbe`/`VyProbe`/`VzProbe`；`FiveHoleCalibFileInfo` 增 `ValidRange`（α/β/Ma min/max） |
| `src/internal/five_hole/csv_writer.go` | 表头/数据行移除"迭代次数/收敛"列，新增"校正空速CAS/静温SAT/动压Qc/密度ρ/Vx/Vy/Vz"列 |
| `src/internal/five_hole/service.go` | `LoadCalibFiles` 透传新 `GetCalibInfo`；`calculateForProbe` 不变（仍调 `interpolator.Calculate`） |
| `src/internal/five_hole/interpolator_test.go` | 重写：基于新 `.cal` 格式（首行 `13 13`）+ 真实算法预期 |
| `src/internal/five_hole/csv_writer_test.go` | 更新数据点构造（去 IterationCount/Converged，加新字段） |
| `src/frontend/src/stores/fiveHoleTest/types.ts` | `FiveHoleInterpolationResult` 接口同步后端字段 |
| `src/frontend/src/stores/fiveHoleTest.ts` | CSV 导出列同步（去 iterationCount，加新列） |
| `src/frontend/src/views/FiveHoleTestView.vue` | 移除"迭代次数"等无效卡片（如有）；现有 6 个数值卡（α/β/Ma/V/Pt/Ps）保留 |
| `src/frontend/bindings/` | Wails 自动重新生成（禁止手改） |

### 删除

| 项 | 原因 |
|----|------|
| `types.FiveHoleCalibEntry` | 旧占位格式（CMa 首行）专用，新算法用 vendored `probeTableRow` |
| `types.FiveHoleCalibData` | 同上，新算法状态封装在 `MultiCalInterpolator` 内 |

> 删除前确认无其他引用；若 `GetCalibInfo` 等公共接口仍需暴露校准信息，改用 `FiveHoleCalibFileInfo` 承载。

## Code Style

遵循 `docs/engineering/coding-standards.md`。关键点：

- vendored 包 `package interpolation`，文件小写加下划线，照搬仓库原貌（注释保留中文）
- 适配器 `interpolator.go` 保持 `package five_hole`，错误用 `%w` 包装，日志用 `log/slog`
- 类型字段中文注释，JSON tag 保留 camelCase
- 前端 `<style lang="scss" scoped>`，TS 路径别名 `@`、`@bindings`

适配器核心示例（示意，非最终代码）：

```go
// interpolator.go —— 适配器
type FiveHoleInterpolator struct {
    multi *interpolation.MultiCalInterpolator
    infos []types.FiveHoleCalibFileInfo
}

func (i *FiveHoleInterpolator) LoadCalibFiles(filePaths []string) error {
    fileData, err := readCalFilesToLines(filePaths) // os.Open + bufio 扫行
    if err != nil { return fmt.Errorf("读取 .cal 文件失败: %w", err) }
    res, err := i.multi.LoadCalData(fileData, nil)  // nil: 从文件名解析 Ma
    if err != nil { return fmt.Errorf("加载校准数据失败: %w", err) }
    i.infos = toCalibInfos(res)
    return nil
}

func (i *FiveHoleInterpolator) Calculate(raw types.FiveHoleRawData) types.FiveHoleInterpolationResult {
    in := interpolation.InterpolationInput{P1: raw.P1, P2: raw.P2, P3: raw.P3, P4: raw.P4, P5: raw.P5, PAtm: raw.PAtm, TAtm: raw.TAtm}
    r, err := i.multi.Calculate(in)
    if err != nil || !r.IsValid {
        return types.FiveHoleInterpolationResult{Valid: false, ErrorMsg: errOrNil(err, r.Warning)}
    }
    return mapResult(r) // 字段映射
}
```

## Testing Strategy

- **vendored 算法测试**：照搬仓库 `*_test.go`，确保 `go test ./internal/five_hole/interpolation/...` 通过（回归保护，证明 vendoring 无损）。
- **适配器测试**（重写 `interpolator_test.go`）：
  - 构造 13×13 合成 `.cal` 文件（参考仓库 `syntheticCalLines`）→ `LoadCalibFiles` 成功
  - `Calculate` 返回 `Valid=true`，α/β 在合理范围
  - 多文件场景：两不同 Ma 文件 → `Calculate` 在区间内返回带"线性插值"警告
  - 异常：空文件、行列数错、非 13×13 → 返回错误
- **CSV 写入测试**（更新 `csv_writer_test.go`）：新表头列齐全，数据行新字段非零
- **前端**：`npm run build` 通过（类型同步）；`npm run test` 通过
- **集成**：`go build ./...` + `wails3 task build DEV=true` 产出 exe

测试文件位置：
- `src/internal/five_hole/interpolation/*_test.go`（vendored）
- `src/internal/five_hole/interpolator_test.go`（适配器）
- `src/internal/five_hole/csv_writer_test.go`（CSV）

## Boundaries

- **Always do**:
  - vendored 算法文件**只改 package 名**，不改动任何算法逻辑/常量/公式
  - 每个 task 完成后跑 `go build ./...` + 受影响包的 `go test`
  - 删除类型前用 Grep 确认全仓库无引用
  - 重新生成 Wails 绑定后 `npm run build` 验证前端类型
- **Ask first**:
  - 修改 vendored 算法的常量（如网格步长 5°、角度范围 ±30°）—— 这些是算法硬约束
  - 调整 `MultiCalInterpolationMode` 默认模式（仓库默认 `ModeLinear`）
  - 前端新增额外数值卡（超出既有 6 个）
- **Never do**:
  - 手动编辑 `src/frontend/bindings/`（Wails 自动生成）
  - 在 vendored 包内引入 `os`/`io` 文件 I/O（破坏纯算法边界）
  - 保留 `IterationCount`/`Converged` 字段做"兼容"（用户已确认移除）

## Success Criteria

1. `cd src && go test ./internal/five_hole/...` 全部通过（vendored + 适配器 + CSV）
2. `cd src && go build ./...` 无错误
3. `cd src/frontend && npm run build` 无类型错误
4. 用 13×13 合成 `.cal` 文件载入后 `Calculate` 返回 `Valid=true`，α/β/Ma/V/Pt/Ps 均为有限值
5. 载入两个不同 Ma 的 `.cal` 文件时，`Calculate` 结果含"线性插值"警告且 `Valid=true`
6. 非法 `.cal` 文件（非 13×13、列数错）载入返回明确错误
7. CSV 表头列与新 `FiveHoleInterpolationResult` 字段一一对应，无"迭代次数/收敛"
8. 前端五孔测试界面既有 6 个数值卡正常显示实时插值结果

## Open Questions

1. **前端数值卡**：现有 6 卡（α/β/Ma/V/Pt/Ps）是否足够，还是需新增 CAS/SAT/动压/密度卡？（默认：保留 6 卡，新字段仅进 CSV；如需加卡请指示）
2. **`Velocity` 字段映射**：仓库 `InterpolationResult` 同时有 `V` 和 `Velocity`（TAS）。需在实现阶段核对 `toInterpolationResult` 实际填充哪个为"速度"，将 TAS 映射到 `VelocityProbe`（默认用 `Velocity`）。
3. **校准文件示例**：`docs/reference/参考输入/` 下有 `三孔校准示例_Ma0.3.cal`。五孔 `.cal` 示例是否已存在？若无，测试用合成数据（参考仓库 `syntheticCalLines`）。

## 算法来源映射

| YX-DAQ 用途 | 仓库文件 | 说明 |
|-------------|---------|------|
| 单文件直接插值 | `cal_interpolator.go` `CalInterpolator` | 13×13 网格 9 区域（4 角+4 边+1 中心） |
| 多文件 Ma 插值 | `multi_cal_interpolator.go` `MultiCalInterpolator` | 中间 Ma 文件算初始 Ma → 区间线性/最近邻 |
| 大气数据计算 | `atmospheric_data.go` `AtmosphericDataCalculator` | Ma/SAT/CAS/TAS/动压/密度 |
| 类型契约 | `types.go` | `InterpolationInput`/`InterpolationResult`/`CalValidRange` |

> 仓库的 `five_hole_new_interpolator.go`（AA 公式/CSV 格式）**不引入**，与本需求".cal 格式"不符。
