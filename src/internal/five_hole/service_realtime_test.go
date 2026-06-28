package five_hole

import (
	"path/filepath"
	"testing"
	"time"

	"yx-daq/internal/types"
)

// makeRealtimeBatchGetter5H 返回一个能提供全部通道数据的五孔 multi-device batchGetter
// 设备ID：devPAtm(PAtm) / devTAtm(TAtm) / d1(probe1 P1-P5) / d2(probe2 P1-P5)
func makeRealtimeBatchGetter5H() FiveHoleMultiDeviceBatchGetter {
	return func(deviceID string, channels []int) (map[int]float64, int64, error) {
		result := make(map[int]float64)
		switch deviceID {
		case "devPAtm":
			result[0] = 101.325
		case "devTAtm":
			result[0] = 20.5
		case "d1":
			result[0] = 100.0
			result[1] = 101.0
			result[2] = 99.0
			result[3] = 100.5
			result[4] = 99.5
		case "d2":
			result[10] = 200.0
			result[11] = 201.0
			result[12] = 199.0
			result[13] = 200.5
			result[14] = 199.5
		}
		return result, time.Now().UnixMilli(), nil
	}
}

// makeRealtimeProbe5H 构造一个用于 realtime 监控测试的启用探针配置（deviceID=d1）
func makeRealtimeProbe5H(probeID string) types.FiveHoleProbeConfig {
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
	}
}

// makeRealtimeConfig5H 构造一个用于 realtime 监控测试的五孔配置
func makeRealtimeConfig5H(t *testing.T, probes ...types.FiveHoleProbeConfig) types.FiveHoleTraversalConfig {
	t.Helper()
	cfg := makeValidFiveHoleConfig(t, probes...)
	// 覆盖 PAtm/TAtm 设备ID 以匹配 makeRealtimeBatchGetter5H
	cfg.PAtmDeviceID = "devPAtm"
	cfg.TAtmDeviceID = "devTAtm"
	return cfg
}

// TestService_StartRealtimeMonitor_EmitsRealtimeEvents 启动监控后应发射 realtime 事件
func TestService_StartRealtimeMonitor_EmitsRealtimeEvents(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := NewFiveHoleTraversalService(publisher)
	service.SetMultiDeviceBatchGetter(makeRealtimeBatchGetter5H())

	config := makeRealtimeConfig5H(t, makeRealtimeProbe5H("probe1"))
	service.StartRealtimeMonitor(config)

	// 等待至少 2 个 ticker 周期（100ms × 2 + 缓冲）
	time.Sleep(350 * time.Millisecond)

	service.StopRealtimeMonitor()

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
		if len(evt.ProbeRealtime) != 1 {
			t.Errorf("ProbeRealtime 应有 1 个探针，实际 %d", len(evt.ProbeRealtime))
		}
		if len(evt.ProbeRealtime) > 0 && evt.ProbeRealtime[0].ProbeID != "probe1" {
			t.Errorf("ProbeID 应为 probe1，实际 %s", evt.ProbeRealtime[0].ProbeID)
		}
		if len(evt.ProbeRealtime) > 0 && evt.ProbeRealtime[0].RawData.P1 != 100.0 {
			t.Errorf("P1 应为 100.0，实际 %f", evt.ProbeRealtime[0].RawData.P1)
		}
	}
}

// TestService_StartRealtimeMonitor_MultiProbe 单事件含多探针
func TestService_StartRealtimeMonitor_MultiProbe(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := NewFiveHoleTraversalService(publisher)
	service.SetMultiDeviceBatchGetter(makeRealtimeBatchGetter5H())

	// 两个探针：probe1(d1) 和 probe2(d2)
	probe2 := makeRealtimeProbe5H("probe2")
	probe2.ProbeChannels = []types.FiveHoleProbeChannelConfig{
		{Role: types.Role5H_P1, DeviceID: "d2", Channel: 10, Enabled: true},
		{Role: types.Role5H_P2, DeviceID: "d2", Channel: 11, Enabled: true},
		{Role: types.Role5H_P3, DeviceID: "d2", Channel: 12, Enabled: true},
		{Role: types.Role5H_P4, DeviceID: "d2", Channel: 13, Enabled: true},
		{Role: types.Role5H_P5, DeviceID: "d2", Channel: 14, Enabled: true},
	}

	config := makeRealtimeConfig5H(t, makeRealtimeProbe5H("probe1"), probe2)
	service.StartRealtimeMonitor(config)

	time.Sleep(350 * time.Millisecond)
	service.StopRealtimeMonitor()

	events := publisher.GetRealtimeEvents()
	if len(events) == 0 {
		t.Fatal("应发射 realtime 事件")
	}

	// 每个事件应包含 2 个探针数据
	evt := events[0]
	if len(evt.ProbeRealtime) != 2 {
		t.Errorf("ProbeRealtime 应有 2 个探针，实际 %d", len(evt.ProbeRealtime))
	}
}

// TestService_StartRealtimeMonitor_Idempotent_UpdatesConfig 已运行时再次 Start 仅更新配置
func TestService_StartRealtimeMonitor_Idempotent_UpdatesConfig(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := NewFiveHoleTraversalService(publisher)
	service.SetMultiDeviceBatchGetter(makeRealtimeBatchGetter5H())

	config1 := makeRealtimeConfig5H(t, makeRealtimeProbe5H("probe1"))
	config1.DwellTimeMs = 100
	service.StartRealtimeMonitor(config1)
	defer service.StopRealtimeMonitor()

	// 再次 Start（应仅更新配置，不启动新 goroutine）
	config2 := makeRealtimeConfig5H(t, makeRealtimeProbe5H("probe1"))
	config2.DwellTimeMs = 500
	service.StartRealtimeMonitor(config2)

	time.Sleep(250 * time.Millisecond)
	service.StopRealtimeMonitor()

	// 验证配置已更新
	if service.monitorConfig.DwellTimeMs != 500 {
		t.Errorf("DwellTimeMs 应为 500，实际 %d", service.monitorConfig.DwellTimeMs)
	}

	events := publisher.GetRealtimeEvents()
	if len(events) == 0 {
		t.Error("应发射 realtime 事件")
	}
}

// TestService_StopRealtimeMonitor_StopsEmitting 停止后不再发射事件
func TestService_StopRealtimeMonitor_StopsEmitting(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := NewFiveHoleTraversalService(publisher)
	service.SetMultiDeviceBatchGetter(makeRealtimeBatchGetter5H())

	config := makeRealtimeConfig5H(t, makeRealtimeProbe5H("probe1"))
	service.StartRealtimeMonitor(config)

	time.Sleep(250 * time.Millisecond)
	service.StopRealtimeMonitor()

	countAfterStop := publisher.RealtimeCount()

	// 等待一段时间，确认没有新事件
	time.Sleep(300 * time.Millisecond)

	countAfterWait := publisher.RealtimeCount()
	if countAfterWait != countAfterStop {
		t.Errorf("停止后不应再发射事件，停止时 %d 个，等待后 %d 个", countAfterStop, countAfterWait)
	}
}

// TestService_StartRealtimeRecording_DelegatesToRecorder StartRealtimeRecording 委托给 RealtimeRecorder
func TestService_StartRealtimeRecording_DelegatesToRecorder(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := NewFiveHoleTraversalService(publisher)

	dir := t.TempDir()
	path := filepath.Join(dir, "rt5h.csv")

	if err := service.StartRealtimeRecording(path); err != nil {
		t.Fatalf("StartRealtimeRecording failed: %v", err)
	}

	if !service.IsRealtimeRecording() {
		t.Error("StartRealtimeRecording 后 IsRealtimeRecording 应为 true")
	}

	service.StopRealtimeRecording()
	if service.IsRealtimeRecording() {
		t.Error("StopRealtimeRecording 后 IsRealtimeRecording 应为 false")
	}
}

// TestService_StopRealtimeRecording_Idempotent 多次 StopRealtimeRecording 不 panic
func TestService_StopRealtimeRecording_Idempotent(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := NewFiveHoleTraversalService(publisher)

	service.StopRealtimeRecording() // 未 Start 时调用
	service.StopRealtimeRecording() // 不应 panic
}

// TestService_IsRealtimeRecording_InitialFalse 初始状态 IsRealtimeRecording 为 false
func TestService_IsRealtimeRecording_InitialFalse(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := NewFiveHoleTraversalService(publisher)

	if service.IsRealtimeRecording() {
		t.Error("初始状态 IsRealtimeRecording 应为 false")
	}
}

// TestService_RunRealtimeMonitor_NoPAtmDevice_SkipsEmit PAtmDeviceID 为空时跳过发射
func TestService_RunRealtimeMonitor_NoPAtmDevice_SkipsEmit(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := NewFiveHoleTraversalService(publisher)
	service.SetMultiDeviceBatchGetter(makeRealtimeBatchGetter5H())

	config := makeRealtimeConfig5H(t, makeRealtimeProbe5H("probe1"))
	config.PAtmDeviceID = "" // 清空 PAtm 设备
	service.StartRealtimeMonitor(config)

	time.Sleep(300 * time.Millisecond)
	service.StopRealtimeMonitor()

	events := publisher.GetRealtimeEvents()
	if len(events) != 0 {
		t.Errorf("PAtmDeviceID 为空时不应发射事件，实际 %d 个", len(events))
	}
}

// TestService_RunRealtimeMonitor_NoTAtmDevice_SkipsEmit TAtmDeviceID 为空时跳过发射
func TestService_RunRealtimeMonitor_NoTAtmDevice_SkipsEmit(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := NewFiveHoleTraversalService(publisher)
	service.SetMultiDeviceBatchGetter(makeRealtimeBatchGetter5H())

	config := makeRealtimeConfig5H(t, makeRealtimeProbe5H("probe1"))
	config.TAtmDeviceID = "" // 清空 TAtm 设备
	service.StartRealtimeMonitor(config)

	time.Sleep(300 * time.Millisecond)
	service.StopRealtimeMonitor()

	events := publisher.GetRealtimeEvents()
	if len(events) != 0 {
		t.Errorf("TAtmDeviceID 为空时不应发射事件，实际 %d 个", len(events))
	}
}

// TestService_RunRealtimeMonitor_NoEnabledProbes_SkipsEmit 无启用探针时跳过发射
func TestService_RunRealtimeMonitor_NoEnabledProbes_SkipsEmit(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := NewFiveHoleTraversalService(publisher)
	service.SetMultiDeviceBatchGetter(makeRealtimeBatchGetter5H())

	// 构造一个有探针但全部禁用的配置
	disabledProbe := makeRealtimeProbe5H("probe1")
	disabledProbe.Enabled = false
	config := makeRealtimeConfig5H(t, disabledProbe)
	service.StartRealtimeMonitor(config)

	time.Sleep(300 * time.Millisecond)
	service.StopRealtimeMonitor()

	events := publisher.GetRealtimeEvents()
	if len(events) != 0 {
		t.Errorf("无启用探针时不应发射事件，实际 %d 个", len(events))
	}
}

// TestService_RunRealtimeMonitor_TestRunning_SkipsEmit testRunning=true 时监控不推送
func TestService_RunRealtimeMonitor_TestRunning_SkipsEmit(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := NewFiveHoleTraversalService(publisher)
	service.SetMultiDeviceBatchGetter(makeRealtimeBatchGetter5H())

	// 标记测试正在运行
	service.testRunning.Store(true)

	config := makeRealtimeConfig5H(t, makeRealtimeProbe5H("probe1"))
	service.StartRealtimeMonitor(config)

	time.Sleep(350 * time.Millisecond)
	service.StopRealtimeMonitor()

	events := publisher.GetRealtimeEvents()
	if len(events) != 0 {
		t.Errorf("testRunning=true 时监控不应推送事件，实际推送 %d 个", len(events))
	}
}

// TestService_StopStart_RapidSwitch_NoRace 快速 Stop+Start 切换不应 race 或死锁
func TestService_StopStart_RapidSwitch_NoRace(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := NewFiveHoleTraversalService(publisher)
	service.SetMultiDeviceBatchGetter(makeRealtimeBatchGetter5H())

	config := makeRealtimeConfig5H(t, makeRealtimeProbe5H("probe1"))

	// 快速 Stop+Start 循环（验证 monitorWg 保护）
	for i := 0; i < 5; i++ {
		service.StartRealtimeMonitor(config)
		time.Sleep(50 * time.Millisecond)
		service.StopRealtimeMonitor()
	}

	// 最终启动并验证正常工作
	service.StartRealtimeMonitor(config)
	time.Sleep(250 * time.Millisecond)
	service.StopRealtimeMonitor()

	events := publisher.GetRealtimeEvents()
	if len(events) == 0 {
		t.Error("快速切换后监控应仍能正常发射事件")
	}
}

// TestService_RealtimeRecording_WithMonitor 监控运行时录制器应同步写入数据
func TestService_RealtimeRecording_WithMonitor(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := NewFiveHoleTraversalService(publisher)
	service.SetMultiDeviceBatchGetter(makeRealtimeBatchGetter5H())

	dir := t.TempDir()
	path := filepath.Join(dir, "rt5h.csv")

	// 开始录制
	if err := service.StartRealtimeRecording(path); err != nil {
		t.Fatalf("StartRealtimeRecording failed: %v", err)
	}

	// 启动监控
	config := makeRealtimeConfig5H(t, makeRealtimeProbe5H("probe1"))
	service.StartRealtimeMonitor(config)

	// 等待事件发射和录制
	time.Sleep(350 * time.Millisecond)

	service.StopRealtimeMonitor()
	service.StopRealtimeRecording()

	// 验证 CSV 文件有数据行
	records := readRealtimeCSV5H(t, path)
	if len(records) < 2 {
		t.Errorf("录制文件应至少包含表头+1行数据，实际 %d 行", len(records))
	}
}

// TestService_RunRealtimeMonitor_NoBatchGetter_NoPanic batchGetter=nil 时监控不崩溃
func TestService_RunRealtimeMonitor_NoBatchGetter_NoPanic(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := NewFiveHoleTraversalService(publisher)
	// 不设置 batchGetter（为 nil）

	config := makeRealtimeConfig5H(t, makeRealtimeProbe5H("probe1"))
	service.StartRealtimeMonitor(config)

	time.Sleep(250 * time.Millisecond)
	service.StopRealtimeMonitor()

	// 不应 panic（ReadAllProbesRawData 会返回错误，emitRealtimeForAllProbes 降级为 debug 日志）
	events := publisher.GetRealtimeEvents()
	if len(events) != 0 {
		t.Errorf("batchGetter=nil 时不应发射事件，实际 %d 个", len(events))
	}
}
