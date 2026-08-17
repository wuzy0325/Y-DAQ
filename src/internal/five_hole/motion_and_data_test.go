package five_hole

import (
	"strings"
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
			MotionX: types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "X"},
			MotionY: types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "Y"},
		},
		{
			ProbeID: "probe2", Enabled: true,
			MotionX: types.FiveHoleMotionAxisMapping{ControllerID: "c2", Axis: "X"},
			MotionY: types.FiveHoleMotionAxisMapping{ControllerID: "c2", Axis: "Y"},
		},
	}
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternRectangle,
		Rectangle: &types.RectangleLayout{
			XMin: 0, XMax: 10, YMin: 0, YMax: 10,
			XSteps: []types.StepSegment{{Start: 0, End: 10, Step: 10}},
			YSteps: []types.StepSegment{{Start: 0, End: 10, Step: 10}},
			XAxis:  "X",
			YAxis:  "Y",
		},
	}
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
			MotionX: types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "X"},
			MotionY: types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "Y"},
		},
	}
	// 直线单轴：仅 X 变化
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternLine,
		Line: &types.LineLayout{
			Axis:  "X",
			Start: 0,
			End:   10,
			Step:  5,
			Fixed: 5,
		},
	}
	point := types.TraversalPoint{ID: "p1", X: 10, Y: 5}

	err := mc.MoveAllProbesToPoint(point, probes, layout, 1000)
	if err != nil {
		t.Fatalf("MoveAllProbesToPoint failed: %v", err)
	}
	// 仅 X 方向轴移动，Y 方向轴跳过
	if !movedAxes["X"] {
		t.Fatal("expected X axis moved")
	}
	if movedAxes["Y"] {
		t.Fatal("expected Y axis skipped")
	}
}

func TestMotionCoordinator_LineAlwaysUsesMotionX(t *testing.T) {
	// 直线模式永远只用 MotionX，不管 layout.Line.Axis 配什么值
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
			MotionX: types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "X"},
			MotionY: types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "Y"},
		},
	}
	// Axis=Y 也仍然只用 MotionX
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternLine,
		Line: &types.LineLayout{
			Axis:  "Y",
			Start: 0,
			End:   10,
			Step:  5,
			Fixed: 5,
		},
	}
	point := types.TraversalPoint{ID: "p1", X: 10, Y: 0}

	err := mc.MoveAllProbesToPoint(point, probes, layout, 1000)
	if err != nil {
		t.Fatalf("MoveAllProbesToPoint failed: %v", err)
	}
	// 永远是 MotionX(X) 移动，MotionY(Y) 跳过
	if !movedAxes["X"] {
		t.Fatal("expected X axis (MotionX) moved")
	}
	if movedAxes["Y"] {
		t.Fatal("expected Y axis (MotionY) skipped")
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
			MotionX: types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "X"},
			MotionY: types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "Y"},
		},
		{
			ProbeID: "probe2", Enabled: false, // 禁用
		},
	}
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternRectangle,
		Rectangle: &types.RectangleLayout{
			XMin: 0, XMax: 10, YMin: 0, YMax: 10,
			XSteps: []types.StepSegment{{Start: 0, End: 10, Step: 10}},
			YSteps: []types.StepSegment{{Start: 0, End: 10, Step: 10}},
			XAxis:  "X",
			YAxis:  "Y",
		},
	}
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

// ===== 共用轴位/物理轴去重测试 =====

func rectangleLayoutForDedup() types.TraversalLayout {
	return types.TraversalLayout{
		Pattern: types.TraversalPatternRectangle,
		Rectangle: &types.RectangleLayout{
			XMin: 0, XMax: 10, YMin: 0, YMax: 10,
			XSteps: []types.StepSegment{{Start: 0, End: 10, Step: 10}},
			YSteps: []types.StepSegment{{Start: 0, End: 10, Step: 10}},
			XAxis:  "X",
			YAxis:  "Y",
		},
	}
}

// 多探针共用同一物理轴（同轴同目标）：去重后同一物理轴只发一次移动命令
func TestMotionCoordinator_SharedPhysicalAxisDedup(t *testing.T) {
	var mu sync.Mutex
	calls := make(map[string]int) // key: controllerID|axis
	mover := func(controllerID string, axis types.AxisName, position float64) error {
		mu.Lock()
		calls[controllerID+"|"+string(axis)]++
		mu.Unlock()
		return nil
	}
	waiter := func(controllerID string, axis types.AxisName, timeoutMs int) error {
		return nil
	}
	mc := NewMotionCoordinator(mover, waiter)

	// 两根探针共用同一控制器同一轴（共用轴位归一化后的形态）
	sharedX := types.FiveHoleMotionAxisMapping{ControllerID: "mc-shared", Axis: "X"}
	sharedY := types.FiveHoleMotionAxisMapping{ControllerID: "mc-shared", Axis: "Y"}
	probes := []types.FiveHoleProbeConfig{
		{ProbeID: "probe1", Enabled: true, MotionX: sharedX, MotionY: sharedY},
		{ProbeID: "probe2", Enabled: true, MotionX: sharedX, MotionY: sharedY},
	}

	err := mc.MoveAllProbesToPoint(types.TraversalPoint{ID: "p1", X: 10, Y: 20}, probes, rectangleLayoutForDedup(), 1000)
	if err != nil {
		t.Fatalf("MoveAllProbesToPoint failed: %v", err)
	}
	// 共享 2 根物理轴：X/Y 各只发一次（而非 2 探针 × 2 轴 = 4 次）
	if calls["mc-shared|X"] != 1 || calls["mc-shared|Y"] != 1 {
		t.Fatalf("expected each shared axis moved exactly once, got %v", calls)
	}
}

// 同一物理轴被要求移动到不同目标：报物理轴冲突错误
func TestMotionCoordinator_PhysicalAxisConflict(t *testing.T) {
	mover := func(controllerID string, axis types.AxisName, position float64) error {
		return nil
	}
	waiter := func(controllerID string, axis types.AxisName, timeoutMs int) error {
		return nil
	}
	mc := NewMotionCoordinator(mover, waiter)

	// probe1 的 Y 与 probe2 的 X 指向同一物理轴 mc-1|X，
	// 矩形模式下 target 分别为 point.Y(20) 与 point.X(10) → 冲突
	probes := []types.FiveHoleProbeConfig{
		{
			ProbeID: "probe1", Enabled: true,
			MotionX: types.FiveHoleMotionAxisMapping{ControllerID: "mc-1", Axis: "X"},
			MotionY: types.FiveHoleMotionAxisMapping{ControllerID: "mc-1", Axis: "X"},
		},
		{
			ProbeID: "probe2", Enabled: true,
			MotionX: types.FiveHoleMotionAxisMapping{ControllerID: "mc-1", Axis: "X"},
			MotionY: types.FiveHoleMotionAxisMapping{ControllerID: "mc-2", Axis: "Y"},
		},
	}

	err := mc.MoveAllProbesToPoint(types.TraversalPoint{ID: "p1", X: 10, Y: 20}, probes, rectangleLayoutForDedup(), 1000)
	if err == nil || !strings.Contains(err.Error(), "物理轴冲突") {
		t.Fatalf("expected physical axis conflict error, got %v", err)
	}
}

// 共用轴位返回初始位置：同一物理轴只发一次命令
func TestMotionCoordinator_ReturnInitialPositions_SharedAxisDedup(t *testing.T) {
	var mu sync.Mutex
	calls := make(map[string]int)
	mover := func(controllerID string, axis types.AxisName, position float64) error {
		mu.Lock()
		calls[controllerID+"|"+string(axis)]++
		mu.Unlock()
		return nil
	}
	waiter := func(controllerID string, axis types.AxisName, timeoutMs int) error {
		return nil
	}
	mc := NewMotionCoordinator(mover, waiter)

	sharedX := types.FiveHoleMotionAxisMapping{ControllerID: "mc-shared", Axis: "X"}
	sharedY := types.FiveHoleMotionAxisMapping{ControllerID: "mc-shared", Axis: "Y"}
	probes := []types.FiveHoleProbeConfig{
		{ProbeID: "probe1", Enabled: true, MotionX: sharedX, MotionY: sharedY},
		{ProbeID: "probe2", Enabled: true, MotionX: sharedX, MotionY: sharedY},
	}
	// 共用轴位下两根探针保存的是同一物理轴的初始位置（相同值）
	initial := map[string]types.TraversalPoint{
		"probe1": {X: 1, Y: 2},
		"probe2": {X: 1, Y: 2},
	}

	err := mc.ReturnProbesToInitialPositions(initial, probes, 1000)
	if err != nil {
		t.Fatalf("ReturnProbesToInitialPositions failed: %v", err)
	}
	if calls["mc-shared|X"] != 1 || calls["mc-shared|Y"] != 1 {
		t.Fatalf("expected each shared axis moved exactly once, got %v", calls)
	}
}

// 共用轴位保存初始位置：同一物理轴只读一次，各探针共享同一读数
// （避免两次读数抖动导致返回初始位置时被去重逻辑判为"物理轴冲突"）
func TestCaptureInitialPositions_SharedAxisReadOnce(t *testing.T) {
	service := NewFiveHoleTraversalService(nil)
	var mu sync.Mutex
	readCalls := 0
	service.SetProbeAxisPositionGetter(func(controllerID string, axis types.AxisName) (float64, error) {
		mu.Lock()
		defer mu.Unlock()
		readCalls++
		// 模拟读数抖动：每次调用返回不同值
		return float64(100 + readCalls), nil
	})

	sharedX := types.FiveHoleMotionAxisMapping{ControllerID: "mc-s", Axis: "X"}
	sharedY := types.FiveHoleMotionAxisMapping{ControllerID: "mc-s", Axis: "Y"}
	config := types.FiveHoleTraversalConfig{
		Probes: []types.FiveHoleProbeConfig{
			{ProbeID: "probe1", Enabled: true, MotionX: sharedX, MotionY: sharedY},
			{ProbeID: "probe2", Enabled: true, MotionX: sharedX, MotionY: sharedY},
		},
	}

	positions := service.captureInitialPositions(config)
	// 2 根物理轴（X/Y）各只读一次，共 2 次（而非 2 探针 × 2 轴 = 4 次）
	if readCalls != 2 {
		t.Fatalf("expected 2 getter calls (one per physical axis), got %d", readCalls)
	}
	// 各探针共享同一读数（同轴同值 → 返回初始位置去重不会判冲突）
	if positions["probe1"] != positions["probe2"] {
		t.Fatalf("expected identical positions for shared axis, got %+v vs %+v", positions["probe1"], positions["probe2"])
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

	results, _, err := dp.ReadAllProbesRawData(probes, "dev_pAtm", 0, "dev_tAtm", 0, "", 0, nil)
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
			MotionX: types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "X"},
			MotionY: types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "Y"}},
		{ProbeID: "probe2", Enabled: true,
			MotionX: types.FiveHoleMotionAxisMapping{ControllerID: "c2", Axis: "X"},
			MotionY: types.FiveHoleMotionAxisMapping{ControllerID: "c2", Axis: "Y"}},
		{ProbeID: "probe3", Enabled: true,
			MotionX: types.FiveHoleMotionAxisMapping{ControllerID: "c3", Axis: "X"},
			MotionY: types.FiveHoleMotionAxisMapping{ControllerID: "c3", Axis: "Y"}},
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
