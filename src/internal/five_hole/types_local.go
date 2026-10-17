package five_hole

import (
	"errors"

	"yx-daq/internal/types"
)

// ==================== 依赖接口（函数类型） ====================

// FiveHoleMultiDeviceBatchGetter 多设备批量通道数据获取
// 与三孔 ThreeHoleBatchGetter 区别：按 DeviceID 分组读取，支持每通道独立设备
// 返回值：map[通道号]数值, 该设备当前帧序号（单调递增）, error
// 帧序号用于采样时判断是否读到新帧（避免重复采同一帧）；用序号而非墙钟时间戳，
// 是因为 Windows 墙钟约 1ms 一档，同一毫秒内到达的多帧时间戳相同、无法区分。
// channels 为空表示只需帧序号（新鲜度探测），实现方可跳过数值读取。
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

// FiveHoleAxisSpeedGetter 单轴最大速度获取（每轴独立 ControllerID）
// 用于耗时预估：移动耗时 = 距离 / 轴速度。线性轴单位 mm/s，旋转轴单位 °/s
// 返回 (速度, true) 表示找到对应控制器和轴；返回 (_, false) 表示未找到或速度未配置
type FiveHoleAxisSpeedGetter func(controllerID string, axis types.AxisName) (float64, bool)

// FiveHoleDevicePeriodGetter 采集设备出帧周期获取（按 DeviceID 查询，单位 ms）
// 用于耗时预估：采样耗时 = 采样次数 × 最慢设备出帧周期（采样率 = 1000/PeriodMs）
// 返回 0 表示未配置/未知，调用方回退默认周期
type FiveHoleDevicePeriodGetter func(deviceID string) int

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
