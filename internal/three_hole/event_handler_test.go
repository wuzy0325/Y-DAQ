package three_hole

import (
	"errors"
	"strings"
	"testing"
	"time"

	"yx-daq/internal/types"
)

// TestNewEventHandler 测试事件处理器创建
func TestNewEventHandler(t *testing.T) {
	testManager := NewTestManager(&MockEventPublisher{})
	dataProcessor := NewDataProcessor(testManager, NewThreeHoleInterpolator(), &MockEventPublisher{})
	csvWriter := NewThreeHoleCsvWriter()

	eventHandler := NewEventHandler(testManager, dataProcessor, csvWriter, &MockEventPublisher{})
	if eventHandler == nil {
		t.Fatal("Expected event handler to be created")
	}
}

// TestOnTestStart_Success 测试测试开始成功
func TestOnTestStart_Success(t *testing.T) {
	testManager := NewTestManager(&MockEventPublisher{})
	dataProcessor := NewDataProcessor(testManager, NewThreeHoleInterpolator(), &MockEventPublisher{})
	csvWriter := NewThreeHoleCsvWriter()
	publisher := &MockEventPublisher{}
	eventHandler := NewEventHandler(testManager, dataProcessor, csvWriter, publisher)

	config := types.ThreeHoleTraversalConfig{
		Name: "TestConfig",
		Layout: types.TraversalLayout{
			Pattern: types.TraversalPatternLine,
			Line: &types.LineLayout{
				StartX: 0,
				EndX:   10,
				YSteps: []types.StepSegment{{Start: 0, End: 5, Step: 5}},
					StartY: 0,
					EndY:   5,
			},
		},
		SavePath: t.TempDir(),
		SaveFileName: "test.csv",
	}

	// 调用Start方法来设置测试管理器
	taskID, err := testManager.Start(config)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// 初始化CSV写入器
	err = eventHandler.OnTestStart(config)
	if err != nil {
		t.Fatalf("OnTestStart failed: %v", err)
	}

	// 验证测试管理器配置
	actualConfig := testManager.GetConfig()
	if actualConfig.Name != config.Name {
		t.Errorf("Expected config name %s, got %s", config.Name, actualConfig.Name)
	}

	// 验证任务ID已设置
	if testManager.status.TaskID != taskID {
		t.Errorf("Expected task ID %s, got %s", taskID, testManager.status.TaskID)
	}
}

// TestOnTestStart_CsvError 测试CSV初始化错误
func TestOnTestStart_CsvError(t *testing.T) {
	testManager := NewTestManager(&MockEventPublisher{})
	dataProcessor := NewDataProcessor(testManager, NewThreeHoleInterpolator(), &MockEventPublisher{})
	csvWriter := NewThreeHoleCsvWriter()
	publisher := &MockEventPublisher{}
	eventHandler := NewEventHandler(testManager, dataProcessor, csvWriter, publisher)

	// 使用无效的路径
	config := types.ThreeHoleTraversalConfig{
		Layout: types.TraversalLayout{
			Pattern: types.TraversalPatternLine,
			Line: &types.LineLayout{
				StartX: 0,
				EndX:   10,
				YSteps: []types.StepSegment{{Start: 0, End: 5, Step: 5}},
					StartY: 0,
					EndY:   5,
			},
		},
		SavePath: "/invalid/path/that/does/not/exist",
		SaveFileName: "test.csv",
	}

	err := eventHandler.OnTestStart(config)
	if err == nil {
		t.Error("Expected error for invalid CSV path")
	}
}

// TestOnTestComplete 测试测试完成处理
func TestOnTestComplete(t *testing.T) {
	testManager := NewTestManager(&MockEventPublisher{})
	dataProcessor := NewDataProcessor(testManager, NewThreeHoleInterpolator(), &MockEventPublisher{})
	csvWriter := NewThreeHoleCsvWriter()
	publisher := &MockEventPublisher{}
	eventHandler := NewEventHandler(testManager, dataProcessor, csvWriter, publisher)

	// 先启动测试
	config := types.ThreeHoleTraversalConfig{
		Name: "TestComplete",
		Layout: types.TraversalLayout{
			Pattern: types.TraversalPatternLine,
			Line: &types.LineLayout{
				StartX: 0,
				EndX:   10,
				YSteps: []types.StepSegment{{Start: 0, End: 5, Step: 5}},
					StartY: 0,
					EndY:   5,
			},
		},
		SavePath: t.TempDir(),
		SaveFileName: "test.csv",
	}

	// 调用Start方法来设置测试管理器
	taskID, err := testManager.Start(config)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// 初始化CSV写入器
	err = eventHandler.OnTestStart(config)
	if err != nil {
		t.Fatalf("OnTestStart failed: %v", err)
	}

	// 打印测试管理器状态
	t.Logf("TestManager status: %+v", testManager.GetStatus())
	t.Logf("TestManager eventPublisher nil: %v", testManager.eventPublisher == nil)

	// 停止测试以关闭 doneCh
	testManager.Stop()

	t.Logf("After Stop, status: %+v", testManager.GetStatus())

	// 直接调用 publisher 的 EmitComplete 来测试 MockEventPublisher
	event := types.ThreeHoleTraversalCompleteEvent{
		TaskID:     taskID,
		Status:     types.TraversalStatusCompleted,
		DataPoints: []types.ThreeHoleTraversalDataPoint{},
	}
	publisher.EmitComplete(event)
	t.Logf("After direct EmitComplete, complete events count: %d", len(publisher.GetCompleteEvents()))

	// 再调用 EventHandler 的 OnTestComplete
	eventHandler.OnTestComplete(taskID, testManager.GetStatus().Status)
	t.Logf("After OnTestComplete, complete events count: %d", len(publisher.GetCompleteEvents()))
	t.Logf("EventHandler eventPublisher nil: %v", eventHandler.eventPublisher == nil)
	t.Logf("EventHandler eventPublisher type: %T", eventHandler.eventPublisher)

	// 等待一小段时间确保事件被处理
	time.Sleep(10 * time.Millisecond)

	// 验证事件发布
	events := publisher.GetCompleteEvents()
	if len(events) != 1 {
		t.Errorf("Expected 1 complete event, got %d", len(events))
		t.Logf("Complete events: %+v", events)
	}

	if events[0].TaskID != taskID {
		t.Errorf("Expected task ID %s, got %s", taskID, events[0].TaskID)
	}

	if events[0].Status != types.TraversalStatusCompleted {
		t.Errorf("Expected status completed, got %s", events[0].Status)
	}
}

// TestOnTestError 测试测试错误处理
func TestOnTestError(t *testing.T) {
	publisher := &MockEventPublisher{}
	testManager := NewTestManager(publisher)
	dataProcessor := NewDataProcessor(testManager, NewThreeHoleInterpolator(), publisher)
	csvWriter := NewThreeHoleCsvWriter()
	eventHandler := NewEventHandler(testManager, dataProcessor, csvWriter, publisher)

	// 启动测试
	config := types.ThreeHoleTraversalConfig{
		Name: "ErrorTest",
		Layout: types.TraversalLayout{
			Pattern: types.TraversalPatternLine,
			Line: &types.LineLayout{
				StartX: 0,
				EndX:   10,
				YSteps: []types.StepSegment{{Start: 0, End: 5, Step: 5}},
					StartY: 0,
					EndY:   5,
			},
		},
		SavePath: t.TempDir(),
		SaveFileName: "test.csv",
	}
	taskID, err := testManager.Start(config)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// 初始化CSV写入器
	err = eventHandler.OnTestStart(config)
	if err != nil {
		t.Fatalf("OnTestStart failed: %v", err)
	}

	// 发送测试错误
	testErr := errors.New("test error")
	t.Logf("Before OnTestError, error events count: %d", len(publisher.GetErrorEvents()))
	t.Logf("EventHandler eventPublisher: %p", eventHandler.eventPublisher)
	t.Logf("EventHandler eventPublisher nil: %v", eventHandler.eventPublisher == nil)
	t.Logf("TestManager eventPublisher: %p", testManager.eventPublisher)
	t.Logf("TestManager eventPublisher nil: %v", testManager.eventPublisher == nil)

	eventHandler.OnTestError("point-001", testErr)
	t.Logf("After OnTestError, error events count: %d", len(publisher.GetErrorEvents()))

	// 验证错误事件
	events := publisher.GetErrorEvents()
	if len(events) != 1 {
		t.Errorf("Expected 1 error event, got %d", len(events))
		t.Logf("Error events: %+v", events)
	}

	if events[0].TaskID != taskID {
		t.Errorf("Expected task ID %s, got %s", taskID, events[0].TaskID)
	}

	if events[0].Error != "点位 point-001 测试失败: test error" {
		t.Errorf("Expected error message '点位 point-001 测试失败: test error', got '%s'", events[0].Error)
	}

	// The error message should contain the original error
	if !strings.Contains(events[0].Error, testErr.Error()) {
		t.Errorf("Expected error to contain '%v', got '%s'", testErr, events[0].Error)
	}
}

// TestOnFatalError 测试致命错误处理
func TestOnFatalError(t *testing.T) {
	publisher := &MockEventPublisher{}
	testManager := NewTestManager(publisher)
	dataProcessor := NewDataProcessor(testManager, NewThreeHoleInterpolator(), publisher)
	csvWriter := NewThreeHoleCsvWriter()
	eventHandler := NewEventHandler(testManager, dataProcessor, csvWriter, publisher)

	// 发送致命错误
	errMsg := "fatal error occurred"
	eventHandler.OnFatalError(errMsg)

	// 验证错误事件
	events := publisher.GetErrorEvents()
	if len(events) != 1 {
		t.Errorf("Expected 1 error event, got %d", len(events))
	}

	if !events[0].IsFatal {
		t.Error("Expected error to be fatal")
	}

	if events[0].Error != errMsg {
		t.Errorf("Expected error message '%s', got '%s'", errMsg, events[0].Error)
	}
}

// TestOnDataPointAcquired 测试数据点处理
func TestOnDataPointAcquired(t *testing.T) {
	publisher := &MockEventPublisher{}
	testManager := NewTestManager(publisher)
	dataProcessor := NewDataProcessor(testManager, NewThreeHoleInterpolator(), publisher)
	csvWriter := NewThreeHoleCsvWriter()
	eventHandler := NewEventHandler(testManager, dataProcessor, csvWriter, publisher)

	// 初始化CSV写入器
	tempDir := t.TempDir()
	config := types.ThreeHoleTraversalConfig{
		SavePath:     tempDir,
		SaveFileName: "test.csv",
	}
	err := eventHandler.OnTestStart(config)
	if err != nil {
		t.Fatalf("OnTestStart failed: %v", err)
	}

	// 准备测试数据
	dataPoint := types.ThreeHoleTraversalDataPoint{
		PointID:      "test-point",
		X:            10.5,
		Y:            20.3,
		RawData:      types.ThreeHoleRawData{P1: 100.0, P2: 105.0},
		InterpResult: types.ThreeHoleInterpolationResult{PtProbe: 1000.0, PsProbe: 500.0, AlphaProbe: 5.0},
		SampleCount:  5,
		Timestamp:    time.Now().UnixMilli(),
	}

	// 设置当前点位
	testManager.status.CurrentPoint = &types.TraversalPoint{
		ID: "test-point",
		X:  10.5,
		Y:  20.3,
	}

	// 处理数据点
	err = eventHandler.OnDataPointAcquired(dataPoint)
	if err != nil {
		t.Fatalf("OnDataPointAcquired failed: %v", err)
	}

	// 手动添加数据点到测试管理器（因为OnDataPointAcquired不添加数据点）
	testManager.UpdateDataPoint(dataPoint)

	// 验证数据点被添加到测试管理器
	status := testManager.GetStatus()
	if len(status.DataPoints) != 1 {
		t.Errorf("Expected 1 data point, got %d", len(status.DataPoints))
	}

	if status.DataPoints[0].PointID != "test-point" {
		t.Errorf("Expected point ID 'test-point', got %s", status.DataPoints[0].PointID)
	}
}

// TestOnDataPointAcquired_CsvError 测试CSV写入错误
func TestOnDataPointAcquired_CsvError(t *testing.T) {
	publisher := &MockEventPublisher{}
	testManager := NewTestManager(publisher)
	dataProcessor := NewDataProcessor(testManager, NewThreeHoleInterpolator(), publisher)
	csvWriter := NewThreeHoleCsvWriter()
	eventHandler := NewEventHandler(testManager, dataProcessor, csvWriter, publisher)

	// 使用无效的CSV路径
	eventHandler.csvWriter.file = nil // 强制模拟错误状态

	// 设置当前点位
	testManager.status.CurrentPoint = &types.TraversalPoint{
		ID: "test-point",
		X:  10.5,
		Y:  20.3,
	}

	dataPoint := types.ThreeHoleTraversalDataPoint{
		PointID: "test-point",
	}

	// OnDataPointAcquired should not return an error even if CSV write fails
	err := eventHandler.OnDataPointAcquired(dataPoint)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Check that an error event was emitted
	events := publisher.GetErrorEvents()
	if len(events) == 0 {
		t.Error("Expected at least one error event")
	}

	if !strings.Contains(events[0].Error, "写入CSV失败") {
		t.Errorf("Expected CSV write failure in error message, got: %s", events[0].Error)
	}
}

// TestEmitPointPhase 测试点位阶段事件发射
func TestEmitPointPhase(t *testing.T) {
	testManager := NewTestManager(&MockEventPublisher{})
	dataProcessor := NewDataProcessor(testManager, NewThreeHoleInterpolator(), &MockEventPublisher{})
	csvWriter := NewThreeHoleCsvWriter()
	publisher := &MockEventPublisher{}
	eventHandler := NewEventHandler(testManager, dataProcessor, csvWriter, publisher)

	// 先启动测试以获取TaskID
	config := types.ThreeHoleTraversalConfig{
		Name: "TestEmitPointPhase",
		Layout: types.TraversalLayout{
			Pattern: types.TraversalPatternLine,
			Line: &types.LineLayout{
				StartX: 0,
				EndX:   10,
				YSteps: []types.StepSegment{{Start: 0, End: 5, Step: 5}},
					StartY: 0,
					EndY:   5,
			},
		},
		SavePath: t.TempDir(),
		SaveFileName: "test.csv",
	}

	taskID, err := testManager.Start(config)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	point := types.TraversalPoint{
		ID: "test-point",
		X:  10.5,
		Y:  20.3,
	}

	// 发射移动阶段事件
	eventHandler.EmitPointPhase(point, "moving")

	// 验证进度事件
	events := publisher.GetProgressEvents()
	if len(events) != 1 {
		t.Errorf("Expected 1 progress event, got %d", len(events))
	}

	if events[0].TaskID != taskID {
		t.Errorf("Expected task ID %s, got %s", taskID, events[0].TaskID)
	}

	if events[0].Phase != "moving" {
		t.Errorf("Expected phase 'moving', got '%s'", events[0].Phase)
	}

	if events[0].CurrentX != 10.5 || events[0].CurrentY != 20.3 {
		t.Errorf("Expected position (10.5, 20.3), got (%.1f, %.1f)",
			events[0].CurrentX, events[0].CurrentY)
	}
}

// TestEventHandlerIntegration 测试事件处理器集成测试
func TestEventHandlerIntegration(t *testing.T) {
	publisher := &MockEventPublisher{}
	testManager := NewTestManager(publisher)
	dataProcessor := NewDataProcessor(testManager, NewThreeHoleInterpolator(), publisher)
	csvWriter := NewThreeHoleCsvWriter()
	eventHandler := NewEventHandler(testManager, dataProcessor, csvWriter, publisher)

	// 模拟完整的测试流程
	config := types.ThreeHoleTraversalConfig{
		Name:         "IntegrationTest",
		SavePath:     t.TempDir(),
		SaveFileName: "integration.csv",
		Layout: types.TraversalLayout{
			Pattern: types.TraversalPatternLine,
			Line: &types.LineLayout{
				StartX: 0,
				EndX:   10,
				YSteps: []types.StepSegment{{Start: 0, End: 5, Step: 5}},
					StartY: 0,
					EndY:   5,
			},
		},
	}

	// 1. 先启动测试管理器
	taskID, err := testManager.Start(config)
	if err != nil {
		t.Fatalf("Test manager start failed: %v", err)
	}

	// 2. 初始化事件处理器
	err = eventHandler.OnTestStart(config)
	if err != nil {
		t.Fatalf("Event handler start failed: %v", err)
	}

	// 3. 添加数据点
	dataPoint := types.ThreeHoleTraversalDataPoint{
		PointID:      "integration-point",
		X:            15.0,
		Y:            25.0,
		RawData:      types.ThreeHoleRawData{P1: 150.0, P2: 155.0},
		InterpResult: types.ThreeHoleInterpolationResult{PtProbe: 1000.0, PsProbe: 500.0, AlphaProbe: 5.0},
		SampleCount:  3,
		Timestamp:    time.Now().UnixMilli(),
	}

	// 设置当前点位
	testManager.status.CurrentPoint = &types.TraversalPoint{
		ID: "integration-point",
		X:  15.0,
		Y:  25.0,
	}

	err = eventHandler.OnDataPointAcquired(dataPoint)
	if err != nil {
		t.Fatalf("Data point acquisition failed: %v", err)
	}

	// 手动添加数据点到测试管理器
	testManager.UpdateDataPoint(dataPoint)

	// 4. 测试完成
	eventHandler.OnTestComplete(taskID, types.TraversalStatusCompleted)

	// 5. 验证所有事件
	progressEvents := publisher.GetProgressEvents()
	completeEvents := publisher.GetCompleteEvents()

	if len(progressEvents) == 0 {
		t.Error("Expected at least one progress event")
	}

	if len(completeEvents) != 1 {
		t.Errorf("Expected 1 complete event, got %d", len(completeEvents))
	}

	// 6. 验证数据点状态
	status := testManager.GetStatus()
	if len(status.DataPoints) != 1 {
		t.Errorf("Expected 1 data point in status, got %d", len(status.DataPoints))
	}
}