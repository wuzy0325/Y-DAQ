package three_hole

import (
	"sync"
	"testing"
	"time"

	"yx-daq/internal/types"
)

// makeTMConfig 构造仅含 Layout 的最小三孔配置（TestManager.Start 不调用 Validate）
// 用于 TestManager 状态机测试
func makeTMConfig() types.ThreeHoleTraversalConfig {
	return types.ThreeHoleTraversalConfig{
		Name: "TMStateMachineTest",
		Layout: types.TraversalLayout{
			Pattern: types.TraversalPatternRectangle,
			Rectangle: &types.RectangleLayout{
				XMin: 0, XMax: 10, YMin: 0, YMax: 10,
				XSteps: []types.StepSegment{{Start: 0, End: 10, Step: 10}},
				YSteps: []types.StepSegment{{Start: 0, End: 10, Step: 10}},
			},
		},
	}
}

// P1-1 TestEmitFatalError_TransitionsToError G3：status=Error、running=false、paused=false、errorEvents 含 isFatal=true
func TestEmitFatalError_TransitionsToError(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)

	taskID, err := tm.Start(makeTMConfig())
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer tm.Stop()
	tm.Pause() // 先暂停，验证 EmitFatalError 会同时清除 paused

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
}

// P1-2 TestEmitFatalError_FromIdle_NoStateCorruption Idle 下调用 EmitFatalError 不应让 status 卡在非 Idle
// 验证修复：EmitFatalError 仅在 Running/Paused 时切换状态，错误事件仍正常发射
func TestEmitFatalError_FromIdle_NoStateCorruption(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)

	// Idle 下直接调用 EmitFatalError（典型场景：Start 内部失败时调用 OnFatalError）
	tm.EmitFatalError("启动测试失败：xxx")

	if st := tm.GetStatus(); st.Status != types.TraversalStatusIdle {
		t.Errorf("Idle 下调用 EmitFatalError 不应改变 status，实际 %s", st.Status)
	}
	if tm.running.Load() {
		t.Error("running.Load() 应保持 false")
	}
	if tm.paused.Load() {
		t.Error("paused.Load() 应保持 false")
	}

	// 错误事件仍应发射（便于调用方报告错误）
	errs := publisher.GetErrorEvents()
	if len(errs) != 1 {
		t.Fatalf("应仍发射 1 个 error 事件，实际 %d", len(errs))
	}
	if !errs[0].IsFatal {
		t.Error("错误事件 IsFatal 应为 true")
	}
}

// P1-3 TestStop_FromPaused G6：Paused → Stop → Idle
func TestStop_FromPaused(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)

	if _, err := tm.Start(makeTMConfig()); err != nil {
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

// P1-4 TestPause_FromCompleted_NoOp G7：Completed 下 Pause 不改状态
func TestPause_FromCompleted_NoOp(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)

	if _, err := tm.Start(makeTMConfig()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer tm.Stop()
	tm.SetStatus(types.TraversalStatusCompleted)

	tm.Pause()

	if st := tm.GetStatus(); st.Status != types.TraversalStatusCompleted {
		t.Errorf("Completed 下 Pause 不应改变 status，实际 %s", st.Status)
	}
	if tm.paused.Load() {
		t.Error("paused.Load() 应保持 false")
	}
}

// P1-5 TestPause_FromError_NoOp G7：Error 下 Pause 不改状态
func TestPause_FromError_NoOp(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)

	if _, err := tm.Start(makeTMConfig()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer tm.Stop()
	tm.SetStatus(types.TraversalStatusError)

	tm.Pause()

	if st := tm.GetStatus(); st.Status != types.TraversalStatusError {
		t.Errorf("Error 下 Pause 不应改变 status，实际 %s", st.Status)
	}
	if tm.paused.Load() {
		t.Error("paused.Load() 应保持 false")
	}
}

// P1-6 TestStart_AfterCompleted_SucceedsWithNewTaskID Completed → Start 应成功，taskID 不同，testGen+1
func TestStart_AfterCompleted_SucceedsWithNewTaskID(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)

	id1, err := tm.Start(makeTMConfig())
	if err != nil {
		t.Fatalf("第一次 Start failed: %v", err)
	}
	gen1 := tm.testGen.Load()
	tm.SetStatus(types.TraversalStatusCompleted)

	// Completed 状态下应允许重新 Start
	time.Sleep(15 * time.Millisecond) // 保证 taskID 时间戳不同
	id2, err := tm.Start(makeTMConfig())
	if err != nil {
		t.Fatalf("Completed 后 Start failed: %v", err)
	}
	defer tm.Stop()

	if id1 == id2 {
		t.Errorf("taskID 应不同，均=%s", id1)
	}
	gen2 := tm.testGen.Load()
	if gen2 != gen1+1 {
		t.Errorf("testGen 应递增 %d → %d", gen1, gen2)
	}
}

// P1-7 TestStart_AfterError_SucceedsWithNewTaskID Error → Start 应成功
func TestStart_AfterError_SucceedsWithNewTaskID(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)

	id1, err := tm.Start(makeTMConfig())
	if err != nil {
		t.Fatalf("第一次 Start failed: %v", err)
	}
	gen1 := tm.testGen.Load()
	// 通过 EmitFatalError 切到 Error（先暂停再调用，确保状态会切换）
	tm.Pause()
	tm.EmitFatalError("测试错误")

	if st := tm.GetStatus(); st.Status != types.TraversalStatusError {
		t.Fatalf("预备条件失败：status 应为 error，实际 %s", st.Status)
	}

	time.Sleep(15 * time.Millisecond)
	id2, err := tm.Start(makeTMConfig())
	if err != nil {
		t.Fatalf("Error 后 Start failed: %v", err)
	}
	defer tm.Stop()

	if id1 == id2 {
		t.Errorf("taskID 应不同，均=%s", id1)
	}
	gen2 := tm.testGen.Load()
	if gen2 != gen1+1 {
		t.Errorf("testGen 应递增 %d → %d", gen1, gen2)
	}
}

// P1-8 TestConcurrent_PauseResumeStop_DuringRun 启动测试循环后并发 Pause/Resume/Stop，1s 内不 panic 不死锁
// 注：three_hole TestManager 的 Start 不启动测试循环（循环在 Service 层），所以这里只测 TestManager 自身的并发安全
func TestConcurrent_PauseResumeStop_DuringRun(t *testing.T) {
	publisher := &MockEventPublisher{}
	tm := NewTestManager(publisher)

	config := makeTMConfig()

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// goroutine A: Start 循环（成功后关闭 doneCh 避免泄漏）
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
				tm.mu.Lock()
				if tm.doneCh != nil {
					close(tm.doneCh)
					tm.doneCh = nil
				}
				tm.mu.Unlock()
			}
		}
	}()

	// goroutine B/C: Pause/Resume/Stop
	for i := 0; i < 2; i++ {
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

	// goroutine D: 并发 GetStatus
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

	// goroutine E: 并发 CheckCancelled（覆盖 R5 修复：读 tm.ctx）
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			_ = tm.CheckCancelled()
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
