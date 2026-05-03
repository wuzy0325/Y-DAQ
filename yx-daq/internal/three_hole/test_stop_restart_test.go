package three_hole

import (
	"testing"
	"time"

	"yx-daq/internal/types"
)

// TestStopRestart 测试停止后再启动的场景
func TestStopRestart(t *testing.T) {
	publisher := &MockEventPublisher{}
	testManager := NewTestManager(publisher)

	config := types.ThreeHoleTraversalConfig{
		Name: "StopRestartTest",
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

	// 第一次启动
	taskID1, err := testManager.Start(config)
	if err != nil {
		t.Fatalf("First start failed: %v", err)
	}

	// 验证状态
	status := testManager.GetStatus()
	if status.Status != types.TraversalStatusRunning {
		t.Errorf("After first start, expected status running, got %s", status.Status)
	}

	// 停止任务
	testManager.Stop()

	// 验证已停止
	status = testManager.GetStatus()
	if status.Status != types.TraversalStatusIdle {
		t.Errorf("After stop, expected status idle, got %s", status.Status)
	}

	// 等待一小段时间确保时间戳不同
	time.Sleep(20 * time.Millisecond)

	// 第二次启动（停止后立即启动）
	taskID2, err := testManager.Start(config)
	if err != nil {
		t.Fatalf("Second start failed: %v", err)
	}

	// 验证新的任务ID不同（证明是新任务）
	if taskID1 == taskID2 {
		t.Errorf("Expected different task IDs for restart, got same ID: %s", taskID1)
	}

	// 验证状态再次为运行中
	status = testManager.GetStatus()
	if status.Status != types.TraversalStatusRunning {
		t.Errorf("After second start, expected status running, got %s", status.Status)
	}

	// 再次停止
	testManager.Stop()

	// 等待一小段时间确保时间戳不同
	time.Sleep(20 * time.Millisecond)

	// 第三次启动
	taskID3, err := testManager.Start(config)
	if err != nil {
		t.Fatalf("Third start failed: %v", err)
	}

	// 验证任务ID又不同
	if taskID2 == taskID3 {
		t.Errorf("Expected different task ID for third start, got same as second: %s", taskID2)
	}

	// 使用声明的变量
	_ = taskID2
}

// TestStopRestartRaceCondition 测试停止和启动之间的竞态条件
func TestStopRestartRaceCondition(t *testing.T) {
	publisher := &MockEventPublisher{}
	testManager := NewTestManager(publisher)

	config := types.ThreeHoleTraversalConfig{
		Name: "RaceConditionTest",
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

	// 快速连续启动和停止多次
	for i := 0; i < 10; i++ {
		_, err := testManager.Start(config)
		if err != nil {
			t.Fatalf("Start iteration %d failed: %v", i, err)
		}

		// 验证状态
		status := testManager.GetStatus()
		if status.Status != types.TraversalStatusRunning {
			t.Errorf("Iteration %d: after start, expected status running, got %s", i, status.Status)
		}

		// 短暂等待
		time.Sleep(10 * time.Millisecond)

		// 停止
		testManager.Stop()

		// 验证已停止
		status = testManager.GetStatus()
		if status.Status != types.TraversalStatusIdle {
			t.Errorf("Iteration %d: after stop, expected status idle, got %s", i, status.Status)
		}
	}
}

// TestStopRestartWithConfigChange 测试停止后修改配置再启动
func TestStopRestartWithConfigChange(t *testing.T) {
	publisher := &MockEventPublisher{}
	testManager := NewTestManager(publisher)

	// 初始配置
	config1 := types.ThreeHoleTraversalConfig{
		Name: "ConfigChangeTest",
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

	// 启动并停止
	_, err := testManager.Start(config1)
	if err != nil {
		t.Fatalf("First start failed: %v", err)
	}
	testManager.Stop()

	// 修改配置
	config2 := types.ThreeHoleTraversalConfig{
		Name: "ConfigChangeTestModified",
		Layout: types.TraversalLayout{
			Pattern: types.TraversalPatternRectangle,
			Rectangle: &types.RectangleLayout{
				XMin: 0,
				XMax: 20,
				YMin: 0,
				YMax: 10,
			},
		},
	}

	// 使用新配置重新启动
	testManager.Start(config2)
	if err != nil {
		t.Fatalf("Second start with new config failed: %v", err)
	}

	// 验证状态
	status := testManager.GetStatus()
	if status.Status != types.TraversalStatusRunning {
		t.Errorf("After restart with new config, expected status running, got %s", status.Status)
	}

	// 验证配置已更新
	currentConfig := testManager.GetConfig()
	if currentConfig.Name != "ConfigChangeTestModified" {
		t.Errorf("Expected config name to be updated, got %s", currentConfig.Name)
	}
}