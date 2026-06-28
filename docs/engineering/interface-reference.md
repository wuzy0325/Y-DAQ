# 核心接口参考

> 本文档列出系统的**扩展点接口**——当你需要接入新设备、新运动控制器或新事件通道时，必须实现或注入的契约。所有接口的依赖注入汇聚点在 `internal/app/core.go`。

---

## 1. DeviceDriver — 设备驱动接口

```go
// 文件: internal/manager/device_manager.go:14
type DeviceDriver interface {
    Connect() error
    Disconnect()
    IsConnected() bool
    IsAcquiring() bool
    StartAcquisition(periodMs int) error
    StopAcquisition() error
    SetDataCallback(cb types.DataCallback)
    UpdateChannels(channels []types.ChannelConfig)
}
```

**用途**：统一封装不同品牌采集卡的硬件通信协议，使 `DeviceManager` 不依赖具体硬件。

**现有实现**：

| 实现 | 文件 | 说明 |
|------|------|------|
| `XYDAQDriver` | `internal/driver/xy_daq16.go:17` | TCP 协议采集卡 |
| `SimulatedDevice` | `internal/driver/simulated_device.go:13` | 模拟器，无硬件依赖 |

**接入新设备**：在 `internal/driver/` 下新建文件，实现 `DeviceDriver` 全部 8 个方法，然后在 `DeviceManager.Connect()`（`device_manager.go:131`）的 `switch` 中添加新分支。

**可选扩展**：如果设备支持设置单位，额外实现 `UnitSetter` 接口（见下）。

---

## 2. UnitSetter — 单位设置接口

```go
// 文件: internal/manager/device_manager.go:289
// 注释: UnitSetter 单位设置接口（仅 XY-DAQ 驱动实现）
type UnitSetter interface {
    SetUnit(unit string) error
}
```

**用途**：可选接口，允许 `DeviceManager.SetUnit()` 通过类型断言动态调用。调用方判断方式：

```go
setter, ok := drv.(UnitSetter)
if !ok {
    return fmt.Errorf("device does not support SetUnit: %s", id)
}
```

**现有实现**：仅 `XYDAQDriver` 实现，`SimulatedDevice` 不实现。

---

## 3. MotionController — 运动控制器接口

```go
// 文件: internal/manager/motion_manager.go:15
type MotionController interface {
    Connect() error
    Disconnect()
    IsConnected() bool
    MoveTo(axis types.AxisName, position float64) error
    MoveBy(axis types.AxisName, delta float64) error
    Jog(axis types.AxisName, direction int, speed float64) error
    Home(axis types.AxisName) error
    Stop(axis types.AxisName) error
    StopAll() error
    EmergencyStop() error
    DefinePosition(axis types.AxisName, position float64) error
    GetAxisStatus(axis types.AxisName) (types.AxisStatus, error)
    GetAllAxisStatus() ([]types.AxisStatus, error)
    SetSpeed(axis types.AxisName, speed float64) error
    SetAcceleration(axis types.AxisName, accel float64) error
    SetDeceleration(axis types.AxisName, decel float64) error
    IsMoving() (bool, error)
    IsAxisMoving(axis types.AxisName) (bool, error)
    GetLimitStatus(axis types.AxisName) (types.LimitStatus, error)
    WaitForMotionComplete(axis types.AxisName, timeoutMs int) error
    MotorOff() error
    SetAxisDirection(axis types.AxisName, reverse bool) error
}
```

**用途**：统一封装运动控制器（如 Galil DMC 系列）的指令协议，使 `MotionControllerManager` 不依赖具体硬件。

**现有实现**：

| 实现 | 文件 | 说明 |
|------|------|------|
| `B140MotionController` | `internal/driver/b140.go:107` | Galil DMC-B140-M，22 方法全实现 |
| `SimulatedMotionController` | `internal/driver/simulated_motion.go:12` | 模拟器，22 方法全实现 |

**接入新控制器**：在 `internal/driver/` 下新建文件实现全部 22 个方法，然后在 `MotionControllerManager.Connect()`（`motion_manager.go:99`）的 `switch` 中添加新分支。

---

## 4. EventPublisher（五孔探针） — 事件发布接口

```go
// 文件: internal/calibration/service.go:24
type EventPublisher interface {
    EmitProgress(event types.CalibrationProgressEvent)
    EmitRealtime(event types.CalibrationRealtimeEvent)
    EmitComplete(event types.CalibrationCompleteEvent)
}
```

**用途**：将 `CalibrationService` 的进度、实时数据和完成事件发布到前端。解耦业务逻辑与 Wails 运行时。

**现有实现**：

| 实现 | 文件 | 说明 |
|------|------|------|
| `CalibrationEventPublisher` | `internal/app/event_publishers.go:10` | 包装 `app.Event.Emit()`，发射到通道 `calibration:progress/realtime/complete` |
| `mockEventPublisher`（测试用） | `internal/calibration/service_test.go:11` | 将事件追加到切片供断言验证 |

**注入方式**：在 `core.go:119` 通过 `NewCalibrationService(&CalibrationEventPublisher{app: c.App})` 作为构造函数参数传入。

---

## 5. ThreeHoleEventPublisher（三孔探针） — 事件发布接口

```go
// 文件: internal/three_hole/service.go:23
type ThreeHoleEventPublisher interface {
    EmitProgress(event types.ThreeHoleTraversalProgressEvent)
    EmitRealtime(event types.ThreeHoleTraversalRealtimeEvent)
    EmitComplete(event types.ThreeHoleTraversalCompleteEvent)
    EmitError(event types.ThreeHoleTraversalErrorEvent)
}
```

**用途**：同 `EventPublisher`，专用于三孔探针移位布点测试场景。

**现有实现**：

| 实现 | 文件 | 说明 |
|------|------|------|
| `ThreeHoleEventPublisher` | `internal/app/event_publishers.go:27` | 包装 `app.Event.Emit()`，发射到 `three-hole:<probeID>:progress/realtime/complete/error` |
| `MockEventPublisher`（测试用） | `internal/three_hole/test_helpers.go:8` | 将事件追加到切片供断言验证 |

**注入方式**：在 `core.go:155` 通过 `NewThreeHoleTraversalService(&ThreeHoleEventPublisher{app: c.App, probeID: probeID})` 作为构造函数参数传入。该 publisher 进一步注入到 `TestManager` 和 `DataProcessor`。

---

## 6. ChannelBatchGetter（五孔探针） — 批量采数函数类型

```go
// 文件: internal/calibration/service.go:18
type ChannelBatchGetter func(channels []types.ProbeChannelConfig) (map[int]float64, error)
```

**用途**：从全局数据总线（`AcquisitionHub`）一次拉取多个通道的当前值，供五孔探针系数计算使用。

**实现**：在 `core.go:123` 以闭包形式注入，内部调用 `AcquisitionHub.GetLatestValue()`。

**使用链路**：`runCalibrationLoop()` → `acquireData()` → `readRawData()` → `batchGetter(channels)`

---

## 7. DataGetter（五孔探针） — 单通道采数函数类型

```go
// 文件: internal/calibration/service.go:15
type DataGetter func(deviceID string, channelIndex int) (float64, bool)
```

> **当前状态：死代码**。注入点在 `core.go:120`，但 `CalibrationService` 没有任何代码调用 `s.dataGetter(...)`，实际采数走的是 `batchGetter`。保留仅作兼容，后续应清理。

---

## 8. ThreeHoleMotionController（三孔探针） — 运动控制函数类型

```go
// 文件: internal/three_hole/service.go:17
type ThreeHoleMotionController func(axis types.AxisName, position float64) error
```

**用途**：三孔探针测试中移动平台到指定坐标。包装了 `MotionControllerManager.MoveTo()`，与 `MotionController` 接口的关系是"调用方视角的窄接口"——测试不需要知道全部 22 个运动方法。

**实现**：在 `core.go:195` 以闭包形式注入，遍历已连接的运动控制器执行 `MoveTo`。

---

## 9. calibration.MotionController（五孔探针） — 运动控制函数类型

```go
// 文件: internal/calibration/service.go:21
type MotionController func(axis types.AxisName, position float64) error
```

**用途**：五孔探针球形水箱标定中移动探针角度。同三孔版本一样是窄接口包装。

**实现**：在 `core.go:136` 以闭包形式注入。

---

## 依赖注入总览

所有接口的实现选择都在 `internal/app/core.go` 中完成，`Core` 结构体扮演依赖注入容器角色：

```
Core.Init()
├── NewCalibrationService(&CalibrationEventPublisher{app})  // EventPublisher
│   ├── .SetDataGetter(fn)                                  // DataGetter → DeviceManager.GetChannelValue
│   ├── .SetBatchGetter(fn)                                 // ChannelBatchGetter → AcquisitionHub.GetLatestValue
│   └── .SetMotionController(fn)                            // MotionController → MotionManager.MoveTo
│
├── NewThreeHoleTraversalService(&ThreeHoleEventPublisher{app})  // ThreeHoleEventPublisher
│   └── .SetMotionController(fn)                            // ThreeHoleMotionController → MotionManager.MoveTo
│
├── DeviceManager
│   └── .Connect() → switch type → DeviceDriver             // XYDAQDriver / SimulatedDevice
│
└── MotionControllerManager
    └── .Connect() → switch type → MotionController          // B140MotionController / SimulatedMotionController
```
