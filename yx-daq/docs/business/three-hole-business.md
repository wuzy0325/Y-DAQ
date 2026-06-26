# 三孔移位插值测试 — 业务逻辑

## 一、业务目的

三孔移位插值测试的目的是在**风洞/流场**中对三孔探针进行 **机械移位布点**，在每个布点位置采集 P1/P2/P3 压力数据，结合**预标定校准文件**通过插值算法反算出该点的**总压、静压、马赫数、攻角**，从而对流场进行空间扫描测量。

核心流程：**布点规划 → 运动定位 → 驻留稳定 → 数据采集 → 插值计算 → 结果记录 → 自动遍历下一位置**

---

## 二、数据流

```
用户配置
  │
  ├── 布点参数 (直线/矩形/自定义)
  ├── 采集设备 + 通道映射
  ├── 运动控制器 + 轴映射
  ├── 校准文件（.txt，含多个马赫数的标定表）
  └── 采样参数 (驻留时间、每点采样数)
  │
  ▼
三孔移位测试服务 (ThreeHoleTraversalService)
  │
  ├── 1. 校验校准文件是否已加载
  ├── 2. 根据布点参数生成所有测试点位 (generatePoints)
  ├── 3. 遍历每个点位:
  │        ├── 移动运动控制器到 (X, Y)
  │        ├── 驻留等待 (dwellWithRealtimeUpdate)
  │        ├── 多次采样 (acquireAndInterpolate)
  │        └── 计算平均值 → 插值计算 → 写入CSV
  │
  └── 4. 完成/错误 → 发送完成事件
  │
  ▼
事件推送到前端 → 实时更新UI / 进度条 / 结果表格 / ECharts
CSV文件写入磁盘（UTF-8 BOM）
```

---

## 三、布点模式（点位生成）

三种布点模式，用户界面配置，后台统一生成 `TraversalPoint` 列表。

### 3.1 直线布点 (Line)

```
Start (X₁, Y₁) ── 分段步长间隔 ──→ End (X₂, Y₂)
```

- 通过 `XSteps` / `YSteps` 定义多段 `StepSegment{Start, End, Step}`
- `expandStepSegments` 展开每段为数列
- X × Y 两轴值**笛卡尔积**生成所有点位
- 如果没有分段步长，只取起止两点

### 3.2 矩形布点 (Rectangle)

```
Ymax ──────────────────────
  │  ·  ·  ·  ·  ·  ·  ·  │
  │  ·  ·  ·  ·  ·  ·  ·  │
  │  ·  ·  ·  ·  ·  ·  ·  │
Ymin ──────────────────────
     Xmin                 Xmax
```

- 边界 `[XMin, XMax] × [YMin, YMax]`
- 通过 `XSteps` / `YSteps` 定义 X 和 Y 方向的分段步长
- 同样笛卡尔积生成网格点
- 如果没有分段步长，只取四个角点

### 3.3 自定义布点 (Custom)

- 用户直接提供一个 `TraversalPoint` 列表
- 不做任何展开，直接使用

---

## 四、单点测试生命周期

对每个点位，依次经历三个阶段：

### 阶段1：移动 (moving)

```
X_target = point.X * X_scale + X_offset
Y_target = point.Y * Y_scale + Y_offset

motionCtrl(X_axis, X_target)
motionCtrl(Y_axis, Y_target)
```

- 通过 `MotionAxisMapping{Scale, Offset}` 将坐标值转换为运动控制器的实际位置
- 先后移动 X 轴和 Y 轴（当前是串行移动）

### 阶段2：驻留 (waiting)

- 等待系统稳定，时长为 `DwellTimeMs`
- 驻留期间每 **100ms** 推送一次实时数据到前端（`three-hole:realtime`），保持 UI 不卡顿
- 支持暂停/恢复：暂停期间 deadling 自动顺延

### 阶段3：采集 + 插值 (acquiring)

```
for i = 0; i < SamplesPerPoint; i++ {
    rawData = readRawData()              // 从采集设备读 P1,P2,P3,PAtm,TAtm
    interpResult = interpolator.Calculate(rawData)  // 实时插值
    emitRealtime(raw, interpResult)      // 推送到UI
    sleep(50ms)
}

avgData = calculateAverage(allSamples)   // 多次采样的平均值
finalResult = interpolator.Calculate(avgData)  // 对平均值做最终插值

appendResultToCSV(点号, X, Y, avgData, finalResult)
```

---

## 五、插值算法（核心）

### 5.1 原理概述

三孔探针通过测量三个孔的压力（P1、P2、P3），结合预先标定的校准数据，反推流场参数。

校准数据来自**风洞吹风标定**，即在已知马赫数（CMa）和攻角（Alpha）条件下，记录压力系数 Kb、Kt、Sb。标定时在每个马赫数下扫掠多个攻角，形成一张标定表。

> 实际运行时，ΔP = 2·P₂ − P₁ − P₃ 反映了攻角大小；Kb = (P₃ − P₁) / ΔP 唯一对应某个攻角。但 Kb 与攻角的对应关系**依赖于马赫数**，因此需要**二维插值**（马赫数方向 × Kb 方向）。

### 5.2 校准文件格式

```
0.3              ← CMa（校准马赫数）
12               ← Nalpha（攻角条数）
-1.2000  0.9800  0.1000  -20.0    ← Kb  Kt  Sb  Alpha
-0.8000  0.9700  0.1200  -15.0
...
0.0000  0.9500  0.1500  0.0
...
1.2000  0.9800  0.1000  20.0
```

每个文件对应一个CMa，文件内每行对应一个攻角。所有文件必须使用**相同的攻角序列**。

### 5.3 加载校验

```go
func (interp *ThreeHoleInterpolator) LoadCalibFiles(filePaths []string) error
```

1. 解析每个文件 → `ThreeHoleCalibData{CMa, Entries[]}`
2. 按 CMa 升序排序
3. **校验所有文件攻角序列一致**（长度相同且 Alpha 值逐个匹配，`tolerance=1e-6`）
4. 计算 `initMa`（所有 CMa 的平均值，作为迭代初值）
5. 记录 `minMa` / `maxMa`（马赫数范围）

### 5.4 计算结果类型 `ThreeHoleInterpolationResult`

| 字段 | 含义 |
|------|------|
| `PtProbe` | 探针计算总压（表压 Pa） |
| `PsProbe` | 探针计算静压（表压 Pa） |
| `MachProbe` | 计算马赫数 |
| `AlphaProbe` | 计算攻角（度） |
| `IterationCount` | 迭代收敛次数 |
| `Valid` | 结果是否有效 |

### 5.5 计算步骤

```
输入: P1, P2, P3, PAtm
输出: Pt, Ps, Ma, Alpha

步骤1: 计算ΔP和Kb_temp
    ΔP = 2·P₂ − P₁ − P₃
    Kb_temp = (P₃ − P₁) / ΔP
    
步骤2: 迭代求解（最多20次）
    for i = 0..maxIterations:
        a) 二维插值: (Kb_temp, Ma_current) → (Kt, Sb)  // 查标定表
        b) 计算总压: Pt = P₂ + Kt · ΔP
        c) 计算静压: Ps = Pt − Sb · ΔP
        d) 计算马赫数: Ma_new = sqrt(5 · |(Pt+Pa)/(Ps+Pa)^0.2857 − 1|)
        e) 收敛判定: |Ma_new − Ma_current| < 1e-4 → break
        f) 更新: Ma_current = clamp(Ma_new, minMa, maxMa)
    
步骤3: 用最终Ma做一次最终插值 → (Alpha_final, Kt_final, Sb_final)
    Pt = P₂ + Kt_final · ΔP
    Ps = Pt − Sb_final · ΔP
    Ma = calcMach(Pt, Ps, Pa)
```

### 5.6 二维插值算法 `interpolate2D`

```
输入: Kb_temp (实测值), Ma_current (当前迭代值)
输出: Alpha, Kt, Sb

1. findNearestTwoCalib(Ma_current)
   → 在calibData中找到包围Ma_current的两个相邻校准马赫数
   → 超范围时取最近的两个

2. interpolateInMaDirection(calib1, calib2, Ma_current)
   → 在两个CMa之间线性插值，构建当前Ma下的系数表
   → 输出: []kbAlphaEntry{Kb, Alpha, Kt, Sb}
   
3. 按Kb升序排序

4. interpolateInKbDirection(entries, Kb_temp)
   → 在Kb方向上线性插值，反查Alpha/Kt/Sb
   → 边界使用最近值（不外推）
```

### 5.7 马赫数计算公式

```
Ma = sqrt(5 · |(Pt + Pa) / (Ps + Pa)^0.2857 − 1|)
```
其中 0.2857 = (γ−1)/γ，γ = 1.4（空气比热比）。

### 5.8 异常/边界处理

| 条件 | 行为 |
|------|------|
| ΔP ≈ 0（攻角接近零） | 标记 `Valid=false`，Pt=Ps=P₂ Ma=0 Alpha=0 |
| Kb_temp 为 Inf/NaN | 标记 `Valid=false`，回退初值 |
| 马赫数超校准范围 | 截断到 `[minMa, maxMa]` |
| Kb 超标定表范围 | 用最近边界值（不外推） |
| 校准文件未加载 | 返回 `Valid=false` |

---

## 六、暂停/恢复/取消

| 操作 | 行为 |
|------|------|
| **暂停** | 设置 `paused=true`，主循环进入 pause loop |
| **恢复** | 设置 `paused=false`，通过 `resumeCh` 唤醒 |
| **取消** | 通过 `cancelCh` 通知，goroutine 内 `select` 检查后 return |
| **暂停中继续推数据** | pause loop 内每 100ms 采集一次原始数据并推送实时事件 |

暂停对驻留时间的影响：暂停期间 deadling 自动顺延，保证驻留总时长正确。

---

## 七、实时监控模式

测试未运行时，如果采集设备在运行，可启动**独立实时监控**：

```
StartRealtimeMonitor() → goroutine
  每100ms:
    rawData = readRawData()
    interpResult = interpolator.Calculate(rawData)  // 如果校准文件已加载
    emitRealtime()
```

独立于测试循环，使用 `monitorRunning` / `monitorCancel` 分别控制。

---

## 八、数据输出

### 8.1 CSV 格式（UTF-8 BOM）

| 列 | 内容 |
|----|------|
| 点号 | pt-0, pt-1, ... |
| X | 布点X坐标 |
| Y | 布点Y坐标 |
| P1 | 3号孔压力平均值 |
| P2 | 2号孔(中心)压力平均值 |
| P3 | 1号孔压力平均值 |
| P∞ | 大气压平均值 |
| T∞ | 大气温度平均值 |
| 总压Pt | 插值计算总压 |
| 静压Ps | 插值计算静压 |
| 马赫数Ma | 插值马赫数 |
| 攻角Alpha | 插值攻角（度） |
| 迭代次数 | 插值收敛迭代次数 |
| 采样数 | 该点有效采样数 |
| 时间戳 | unix ms |

### 8.2 前端事件

| 事件名 | 触发时机 | 内容 |
|--------|---------|------|
| `three-hole:progress` | 每个点位完成 | 进度、已完成数、总数、当前X/Y |
| `three-hole:realtime` | 驻留/采集中每100ms | 当前原始数据 + 插值结果 |
| `three-hole:complete` | 全部完成或错误终止 | 完整结果列表 |
| `three-hole:error` | 点位错误或致命错误 | 错误消息 |

---

## 九、配置完整性

```
ThreeHoleTraversalConfig:
  ├── Name               — 测试名称
  ├── DeviceID           — 关联的采集设备ID
  ├── MotionControllerID — 关联的运动控制器ID
  ├── Layout             — 布点配置
  │     ├── Pattern      — line | rectangle | custom
  │     ├── Line         — 直线参数 (StartX/Y, EndX/Y, XSteps, YSteps)
  │     ├── Rectangle    — 矩形参数 (XMin/XMax, YMin/YMax, XSteps, YSteps)
  │     └── CustomPoints — 自定义点位列表
  ├── ProbeChannels[]    — 三孔通道映射 (P1, P2, P3, PAtm, TAtm)
  ├── MotionX            — X轴映射 (Axis + Scale + Offset)
  ├── MotionY            — Y轴映射
  ├── CalibFiles[]       — 校准文件列表（含CMa信息）
  ├── DwellTimeMs        — 每点驻留时间 (ms)
  ├── SamplesPerPoint    — 每点采样次数
  ├── SavePath           — CSV 输出目录
  └── SaveFileName       — CSV 文件名
```
