package types

import "fmt"

// MaxFiveHoleProbes 五孔测试最大启用探针数（后端校验上限，前端 MAX_PROBES 与此保持一致）
const MaxFiveHoleProbes = 8

// ==================== 五孔测试配置 ====================

// FiveHoleTraversalConfig 五孔移位测试配置（全局配置，含 1-MaxFiveHoleProbes 根探针）
type FiveHoleTraversalConfig struct {
	Name             string               `json:"name"`
	Layout           TraversalLayout      `json:"layout"`           // 布点（复用三孔 TraversalLayout）
	DwellTimeMs      int                  `json:"dwellTimeMs"`      // 驻留时间（所有探针共用）
	SamplesPerPoint  int                  `json:"samplesPerPoint"`  // 采样次数（所有探针共用）
	SampleIntervalMs int                  `json:"sampleIntervalMs"` // 采样间隔（所有探针共用）
	MotionTimeoutMs  int                  `json:"motionTimeoutMs"`  // 运动等待超时（所有探针共用）
	// 共用轴位：true 时所有启用探针统一使用 SharedMotionX/Y（多探针装在同一位移机构场景），
	// 各探针独立 MotionX/MotionY 被忽略（配置保留，切回独立模式时仍可用）
	SharedMotion     bool                     `json:"sharedMotion"`
	SharedMotionX    FiveHoleMotionAxisMapping `json:"sharedMotionX"`
	SharedMotionY    FiveHoleMotionAxisMapping `json:"sharedMotionY"`
	// 1-8 根探针（配几根跑几根）
	Probes           []FiveHoleProbeConfig `json:"probes"`
	SavePath         string               `json:"savePath"`
	SaveFileName     string               `json:"saveFileName"`
}

// 注：FiveHoleRawData 已在 calibration.go 定义（P1-P5+PAtm+TAtm+可选PTotal），
// 五孔移位测试模块直接复用该类型，PTotal 字段可选可忽略。

// ==================== Validate ====================

// ApplySharedMotion 返回应用共用轴位后的配置副本
// SharedMotion=true 时，启用探针的 MotionX/MotionY 被 SharedMotionX/Y 覆盖；
// 浅拷贝 Probes 切片，不修改原配置（各探针独立轴位保留，切回独立模式时仍可用）
// SharedMotion=false 时原样返回
func (c FiveHoleTraversalConfig) ApplySharedMotion() FiveHoleTraversalConfig {
	if !c.SharedMotion {
		return c
	}
	probes := make([]FiveHoleProbeConfig, len(c.Probes))
	copy(probes, c.Probes)
	for i := range probes {
		if probes[i].Enabled {
			probes[i].MotionX = c.SharedMotionX
			probes[i].MotionY = c.SharedMotionY
		}
	}
	c.Probes = probes
	return c
}

// Validate 验证五孔移位测试配置
// 共用轴位模式：先校验全局轴位非空（直线布点只走单轴，Y 方向不校验，与UI隐藏Y选择器一致），
// 归一化到各探针副本后走通用校验（不修改原配置）
func (c *FiveHoleTraversalConfig) Validate() error {
	if c.SharedMotion {
		if c.SharedMotionX.ControllerID == "" || c.SharedMotionX.Axis == "" {
			return fmt.Errorf("共用轴位的X方向未选择位移机构或轴号")
		}
		if c.Layout.Pattern != TraversalPatternLine {
			if c.SharedMotionY.ControllerID == "" || c.SharedMotionY.Axis == "" {
				return fmt.Errorf("共用轴位的Y方向未选择位移机构或轴号")
			}
		}
		return c.ApplySharedMotion().validate()
	}
	return c.validate()
}

// validate 通用配置校验（探针轴位等字段已被调用方归一化；只读，值接收者）
func (c FiveHoleTraversalConfig) validate() error {
	if c.Name == "" {
		return fmt.Errorf("测试名称不能为空")
	}

	// 采样参数验证（所有探针共用）
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
		if enabledCount > MaxFiveHoleProbes {
			return fmt.Errorf("最多启用%d根探针", MaxFiveHoleProbes)
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

		// 大气压/气流温度数据源验证（每探针独立：设备读取须选设备，手动写入无额外约束）
		if err := p.PAtmSource.validate("大气压"); err != nil {
			return fmt.Errorf("探针%s的%w", p.ProbeID, err)
		}
		if err := p.TAtmSource.validate("气流温度"); err != nil {
			return fmt.Errorf("探针%s的%w", p.ProbeID, err)
		}

		// 运动轴配置验证（X、Y 方向各自选位移机构+轴号）
		// 直线布点只走 X 单轴（buildMoveTasks 跳过 Y），Y 不校验（与UI直线模式隐藏Y选择器一致）
		if p.MotionX.ControllerID == "" {
			return fmt.Errorf("探针%s的X方向未选择位移机构", p.ProbeID)
		}
		if p.MotionX.Axis == "" {
			return fmt.Errorf("探针%s的X方向轴号不能为空", p.ProbeID)
		}
		if c.Layout.Pattern != TraversalPatternLine {
			if p.MotionY.ControllerID == "" {
				return fmt.Errorf("探针%s的Y方向未选择位移机构", p.ProbeID)
			}
			if p.MotionY.Axis == "" {
				return fmt.Errorf("探针%s的Y方向轴号不能为空", p.ProbeID)
			}
		}

		// 校准文件验证（启用探针必须载入至少一个 .cal）
		if len(p.CalibFiles) == 0 {
			return fmt.Errorf("探针%s必须载入至少1个校准文件", p.ProbeID)
		}
	}

	if enabledCount == 0 {
		return fmt.Errorf("必须启用至少1根探针")
	}

	// 布局配置验证
	switch c.Layout.Pattern {
	case TraversalPatternLine:
		if c.Layout.Line == nil {
			return fmt.Errorf("直线布点需要Line配置")
		}
		if c.Layout.Line.Step <= 0 {
			return fmt.Errorf("直线布点Step必须>0")
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
		// 矩形模式：每根启用探针 MotionX/MotionY 不能指向同一根物理轴
		for _, p := range c.Probes {
			if !p.Enabled {
				continue
			}
			if p.MotionX.ControllerID == p.MotionY.ControllerID && p.MotionX.Axis == p.MotionY.Axis {
				return fmt.Errorf("探针%s的X/Y方向不能指向同一根物理轴", p.ProbeID)
			}
		}
	case TraversalPatternCustom:
		if len(c.Layout.CustomPoints) == 0 {
			return fmt.Errorf("自定义布点需要至少1个点位")
		}
	case TraversalPatternFan:
		if c.Layout.Fan == nil {
			return fmt.Errorf("扇形布点需要Fan配置")
		}
		if len(c.Layout.Fan.RSteps) == 0 {
			return fmt.Errorf("扇形布点R方向步进不能为空")
		}
		if len(c.Layout.Fan.ThetaSteps) == 0 {
			return fmt.Errorf("扇形布点θ方向步进不能为空")
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
