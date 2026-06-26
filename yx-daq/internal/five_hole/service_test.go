package five_hole

import (
	"os"
	"path/filepath"
	"testing"

	"yx-daq/internal/types"
)

// MockEventPublisher 用于测试的模拟事件发布器（五孔）
type MockEventPublisher struct {
	progressEvents []types.FiveHoleTraversalProgressEvent
	completeEvents []types.FiveHoleTraversalCompleteEvent
	errorEvents    []types.FiveHoleTraversalErrorEvent
	realtimeEvents []types.FiveHoleTraversalRealtimeEvent
}

func (m *MockEventPublisher) EmitProgress(event types.FiveHoleTraversalProgressEvent) {
	m.progressEvents = append(m.progressEvents, event)
}

func (m *MockEventPublisher) EmitRealtime(event types.FiveHoleTraversalRealtimeEvent) {
	m.realtimeEvents = append(m.realtimeEvents, event)
}

func (m *MockEventPublisher) EmitComplete(event types.FiveHoleTraversalCompleteEvent) {
	m.completeEvents = append(m.completeEvents, event)
}

func (m *MockEventPublisher) EmitError(event types.FiveHoleTraversalErrorEvent) {
	m.errorEvents = append(m.errorEvents, event)
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
		MotionAlpha: types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "X"},
		MotionBeta:  types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "Y"},
		CalibFiles: []types.FiveHoleCalibFileInfo{
			{FilePath: "fake.prb", FileName: "fake.prb", CMa: 0.5},
		},
	}
}

// writeTestPrbFile 在临时目录写入一个测试用 .prb 校准文件，返回路径
func writeTestPrbFile(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	prbPath := filepath.Join(tmpDir, "test.prb")
	content := `0.5
0.1 0.2 1.0 0.9 5.0 1.0
0.2 0.3 1.1 0.95 10.0 2.0
0.3 0.4 1.2 1.0 15.0 3.0
`
	if err := os.WriteFile(prbPath, []byte(content), 0644); err != nil {
		t.Fatalf("write test prb failed: %v", err)
	}
	return prbPath
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

	prbPath := writeTestPrbFile(t)

	// 载入前应为未加载
	if service.IsCalibLoaded("probe1") {
		t.Fatal("probe1 载入前不应为已加载")
	}
	if service.IsCalibLoaded("probe2") {
		t.Fatal("probe2 载入前不应为已加载")
	}

	// probe1 独立载入
	if err := service.LoadCalibFiles("probe1", []string{prbPath}); err != nil {
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
	if err := service.LoadCalibFiles("probe2", []string{prbPath}); err != nil {
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
