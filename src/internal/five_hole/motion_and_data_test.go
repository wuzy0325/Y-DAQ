package five_hole

import (
	"sync"
	"testing"
	"time"

	"yx-daq/internal/types"
)

// ===== 运动协调器测试 =====

func TestMotionCoordinator_AllProbesMoveParallel(t *testing.T) {
	callCount := 0
	var mu sync.Mutex
	mover := func(controllerID string, axis types.AxisName, position float64) error {
		mu.Lock()
		callCount++
		mu.Unlock()
		return nil
	}
	waiter := func(controllerID string, axis types.AxisName, timeoutMs int) error {
		return nil
	}
	mc := NewMotionCoordinator(mover, waiter)

	probes := []types.FiveHoleProbeConfig{
		{
			ProbeID: "probe1", Enabled: true,
			MotionAlpha: types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "X"},
			MotionBeta:  types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "Y"},
		},
		{
			ProbeID: "probe2", Enabled: true,
			MotionAlpha: types.FiveHoleMotionAxisMapping{ControllerID: "c2", Axis: "X"},
			MotionBeta:  types.FiveHoleMotionAxisMapping{ControllerID: "c2", Axis: "Y"},
		},
	}
	layout := types.TraversalLayout{Pattern: types.TraversalPatternRectangle}
	point := types.TraversalPoint{ID: "p1", X: 10, Y: 20}

	err := mc.MoveAllProbesToPoint(point, probes, layout, 1000)
	if err != nil {
		t.Fatalf("MoveAllProbesToPoint failed: %v", err)
	}
	// 2 探针 × 2 轴 = 4 次调用
	if callCount != 4 {
		t.Fatalf("expected 4 mover calls, got %d", callCount)
	}
}

func TestMotionCoordinator_LineSingleAxisX_SkipBeta(t *testing.T) {
	movedAxes := make(map[string]bool)
	var mu sync.Mutex
	mover := func(controllerID string, axis types.AxisName, position float64) error {
		mu.Lock()
		movedAxes[string(axis)] = true
		mu.Unlock()
		return nil
	}
	waiter := func(controllerID string, axis types.AxisName, timeoutMs int) error {
		return nil
	}
	mc := NewMotionCoordinator(mover, waiter)

	probes := []types.FiveHoleProbeConfig{
		{
			ProbeID: "probe1", Enabled: true,
			MotionAlpha: types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "X"},
			MotionBeta:  types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "Y"},
		},
	}
	// 直线单轴：仅 X 变化
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternLine,
		Line: &types.LineLayout{
			StartX: 0, StartY: 5,
			EndX:   10, EndY: 5,
			XSteps: []types.StepSegment{{Start: 0, End: 10, Step: 5}},
		},
	}
	point := types.TraversalPoint{ID: "p1", X: 10, Y: 5}

	err := mc.MoveAllProbesToPoint(point, probes, layout, 1000)
	if err != nil {
		t.Fatalf("MoveAllProbesToPoint failed: %v", err)
	}
	// 仅 α（X 轴）移动，β（Y 轴）跳过
	if !movedAxes["X"] {
		t.Fatal("expected X axis moved")
	}
	if movedAxes["Y"] {
		t.Fatal("expected Y axis skipped")
	}
}

func TestMotionCoordinator_LineSingleAxisY_SkipAlpha(t *testing.T) {
	movedAxes := make(map[string]bool)
	var mu sync.Mutex
	mover := func(controllerID string, axis types.AxisName, position float64) error {
		mu.Lock()
		movedAxes[string(axis)] = true
		mu.Unlock()
		return nil
	}
	waiter := func(controllerID string, axis types.AxisName, timeoutMs int) error {
		return nil
	}
	mc := NewMotionCoordinator(mover, waiter)

	probes := []types.FiveHoleProbeConfig{
		{
			ProbeID: "probe1", Enabled: true,
			MotionAlpha: types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "X"},
			MotionBeta:  types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "Y"},
		},
	}
	// 直线单轴：仅 Y 变化
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternLine,
		Line: &types.LineLayout{
			StartX: 5, StartY: 0,
			EndX:   5, EndY: 10,
			YSteps: []types.StepSegment{{Start: 0, End: 10, Step: 5}},
		},
	}
	point := types.TraversalPoint{ID: "p1", X: 5, Y: 10}

	err := mc.MoveAllProbesToPoint(point, probes, layout, 1000)
	if err != nil {
		t.Fatalf("MoveAllProbesToPoint failed: %v", err)
	}
	// 仅 β（Y 轴）移动，α（X 轴）跳过
	if !movedAxes["Y"] {
		t.Fatal("expected Y axis moved")
	}
	if movedAxes["X"] {
		t.Fatal("expected X axis skipped")
	}
}

func TestMotionCoordinator_DisabledProbeSkipped(t *testing.T) {
	callCount := 0
	var mu sync.Mutex
	mover := func(controllerID string, axis types.AxisName, position float64) error {
		mu.Lock()
		callCount++
		mu.Unlock()
		return nil
	}
	waiter := func(controllerID string, axis types.AxisName, timeoutMs int) error {
		return nil
	}
	mc := NewMotionCoordinator(mover, waiter)

	probes := []types.FiveHoleProbeConfig{
		{
			ProbeID: "probe1", Enabled: true,
			MotionAlpha: types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "X"},
			MotionBeta:  types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "Y"},
		},
		{
			ProbeID: "probe2", Enabled: false, // 禁用
		},
	}
	layout := types.TraversalLayout{Pattern: types.TraversalPatternRectangle}
	point := types.TraversalPoint{ID: "p1", X: 10, Y: 20}

	err := mc.MoveAllProbesToPoint(point, probes, layout, 1000)
	if err != nil {
		t.Fatalf("MoveAllProbesToPoint failed: %v", err)
	}
	// 仅 1 个启用探针 × 2 轴 = 2 次调用
	if callCount != 2 {
		t.Fatalf("expected 2 mover calls, got %d", callCount)
	}
}

// ===== 数据处理器测试 =====

func TestDataProcessor_ReadAllProbesRawData(t *testing.T) {
	// mock batchGetter（返回 timestamp 用于去重）
	batchGetter := func(deviceID string, channels []int) (map[int]float64, int64, error) {
		result := make(map[int]float64)
		for _, ch := range channels {
			// 不同设备返回不同值以区分
			switch deviceID {
			case "dev_pAtm":
				result[ch] = 101.325
			case "dev_tAtm":
				result[ch] = 20.5
			case "dev1":
				// probe1 P1-P5
				result[0] = 100.0
				result[1] = 101.0
				result[2] = 99.0
				result[3] = 100.5
				result[4] = 99.5
			case "dev2":
				// probe2 P1-P5
				result[10] = 200.0
				result[11] = 201.0
				result[12] = 199.0
				result[13] = 200.5
				result[14] = 199.5
			}
		}
		return result, time.Now().UnixMilli(), nil
	}

	dp := NewDataProcessor()
	dp.SetBatchGetter(batchGetter)

	probes := []types.FiveHoleProbeConfig{
		{
			ProbeID: "probe1", Enabled: true,
			ProbeChannels: []types.FiveHoleProbeChannelConfig{
				{Role: types.Role5H_P1, DeviceID: "dev1", Channel: 0, Enabled: true},
				{Role: types.Role5H_P2, DeviceID: "dev1", Channel: 1, Enabled: true},
				{Role: types.Role5H_P3, DeviceID: "dev1", Channel: 2, Enabled: true},
				{Role: types.Role5H_P4, DeviceID: "dev1", Channel: 3, Enabled: true},
				{Role: types.Role5H_P5, DeviceID: "dev1", Channel: 4, Enabled: true},
			},
		},
		{
			ProbeID: "probe2", Enabled: true,
			ProbeChannels: []types.FiveHoleProbeChannelConfig{
				{Role: types.Role5H_P1, DeviceID: "dev2", Channel: 10, Enabled: true},
				{Role: types.Role5H_P2, DeviceID: "dev2", Channel: 11, Enabled: true},
				{Role: types.Role5H_P3, DeviceID: "dev2", Channel: 12, Enabled: true},
				{Role: types.Role5H_P4, DeviceID: "dev2", Channel: 13, Enabled: true},
				{Role: types.Role5H_P5, DeviceID: "dev2", Channel: 14, Enabled: true},
			},
		},
	}

	results, _, err := dp.ReadAllProbesRawData(probes, "dev_pAtm", 0, "dev_tAtm", 0, nil)
	if err != nil {
		t.Fatalf("ReadAllProbesRawData failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// 验证 probe1
	if results[0].P1 != 100.0 || results[0].P5 != 99.5 {
		t.Fatalf("probe1 rawData mismatch: %+v", results[0])
	}
	if results[0].PAtm != 101.325 || results[0].TAtm != 20.5 {
		t.Fatalf("probe1 PAtm/TAtm mismatch: %+v", results[0])
	}
	// 验证 probe2
	if results[1].P1 != 200.0 || results[1].P5 != 199.5 {
		t.Fatalf("probe2 rawData mismatch: %+v", results[1])
	}
}

func TestOutlierFilteredAvg(t *testing.T) {
	// 无异常值
	values := []float64{100, 101, 99, 100, 100}
	avg := OutlierFilteredAvg(values)
	if avg < 99 || avg > 101 {
		t.Fatalf("expected ~100, got %f", avg)
	}

	// 含异常值（多数正常值时，异常值应被 3σ 滤除）
	// 用 11 个正常值 + 1 个极端异常值，异常值对均值/标准差影响被稀释
	values = []float64{100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 10000}
	avg = OutlierFilteredAvg(values)
	if avg > 200 {
		t.Fatalf("expected filtered avg ~100, got %f (outlier not removed)", avg)
	}

	// 空数组
	if OutlierFilteredAvg(nil) != 0 {
		t.Fatal("expected 0 for empty")
	}

	// 单值
	if OutlierFilteredAvg([]float64{42}) != 42 {
		t.Fatal("expected 42 for single value")
	}
}

// 验证并行执行（耗时约等于最慢的一次调用，而非串行总和）
func TestMotionCoordinator_ParallelExecution(t *testing.T) {
	mover := func(controllerID string, axis types.AxisName, position float64) error {
		time.Sleep(50 * time.Millisecond) // 模拟运动耗时
		return nil
	}
	waiter := func(controllerID string, axis types.AxisName, timeoutMs int) error {
		return nil
	}
	mc := NewMotionCoordinator(mover, waiter)

	probes := []types.FiveHoleProbeConfig{
		{ProbeID: "probe1", Enabled: true,
			MotionAlpha: types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "X"},
			MotionBeta:  types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "Y"}},
		{ProbeID: "probe2", Enabled: true,
			MotionAlpha: types.FiveHoleMotionAxisMapping{ControllerID: "c2", Axis: "X"},
			MotionBeta:  types.FiveHoleMotionAxisMapping{ControllerID: "c2", Axis: "Y"}},
		{ProbeID: "probe3", Enabled: true,
			MotionAlpha: types.FiveHoleMotionAxisMapping{ControllerID: "c3", Axis: "X"},
			MotionBeta:  types.FiveHoleMotionAxisMapping{ControllerID: "c3", Axis: "Y"}},
	}
	layout := types.TraversalLayout{Pattern: types.TraversalPatternRectangle}
	point := types.TraversalPoint{ID: "p1", X: 10, Y: 20}

	start := time.Now()
	_ = mc.MoveAllProbesToPoint(point, probes, layout, 1000)
	elapsed := time.Since(start)

	// 串行 = 6 × 50ms = 300ms，并行 ≈ 50ms（容忍 100ms 内）
	if elapsed > 200*time.Millisecond {
		t.Fatalf("expected parallel execution (~50ms), got %v (serial?)", elapsed)
	}
}
