package calibration

import (
	"math"

	"yx-daq/internal/types"
)

// CalculateFiveHoleCoefficients 计算五孔探针系数
func CalculateFiveHoleCoefficients(data types.FiveHoleRawData) types.FiveHoleCoefficients {
	// 四孔平均压力
	pAvg := (data.P2 + data.P3 + data.P4 + data.P5) / 4.0

	// 分母（避免除零）
	denominator := data.P1 - pAvg
	safeDenominator := denominator
	if math.Abs(denominator) < 1e-6 {
		safeDenominator = 1e-6
	}

	// 攻角系数 Kα
	kalpha := (data.P2 - data.P3) / safeDenominator

	// 侧滑角系数 Kβ
	kbeta := (data.P4 - data.P5) / safeDenominator

	// 总压系数 CPT
	cpt := 0.0
	if data.PTotal != nil && *data.PTotal > data.PAtm {
		cpt = (data.P1 - data.PAtm) / (*data.PTotal - data.PAtm)
	}

	// 静压系数 CPS
	cps := 0.0
	if data.P1 > data.PAtm {
		cps = (pAvg - data.PAtm) / (data.P1 - data.PAtm)
	}

	return types.FiveHoleCoefficients{
		Kalpha: kalpha,
		Kbeta:  kbeta,
		CPT:    cpt,
		CPS:    cps,
	}
}

// CalculateAverage 计算多次采样的平均值
func CalculateAverage(samples []types.FiveHoleRawData) types.FiveHoleRawData {
	if len(samples) == 0 {
		return types.FiveHoleRawData{}
	}

	n := float64(len(samples))
	result := types.FiveHoleRawData{}

	for _, s := range samples {
		result.P1 += s.P1
		result.P2 += s.P2
		result.P3 += s.P3
		result.P4 += s.P4
		result.P5 += s.P5
		result.PAtm += s.PAtm
		result.TAtm += s.TAtm
		if s.PTotal != nil {
			if result.PTotal == nil {
				pt := 0.0
				result.PTotal = &pt
			}
			*result.PTotal += *s.PTotal
		}
	}

	result.P1 /= n
	result.P2 /= n
	result.P3 /= n
	result.P4 /= n
	result.P5 /= n
	result.PAtm /= n
	result.TAtm /= n
	if result.PTotal != nil {
		*result.PTotal /= n
	}

	return result
}

// CalculateStdDev 计算标准差
func CalculateStdDev(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}

	n := float64(len(values))
	mean := 0.0
	for _, v := range values {
		mean += v
	}
	mean /= n

	variance := 0.0
	for _, v := range values {
		diff := v - mean
		variance += diff * diff
	}
	variance /= (n - 1)

	return math.Sqrt(variance)
}

// MatchChannelByRole 按角色+名称双重匹配通道
func MatchChannelByRole(role types.ProbeChannelRole, name string) bool {
	switch role {
	case types.RoleP1:
		return name == "P1" || name == "孔1压力" || contains(name, "孔1")
	case types.RoleP2:
		return name == "P2" || name == "孔2压力" || contains(name, "孔2")
	case types.RoleP3:
		return name == "P3" || name == "孔3压力" || contains(name, "孔3")
	case types.RoleP4:
		return name == "P4" || name == "孔4压力" || contains(name, "孔4")
	case types.RoleP5:
		return name == "P5" || name == "孔5压力" || contains(name, "孔5")
	case types.RolePAtm:
		return name == "大气压力" || name == "P∞" || contains(name, "大气压")
	case types.RoleTAtm:
		return name == "大气温度" || name == "T∞" || contains(name, "大气温")
	case types.RolePTotal:
		return name == "总压" || name == "Pt" || contains(name, "总压")
	default:
		return false
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
