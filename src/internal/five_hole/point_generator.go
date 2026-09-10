package five_hole

import (
	"fmt"

	"yx-daq/internal/types"
)

// maxTraversalPoints 最大点位数量限制，防止配置不当导致内存溢出
const maxTraversalPoints = 50000

// generatePoints 根据五孔测试配置生成测试点位
// 五孔直线布点只用 MotionX（单轴），生成 point{X: v, Y: 0}；物理轴由每根探针的 MotionX 决定
func generatePoints(config types.FiveHoleTraversalConfig) ([]types.TraversalPoint, error) {
	var points []types.TraversalPoint
	switch config.Layout.Pattern {
	case types.TraversalPatternLine:
		points = generateLinePoints(config.Layout.Line)
	case types.TraversalPatternRectangle:
		points = generateRectanglePoints(config.Layout.Rectangle)
	case types.TraversalPatternFan:
		points = generateFanPoints(config.Layout.Fan)
	case types.TraversalPatternCustom:
		points = config.Layout.CustomPoints
	default:
		return nil, fmt.Errorf("不支持的布点模式: %s", config.Layout.Pattern)
	}

	if len(points) == 0 {
		return nil, fmt.Errorf("布点配置生成 0 个点位")
	}
	if len(points) > maxTraversalPoints {
		return nil, fmt.Errorf("点位数 %d 超过上限 %d", len(points), maxTraversalPoints)
	}
	return points, nil
}

// generateLinePoints 直线布点（单轴）
// 直线模式只用 MotionX，沿 Start→End 按步长 Step 取点，生成 point{X: v, Y: 0}
// 物理轴由每根探针的 MotionX 决定，line.Axis 字段不再使用
func generateLinePoints(line *types.LineLayout) []types.TraversalPoint {
	if line == nil {
		return nil
	}

	values := types.ExpandLineAxisValues(line.Start, line.End, line.Step)

	points := make([]types.TraversalPoint, 0, len(values))
	for i, v := range values {
		points = append(points, types.TraversalPoint{ID: fmt.Sprintf("pt-%d", i), X: v, Y: 0})
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

// generateFanPoints 扇形布点
// R 方向为线性轴，θ 方向为旋转轴
// 点位直接使用轴坐标：X=R（半径轴绝对位置，mm）、Y=θ（角度轴绝对位置，°）
// 数据点记录与 CSV 导出直接使用 point.X/Y，即实际轴位置；
// 前端预览画布自行把 (R, θ) 转为笛卡尔坐标绘制
func generateFanPoints(fan *types.FanLayout) []types.TraversalPoint {
	if fan == nil {
		return nil
	}

	rValues := expandStepSegments(fan.RSteps)
	thetaValues := expandStepSegments(fan.ThetaSteps)
	if len(rValues) == 0 || len(thetaValues) == 0 {
		return nil
	}

	points := make([]types.TraversalPoint, 0, len(rValues)*len(thetaValues))
	id := 0
	for _, r := range rValues {
		for _, thetaDeg := range thetaValues {
			points = append(points, types.TraversalPoint{
				ID: fmt.Sprintf("pt-%d", id),
				X:  r,
				Y:  thetaDeg,
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

