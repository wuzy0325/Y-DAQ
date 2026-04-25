package types

// 数据帧常量
const (
	StreamFrameHeaderSize = 5

	// 以下为 DAQ16 兼容常量，新代码应使用 DeviceType 的方法获取通道规格
	MaxDaqChannels       = 18 // DAQ16: 16压力 + 大气压 + 大气温度
	PressureChannelCount = 16 // DAQ16 压力通道数
	StreamFrameSize      = 5 + 18*4 // DAQ16 帧大小: 77 bytes
)

// 数据发布常量
const (
	SnapshotPublishHz    = 20
	SnapshotPublishMs    = 1000 / SnapshotPublishHz
)

// 运动控制常量
const (
	MotionPollHz        = 10
	MotionPollIntervalMs = 100
)

// 重连常量
const (
	MaxReconnectAttempts    = 5
	ReconnectBaseDelayMs   = 1000
	ReconnectMaxDelayMs    = 30000
)

// 命令超时
const (
	CommandTimeoutMs = 2000
)

// 编码器补偿默认值
const (
	DefaultEncoderCompensationTolerance  = 0.01
	DefaultEncoderCompensationMaxCycles  = 3
	DefaultEncoderScale                  = 0.005
	DefaultEncoderCompensationSettleMs   = 100
	DefaultEncoderCompensationTimeoutMs  = 5000
)

// B140 轴映射
var AxisNameToB140 = map[AxisName]string{
	AxisX: "A",
	AxisY: "B",
	AxisZ: "C",
	AxisU: "D",
}
