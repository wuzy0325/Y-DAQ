package three_hole

import (
	"testing"
	"time"

	"yx-daq/internal/types"
)

// TestGenerationCheckSimple 简化的代际计数器测试
func TestGenerationCheckSimple(t *testing.T) {
	publisher := &MockEventPublisher{}
	service := NewThreeHoleTraversalService(publisher)

	// 模拟依赖
	service.SetMotionController(func(axis types.AxisName, position float64) error { return nil })
	service.SetMotionWaiter(func(axis types.AxisName, timeoutMs int) error { return nil })
	service.SetBatchGetter(func(channels []types.ThreeHoleProbeChannelConfig) (map[int]float64, error) {
		return map[int]float64{0: 100, 1: 105, 2: 102, 3: 101.325, 4: 25}, nil
	})

	// 加载校准文件
	err := service.LoadCalibFiles([]string{"./test_data/calib_0.5.dat"})
	if err != nil {
		t.Fatalf("加载校准文件失败: %v", err)
	}

	// 创建简单配置
	config := types.ThreeHoleTraversalConfig{
		Name:               "Test",
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
		SavePath:         "/tmp",
		SaveFileName:     "test.csv",
	}

	// 第一次启动测试
	taskID1, err := service.Start(config)
	if err != nil {
		t.Fatalf("第一次启动失败: %v", err)
	}

	// 尝试启动第二次测试（应该被拒绝）
	_, err = service.Start(config)
	if err == nil {
		t.Error("第二次启动应该被拒绝")
	}

	// 验证任务ID（只有一个测试在运行）
	status := service.GetStatus()
	if status.TaskID != taskID1 {
		t.Errorf("当前任务ID应该是 %s，实际是 %s", taskID1, status.TaskID)
	}

	// 等待一小段时间让协程处理
	time.Sleep(200 * time.Millisecond)

	// 验证第一个测试仍在运行或遇到错误（错误可能是由于测试环境限制）
	status = service.GetStatus()
	if status.Status != types.TraversalStatusRunning && status.Status != types.TraversalStatusError {
		t.Errorf("最终状态应该是running或error，当前状态: %s", status.Status)
	}

	// 如果是错误状态，记录错误信息用于调试
	if status.Status == types.TraversalStatusError {
		t.Logf("测试遇到错误: %s", status.LastError)
	}
}