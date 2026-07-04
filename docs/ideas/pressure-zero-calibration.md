# 压力采集设备校零功能

## Problem Statement

How Might We 让操作员在采集前快速建立压力通道的零位基准，使得所有下游消费者（仪表盘显示、CSV 录制、三孔/五孔测试、五孔角度校准）看到一致且无偏移的压力读数？

## Recommended Direction

**在 `AcquisitionHub.OnData` 入口应用单位感知的零位偏移，扩展 `ChannelConfig` 持久化校零元数据，DeviceView 通道表格内联校零 UI。**

核心数据流改动点在 [src/internal/app/core.go L75-80](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/app/core.go#L75-L80) 的 `dataSink` 回调：在转发给 `AcquisitionHub.OnData` 和 `DataStorage.HandlePayload` **之前**对 `payload.Channels` 减去零位偏移。这一处插入即可透明影响所有消费者（前端 20Hz 快照、CSV 录制、三孔/五孔测试 batchGetter、五孔角度校准 DataGetter），无需在各消费者各自修正。

校零时连续采样 10 帧（固定值，不可配置），对每个启用通道取均值作为零位值，连同当前单位一起存入 `ChannelConfig.ZeroOffset` + `ChannelConfig.ZeroOffsetUnit`。应用偏移时若 `ChannelUnit != ZeroOffsetUnit`，按 Pa 基准换算后相减。存储沿用 `devices.json`，与通道配置同生命周期，删除通道时零位字段随之清理。

UI 层在 [DeviceView.vue](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/frontend/src/views/DeviceView.vue) 的通道编辑表格新增"零位"列：显示 `101325.0 Pa (校准于 14:30)` 或"未校准"，每行带"校零"按钮，表头带"批量校零"按钮，已校准时显示"去校零"按钮。操作粒度按设备批量校零（一键校准该设备所有启用压力通道），同时保留通道级单独校零入口。

## Key Assumptions to Validate

- [ ] **所有压力通道单位可换算到 Pa 基准** —— 设备协议（[xy_daq16.go L380-403](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/driver/xy_daq16.go#L380-L403)）硬件支持 9 种压力单位，但 **UI 与校零白名单只暴露 6 种**：psi / kgf/cm² / bar / kPa / MPa / Pa（删除 mmHg / atm / mbar，因实验室不常用且易混淆）。换算系数复用驱动已有 `unitToCoeff` 表（系数 = 1 psi 对应的该单位值，如 kPa 系数 6.89476 意味 1 psi = 6.89476 kPa；换算到 Pa 时 `value_Pa = value_unit × 6894.76 / coeff`）。温度通道（°C，YX-DAQ-T 设备）不在压力设备类型中，天然不会触发校零。验证方式：6 种单位往返换算（unit → Pa → unit）应恢复原值（误差 < 1e-9）。
- [ ] **校零时设备处于零压状态** —— 软件无法验证物理状态，依赖操作员。验证方式：UI 显示校零时刻原始读数 + 单位，让操作员肉眼核对"这看起来像零吗"。
- [ ] **10 帧均值足够稳定** —— 20Hz 下约 0.5 秒即可采完，运动控制启停瞬间会有压力扰动。验证方式：在模拟设备上人为注入噪声测试均值稳定性；校零时 UI 提示"请保持设备静止"（不禁用其他操作，因采样很快）。
- [ ] **换单位后旧偏移可精确换算** —— 压力单位换算系数精确（1 kPa = 1000 Pa），浮点误差可忽略。验证方式：换算后四舍五入到通道 `Precision` 字段指定的小数位。
- [ ] **偏移在 hub 入口应用不影响调试** —— 若需查看驱动原始读数，靠 `slog.Debug` 日志。验证方式：在 `OnData` 修正前后各打一条 Debug 日志，开发模式可查。

## MVP Scope

**In:**
- `types.ChannelConfig` 新增 `ZeroOffset float64` + `ZeroOffsetUnit string` + `ZeroCalibratedAt time.Time` 字段
- `DeviceManager` 新增 `ZeroCalibrate(deviceID string) error`（批量校零该设备所有启用压力通道，固定 10 帧采样均值）+ `ZeroCalibrateChannel(deviceID, channelIndex) error` + `ClearZeroOffset(deviceID, channelIndex) error` + `ClearAllZeroOffsets(deviceID) error`
- `ClearZeroOffset` 语义：将 `ZeroOffset = 0` + `ZeroOffsetUnit = ""` + `ZeroCalibratedAt = time.Time{}` 写回 `ChannelConfig` 并持久化，运行时偏移随即失效（采集数据不再减去任何值）
- 单位换算工具函数 `convertPressureToPa(value, fromUnit) (float64, error)` + `convertPaToUnit(value, toUnit) (float64, error)`，复用 [xy_daq16.go L380-403](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/driver/xy_daq16.go#L380-L403) 已有 `unitToCoeff` 系数表，白名单 6 种：psi / kgf/cm² / bar / kPa / MPa / Pa
- 压力设备单位下拉选项裁剪为 6 种（psi / kgf/cm² / bar / kPa / MPa / Pa），删除 mmHg / atm / mbar——操作员无法选到这 3 种，自然不会触发校零失败
- 边缘情况：若设备硬件 EU 单位为 mmHg/atm/mbar（`readAndUpdateEUUnit` 读到），通道 `Unit` 字段会被设为这 3 种之一；校零时检测到非白名单单位应报错"当前单位 {unit} 不支持校零，请先切换到 {白名单}"
- `core.go` 的 `dataSink` 回调中插入偏移应用逻辑（在 `AcquisitionHub.OnData` 和 `DataStorage.HandlePayload` 之前）
- `DeviceService` 暴露上述 4 个方法给前端
- 前端 `device.ts` store 新增 `zeroCalibrate` / `zeroCalibrateChannel` / `clearZeroOffset` / `clearAllZeroOffsets` action（参考 `setUnit` 的 `withDeviceAction` 包装器写法）
- `DeviceView.vue` 通道编辑表格新增"零位"列：显示零位值+单位+校准时间，或"未校准"；每行"校零"/"去校零"按钮；表头"批量校零"按钮；校零期间不禁用其他 UI
- `wails3 generate bindings` 重新生成前端绑定

**Out:**
- 3σ 滤波（多帧均值已足够）
- 漂移监控 / 重校零提醒
- 三孔/五孔测试启动前自动校零（保持手动触发）
- 校零历史审计 / 导出
- CSV 录制原始值（全局统一修正，简化优先）
- 独立 `zero_offsets.json` 存储文件（与 ChannelConfig 同生命周期更简单）

## Not Doing (and Why)

- **3σ 滤波离群点剔除** —— 1 秒 20 帧均值已能平滑瞬时噪声；3σ 增加 ~50 行代码与测试，MVP 阶段过度设计。若实测发现校零值不稳定再补。
- **漂移监控与重校零提醒** —— 超出"校零"核心需求；需引入历史记录存储与阈值配置，体量大。未来增强项。
- **三孔/五孔测试启动前自动校零** —— 用户明确要求"实验员操作前手动校准"。自动钩子会剥夺操作员对物理状态的判断权（操作员需确认管路处于零压状态才能校零）。
- **校零历史审计 / 导出** —— 无明确需求；实验室通常靠纸质记录或 SOP 而非软件审计。
- **CSV 录制原始值（双轨制）** —— 用户选择"全局统一修正"。双轨制会让 CSV 列含义模糊（原始还是修正？），简化优先。若未来需追溯，可加 `slog.Debug` 日志或独立调试模式。
- **独立 `zero_offsets.json` 存储文件** —— 零位与通道强绑定（删除通道应级联清理零位），扩展 `ChannelConfig` 字段即可随 `devices.json` 持久化，无需多一个文件 + 多一处同步逻辑。
- **支持非压力通道校零** —— 温度通道（YX-DAQ-T）无需校零；零位校准仅作用于压力设备（XY-DAQ8/DAQ16/SIMULATED）。

## Open Questions

- 校零采样的 1 秒窗口是否需要可配置？**已决策：固定 10 帧，不可配置。**
- "去校零"是清空字段还是标记为"已清除"？**已决策：清空字段（`ZeroOffset = 0` + `ZeroOffsetUnit = ""` + `ZeroCalibratedAt = time.Time{}`）。**
- 校零进行中是否需要禁用 UI？**已决策：不禁用，因采样很快。**
- 单位换算的白名单是否需扩展到 `mmH2O`/`inHg` 等小众单位？**已决策：不扩展。设备协议（[xy_daq16.go L380-403](file:///d:/work/wuli/YX_DAQ/YX-DAQ/src/internal/driver/xy_daq16.go#L380-L403)）硬件支持 9 种，但 UI 与校零白名单进一步裁剪为 6 种（psi / kgf/cm² / bar / kPa / MPa / Pa），删除 mmHg / atm / mbar——实验室不常用且易混淆。驱动层 `unitToCoeff`/`coeffToUnit` 保留 9 种（硬件协议层不动），仅 UI 下拉与校零白名单裁剪。**
