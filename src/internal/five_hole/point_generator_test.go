package five_hole

import (
	"testing"

	"yx-daq/internal/types"
)

func TestGeneratePoints_Rectangle(t *testing.T) {
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternRectangle,
		Rectangle: &types.RectangleLayout{
			XMin: 0, XMax: 10,
			YMin: 0, YMax: 10,
			XSteps: []types.StepSegment{{Start: 0, End: 10, Step: 5}},  // 0,5,10 = 3 点
			YSteps: []types.StepSegment{{Start: 0, End: 10, Step: 5}},  // 0,5,10 = 3 点
		},
	}
	points, err := generatePoints(layout)
	if err != nil {
		t.Fatalf("generatePoints failed: %v", err)
	}
	// 3x3 = 9 点
	if len(points) != 9 {
		t.Fatalf("expected 9 points, got %d", len(points))
	}
}

func TestGeneratePoints_Line_SingleAxisX(t *testing.T) {
	// 单轴直线：仅 X 方向变化
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternLine,
		Line: &types.LineLayout{
			StartX: 0, StartY: 5,
			EndX: 10, EndY: 5,
			XSteps: []types.StepSegment{{Start: 0, End: 10, Step: 5}},  // 0,5,10
			YSteps: nil,
		},
	}
	points, err := generatePoints(layout)
	if err != nil {
		t.Fatalf("generatePoints failed: %v", err)
	}
	if len(points) != 3 {
		t.Fatalf("expected 3 points, got %d", len(points))
	}
	// 验证 Y 固定为 5
	for _, p := range points {
		if p.Y != 5 {
			t.Fatalf("expected Y=5, got %f", p.Y)
		}
	}
}

func TestGeneratePoints_Line_SingleAxisY(t *testing.T) {
	// 单轴直线：仅 Y 方向变化
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternLine,
		Line: &types.LineLayout{
			StartX: 5, StartY: 0,
			EndX: 5, EndY: 10,
			XSteps: nil,
			YSteps: []types.StepSegment{{Start: 0, End: 10, Step: 5}},
		},
	}
	points, err := generatePoints(layout)
	if err != nil {
		t.Fatalf("generatePoints failed: %v", err)
	}
	if len(points) != 3 {
		t.Fatalf("expected 3 points, got %d", len(points))
	}
	for _, p := range points {
		if p.X != 5 {
			t.Fatalf("expected X=5, got %f", p.X)
		}
	}
}

func TestGeneratePoints_Line_TwoAxis(t *testing.T) {
	// 双轴网格直线
	layout := types.TraversalLayout{
		Pattern: types.TraversalPatternLine,
		Line: &types.LineLayout{
			StartX: 0, StartY: 0,
			EndX: 10, EndY: 10,
			XSteps: []types.StepSegment{{Start: 0, End: 10, Step: 5}},  // 3 点
			YSteps: []types.StepSegment{{Start: 0, End: 10, Step: 5}},  // 3 点
		},
	}
	points, err := generatePoints(layout)
	if err != nil {
		t.Fatalf("generatePoints failed: %v", err)
	}
	if len(points) != 9 {
		t.Fatalf("expected 9 points, got %d", len(points))
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
	points, err := generatePoints(layout)
	if err != nil {
		t.Fatalf("generatePoints failed: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("expected 2 points, got %d", len(points))
	}
}

func TestGeneratePoints_EmptyLayout(t *testing.T) {
	layout := types.TraversalLayout{Pattern: types.TraversalPatternCustom}
	_, err := generatePoints(layout)
	if err == nil {
		t.Fatal("expected error for empty points")
	}
}

func TestIsLineSingleAxis_XChange(t *testing.T) {
	line := &types.LineLayout{
		StartX: 0, StartY: 5,
		EndX:   10, EndY: 5,
		XSteps: []types.StepSegment{{Start: 0, End: 10, Step: 5}},
	}
	single, isX := isLineSingleAxis(line)
	if !single {
		t.Fatal("expected single axis")
	}
	if !isX {
		t.Fatal("expected X change")
	}
}

func TestIsLineSingleAxis_YChange(t *testing.T) {
	line := &types.LineLayout{
		StartX: 5, StartY: 0,
		EndX:   5, EndY: 10,
		YSteps: []types.StepSegment{{Start: 0, End: 10, Step: 5}},
	}
	single, isX := isLineSingleAxis(line)
	if !single {
		t.Fatal("expected single axis")
	}
	if isX {
		t.Fatal("expected Y change (isX=false)")
	}
}

func TestIsLineSingleAxis_TwoAxis(t *testing.T) {
	line := &types.LineLayout{
		StartX: 0, StartY: 0,
		EndX:   10, EndY: 10,
		XSteps: []types.StepSegment{{Start: 0, End: 10, Step: 5}},
		YSteps: []types.StepSegment{{Start: 0, End: 10, Step: 5}},
	}
	single, _ := isLineSingleAxis(line)
	if single {
		t.Fatal("expected not single axis")
	}
}
