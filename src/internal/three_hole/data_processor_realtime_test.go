package three_hole

import (
	"path/filepath"
	"testing"
	"time"

	"yx-daq/internal/types"
)

// waitForMonitorExit 轮询等待监控 goroutine 退出（monitorRunning 变为 false）
func waitForMonitorExit(dp *DataProcessor, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !dp.monitorRunning.Load() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

// makeRealtimeTestConfig 构造一个用于 realtime 监控测试的三孔配置
func makeRealtimeTestConfig() types.ThreeHoleTraversalConfig {
	return types.ThreeHoleTraversalConfig{
		Name:               "RealtimeTest",
		DeviceID:           "test-device",
		MotionControllerID: "test-motion",
		ProbeChannels: []types.ThreeHoleProbeChannelConfig{
			{Channel: 0, Role: types.Role3H_P1, Enabled: true},
			{Channel: 1, Role: types.Role3H_P2, Enabled: true},
			{Channel: 2, Role: types.Role3H_P3, Enabled: true},
			{Channel: 3, Role: types.Role3H_PAtm, Enabled: true},
			{Channel: 4, Role: types.Role3H_TAtm, Enabled: true},
		},
		MotionAlpha:      types.MotionAxisMapping{Axis: "alpha"},
		MotionBeta:       types.MotionAxisMapping{Axis: "beta"},
		DwellTimeMs:      100,
		SamplesPerPoint:  1,
		SampleIntervalMs: 10,
		MotionTimeoutMs:  1000,
	}
}

// makeFullBatchGetter 返回一个能提供全部通道数据的 batchGetter
func makeFullBatchGetter() ThreeHoleBatchGetter {
	return func(channels []types.ThreeHoleProbeChannelConfig) (map[int]float64, error) {
		return map[int]float64{
			0: 100.0,  // P1
			1: 105.0,  // P2
			2: 102.0,  // P3
			3: 101.325, // PAtm
			4: 25.0,   // TAtm
		}, nil
	}
}

// TestDataProcessor_StartRealtimeMonitor_EmitsRealtimeEvents 启动监控后应发射 realtime 事件
func TestDataProcessor_StartRealtimeMonitor_EmitsRealtimeEvents(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)
	interp := NewThreeHoleInterpolator()
	dp := NewDataProcessor(tm, interp, publisher)
	dp.SetBatchGetter(makeFullBatchGetter())

	config := makeRealtimeTestConfig()
	dp.StartRealtimeMonitor(config)

	// 等待至少 2 个 ticker 周期（100ms × 2 + 缓冲）
	time.Sleep(350 * time.Millisecond)

	dp.StopRealtimeMonitor()
	if !waitForMonitorExit(dp, time.Second) {
		t.Fatal("监控 goroutine 未在超时内退出")
	}

	events := publisher.GetRealtimeEvents()
	if len(events) == 0 {
		t.Error("启动监控后应发射 realtime 事件，但收到 0 个")
	}

	// 验证事件内容
	if len(events) > 0 {
		evt := events[0]
		if evt.TaskID != "monitor" {
			t.Errorf("TaskID 应为 monitor，实际 %s", evt.TaskID)
		}
		if evt.PointID != "realtime" {
			t.Errorf("PointID 应为 realtime，实际 %s", evt.PointID)
		}
		if evt.RawData.P1 != 100.0 {
			t.Errorf("P1 应为 100.0，实际 %f", evt.RawData.P1)
		}
	}
}

// TestDataProcessor_StartRealtimeMonitor_Idempotent 已运行时再次 Start 不启动第二个 goroutine
func TestDataProcessor_StartRealtimeMonitor_Idempotent(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)
	interp := NewThreeHoleInterpolator()
	dp := NewDataProcessor(tm, interp, publisher)
	dp.SetBatchGetter(makeFullBatchGetter())

	config := makeRealtimeTestConfig()
	dp.StartRealtimeMonitor(config)
	defer dp.StopRealtimeMonitor()

	// 再次 Start（应仅更新配置，不启动新 goroutine）
	dp.StartRealtimeMonitor(config)

	time.Sleep(250 * time.Millisecond)
	dp.StopRealtimeMonitor()
	if !waitForMonitorExit(dp, time.Second) {
		t.Fatal("监控 goroutine 未在超时内退出")
	}

	// 验证没有 panic 或异常，事件正常发射
	events := publisher.GetRealtimeEvents()
	if len(events) == 0 {
		t.Error("应发射 realtime 事件")
	}
}

// TestDataProcessor_StopRealtimeMonitor_StopsEmitting 停止后不再发射事件
func TestDataProcessor_StopRealtimeMonitor_StopsEmitting(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)
	interp := NewThreeHoleInterpolator()
	dp := NewDataProcessor(tm, interp, publisher)
	dp.SetBatchGetter(makeFullBatchGetter())

	config := makeRealtimeTestConfig()
	dp.StartRealtimeMonitor(config)

	time.Sleep(250 * time.Millisecond)
	dp.StopRealtimeMonitor()
	if !waitForMonitorExit(dp, time.Second) {
		t.Fatal("监控 goroutine 未在超时内退出")
	}

	countAfterStop := publisher.RealtimeCount()

	// 等待一段时间，确认没有新事件
	time.Sleep(300 * time.Millisecond)

	countAfterWait := publisher.RealtimeCount()
	if countAfterWait != countAfterStop {
		t.Errorf("停止后不应再发射事件，停止时 %d 个，等待后 %d 个", countAfterStop, countAfterWait)
	}
}

// TestDataProcessor_StartRealtimeRecording_DelegatesToRecorder StartRealtimeRecording 委托给 RealtimeRecorder
func TestDataProcessor_StartRealtimeRecording_DelegatesToRecorder(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)
	interp := NewThreeHoleInterpolator()
	dp := NewDataProcessor(tm, interp, publisher)

	dir := t.TempDir()
	path := filepath.Join(dir, "rt.csv")

	if err := dp.StartRealtimeRecording(path); err != nil {
		t.Fatalf("StartRealtimeRecording failed: %v", err)
	}

	if !dp.IsRealtimeRecording() {
		t.Error("StartRealtimeRecording 后 IsRealtimeRecording 应为 true")
	}

	dp.StopRealtimeRecording()
	if dp.IsRealtimeRecording() {
		t.Error("StopRealtimeRecording 后 IsRealtimeRecording 应为 false")
	}
}

// TestDataProcessor_StopRealtimeRecording_Idempotent 多次 StopRealtimeRecording 不 panic
func TestDataProcessor_StopRealtimeRecording_Idempotent(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)
	interp := NewThreeHoleInterpolator()
	dp := NewDataProcessor(tm, interp, publisher)

	dp.StopRealtimeRecording() // 未 Start 时调用
	dp.StopRealtimeRecording() // 不应 panic
}

// TestDataProcessor_IsRealtimeRecording_InitialFalse 初始状态 IsRealtimeRecording 为 false
func TestDataProcessor_IsRealtimeRecording_InitialFalse(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)
	interp := NewThreeHoleInterpolator()
	dp := NewDataProcessor(tm, interp, publisher)

	if dp.IsRealtimeRecording() {
		t.Error("初始状态 IsRealtimeRecording 应为 false")
	}
}

// TestDataProcessor_RunRealtimeMonitor_TestRunning_SkipsEmit testRunning=true 时监控不推送
func TestDataProcessor_RunRealtimeMonitor_TestRunning_SkipsEmit(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)
	interp := NewThreeHoleInterpolator()
	dp := NewDataProcessor(tm, interp, publisher)
	dp.SetBatchGetter(makeFullBatchGetter())

	// 标记测试正在运行
	dp.testRunning.Store(true)

	config := makeRealtimeTestConfig()
	dp.StartRealtimeMonitor(config)

	time.Sleep(350 * time.Millisecond)
	dp.StopRealtimeMonitor()
	if !waitForMonitorExit(dp, time.Second) {
		t.Fatal("监控 goroutine 未在超时内退出")
	}

	events := publisher.GetRealtimeEvents()
	if len(events) != 0 {
		t.Errorf("testRunning=true 时监控不应推送事件，实际推送 %d 个", len(events))
	}
}

// TestDataProcessor_RunRealtimeMonitor_NoBatchGetter_NoPanic batchGetter=nil 时监控不崩溃
func TestDataProcessor_RunRealtimeMonitor_NoBatchGetter_NoPanic(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)
	interp := NewThreeHoleInterpolator()
	dp := NewDataProcessor(tm, interp, publisher)
	// 不设置 batchGetter（为 nil）

	config := makeRealtimeTestConfig()
	dp.StartRealtimeMonitor(config)

	time.Sleep(250 * time.Millisecond)
	dp.StopRealtimeMonitor()
	if !waitForMonitorExit(dp, time.Second) {
		t.Fatal("监控 goroutine 未在超时内退出")
	}

	// 不应 panic，也不应发射事件
	events := publisher.GetRealtimeEvents()
	if len(events) != 0 {
		t.Errorf("batchGetter=nil 时不应发射事件，实际 %d 个", len(events))
	}
}

// TestDataProcessor_RunRealtimeMonitor_MissingChannelData_SkipsEmit 通道数据缺失时不发射
func TestDataProcessor_RunRealtimeMonitor_MissingChannelData_SkipsEmit(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)
	interp := NewThreeHoleInterpolator()
	dp := NewDataProcessor(tm, interp, publisher)

	// batchGetter 返回不完整数据（缺少 P3）
	incompleteGetter := func(channels []types.ThreeHoleProbeChannelConfig) (map[int]float64, error) {
		return map[int]float64{
			0: 100.0,  // P1
			1: 105.0,  // P2
			// P3 (channel 2) 缺失
			3: 101.325, // PAtm
			4: 25.0,   // TAtm
		}, nil
	}
	dp.SetBatchGetter(incompleteGetter)

	config := makeRealtimeTestConfig()
	dp.StartRealtimeMonitor(config)

	time.Sleep(250 * time.Millisecond)
	dp.StopRealtimeMonitor()
	if !waitForMonitorExit(dp, time.Second) {
		t.Fatal("监控 goroutine 未在超时内退出")
	}

	events := publisher.GetRealtimeEvents()
	if len(events) != 0 {
		t.Errorf("通道数据缺失时不应发射事件，实际 %d 个", len(events))
	}
}

// TestDataProcessor_RealtimeRecording_WithMonitor 监控运行时录制器应同步写入数据
func TestDataProcessor_RealtimeRecording_WithMonitor(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)
	interp := NewThreeHoleInterpolator()
	dp := NewDataProcessor(tm, interp, publisher)
	dp.SetBatchGetter(makeFullBatchGetter())

	dir := t.TempDir()
	path := filepath.Join(dir, "rt.csv")

	// 开始录制
	if err := dp.StartRealtimeRecording(path); err != nil {
		t.Fatalf("StartRealtimeRecording failed: %v", err)
	}

	// 启动监控
	config := makeRealtimeTestConfig()
	dp.StartRealtimeMonitor(config)

	// 等待事件发射和录制
	time.Sleep(350 * time.Millisecond)

	dp.StopRealtimeMonitor()
	if !waitForMonitorExit(dp, time.Second) {
		t.Fatal("监控 goroutine 未在超时内退出")
	}
	dp.StopRealtimeRecording()

	// 验证 CSV 文件有数据行
	records := readRealtimeCSV(t, path)
	if len(records) < 2 {
		t.Errorf("录制文件应至少包含表头+1行数据，实际 %d 行", len(records))
	}
}

// TestDataProcessor_StartRealtimeMonitor_UpdatesConfig StartRealtimeMonitor 应更新 testManager.config
func TestDataProcessor_StartRealtimeMonitor_UpdatesConfig(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)
	interp := NewThreeHoleInterpolator()
	dp := NewDataProcessor(tm, interp, publisher)
	dp.SetBatchGetter(makeFullBatchGetter())

	config := makeRealtimeTestConfig()
	config.Name = "UpdatedConfig"
	dp.StartRealtimeMonitor(config)

	// 验证 config 已传递到 testManager
	if dp.testManager.config.Name != "UpdatedConfig" {
		t.Errorf("config.Name 应为 UpdatedConfig，实际 %s", dp.testManager.config.Name)
	}

	dp.StopRealtimeMonitor()
	if !waitForMonitorExit(dp, time.Second) {
		t.Fatal("监控 goroutine 未在超时内退出")
	}
}
