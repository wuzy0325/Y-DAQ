package calibration

import (
	"testing"
	"time"

	"yx-daq/internal/types"
)

// mockEventPublisher 记录事件用于测试验证
type mockEventPublisher struct {
	progressEvents []types.CalibrationProgressEvent
	realtimeEvents []types.CalibrationRealtimeEvent
	completeEvents []types.CalibrationCompleteEvent
}

func (m *mockEventPublisher) EmitProgress(event types.CalibrationProgressEvent) {
	m.progressEvents = append(m.progressEvents, event)
}

func (m *mockEventPublisher) EmitRealtime(event types.CalibrationRealtimeEvent) {
	m.realtimeEvents = append(m.realtimeEvents, event)
}

func (m *mockEventPublisher) EmitComplete(event types.CalibrationCompleteEvent) {
	m.completeEvents = append(m.completeEvents, event)
}

func TestNewCalibrationService(t *testing.T) {
	mock := &mockEventPublisher{}
	svc := NewCalibrationService(mock)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	status := svc.GetStatus()
	if status.Status != "" && status.Status != types.CalibStatusIdle {
		t.Errorf("expected empty or idle status, got %s", status.Status)
	}
}

func TestStartStopLifecycle(t *testing.T) {
	mock := &mockEventPublisher{}
	svc := NewCalibrationService(mock)

	// 设置 mock 运动控制器（不执行实际移动）
	svc.SetMotionController(func(axis types.AxisName, position float64) error {
		return nil
	})

	// 设置 mock 数据获取器
	svc.SetBatchGetter(func(channels []types.ProbeChannelConfig) (map[int]float64, error) {
		result := make(map[int]float64)
		for _, ch := range channels {
			if ch.Enabled {
				result[ch.Channel] = 100.0
			}
		}
		return result, nil
	})

	config := types.CalibrationConfig{
		Type:            types.CalibrationTypeFiveHole,
		DeviceID:        "test-device",
		AlphaAxis:       "X",
		BetaAxis:        "Y",
		DwellTimeMs:     10,
		SamplesPerPoint: 3,
		ProbeChannels: []types.ProbeChannelConfig{
			{Name: "P1", Role: types.RoleP1, Channel: 0, Enabled: true},
			{Name: "P2", Role: types.RoleP2, Channel: 1, Enabled: true},
			{Name: "P3", Role: types.RoleP3, Channel: 2, Enabled: true},
			{Name: "P4", Role: types.RoleP4, Channel: 3, Enabled: true},
			{Name: "P5", Role: types.RoleP5, Channel: 4, Enabled: true},
			{Name: "P∞", Role: types.RolePAtm, Channel: 5, Enabled: true},
			{Name: "T∞", Role: types.RoleTAtm, Channel: 6, Enabled: true},
		},
		Points: []types.CalibrationPoint{
			{ID: "pt-1", Alpha: 0, Beta: 0},
		},
	}

	// Start
	taskID, err := svc.Start(config)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if taskID == "" {
		t.Error("expected non-empty task ID")
	}

	status := svc.GetStatus()
	if status.Status != types.CalibStatusRunning {
		t.Errorf("expected running status, got %s", status.Status)
	}

	// 等待测试完成（1 point × 3 samples × 50ms + 10ms dwell）
	time.Sleep(500 * time.Millisecond)

	status = svc.GetStatus()
	if status.Status != types.CalibStatusCompleted {
		t.Errorf("expected completed status, got %s (points: %d/%d)", status.Status, status.CompletedPoints, status.TotalPoints)
	}
	if len(status.DataPoints) != 1 {
		t.Errorf("expected 1 data point, got %d", len(status.DataPoints))
	}
	if len(mock.completeEvents) != 1 {
		t.Errorf("expected 1 complete event, got %d", len(mock.completeEvents))
	}
}

func TestPauseResume(t *testing.T) {
	mock := &mockEventPublisher{}
	svc := NewCalibrationService(mock)

	svc.SetMotionController(func(axis types.AxisName, position float64) error {
		return nil
	})
	svc.SetBatchGetter(func(channels []types.ProbeChannelConfig) (map[int]float64, error) {
		result := make(map[int]float64)
		for _, ch := range channels {
			if ch.Enabled {
				result[ch.Channel] = 100.0
			}
		}
		return result, nil
	})

	config := types.CalibrationConfig{
		Type:            types.CalibrationTypeFiveHole,
		DeviceID:        "test-device",
		AlphaAxis:       "X",
		BetaAxis:        "Y",
		DwellTimeMs:     10,
		SamplesPerPoint: 3,
		ProbeChannels: []types.ProbeChannelConfig{
			{Name: "P1", Role: types.RoleP1, Channel: 0, Enabled: true},
			{Name: "P∞", Role: types.RolePAtm, Channel: 5, Enabled: true},
		},
		Points: []types.CalibrationPoint{
			{ID: "pt-1", Alpha: 0, Beta: 0},
			{ID: "pt-2", Alpha: 5, Beta: 0},
		},
	}

	svc.Start(config)

	// Pause
	svc.Pause()
	status := svc.GetStatus()
	if status.Status != types.CalibStatusPaused {
		t.Errorf("expected paused status, got %s", status.Status)
	}

	// Resume
	svc.Resume()
	status = svc.GetStatus()
	if status.Status != types.CalibStatusRunning {
		t.Errorf("expected running status after resume, got %s", status.Status)
	}

	// 等待完成
	time.Sleep(800 * time.Millisecond)

	status = svc.GetStatus()
	if status.Status != types.CalibStatusCompleted {
		t.Errorf("expected completed status, got %s", status.Status)
	}
}

func TestStop(t *testing.T) {
	mock := &mockEventPublisher{}
	svc := NewCalibrationService(mock)

	svc.SetMotionController(func(axis types.AxisName, position float64) error {
		return nil
	})
	svc.SetBatchGetter(func(channels []types.ProbeChannelConfig) (map[int]float64, error) {
		result := make(map[int]float64)
		for _, ch := range channels {
			if ch.Enabled {
				result[ch.Channel] = 100.0
			}
		}
		return result, nil
	})

	config := types.CalibrationConfig{
		Type:            types.CalibrationTypeFiveHole,
		DeviceID:        "test-device",
		AlphaAxis:       "X",
		BetaAxis:        "Y",
		DwellTimeMs:     100,
		SamplesPerPoint: 5,
		ProbeChannels: []types.ProbeChannelConfig{
			{Name: "P1", Role: types.RoleP1, Channel: 0, Enabled: true},
		},
		Points: []types.CalibrationPoint{
			{ID: "pt-1", Alpha: 0, Beta: 0},
			{ID: "pt-2", Alpha: 5, Beta: 0},
			{ID: "pt-3", Alpha: 10, Beta: 0},
		},
	}

	svc.Start(config)
	time.Sleep(50 * time.Millisecond)

	svc.Stop()
	status := svc.GetStatus()
	if status.Status != types.CalibStatusIdle {
		t.Errorf("expected idle after stop, got %s", status.Status)
	}
}

func TestStartWhileRunning(t *testing.T) {
	mock := &mockEventPublisher{}
	svc := NewCalibrationService(mock)

	svc.SetMotionController(func(axis types.AxisName, position float64) error {
		return nil
	})
	svc.SetBatchGetter(func(channels []types.ProbeChannelConfig) (map[int]float64, error) {
		result := make(map[int]float64)
		for _, ch := range channels {
			if ch.Enabled {
				result[ch.Channel] = 100.0
			}
		}
		return result, nil
	})

	config := types.CalibrationConfig{
		Type:            types.CalibrationTypeFiveHole,
		DeviceID:        "test-device",
		AlphaAxis:       "X",
		BetaAxis:        "Y",
		DwellTimeMs:     200,
		SamplesPerPoint: 5,
		ProbeChannels: []types.ProbeChannelConfig{
			{Name: "P1", Role: types.RoleP1, Channel: 0, Enabled: true},
		},
		Points: []types.CalibrationPoint{
			{ID: "pt-1", Alpha: 0, Beta: 0},
		},
	}

	_, err := svc.Start(config)
	if err != nil {
		t.Fatalf("first start failed: %v", err)
	}

	_, err = svc.Start(config)
	if err == nil {
		t.Error("expected error when starting while already running")
	}

	svc.Stop()
}

func TestReadRawData(t *testing.T) {
	mock := &mockEventPublisher{}
	svc := NewCalibrationService(mock)
	svc.config = types.CalibrationConfig{
		ProbeChannels: []types.ProbeChannelConfig{
			{Name: "P1", Role: types.RoleP1, Channel: 0, Enabled: true},
			{Name: "P2", Role: types.RoleP2, Channel: 1, Enabled: true},
			{Name: "P∞", Role: types.RolePAtm, Channel: 5, Enabled: true},
			{Name: "Disabled", Role: types.RoleP3, Channel: 2, Enabled: false},
		},
	}

	svc.SetBatchGetter(func(channels []types.ProbeChannelConfig) (map[int]float64, error) {
		return map[int]float64{
			0: 101.0,
			1: 100.5,
			5: 101.325,
		}, nil
	})

	result := svc.readRawData()
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.P1 != 101.0 {
		t.Errorf("P1: expected 101.0, got %.4f", result.P1)
	}
	if result.P2 != 100.5 {
		t.Errorf("P2: expected 100.5, got %.4f", result.P2)
	}
	if result.PAtm != 101.325 {
		t.Errorf("PAtm: expected 101.325, got %.4f", result.PAtm)
	}
	// 禁用的通道应为零值
	if result.P3 != 0 {
		t.Errorf("P3: expected 0 (disabled), got %.4f", result.P3)
	}
}

func TestReadRawData_NilBatchGetter(t *testing.T) {
	mock := &mockEventPublisher{}
	svc := NewCalibrationService(mock)
	result := svc.readRawData()
	if result != nil {
		t.Error("expected nil when batchGetter is nil")
	}
}

func TestAcquireData(t *testing.T) {
	mock := &mockEventPublisher{}
	svc := NewCalibrationService(mock)
	svc.eventPublisher = mock

	svc.config = types.CalibrationConfig{
		SamplesPerPoint: 5,
		ProbeChannels: []types.ProbeChannelConfig{
			{Name: "P1", Role: types.RoleP1, Channel: 0, Enabled: true},
			{Name: "P∞", Role: types.RolePAtm, Channel: 5, Enabled: true},
		},
	}
	svc.status = types.CalibrationTaskStatus{TaskID: "test-task"}

	callCount := 0
	svc.SetBatchGetter(func(channels []types.ProbeChannelConfig) (map[int]float64, error) {
		callCount++
		return map[int]float64{
			0: 100.0 + float64(callCount),
			5: 101.325,
		}, nil
	})

	point := types.CalibrationPoint{ID: "pt-1", Alpha: 0, Beta: 0}
	result := svc.acquireData(point)

	if result.PointID != "pt-1" {
		t.Errorf("PointID: expected pt-1, got %s", result.PointID)
	}
	if result.SampleCount != 5 {
		t.Errorf("SampleCount: expected 5, got %d", result.SampleCount)
	}
	// P1 应该是5次采样 (101, 102, 103, 104, 105) 的平均值 = 103
	if result.RawData.P1 < 102.9 || result.RawData.P1 > 103.1 {
		t.Errorf("P1: expected ~103.0, got %.4f", result.RawData.P1)
	}
}
