package three_hole

import (
	"testing"
	"time"

	"yx-daq/internal/types"
)

// TestNewTestManager 测试测试管理器创建
func TestNewTestManager(t *testing.T) {
	publisher := &MockEventPublisher{}
	testManager := NewTestManager(publisher)
	if testManager == nil {
		t.Fatal("Expected test manager to be created")
	}
}

// TestStart_NewTask 测试启动新任务
func TestStart_NewTask(t *testing.T) {
	publisher := &MockEventPublisher{}
	testManager := NewTestManager(publisher)

	config := types.ThreeHoleTraversalConfig{
		Name: "TestStart",
		Layout: types.TraversalLayout{
			Pattern: types.TraversalPatternLine,
			Line: &types.LineLayout{
				StartX: 0,
				EndX:   10,
				StartY: 0,
				EndY:   5,
			},
		},
	}

	taskID, err := testManager.Start(config)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if taskID == "" {
		t.Error("Expected task ID to be non-empty")
	}

	// 验证状态
	status := testManager.GetStatus()
	if status.Status != types.TraversalStatusRunning {
		t.Errorf("Expected status running, got %s", status.Status)
	}

	if status.TaskID != taskID {
		t.Errorf("Task ID mismatch: expected %s, got %s", taskID, status.TaskID)
	}
}

// TestStart_AlreadyRunning 测试启动已运行的任务
func TestStart_AlreadyRunning(t *testing.T) {
	publisher := &MockEventPublisher{}
	testManager := NewTestManager(publisher)

	config := types.ThreeHoleTraversalConfig{
		Name: "AlreadyRunning",
		Layout: types.TraversalLayout{
			Pattern: types.TraversalPatternLine,
			Line: &types.LineLayout{
				StartX: 0,
				EndX:   10,
				StartY: 0,
				EndY:   5,
			},
		},
	}

	// 启动第一个任务
	_, err := testManager.Start(config)
	if err != nil {
		t.Fatalf("First start failed: %v", err)
	}

	// 尝试启动第二个任务
	_, err = testManager.Start(config)
	if err == nil {
		t.Error("Expected error when starting second task")
	}
}

// TestPause_RunningTask 测试暂停运行中的任务
func TestPause_RunningTask(t *testing.T) {
	publisher := &MockEventPublisher{}
	testManager := NewTestManager(publisher)

	// 启动任务
	config := types.ThreeHoleTraversalConfig{
		Name: "PauseTest",
		Layout: types.TraversalLayout{
			Pattern: types.TraversalPatternLine,
			Line: &types.LineLayout{
				StartX: 0,
				EndX:   10,
				StartY: 0,
				EndY:   5,
			},
		},
	}
	testManager.Start(config)

	// 暂停任务
	testManager.Pause()

	// 验证状态
	status := testManager.GetStatus()
	if status.Status != types.TraversalStatusPaused {
		t.Errorf("Expected status paused, got %s", status.Status)
	}
}

// TestPause_NotRunning 测试暂停未运行的任务
func TestPause_NotRunning(t *testing.T) {
	publisher := &MockEventPublisher{}
	testManager := NewTestManager(publisher)

	// 尝试暂停未运行的任务
	testManager.Pause()

	// 状态应该保持不变
	status := testManager.GetStatus()
	if status.Status != types.TraversalStatusIdle {
		t.Errorf("Expected status idle, got %s", status.Status)
	}
}

// TestResume_PausedTask 测试恢复暂停的任务
func TestResume_PausedTask(t *testing.T) {
	publisher := &MockEventPublisher{}
	testManager := NewTestManager(publisher)

	// 启动并暂停任务
	config := types.ThreeHoleTraversalConfig{
		Name: "ResumeTest",
		Layout: types.TraversalLayout{
			Pattern: types.TraversalPatternLine,
			Line: &types.LineLayout{
				StartX: 0,
				EndX:   10,
				StartY: 0,
				EndY:   5,
			},
		},
	}
	testManager.Start(config)
	testManager.Pause()

	// 恢复任务
	testManager.Resume()

	// 验证状态
	status := testManager.GetStatus()
	if status.Status != types.TraversalStatusRunning {
		t.Errorf("Expected status running, got %s", status.Status)
	}
}

// TestResume_NotPaused 测试恢复未暂停的任务
func TestResume_NotPaused(t *testing.T) {
	publisher := &MockEventPublisher{}
	testManager := NewTestManager(publisher)

	// 尝试恢复未暂停的任务
	testManager.Resume()

	// 状态应该保持不变
	status := testManager.GetStatus()
	if status.Status != types.TraversalStatusIdle {
		t.Errorf("Expected status idle, got %s", status.Status)
	}
}

// TestStop_RunningTask 测试停止运行中的任务
func TestStop_RunningTask(t *testing.T) {
	publisher := &MockEventPublisher{}
	testManager := NewTestManager(publisher)

	// 启动任务
	config := types.ThreeHoleTraversalConfig{
		Name: "StopTest",
		Layout: types.TraversalLayout{
			Pattern: types.TraversalPatternLine,
			Line: &types.LineLayout{
				StartX: 0,
				EndX:   10,
				StartY: 0,
				EndY:   5,
			},
		},
	}
	testManager.Start(config)

	// 停止任务
	testManager.Stop()

	// 验证状态
	status := testManager.GetStatus()
	if status.Status != types.TraversalStatusIdle {
		t.Errorf("Expected status idle, got %s", status.Status)
	}
}

// TestStop_CompletedTask 测试停止已完成的任务
func TestStop_CompletedTask(t *testing.T) {
	publisher := &MockEventPublisher{}
	testManager := NewTestManager(publisher)

	// 启动任务
	config := types.ThreeHoleTraversalConfig{Name: "StopCompleted"}
	taskID, _ := testManager.Start(config)

	// 设置任务为完成状态
	testManager.SetStatus(types.TraversalStatusCompleted)

	// 停止任务
	testManager.Stop()

	// 验证状态
	status := testManager.GetStatus()
	if status.Status != types.TraversalStatusIdle {
		t.Errorf("Expected status idle, got %s", status.Status)
	}

	// 验证任务ID
	if status.TaskID != taskID {
		t.Errorf("Task ID should remain: %s, got: %s", taskID, status.TaskID)
	}
}

// TestGetStatus 测试获取状态
func TestGetStatus(t *testing.T) {
	publisher := &MockEventPublisher{}
	testManager := NewTestManager(publisher)

	// 初始状态
	status := testManager.GetStatus()
	if status.Status != types.TraversalStatusIdle {
		t.Errorf("Expected initial status idle, got %s", status.Status)
	}
}

// TestGetConfig 测试获取配置
func TestGetConfig(t *testing.T) {
	publisher := &MockEventPublisher{}
	testManager := NewTestManager(publisher)

	config := types.ThreeHoleTraversalConfig{
		Name: "GetConfigTest",
		Layout: types.TraversalLayout{
			Pattern: types.TraversalPatternLine,
			Line: &types.LineLayout{
				StartX: 0,
				EndX:   10,
				StartY: 0,
				EndY:   5,
			},
		},
	}

	// 启动任务
	testManager.Start(config)

	// 获取配置
	retrievedConfig := testManager.GetConfig()
	if retrievedConfig.Name != config.Name {
		t.Errorf("Expected config name %s, got %s", config.Name, retrievedConfig.Name)
	}
}

// TestUpdateDataPoint 测试更新数据点
func TestUpdateDataPoint(t *testing.T) {
	publisher := &MockEventPublisher{}
	testManager := NewTestManager(publisher)

	dataPoint := types.ThreeHoleTraversalDataPoint{
		PointID: "test-point",
		X:       10.5,
		Y:       20.3,
	}

	// 更新数据点
	testManager.UpdateDataPoint(dataPoint)

	// 验证状态
	status := testManager.GetStatus()
	if len(status.DataPoints) != 1 {
		t.Errorf("Expected 1 data point, got %d", len(status.DataPoints))
	}

	if status.DataPoints[0].PointID != "test-point" {
		t.Errorf("Expected point ID 'test-point', got %s", status.DataPoints[0].PointID)
	}
}

// TestUpdateProgress 测试更新进度
func TestUpdateProgress(t *testing.T) {
	publisher := &MockEventPublisher{}
	testManager := NewTestManager(publisher)

	// 启动任务
	config := types.ThreeHoleTraversalConfig{
		Name: "ProgressTest",
		Layout: types.TraversalLayout{
			Pattern: types.TraversalPatternLine,
			Line: &types.LineLayout{
				StartX: 0,
				EndX:   10,
				StartY: 0,
				EndY:   5,
			},
		},
	}
	testManager.Start(config)

	currentPoint := types.TraversalPoint{ID: "current-point", X: 15.0, Y: 25.0}

	// 更新进度
	testManager.UpdateProgress(5, 10, &currentPoint)

	// 验证状态
	status := testManager.GetStatus()
	if status.CompletedPoints != 5 {
		t.Errorf("Expected 5 completed points, got %d", status.CompletedPoints)
	}

	if status.TotalPoints != 2 {
		t.Errorf("Expected 2 total points, got %d", status.TotalPoints)
	}

	if status.Progress != 50.0 {
		t.Errorf("Expected 50.0 progress, got %.1f", status.Progress)
	}

	if status.CurrentPoint == nil || status.CurrentPoint.ID != "current-point" {
		t.Error("Current point should be set correctly")
	}
}

// TestSetStatus 测试设置状态
func TestSetStatus(t *testing.T) {
	publisher := &MockEventPublisher{}
	testManager := NewTestManager(publisher)

	// 设置状态
	testManager.SetStatus(types.TraversalStatusError)

	// 验证状态
	status := testManager.GetStatus()
	if status.Status != types.TraversalStatusError {
		t.Errorf("Expected status error, got %s", status.Status)
	}
}

// TestSetLastError 测试设置错误信息
func TestSetLastError(t *testing.T) {
	publisher := &MockEventPublisher{}
	testManager := NewTestManager(publisher)

	errMsg := "test error message"
	testManager.SetLastError(errMsg)

	// 验证错误信息
	status := testManager.GetStatus()
	if status.LastError != errMsg {
		t.Errorf("Expected error message '%s', got '%s'", errMsg, status.LastError)
	}
}

// TestEmitProgress 测试发射进度事件
func TestEmitProgress(t *testing.T) {
	publisher := &MockEventPublisher{}
	testManager := NewTestManager(publisher)

	// 发射进度事件
	testManager.EmitProgress("test-task", 10, 5, 50.0, 15.0, 25.0, "moving")

	// 验证事件
	events := publisher.GetProgressEvents()
	if len(events) != 1 {
		t.Errorf("Expected 1 progress event, got %d", len(events))
	}

	event := events[0]
	if event.TaskID != "test-task" {
		t.Errorf("Expected task ID 'test-task', got %s", event.TaskID)
	}

	if event.TotalPoints != 10 {
		t.Errorf("Expected 10 total points, got %d", event.TotalPoints)
	}

	if event.CompletedPoints != 5 {
		t.Errorf("Expected 5 completed points, got %d", event.CompletedPoints)
	}

	if event.Phase != "moving" {
		t.Errorf("Expected phase 'moving', got '%s'", event.Phase)
	}
}

// TestCheckCancelled 测试检查取消状态
func TestCheckCancelled(t *testing.T) {
	publisher := &MockEventPublisher{}
	testManager := NewTestManager(publisher)

	if err := testManager.CheckCancelled(); err != nil {
		t.Errorf("Unexpected error when not cancelled: %v", err)
	}

	testManager.cancel()

	if err := testManager.CheckCancelled(); err == nil {
		t.Error("Expected error when cancelled")
	}
}

// TestWaitForResume 测试等待恢复
func TestWaitForResume(t *testing.T) {
	publisher := &MockEventPublisher{}
	testManager := NewTestManager(publisher)

	// 设置暂停状态
	testManager.paused.Store(true)

	// 在另一个goroutine中等待恢复
	done := make(chan bool)
	go func() {
		testManager.WaitForResume()
		done <- true
	}()

	// 恢复状态
	testManager.paused.Store(false)

	// 等待goroutine完成
	select {
	case <-done:
		// 成功
	case <-time.After(1 * time.Second):
		t.Error("WaitForResume did not complete")
	}
}

// TestConcurrentAccess 测试并发访问
func TestConcurrentAccess(t *testing.T) {
	publisher := &MockEventPublisher{}
	testManager := NewTestManager(publisher)

	config := types.ThreeHoleTraversalConfig{
		Name: "ConcurrentTest",
		Layout: types.TraversalLayout{
			Pattern: types.TraversalPatternLine,
			Line: &types.LineLayout{
				StartX: 0,
				EndX:   10,
				StartY: 0,
				EndY:   5,
			},
		},
	}

	// 并发启动多个测试（应该只有一个成功）
	results := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func() {
			_, err := testManager.Start(config)
			results <- err
		}()
	}

	// 收集结果
	successCount := 0
	for i := 0; i < 5; i++ {
		err := <-results
		if err == nil {
			successCount++
		}
	}

	if successCount != 1 {
		t.Errorf("Expected exactly 1 successful start, got %d", successCount)
	}
}