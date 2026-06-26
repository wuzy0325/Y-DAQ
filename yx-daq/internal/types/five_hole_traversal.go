package types

import "fmt"

// ==================== 五孔探针通道角色 ====================

// FiveHoleChannelRole 五孔探针通道语义角色
type FiveHoleChannelRole string

const (
	Role5H_P1   FiveHoleChannelRole = "fiveHole.p1"   // 1号孔压力
	Role5H_P2   FiveHoleChannelRole = "fiveHole.p2"   // 2号孔压力（中心孔）
	Role5H_P3   FiveHoleChannelRole = "fiveHole.p3"   // 3号孔压力
	Role5H_P4   FiveHoleChannelRole = "fiveHole.p4"   // 4号孔压力
	Role5H_P5   FiveHoleChannelRole = "fiveHole.p5"   // 5号孔压力
	Role5H_PAtm FiveHoleChannelRole = "fiveHole.pAtm" // 大气压（全局共享）
	Role5H_TAtm FiveHoleChannelRole = "fiveHole.tAtm" // 大气温度（全局共享）
)

// FiveHoleProbeChannelConfig 五孔探针通道配置（每通道独立选采集设备）
type FiveHoleProbeChannelConfig struct {
	Name     string              `json:"name"`
	Role     FiveHoleChannelRole `json:"role"`
	DeviceID string              `json:"deviceId"` // 每通道独立选采集设备
	Channel  int                 `json:"channel"`
	Enabled  bool                `json:"enabled"`
}

// ==================== 五孔运动轴映射 ====================

// FiveHoleMotionAxisMapping 五孔运动轴映射（每轴独立选位移机构）
// 与三孔 MotionAxisMapping 区别：含 ControllerID，支持每轴独立位移机构
type FiveHoleMotionAxisMapping struct {
	ControllerID string  `json:"controllerId"` // 位移机构ID
	Axis        AxisName `json:"axis"`         // 轴名（X/Y/Z/U）
}

// ==================== 五孔校准文件 ====================

// FiveHoleCalibFileInfo 五孔校准文件信息
type FiveHoleCalibFileInfo struct {
	FilePath string  `json:"filePath"`
	FileName string  `json:"fileName"`
	CMa      float64 `json:"cMa"` // 校准马赫数
}

// FiveHoleCalibEntry 五孔校准条目（占位，待算法确认实际字段）
// 参考文档提到 .prb 是 13×13 网格，含 ka kb cpt cps alpha beta 六列
type FiveHoleCalibEntry struct {
	Ka    float64 `json:"ka"`
	Kb    float64 `json:"kb"`
	Cpt   float64 `json:"cpt"`
	Cps   float64 `json:"cps"`
	Alpha float64 `json:"alpha"`
	Beta  float64 `json:"beta"`
}

// FiveHoleCalibData 单个校准马赫数下的五孔校准数据
type FiveHoleCalibData struct {
	CMa      float64              `json:"cMa"`
	FilePath string               `json:"filePath"`
	FileName string               `json:"fileName"`
	Entries  []FiveHoleCalibEntry `json:"entries"`
}

// ==================== 五孔探针配置 ====================

// FiveHoleProbeConfig 单根五孔探针配置
type FiveHoleProbeConfig struct {
	ProbeID       string                       `json:"probeId"`       // probe1/probe2/probe3
	Enabled       bool                         `json:"enabled"`       // 是否启用（配几根跑几根）
	ProbeChannels []FiveHoleProbeChannelConfig `json:"probeChannels"` // P1-P5 各自数据源
	MotionAlpha   FiveHoleMotionAxisMapping    `json:"motionAlpha"`   // α 轴：位移机构 + 轴号
	MotionBeta    FiveHoleMotionAxisMapping    `json:"motionBeta"`    // β 轴：位移机构 + 轴号
	CalibFiles    []FiveHoleCalibFileInfo       `json:"calibFiles"`    // .prb 校准文件（每探针独立载入）
}

// ==================== 五孔测试配置 ====================

// FiveHoleTraversalConfig 五孔移位测试配置（全局配置，含 1-3 探针）
type FiveHoleTraversalConfig struct {
	Name             string               `json:"name"`
	Layout           TraversalLayout      `json:"layout"`           // 布点（复用三孔 TraversalLayout）
	DwellTimeMs      int                  `json:"dwellTimeMs"`      // 驻留时间（三根共用）
	SamplesPerPoint  int                  `json:"samplesPerPoint"`  // 采样次数（三根共用）
	SampleIntervalMs int                  `json:"sampleIntervalMs"` // 采样间隔（三根共用）
	MotionTimeoutMs  int                  `json:"motionTimeoutMs"`  // 运动等待超时（三根共用）
	// PAtm/TAtm 全局共享数据源（三根共用）
	PAtmDeviceID     string               `json:"pAtmDeviceId"`
	PAtmChannel      int                  `json:"pAtmChannel"`
	TAtmDeviceID     string               `json:"tAtmDeviceId"`
	TAtmChannel      int                  `json:"tAtmChannel"`
	// 1-3 根探针（配几根跑几根）
	Probes           []FiveHoleProbeConfig `json:"probes"`
	SavePath         string               `json:"savePath"`
	SaveFileName     string               `json:"saveFileName"`
}

// ==================== 五孔原始数据 ====================
// 注：FiveHoleRawData 已在 calibration.go 定义（P1-P5+PAtm+TAtm+可选PTotal），
// 五孔移位测试模块直接复用该类型，PTotal 字段可选可忽略。

// ==================== 五孔插值结果 ====================

// FiveHoleInterpolationResult 五孔插值结果
type FiveHoleInterpolationResult struct {
	PtProbe        float64 `json:"ptProbe"`            // 总压（表压 Pa）
	PsProbe        float64 `json:"psProbe"`            // 静压（表压 Pa）
	MachProbe      float64 `json:"machProbe"`          // 马赫数
	AlphaProbe     float64 `json:"alphaProbe"`         // 攻角（度）
	BetaProbe      float64 `json:"betaProbe"`          // 侧滑角（度）
	VelocityProbe  float64 `json:"velocityProbe"`      // 速度（m/s）
	IterationCount int     `json:"iterationCount"`     // 迭代收敛次数
	Converged      bool    `json:"converged"`          // 迭代是否收敛
	Valid          bool    `json:"valid"`               // 结果是否有效
	ErrorMsg       string  `json:"errorMsg,omitempty"`  // 无效/警告原因描述
}

// ==================== 五孔测试数据点 ====================

// FiveHoleTraversalDataPoint 五孔移位测试数据点（每探针一份）
type FiveHoleTraversalDataPoint struct {
	PointID      string                      `json:"pointId"`
	ProbeID      string                      `json:"probeId"`
	X            float64                     `json:"x"`
	Y            float64                     `json:"y"`
	RawData      FiveHoleRawData             `json:"rawData"`
	InterpResult FiveHoleInterpolationResult `json:"interpResult"`
	SampleCount  int                         `json:"sampleCount"`
	Timestamp    int64                       `json:"timestamp"`
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

// ==================== Validate ====================

// Validate 验证五孔移位测试配置
func (c *FiveHoleTraversalConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("测试名称不能为空")
	}

	// 采样参数验证（三根共用）
	if c.SamplesPerPoint < 1 {
		return fmt.Errorf("每点位采样数必须≥1")
	}
	if c.DwellTimeMs < 100 {
		return fmt.Errorf("驻留时间必须≥100ms")
	}
	if c.SampleIntervalMs < 10 {
		return fmt.Errorf("采样间隔必须≥10ms")
	}
	if c.MotionTimeoutMs < 1000 {
		return fmt.Errorf("运动超时时间必须≥1000ms")
	}

	// 全局 PAtm/TAtm 数据源验证
	if c.PAtmDeviceID == "" {
		return fmt.Errorf("大气压采集设备ID不能为空")
	}
	if c.PAtmChannel < 0 {
		return fmt.Errorf("大气压通道号必须≥0")
	}
	if c.TAtmDeviceID == "" {
		return fmt.Errorf("大气温度采集设备ID不能为空")
	}
	if c.TAtmChannel < 0 {
		return fmt.Errorf("大气温度通道号必须≥0")
	}

	// 探针配置验证
	if len(c.Probes) == 0 {
		return fmt.Errorf("必须配置至少1根探针")
	}

	// 统计启用探针数
	enabledCount := 0
	probeIDs := make(map[string]bool)
	for i := range c.Probes {
		p := &c.Probes[i]
		if !p.Enabled {
			continue
		}
		enabledCount++
		if enabledCount > 3 {
			return fmt.Errorf("最多启用3根探针")
		}

		// ProbeID 唯一性
		if p.ProbeID == "" {
			return fmt.Errorf("探针%d的ProbeID不能为空", i+1)
		}
		if probeIDs[p.ProbeID] {
			return fmt.Errorf("探针ProbeID重复: %s", p.ProbeID)
		}
		probeIDs[p.ProbeID] = true

		// 通道配置验证：P1-P5 必须启用且通道号有效
		if len(p.ProbeChannels) == 0 {
			return fmt.Errorf("探针%s必须配置通道", p.ProbeID)
		}
		channelRoles := make(map[FiveHoleChannelRole]bool)
		for _, ch := range p.ProbeChannels {
			if ch.Channel < 0 {
				return fmt.Errorf("探针%s的通道号必须≥0", p.ProbeID)
			}
			if ch.DeviceID == "" {
				return fmt.Errorf("探针%s的通道%s未选择采集设备", p.ProbeID, ch.Role)
			}
			if !ch.Enabled {
				continue
			}
			if channelRoles[ch.Role] {
				return fmt.Errorf("探针%s的角色重复: %s", p.ProbeID, ch.Role)
			}
			channelRoles[ch.Role] = true
		}

		// 检查 P1-P5 必要角色
		requiredRoles := map[FiveHoleChannelRole]bool{
			Role5H_P1: false,
			Role5H_P2: false,
			Role5H_P3: false,
			Role5H_P4: false,
			Role5H_P5: false,
		}
		for _, ch := range p.ProbeChannels {
			if ch.Enabled {
				if _, ok := requiredRoles[ch.Role]; ok {
					requiredRoles[ch.Role] = true
				}
			}
		}
		for role, ok := range requiredRoles {
			if !ok {
				return fmt.Errorf("探针%s必须启用%s通道", p.ProbeID, role)
			}
		}

		// 运动轴配置验证（α、β 各自选位移机构+轴号）
		if p.MotionAlpha.ControllerID == "" {
			return fmt.Errorf("探针%s的α轴未选择位移机构", p.ProbeID)
		}
		if p.MotionAlpha.Axis == "" {
			return fmt.Errorf("探针%s的α轴号不能为空", p.ProbeID)
		}
		if p.MotionBeta.ControllerID == "" {
			return fmt.Errorf("探针%s的β轴未选择位移机构", p.ProbeID)
		}
		if p.MotionBeta.Axis == "" {
			return fmt.Errorf("探针%s的β轴号不能为空", p.ProbeID)
		}

		// 校准文件验证（启用探针必须载入至少一个 .prb）
		if len(p.CalibFiles) == 0 {
			return fmt.Errorf("探针%s必须载入至少1个校准文件", p.ProbeID)
		}
	}

	if enabledCount == 0 {
		return fmt.Errorf("必须启用至少1根探针")
	}

	// 布局配置验证（照三孔）
	switch c.Layout.Pattern {
	case TraversalPatternLine:
		if c.Layout.Line == nil {
			return fmt.Errorf("直线布点需要Line配置")
		}
		if c.Layout.Line.StartX == 0 && c.Layout.Line.EndX == 0 &&
			len(c.Layout.Line.XSteps) == 0 {
			return fmt.Errorf("直线布点必须有X方向配置")
		}
	case TraversalPatternRectangle:
		if c.Layout.Rectangle == nil {
			return fmt.Errorf("矩形布点需要Rectangle配置")
		}
		if c.Layout.Rectangle.XMin > c.Layout.Rectangle.XMax {
			return fmt.Errorf("XMin必须≤XMax")
		}
		if c.Layout.Rectangle.YMin > c.Layout.Rectangle.YMax {
			return fmt.Errorf("YMin必须≤YMax")
		}
	case TraversalPatternCustom:
		if len(c.Layout.CustomPoints) == 0 {
			return fmt.Errorf("自定义布点需要至少1个点位")
		}
	default:
		return fmt.Errorf("不支持的布点模式: %s", c.Layout.Pattern)
	}

	// 文件保存配置验证
	if c.SavePath == "" {
		return fmt.Errorf("保存路径不能为空")
	}
	if c.SaveFileName == "" {
		return fmt.Errorf("保存文件名不能为空")
	}

	return nil
}
