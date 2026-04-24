package types

// AxisName 逻辑轴名
type AxisName string

const (
	AxisX AxisName = "X"
	AxisY AxisName = "Y"
	AxisZ AxisName = "Z"
	AxisU AxisName = "U"
)

// AxisKind 轴类型
type AxisKind string

const (
	AxisKindLinear  AxisKind = "LINEAR"
	AxisKindRotary  AxisKind = "ROTARY"
)

// MotionControllerType 运动控制器类型
type MotionControllerType string

const (
	MotionTypeSimulated MotionControllerType = "SIMULATED-MC"
	MotionTypeB140      MotionControllerType = "B140-MC"
)

// EncoderCompensationConfig 编码器补偿配置
type EncoderCompensationConfig struct {
	Enabled   bool    `json:"enabled"`
	Tolerance float64 `json:"tolerance"`
	MaxCycles int     `json:"maxCycles"`
	SettleMs  int     `json:"settleMs"`
	MinStep   float64 `json:"minStep"`
	TimeoutMs int     `json:"timeoutMs"`
}

// AxisConfig 轴配置
type AxisConfig struct {
	Name                 AxisName                 `json:"name"`
	Enabled              bool                     `json:"enabled"`
	Kind                 AxisKind                 `json:"kind"`
	Inverted             bool                     `json:"inverted"`
	StepAngleDeg         float64                  `json:"stepAngleDeg"`
	MicroSteps           int                      `json:"microSteps"`
	Lead                 float64                  `json:"lead"`
	MaxSpeed             float64                  `json:"maxSpeed"`
	EncoderScale         float64                  `json:"encoderScale"`
	EncoderCompensation  EncoderCompensationConfig `json:"encoderCompensation"`
}

// MotionControllerProfile 运动控制器配置
type MotionControllerProfile struct {
	ID      string                   `json:"id"`
	Name    string                   `json:"name"`
	Type    MotionControllerType     `json:"type"`
	Address string                   `json:"address"`
	Port    int                      `json:"port"`
	TimeoutMs int                    `json:"timeoutMs"`
	Axes    []AxisConfig             `json:"axes"`
}

// AxisStatus 轴运行时状态
type AxisStatus struct {
	Name          AxisName `json:"name"`
	Position      float64  `json:"position"`
	Moving        bool     `json:"moving"`
	Homed         bool     `json:"homed"`
	PosLimit      bool     `json:"posLimit"`
	NegLimit      bool     `json:"negLimit"`
	Compensating  bool     `json:"compensating"`
}

// MotionControllerStatus 运动控制器状态
type MotionControllerStatus struct {
	ID         string           `json:"id"`
	Name       string           `json:"name"`
	Type       MotionControllerType `json:"type"`
	Status     ConnectionStatus `json:"status"`
	Axes       []AxisStatus     `json:"axes"`
	LastError  string           `json:"lastError"`
}

// LimitStatus 限位状态
type LimitStatus struct {
	PosLimit bool `json:"posLimit"`
	NegLimit bool `json:"negLimit"`
}
