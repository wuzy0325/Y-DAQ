package types

// XY-DAQ16 常量
const (
	MaxDaqChannels        = 18
	PressureChannelCount  = 16
	StreamFrameHeaderSize = 5
	StreamFrameSize      = 5 + 18*4 // 77 bytes
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
