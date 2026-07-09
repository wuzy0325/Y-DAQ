package three_hole

import (
	"testing"

	"yx-daq/internal/types"
)

// TestGeneratePoints_EmptyLayout 测试空布局
func TestGeneratePoints_EmptyLayout(t *testing.T) {
	layout := types.TraversalLayout{}
	points := generatePoints(layout)
	if len(points) != 0 {
		t.Errorf("Expected 0 points for empty layout, got %d", len(points))
	}
}

// TestGeneratePoints_LineBasic 测试 X 轴直线布点
func TestGeneratePoints_LineBasic(t *testing.T) {
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternLine,
		Line: &types.LineLayout{
			Axis:  types.LineAxisX,
			Start: 0,
			End:   10,
			Step:  5,
			Fixed: 0,
		},
	}

	points := generatePoints(layout)
	if len(points) != 3 { // 0, 5, 10
		t.Fatalf("Expected 3 points for X-axis line, got %d", len(points))
	}

	// 验证点位：(0,0), (5,0), (10,0)
	expected := []struct {
		x, y float64
	}{
		{0, 0}, {5, 0}, {10, 0},
	}
	for i, pt := range points {
		if pt.X != expected[i].x || pt.Y != expected[i].y {
			t.Errorf("Point %d: expected (%.1f, %.1f), got (%.1f, %.1f)",
				i, expected[i].x, expected[i].y, pt.X, pt.Y)
		}
	}
}

// TestGeneratePoints_LineYAxis 测试 Y 轴直线布点
func TestGeneratePoints_LineYAxis(t *testing.T) {
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternLine,
		Line: &types.LineLayout{
			Axis:  types.LineAxisY,
			Start: 0,
			End:   10,
			Step:  5,
			Fixed: 7,
		},
	}

	points := generatePoints(layout)
	if len(points) != 3 {
		t.Fatalf("Expected 3 points for Y-axis line, got %d", len(points))
	}

	// 验证点位：(7,0), (7,5), (7,10)
	expected := []struct {
		x, y float64
	}{
		{7, 0}, {7, 5}, {7, 10},
	}
	for i, pt := range points {
		if pt.X != expected[i].x || pt.Y != expected[i].y {
			t.Errorf("Point %d: expected (%.1f, %.1f), got (%.1f, %.1f)",
				i, expected[i].x, expected[i].y, pt.X, pt.Y)
		}
	}
}

// TestGeneratePoints_LineReverse 测试反向直线（start > end）
func TestGeneratePoints_LineReverse(t *testing.T) {
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternLine,
		Line: &types.LineLayout{
			Axis:  types.LineAxisX,
			Start: 10,
			End:   0,
			Step:  5,
			Fixed: 0,
		},
	}

	points := generatePoints(layout)
	if len(points) != 3 { // 10, 5, 0
		t.Fatalf("Expected 3 points for reverse line, got %d", len(points))
	}

	expected := []float64{10, 5, 0}
	for i, pt := range points {
		if pt.X != expected[i] {
			t.Errorf("Point %d: expected X=%.1f, got %.1f", i, expected[i], pt.X)
		}
	}
}

// TestGeneratePoints_LineNotDivisible 测试步长不整除时强制包含终点
func TestGeneratePoints_LineNotDivisible(t *testing.T) {
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternLine,
		Line: &types.LineLayout{
			Axis:  types.LineAxisX,
			Start: 0,
			End:   10,
			Step:  3,
			Fixed: 0,
		},
	}

	points := generatePoints(layout)
	// 0, 3, 6, 9, 10（最后一点强制为 end=10）
	if len(points) != 5 {
		t.Fatalf("Expected 5 points (step not divisible, end included), got %d", len(points))
	}
	if points[0].X != 0 {
		t.Errorf("Expected first X=0, got %.1f", points[0].X)
	}
	if points[len(points)-1].X != 10 {
		t.Errorf("Expected last X=10 (end forced), got %.1f", points[len(points)-1].X)
	}
}

// TestGeneratePoints_LineZeroStep 测试步长 <= 0（仅起点）
func TestGeneratePoints_LineZeroStep(t *testing.T) {
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternLine,
		Line: &types.LineLayout{
			Axis:  types.LineAxisX,
			Start: 0,
			End:   10,
			Step:  0, // 零步长
			Fixed: 0,
		},
	}

	points := generatePoints(layout)
	if len(points) != 1 {
		t.Fatalf("Expected 1 point for zero step, got %d", len(points))
	}
	if points[0].X != 0 {
		t.Errorf("Expected X=0, got %.1f", points[0].X)
	}
}

// TestGeneratePoints_LineStartEqualsEnd 测试起点等于终点
func TestGeneratePoints_LineStartEqualsEnd(t *testing.T) {
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternLine,
		Line: &types.LineLayout{
			Axis:  types.LineAxisX,
			Start: 5,
			End:   5,
			Step:  1,
			Fixed: 3,
		},
	}

	points := generatePoints(layout)
	if len(points) != 1 {
		t.Fatalf("Expected 1 point for start==end, got %d", len(points))
	}
	if points[0].X != 5 || points[0].Y != 3 {
		t.Errorf("Expected (5, 3), got (%.1f, %.1f)", points[0].X, points[0].Y)
	}
}

// TestGeneratePoints_RectangleBasic 测试基本矩形布点
func TestGeneratePoints_RectangleBasic(t *testing.T) {
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternRectangle,
		Rectangle: &types.RectangleLayout{
			XMin: 0, XMax: 10,
			YMin: 0, YMax: 5,
		},
	}

	points := generatePoints(layout)
	if len(points) != 4 {
		t.Errorf("Expected 4 points for basic rectangle, got %d", len(points))
	}

	// 验证四个角
	expectedPoints := map[string][2]float64{
		"pt-0": {0, 0},
		"pt-1": {0, 5},
		"pt-2": {10, 0},
		"pt-3": {10, 5},
	}

	for _, point := range points {
		expected, exists := expectedPoints[point.ID]
		if !exists {
			t.Errorf("Unexpected point ID: %s", point.ID)
			continue
		}
		if point.X != expected[0] || point.Y != expected[1] {
			t.Errorf("Point %s: expected (%.1f, %.1f), got (%.1f, %.1f)",
				point.ID, expected[0], expected[1], point.X, point.Y)
		}
	}
}

// TestGeneratePoints_RectangleWithSteps 测试带步长的矩形布点
func TestGeneratePoints_RectangleWithSteps(t *testing.T) {
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternRectangle,
		Rectangle: &types.RectangleLayout{
			XMin: 0, XMax: 4,
			YMin: 0, YMax: 2,
			XSteps: []types.StepSegment{{Start: 0, End: 4, Step: 2}},
			YSteps: []types.StepSegment{{Start: 0, End: 2, Step: 1}},
		},
	}

	points := generatePoints(layout)
	if len(points) != 9 { // 3 X * 3 Y = 9 points
		t.Errorf("Expected 9 points for rectangle with steps, got %d", len(points))
	}

	// 验证网格点
	expectedPoints := map[string][2]float64{
		"pt-0": {0, 0},
		"pt-1": {0, 1},
		"pt-2": {0, 2},
		"pt-3": {2, 0},
		"pt-4": {2, 1},
		"pt-5": {2, 2},
		"pt-6": {4, 0},
		"pt-7": {4, 1},
		"pt-8": {4, 2},
	}

	for _, point := range points {
		expected, exists := expectedPoints[point.ID]
		if !exists {
			t.Errorf("Unexpected point ID: %s", point.ID)
			continue
		}
		if point.X != expected[0] || point.Y != expected[1] {
			t.Errorf("Point %s: expected (%.1f, %.1f), got (%.1f, %.1f)",
				point.ID, expected[0], expected[1], point.X, point.Y)
		}
	}
}

// TestGeneratePoints_RectangleInvalidRange 测试无效矩形范围
func TestGeneratePoints_RectangleInvalidRange(t *testing.T) {
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternRectangle,
		Rectangle: &types.RectangleLayout{
			XMin: 10, XMax: 0, // 反向
			YMin: 0, YMax: 5,
		},
	}

	points := generatePoints(layout)
	if len(points) != 0 {
		t.Errorf("Expected 0 points for invalid rectangle range, got %d", len(points))
	}
}

// TestGeneratePoints_CustomPoints 测试自定义点位
func TestGeneratePoints_CustomPoints(t *testing.T) {
	customPoints := []types.TraversalPoint{
		{ID: "custom-1", X: 10, Y: 20},
		{ID: "custom-2", X: 30, Y: 40},
	}

	layout := types.TraversalLayout{
		Pattern:     types.TraversalPatternCustom,
		CustomPoints: customPoints,
	}

	points := generatePoints(layout)
	if len(points) != 2 {
		t.Errorf("Expected 2 custom points, got %d", len(points))
	}

	if points[0].ID != "custom-1" || points[1].ID != "custom-2" {
		t.Error("Custom points should preserve original IDs")
	}
}

// TestGeneratePoints_UnsupportedPattern 测试不支持的图案
func TestGeneratePoints_UnsupportedPattern(t *testing.T) {
	layout := types.TraversalLayout{
		Pattern: "unsupported", // 不支持的图案
	}

	points := generatePoints(layout)
	if len(points) != 0 {
		t.Errorf("Expected 0 points for unsupported pattern, got %d", len(points))
	}
}

// TestGenerateLinePoints_Empty 测试空直线布局
func TestGenerateLinePoints_Empty(t *testing.T) {
	points := generateLinePoints(nil)
	if points != nil {
		t.Error("Expected nil for nil line layout")
	}
}

// TestGenerateLinePoints_NoSteps 测试步长 > 距离（仅起止两点，中间无点）
func TestGenerateLinePoints_NoSteps(t *testing.T) {
	line := &types.LineLayout{
		Axis:  types.LineAxisX,
		Start: 0,
		End:   10,
		Step:  20, // 步长 > 距离，仅 start 和 end
		Fixed: 0,
	}

	points := generateLinePoints(line)
	if len(points) != 2 {
		t.Fatalf("Expected 2 points (step > delta), got %d", len(points))
	}
	if points[0].X != 0 || points[1].X != 10 {
		t.Errorf("Expected (0, 10), got (%.1f, %.1f)", points[0].X, points[1].X)
	}
}

// TestGenerateLinePoints_NegativeStep 测试负步长（视为无效，仅起点）
func TestGenerateLinePoints_NegativeStep(t *testing.T) {
	line := &types.LineLayout{
		Axis:  types.LineAxisX,
		Start: 0,
		End:   10,
		Step:  -1, // 负步长
		Fixed: 0,
	}

	points := generateLinePoints(line)
	// 负步长视为无效，仅生成起点
	if len(points) != 1 {
		t.Fatalf("Expected 1 point for negative step, got %d", len(points))
	}
	if points[0].X != 0 {
		t.Errorf("Expected X=0, got %.1f", points[0].X)
	}
}

// TestGenerateRectanglePoints_Empty 测试空矩形布局
func TestGenerateRectanglePoints_Empty(t *testing.T) {
	points := generateRectanglePoints(nil)
	if points != nil {
		t.Error("Expected nil for nil rectangle layout")
	}
}

// TestExpandStepSegments_Basic 测试基本步段展开
func TestExpandStepSegments_Basic(t *testing.T) {
	segments := []types.StepSegment{
		{Start: 0, End: 5, Step: 1},
	}

	result := expandStepSegments(segments)
	if len(result) != 6 { // 0,1,2,3,4,5
		t.Errorf("Expected 6 values, got %d", len(result))
	}

	expected := []float64{0, 1, 2, 3, 4, 5}
	for i, val := range result {
		if val != expected[i] {
			t.Errorf("Expected %.1f at index %d, got %.1f", expected[i], i, val)
		}
	}
}

// TestExpandStepSegments_ZeroStep 测试零步长
func TestExpandStepSegments_ZeroStep(t *testing.T) {
	segments := []types.StepSegment{
		{Start: 0, End: 5, Step: 0},
	}

	result := expandStepSegments(segments)
	if len(result) != 2 {
		t.Errorf("Expected 2 values (start and end), got %d", len(result))
	}

	if result[0] != 0 || result[1] != 5 {
		t.Errorf("Expected [0, 5], got [%.1f, %.1f]", result[0], result[1])
	}
}

// TestExpandStepSegments_InvalidDirection 测试无效方向
func TestExpandStepSegments_InvalidDirection(t *testing.T) {
	segments := []types.StepSegment{
		{Start: 5, End: 0, Step: 1}, // 反向
	}

	result := expandStepSegments(segments)
	if len(result) != 0 {
		t.Errorf("Expected 0 values for invalid direction, got %d", len(result))
	}
}

// TestExpandStepSegments_SmallCount 测试小数量步段
func TestExpandStepSegments_SmallCount(t *testing.T) {
	segments := []types.StepSegment{
		{Start: 0, End: 2, Step: 1},
	}

	result := expandStepSegments(segments)
	if len(result) != 3 {
		t.Errorf("Expected 3 values, got %d", len(result))
	}
}

// TestGeneratePoints_LargeDataset 测试大数据集（验证限制）
func TestGeneratePoints_LargeDataset(t *testing.T) {
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternRectangle,
		Rectangle: &types.RectangleLayout{
			XMin: 0, XMax: 100,
			YMin: 0, YMax: 100,
			XSteps: []types.StepSegment{{Start: 0, End: 100, Step: 1}}, // 101 points
			YSteps: []types.StepSegment{{Start: 0, End: 100, Step: 1}}, // 101 points
		},
	}

	points := generatePoints(layout)
	// 101 * 101 = 10201 points, should be less than maxTraversalPoints (50000)
	if len(points) == 0 {
		t.Error("Expected some points for large dataset")
	}

	if len(points) > 50000 {
		t.Errorf("Expected points to be limited to 50000, got %d", len(points))
	}
}

// TestGeneratePoints_ExactBoundary 测试整除场景（终点正好落在步长上）
func TestGeneratePoints_ExactBoundary(t *testing.T) {
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternLine,
		Line: &types.LineLayout{
			Axis:  types.LineAxisX,
			Start: 0,
			End:   10,
			Step:  5, // 整除：0, 5, 10
			Fixed: 0,
		},
	}

	points := generatePoints(layout)
	if len(points) != 3 {
		t.Errorf("Expected 3 points for exact boundary, got %d", len(points))
	}
	// 终点必须包含
	if points[len(points)-1].X != 10 {
		t.Errorf("Expected last X=10, got %.1f", points[len(points)-1].X)
	}
}