package three_hole

import (
	"fmt"

	"yx-daq/internal/types"
)

// ==================== 三孔落盘小数位 ====================
//
// 与前端 ThreeHoleTestView 的实时显示精度保持一致（界面显示几位，文件就写几位）：
//   - P1/P2/P3 按各角色通道配置的采集精度
//   - P∞/T∞ 按对应角色通道配置的采集精度
//   - 插值列用 types.WritePrec* 常量
//
// 前端对应实现：src/frontend/src/stores/threeHoleTest.ts + utils/writePrecision.ts

// ProbePrecision 三孔原始采集列的落盘小数位
type ProbePrecision struct {
	P1, P2, P3 int
	PAtm, TAtm int
}

// DefaultProbePrecision 未解析到通道配置时的回退精度（全部取 types.FallbackChannelPrecision）
func DefaultProbePrecision() ProbePrecision {
	d := types.FallbackChannelPrecision
	return ProbePrecision{P1: d, P2: d, P3: d, PAtm: d, TAtm: d}
}

// ResolveProbePrecision 按三孔配置的通道角色解析落盘小数位
// get 未注入（nil）或未命中通道时，对应列回退 types.FallbackChannelPrecision
func ResolveProbePrecision(config types.ThreeHoleTraversalConfig, get types.ChannelPrecisionGetter) ProbePrecision {
	prec := DefaultProbePrecision()
	for _, ch := range config.ProbeChannels {
		p := types.ResolveChannelPrecision(get, config.DeviceID, ch.Channel)
		switch ch.Role {
		case types.Role3H_P1:
			prec.P1 = p
		case types.Role3H_P2:
			prec.P2 = p
		case types.Role3H_P3:
			prec.P3 = p
		case types.Role3H_PAtm:
			prec.PAtm = p
		case types.Role3H_TAtm:
			prec.TAtm = p
		}
	}
	return prec
}

// formatPrec 按指定小数位格式化数值（CSV 落盘统一入口）
func formatPrec(val float64, prec int) string {
	return fmt.Sprintf("%.*f", prec, val)
}
