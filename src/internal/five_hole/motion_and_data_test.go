package five_hole

import (
	"fmt"
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

	err := mc.ReturnProbesToInitialPositions(initial, probes, rectangleLayoutForDedup(), 1000)
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

// ===== 扇形布点分相位运动测试 =====

// fanLayoutForPhase 构建扇形布点配置：R 0→10 步长10，θ 0→30 步长30
func fanLayoutForPhase() types.TraversalLayout {
	return types.TraversalLayout{
		Pattern: types.TraversalPatternFan,
		Fan: &types.FanLayout{
			RSteps:     []types.StepSegment{{Start: 0, End: 10, Step: 10}},
			ThetaSteps: []types.StepSegment{{Start: 0, End: 30, Step: 30}},
		},
	}
}

// 扇形布点：先旋转（θ/MotionY 轴）移动并等待完成，再平移（R/MotionX 轴）
// 单探针单轴场景下相位内事件顺序确定，可断言精确序列
func TestMotionCoordinator_FanRotateBeforeTranslate(t *testing.T) {
	var mu sync.Mutex
	var events []string
	mover := func(controllerID string, axis types.AxisName, position float64) error {
		mu.Lock()
		events = append(events, "move:"+string(axis))
		mu.Unlock()
		return nil
	}
	waiter := func(controllerID string, axis types.AxisName, timeoutMs int) error {
		mu.Lock()
		events = append(events, "wait:"+string(axis))
		mu.Unlock()
		return nil
	}
	mc := NewMotionCoordinator(mover, waiter)

	// MotionX→R（线性，Z 轴），MotionY→θ（旋转，U 轴）
	probes := []types.FiveHoleProbeConfig{
		{
			ProbeID: "probe1", Enabled: true,
			MotionX: types.FiveHoleMotionAxisMapping{ControllerID: "c-lin", Axis: "Z"},
			MotionY: types.FiveHoleMotionAxisMapping{ControllerID: "c-rot", Axis: "U"},
		},
	}
	// 轴坐标 (R=10, θ=0)：X 即 R 轴绝对位置，Y 即 θ 轴绝对位置
	point := types.TraversalPoint{ID: "p1", X: 10, Y: 0}

	if err := mc.MoveAllProbesToPoint(point, probes, fanLayoutForPhase(), 1000); err != nil {
		t.Fatalf("MoveAllProbesToPoint failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	want := []string{"move:U", "wait:U", "move:Z", "wait:Z"}
	if len(events) != len(want) {
		t.Fatalf("expected %d events, got %v", len(want), events)
	}
	for i, w := range want {
		if events[i] != w {
			t.Fatalf("event[%d]=%s, want %s (full: %v)", i, events[i], w, events)
		}
	}
}

// 多探针扇形布点：旋转相位全部事件（含等待）必须早于平移相位的首个移动事件
// （相位间严格串行：先旋转到位，后平移）
func TestMotionCoordinator_FanMultiProbePhaseBoundary(t *testing.T) {
	var mu sync.Mutex
	var events []string
	mover := func(controllerID string, axis types.AxisName, position float64) error {
		mu.Lock()
		events = append(events, "move:"+string(axis))
		mu.Unlock()
		return nil
	}
	waiter := func(controllerID string, axis types.AxisName, timeoutMs int) error {
		mu.Lock()
		events = append(events, "wait:"+string(axis))
		mu.Unlock()
		return nil
	}
	mc := NewMotionCoordinator(mover, waiter)

	// 2 探针各自独立控制器：MotionX→R（Z 线性），MotionY→θ（U 旋转）
	probes := []types.FiveHoleProbeConfig{
		{
			ProbeID: "probe1", Enabled: true,
			MotionX: types.FiveHoleMotionAxisMapping{ControllerID: "c1-lin", Axis: "Z"},
			MotionY: types.FiveHoleMotionAxisMapping{ControllerID: "c1-rot", Axis: "U"},
		},
		{
			ProbeID: "probe2", Enabled: true,
			MotionX: types.FiveHoleMotionAxisMapping{ControllerID: "c2-lin", Axis: "Z"},
			MotionY: types.FiveHoleMotionAxisMapping{ControllerID: "c2-rot", Axis: "U"},
		},
	}
	point := types.TraversalPoint{ID: "p1", X: 10, Y: 0}

	if err := mc.MoveAllProbesToPoint(point, probes, fanLayoutForPhase(), 1000); err != nil {
		t.Fatalf("MoveAllProbesToPoint failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	// 2 探针 × (move+wait) × 2 相位 = 8 事件
	if len(events) != 8 {
		t.Fatalf("expected 8 events, got %v", events)
	}
	lastRotary, firstLinear := -1, -1
	for i, e := range events {
		if strings.HasSuffix(e, ":U") {
			lastRotary = i
		}
		if strings.HasSuffix(e, ":Z") && firstLinear == -1 {
			firstLinear = i
		}
	}
	if lastRotary == -1 || firstLinear == -1 {
		t.Fatalf("missing rotary/linear events: %v", events)
	}
	if lastRotary > firstLinear {
		t.Fatalf("rotary events must all precede linear events, got %v", events)
	}
}

// 扇形布点：旋转（θ）相位失败时不得发起平移（R）相位
func TestMotionCoordinator_FanRotateFailureSkipsTranslate(t *testing.T) {
	var mu sync.Mutex
	linearMoved := false
	mover := func(controllerID string, axis types.AxisName, position float64) error {
		if axis == types.AxisU {
			return fmt.Errorf("simulated rotary failure")
		}
		mu.Lock()
		linearMoved = true
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
			MotionX: types.FiveHoleMotionAxisMapping{ControllerID: "c-lin", Axis: "Z"},
			MotionY: types.FiveHoleMotionAxisMapping{ControllerID: "c-rot", Axis: "U"},
		},
	}
	point := types.TraversalPoint{ID: "p1", X: 10, Y: 0}

	err := mc.MoveAllProbesToPoint(point, probes, fanLayoutForPhase(), 1000)
	if err == nil || !strings.Contains(err.Error(), "旋转(θ)相位失败") {
		t.Fatalf("expected rotary phase failure error, got %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if linearMoved {
		t.Fatal("linear (R) axis must not move when rotary phase failed")
	}
}

// 扇形布点：θ 相 waiter 超时必须阻断 R 相发起（轴未确认到位不得进入下一相位）
func TestMotionCoordinator_FanWaitTimeoutBlocksTranslate(t *testing.T) {
	var mu sync.Mutex
	linearMoved := false
	mover := func(controllerID string, axis types.AxisName, position float64) error {
		if axis == types.AxisZ {
			mu.Lock()
			linearMoved = true
			mu.Unlock()
		}
		return nil
	}
	waiter := func(controllerID string, axis types.AxisName, timeoutMs int) error {
		if axis == types.AxisU {
			return fmt.Errorf("simulated wait timeout")
		}
		return nil
	}
	mc := NewMotionCoordinator(mover, waiter)

	probes := []types.FiveHoleProbeConfig{
		{
			ProbeID: "probe1", Enabled: true,
			MotionX: types.FiveHoleMotionAxisMapping{ControllerID: "c-lin", Axis: "Z"},
			MotionY: types.FiveHoleMotionAxisMapping{ControllerID: "c-rot", Axis: "U"},
		},
	}
	point := types.TraversalPoint{ID: "p1", X: 10, Y: 0}

	err := mc.MoveAllProbesToPoint(point, probes, fanLayoutForPhase(), 1000)
	if err == nil || !strings.Contains(err.Error(), "旋转(θ)相位失败") {
		t.Fatalf("expected rotary phase wait-timeout error, got %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if linearMoved {
		t.Fatal("linear (R) axis must not move when rotary phase wait timed out")
	}
}

// 扇形布点返回初始位置：同样先旋转（Y=θ）后平移（X=R）
func TestMotionCoordinator_FanReturnInitialRotaryFirst(t *testing.T) {
	var mu sync.Mutex
	var events []string
	mover := func(controllerID string, axis types.AxisName, position float64) error {
		mu.Lock()
		events = append(events, "move:"+string(axis))
		mu.Unlock()
		return nil
	}
	waiter := func(controllerID string, axis types.AxisName, timeoutMs int) error {
		mu.Lock()
		events = append(events, "wait:"+string(axis))
		mu.Unlock()
		return nil
	}
	mc := NewMotionCoordinator(mover, waiter)

	probes := []types.FiveHoleProbeConfig{
		{
			ProbeID: "probe1", Enabled: true,
			MotionX: types.FiveHoleMotionAxisMapping{ControllerID: "c-lin", Axis: "Z"},
			MotionY: types.FiveHoleMotionAxisMapping{ControllerID: "c-rot", Axis: "U"},
		},
	}
	initial := map[string]types.TraversalPoint{
		"probe1": {X: 0, Y: 0},
	}

	if err := mc.ReturnProbesToInitialPositions(initial, probes, fanLayoutForPhase(), 1000); err != nil {
		t.Fatalf("ReturnProbesToInitialPositions failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	want := []string{"move:U", "wait:U", "move:Z", "wait:Z"}
	if len(events) != len(want) {
		t.Fatalf("expected %d events, got %v", len(want), events)
	}
	for i, w := range want {
		if events[i] != w {
			t.Fatalf("event[%d]=%s, want %s (full: %v)", i, events[i], w, events)
		}
	}
}

// 扇形布点：点位即轴坐标（X=R 绝对位置，Y=θ 绝对位置），轴目标直接透传不做反推，
// θ 超过 ±180° 时不得绕回（如 θ=270° 不能变成 -90°）
func TestMotionCoordinator_FanAxisTargetPassThrough(t *testing.T) {
	var mu sync.Mutex
	targets := make(map[string]float64)
	mover := func(controllerID string, axis types.AxisName, position float64) error {
		mu.Lock()
		targets[string(axis)] = position
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
			MotionX: types.FiveHoleMotionAxisMapping{ControllerID: "c-lin", Axis: "Z"},
			MotionY: types.FiveHoleMotionAxisMapping{ControllerID: "c-rot", Axis: "U"},
		},
	}
	// 轴坐标点位：R=150mm，θ=270°（非零起始，超出 ±180°）
	point := types.TraversalPoint{ID: "p1", X: 150, Y: 270}

	if err := mc.MoveAllProbesToPoint(point, probes, fanLayoutForPhase(), 1000); err != nil {
		t.Fatalf("MoveAllProbesToPoint failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if targets["Z"] != 150 {
		t.Fatalf("R axis target should be 150, got %f", targets["Z"])
	}
	if targets["U"] != 270 {
		t.Fatalf("θ axis target should be 270 (no wrap-around), got %f", targets["U"])
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
			case "dev_manual":
				// probe2 使用 device 模式读取不同 PAtm/TAtm
				switch ch {
				case 3:
					result[ch] = 98.765
				case 7:
					result[ch] = 30.1
				}
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
			// probe1：设备读取 PAtm/TAtm
			PAtmSource: types.FiveHoleAtmSource{Mode: types.FiveHoleSourceDevice, DeviceID: "dev_pAtm", Channel: 0},
			TAtmSource: types.FiveHoleAtmSource{Mode: types.FiveHoleSourceDevice, DeviceID: "dev_tAtm", Channel: 0},
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
			// probe2：PAtm 设备读取（不同设备/通道），TAtm 手动写入
			PAtmSource: types.FiveHoleAtmSource{Mode: types.FiveHoleSourceDevice, DeviceID: "dev_manual", Channel: 3},
			TAtmSource: types.FiveHoleAtmSource{Mode: types.FiveHoleSourceManual, ManualValue: 15.5},
		},
	}

	results, _, err := dp.ReadAllProbesRawData(probes, nil)
	if err != nil {
		t.Fatalf("ReadAllProbesRawData failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// 验证 probe1（设备读取 PAtm/TAtm）
	if results[0].P1 != 100.0 || results[0].P5 != 99.5 {
		t.Fatalf("probe1 rawData mismatch: %+v", results[0])
	}
	if results[0].PAtm != 101.325 || results[0].TAtm != 20.5 {
		t.Fatalf("probe1 PAtm/TAtm mismatch: %+v", results[0])
	}
	// 验证 probe2（PAtm 设备读取独立值，TAtm 手动值）
	if results[1].P1 != 200.0 || results[1].P5 != 199.5 {
		t.Fatalf("probe2 rawData mismatch: %+v", results[1])
	}
	if results[1].PAtm != 98.765 || results[1].TAtm != 15.5 {
		t.Fatalf("probe2 PAtm/TAtm mismatch: %+v", results[1])
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
