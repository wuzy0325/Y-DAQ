package types

// ==================== 五孔插值结果 ====================

// FiveHoleInterpolationResult 五孔插值结果
// 字段对齐 vendored interpolation.InterpolationResult，移除 IterationCount/Converged，
// 新增 CAS/SAT/动压/密度/三向速度分量。
type FiveHoleInterpolationResult struct {
	PtProbe         float64 `json:"ptProbe"`             // 总压（表压 Pa）
	PsProbe         float64 `json:"psProbe"`             // 静压（表压 Pa）
	MachProbe       float64 `json:"machProbe"`           // 马赫数
	AlphaProbe      float64 `json:"alphaProbe"`          // 攻角（度）
	BetaProbe       float64 `json:"betaProbe"`           // 侧滑角（度）
	VelocityProbe   float64 `json:"velocityProbe"`       // 真空速 TAS（m/s）
	CASProbe        float64 `json:"casProbe"`            // 校正空速 CAS（m/s）
	SATProbe        float64 `json:"satProbe"`            // 静温 SAT（K）
	DynamicPressure float64 `json:"dynamicPressure"`     // 动压（Pa）
	Density         float64 `json:"density"`             // 密度（kg/m³）
	VxProbe         float64 `json:"vxProbe"`             // X 方向速度分量（m/s）
	VyProbe         float64 `json:"vyProbe"`             // Y 方向速度分量（m/s）
	VzProbe         float64 `json:"vzProbe"`             // Z 方向速度分量（m/s）
	Valid           bool    `json:"valid"`               // 结果是否有效
	ErrorMsg        string  `json:"errorMsg,omitempty"`  // 无效/警告原因描述
}

// ==================== 五孔测试数据点 ====================

// FiveHoleTraversalDataPoint 五孔移位测试数据点（每探针一份）
type FiveHoleTraversalDataPoint struct {
	PointID         string                      `json:"pointId"`
	ProbeID         string                      `json:"probeId"`
	X               float64                     `json:"x"`
	Y               float64                     `json:"y"`
	XControllerName string                      `json:"xControllerName"` // X 方向位移机构名
	XAxis           AxisName                    `json:"xAxis"`           // X 方向轴号
	YControllerName string                      `json:"yControllerName"` // Y 方向位移机构名
	YAxis           AxisName                    `json:"yAxis"`           // Y 方向轴号
	RawData         FiveHoleRawData             `json:"rawData"`
	InterpResult    FiveHoleInterpolationResult `json:"interpResult"`
	SampleCount     int                         `json:"sampleCount"`
	Timestamp       int64                       `json:"timestamp"`
}

// ==================== 五孔测试状态 ====================

// FiveHoleProbeStatus 单根探针实时状态
type FiveHoleProbeStatus struct {
	ProbeID      string                      `json:"probeId"`
	Phase        string                      `json:"phase"`        // moving/waiting/acquiring/completed
	CurrentX     float64                     `json:"currentX"`
	CurrentY     float64                     `json:"currentY"`
	RawData      *FiveHoleRawData            `json:"rawData,omitempty"`
	InterpResult *FiveHoleInterpolationResult `json:"interpResult,omitempty"`
}

// FiveHoleTraversalTaskStatus 五孔测试任务状态
type FiveHoleTraversalTaskStatus struct {
	TaskID          string                 `json:"taskId"`
	Status          TraversalTestStatus    `json:"status"`          // 统一状态
	TotalPoints     int                    `json:"totalPoints"`
	CompletedPoints int                    `json:"completedPoints"` // 统一进度（等最慢探针）
	Progress        float64                `json:"progress"`
	CurrentPoint    *TraversalPoint        `json:"currentPoint,omitempty"`
	// 每探针独立 phase/坐标（统一进度 + 各探针 phase 指示）
	ProbeStatuses   []FiveHoleProbeStatus  `json:"probeStatuses"`
	LastError       string                 `json:"lastError,omitempty"`
}

// ==================== 五孔事件类型 ====================

// FiveHoleTraversalProgressEvent 进度事件
type FiveHoleTraversalProgressEvent struct {
	TaskID          string                `json:"taskId"`
	TotalPoints     int                   `json:"totalPoints"`
	CompletedPoints int                   `json:"completedPoints"`
	Progress        float64               `json:"progress"`
	CurrentX        float64               `json:"currentX"`
	CurrentY        float64               `json:"currentY"`
	Phase           string                `json:"phase,omitempty"`
	ProbeStatuses   []FiveHoleProbeStatus `json:"probeStatuses"`
}

// FiveHoleTraversalRealtimeEvent 实时数据事件（含所有启用探针的实时数据）
type FiveHoleTraversalRealtimeEvent struct {
	TaskID       string                      `json:"taskId"`
	PointID      string                      `json:"pointId"`
	Phase        string                      `json:"phase,omitempty"`
	ProbeRealtime []FiveHoleProbeRealtimeItem `json:"probeRealtime"`
}

// FiveHoleProbeRealtimeItem 单根探针实时数据项
type FiveHoleProbeRealtimeItem struct {
	ProbeID      string                      `json:"probeId"`
	RawData      FiveHoleRawData             `json:"rawData"`
	InterpResult FiveHoleInterpolationResult `json:"interpResult"`
}

// FiveHoleTraversalCompleteEvent 完成事件
type FiveHoleTraversalCompleteEvent struct {
	TaskID string                        `json:"taskId"`
	Status TraversalTestStatus           `json:"status"`
	// 每探针的数据点列表（每探针独立 CSV）
	ProbeDataPoints map[string][]FiveHoleTraversalDataPoint `json:"probeDataPoints"`
}

// FiveHoleTraversalErrorEvent 错误事件
type FiveHoleTraversalErrorEvent struct {
	TaskID  string `json:"taskId"`
	Error   string `json:"error"`
	IsFatal bool   `json:"isFatal"`
}
