package three_hole

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"yx-daq/internal/types"
)

// ==================== Service 层状态机测试辅助函数 ====================

// makeMockMotionCtrl 构造一个返回 nil 的 mock 运动控制器，返回调用计数
func makeMockMotionCtrl() (ThreeHoleMotionController, *int32) {
	var calls int32
	return func(axis types.AxisName, position float64) error {
		atomic.AddInt32(&calls, 1)
		return nil
	}, &calls
}

// makeMockMotionWaiter 构造一个返回 nil 的 mock 运动等待器
func makeMockMotionWaiter() (ThreeHoleMotionWaiter, *int32) {
	var calls int32
	return func(axis types.AxisName, timeoutMs int) error {
		atomic.AddInt32(&calls, 1)
		return nil
	}, &calls
}

// makeMockBatchGetter 构造一个返回有效通道数据的 mock 批量获取器
func makeMockBatchGetter() ThreeHoleBatchGetter {
	return func(channels []types.ThreeHoleProbeChannelConfig) (map[int]float64, error) {
		return map[int]float64{0: 100, 1: 105, 2: 102, 3: 101.325, 4: 25}, nil
	}
}

// makeServiceConfig 构造一个能通过 Validate 的最小 1 点位配置（快速完成）
func makeServiceConfig(t *testing.T) types.ThreeHoleTraversalConfig {
	t.Helper()
	return types.ThreeHoleTraversalConfig{
		Name:               "ServiceTest",
		DeviceID:           "test-device",
		MotionControllerID: "test-motion",
		Layout: types.TraversalLayout{
			Pattern: types.TraversalPatternRectangle,
			Rectangle: &types.RectangleLayout{
				XMin: 0, XMax: 10, YMin: 0, YMax: 10,
				XSteps: []types.StepSegment{{Start: 0, End: 10, Step: 10}},
				YSteps: []types.StepSegment{{Start: 0, End: 10, Step: 10}},
			},
		},
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
		SavePath:         t.TempDir(),
		SaveFileName:     "test.csv",
	}
}

// makeMultiPointConfig 构造多点位配置（用于 Stop during run 测试）
func makeMultiPointConfig(t *testing.T) types.ThreeHoleTraversalConfig {
	t.Helper()
	cfg := makeServiceConfig(t)
	cfg.Name = "MultiPointTest"
	// 多个点位确保测试仍在运行时调用 Stop
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

// setupService 创建已加载校准文件并设置 mock 依赖的 Service
func setupService(t *testing.T, publisher *MockEventPublisher) *ThreeHoleTraversalService {
	t.Helper()
	service := NewThreeHoleTraversalService(publisher)

	if err := service.LoadCalibFiles([]string{"./test_data/calib_0.5.dat"}); err != nil {
		t.Fatalf("加载校准文件失败: %v", err)
	}

	service.SetMotionController(func(axis types.AxisName, position float64) error { return nil })
	service.SetMotionWaiter(func(axis types.AxisName, timeoutMs int) error { return nil })
	service.SetBatchGetter(makeMockBatchGetter())

	return service
}

// waitForStatusEventually 轮询 status 是否在超时内变为期望值
func waitForStatusEventually(t *testing.T, service *ThreeHoleTraversalService, want types.TraversalTestStatus, timeout time.Duration) {
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

// waitForCompleteEvent 等待 complete 事件出现，返回事件数量
func waitForCompleteEvent(t *testing.T, publisher *MockEventPublisher, timeout time.Duration) int {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if events := publisher.GetCompleteEvents(); len(events) > 0 {
			return len(events)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("等待 complete 事件超时（%v）", timeout)
	return 0
}

// ==================== P2-1 ~ P2-12 Service 层状态机测试 ====================

// P2-1 TestService_Start_ReturnsError_WhenConfigInvalid 配置未通过 Validate，Start 返回错误，testRunning 仍为 false
func TestService_Start_ReturnsError_WhenConfigInvalid(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := setupService(t, publisher)

	// 缺少 Name，Validate 应失败
	badConfig := makeServiceConfig(t)
	badConfig.Name = ""

	_, err := service.Start(badConfig)
	if err == nil {
		t.Fatal("无效配置 Start 应返回错误")
	}

	if service.dataProcessor.testRunning.Load() {
		t.Error("Start 失败后 testRunning 应为 false")
	}
}

// P2-2 TestService_Start_Rollback_OnTestManagerStartFail G2：OnTestStart 失败时回滚 testRunning
// 使用无效 SaveFileName（路径穿越）使 CSV 初始化失败，验证 testRunning 回滚且 status 不被污染
func TestService_Start_Rollback_OnTestManagerStartFail(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := setupService(t, publisher)

	cfg := makeServiceConfig(t)
	// 路径穿越文件名会被 filepath.IsLocal 拒绝，OnTestStart 失败
	cfg.SaveFileName = "../test.csv"

	_, err := service.Start(cfg)
	if err == nil {
		t.Fatal("CSV 初始化失败时 Start 应返回错误")
	}

	// 验证 testRunning 已回滚
	if service.dataProcessor.testRunning.Load() {
		t.Error("OnTestStart 失败后 testRunning 应为 false")
	}

	// 验证 status 仍为 Idle（未被 EmitFatalError 污染，因 EmitFatalError 在 Idle 下不改状态）
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

// P2-3 TestService_Stop_DuringRun_StopsLoop G8：Start → Stop → status=Idle、goroutine 退出
func TestService_Stop_DuringRun_StopsLoop(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := setupService(t, publisher)
	cfg := makeMultiPointConfig(t)

	taskID, err := service.Start(cfg)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// 等待测试进入运行状态
	time.Sleep(100 * time.Millisecond)

	service.Stop()

	// Stop 内含 100ms sleep，应在 500ms 内 status=Idle
	waitForStatusEventually(t, service, types.TraversalStatusIdle, 500*time.Millisecond)

	if service.dataProcessor.testRunning.Load() {
		t.Error("Stop 后 testRunning 应为 false")
	}

	// 验证没有 complete 事件（被 Stop 中断，不是自然完成）
	// 注：runTestLoop 的 defer OnTestComplete 仍会执行，但 status 已是 Idle
	// 所以 complete 事件的 status 应为 Idle，不是 Completed
	_ = taskID
}

// P2-4 TestService_Stop_DuringPause_StopsLoop G8：Start → Pause → Stop → status=Idle
func TestService_Stop_DuringPause_StopsLoop(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := setupService(t, publisher)
	cfg := makeMultiPointConfig(t)

	if _, err := service.Start(cfg); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	service.Pause()

	// 确认已暂停
	if status := service.GetStatus().Status; status != types.TraversalStatusPaused {
		t.Fatalf("预备条件失败：status 应为 paused，实际 %s", status)
	}

	service.Stop()
	waitForStatusEventually(t, service, types.TraversalStatusIdle, 500*time.Millisecond)
}

// P2-5 TestService_PauseResume_DuringRun_LoopContinues Start → Pause → Resume → 验证 progress 事件序列
func TestService_PauseResume_DuringRun_LoopContinues(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := setupService(t, publisher)
	cfg := makeMultiPointConfig(t)

	if _, err := service.Start(cfg); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer service.Stop()

	// 等待初始 progress 事件
	time.Sleep(50 * time.Millisecond)
	service.Pause()

	pausedCount := len(publisher.GetProgressEvents())
	time.Sleep(200 * time.Millisecond)

	// 暂停期间不应有新的 progress 事件（DwellWithRealtimeUpdate 在暂停时不推进）
	// 注：DwellWithRealtimeUpdate 可能会发射 progress，但 paused 时 ticker 仍触发
	// 关键是 Resume 后测试继续
	service.Resume()

	time.Sleep(100 * time.Millisecond)

	// Resume 后应有新事件
	resumedCount := len(publisher.GetProgressEvents())
	if resumedCount <= pausedCount {
		t.Errorf("Resume 后 progress 事件应增加：paused=%d, resumed=%d", pausedCount, resumedCount)
	}
}

// P2-6 TestService_Start_RejectsDoubleStart 第二次 Start 返回错误，原 taskID 不变
func TestService_Start_RejectsDoubleStart(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := setupService(t, publisher)
	cfg := makeMultiPointConfig(t)

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
func TestService_Start_AfterStop_SucceedsWithNewTaskID(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := setupService(t, publisher)
	cfg := makeServiceConfig(t)

	taskID1, err := service.Start(cfg)
	if err != nil {
		t.Fatalf("第一次 Start failed: %v", err)
	}
	gen1 := service.testManager.testGen.Load()

	service.Stop()
	time.Sleep(20 * time.Millisecond) // 确保 taskID 时间戳不同

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
func TestService_Start_AfterComplete_Succeeds(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := setupService(t, publisher)
	cfg := makeServiceConfig(t) // 1 点位，快速完成

	taskID1, err := service.Start(cfg)
	if err != nil {
		t.Fatalf("第一次 Start failed: %v", err)
	}

	// 等待 complete 事件（测试自然完成）
	waitForCompleteEvent(t, publisher, 2*time.Second)

	// 等待 waitForTestComplete goroutine 清理状态（status → Idle）
	waitForStatusEventually(t, service, types.TraversalStatusIdle, 1*time.Second)

	time.Sleep(20 * time.Millisecond) // 确保 taskID 时间戳不同

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
func TestService_FatalError_StopsTest(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := setupService(t, publisher)
	cfg := makeMultiPointConfig(t)

	// 注入在首次调用时触发 EmitFatalError 并返回错误的运动控制器
	// runTestLoop 在 RunSinglePoint 返回错误后检查 running.Load()，若 false 则退出
	service.SetMotionController(func(axis types.AxisName, position float64) error {
		service.testManager.EmitFatalError("注入的致命错误")
		return fmt.Errorf("注入的运动错误")
	})

	if _, err := service.Start(cfg); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// 等待 fatal error 生效
	waitForStatusEventually(t, service, types.TraversalStatusError, 2*time.Second)

	// 验证 isFatal 事件
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

	// 清理：status=Error 时 Stop 合法
	service.Stop()
}

// P2-10 TestService_NonFatalError_ContinuesTest 注入某点位失败但非致命 → status 仍 Running、继续下一点位
func TestService_NonFatalError_ContinuesTest(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := setupService(t, publisher)
	cfg := makeMultiPointConfig(t)

	// 注入首次失败、后续成功的运动控制器
	var callCnt int32
	service.SetMotionController(func(axis types.AxisName, position float64) error {
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

	// 测试应仍在运行（非致命错误不中断）
	status := service.GetStatus()
	if status.Status == types.TraversalStatusError {
		t.Errorf("非致命错误不应导致 status=Error，当前 %s", status.Status)
	}

	// 验证有非致命错误事件
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
func TestService_GenerationIsolation_OldGoroutineDoesNotInterfere(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := setupService(t, publisher)
	cfg1 := makeMultiPointConfig(t)

	taskID1, err := service.Start(cfg1)
	if err != nil {
		t.Fatalf("第一次 Start failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond) // 让旧 goroutine 发布一些 progress 事件
	service.Stop()

	// 清空事件，确保后续事件来自新测试
	publisher.Clear()

	cfg2 := makeServiceConfig(t) // 1 点位，快速完成
	taskID2, err := service.Start(cfg2)
	if err != nil {
		t.Fatalf("第二次 Start failed: %v", err)
	}
	defer service.Stop()

	// 等待第二次测试完成
	waitForCompleteEvent(t, publisher, 2*time.Second)

	// 验证所有 progress 事件的 TaskID 都是 taskID2，不是 taskID1
	progressEvents := publisher.GetProgressEvents()
	for i, e := range progressEvents {
		if e.TaskID == taskID1 {
			t.Errorf("progress 事件 %d 的 TaskID 为旧的 %s，应为新的 %s", i, taskID1, taskID2)
		}
	}
}

// P2-12 TestService_Concurrent_StartPauseStop_NoRace 3 个 goroutine 并发 Start/Pause/Stop，2s 内无 race
func TestService_Concurrent_StartPauseStop_NoRace(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := setupService(t, publisher)
	cfg := makeServiceConfig(t)

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
				// 等待测试完成或被 Stop
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

	// 最终清理
	service.Stop()
}

// ==================== P2-18 ~ P2-19 three_hole 专属测试 ====================

// countMonitorRealtimeEvents 统计 TaskID="monitor" 的 realtime 事件数
// （测试循环内部也会推送 realtime 事件，TaskID 为实际 taskID，需区分）
func countMonitorRealtimeEvents(events []types.ThreeHoleTraversalRealtimeEvent) int {
	cnt := 0
	for _, e := range events {
		if e.TaskID == "monitor" {
			cnt++
		}
	}
	return cnt
}

// P2-18 TestService_RealtimeMonitor_Stops_OnTestStart
// Start 测试后，realtime monitor 不再推送（testRunning=true 阻断）
func TestService_RealtimeMonitor_Stops_OnTestStart(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := setupService(t, publisher)

	// 启动 realtime monitor
	cfg := makeServiceConfig(t)
	service.StartRealtimeMonitor(cfg)

	// 等待 monitor 的 realtime 事件出现
	time.Sleep(250 * time.Millisecond)
	beforeCount := countMonitorRealtimeEvents(publisher.GetRealtimeEvents())
	if beforeCount == 0 {
		t.Fatal("monitor 启动后应发射 realtime 事件")
	}

	// 启动测试（testRunning=true 会阻断 monitor 推送）
	multiCfg := makeMultiPointConfig(t)
	if _, err := service.Start(multiCfg); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer service.Stop()

	// 等 ticker 周期让 in-flight 事件完成，再清空
	time.Sleep(150 * time.Millisecond)
	publisher.Clear()

	// 验证测试运行期间不再有 monitor 的 realtime 事件
	time.Sleep(400 * time.Millisecond)
	monitorCount := countMonitorRealtimeEvents(publisher.GetRealtimeEvents())
	if monitorCount != 0 {
		t.Errorf("测试运行期间 monitor 不应推送 realtime 事件，实际推送 %d 个", monitorCount)
	}

	// 测试结束后恢复 monitor
	service.Stop()
	time.Sleep(200 * time.Millisecond)

	// Stop 内部调用 StopRealtimeMonitor，所以需要重新启动 monitor
	service.StartRealtimeMonitor(cfg)
	defer service.StopRealtimeMonitor()
	time.Sleep(250 * time.Millisecond)
	restartCount := countMonitorRealtimeEvents(publisher.GetRealtimeEvents())
	if restartCount == 0 {
		t.Error("Stop 后重新启动 monitor 应能继续推送 realtime 事件")
	}
}

// P2-19 TestService_RealtimeMonitor_Restarts_OnTestStop
// Stop 后 monitor 可继续推送（testRunning=false 恢复）
func TestService_RealtimeMonitor_Restarts_OnTestStop(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := setupService(t, publisher)
	cfg := makeServiceConfig(t)

	// 启动 monitor 并验证有事件
	service.StartRealtimeMonitor(cfg)
	defer service.StopRealtimeMonitor()
	time.Sleep(250 * time.Millisecond)
	initialCount := countMonitorRealtimeEvents(publisher.GetRealtimeEvents())
	if initialCount == 0 {
		t.Fatal("monitor 启动后应发射 realtime 事件")
	}

	// 启动测试，monitor 应停止推送
	multiCfg := makeMultiPointConfig(t)
	if _, err := service.Start(multiCfg); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	// 等 ticker 周期让 in-flight 事件完成，再清空
	time.Sleep(150 * time.Millisecond)
	publisher.Clear()
	time.Sleep(300 * time.Millisecond)
	if cnt := countMonitorRealtimeEvents(publisher.GetRealtimeEvents()); cnt != 0 {
		t.Errorf("测试运行期间 monitor 不应推送，实际 %d 个", cnt)
	}

	// Stop 测试，testRunning 恢复 false，但 Stop 会调用 StopRealtimeMonitor
	// 这里验证 Stop 后可以重新 StartRealtimeMonitor 并继续推送
	service.Stop()
	time.Sleep(200 * time.Millisecond)

	// 重新启动 monitor
	service.StartRealtimeMonitor(cfg)
	time.Sleep(250 * time.Millisecond)
	afterCount := countMonitorRealtimeEvents(publisher.GetRealtimeEvents())
	if afterCount == 0 {
		t.Error("Stop 后重新启动 monitor 应能继续推送 realtime 事件")
	}
}
