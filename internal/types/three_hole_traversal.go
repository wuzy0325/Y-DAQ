package types

// ==================== 三孔探针通道角色 ====================

// ThreeHoleChannelRole 三孔探针通道语义角色
type ThreeHoleChannelRole string

const (
	Role3H_P1   ThreeHoleChannelRole = "threeHole.p1"   // 1号孔压力
	Role3H_P2   ThreeHoleChannelRole = "threeHole.p2"   // 2号孔压力（中心孔）
	Role3H_P3   ThreeHoleChannelRole = "threeHole.p3"   // 3号孔压力
	Role3H_PAtm ThreeHoleChannelRole = "threeHole.pAtm" // 大气压
	Role3H_TAtm ThreeHoleChannelRole = "threeHole.tAtm" // 大气温度
)

// ThreeHoleProbeChannelConfig 三孔探针通道配置
type ThreeHoleProbeChannelConfig struct {
	Name    string               `json:"name"`
	Role    ThreeHoleChannelRole `json:"role"`
	Channel int                  `json:"channel"`
	Enabled bool                 `json:"enabled"`
}

// ==================== 布点模式 ====================

// TraversalPattern 布点模式
type TraversalPattern string

const (
	TraversalPatternLine      TraversalPattern = "line"
	TraversalPatternRectangle TraversalPattern = "rectangle"
	TraversalPatternCustom    TraversalPattern = "custom"
)

// StepSegment 分段步长区段
type StepSegment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Step  float64 `json:"step"`
}

// LineLayout 直线布点配置
type LineLayout struct {
	StartX float64       `json:"startX"`
	StartY float64       `json:"startY"`
	EndX   float64       `json:"endX"`
	EndY   float64       `json:"endY"`
	XSteps []StepSegment `json:"xSteps"`
	YSteps []StepSegment `json:"ySteps"`
}

// RectangleLayout 矩形布点配置
type RectangleLayout struct {
	XMin   float64       `json:"xMin"`
	XMax   float64       `json:"xMax"`
	YMin   float64       `json:"yMin"`
	YMax   float64       `json:"yMax"`
	XSteps []StepSegment `json:"xSteps"`
	YSteps []StepSegment `json:"ySteps"`
}

// TraversalLayout 布点配置
type TraversalLayout struct {
	Pattern      TraversalPattern `json:"pattern"`
	Line         *LineLayout      `json:"line,omitempty"`
	Rectangle    *RectangleLayout `json:"rectangle,omitempty"`
	CustomPoints []TraversalPoint `json:"customPoints,omitempty"`
}

// TraversalPoint 测试点位
type TraversalPoint struct {
	ID string  `json:"id"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
}

// ==================== 运动轴映射 ====================

// MotionAxisMapping 运动轴映射
type MotionAxisMapping struct {
	Axis AxisName `json:"axis"`
}

// ==================== 校准文件 ====================

// ThreeHoleCalibFileInfo 三孔校准文件信息
type ThreeHoleCalibFileInfo struct {
	FilePath string  `json:"filePath"`
	FileName string  `json:"fileName"`
	CMa      float64 `json:"cMa"` // 校准马赫数
}

// ==================== 测试配置 ====================

// ThreeHoleTraversalConfig 三孔移位测试配置
type ThreeHoleTraversalConfig struct {
	Name               string                        `json:"name"`
	DeviceID           string                        `json:"deviceId"`           // 采集设备ID
	MotionControllerID string                        `json:"motionControllerId"` // 运动控制器ID
	Layout             TraversalLayout               `json:"layout"`
	ProbeChannels      []ThreeHoleProbeChannelConfig `json:"probeChannels"`
	MotionAlpha        MotionAxisMapping             `json:"motionAlpha"`
	MotionBeta         MotionAxisMapping             `json:"motionBeta"`
	CalibFiles         []ThreeHoleCalibFileInfo      `json:"calibFiles"`
	DwellTimeMs        int                           `json:"dwellTimeMs"`
	SamplesPerPoint    int                           `json:"samplesPerPoint"`
	SampleIntervalMs   int                           `json:"sampleIntervalMs"` // 采样间隔（毫秒）
	MotionTimeoutMs    int                           `json:"motionTimeoutMs"`  // 运动等待超时（毫秒）
	SavePath           string                        `json:"savePath"`
	SaveFileName       string                        `json:"saveFileName"`
}

// ==================== 三孔原始数据 ====================

// ThreeHoleRawData 三孔原始数据
type ThreeHoleRawData struct {
	P1   float64 `json:"p1"`
	P2   float64 `json:"p2"`
	P3   float64 `json:"p3"`
	PAtm float64 `json:"pAtm"`
	TAtm float64 `json:"tAtm"`
}

// ==================== 插值结果 ====================

// ThreeHoleInterpolationResult 三孔插值结果
type ThreeHoleInterpolationResult struct {
	PtProbe        float64 `json:"ptProbe"`            // 探针计算总压（表压 Pa）
	PsProbe        float64 `json:"psProbe"`            // 探针计算静压（表压 Pa）
	MachProbe      float64 `json:"machProbe"`          // 计算马赫数
	AlphaProbe     float64 `json:"alphaProbe"`         // 计算攻角（度）
	IterationCount int     `json:"iterationCount"`     // 迭代收敛次数
	Converged      bool    `json:"converged"`          // 迭代是否收敛
	Valid          bool    `json:"valid"`              // 结果是否有效
	ErrorMsg       string  `json:"errorMsg,omitempty"` // 无效/警告原因描述
}

// ==================== 测试数据点 ====================

// ThreeHoleTraversalDataPoint 三孔移位测试数据点
type ThreeHoleTraversalDataPoint struct {
	PointID      string                       `json:"pointId"`
	X            float64                      `json:"x"`
	Y            float64                      `json:"y"`
	RawData      ThreeHoleRawData             `json:"rawData"`
	InterpResult ThreeHoleInterpolationResult `json:"interpResult"`
	SampleCount  int                          `json:"sampleCount"`
	Timestamp    int64                        `json:"timestamp"`
}

// ==================== 测试状态 ====================

// TraversalTestStatus 测试状态
type TraversalTestStatus string

const (
	TraversalStatusIdle      TraversalTestStatus = "idle"
	TraversalStatusRunning   TraversalTestStatus = "running"
	TraversalStatusPaused    TraversalTestStatus = "paused"
	TraversalStatusCompleted TraversalTestStatus = "completed"
	TraversalStatusError     TraversalTestStatus = "error"
)

// ThreeHoleTraversalTaskStatus 三孔移位测试任务状态
type ThreeHoleTraversalTaskStatus struct {
	TaskID          string                        `json:"taskId"`
	Status          TraversalTestStatus           `json:"status"`
	TotalPoints     int                           `json:"totalPoints"`
	CompletedPoints int                           `json:"completedPoints"`
	Progress        float64                       `json:"progress"`
	CurrentPoint    *TraversalPoint               `json:"currentPoint,omitempty"`
	DataPoints      []ThreeHoleTraversalDataPoint `json:"dataPoints"`
	LastError       string                        `json:"lastError,omitempty"`
}

// ==================== 事件类型 ====================

// ThreeHoleTraversalProgressEvent 进度事件
type ThreeHoleTraversalProgressEvent struct {
	TaskID          string  `json:"taskId"`
	TotalPoints     int     `json:"totalPoints"`
	CompletedPoints int     `json:"completedPoints"`
	Progress        float64 `json:"progress"`
	CurrentX        float64 `json:"currentX"`
	CurrentY        float64 `json:"currentY"`
	Phase           string  `json:"phase,omitempty"`
}

// ThreeHoleTraversalRealtimeEvent 实时数据事件
type ThreeHoleTraversalRealtimeEvent struct {
	TaskID       string                       `json:"taskId"`
	PointID      string                       `json:"pointId"`
	RawData      ThreeHoleRawData             `json:"rawData"`
	InterpResult ThreeHoleInterpolationResult `json:"interpResult"`
}

// ThreeHoleTraversalCompleteEvent 完成事件
type ThreeHoleTraversalCompleteEvent struct {
	TaskID     string                        `json:"taskId"`
	Status     TraversalTestStatus           `json:"status"`
	DataPoints []ThreeHoleTraversalDataPoint `json:"dataPoints"`
}

// ThreeHoleTraversalErrorEvent 错误事件
type ThreeHoleTraversalErrorEvent struct {
	TaskID  string `json:"taskId"`
	Error   string `json:"error"`
	IsFatal bool   `json:"isFatal"` // 致命错误会中止测试，非致命仅记录点位error
}

// ==================== 校准数据结构 ====================

// ThreeHoleCalibEntry 单条校准数据
type ThreeHoleCalibEntry struct {
	Kb    float64 `json:"kb"`
	Kt    float64 `json:"kt"`
	Sb    float64 `json:"sb"`
	Alpha float64 `json:"alpha"`
}

// ThreeHoleCalibData 单个校准马赫数下的校准数据
type ThreeHoleCalibData struct {
	CMa      float64               `json:"cMa"`
	FilePath string                `json:"filePath"` // 来源文件路径
	FileName string                `json:"fileName"` // 来源文件名
	Entries  []ThreeHoleCalibEntry `json:"entries"`
}
