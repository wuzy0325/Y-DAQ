package five_hole

import (
	"math"
	"testing"

	"yx-daq/internal/types"
)

const floatEpsilon = 1e-9

// makePGConfig 把布点布局包装成五孔 generatePoints 所需的最小配置
func makePGConfig(layout types.TraversalLayout) types.FiveHoleTraversalConfig {
	return types.FiveHoleTraversalConfig{
		Layout: layout,
		Probes: []types.FiveHoleProbeConfig{
			{
				ProbeID: "p1", Enabled: true,
				MotionX: types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "X"},
				MotionY: types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "Y"},
			},
		},
	}
}

func TestGeneratePoints_Rectangle(t *testing.T) {
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternRectangle,
		Rectangle: &types.RectangleLayout{
			XMin: 0, XMax: 10,
			YMin: 0, YMax: 10,
			XSteps: []types.StepSegment{{Start: 0, End: 10, Step: 5}}, // 0,5,10 = 3 点
			YSteps: []types.StepSegment{{Start: 0, End: 10, Step: 5}}, // 0,5,10 = 3 点
		},
	}
	points, err := generatePoints(makePGConfig(layout))
	if err != nil {
		t.Fatalf("generatePoints failed: %v", err)
	}
	// 3x3 = 9 点
	if len(points) != 9 {
		t.Fatalf("expected 9 points, got %d", len(points))
	}
}

func TestGeneratePoints_Line_XAxis(t *testing.T) {
	// X 方向直线：axis=X, start=0, end=10, step=5, fixed=0
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternLine,
		Line: &types.LineLayout{
			Axis:  "X",
			Start: 0,
			End:   10,
			Step:  5,
			Fixed: 0,
		},
	}
	points, err := generatePoints(makePGConfig(layout))
	if err != nil {
		t.Fatalf("generatePoints failed: %v", err)
	}
	if len(points) != 3 { // 0, 5, 10
		t.Fatalf("expected 3 points, got %d", len(points))
	}
	// 验证 Y 固定为 0
	for _, p := range points {
		if p.Y != 0 {
			t.Fatalf("expected Y=0, got %f", p.Y)
		}
	}
	// 验证 X 序列
	expectedX := []float64{0, 5, 10}
	for i, p := range points {
		if p.X != expectedX[i] {
			t.Fatalf("point %d: expected X=%f, got %f", i, expectedX[i], p.X)
		}
	}
}

func TestGeneratePoints_Line_AxisIgnored(t *testing.T) {
	// 直线模式不再读 Axis 字段：Axis=Y 也仍然 X 变化、Y=0
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
	points, err := generatePoints(makePGConfig(layout))
	if err != nil {
		t.Fatalf("generatePoints failed: %v", err)
	}
	if len(points) != 3 {
		t.Fatalf("expected 3 points, got %d", len(points))
	}
	// Axis 字段被忽略，永远是 X 变化、Y=0
	expectedX := []float64{0, 5, 10}
	for i, p := range points {
		if p.X != expectedX[i] {
			t.Fatalf("point %d: expected X=%f, got %f", i, expectedX[i], p.X)
		}
		if p.Y != 0 {
			t.Fatalf("point %d: expected Y=0, got %f", i, p.Y)
		}
	}
}

func TestGeneratePoints_Line_Reverse(t *testing.T) {
	// 反向直线：start=10, end=0
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternLine,
		Line: &types.LineLayout{
			Axis:  "X",
			Start: 10,
			End:   0,
			Step:  5,
			Fixed: 0,
		},
	}
	points, err := generatePoints(makePGConfig(layout))
	if err != nil {
		t.Fatalf("generatePoints failed: %v", err)
	}
	if len(points) != 3 { // 10, 5, 0
		t.Fatalf("expected 3 points for reverse, got %d", len(points))
	}
	expectedX := []float64{10, 5, 0}
	for i, p := range points {
		if p.X != expectedX[i] {
			t.Fatalf("point %d: expected X=%f, got %f", i, expectedX[i], p.X)
		}
	}
}

func TestGeneratePoints_Line_NotDivisible(t *testing.T) {
	// 步长不整除：start=0, end=10, step=3 → 0,3,6,9,10（强制包含终点）
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternLine,
		Line: &types.LineLayout{
			Axis:  "X",
			Start: 0,
			End:   10,
			Step:  3,
			Fixed: 0,
		},
	}
	points, err := generatePoints(makePGConfig(layout))
	if err != nil {
		t.Fatalf("generatePoints failed: %v", err)
	}
	if len(points) != 5 {
		t.Fatalf("expected 5 points (end forced), got %d", len(points))
	}
	if points[0].X != 0 {
		t.Fatalf("expected first X=0, got %f", points[0].X)
	}
	if points[len(points)-1].X != 10 {
		t.Fatalf("expected last X=10 (end forced), got %f", points[len(points)-1].X)
	}
}

func TestGeneratePoints_Line_StartEqualsEnd(t *testing.T) {
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternLine,
		Line: &types.LineLayout{
			Axis:  "X",
			Start: 5,
			End:   5,
			Step:  1,
			Fixed: 3,
		},
	}
	points, err := generatePoints(makePGConfig(layout))
	if err != nil {
		t.Fatalf("generatePoints failed: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected 1 point for start==end, got %d", len(points))
	}
	if points[0].X != 5 || points[0].Y != 0 {
		t.Fatalf("expected (5, 0), got (%f, %f)", points[0].X, points[0].Y)
	}
}

func TestGeneratePoints_Line_ZeroStep(t *testing.T) {
	// 步长 <= 0：仅起点
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternLine,
		Line: &types.LineLayout{
			Axis:  "X",
			Start: 0,
			End:   10,
			Step:  0,
			Fixed: 0,
		},
	}
	points, err := generatePoints(makePGConfig(layout))
	if err != nil {
		t.Fatalf("generatePoints failed: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected 1 point for zero step, got %d", len(points))
	}
}

func TestGeneratePoints_Custom(t *testing.T) {
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternCustom,
		CustomPoints: []types.TraversalPoint{
			{ID: "c1", X: 1, Y: 2},
			{ID: "c2", X: 3, Y: 4},
		},
	}
	points, err := generatePoints(makePGConfig(layout))
	if err != nil {
		t.Fatalf("generatePoints failed: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("expected 2 points, got %d", len(points))
	}
}

func TestGeneratePoints_EmptyLayout(t *testing.T) {
	layout := types.TraversalLayout{Pattern: types.TraversalPatternCustom}
	_, err := generatePoints(makePGConfig(layout))
	if err == nil {
		t.Fatal("expected error for empty points")
	}
}

func TestGeneratePoints_Fan_SingleRadiusSingleAngle(t *testing.T) {
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternFan,
		Fan: &types.FanLayout{
			RSteps:     []types.StepSegment{{Start: 10, End: 10, Step: 1}},
			ThetaSteps: []types.StepSegment{{Start: 30, End: 30, Step: 1}},
			RAxis:      "X",
			ThetaAxis:  "U",
		},
	}
	points, err := generatePoints(makePGConfig(layout))
	if err != nil {
		t.Fatalf("generatePoints failed: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected 1 point, got %d", len(points))
	}
	// 点位为轴坐标：X=R 轴绝对位置，Y=θ 轴绝对位置
	if points[0].X != 10 || points[0].Y != 30 {
		t.Fatalf("first point should be axis position (10, 30), got (%f, %f)", points[0].X, points[0].Y)
	}
}

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) <= floatEpsilon
}

func TestGeneratePoints_Fan_MultiRadiusSingleAngle(t *testing.T) {
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternFan,
		Fan: &types.FanLayout{
			RSteps:     []types.StepSegment{{Start: 0, End: 10, Step: 5}}, // 0, 5, 10
			ThetaSteps: []types.StepSegment{{Start: 0, End: 0, Step: 1}},   // 0
			RAxis:      "X",
			ThetaAxis:  "U",
		},
	}
	points, err := generatePoints(makePGConfig(layout))
	if err != nil {
		t.Fatalf("generatePoints failed: %v", err)
	}
	if len(points) != 3 {
		t.Fatalf("expected 3 points, got %d", len(points))
	}
	expected := []types.TraversalPoint{
		{X: 0, Y: 0},
		{X: 5, Y: 0},
		{X: 10, Y: 0},
	}
	for i, p := range points {
		if !approxEqual(p.X, expected[i].X) || !approxEqual(p.Y, expected[i].Y) {
			t.Fatalf("point %d: expected (%f, %f), got (%f, %f)", i, expected[i].X, expected[i].Y, p.X, p.Y)
		}
	}
}

func TestGeneratePoints_Fan_MultiRadiusMultiAngle(t *testing.T) {
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternFan,
		Fan: &types.FanLayout{
			RSteps:     []types.StepSegment{{Start: 0, End: 10, Step: 10}}, // 0, 10
			ThetaSteps: []types.StepSegment{{Start: 0, End: 90, Step: 90}},  // 0, 90
			RAxis:      "X",
			ThetaAxis:  "U",
		},
	}
	points, err := generatePoints(makePGConfig(layout))
	if err != nil {
		t.Fatalf("generatePoints failed: %v", err)
	}
	if len(points) != 4 {
		t.Fatalf("expected 4 points, got %d", len(points))
	}
	// 顺序：先遍历 θ，再遍历 R（与矩形保持一致）
	// 点位为轴坐标：X=R（mm），Y=θ（°）
	expected := []types.TraversalPoint{
		{X: 0, Y: 0},   // R=0, θ=0
		{X: 0, Y: 90},  // R=0, θ=90
		{X: 10, Y: 0},  // R=10, θ=0
		{X: 10, Y: 90}, // R=10, θ=90
	}
	for i, p := range points {
		if !approxEqual(p.X, expected[i].X) || !approxEqual(p.Y, expected[i].Y) {
			t.Fatalf("point %d: expected (%f, %f), got (%f, %f)", i, expected[i].X, expected[i].Y, p.X, p.Y)
		}
	}
}

func TestGeneratePoints_Fan_NonZeroStart(t *testing.T) {
	// 非零起始半径 5、起始角度 30°，验证点位为轴绝对坐标
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternFan,
		Fan: &types.FanLayout{
			RSteps:     []types.StepSegment{{Start: 5, End: 15, Step: 10}}, // 5, 15
			ThetaSteps: []types.StepSegment{{Start: 30, End: 30, Step: 1}},  // 30
			RAxis:      "X",
			ThetaAxis:  "U",
		},
	}
	points, err := generatePoints(makePGConfig(layout))
	if err != nil {
		t.Fatalf("generatePoints failed: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("expected 2 points, got %d", len(points))
	}
	if !approxEqual(points[0].X, 5) || !approxEqual(points[0].Y, 30) {
		t.Fatalf("first point should be axis position (5, 30), got (%f, %f)", points[0].X, points[0].Y)
	}
	// 第二点：R 轴位置 15，θ 轴位置 30
	if !approxEqual(points[1].X, 15) || !approxEqual(points[1].Y, 30) {
		t.Fatalf("second point should be (15, 30), got (%f, %f)", points[1].X, points[1].Y)
	}
}
