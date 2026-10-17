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

// FiveHoleTraversalTiming 五孔移位测试耗时统计（预估 vs 实际）
//
// 预估口径（理论值，启动前即可算出）：
//   移动 = 点位间距离 / 轴配置最大速度（同相位多轴并行取最大；扇面 θ 相与 R 相串行相加）
//   驻留 = 配置驻留时间
//   采样 = 采样次数 × 最慢采集设备出帧周期（采样频率 = 1000/PeriodMs）
//
// 实际口径：移动/驻留/采样三段分别实测（暂停时长不计入），
// 总耗时为测试开始至今扣除暂停时间。
type FiveHoleTraversalTiming struct {
	// ===== 预估（全点位合计） =====
	EstimatedTotalMs int64 `json:"estimatedTotalMs"` // 预估总时长
	EstMoveTotalMs   int64 `json:"estMoveTotalMs"`   // 预估移动耗时合计
	EstDwellTotalMs  int64 `json:"estDwellTotalMs"`  // 预估驻留耗时合计
	EstSampleTotalMs int64 `json:"estSampleTotalMs"` // 预估采样耗时合计
	// ===== 预估（单点位，界面展示"每点：移动+驻留+采样"） =====
	EstMoveMsPerPoint   int64   `json:"estMoveMsPerPoint"`
	EstDwellMsPerPoint  int64   `json:"estDwellMsPerPoint"`
	EstSampleMsPerPoint int64   `json:"estSampleMsPerPoint"`
	SampleHz            float64 `json:"sampleHz"` // 参与预估的最慢设备采样频率（Hz）
	// ===== 实际（累计） =====
	ActualElapsedMs int64 `json:"actualElapsedMs"` // 实际已用总时长（不含暂停）
	ActualMoveMs    int64 `json:"actualMoveMs"`    // 移动耗时累计
	ActualDwellMs   int64 `json:"actualDwellMs"`   // 驻留耗时累计
	ActualSampleMs  int64 `json:"actualSampleMs"`  // 采样耗时累计
	ActualPoints    int   `json:"actualPoints"`    // 已完成点位数量（用于折算每点实际耗时）
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
	// 耗时预估 vs 实际（测试未启动时为 nil）
	Timing *FiveHoleTraversalTiming `json:"timing,omitempty"`
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
	// 耗时预估 vs 实际（测试未启动时为 nil）
	Timing *FiveHoleTraversalTiming `json:"timing,omitempty"`
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
