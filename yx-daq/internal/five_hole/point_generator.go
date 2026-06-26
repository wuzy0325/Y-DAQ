package five_hole

import (
	"fmt"

	"yx-daq/internal/types"
)

// maxTraversalPoints 最大点位数量限制，防止配置不当导致内存溢出
const maxTraversalPoints = 50000

// generatePoints 根据布点配置生成测试点位
// 复用三孔 point_generator 逻辑（TraversalLayout 是共享类型）
func generatePoints(layout types.TraversalLayout) ([]types.TraversalPoint, error) {
	var points []types.TraversalPoint
	switch layout.Pattern {
	case types.TraversalPatternLine:
		points = generateLinePoints(layout.Line)
	case types.TraversalPatternRectangle:
		points = generateRectanglePoints(layout.Rectangle)
	case types.TraversalPatternCustom:
		points = layout.CustomPoints
	default:
		return nil, fmt.Errorf("不支持的布点模式: %s", layout.Pattern)
	}

	if len(points) == 0 {
		return nil, fmt.Errorf("布点配置生成 0 个点位")
	}
	if len(points) > maxTraversalPoints {
		return nil, fmt.Errorf("点位数 %d 超过上限 %d", len(points), maxTraversalPoints)
	}
	return points, nil
}

// generateLinePoints 直线/网格布点（当XSteps和YSteps都有值时生成X*Y网格点）
func generateLinePoints(line *types.LineLayout) []types.TraversalPoint {
	if line == nil {
		return nil
	}

	var points []types.TraversalPoint
	id := 0

	xValues := expandStepSegments(line.XSteps)
	yValues := expandStepSegments(line.YSteps)

	if len(xValues) == 0 && len(yValues) == 0 {
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
// 使用整数步数计算，避免浮点累加精度问题（照三孔实现）
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

// isLineSingleAxis 判断直线布点是否为单轴（仅一个方向变化）
// 用于 motion_coordinator 跳过静止轴
func isLineSingleAxis(line *types.LineLayout) (singleAxis bool, isXChange bool) {
	if line == nil {
		return false, false
	}
	xChange := line.StartX != line.EndX || len(line.XSteps) > 0
	yChange := line.StartY != line.EndY || len(line.YSteps) > 0
	// 仅一个方向变化 = 单轴
	if xChange != yChange {
		return true, xChange
	}
	return false, false
}
