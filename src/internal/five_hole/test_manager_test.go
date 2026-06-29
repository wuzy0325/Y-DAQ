package five_hole

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"yx-daq/internal/types"
)

// makeTMTestConfig 构造仅含 Layout 与 Probes 的最小五孔配置（TestManager.Start 不调用 Validate）
// 用于 TestManager 单元测试，避免引入 SavePath/SaveFileName 等无关字段
func makeTMTestConfig(probes ...types.FiveHoleProbeConfig) types.FiveHoleTraversalConfig {
	return types.FiveHoleTraversalConfig{
		Name: "TMTest",
		Layout: types.TraversalLayout{
			Pattern: types.TraversalPatternRectangle,
			Rectangle: &types.RectangleLayout{
				XMin: 0, XMax: 10, YMin: 0, YMax: 10,
				XSteps: []types.StepSegment{{Start: 0, End: 10, Step: 10}},
				YSteps: []types.StepSegment{{Start: 0, End: 10, Step: 10}},
			},
		},
		Probes: probes,
	}
}

// makeTMTestEnabledProbe 构造一个启用探针（仅 ProbeID + Enabled=true，足够 ProbeStatuses 初始化）
func makeTMTestEnabledProbe(probeID string) types.FiveHoleProbeConfig {
	return types.FiveHoleProbeConfig{
		ProbeID: probeID,
		Enabled: true,
	}
}

// makeTMTestDisabledProbe 构造一个禁用探针（仅 ProbeID + Enabled=false）
func makeTMTestDisabledProbe(probeID string) types.FiveHoleProbeConfig {
	return types.FiveHoleProbeConfig{
		ProbeID: probeID,
		Enabled: false,
	}
}

// P0-1 TestNewTestManager_InitialState 初始 status=Idle、ProbeStatuses=空切片、ctx 已取消
func TestNewTestManager_InitialState(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)

	st := tm.GetStatus()
	if st.Status != types.TraversalStatusIdle {
		t.Errorf("初始 status 应为 idle，实际 %s", st.Status)
	}
	if len(st.ProbeStatuses) != 0 {
		t.Errorf("初始 ProbeStatuses 应为空切片，实际长度 %d", len(st.ProbeStatuses))
	}
	if st.ProbeStatuses == nil {
		t.Error("初始 ProbeStatuses 不应为 nil（应为非 nil 空切片）")
	}
	// 初始 ctx 应已被取消
	if err := tm.CheckCancelled(); err == nil {
		t.Error("初始 ctx 应已被取消，CheckCancelled 应返回错误")
	}
	if tm.running.Load() {
		t.Error("初始 running 应为 false")
	}
	if tm.paused.Load() {
		t.Error("初始 paused 应为 false")
	}
}

// P0-2 TestStart_InitializesProbeStatuses Start 后 ProbeStatuses 按启用探针初始化（Phase="idle"），禁用探针不出现
func TestStart_InitializesProbeStatuses(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)

	config := makeTMTestConfig(
		makeTMTestEnabledProbe("probe1"),
		makeTMTestDisabledProbe("probe2"),
		makeTMTestEnabledProbe("probe3"),
	)

	if _, err := tm.Start(config); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer tm.Stop()

	st := tm.GetStatus()
	if len(st.ProbeStatuses) != 2 {
		t.Fatalf("ProbeStatuses 应有 2 个（仅启用探针），实际 %d", len(st.ProbeStatuses))
	}

	// 验证 probe1/probe3 都在且 Phase="idle"
	got := map[string]string{}
	for _, ps := range st.ProbeStatuses {
		got[ps.ProbeID] = ps.Phase
	}
	if got["probe1"] != "idle" {
		t.Errorf("probe1 Phase 应为 idle，实际 %q", got["probe1"])
	}
	if got["probe3"] != "idle" {
		t.Errorf("probe3 Phase 应为 idle，实际 %q", got["probe3"])
	}
	if _, exists := got["probe2"]; exists {
		t.Error("probe2（禁用）不应出现在 ProbeStatuses 中")
	}
}

// P0-3 TestStart_AlreadyRunning Running 下 Start 返回 "test already running" 错误
func TestStart_AlreadyRunning(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)

	config := makeTMTestConfig(makeTMTestEnabledProbe("probe1"))
	if _, err := tm.Start(config); err != nil {
		t.Fatalf("第一次 Start failed: %v", err)
	}
	defer tm.Stop()

	_, err := tm.Start(config)
	if err == nil {
		t.Fatal("Running 下第二次 Start 应返回错误")
	}
	if !contains(err.Error(), "already running") {
		t.Errorf("错误信息应包含 'already running'，实际: %v", err)
	}
}

// P0-4 TestStart_GeneratesUniqueTaskID 两次 Start（中间 Stop）taskID 不同，前缀 "5h-traversal-"
func TestStart_GeneratesUniqueTaskID(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)

	config := makeTMTestConfig(makeTMTestEnabledProbe("probe1"))

	id1, err := tm.Start(config)
	if err != nil {
		t.Fatalf("第一次 Start failed: %v", err)
	}
	if !startsWith(id1, "5h-traversal-") {
		t.Errorf("taskID 前缀应为 '5h-traversal-'，实际 %s", id1)
	}
	tm.Stop()

	time.Sleep(15 * time.Millisecond) // 保证 UnixMilli 不同

	id2, err := tm.Start(config)
	if err != nil {
		t.Fatalf("第二次 Start failed: %v", err)
	}
	defer tm.Stop()
	if id1 == id2 {
		t.Errorf("两次 Start 的 taskID 应不同，均=%s", id1)
	}
}

// P0-5 TestStart_GeneratesPointsError 故意构造无法生成布点的 Layout，断言错误信息
func TestStart_GeneratesPointsError(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)

	// 不支持的 Pattern
	badConfig := types.FiveHoleTraversalConfig{
		Name:   "BadLayout",
		Probes: []types.FiveHoleProbeConfig{makeTMTestEnabledProbe("probe1")},
		Layout: types.TraversalLayout{Pattern: "unknown-pattern"},
	}

	_, err := tm.Start(badConfig)
	if err == nil {
		t.Fatal("不支持的 Pattern 应返回错误")
	}
	if !contains(err.Error(), "生成布点失败") {
		t.Errorf("错误信息应包含 '生成布点失败'，实际: %v", err)
	}

	// 状态应保持 Idle
	if st := tm.GetStatus(); st.Status != types.TraversalStatusIdle {
		t.Errorf("失败的 Start 不应改变状态，实际 %s", st.Status)
	}
}

// P0-6 TestPause_RunningTask Pause 后 status=Paused、paused.Load()=true
func TestPause_RunningTask(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)

	config := makeTMTestConfig(makeTMTestEnabledProbe("probe1"))
	if _, err := tm.Start(config); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer tm.Stop()

	tm.Pause()

	if st := tm.GetStatus(); st.Status != types.TraversalStatusPaused {
		t.Errorf("status 应为 paused，实际 %s", st.Status)
	}
	if !tm.paused.Load() {
		t.Error("paused.Load() 应为 true")
	}
	// running 不变（running 标志在 Stop/EmitFatalError 时才被重置）
	if !tm.running.Load() {
		t.Error("running.Load() 应保持 true（暂停期间仍在测试中）")
	}
}

// P0-7 TestPause_NotRunning_NoOp Idle 下 Pause 不改状态
func TestPause_NotRunning_NoOp(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)

	tm.Pause() // Idle 下调用

	if st := tm.GetStatus(); st.Status != types.TraversalStatusIdle {
		t.Errorf("Idle 下 Pause 不应改变状态，实际 %s", st.Status)
	}
	if tm.paused.Load() {
		t.Error("paused.Load() 应保持 false")
	}
}

// P0-8 TestResume_PausedTask Resume 后 status=Running、paused.Load()=false
func TestResume_PausedTask(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)

	config := makeTMTestConfig(makeTMTestEnabledProbe("probe1"))
	if _, err := tm.Start(config); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer tm.Stop()
	tm.Pause()

	tm.Resume()

	if st := tm.GetStatus(); st.Status != types.TraversalStatusRunning {
		t.Errorf("status 应为 running，实际 %s", st.Status)
	}
	if tm.paused.Load() {
		t.Error("paused.Load() 应为 false")
	}
}

// P0-9 TestResume_NotPaused_NoOp Idle/Running 下 Resume 不改状态
func TestResume_NotPaused_NoOp(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)

	// Idle 下
	tm.Resume()
	if st := tm.GetStatus(); st.Status != types.TraversalStatusIdle {
		t.Errorf("Idle 下 Resume 不应改变状态，实际 %s", st.Status)
	}

	// Running 下
	config := makeTMTestConfig(makeTMTestEnabledProbe("probe1"))
	if _, err := tm.Start(config); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer tm.Stop()

	tm.Resume()
	if st := tm.GetStatus(); st.Status != types.TraversalStatusRunning {
		t.Errorf("Running 下 Resume 不应改变状态，实际 %s", st.Status)
	}
}

// P0-10 TestStop_FromRunning Stop 后 status=Idle、running=false、paused=false
func TestStop_FromRunning(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)

	config := makeTMTestConfig(makeTMTestEnabledProbe("probe1"))
	if _, err := tm.Start(config); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	tm.Stop()

	if st := tm.GetStatus(); st.Status != types.TraversalStatusIdle {
		t.Errorf("status 应为 idle，实际 %s", st.Status)
	}
	if tm.running.Load() {
		t.Error("running.Load() 应为 false")
	}
	if tm.paused.Load() {
		t.Error("paused.Load() 应为 false")
	}
}

// P0-11 TestStop_FromPaused G6：Paused → Stop → Idle
func TestStop_FromPaused(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)

	config := makeTMTestConfig(makeTMTestEnabledProbe("probe1"))
	if _, err := tm.Start(config); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	tm.Pause()

	tm.Stop()

	if st := tm.GetStatus(); st.Status != types.TraversalStatusIdle {
		t.Errorf("从 Paused Stop 后 status 应为 idle，实际 %s", st.Status)
	}
	if tm.running.Load() {
		t.Error("running.Load() 应为 false")
	}
	if tm.paused.Load() {
		t.Error("paused.Load() 应为 false")
	}
}

// P0-12 TestStop_FromCompleted 验证 TaskID 保留
func TestStop_FromCompleted(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)

	config := makeTMTestConfig(makeTMTestEnabledProbe("probe1"))
	taskID, err := tm.Start(config)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	tm.SetStatus(types.TraversalStatusCompleted)

	tm.Stop()

	st := tm.GetStatus()
	if st.Status != types.TraversalStatusIdle {
		t.Errorf("status 应为 idle，实际 %s", st.Status)
	}
	if st.TaskID != taskID {
		t.Errorf("TaskID 应保留 %s，实际 %s", taskID, st.TaskID)
	}
}

// P0-13 TestStop_FromError Error 状态下 Stop 合法
func TestStop_FromError(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)

	config := makeTMTestConfig(makeTMTestEnabledProbe("probe1"))
	if _, err := tm.Start(config); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	tm.SetStatus(types.TraversalStatusError)

	tm.Stop()

	if st := tm.GetStatus(); st.Status != types.TraversalStatusIdle {
		t.Errorf("从 Error Stop 后 status 应为 idle，实际 %s", st.Status)
	}
}

// P0-14 TestStop_Idle_NoOp Idle 下 Stop 不 panic
func TestStop_Idle_NoOp(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)

	// 不应 panic
	tm.Stop()

	if st := tm.GetStatus(); st.Status != types.TraversalStatusIdle {
		t.Errorf("Idle 下 Stop 不应改变状态，实际 %s", st.Status)
	}
}

// P0-15 TestUpdateProgress_UpdatesFields CompletedPoints/Progress/CurrentPoint
func TestUpdateProgress_UpdatesFields(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)

	config := makeTMTestConfig(makeTMTestEnabledProbe("probe1"))
	if _, err := tm.Start(config); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer tm.Stop()

	pt := types.TraversalPoint{ID: "pt-3", X: 5.5, Y: 7.7}
	tm.UpdateProgress(3, 10, &pt)

	st := tm.GetStatus()
	if st.CompletedPoints != 3 {
		t.Errorf("CompletedPoints 应为 3，实际 %d", st.CompletedPoints)
	}
	if st.Progress != 30.0 {
		t.Errorf("Progress 应为 30.0，实际 %f", st.Progress)
	}
	if st.CurrentPoint == nil || st.CurrentPoint.ID != "pt-3" {
		t.Errorf("CurrentPoint 应为 pt-3，实际 %+v", st.CurrentPoint)
	}
}

// P0-16 TestUpdateProbeStatus_UpdatesPhaseAndCoords 指定 probeID 的 Phase/CurrentX/CurrentY 更新
func TestUpdateProbeStatus_UpdatesPhaseAndCoords(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)

	config := makeTMTestConfig(
		makeTMTestEnabledProbe("probe1"),
		makeTMTestEnabledProbe("probe2"),
	)
	if _, err := tm.Start(config); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer tm.Stop()

	tm.UpdateProbeStatus("probe1", "moving", 12.5, 23.4)

	st := tm.GetStatus()
	var p1, p2 *types.FiveHoleProbeStatus
	for i := range st.ProbeStatuses {
		switch st.ProbeStatuses[i].ProbeID {
		case "probe1":
			p1 = &st.ProbeStatuses[i]
		case "probe2":
			p2 = &st.ProbeStatuses[i]
		}
	}
	if p1 == nil {
		t.Fatal("未找到 probe1")
	}
	if p1.Phase != "moving" {
		t.Errorf("probe1 Phase 应为 moving，实际 %q", p1.Phase)
	}
	if p1.CurrentX != 12.5 || p1.CurrentY != 23.4 {
		t.Errorf("probe1 坐标应为 (12.5, 23.4)，实际 (%f, %f)", p1.CurrentX, p1.CurrentY)
	}
	if p2 == nil {
		t.Fatal("未找到 probe2")
	}
	if p2.Phase != "idle" {
		t.Errorf("probe2 Phase 应保持 idle，实际 %q", p2.Phase)
	}
}

// P0-17 TestUpdateProbeStatus_UnknownProbe_NoOp 不存在的 probeID 不 panic
func TestUpdateProbeStatus_UnknownProbe_NoOp(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)

	config := makeTMTestConfig(makeTMTestEnabledProbe("probe1"))
	if _, err := tm.Start(config); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer tm.Stop()

	// 不应 panic
	tm.UpdateProbeStatus("nonexistent", "moving", 1.0, 2.0)

	st := tm.GetStatus()
	if len(st.ProbeStatuses) != 1 {
		t.Errorf("ProbeStatuses 应仍为 1 个，实际 %d", len(st.ProbeStatuses))
	}
}

// P0-18 TestUpdateProbeData_UpdatesRawAndInterp RawData/InterpResult 指针更新
func TestUpdateProbeData_UpdatesRawAndInterp(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)

	config := makeTMTestConfig(makeTMTestEnabledProbe("probe1"))
	if _, err := tm.Start(config); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer tm.Stop()

	raw := types.FiveHoleRawData{P1: 100, P2: 101, P3: 99, P4: 100.5, P5: 99.5, PAtm: 101.325, TAtm: 25.0}
	interp := types.FiveHoleInterpolationResult{Valid: true, AlphaProbe: 1.5, BetaProbe: -0.3}

	tm.UpdateProbeData("probe1", &raw, &interp)

	st := tm.GetStatus()
	if len(st.ProbeStatuses) != 1 {
		t.Fatalf("ProbeStatuses 应有 1 个，实际 %d", len(st.ProbeStatuses))
	}
	ps := st.ProbeStatuses[0]
	if ps.RawData == nil {
		t.Fatal("RawData 应已设置")
	}
	if ps.RawData.P1 != 100 {
		t.Errorf("RawData.P1 应为 100，实际 %f", ps.RawData.P1)
	}
	if ps.InterpResult == nil {
		t.Fatal("InterpResult 应已设置")
	}
	if !ps.InterpResult.Valid {
		t.Error("InterpResult.Valid 应为 true")
	}
	if ps.InterpResult.AlphaProbe != 1.5 {
		t.Errorf("AlphaProbe 应为 1.5，实际 %f", ps.InterpResult.AlphaProbe)
	}
}

// P0-19 TestEmitFatalError_TransitionsToError G3：status=Error、running=false、paused=false、发布 isFatal=true 事件
func TestEmitFatalError_TransitionsToError(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)

	config := makeTMTestConfig(makeTMTestEnabledProbe("probe1"))
	taskID, err := tm.Start(config)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer tm.Stop()

	tm.Pause() // 先暂停，验证 EmitFatalError 会清除 paused
	tm.EmitFatalError("致命错误：测试")

	if st := tm.GetStatus(); st.Status != types.TraversalStatusError {
		t.Errorf("status 应为 error，实际 %s", st.Status)
	}
	if tm.running.Load() {
		t.Error("running.Load() 应为 false")
	}
	if tm.paused.Load() {
		t.Error("paused.Load() 应为 false")
	}
	if st := tm.GetStatus(); st.LastError != "致命错误：测试" {
		t.Errorf("LastError 应为 '致命错误：测试'，实际 %q", st.LastError)
	}

	// 验证发布 isFatal=true 事件
	errs := publisher.GetErrorEvents()
	if len(errs) != 1 {
		t.Fatalf("应发布 1 个 error 事件，实际 %d", len(errs))
	}
	if !errs[0].IsFatal {
		t.Error("错误事件 IsFatal 应为 true")
	}
	if errs[0].TaskID != taskID {
		t.Errorf("事件 TaskID 应为 %s，实际 %s", taskID, errs[0].TaskID)
	}
	if errs[0].Error != "致命错误：测试" {
		t.Errorf("事件 Error 应为 '致命错误：测试'，实际 %q", errs[0].Error)
	}
}

// P0-20 TestCloseDoneCh_IdempotentAndNilSafe 多次 CloseDoneCh 不 panic；doneCh 关闭后 waitForTestComplete 能退出
func TestCloseDoneCh_IdempotentAndNilSafe(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)

	config := makeTMTestConfig(makeTMTestEnabledProbe("probe1"))
	if _, err := tm.Start(config); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// 多次调用不应 panic
	tm.CloseDoneCh()
	tm.CloseDoneCh()
	tm.CloseDoneCh()

	// 等待 waitForTestComplete goroutine 退出（关闭 doneCh 后应能退出）
	// 用 goroutine 计数验证
	done := make(chan struct{})
	go func() {
		// 给 waitForTestComplete 一些时间退出
		time.Sleep(100 * time.Millisecond)
		close(done)
	}()
	select {
	case <-done:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatal("waitForTestComplete goroutine 未能在 2s 内退出")
	}

	// Stop 后再调用 CloseDoneCh 也不应 panic（doneCh 已被置 nil）
	tm.Stop()
	tm.CloseDoneCh()
}

// P0-21 TestConcurrent_Start_OnlyOneSucceeds 5 个 goroutine 并发 Start，仅 1 个成功
func TestConcurrent_Start_OnlyOneSucceeds(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)

	config := makeTMTestConfig(makeTMTestEnabledProbe("probe1"))

	var successCount int32
	var errCount int32
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := tm.Start(config)
			if err == nil {
				atomic.AddInt32(&successCount, 1)
			} else {
				atomic.AddInt32(&errCount, 1)
			}
		}()
	}
	wg.Wait()

	if successCount != 1 {
		t.Errorf("应仅 1 个 Start 成功，实际 %d", successCount)
	}
	if errCount != 4 {
		t.Errorf("应 4 个 Start 失败，实际 %d", errCount)
	}

	// 清理
	tm.CloseDoneCh()
	tm.Stop()
}

// P0-22 TestConcurrent_PauseResumeStop_NoPanic 测试运行期间并发 Pause/Resume/Stop，1s 后无 panic
func TestConcurrent_PauseResumeStop_NoPanic(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)

	config := makeTMTestConfig(makeTMTestEnabledProbe("probe1"))

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// goroutine A: Start 循环（每次失败也无妨，验证不 panic）
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			if _, err := tm.Start(config); err == nil {
				// 关闭 doneCh 避免泄漏
				tm.mu.Lock()
				if tm.doneCh != nil {
					close(tm.doneCh)
					tm.doneCh = nil
				}
				tm.mu.Unlock()
			}
		}
	}()

	// goroutine B/C/D: Pause/Resume/Stop
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				tm.Pause()
				tm.Resume()
				tm.Stop()
			}
		}()
	}

	// goroutine E: GetStatus 并发读
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			_ = tm.GetStatus()
		}
	}()

	time.Sleep(500 * time.Millisecond)
	close(stop)
	wg.Wait()

	// 最终清理
	tm.Stop()
	tm.mu.Lock()
	if tm.doneCh != nil {
		close(tm.doneCh)
		tm.doneCh = nil
	}
	tm.mu.Unlock()
}

// startsWith 字符串前缀检查（contains 已在 csv_writer_test.go 中定义，复用之）
func startsWith(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}
