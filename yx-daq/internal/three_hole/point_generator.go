package three_hole

import (
	"fmt"

	"yx-daq/internal/types"
)

// maxTraversalPoints 最大点位数量限制，防止配置不当导致内存溢出
const maxTraversalPoints = 50000

// generatePoints 根据布点配置生成测试点位
func generatePoints(layout types.TraversalLayout) []types.TraversalPoint {
	switch layout.Pattern {
	case types.TraversalPatternLine:
		return generateLinePoints(layout.Line)
	case types.TraversalPatternRectangle:
		return generateRectanglePoints(layout.Rectangle)
	case types.TraversalPatternCustom:
		return layout.CustomPoints
	default:
		return []types.TraversalPoint{}
	}
}

// generateLinePoints 直线/网格布点（当XSteps和YSteps都有值时生成X*Y网格点）
func generateLinePoints(line *types.LineLayout) []types.TraversalPoint {
	if line == nil {
		return nil
	}

	var points []types.TraversalPoint
	id := 0

	// 生成X方向点位
	xValues := expandStepSegments(line.XSteps)
	yValues := expandStepSegments(line.YSteps)

	if len(xValues) == 0 && len(yValues) == 0 {
		// 如果没有分段步长，直接用起止点
		points = append(points, types.TraversalPoint{ID: fmt.Sprintf("pt-%d", id), X: line.StartX, Y: line.StartY})
		id++
		points = append(points, types.TraversalPoint{ID: fmt.Sprintf("pt-%d", id), X: line.EndX, Y: line.EndY})
		return points
	}

	if len(yValues) == 0 {
		yValues = []float64{line.StartY}
	}
	if len(xValues) == 0 {
		xValues = []float64{line.StartX}
	}

	for _, x := range xValues {
		for _, y := range yValues {
			points = append(points, types.TraversalPoint{
				ID: fmt.Sprintf("pt-%d", id),
				X:  x,
				Y:  y,
			})
			id++
		}
	}

	return points
}

// generateRectanglePoints 矩形布点
func generateRectanglePoints(rect *types.RectangleLayout) []types.TraversalPoint {
	if rect == nil {
		return nil
	}
	if rect.XMin > rect.XMax || rect.YMin > rect.YMax {
		return nil
	}

	var points []types.TraversalPoint
	id := 0

	xValues := expandStepSegments(rect.XSteps)
	yValues := expandStepSegments(rect.YSteps)

	// 如果没有分段步长，使用默认步长
	if len(xValues) == 0 {
		xValues = []float64{rect.XMin, rect.XMax}
	}
	if len(yValues) == 0 {
		yValues = []float64{rect.YMin, rect.YMax}
	}

	for _, x := range xValues {
		for _, y := range yValues {
			points = append(points, types.TraversalPoint{
				ID: fmt.Sprintf("pt-%d", id),
				X:  x,
				Y:  y,
			})
			id++
		}
	}

	return points
}

// expandStepSegments 展开分段步长为具体数值列表
func expandStepSegments(segments []types.StepSegment) []float64 {
	var values []float64
	for _, seg := range segments {
		if seg.Start > seg.End {
			continue
		}
		if seg.Step == 0 {
			values = append(values, seg.Start, seg.End)
			continue
		}
		if seg.Step < 0 {
			continue
		}
		// 使用整数步数计算，避免浮点累加精度问题
		n := int((seg.End-seg.Start)/seg.Step + 0.5)
		if n < 0 || n > maxTraversalPoints {
			continue
		}
		for i := 0; i <= n; i++ {
			values = append(values, seg.Start+float64(i)*seg.Step)
		}
	}
	return values
}