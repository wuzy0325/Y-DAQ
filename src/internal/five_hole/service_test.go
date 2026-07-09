package five_hole

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"yx-daq/internal/types"
)

// MockEventPublisher 用于测试的模拟事件发布器（五孔）
// 含 sync.Mutex 保护，可安全用于 realtime monitor 等并发场景
type MockEventPublisher struct {
	mu             sync.Mutex
	progressEvents []types.FiveHoleTraversalProgressEvent
	completeEvents []types.FiveHoleTraversalCompleteEvent
	errorEvents    []types.FiveHoleTraversalErrorEvent
	realtimeEvents []types.FiveHoleTraversalRealtimeEvent
}

func (m *MockEventPublisher) EmitProgress(event types.FiveHoleTraversalProgressEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.progressEvents = append(m.progressEvents, event)
}

func (m *MockEventPublisher) EmitRealtime(event types.FiveHoleTraversalRealtimeEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.realtimeEvents = append(m.realtimeEvents, event)
}

func (m *MockEventPublisher) EmitComplete(event types.FiveHoleTraversalCompleteEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completeEvents = append(m.completeEvents, event)
}

func (m *MockEventPublisher) EmitError(event types.FiveHoleTraversalErrorEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorEvents = append(m.errorEvents, event)
}

// GetRealtimeEvents 线程安全地获取已收集的 realtime 事件
func (m *MockEventPublisher) GetRealtimeEvents() []types.FiveHoleTraversalRealtimeEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.realtimeEvents
}

// GetErrorEvents 线程安全地获取已收集的 error 事件
func (m *MockEventPublisher) GetErrorEvents() []types.FiveHoleTraversalErrorEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.errorEvents
}

// GetProgressEvents 线程安全地获取已收集的 progress 事件
func (m *MockEventPublisher) GetProgressEvents() []types.FiveHoleTraversalProgressEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.progressEvents
}

// GetCompleteEvents 线程安全地获取已收集的 complete 事件
func (m *MockEventPublisher) GetCompleteEvents() []types.FiveHoleTraversalCompleteEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.completeEvents
}

// RealtimeCount 线程安全地获取 realtime 事件数量
func (m *MockEventPublisher) RealtimeCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.realtimeEvents)
}

// Clear 清空所有已收集事件
func (m *MockEventPublisher) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.progressEvents = nil
	m.completeEvents = nil
	m.errorEvents = nil
	m.realtimeEvents = nil
}

// makeValidFiveHoleConfig 构造一个能通过 Validate() 的最小五孔配置
// 参数：是否在 Probes 中放置启用的探针
func makeValidFiveHoleConfig(t *testing.T, enabledProbes ...types.FiveHoleProbeConfig) types.FiveHoleTraversalConfig {
	t.Helper()
	probes := enabledProbes
	if probes == nil {
		probes = []types.FiveHoleProbeConfig{}
	}
	return types.FiveHoleTraversalConfig{
		Name:             "Test",
		DwellTimeMs:      100,
		SamplesPerPoint:  1,
		SampleIntervalMs: 10,
		MotionTimeoutMs:  1000,
		PAtmDeviceID:     "devP",
		PAtmChannel:      0,
		TAtmDeviceID:     "devT",
		TAtmChannel:      0,
		Probes:           probes,
		Layout: types.TraversalLayout{
			Pattern: types.TraversalPatternRectangle,
			Rectangle: &types.RectangleLayout{
				XMin: 0, XMax: 10, YMin: 0, YMax: 10,
				XSteps: []types.StepSegment{{Start: 0, End: 10, Step: 10}},
				YSteps: []types.StepSegment{{Start: 0, End: 10, Step: 10}},
				XAxis:  "X",
				YAxis:  "Y",
			},
		},
		SavePath:     t.TempDir(),
		SaveFileName: "test",
	}
}

// makeEnabledProbe 构造一个启用的探针配置（含 P1-P5 通道 + 运动轴 + 占位校准文件信息）
func makeEnabledProbe(probeID string) types.FiveHoleProbeConfig {
	return types.FiveHoleProbeConfig{
		ProbeID: probeID,
		Enabled: true,
		ProbeChannels: []types.FiveHoleProbeChannelConfig{
			{Role: types.Role5H_P1, DeviceID: "d1", Channel: 0, Enabled: true},
			{Role: types.Role5H_P2, DeviceID: "d1", Channel: 1, Enabled: true},
			{Role: types.Role5H_P3, DeviceID: "d1", Channel: 2, Enabled: true},
			{Role: types.Role5H_P4, DeviceID: "d1", Channel: 3, Enabled: true},
			{Role: types.Role5H_P5, DeviceID: "d1", Channel: 4, Enabled: true},
		},
		MotionX: types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "X"},
		MotionY: types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "Y"},
		CalibFiles: []types.FiveHoleCalibFileInfo{
			{FilePath: "fake.cal", FileName: "fake.cal", CMa: 0.5},
		},
	}
}

// writeTestCalFile 在临时目录写入一个测试用 .cal 校准文件，返回路径
// 文件名含 Ma0.5 以触发算法包的马赫数解析；内容为 13×13 合成网格。
func writeTestCalFile(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	calPath := filepath.Join(tmpDir, "Ma0.5.cal")
	if err := os.WriteFile(calPath, []byte(syntheticCalLines()), 0644); err != nil {
		t.Fatalf("write test cal failed: %v", err)
	}
	return calPath
}

// TestService_Start_ValidateFail 无探针启用时 Start 应返回错误
func TestService_Start_ValidateFail(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := NewFiveHoleTraversalService(publisher)

	// 无探针配置，Validate 应失败
	config := makeValidFiveHoleConfig(t)
	_, err := service.Start(config)
	if err == nil {
		t.Fatal("无探针启用时 Start 应返回错误")
	}
}

// TestService_Start_NoCalibLoaded 启用探针但未载入校准应返回错误
func TestService_Start_NoCalibLoaded(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := NewFiveHoleTraversalService(publisher)

	// 启用探针但未调用 LoadCalibFiles
	config := makeValidFiveHoleConfig(t, makeEnabledProbe("probe1"))
	_, err := service.Start(config)
	if err == nil {
		t.Fatal("启用探针但未载入校准时 Start 应返回错误")
	}
}

// TestService_LoadCalibFiles_PerProbe LoadCalibFiles(probeID, ...) 各探针独立载入
func TestService_LoadCalibFiles_PerProbe(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := NewFiveHoleTraversalService(publisher)

	calPath := writeTestCalFile(t)

	// 载入前应为未加载
	if service.IsCalibLoaded("probe1") {
		t.Fatal("probe1 载入前不应为已加载")
	}
	if service.IsCalibLoaded("probe2") {
		t.Fatal("probe2 载入前不应为已加载")
	}

	// probe1 独立载入
	if err := service.LoadCalibFiles("probe1", []string{calPath}); err != nil {
		t.Fatalf("LoadCalibFiles probe1 failed: %v", err)
	}
	if !service.IsCalibLoaded("probe1") {
		t.Fatal("probe1 载入后应为已加载")
	}
	// probe2 不应受影响
	if service.IsCalibLoaded("probe2") {
		t.Fatal("probe2 不应被 probe1 的载入影响")
	}

	// probe2 独立载入
	if err := service.LoadCalibFiles("probe2", []string{calPath}); err != nil {
		t.Fatalf("LoadCalibFiles probe2 failed: %v", err)
	}
	if !service.IsCalibLoaded("probe2") {
		t.Fatal("probe2 载入后应为已加载")
	}

	// 校验 GetCalibInfo 各探针独立
	infos1 := service.GetCalibInfo("probe1")
	if len(infos1) != 1 {
		t.Fatalf("probe1 expected 1 calib info, got %d", len(infos1))
	}
	if infos1[0].CMa != 0.5 {
		t.Fatalf("probe1 expected CMa=0.5, got %f", infos1[0].CMa)
	}
	infos2 := service.GetCalibInfo("probe2")
	if len(infos2) != 1 {
		t.Fatalf("probe2 expected 1 calib info, got %d", len(infos2))
	}

	// probe3 未载入
	if service.IsCalibLoaded("probe3") {
		t.Fatal("probe3 未载入应为未加载")
	}
	if service.GetCalibInfo("probe3") != nil {
		t.Fatal("probe3 未载入 GetCalibInfo 应为 nil")
	}
}

// makeFanConfig 构造一个能通过 Validate() 的扇面模式五孔配置
// 探针 MotionX=线性轴 X，MotionY=旋转轴 U
func makeFanConfig(t *testing.T, probes ...types.FiveHoleProbeConfig) types.FiveHoleTraversalConfig {
	t.Helper()
	ps := probes
	if ps == nil {
		ps = []types.FiveHoleProbeConfig{}
	}
	return types.FiveHoleTraversalConfig{
		Name:             "Test",
		DwellTimeMs:      100,
		SamplesPerPoint:  1,
		SampleIntervalMs: 10,
		MotionTimeoutMs:  1000,
		PAtmDeviceID:     "devP",
		PAtmChannel:      0,
		TAtmDeviceID:     "devT",
		TAtmChannel:      0,
		Probes:           ps,
		Layout: types.TraversalLayout{
			Pattern: types.TraversalPatternFan,
			Fan: &types.FanLayout{
				RStart:     0,
				ThetaStart: 0,
				RSteps:     []types.StepSegment{{Start: 0, End: 10, Step: 5}},
				ThetaSteps: []types.StepSegment{{Start: 0, End: 30, Step: 15}},
			},
		},
		SavePath:     t.TempDir(),
		SaveFileName: "test",
	}
}

// TestValidateFanAxisKinds_GetterNil getter 未设置时跳过校验
func TestValidateFanAxisKinds_GetterNil(t *testing.T) {
	service := NewFiveHoleTraversalService(&MockEventPublisher{})
	config := makeFanConfig(t, makeEnabledProbe("probe1"))
	// 未调用 SetAxisKindGetter，应直接返回 nil
	if err := service.validateFanAxisKinds(config); err != nil {
		t.Fatalf("getter 未设置时应跳过校验返回 nil, got: %v", err)
	}
}

// TestValidateFanAxisKinds_LinearRotaryPass MotionX=线性轴、MotionY=旋转轴 → 通过
func TestValidateFanAxisKinds_LinearRotaryPass(t *testing.T) {
	service := NewFiveHoleTraversalService(&MockEventPublisher{})
	// getter: c1 的 X 为线性轴, c1 的 U 为旋转轴
	service.SetAxisKindGetter(func(controllerID string, axis types.AxisName) (types.AxisKind, bool) {
		if controllerID == "c1" && axis == "X" {
			return types.AxisKindLinear, true
		}
		if controllerID == "c1" && axis == "U" {
			return types.AxisKindRotary, true
		}
		return "", false
	})

	probe := makeEnabledProbe("probe1")
	probe.MotionX = types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "X"}
	probe.MotionY = types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "U"}
	config := makeFanConfig(t, probe)

	if err := service.validateFanAxisKinds(config); err != nil {
		t.Fatalf("线性/旋转轴组合应通过校验, got: %v", err)
	}
}

// TestValidateFanAxisKinds_MotionXNotLinear MotionX=旋转轴 → 失败
func TestValidateFanAxisKinds_MotionXNotLinear(t *testing.T) {
	service := NewFiveHoleTraversalService(&MockEventPublisher{})
	service.SetAxisKindGetter(func(controllerID string, axis types.AxisName) (types.AxisKind, bool) {
		if controllerID == "c1" && axis == "U" {
			return types.AxisKindRotary, true
		}
		return "", false
	})

	probe := makeEnabledProbe("probe1")
	probe.MotionX = types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "U"} // 旋转轴
	probe.MotionY = types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "U"}
	config := makeFanConfig(t, probe)

	err := service.validateFanAxisKinds(config)
	if err == nil {
		t.Fatal("MotionX 为旋转轴应返回错误")
	}
}

// TestValidateFanAxisKinds_MotionYNotRotary MotionY=线性轴 → 失败
func TestValidateFanAxisKinds_MotionYNotRotary(t *testing.T) {
	service := NewFiveHoleTraversalService(&MockEventPublisher{})
	service.SetAxisKindGetter(func(controllerID string, axis types.AxisName) (types.AxisKind, bool) {
		if controllerID == "c1" && axis == "X" {
			return types.AxisKindLinear, true
		}
		if controllerID == "c1" && axis == "Y" {
			return types.AxisKindLinear, true
		}
		return "", false
	})

	probe := makeEnabledProbe("probe1")
	probe.MotionX = types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "X"}
	probe.MotionY = types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "Y"} // 线性轴
	config := makeFanConfig(t, probe)

	err := service.validateFanAxisKinds(config)
	if err == nil {
		t.Fatal("MotionY 为线性轴应返回错误")
	}
}

// TestValidateFanAxisKinds_AxisNotFound 控制器/轴未找到 → 失败
func TestValidateFanAxisKinds_AxisNotFound(t *testing.T) {
	service := NewFiveHoleTraversalService(&MockEventPublisher{})
	// getter 永远返回未找到
	service.SetAxisKindGetter(func(controllerID string, axis types.AxisName) (types.AxisKind, bool) {
		return "", false
	})

	probe := makeEnabledProbe("probe1")
	probe.MotionX = types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "X"}
	probe.MotionY = types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "U"}
	config := makeFanConfig(t, probe)

	err := service.validateFanAxisKinds(config)
	if err == nil {
		t.Fatal("轴未找到应返回错误")
	}
}

// TestValidateFanAxisKinds_DisabledProbeSkipped 禁用探针跳过校验
func TestValidateFanAxisKinds_DisabledProbeSkipped(t *testing.T) {
	service := NewFiveHoleTraversalService(&MockEventPublisher{})
	service.SetAxisKindGetter(func(controllerID string, axis types.AxisName) (types.AxisKind, bool) {
		// 仅 c1/X 和 c1/U 通过，其它都未找到
		if controllerID == "c1" && axis == "X" {
			return types.AxisKindLinear, true
		}
		if controllerID == "c1" && axis == "U" {
			return types.AxisKindRotary, true
		}
		return "", false
	})

	probe1 := makeEnabledProbe("probe1")
	probe1.MotionX = types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "X"}
	probe1.MotionY = types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "U"}

	// 禁用探针（轴配置错误，但不应被校验）
	probe2 := makeEnabledProbe("probe2")
	probe2.Enabled = false
	probe2.MotionX = types.FiveHoleMotionAxisMapping{ControllerID: "unknown", Axis: "X"}
	probe2.MotionY = types.FiveHoleMotionAxisMapping{ControllerID: "unknown", Axis: "U"}

	config := makeFanConfig(t, probe1, probe2)

	if err := service.validateFanAxisKinds(config); err != nil {
		t.Fatalf("禁用探针应被跳过, got: %v", err)
	}
}

// TestValidateFanAxisKinds_MultiProbeErrorsMerged 多探针错误合并返回
func TestValidateFanAxisKinds_MultiProbeErrorsMerged(t *testing.T) {
	service := NewFiveHoleTraversalService(&MockEventPublisher{})
	// getter 永远返回线性轴（MotionY 必失败）
	service.SetAxisKindGetter(func(controllerID string, axis types.AxisName) (types.AxisKind, bool) {
		return types.AxisKindLinear, true
	})

	probe1 := makeEnabledProbe("probe1")
	probe1.MotionX = types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "X"}
	probe1.MotionY = types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "U"}

	probe2 := makeEnabledProbe("probe2")
	probe2.MotionX = types.FiveHoleMotionAxisMapping{ControllerID: "c2", Axis: "X"}
	probe2.MotionY = types.FiveHoleMotionAxisMapping{ControllerID: "c2", Axis: "U"}

	config := makeFanConfig(t, probe1, probe2)

	err := service.validateFanAxisKinds(config)
	if err == nil {
		t.Fatal("应返回错误")
	}
	// 两根探针的 MotionY 都应该报错（合并返回）
	errMsg := err.Error()
	if !strings.Contains(errMsg, "probe1") {
		t.Fatalf("错误信息应包含 probe1, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "probe2") {
		t.Fatalf("错误信息应包含 probe2, got: %s", errMsg)
	}
}
