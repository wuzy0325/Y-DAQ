# 三孔移位测试业务逻辑 Review 报告

> 生成时间：2026-04-30  
> 范围：UI → Handler → Service → 插值算法 → CSV → 类型定义 全链路

---

## 一、UI 层 (`ThreeHoleTestView.vue` + `threeHoleTest.ts`)

| # | 逻辑点 | 审视 | 问题 |
|---|--------|------|------|
| 1 | `startTest()` 前置检查：`calibLoaded` + `deviceId` | ✅ 合理 | 但**缺少运动控制器连接检查**——未验证 motionControllerId 对应的控制器是否已连接 |
| 2 | `isRunning`/`isPaused` 纯前端状态，与后端 `taskStatus` 可能不一致 | ⚠️ 隐患 | `stopTest()` 直接置 `isRunning=false`，若后端 Stop 实际失败，前端会显示已停止但后端仍在运行。**缺少 Stop 后的状态回查确认** |
| 3 | 事件监听 `startListening()` 只注册一次，无重连机制 | ⚠️ 风险 | Wails 事件通道若断开（如运行时异常），无重试/重新注册逻辑，UI 将不再收到任何更新 |
| 4 | `complete` 事件处理器调用 `fetchStatus()` 但未 `await` | ⚠️ 小问题 | `fetchStatus()` 是 async 但未 await，结果可能未及时反映 |
| 5 | 实时监控 `startRealtimeMonitor()` 在 `onMounted` 启动，配置变更时重启 | ✅ 合理 | 但**测试运行期间配置变更不会重启监控**（`if (!store.isRunning)` 限制），合理 |
| 6 | `onUnmounted` 调用 `store.stopTest()` | ⚠️ 过激 | 离开页面就停止测试，用户可能只是切换页面查看其他内容。应提示用户确认，或让测试继续在后台运行 |
| 7 | 布点预览 `previewPoints` 中，标记当前点位靠 `dist < 0.01` 距离匹配 | ⚠️ 脆弱 | 浮点距离匹配在步长极小时可能误判；且只匹配第一个，若布点有重复坐标会错标 |
| 8 | 步长输入 `xStep`/`yStep` 最小值 `:min="1"` | ⚠️ 限制过松 | 步长=1 配合大范围(如-100~100)会产生201个点位，X*Y 可能上万个点，无上限提示 |
| 9 | `expandSteps()` 前端实现中 `step = seg.step \|\| 1` | ⚠️ 与后端不一致 | 后端 `expandStepSegments()` 中 step=0 时只取 start/end 两点；前端 step=0 时 fallback 到 1，**前后端点位生成逻辑不一致** |
| 10 | 配置持久化同时写 localStorage 和后端 `SaveThreeHoleConfig` | ⚠️ 双写 | localStorage 与后端配置可能版本不一致，`loadConfig()` 优先从后端加载但 catch 时 fallback 到 localStorage，可能导致旧配置覆盖新配置 |
| 11 | `exportCSV()` 前端实现 | ⚠️ 冗余/不一致 | 后端已有 `ThreeHoleCsvWriter` 在测试过程中实时写 CSV。前端又实现了一套导出，两者格式一致但**后端 CSV 已保存到文件，前端导出是重复功能**，且前端从内存 dataPoints 导出可能数据不全（大量点位时） |

---

## 二、Handler 层 (`handlers_3h.go`)

| # | 逻辑点 | 审视 | 问题 |
|---|--------|------|------|
| 12 | `StartThreeHoleTraversal` 自动启动采集 | ✅ 合理 | 但轮询等待 `for i:=0; i<40; i++` (2秒超时) 硬编码 |
| 13 | `StopThreeHoleTraversal` 后执行急停 | ⚠️ 潜在问题 | 先调用 `threeHoleService.Stop()`，再调 `motionManager.EmergencyStop()`。但 `Stop()` 内部已将 status 设为 Idle 并返回，**急停是"最佳努力"——若运动控制器连接已断开，急停会失败且无重试** |
| 14 | 无运动控制器ID时，遍历所有 profile 急停 | ⚠️ 过于粗暴 | `StopThreeHoleTraversal` 中 `else` 分支对所有已连接运动控制器执行急停，可能影响其他正在使用的控制器 |

---

## 三、Service 层 (`service.go`)

| # | 逻辑点 | 审视 | 问题 |
|---|--------|------|------|
| 15 | `Start()` 加锁检查状态 | ✅ 合理 | 互斥锁 + atomic bool 双重控制，正确 |
| 16 | `testGen` 代际计数器防残留 goroutine | ✅ 优秀设计 | 防止 Stop 超时后旧 goroutine 干扰新测试 |
| 17 | `Pause()` 不阻塞等待当前点位完成 | ⚠️ 行为待确认 | Pause 只设标志位，当前点位（可能在运动中或采集中）会继续完成后才停。这对精密测量合理（保证数据完整性），但UI显示"已暂停"时实际还在执行当前点位，**用户可能困惑** |
| 18 | `Stop()` 不等待 goroutine 退出 | ⚠️ 设计取舍 | 注释说明依赖代际计数器，但 `doneCh` 未被 `Stop()` 使用。若用户快速 Stop→Start，新 Start 可能在旧 goroutine 还没退出时就开始，**此时 config 会被旧 goroutine 的 defer 清除 running 标志影响新 goroutine**（代际计数器可防止，但时序窗口仍存在） |
| 19 | `runSinglePoint` 运动等待超时硬编码 30000ms | ⚠️ 不灵活 | 不同运动机构、不同行程距离需要不同超时，应可配置 |
| 20 | 运动控制：先移动X轴再移动Y轴 | ⚠️ 无并行 | X和Y轴串行移动，对大行程点位会增加耗时。可考虑并行移动（若运动控制器支持） |
| 21 | `dwellWithRealtimeUpdate` 暂停期间延长 deadline | ✅ 合理 | 暂停时间不计入驻留时间，符合测量逻辑 |
| 22 | `acquireAndInterpolate` 采样间隔硬编码 `time.Sleep(50ms)` | ⚠️ 不灵活 | 应与采集周期匹配，且 50ms 间隔可能太短（采集周期可能更长），导致读到重复数据 |
| 23 | `readRawData` 中 batchGetter 失败时静默返回 nil | ⚠️ 隐患 | 采集设备断开时，所有采样都会失败但无错误上报，最终 `no samples collected` 错误信息不明确 |
| 24 | 点位错误 `emitPointError` 不中断测试 | ✅ 合理 | 容错设计，单点失败不影响整体 |
| 25 | `calculateThreeHoleAverage` 简单算术平均 | ✅ 合理 | 但无异常值剔除（如某个采样明显偏离），对噪声大的环境可能不够稳健 |

---

## 四、布点生成 (`service.go:679-781`)

| # | 逻辑点 | 审视 | 问题 |
|---|--------|------|------|
| 26 | `generateLinePoints` 名字叫"直线"但实际生成网格 | ⚠️ 语义误导 | 当 XSteps 和 YSteps 都有值时生成的是 X*Y 的网格点，不是直线上的点。应叫 `GridPoints` 或在文档中说明 |
| 27 | `expandStepSegments` 浮点累加 `v += seg.Step` | ⚠️ 精度风险 | 浮点累加可能导致末尾点多一个或少一个（已有 `+1e-9` 容差，但极端情况仍可能出错）。应改用整数步数计算 |
| 28 | 矩形布点无范围校验 | ⚠️ 缺失 | xMin > xMax 或 step 为负时无检查，会生成空数组或倒序点位 |
| 29 | 无最大点位数限制 | ⚠️ 风险 | 用户配置不当（如 step=0.001 + range=-100~100）可生成数十万点位，导致内存溢出或测试永不结束 |

---

## 五、插值算法 (`interpolator.go`)

| # | 逻辑点 | 审视 | 问题 |
|---|--------|------|------|
| 30 | `deltaP = 2*P2 - P1 - P3` | ✅ 正确 | 标准三孔探针压力差分公式 |
| 31 | ΔP 判零阈值 `1e-6` (Pa) | ⚠️ 阈值过小 | 1e-6 Pa 约 1e-11 atm，远低于任何实际传感器噪声。实际 ΔP=0 的判断阈值应与传感器精度匹配（如 0.1 Pa） |
| 32 | Kb 为 Inf/NaN 时返回 `MachProbe: initMa` | ⚠️ 误导 | 返回 `Valid: false` 但给了马赫数值，调用方可能忽略 Valid 标志而使用此值 |
| 33 | 迭代收敛容差 `1e-4` | ✅ 合理 | 对马赫数迭代，1e-4 精度足够 |
| 34 | 最大迭代 20 次 | ✅ 合理 | 但不收敛时无特殊处理，直接用最后一次值，**应在结果中标记是否收敛** |
| 35 | `interpolate2D` 先马赫数方向插值再 Kb 方向插值 | ✅ 正确 | 标准双线性插值策略 |
| 36 | `findNearestTwoCalib` 超出范围时取最近两个 | ⚠️ 外推变内插 | 超出校准范围时用边界两个点做内插而非外推，意味着边界外结果和边界处相同。**应警告用户马赫数超出校准范围** |
| 37 | Kb 方向插值假设 Kb 单调 | ⚠️ 隐含假设 | `interpolateInKbDirection` 依赖 Kb 升序排列做线性搜索，若校准数据 Kb 不单调（某些马赫数/攻角范围下可能出现），**会跳过区间导致错误结果** |
| 38 | `calculateMachNumber` 使用 `math.Abs(maSq)` | ⚠️ 掩盖问题 | maSq 为负说明总压<静压（物理不合理），取绝对值后得到虚数马赫数的模，**应标记结果异常而非静默处理** |
| 39 | 校准文件要求所有文件攻角序列完全一致 | ⚠️ 限制严格 | 实际校准时不同马赫数下可能使用不同攻角范围，此限制要求统一，不够灵活 |

---

## 六、CSV 写入 (`csv_writer.go`)

| # | 逻辑点 | 审视 | 问题 |
|---|--------|------|------|
| 40 | `Initialize` 中 `os.MkdirAll` 忽略错误 | ⚠️ 风险 | 目录创建失败时后续 `os.Create` 也会失败，但错误信息不够明确 |
| 41 | `AppendPoint` 每次 Flush | ⚠️ 性能 | 每写一个点位都 Flush，对大量点位（如上万点）会有性能损失。可改为批量 Flush 或定时 Flush |
| 42 | 无文件锁定 | ⚠️ 风险 | 若用户同时启动两个测试指向同一文件，会互相覆盖 |

---

## 七、App 初始化 (`app.go`)

| # | 逻辑点 | 审视 | 问题 |
|---|--------|------|------|
| 43 | `SetBatchGetter` 从 AcquisitionHub 快照读取 | ✅ 合理 | 但无数据有效性检查——快照可能是过期的（采集已停止但快照仍在） |
| 44 | 运动控制器回退：先尝试指定ID，失败则遍历所有 | ⚠️ 隐患 | 批量Getter 中无运动控制器ID时不区分设备，可能读到错误设备的数据；运动控制器回退可能操作非预期的控制器 |
| 45 | `shutdown` 中 `threeHoleService.Stop()` | ✅ 合理 | 但未等待测试完成（CSV 可能未完整写入） |

---

## 八、类型定义 (`three_hole_traversal.go`)

| # | 逻辑点 | 审视 | 问题 |
|---|--------|------|------|
| 46 | `ThreeHoleCalibFileInfo` 缺少 `FilePath`/`FileName` 在 `GetCalibInfo()` 中 | ⚠️ 不完整 | `interpolator.GetCalibInfo()` 只填充 `CMa`，不填充 `FilePath`/`FileName`，前端拿不到文件名信息 |
| 47 | `ThreeHoleInterpolationResult.Valid` 但无错误信息字段 | ⚠️ 不足 | Valid=false 时无法知道具体原因（ΔP=0? NaN? 插值失败?），对调试不友好 |

---

## 高优先级问题汇总

| 优先级 | 编号 | 问题 | 影响 |
|--------|------|------|------|
| P0 | #9 | 前后端布点生成逻辑不一致（step=0 行为不同） | 预览与实际测试点位不匹配 |
| P0 | #27 | 浮点累加精度问题 | 可能多/少生成点位 |
| P0 | #29 | 无最大点位数限制 | 可导致内存溢出 |
| P0 | #37 | Kb 非单调时插值错误 | 静默产生错误结果 |
| P1 | #31 | ΔP 判零阈值过小 | 实际中几乎不可能触发，形同虚设 |
| P1 | #22 | 采样间隔硬编码 50ms | 可能与采集周期不匹配，读到重复数据 |
| P1 | #6 | onUnmounted 自动停止测试 | 用户体验问题 |
| P1 | #32 | Valid=false 但返回马赫数值 | 调用方可能误用 |

---

## 涉及文件清单

| 文件 | 角色 |
|------|------|
| `yx-daq/frontend/src/views/ThreeHoleTestView.vue` | UI 视图 |
| `yx-daq/frontend/src/stores/threeHoleTest.ts` | 前端状态管理 |
| `yx-daq/handlers_3h.go` | Wails Handler 层 |
| `yx-daq/internal/three_hole/service.go` | 测试服务核心逻辑 |
| `yx-daq/internal/three_hole/interpolator.go` | 三孔插值算法 |
| `yx-daq/internal/three_hole/csv_writer.go` | CSV 写入 |
| `yx-daq/internal/types/three_hole_traversal.go` | 类型定义 |
| `yx-daq/app.go` | 应用初始化与依赖注入 |
