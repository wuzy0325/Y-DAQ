package types

import (
	"strings"
	"testing"
)

// ==================== 三孔 Validate ====================

// validThreeHoleConfig 构造一个通过 Validate 的三孔配置
func validThreeHoleConfig() ThreeHoleTraversalConfig {
	return ThreeHoleTraversalConfig{
		Name:               "test",
		DeviceID:           "dev-1",
		MotionControllerID: "mc-1",
		Layout: TraversalLayout{
			Pattern: TraversalPatternLine,
			Line: &LineLayout{
				Axis:  LineAxisX,
				Start: 0,
				End:   10,
				Step:  5,
				Fixed: 0,
			},
		},
		ProbeChannels: []ThreeHoleProbeChannelConfig{
			{Role: Role3H_P1, Channel: 0, Enabled: true},
			{Role: Role3H_P2, Channel: 1, Enabled: true},
			{Role: Role3H_P3, Channel: 2, Enabled: true},
			{Role: Role3H_PAtm, Channel: 3, Enabled: true},
		},
		MotionAlpha:      MotionAxisMapping{Axis: AxisX},
		MotionBeta:       MotionAxisMapping{Axis: AxisY},
		DwellTimeMs:      100,
		SamplesPerPoint:  1,
		SampleIntervalMs: 10,
		MotionTimeoutMs:  1000,
		SavePath:         "/tmp",
		SaveFileName:     "out.csv",
	}
}

func TestThreeHoleValidate_OK(t *testing.T) {
	c := validThreeHoleConfig()
	if err := c.Validate(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestThreeHoleValidate_BasicFields(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*ThreeHoleTraversalConfig)
		wantSub string
	}{
		{"empty name", func(c *ThreeHoleTraversalConfig) { c.Name = "" }, "测试名称"},
		{"empty deviceID", func(c *ThreeHoleTraversalConfig) { c.DeviceID = "" }, "采集设备ID"},
		{"empty motionControllerID", func(c *ThreeHoleTraversalConfig) { c.MotionControllerID = "" }, "运动控制器ID"},
		{"samples < 1", func(c *ThreeHoleTraversalConfig) { c.SamplesPerPoint = 0 }, "采样数"},
		{"dwell < 100", func(c *ThreeHoleTraversalConfig) { c.DwellTimeMs = 99 }, "驻留时间"},
		{"interval < 10", func(c *ThreeHoleTraversalConfig) { c.SampleIntervalMs = 9 }, "采样间隔"},
		{"motion timeout < 1000", func(c *ThreeHoleTraversalConfig) { c.MotionTimeoutMs = 999 }, "运动超时"},
		{"empty alpha axis", func(c *ThreeHoleTraversalConfig) { c.MotionAlpha = MotionAxisMapping{} }, "Alpha轴"},
		{"empty beta axis", func(c *ThreeHoleTraversalConfig) { c.MotionBeta = MotionAxisMapping{} }, "Beta轴"},
		{"empty savePath", func(c *ThreeHoleTraversalConfig) { c.SavePath = "" }, "保存路径"},
		{"empty saveFileName", func(c *ThreeHoleTraversalConfig) { c.SaveFileName = "" }, "保存文件名"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validThreeHoleConfig()
			tc.mutate(&c)
			err := c.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("expected error containing %q, got %v", tc.wantSub, err)
			}
		})
	}
}

func TestThreeHoleValidate_Layout(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*ThreeHoleTraversalConfig)
		wantSub string
	}{
		{
			"line nil",
			func(c *ThreeHoleTraversalConfig) {
				c.Layout = TraversalLayout{Pattern: TraversalPatternLine}
			},
			"Line配置",
		},
		{
			"line empty axis",
			func(c *ThreeHoleTraversalConfig) {
				c.Layout = TraversalLayout{
					Pattern: TraversalPatternLine,
					Line:    &LineLayout{Axis: "", Start: 0, End: 10, Step: 5, Fixed: 0},
				}
			},
			"Axis",
		},
		{
			"line zero step",
			func(c *ThreeHoleTraversalConfig) {
				c.Layout = TraversalLayout{
					Pattern: TraversalPatternLine,
					Line:    &LineLayout{Axis: LineAxisX, Start: 0, End: 10, Step: 0, Fixed: 0},
				}
			},
			"Step",
		},
		{
			"rectangle nil",
			func(c *ThreeHoleTraversalConfig) {
				c.Layout = TraversalLayout{Pattern: TraversalPatternRectangle}
			},
			"Rectangle配置",
		},
		{
			"rectangle XMin>XMax",
			func(c *ThreeHoleTraversalConfig) {
				c.Layout = TraversalLayout{
					Pattern:   TraversalPatternRectangle,
					Rectangle: &RectangleLayout{XMin: 10, XMax: 5, YMin: 0, YMax: 5},
				}
			},
			"XMin",
		},
		{
			"rectangle YMin>YMax",
			func(c *ThreeHoleTraversalConfig) {
				c.Layout = TraversalLayout{
					Pattern:   TraversalPatternRectangle,
					Rectangle: &RectangleLayout{XMin: 0, XMax: 5, YMin: 10, YMax: 5},
				}
			},
			"YMin",
		},
		{
			"custom empty",
			func(c *ThreeHoleTraversalConfig) {
				c.Layout = TraversalLayout{Pattern: TraversalPatternCustom}
			},
			"自定义布点",
		},
		{
			"unknown pattern",
			func(c *ThreeHoleTraversalConfig) {
				c.Layout = TraversalLayout{Pattern: "unknown"}
			},
			"布点模式",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validThreeHoleConfig()
			tc.mutate(&c)
			err := c.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("expected error containing %q, got %v", tc.wantSub, err)
			}
		})
	}
}

func TestThreeHoleValidate_Channels(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*ThreeHoleTraversalConfig)
		wantSub string
	}{
		{
			"no channels",
			func(c *ThreeHoleTraversalConfig) { c.ProbeChannels = nil },
			"通道",
		},
		{
			"negative channel",
			func(c *ThreeHoleTraversalConfig) {
				c.ProbeChannels[0].Channel = -1
			},
			"通道号",
		},
		{
			"duplicate role",
			func(c *ThreeHoleTraversalConfig) {
				c.ProbeChannels = append(c.ProbeChannels,
					ThreeHoleProbeChannelConfig{Role: Role3H_P1, Channel: 9, Enabled: true})
			},
			"角色重复",
		},
		{
			"missing P1",
			func(c *ThreeHoleTraversalConfig) {
				c.ProbeChannels[0].Enabled = false
			},
			"P1",
		},
		{
			"missing PAtm",
			func(c *ThreeHoleTraversalConfig) {
				c.ProbeChannels[3].Enabled = false
			},
			"大气压",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validThreeHoleConfig()
			tc.mutate(&c)
			err := c.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("expected error containing %q, got %v", tc.wantSub, err)
			}
		})
	}
}

// ==================== 五孔 Validate ====================

// validFiveHoleConfig 构造一个通过 Validate 的五孔配置
func validFiveHoleConfig() FiveHoleTraversalConfig {
	return FiveHoleTraversalConfig{
		Name:             "test",
		DwellTimeMs:      100,
		SamplesPerPoint:  1,
		SampleIntervalMs: 10,
		MotionTimeoutMs:  1000,
		PAtmDeviceID:     "dev-atm",
		PAtmChannel:      0,
		TAtmDeviceID:     "dev-tatm",
		TAtmChannel:      1,
		Probes: []FiveHoleProbeConfig{
			{
				ProbeID: "probe1",
				Enabled: true,
				ProbeChannels: []FiveHoleProbeChannelConfig{
					{Role: Role5H_P1, DeviceID: "d1", Channel: 0, Enabled: true},
					{Role: Role5H_P2, DeviceID: "d1", Channel: 1, Enabled: true},
					{Role: Role5H_P3, DeviceID: "d1", Channel: 2, Enabled: true},
					{Role: Role5H_P4, DeviceID: "d1", Channel: 3, Enabled: true},
					{Role: Role5H_P5, DeviceID: "d1", Channel: 4, Enabled: true},
				},
				MotionX: FiveHoleMotionAxisMapping{ControllerID: "mc-1", Axis: AxisX},
				MotionY: FiveHoleMotionAxisMapping{ControllerID: "mc-1", Axis: AxisY},
				CalibFiles:  []FiveHoleCalibFileInfo{{FilePath: "a.cal", FileName: "a.cal"}},
			},
		},
		Layout: TraversalLayout{
			Pattern: TraversalPatternLine,
			Line: &LineLayout{
				Axis:  string(AxisX),
				Start: 0,
				End:   10,
				Step:  5,
				Fixed: 0,
			},
		},
		SavePath:     "/tmp",
		SaveFileName: "out.csv",
	}
}

func TestFiveHoleValidate_OK(t *testing.T) {
	c := validFiveHoleConfig()
	if err := c.Validate(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestFiveHoleValidate_BasicFields(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*FiveHoleTraversalConfig)
		wantSub string
	}{
		{"empty name", func(c *FiveHoleTraversalConfig) { c.Name = "" }, "测试名称"},
		{"samples < 1", func(c *FiveHoleTraversalConfig) { c.SamplesPerPoint = 0 }, "采样数"},
		{"dwell < 100", func(c *FiveHoleTraversalConfig) { c.DwellTimeMs = 99 }, "驻留时间"},
		{"interval < 10", func(c *FiveHoleTraversalConfig) { c.SampleIntervalMs = 9 }, "采样间隔"},
		{"motion timeout < 1000", func(c *FiveHoleTraversalConfig) { c.MotionTimeoutMs = 999 }, "运动超时"},
		{"empty pAtm deviceID", func(c *FiveHoleTraversalConfig) { c.PAtmDeviceID = "" }, "大气压采集设备"},
		{"negative pAtm channel", func(c *FiveHoleTraversalConfig) { c.PAtmChannel = -1 }, "大气压通道"},
		{"empty tAtm deviceID", func(c *FiveHoleTraversalConfig) { c.TAtmDeviceID = "" }, "大气温度采集设备"},
		{"negative tAtm channel", func(c *FiveHoleTraversalConfig) { c.TAtmChannel = -1 }, "大气温度通道"},
		{"negative tTotal channel", func(c *FiveHoleTraversalConfig) {
			c.TTotalDeviceID = "dev-ttotal"
			c.TTotalChannel = -1
		}, "总温通道"},
		{"no probes", func(c *FiveHoleTraversalConfig) { c.Probes = nil }, "探针"},
		{"empty savePath", func(c *FiveHoleTraversalConfig) { c.SavePath = "" }, "保存路径"},
		{"empty saveFileName", func(c *FiveHoleTraversalConfig) { c.SaveFileName = "" }, "保存文件名"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validFiveHoleConfig()
			tc.mutate(&c)
			err := c.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("expected error containing %q, got %v", tc.wantSub, err)
			}
		})
	}
}

func TestFiveHoleValidate_Probes(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*FiveHoleTraversalConfig)
		wantSub string
	}{
		{
			"empty probeID",
			func(c *FiveHoleTraversalConfig) { c.Probes[0].ProbeID = "" },
			"ProbeID",
		},
		{
			"duplicate probeID",
			func(c *FiveHoleTraversalConfig) {
				c.Probes = append(c.Probes, c.Probes[0])
			},
			"重复",
		},
		{
			"no channels",
			func(c *FiveHoleTraversalConfig) { c.Probes[0].ProbeChannels = nil },
			"通道",
		},
		{
			"negative channel",
			func(c *FiveHoleTraversalConfig) { c.Probes[0].ProbeChannels[0].Channel = -1 },
			"通道号",
		},
		{
			"empty deviceID",
			func(c *FiveHoleTraversalConfig) { c.Probes[0].ProbeChannels[0].DeviceID = "" },
			"采集设备",
		},
		{
			"duplicate role",
			func(c *FiveHoleTraversalConfig) {
				c.Probes[0].ProbeChannels = append(c.Probes[0].ProbeChannels,
					FiveHoleProbeChannelConfig{Role: Role5H_P1, DeviceID: "d1", Channel: 9, Enabled: true})
			},
			"角色重复",
		},
		{
			"missing P5",
			func(c *FiveHoleTraversalConfig) { c.Probes[0].ProbeChannels[4].Enabled = false },
			"fiveHole.p5",
		},
		{
			"empty x controllerID",
			func(c *FiveHoleTraversalConfig) { c.Probes[0].MotionX.ControllerID = "" },
			"X方向",
		},
		{
			"empty x axis",
			func(c *FiveHoleTraversalConfig) { c.Probes[0].MotionX.Axis = "" },
			"X方向轴号",
		},
		{
			"empty y controllerID (rectangle)",
			func(c *FiveHoleTraversalConfig) {
				c.Layout = TraversalLayout{
					Pattern: TraversalPatternRectangle,
					Rectangle: &RectangleLayout{
						XMin: 0, XMax: 10, YMin: 0, YMax: 10,
						XSteps: []StepSegment{{Start: 0, End: 10, Step: 10}},
						YSteps: []StepSegment{{Start: 0, End: 10, Step: 10}},
					},
				}
				c.Probes[0].MotionY.ControllerID = ""
			},
			"Y方向",
		},
		{
			"empty y axis (rectangle)",
			func(c *FiveHoleTraversalConfig) {
				c.Layout = TraversalLayout{
					Pattern: TraversalPatternRectangle,
					Rectangle: &RectangleLayout{
						XMin: 0, XMax: 10, YMin: 0, YMax: 10,
						XSteps: []StepSegment{{Start: 0, End: 10, Step: 10}},
						YSteps: []StepSegment{{Start: 0, End: 10, Step: 10}},
					},
				}
				c.Probes[0].MotionY.Axis = ""
			},
			"Y方向轴号",
		},
		{
			"no calib files",
			func(c *FiveHoleTraversalConfig) { c.Probes[0].CalibFiles = nil },
			"校准文件",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validFiveHoleConfig()
			tc.mutate(&c)
			err := c.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("expected error containing %q, got %v", tc.wantSub, err)
			}
		})
	}
}

func TestFiveHoleValidate_TooManyProbes(t *testing.T) {
	c := validFiveHoleConfig()
	// 添加到 4 个启用探针
	for i := 2; i <= 4; i++ {
		p := c.Probes[0]
		p.ProbeID = "probe" + string(rune('0'+i))
		c.Probes = append(c.Probes, p)
	}
	err := c.Validate()
	if err == nil || !strings.Contains(err.Error(), "最多启用3根") {
		t.Fatalf("expected 最多启用3根 error, got %v", err)
	}
}

func TestFiveHoleValidate_NoEnabledProbes(t *testing.T) {
	c := validFiveHoleConfig()
	c.Probes[0].Enabled = false
	err := c.Validate()
	if err == nil || !strings.Contains(err.Error(), "至少1根") {
		t.Fatalf("expected 至少1根 error, got %v", err)
	}
}

// TestFiveHoleValidate_TTotal_Optional TTotal 未配置时应通过（回退用 TAtm）
func TestFiveHoleValidate_TTotal_Optional(t *testing.T) {
	c := validFiveHoleConfig()
	// 默认 TTotalDeviceID 为空，应通过
	if err := c.Validate(); err != nil {
		t.Fatalf("TTotal 未配置时应通过，got %v", err)
	}
	// 显式配置 TTotal 也应通过
	c.TTotalDeviceID = "dev-ttotal"
	c.TTotalChannel = 5
	if err := c.Validate(); err != nil {
		t.Fatalf("TTotal 已配置且通道合法时应通过，got %v", err)
	}
}

func TestFiveHoleValidate_Layout(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*FiveHoleTraversalConfig)
		wantSub string
	}{
		{
			"line nil",
			func(c *FiveHoleTraversalConfig) {
				c.Layout = TraversalLayout{Pattern: TraversalPatternLine}
			},
			"Line配置",
		},
		{
			"rectangle XMin>XMax",
			func(c *FiveHoleTraversalConfig) {
				c.Layout = TraversalLayout{
					Pattern:   TraversalPatternRectangle,
					Rectangle: &RectangleLayout{XMin: 10, XMax: 5, YMin: 0, YMax: 5},
				}
			},
			"XMin",
		},
		{
			"custom empty",
			func(c *FiveHoleTraversalConfig) {
				c.Layout = TraversalLayout{Pattern: TraversalPatternCustom}
			},
			"自定义布点",
		},
		{
			"unknown pattern",
			func(c *FiveHoleTraversalConfig) {
				c.Layout = TraversalLayout{Pattern: "unknown"}
			},
			"布点模式",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validFiveHoleConfig()
			tc.mutate(&c)
			err := c.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("expected error containing %q, got %v", tc.wantSub, err)
			}
		})
	}
}

// ==================== 五孔共用轴位 ====================

func TestFiveHoleValidate_SharedMotion(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*FiveHoleTraversalConfig)
		wantSub string
	}{
		{
			"共用X方向为空",
			func(c *FiveHoleTraversalConfig) {
				c.SharedMotion = true
				c.SharedMotionY = FiveHoleMotionAxisMapping{ControllerID: "mc-2", Axis: AxisY}
			},
			"共用轴位的X方向",
		},
		{
			"共用Y方向轴号为空",
			func(c *FiveHoleTraversalConfig) {
				c.SharedMotion = true
				c.SharedMotionX = FiveHoleMotionAxisMapping{ControllerID: "mc-2", Axis: AxisX}
				c.SharedMotionY = FiveHoleMotionAxisMapping{ControllerID: "mc-2"}
				// 默认布局为直线（跳过Y校验），切矩形使Y校验生效
				c.Layout = TraversalLayout{
					Pattern: TraversalPatternRectangle,
					Rectangle: &RectangleLayout{
						XMin: 0, XMax: 10, YMin: 0, YMax: 10,
						XSteps: []StepSegment{{Start: 0, End: 10, Step: 10}},
						YSteps: []StepSegment{{Start: 0, End: 10, Step: 10}},
					},
				}
			},
			"共用轴位的Y方向",
		},
		{
			"共用X/Y指向同一根物理轴",
			func(c *FiveHoleTraversalConfig) {
				c.SharedMotion = true
				c.SharedMotionX = FiveHoleMotionAxisMapping{ControllerID: "mc-2", Axis: AxisX}
				c.SharedMotionY = FiveHoleMotionAxisMapping{ControllerID: "mc-2", Axis: AxisX}
				c.Layout = TraversalLayout{
					Pattern: TraversalPatternRectangle,
					Rectangle: &RectangleLayout{
						XMin: 0, XMax: 10, YMin: 0, YMax: 10,
						XSteps: []StepSegment{{Start: 0, End: 10, Step: 10}},
						YSteps: []StepSegment{{Start: 0, End: 10, Step: 10}},
					},
				}
			},
			"同一根物理轴",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validFiveHoleConfig()
			tc.mutate(&c)
			err := c.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("expected error containing %q, got %v", tc.wantSub, err)
			}
		})
	}
}

// 直线布点只走X单轴：共用Y方向为空也应通过（与UI直线模式隐藏Y选择器一致）
func TestFiveHoleValidate_SharedMotion_LineSkipsY(t *testing.T) {
	c := validFiveHoleConfig()
	c.SharedMotion = true
	c.SharedMotionX = FiveHoleMotionAxisMapping{ControllerID: "mc-2", Axis: AxisX}
	c.SharedMotionY = FiveHoleMotionAxisMapping{} // 直线模式无需Y
	c.Layout = TraversalLayout{
		Pattern: TraversalPatternLine,
		Line:    &LineLayout{Axis: LineAxisX, Start: 0, End: 10, Step: 1, Fixed: 0},
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("expected nil for line pattern with empty shared Y, got %v", err)
	}
}

// 共用轴位模式下探针独立轴位为空也能通过（被全局轴位覆盖），且原配置不被修改
func TestFiveHoleValidate_SharedMotionOverridesEmptyProbeAxes(t *testing.T) {
	c := validFiveHoleConfig()
	c.SharedMotion = true
	c.SharedMotionX = FiveHoleMotionAxisMapping{ControllerID: "mc-2", Axis: AxisX}
	c.SharedMotionY = FiveHoleMotionAxisMapping{ControllerID: "mc-2", Axis: AxisY}
	// 探针独立轴位清空（前端灰显未配置场景）
	c.Probes[0].MotionX = FiveHoleMotionAxisMapping{}
	c.Probes[0].MotionY = FiveHoleMotionAxisMapping{}
	if err := c.Validate(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if c.Probes[0].MotionX != (FiveHoleMotionAxisMapping{}) || c.Probes[0].MotionY != (FiveHoleMotionAxisMapping{}) {
		t.Fatalf("original config must not be mutated, got %+v/%+v", c.Probes[0].MotionX, c.Probes[0].MotionY)
	}
}

func TestApplySharedMotion(t *testing.T) {
	sharedX := FiveHoleMotionAxisMapping{ControllerID: "mc-s", Axis: AxisX}
	sharedY := FiveHoleMotionAxisMapping{ControllerID: "mc-s", Axis: AxisY}

	t.Run("未启用时原样返回", func(t *testing.T) {
		c := validFiveHoleConfig()
		got := c.ApplySharedMotion()
		if got.Probes[0].MotionX.ControllerID != "mc-1" {
			t.Fatalf("expected probe axes unchanged, got %+v", got.Probes[0].MotionX)
		}
	})

	t.Run("仅覆盖启用探针且不改原配置", func(t *testing.T) {
		c := validFiveHoleConfig()
		c.Probes = append(c.Probes, FiveHoleProbeConfig{
			ProbeID:       "probe2",
			Enabled:       false,
			ProbeChannels: c.Probes[0].ProbeChannels,
			MotionX:       FiveHoleMotionAxisMapping{ControllerID: "keep", Axis: AxisX},
			MotionY:       FiveHoleMotionAxisMapping{ControllerID: "keep", Axis: AxisY},
			CalibFiles:    c.Probes[0].CalibFiles,
		})
		c.SharedMotion = true
		c.SharedMotionX = sharedX
		c.SharedMotionY = sharedY

		got := c.ApplySharedMotion()
		if got.Probes[0].MotionX != sharedX || got.Probes[0].MotionY != sharedY {
			t.Fatalf("enabled probe axes should be overridden, got %+v/%+v", got.Probes[0].MotionX, got.Probes[0].MotionY)
		}
		if got.Probes[1].MotionX.ControllerID != "keep" {
			t.Fatalf("disabled probe axes should stay, got %+v", got.Probes[1].MotionX)
		}
		if c.Probes[0].MotionX.ControllerID != "mc-1" {
			t.Fatalf("original config must not be mutated, got %+v", c.Probes[0].MotionX)
		}
	})
}
