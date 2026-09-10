package types

import "fmt"

// 温度校准点数范围（与前端 MIN_POINTS/MAX_POINTS 同步）
const (
	TempCalibMinPoints = 2
	TempCalibMaxPoints = 10
)

// TempCalibPoint 温度校准单点：设备测量值 vs 手动输入的参考温度。
// 线性回归中 x = Measured，y = Reference。
type TempCalibPoint struct {
	Measured  float64 `json:"measured"`  // 设备实测温度
	Reference float64 `json:"reference"` // 参考温度（手动输入）
}

// TempCalibResult 线性回归拟合结果 y = a*x + b。
type TempCalibResult struct {
	A      float64 `json:"a"`      // 斜率
	B      float64 `json:"b"`      // 截距
	R2     float64 `json:"r2"`     // 决定系数
	Points int     `json:"points"` // 参与拟合的点数
}

// TempCalibSampleResult 温度校准单通道采点结果。
type TempCalibSampleResult struct {
	ChannelIndex int     `json:"channelIndex"` // 通道索引
	ChannelName  string  `json:"channelName"`  // 通道名称
	MeanValue    float64 `json:"meanValue"`    // 采样均值
	SampleCount  int     `json:"sampleCount"`  // 实际采样帧数
	Error        string  `json:"error"`        // 采样失败原因（成功时为空）
}

// FitLinearRegression 对给定校准点做线性回归 y = a*x + b，返回 a/b/R²。
// x = points[i].Measured，y = points[i].Reference。
// 点数范围 [TempCalibMinPoints, TempCalibMaxPoints]，超出拒绝。
// R² 用标准公式 1 - SS_res/SS_tot。
//
// 纯函数，无副作用，不依赖任何 Manager 状态，便于单元测试和前端预演。
func FitLinearRegression(points []TempCalibPoint) (TempCalibResult, error) {
	n := len(points)
	if n < TempCalibMinPoints {
		return TempCalibResult{}, fmt.Errorf("至少需要 %d 个校准点才能拟合，当前 %d 个", TempCalibMinPoints, n)
	}
	if n > TempCalibMaxPoints {
		return TempCalibResult{}, fmt.Errorf("校准点数不能超过 %d 个，当前 %d 个", TempCalibMaxPoints, n)
	}
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0
	for _, p := range points {
		sumX += p.Measured
		sumY += p.Reference
		sumXY += p.Measured * p.Reference
		sumX2 += p.Measured * p.Measured
	}
	denom := float64(n)*sumX2 - sumX*sumX
	if denom == 0 {
		return TempCalibResult{}, fmt.Errorf("校准点 x 值全部相同，无法拟合")
	}
	a := (float64(n)*sumXY - sumX*sumY) / denom
	b := (sumY - a*sumX) / float64(n)
	meanY := sumY / float64(n)
	ssRes, ssTot := 0.0, 0.0
	for _, p := range points {
		yPred := a*p.Measured + b
		ssRes += (p.Reference - yPred) * (p.Reference - yPred)
		ssTot += (p.Reference - meanY) * (p.Reference - meanY)
	}
	r2 := 1.0
	if ssTot > 0 {
		r2 = 1 - ssRes/ssTot
	}
	return TempCalibResult{A: a, B: b, R2: r2, Points: n}, nil
}
