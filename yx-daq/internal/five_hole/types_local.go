package five_hole

import (
	"errors"

	"yx-daq/internal/types"
)

// ==================== 依赖接口（函数类型） ====================

// FiveHoleMultiDeviceBatchGetter 多设备批量通道数据获取
// 与三孔 ThreeHoleBatchGetter 区别：按 DeviceID 分组读取，支持每通道独立设备
// 返回值：map[通道号]数值, 该设备最新数据的 UnixMilli 时间戳, error
// timestamp 用于采样时判断数据新鲜度，避免读到重复帧
type FiveHoleMultiDeviceBatchGetter func(deviceID string, channels []int) (map[int]float64, int64, error)

// ErrDataStagnant 设备数据停滞错误
// 采样时等待新帧超时（设备停采或频率过低），需暂停测试等待用户恢复
var ErrDataStagnant = errors.New("采集设备数据停滞，请检查设备后恢复测试")

// FiveHoleProbeAxisMover 单轴运动控制（每轴独立 ControllerID）
type FiveHoleProbeAxisMover func(controllerID string, axis types.AxisName, position float64) error

// FiveHoleProbeAxisWaiter 单轴运动等待（每轴独立 ControllerID）
type FiveHoleProbeAxisWaiter func(controllerID string, axis types.AxisName, timeoutMs int) error

// FiveHoleEventPublisher 事件发布接口
type FiveHoleEventPublisher interface {
	EmitProgress(event types.FiveHoleTraversalProgressEvent)
	EmitRealtime(event types.FiveHoleTraversalRealtimeEvent)
	EmitComplete(event types.FiveHoleTraversalCompleteEvent)
	EmitError(event types.FiveHoleTraversalErrorEvent)
}
