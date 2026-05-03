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

// TestGeneratePoints_LineBasic 测试基本直线布点
func TestGeneratePoints_LineBasic(t *testing.T) {
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternLine,
		Line: &types.LineLayout{
			StartX: 0,
			EndX:   10,
			StartY: 0,
			EndY:   5,
		},
	}

	points := generatePoints(layout)
	if len(points) != 2 {
		t.Errorf("Expected 2 points for basic line, got %d", len(points))
	}

	// 验证点位
	if points[0].ID != "pt-0" {
		t.Errorf("Expected first point ID 'pt-0', got '%s'", points[0].ID)
	}
	if points[0].X != 0 || points[0].Y != 0 {
		t.Errorf("Expected first point (0, 0), got (%.1f, %.1f)", points[0].X, points[0].Y)
	}

	if points[1].ID != "pt-1" {
		t.Errorf("Expected second point ID 'pt-1', got '%s'", points[1].ID)
	}
	if points[1].X != 10 || points[1].Y != 5 {
		t.Errorf("Expected second point (10, 5), got (%.1f, %.1f)", points[1].X, points[1].Y)
	}
}

// TestGeneratePoints_LineWithSteps 测试带步长的直线布点
func TestGeneratePoints_LineWithSteps(t *testing.T) {
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternLine,
		Line: &types.LineLayout{
			StartX: 0,
			EndX:   10,
			StartY: 0,
			EndY:   4,
			XSteps: []types.StepSegment{{Start: 0, End: 10, Step: 2}},
			YSteps: []types.StepSegment{{Start: 0, End: 4, Step: 2}},
		},
	}

	points := generatePoints(layout)
	if len(points) != 18 { // 6 X * 3 Y = 18 points
		t.Errorf("Expected 18 points for line with steps, got %d", len(points))
	}

	// 验证部分网格点（18个点太多，只验证前几个）
	expectedPoints := map[string][2]float64{
		"pt-0":  {0, 0},
		"pt-1":  {0, 2},
		"pt-2":  {0, 4},
		"pt-3":  {2, 0},
		"pt-4":  {2, 2},
		"pt-5":  {2, 4},
		"pt-6":  {4, 0},
		"pt-7":  {4, 2},
		"pt-8":  {4, 4},
		"pt-9":  {6, 0},
		"pt-10": {6, 2},
		"pt-11": {6, 4},
		"pt-12": {8, 0},
		"pt-13": {8, 2},
		"pt-14": {8, 4},
		"pt-15": {10, 0},
		"pt-16": {10, 2},
		"pt-17": {10, 4},
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

// TestGeneratePoints_LineInvalidStep 测试无效步长
func TestGeneratePoints_LineInvalidStep(t *testing.T) {
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternLine,
		Line: &types.LineLayout{
			StartX: 0,
			EndX:   10,
			StartY: 0,
			EndY:   5,
			XSteps: []types.StepSegment{{Start: 10, End: 0, Step: 2}}, // 倒序
		},
	}

	points := generatePoints(layout)
	// 倒序步长被忽略，但仍会生成起止点
	if len(points) != 2 {
		t.Errorf("Expected 2 points for invalid step direction (start and end points), got %d", len(points))
	}

	// 验证是起止点
	if points[0].X != 0 || points[1].X != 10 {
		t.Errorf("Expected start and end points, got (%.1f, %.1f) and (%.1f, %.1f)",
			points[0].X, points[0].Y, points[1].X, points[1].Y)
	}
}

// TestGeneratePoints_LineZeroStep 测试零步长
func TestGeneratePoints_LineZeroStep(t *testing.T) {
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternLine,
		Line: &types.LineLayout{
			StartX: 0,
			EndX:   10,
			StartY: 0,
			EndY:   5,
			XSteps: []types.StepSegment{{Start: 0, End: 10, Step: 0}}, // 零步长
		},
	}

	points := generatePoints(layout)
	if len(points) != 2 {
		t.Errorf("Expected 2 points for zero step (just endpoints), got %d", len(points))
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

// TestGenerateLinePoints_NoSteps 测试无步长的直线
func TestGenerateLinePoints_NoSteps(t *testing.T) {
	line := &types.LineLayout{
		StartX: 0,
		EndX:   10,
		StartY: 0,
		EndY:   5,
	}

	points := generateLinePoints(line)
	if len(points) != 2 {
		t.Errorf("Expected 2 points for no steps, got %d", len(points))
	}
}

// TestGenerateLinePoints_NegativeStep 测试负步长
func TestGenerateLinePoints_NegativeStep(t *testing.T) {
	line := &types.LineLayout{
		StartX: 0,
		EndX:   10,
		StartY: 0,
		EndY:   5,
		XSteps: []types.StepSegment{{Start: 0, End: 10, Step: -1}},
	}

	points := generateLinePoints(line)
	// 负步长被忽略，但仍会生成起止点
	if len(points) != 2 {
		t.Errorf("Expected 2 points for negative step (start and end points), got %d", len(points))
	}

	// 验证是起止点
	if points[0].X != 0 || points[1].X != 10 {
		t.Errorf("Expected start and end points, got (%.1f, %.1f) and (%.1f, %.1f)",
			points[0].X, points[0].Y, points[1].X, points[1].Y)
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

// TestGeneratePoints_ExactBoundary 测试精确边界值
func TestGeneratePoints_ExactBoundary(t *testing.T) {
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternLine,
		Line: &types.LineLayout{
			StartX: 0,
			EndX:   0, // X相同
			YSteps: []types.StepSegment{{Start: 0, End: 5, Step: 5}},
		},
	}

	points := generatePoints(layout)
	if len(points) != 2 {
		t.Errorf("Expected 2 points for same X, got %d", len(points))
	}
}