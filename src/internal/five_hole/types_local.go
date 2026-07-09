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

// FiveHoleProbeAxisPositionGetter 单轴当前位置获取（每轴独立 ControllerID）
// 用于测试开始前保存初始位置、测试结束后回到初始位置
// 控制器未连接或轴不存在时应返回 error
type FiveHoleProbeAxisPositionGetter func(controllerID string, axis types.AxisName) (float64, error)

// FiveHoleAxisKindGetter 单轴类型获取（每轴独立 ControllerID）
// 用于扇面模式启动前校验 MotionX 为线性轴、MotionY 为旋转轴
// 返回 (AxisKind, true) 表示找到对应控制器和轴；返回 (_, false) 表示未找到
type FiveHoleAxisKindGetter func(controllerID string, axis types.AxisName) (types.AxisKind, bool)

// FiveHoleControllerNameGetter 位移机构名获取（按 ControllerID 查询）
// 用于数据点保存时把内部 ID 转为用户可读的机构名
// 未找到时返回空字符串，调用方可回退到 ID
type FiveHoleControllerNameGetter func(controllerID string) string

// FiveHoleEventPublisher 事件发布接口
type FiveHoleEventPublisher interface {
	EmitProgress(event types.FiveHoleTraversalProgressEvent)
	EmitComplete(event types.FiveHoleTraversalCompleteEvent)
	EmitError(event types.FiveHoleTraversalErrorEvent)
	EmitRealtime(event types.FiveHoleTraversalRealtimeEvent)
}

// FiveHoleReturnToInitialDoneCallback 返回初始位置完成回调（测试用）
// 非 nil 时，returnToInitialPositions 完成后调用一次
// 注意：仅可在服务初始化阶段调用，不要在 runTestLoop 运行期间并发修改
type FiveHoleReturnToInitialDoneCallback func()
