package three_hole

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"yx-daq/internal/types"
)

// ==================== P3 并发/压力测试 ====================

// P3-1 TestService_RapidStopStart_100Cycles_NoLeak
// 100 次 Stop→Start 循环，goroutine 数不增长（用 runtime.NumGoroutine 前后对比）
func TestService_RapidStopStart_100Cycles_NoLeak(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := setupService(t, publisher)
	cfg := makeServiceConfig(t) // 1 点位，快速完成

	// 预热：跑一次让初始化 goroutine 稳定
	if _, err := service.Start(cfg); err != nil {
		t.Fatalf("预热 Start failed: %v", err)
	}
	service.Stop()
	time.Sleep(200 * time.Millisecond)

	// GC 后记录基线 goroutine 数
	runtime.GC()
	baseline := runtime.NumGoroutine()

	for i := 0; i < 100; i++ {
		if _, err := service.Start(cfg); err != nil {
			// 可能上一次还没完全停止，跳过本次
			continue
		}
		service.Stop()
	}

	// 等待所有 goroutine 退出
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		runtime.GC()
		if runtime.NumGoroutine() <= baseline+2 { // 允许少量波动
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	final := runtime.NumGoroutine()
	if final > baseline+5 {
		t.Errorf("goroutine 泄漏：基线 %d，最终 %d（差值 %d）", baseline, final, final-baseline)
	}
}

// P3-2 TestService_RapidStopStart_20Cycles_NoRace
// 20 次 Stop→Start 循环，-race 下无报告（循环次数较少以缩短 -race 运行时间）
func TestService_RapidStopStart_20Cycles_NoRace(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := setupService(t, publisher)
	cfg := makeServiceConfig(t)

	// 并发：一个 goroutine 做 Stop→Start 循环，另一个做 GetStatus
	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			select {
			case <-stop:
				return
			default:
			}
			if _, err := service.Start(cfg); err == nil {
				service.Stop()
			}
		}
	}()

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

	// 等待 Start/Stop goroutine 完成
	time.Sleep(500 * time.Millisecond)
	close(stop)
	wg.Wait()

	service.Stop()
}

// P3-3 TestService_Stop_ThenImmediateStart_NoDeadlock
// G10：Stop 后立即 Start（不等 100ms sleep 完成）应不死锁
func TestService_Stop_ThenImmediateStart_NoDeadlock(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := setupService(t, publisher)
	cfg := makeServiceConfig(t)

	if _, err := service.Start(cfg); err != nil {
		t.Fatalf("第一次 Start failed: %v", err)
	}

	// Stop 内含 100ms sleep，但 Start 不应被阻塞
	// 在另一个 goroutine 中调用 Start，主 goroutine 调用 Stop
	done := make(chan struct{})
	go func() {
		service.Stop()
		close(done)
	}()

	// 立即尝试 Start（Stop 还在 sleep 中）
	// 由于 testRunning 仍为 true（Stop 的 sleep 期间），Start 应返回错误而非死锁
	startDone := make(chan error, 1)
	go func() {
		_, err := service.Start(cfg)
		startDone <- err
	}()

	// Stop 应在 500ms 内完成
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Stop 超时，可能死锁")
	}

	// Start 应在 500ms 内返回（无论成功或失败）
	select {
	case err := <-startDone:
		_ = err // 不关心结果，只要不死锁
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Start 超时，可能死锁")
	}

	// 等 Stop 完全结束后再清理
	time.Sleep(200 * time.Millisecond)
	service.Stop()
}

// P3-4 TestTestManager_WaitForTestComplete_DoesNotLeakOnCancel
// Start → Stop → 验证 waitForTestComplete goroutine 在 1s 内退出
func TestTestManager_WaitForTestComplete_DoesNotLeakOnCancel(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := setupService(t, publisher)
	cfg := makeServiceConfig(t)

	// 记录基线 goroutine 数
	runtime.GC()
	baseline := runtime.NumGoroutine()

	if _, err := service.Start(cfg); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Start 后应多出至少 2 个 goroutine（runTestLoop + waitForTestComplete）
	afterStart := runtime.NumGoroutine()
	if afterStart <= baseline {
		t.Fatalf("Start 后 goroutine 数应增加：基线 %d，启动后 %d", baseline, afterStart)
	}

	service.Stop()

	// 等待 goroutine 退出（runTestLoop 检查 ctx 后返回 → close(doneCh) → waitForTestComplete 退出）
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		runtime.GC()
		if runtime.NumGoroutine() <= baseline+1 { // 允许少量波动
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	current := runtime.NumGoroutine()
	if current > baseline+1 {
		t.Errorf("waitForTestComplete goroutine 未在 1s 内退出：基线 %d，当前 %d", baseline, current)
	}

	// 确认 status 已回到 Idle
	if status := service.GetStatus(); status.Status != types.TraversalStatusIdle {
		t.Errorf("Stop 后 status 应为 Idle，实际 %s", status.Status)
	}
}
