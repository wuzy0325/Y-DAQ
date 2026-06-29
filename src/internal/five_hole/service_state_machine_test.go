package five_hole

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"yx-daq/internal/types"
)

// ==================== 五孔 Service 层状态机测试辅助函数 ====================

// makeMockMover5H 构造一个返回 nil 的 mock 探针单轴运动控制器
func makeMockMover5H() (FiveHoleProbeAxisMover, *int32) {
	var calls int32
	return func(controllerID string, axis types.AxisName, position float64) error {
		atomic.AddInt32(&calls, 1)
		return nil
	}, &calls
}

// makeMockWaiter5H 构造一个返回 nil 的 mock 探针单轴运动等待器
func makeMockWaiter5H() (FiveHoleProbeAxisWaiter, *int32) {
	var calls int32
	return func(controllerID string, axis types.AxisName, timeoutMs int) error {
		atomic.AddInt32(&calls, 1)
		return nil
	}, &calls
}

// makeMockMultiDeviceBatchGetter5H 构造一个返回有效通道数据的 mock 多设备批量获取器
// 设备ID：devP(PAtm) / devT(TAtm) / d1(probe1 P1-P5)
func makeMockMultiDeviceBatchGetter5H() FiveHoleMultiDeviceBatchGetter {
	return func(deviceID string, channels []int) (map[int]float64, int64, error) {
		result := make(map[int]float64)
		switch deviceID {
		case "devP":
			result[0] = 101.325
		case "devT":
			result[0] = 20.5
		case "d1":
			result[0] = 100.0
			result[1] = 101.0
			result[2] = 99.0
			result[3] = 100.5
			result[4] = 99.5
		}
		return result, time.Now().UnixMilli(), nil
	}
}

// makeServiceConfig5H 构造一个能通过 Validate 的最小 1 点位五孔配置（快速完成）
// 使用 makeEnabledProbe（DeviceID=d1）+ devP/devT 大气设备，匹配 makeMockMultiDeviceBatchGetter5H
func makeServiceConfig5H(t *testing.T) types.FiveHoleTraversalConfig {
	t.Helper()
	cfg := makeValidFiveHoleConfig(t, makeEnabledProbe("probe1"))
	cfg.PAtmDeviceID = "devP"
	cfg.TAtmDeviceID = "devT"
	return cfg
}

// makeMultiPointConfig5H 构造多点位五孔配置（用于 Stop during run 测试）
func makeMultiPointConfig5H(t *testing.T) types.FiveHoleTraversalConfig {
	t.Helper()
	cfg := makeServiceConfig5H(t)
	cfg.Name = "MultiPointTest5H"
	cfg.Layout = types.TraversalLayout{
		Pattern: types.TraversalPatternRectangle,
		Rectangle: &types.RectangleLayout{
			XMin: 0, XMax: 90, YMin: 0, YMax: 90,
			XSteps: []types.StepSegment{{Start: 0, End: 90, Step: 10}},
			YSteps: []types.StepSegment{{Start: 0, End: 90, Step: 10}},
		},
	}
	return cfg
}

// setupService5H 创建已加载校准文件并设置 mock 依赖的五孔 Service
func setupService5H(t *testing.T, publisher *MockEventPublisher) *FiveHoleTraversalService {
	t.Helper()
	service := NewFiveHoleTraversalService(publisher)

	// 加载校准文件
	calPath := writeTestCalFile(t)
	if err := service.LoadCalibFiles("probe1", []string{calPath}); err != nil {
		t.Fatalf("加载校准文件失败: %v", err)
	}

	// 设置 mock 依赖
	mover, _ := makeMockMover5H()
	waiter, _ := makeMockWaiter5H()
	service.SetProbeAxisMover(mover)
	service.SetProbeAxisWaiter(waiter)
	service.SetMultiDeviceBatchGetter(makeMockMultiDeviceBatchGetter5H())

	return service
}

// waitForStatusEventually5H 轮询 status 是否在超时内变为期望值
func waitForStatusEventually5H(t *testing.T, service *FiveHoleTraversalService, want types.TraversalTestStatus, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if service.GetStatus().Status == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("等待 status=%s 超时（%v），当前 %s", want, timeout, service.GetStatus().Status)
}

// waitForCompleteEvent5H 等待 complete 事件出现
func waitForCompleteEvent5H(t *testing.T, publisher *MockEventPublisher, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if events := publisher.GetCompleteEvents(); len(events) > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("等待 complete 事件超时（%v）", timeout)
}

// ==================== P2-1 ~ P2-12 五孔 Service 层状态机测试 ====================

// P2-1 TestService_Start_ReturnsError_WhenConfigInvalid 配置未通过 Validate，Start 返回错误
func TestService5H_Start_ReturnsError_WhenConfigInvalid(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := setupService5H(t, publisher)

	badConfig := makeServiceConfig5H(t)
	badConfig.Name = "" // Validate 应失败

	_, err := service.Start(badConfig)
	if err == nil {
		t.Fatal("无效配置 Start 应返回错误")
	}

	if service.testRunning.Load() {
		t.Error("Start 失败后 testRunning 应为 false")
	}
}

// P2-2 TestService_Start_Rollback_OnTestManagerStartFail G2：OnTestStart 失败时回滚 testRunning 和 CSV
// 用一个已存在的文件路径作为 SavePath，使 os.MkdirAll 失败，CSV Initialize 失败
func TestService5H_Start_Rollback_OnTestManagerStartFail(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := setupService5H(t, publisher)

	// 先创建一个普通文件，再把它作为 SavePath（MkdirAll 在文件路径上会失败）
	blockerPath := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blockerPath, []byte("x"), 0644); err != nil {
		t.Fatalf("创建 blocker 文件失败: %v", err)
	}

	cfg := makeServiceConfig5H(t)
	cfg.SavePath = blockerPath // blocker 是文件而非目录，MkdirAll 会失败

	_, err := service.Start(cfg)
	if err == nil {
		t.Fatal("CSV 初始化失败时 Start 应返回错误")
	}

	// 验证 testRunning 已回滚
	if service.testRunning.Load() {
		t.Error("OnTestStart 失败后 testRunning 应为 false")
	}

	// 验证 status 仍为 Idle
	if status := service.GetStatus(); status.Status != types.TraversalStatusIdle {
		t.Errorf("status 应保持 Idle，实际 %s", status.Status)
	}

	// 验证发布了致命错误事件（OnTestStart 内部调用 EmitFatalError）
	errs := publisher.GetErrorEvents()
	if len(errs) == 0 {
		t.Fatal("应发布 error 事件")
	}
	if !errs[len(errs)-1].IsFatal {
		t.Error("最后一个 error 事件应为 IsFatal=true")
	}
}

// P2-3 TestService_Stop_DuringRun_StopsLoop G8：Start → Stop → status=Idle
func TestService5H_Stop_DuringRun_StopsLoop(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := setupService5H(t, publisher)
	cfg := makeMultiPointConfig5H(t)

	if _, err := service.Start(cfg); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	service.Stop()

	waitForStatusEventually5H(t, service, types.TraversalStatusIdle, 500*time.Millisecond)

	if service.testRunning.Load() {
		t.Error("Stop 后 testRunning 应为 false")
	}
}

// P2-4 TestService_Stop_DuringPause_StopsLoop G8：Start → Pause → Stop → status=Idle
func TestService5H_Stop_DuringPause_StopsLoop(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := setupService5H(t, publisher)
	cfg := makeMultiPointConfig5H(t)

	if _, err := service.Start(cfg); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	service.Pause()

	if status := service.GetStatus().Status; status != types.TraversalStatusPaused {
		t.Fatalf("预备条件失败：status 应为 paused，实际 %s", status)
	}

	service.Stop()
	waitForStatusEventually5H(t, service, types.TraversalStatusIdle, 500*time.Millisecond)
}

// P2-5 TestService_PauseResume_DuringRun_LoopContinues Start → Pause → Resume → 验证测试继续
func TestService5H_PauseResume_DuringRun_LoopContinues(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := setupService5H(t, publisher)
	cfg := makeMultiPointConfig5H(t)

	if _, err := service.Start(cfg); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer service.Stop()

	time.Sleep(50 * time.Millisecond)
	service.Pause()

	pausedCount := len(publisher.GetProgressEvents())
	time.Sleep(200 * time.Millisecond)
	service.Resume()

	time.Sleep(100 * time.Millisecond)

	resumedCount := len(publisher.GetProgressEvents())
	if resumedCount <= pausedCount {
		t.Errorf("Resume 后 progress 事件应增加：paused=%d, resumed=%d", pausedCount, resumedCount)
	}
}

// P2-6 TestService_Start_RejectsDoubleStart 第二次 Start 返回错误，原 taskID 不变
func TestService5H_Start_RejectsDoubleStart(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := setupService5H(t, publisher)
	cfg := makeMultiPointConfig5H(t)

	taskID1, err := service.Start(cfg)
	if err != nil {
		t.Fatalf("第一次 Start failed: %v", err)
	}
	defer service.Stop()

	_, err = service.Start(cfg)
	if err == nil {
		t.Fatal("第二次 Start 应被拒绝")
	}

	status := service.GetStatus()
	if status.TaskID != taskID1 {
		t.Errorf("原 taskID 应不变：%s → %s", taskID1, status.TaskID)
	}
}

// P2-7 TestService_Start_AfterStop_SucceedsWithNewTaskID Stop → Start，新 taskID，testGen 递增
func TestService5H_Start_AfterStop_SucceedsWithNewTaskID(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := setupService5H(t, publisher)
	cfg := makeServiceConfig5H(t)

	taskID1, err := service.Start(cfg)
	if err != nil {
		t.Fatalf("第一次 Start failed: %v", err)
	}
	gen1 := service.testManager.testGen.Load()

	service.Stop()
	time.Sleep(20 * time.Millisecond)

	taskID2, err := service.Start(cfg)
	if err != nil {
		t.Fatalf("Stop 后 Start failed: %v", err)
	}
	defer service.Stop()

	if taskID1 == taskID2 {
		t.Errorf("taskID 应不同：%s", taskID1)
	}
	gen2 := service.testManager.testGen.Load()
	if gen2 != gen1+1 {
		t.Errorf("testGen 应递增 %d → %d", gen1, gen2)
	}
}

// P2-8 TestService_Start_AfterComplete_Succeeds 等测试自然完成后 Start 新测试成功
func TestService5H_Start_AfterComplete_Succeeds(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := setupService5H(t, publisher)
	cfg := makeServiceConfig5H(t) // 1 点位，快速完成

	taskID1, err := service.Start(cfg)
	if err != nil {
		t.Fatalf("第一次 Start failed: %v", err)
	}

	waitForCompleteEvent5H(t, publisher, 3*time.Second)
	waitForStatusEventually5H(t, service, types.TraversalStatusIdle, 1*time.Second)

	time.Sleep(20 * time.Millisecond)

	taskID2, err := service.Start(cfg)
	if err != nil {
		t.Fatalf("Complete 后 Start failed: %v", err)
	}
	defer service.Stop()

	if taskID1 == taskID2 {
		t.Errorf("taskID 应不同：%s", taskID1)
	}
}

// P2-9 TestService_FatalError_StopsTest 注入会触发 EmitFatalError 的运动控制器 → status=Error、isFatal 事件
func TestService5H_FatalError_StopsTest(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := setupService5H(t, publisher)
	cfg := makeMultiPointConfig5H(t)

	// 注入在首次调用时触发 EmitFatalError 并返回错误的运动控制器
	service.SetProbeAxisMover(func(controllerID string, axis types.AxisName, position float64) error {
		service.testManager.EmitFatalError("注入的致命错误")
		return fmt.Errorf("注入的运动错误")
	})

	if _, err := service.Start(cfg); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	waitForStatusEventually5H(t, service, types.TraversalStatusError, 3*time.Second)

	errs := publisher.GetErrorEvents()
	foundFatal := false
	for _, e := range errs {
		if e.IsFatal && strings.Contains(e.Error, "注入的致命错误") {
			foundFatal = true
			break
		}
	}
	if !foundFatal {
		t.Errorf("未找到含 '注入的致命错误' 的 isFatal 事件，共 %d 个 error 事件", len(errs))
	}

	service.Stop()
}

// P2-10 TestService_NonFatalError_ContinuesTest 注入某点位失败但非致命 → status 仍 Running、继续下一点位
func TestService5H_NonFatalError_ContinuesTest(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := setupService5H(t, publisher)
	cfg := makeMultiPointConfig5H(t)

	// 注入首次失败、后续成功的运动控制器
	var callCnt int32
	service.SetProbeAxisMover(func(controllerID string, axis types.AxisName, position float64) error {
		n := atomic.AddInt32(&callCnt, 1)
		if n <= 2 { // 第一个点位的 α+β 两次调用都失败
			return fmt.Errorf("非致命运动错误")
		}
		return nil
	})

	if _, err := service.Start(cfg); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer service.Stop()

	time.Sleep(300 * time.Millisecond)

	status := service.GetStatus()
	if status.Status == types.TraversalStatusError {
		t.Errorf("非致命错误不应导致 status=Error，当前 %s", status.Status)
	}

	errs := publisher.GetErrorEvents()
	hasNonFatal := false
	for _, e := range errs {
		if !e.IsFatal {
			hasNonFatal = true
			break
		}
	}
	if !hasNonFatal {
		t.Error("应至少有一个 isFatal=false 的错误事件")
	}
}

// P2-11 TestService_GenerationIsolation_OldGoroutineDoesNotInterfere G4：Start → Stop → 立即 Start → 旧 goroutine 不再发 progress
func TestService5H_GenerationIsolation_OldGoroutineDoesNotInterfere(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := setupService5H(t, publisher)
	cfg1 := makeMultiPointConfig5H(t)

	taskID1, err := service.Start(cfg1)
	if err != nil {
		t.Fatalf("第一次 Start failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	service.Stop()

	publisher.Clear()

	cfg2 := makeServiceConfig5H(t)
	taskID2, err := service.Start(cfg2)
	if err != nil {
		t.Fatalf("第二次 Start failed: %v", err)
	}
	defer service.Stop()

	waitForCompleteEvent5H(t, publisher, 3*time.Second)

	progressEvents := publisher.GetProgressEvents()
	for i, e := range progressEvents {
		if e.TaskID == taskID1 {
			t.Errorf("progress 事件 %d 的 TaskID 为旧的 %s，应为新的 %s", i, taskID1, taskID2)
		}
	}
}

// P2-12 TestService_Concurrent_StartPauseStop_NoRace 3 个 goroutine 并发 Start/Pause/Stop，1s 内无 race
func TestService5H_Concurrent_StartPauseStop_NoRace(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := setupService5H(t, publisher)
	cfg := makeServiceConfig5H(t)

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// goroutine 1: Start 循环
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			if _, err := service.Start(cfg); err == nil {
				time.Sleep(50 * time.Millisecond)
				service.Stop()
			}
		}
	}()

	// goroutine 2: Pause/Resume 循环
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			service.Pause()
			service.Resume()
		}
	}()

	// goroutine 3: GetStatus 循环
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			_ = service.GetStatus()
		}
	}()

	time.Sleep(1 * time.Second)
	close(stop)
	wg.Wait()

	service.Stop()
}

// ==================== P2-13 ~ P2-17 five_hole 专属测试 ====================

// P2-13 TestService5H_Start_Rollback_OnCSVInitFail G2：CSV 初始化失败时 testManager 未进入 Running
// 用一个已存在的文件作为 SavePath，使 MkdirAll 失败 → CSV Initialize 失败 → OnTestStart 失败
func TestService5H_Start_Rollback_OnCSVInitFail(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := setupService5H(t, publisher)

	// blocker 是文件，MkdirAll 会失败
	blockerPath := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blockerPath, []byte("x"), 0644); err != nil {
		t.Fatalf("创建 blocker 文件失败: %v", err)
	}

	cfg := makeServiceConfig5H(t)
	cfg.SavePath = blockerPath

	_, err := service.Start(cfg)
	if err == nil {
		t.Fatal("CSV 初始化失败时 Start 应返回错误")
	}

	// 验证 testRunning 已回滚
	if service.testRunning.Load() {
		t.Error("testRunning 应为 false")
	}

	// 验证 testManager 未进入 Running（status 仍为 Idle）
	if status := service.GetStatus(); status.Status != types.TraversalStatusIdle {
		t.Errorf("testManager status 应为 Idle，实际 %s", status.Status)
	}
}

// P2-14 TestService5H_Start_InitializesProbeStatuses_ForEnabledProbesOnly
// G5：3 个探针（2 启用 1 禁用），Start 后 ProbeStatuses 仅含 2 个，且 ID 匹配
func TestService5H_Start_InitializesProbeStatuses_ForEnabledProbesOnly(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := NewFiveHoleTraversalService(publisher)

	// 加载 3 个探针的校准文件
	calPath := writeTestCalFile(t)
	for _, pid := range []string{"probe1", "probe2", "probe3"} {
		if err := service.LoadCalibFiles(pid, []string{calPath}); err != nil {
			t.Fatalf("LoadCalibFiles(%s) failed: %v", pid, err)
		}
	}

	// probe1/probe2 启用，probe3 禁用
	probe1 := makeEnabledProbe("probe1")
	probe2 := makeEnabledProbe("probe2")
	probe3 := makeEnabledProbe("probe3")
	probe3.Enabled = false

	// probe2 用独立设备 d2，避免共享设备
	probe2.ProbeChannels = []types.FiveHoleProbeChannelConfig{
		{Role: types.Role5H_P1, DeviceID: "d2", Channel: 0, Enabled: true},
		{Role: types.Role5H_P2, DeviceID: "d2", Channel: 1, Enabled: true},
		{Role: types.Role5H_P3, DeviceID: "d2", Channel: 2, Enabled: true},
		{Role: types.Role5H_P4, DeviceID: "d2", Channel: 3, Enabled: true},
		{Role: types.Role5H_P5, DeviceID: "d2", Channel: 4, Enabled: true},
	}

	cfg := makeValidFiveHoleConfig(t, probe1, probe2, probe3)
	cfg.PAtmDeviceID = "devP"
	cfg.TAtmDeviceID = "devT"

	mover, _ := makeMockMover5H()
	waiter, _ := makeMockWaiter5H()
	service.SetProbeAxisMover(mover)
	service.SetProbeAxisWaiter(waiter)
	service.SetMultiDeviceBatchGetter(makeMockMultiDeviceBatchGetter5H())

	if _, err := service.Start(cfg); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer service.Stop()

	status := service.GetStatus()
	if len(status.ProbeStatuses) != 2 {
		t.Fatalf("ProbeStatuses 应有 2 个（仅启用探针），实际 %d", len(status.ProbeStatuses))
	}

	ids := map[string]bool{}
	for _, ps := range status.ProbeStatuses {
		ids[ps.ProbeID] = true
		if ps.Phase != "idle" {
			t.Errorf("探针 %s 初始 Phase 应为 idle，实际 %s", ps.ProbeID, ps.Phase)
		}
	}
	if !ids["probe1"] || !ids["probe2"] {
		t.Errorf("ProbeStatuses 应含 probe1 和 probe2，实际 %v", ids)
	}
	if ids["probe3"] {
		t.Error("禁用的 probe3 不应出现在 ProbeStatuses 中")
	}
}

// P2-15 TestService5H_AutoPause_OnDataStagnant_ThenResume
// G9：构造 batchGetter 始终返回相同 timestamp → WaitForFreshData 2s 超时返回 ErrDataStagnant
// → service 自动 Pause → error 事件含"数据停滞" → Resume 后重新采样
func TestService5H_AutoPause_OnDataStagnant_ThenResume(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := NewFiveHoleTraversalService(publisher)

	calPath := writeTestCalFile(t)
	if err := service.LoadCalibFiles("probe1", []string{calPath}); err != nil {
		t.Fatalf("LoadCalibFiles failed: %v", err)
	}

	mover, _ := makeMockMover5H()
	waiter, _ := makeMockWaiter5H()
	service.SetProbeAxisMover(mover)
	service.SetProbeAxisWaiter(waiter)

	// batchGetter 始终返回固定 timestamp，使第二次采样时 WaitForFreshData 超时
	stagnantGetter := func(deviceID string, channels []int) (map[int]float64, int64, error) {
		result := make(map[int]float64)
		switch deviceID {
		case "devP":
			result[0] = 101.325
		case "devT":
			result[0] = 20.5
		case "d1":
			result[0] = 100.0
			result[1] = 101.0
			result[2] = 99.0
			result[3] = 100.5
			result[4] = 99.5
		}
		return result, 1000, nil // 固定 timestamp
	}
	service.SetMultiDeviceBatchGetter(stagnantGetter)

	cfg := makeServiceConfig5H(t)
	cfg.SamplesPerPoint = 3 // 需要多次采样，触发 WaitForFreshData
	cfg.SampleIntervalMs = 10

	if _, err := service.Start(cfg); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer service.Stop()

	// 等待自动暂停（WaitForFreshData 2s 超时 + 一些缓冲）
	waitForStatusEventually5H(t, service, types.TraversalStatusPaused, 4*time.Second)

	// 验证 error 事件含"数据停滞"
	errs := publisher.GetErrorEvents()
	foundStagnant := false
	for _, e := range errs {
		if strings.Contains(e.Error, "数据停滞") {
			foundStagnant = true
			break
		}
	}
	if !foundStagnant {
		t.Errorf("应发布含'数据停滞'的 error 事件，共 %d 个 error 事件", len(errs))
	}

	// Resume 后测试应继续（切换为正常 batchGetter）
	service.SetMultiDeviceBatchGetter(makeMockMultiDeviceBatchGetter5H())
	service.Resume()
	waitForStatusEventually5H(t, service, types.TraversalStatusRunning, 500*time.Millisecond)
}

// P2-16 TestService5H_MultiProbe_ProbeStatuses_UpdateIndependently
// 多探针场景下，UpdateProbeStatus 对 probe1 不影响 probe2
func TestService5H_MultiProbe_ProbeStatuses_UpdateIndependently(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := NewFiveHoleTraversalService(publisher)

	calPath := writeTestCalFile(t)
	for _, pid := range []string{"probe1", "probe2"} {
		if err := service.LoadCalibFiles(pid, []string{calPath}); err != nil {
			t.Fatalf("LoadCalibFiles(%s) failed: %v", pid, err)
		}
	}

	probe1 := makeEnabledProbe("probe1")
	probe2 := makeEnabledProbe("probe2")
	probe2.ProbeChannels = []types.FiveHoleProbeChannelConfig{
		{Role: types.Role5H_P1, DeviceID: "d2", Channel: 0, Enabled: true},
		{Role: types.Role5H_P2, DeviceID: "d2", Channel: 1, Enabled: true},
		{Role: types.Role5H_P3, DeviceID: "d2", Channel: 2, Enabled: true},
		{Role: types.Role5H_P4, DeviceID: "d2", Channel: 3, Enabled: true},
		{Role: types.Role5H_P5, DeviceID: "d2", Channel: 4, Enabled: true},
	}

	cfg := makeValidFiveHoleConfig(t, probe1, probe2)
	cfg.PAtmDeviceID = "devP"
	cfg.TAtmDeviceID = "devT"

	mover, _ := makeMockMover5H()
	waiter, _ := makeMockWaiter5H()
	service.SetProbeAxisMover(mover)
	service.SetProbeAxisWaiter(waiter)
	// 多设备 batchGetter（支持 d1 和 d2）
	service.SetMultiDeviceBatchGetter(func(deviceID string, channels []int) (map[int]float64, int64, error) {
		result := make(map[int]float64)
		switch deviceID {
		case "devP":
			result[0] = 101.325
		case "devT":
			result[0] = 20.5
		case "d1":
			for _, c := range []int{0, 1, 2, 3, 4} {
				result[c] = 100.0 + float64(c)
			}
		case "d2":
			for _, c := range []int{0, 1, 2, 3, 4} {
				result[c] = 200.0 + float64(c)
			}
		}
		return result, time.Now().UnixMilli(), nil
	})

	if _, err := service.Start(cfg); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer service.Stop()

	// 等待测试进入运行
	time.Sleep(100 * time.Millisecond)

	// 更新 probe1 状态，验证不影响 probe2
	service.testManager.UpdateProbeStatus("probe1", "moving", 5.0, 6.0)

	status := service.GetStatus()
	var p1, p2 *types.FiveHoleProbeStatus
	for i := range status.ProbeStatuses {
		if status.ProbeStatuses[i].ProbeID == "probe1" {
			p1 = &status.ProbeStatuses[i]
		} else if status.ProbeStatuses[i].ProbeID == "probe2" {
			p2 = &status.ProbeStatuses[i]
		}
	}
	if p1 == nil || p2 == nil {
		t.Fatalf("应有两个探针状态，p1=%v p2=%v", p1, p2)
	}
	if p1.Phase != "moving" || p1.CurrentX != 5.0 || p1.CurrentY != 6.0 {
		t.Errorf("probe1 状态未更新: %+v", p1)
	}
	// probe2 不应被影响（CurrentX/CurrentY 仍为 0 或测试中实际值，但 Phase 不应是 "moving"）
	if p2.Phase == "moving" {
		t.Errorf("probe2 Phase 不应被 probe1 的更新影响，实际 %s", p2.Phase)
	}
}

// P2-17 TestService5H_Stop_ClosesAllProbeCSVWriters
// Stop 后所有启用探针的 CSV 文件句柄关闭（用文件重命名测试：Windows 上文件被占用时无法重命名）
func TestService5H_Stop_ClosesAllProbeCSVWriters(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := NewFiveHoleTraversalService(publisher)

	calPath := writeTestCalFile(t)
	for _, pid := range []string{"probe1", "probe2"} {
		if err := service.LoadCalibFiles(pid, []string{calPath}); err != nil {
			t.Fatalf("LoadCalibFiles(%s) failed: %v", pid, err)
		}
	}

	probe1 := makeEnabledProbe("probe1")
	probe2 := makeEnabledProbe("probe2")
	probe2.ProbeChannels = []types.FiveHoleProbeChannelConfig{
		{Role: types.Role5H_P1, DeviceID: "d2", Channel: 0, Enabled: true},
		{Role: types.Role5H_P2, DeviceID: "d2", Channel: 1, Enabled: true},
		{Role: types.Role5H_P3, DeviceID: "d2", Channel: 2, Enabled: true},
		{Role: types.Role5H_P4, DeviceID: "d2", Channel: 3, Enabled: true},
		{Role: types.Role5H_P5, DeviceID: "d2", Channel: 4, Enabled: true},
	}

	cfg := makeValidFiveHoleConfig(t, probe1, probe2)
	cfg.PAtmDeviceID = "devP"
	cfg.TAtmDeviceID = "devT"

	mover, _ := makeMockMover5H()
	waiter, _ := makeMockWaiter5H()
	service.SetProbeAxisMover(mover)
	service.SetProbeAxisWaiter(waiter)
	service.SetMultiDeviceBatchGetter(func(deviceID string, channels []int) (map[int]float64, int64, error) {
		result := make(map[int]float64)
		switch deviceID {
		case "devP":
			result[0] = 101.325
		case "devT":
			result[0] = 20.5
		case "d1":
			for _, c := range []int{0, 1, 2, 3, 4} {
				result[c] = 100.0 + float64(c)
			}
		case "d2":
			for _, c := range []int{0, 1, 2, 3, 4} {
				result[c] = 200.0 + float64(c)
			}
		}
		return result, time.Now().UnixMilli(), nil
	})

	if _, err := service.Start(cfg); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// 等待测试启动并创建 CSV 文件
	time.Sleep(100 * time.Millisecond)
	service.Stop()
	time.Sleep(200 * time.Millisecond) // 等 Stop 的 100ms sleep + 清理

	// 验证两个 CSV 文件都可重命名（说明句柄已关闭）
	for _, pid := range []string{"probe1", "probe2"} {
		original := filepath.Join(cfg.SavePath, fmt.Sprintf("test_%s.csv", pid))
		renamed := filepath.Join(cfg.SavePath, fmt.Sprintf("test_%s.closed.csv", pid))
		if err := os.Rename(original, renamed); err != nil {
			t.Errorf("探针 %s 的 CSV 文件应可重命名（句柄已关闭），但失败: %v", pid, err)
		}
	}
}
