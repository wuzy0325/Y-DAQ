package types

// CalibrationType 校准类型
type CalibrationType string

const (
	CalibrationTypeFiveHole CalibrationType = "five-hole"
)

// CalibrationStatus 校准状态
type CalibrationStatus string

const (
	CalibStatusIdle         CalibrationStatus = "idle"
	CalibStatusConfiguring  CalibrationStatus = "configuring"
	CalibStatusRunning      CalibrationStatus = "running"
	CalibStatusPaused       CalibrationStatus = "paused"
	CalibStatusCompleted    CalibrationStatus = "completed"
	CalibStatusError        CalibrationStatus = "error"
)

// ProbeChannelRole 探针通道语义角色
type ProbeChannelRole string

const (
	RoleP1      ProbeChannelRole = "fiveHole.p1"
	RoleP2      ProbeChannelRole = "fiveHole.p2"
	RoleP3      ProbeChannelRole = "fiveHole.p3"
	RoleP4      ProbeChannelRole = "fiveHole.p4"
	RoleP5      ProbeChannelRole = "fiveHole.p5"
	RolePAtm    ProbeChannelRole = "fiveHole.pAtm"
	RoleTAtm    ProbeChannelRole = "fiveHole.tAtm"
	RolePTotal  ProbeChannelRole = "fiveHole.pTotal"
)

// ProbeChannelConfig 探针通道配置
type ProbeChannelConfig struct {
	Name    string            `json:"name"`
	Role    ProbeChannelRole  `json:"role"`
	Channel int               `json:"channel"`
	Enabled bool              `json:"enabled"`
}

// CalibrationPoint 校准点
type CalibrationPoint struct {
	ID          string  `json:"id"`
	Alpha       float64 `json:"alpha"`
	Beta        float64 `json:"beta"`
}

// SphereTankGateConfig 球罐门控配置
type SphereTankGateConfig struct {
	Enabled       bool    `json:"enabled"`
	ChannelIndex  int     `json:"channelIndex"`
	ThresholdRate float64 `json:"thresholdRate"`
	StableTimeMs  int     `json:"stableTimeMs"`
}

// CalibrationConfig 校准配置
type CalibrationConfig struct {
	Type            CalibrationType      `json:"type"`
	DeviceID        string               `json:"deviceId"`
	ControllerID    string               `json:"controllerId"`
	ProbeChannels   []ProbeChannelConfig `json:"probeChannels"`
	AlphaAxis       AxisName             `json:"alphaAxis"`
	BetaAxis        AxisName             `json:"betaAxis"`
	Points          []CalibrationPoint   `json:"points"`
	DwellTimeMs     int                  `json:"dwellTimeMs"`
	SamplesPerPoint int                  `json:"samplesPerPoint"`
	SphereTankGate  SphereTankGateConfig `json:"sphereTankGate"`
}

// FiveHoleRawData 五孔原始数据
type FiveHoleRawData struct {
	P1      float64  `json:"p1"`
	P2      float64  `json:"p2"`
	P3      float64  `json:"p3"`
	P4      float64  `json:"p4"`
	P5      float64  `json:"p5"`
	PAtm    float64  `json:"pAtm"`
	TAtm    float64  `json:"tAtm"`
	PTotal  *float64 `json:"pTotal,omitempty"`
}

// FiveHoleCoefficients 五孔系数
type FiveHoleCoefficients struct {
	Kalpha float64 `json:"Kalpha"`
	Kbeta  float64 `json:"Kbeta"`
	CPT    float64 `json:"CPT"`
	CPS    float64 `json:"CPS"`
}

// CalibrationDataPoint 校准数据点
type CalibrationDataPoint struct {
	PointID     string              `json:"pointId"`
	Alpha       float64             `json:"alpha"`
	Beta        float64             `json:"beta"`
	RawData     FiveHoleRawData     `json:"rawData"`
	Coefficients FiveHoleCoefficients `json:"coefficients"`
	SampleCount int                 `json:"sampleCount"`
	StdDev      float64             `json:"stdDev"`
}

// CalibrationTaskStatus 校准任务状态
type CalibrationTaskStatus struct {
	TaskID          string              `json:"taskId"`
	Status          CalibrationStatus   `json:"status"`
	TotalPoints     int                 `json:"totalPoints"`
	CompletedPoints int                 `json:"completedPoints"`
	Progress        float64             `json:"progress"`
	CurrentPoint    *CalibrationPoint   `json:"currentPoint,omitempty"`
	DataPoints      []CalibrationDataPoint `json:"dataPoints"`
	LastError       string              `json:"lastError,omitempty"`
}

// CalibrationProgressEvent 校准进度事件
type CalibrationProgressEvent struct {
	TaskID          string  `json:"taskId"`
	TotalPoints     int     `json:"totalPoints"`
	CompletedPoints int     `json:"completedPoints"`
	Progress        float64 `json:"progress"`
	CurrentAlpha    float64 `json:"currentAlpha"`
	CurrentBeta     float64 `json:"currentBeta"`
}

// CalibrationRealtimeEvent 校准实时数据事件
type CalibrationRealtimeEvent struct {
	TaskID       string               `json:"taskId"`
	PointID      string               `json:"pointId"`
	RawData      FiveHoleRawData      `json:"rawData"`
	Coefficients FiveHoleCoefficients  `json:"coefficients"`
}

// CalibrationCompleteEvent 校准完成事件
type CalibrationCompleteEvent struct {
	TaskID     string                 `json:"taskId"`
	Status     CalibrationStatus      `json:"status"`
	DataPoints []CalibrationDataPoint `json:"dataPoints"`
}
