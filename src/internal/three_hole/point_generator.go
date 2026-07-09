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

// generateLinePoints 直线布点（单轴）
// 沿 line.Axis 方向从 Start 到 End 按步长 Step 取点，强制包含终点；另一轴固定为 Fixed
func generateLinePoints(line *types.LineLayout) []types.TraversalPoint {
	if line == nil {
		return nil
	}

	values := types.ExpandLineAxisValues(line.Start, line.End, line.Step)

	points := make([]types.TraversalPoint, 0, len(values))
	for i, v := range values {
		var pt types.TraversalPoint
		if line.Axis == types.LineAxisX {
			pt = types.TraversalPoint{ID: fmt.Sprintf("pt-%d", i), X: v, Y: line.Fixed}
		} else {
			pt = types.TraversalPoint{ID: fmt.Sprintf("pt-%d", i), X: line.Fixed, Y: v}
		}
		points = append(points, pt)
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