package five_hole

import (
	"fmt"

	"yx-daq/internal/types"
)

// formatPrec 按指定小数位格式化数值（CSV 落盘统一入口）
func formatPrec(val float64, prec int) string {
	return fmt.Sprintf("%.*f", prec, val)
}

// ==================== 五孔落盘小数位 ====================
//
// 与前端 FiveHoleTestView 的实时显示精度保持一致（界面显示几位，文件就写几位）：
//   - P1..P5 按各探针各通道配置的采集精度
//   - P∞/T∞ 按数据源：device 模式取该设备通道精度，manual 模式用回退精度
//   - 插值列用 types.WritePrec* 常量
//
// 前端对应实现：src/frontend/src/stores/fiveHoleTest.ts + utils/writePrecision.ts

// ProbePrecision 单探针原始采集列的落盘小数位
type ProbePrecision struct {
	P1, P2, P3, P4, P5 int
	PAtm, TAtm         int
}

// DefaultProbePrecision 未解析到通道配置时的回退精度（全部取 types.FallbackChannelPrecision）
func DefaultProbePrecision() ProbePrecision {
	d := types.FallbackChannelPrecision
	return ProbePrecision{P1: d, P2: d, P3: d, P4: d, P5: d, PAtm: d, TAtm: d}
}

// ResolveProbePrecision 按探针通道配置与 P∞/T∞ 数据源解析落盘小数位
// get 未注入（nil）或未命中通道时，对应列回退 types.FallbackChannelPrecision
func ResolveProbePrecision(probe types.FiveHoleProbeConfig, get types.ChannelPrecisionGetter) ProbePrecision {
	prec := DefaultProbePrecision()
	for _, ch := range probe.ProbeChannels {
		p := types.ResolveChannelPrecision(get, ch.DeviceID, ch.Channel)
		switch ch.Role {
		case types.Role5H_P1:
			prec.P1 = p
		case types.Role5H_P2:
			prec.P2 = p
		case types.Role5H_P3:
			prec.P3 = p
		case types.Role5H_P4:
			prec.P4 = p
		case types.Role5H_P5:
			prec.P5 = p
		}
	}
	prec.PAtm = resolveAtmPrecision(probe.PAtmSource, get)
	prec.TAtm = resolveAtmPrecision(probe.TAtmSource, get)
	return prec
}

// ResolveProbesPrecision 解析多个探针的落盘小数位（key=ProbeID，仅含启用探针）
func ResolveProbesPrecision(probes []types.FiveHoleProbeConfig, get types.ChannelPrecisionGetter) map[string]ProbePrecision {
	result := make(map[string]ProbePrecision, len(probes))
	for _, p := range probes {
		if !p.Enabled {
			continue
		}
		result[p.ProbeID] = ResolveProbePrecision(p, get)
	}
	return result
}

// resolveAtmPrecision 解析 P∞/T∞ 落盘小数位
// manual 模式（或未配置设备）用回退精度，与前端 getAtmPrecision 的显示行为一致
func resolveAtmPrecision(src types.FiveHoleAtmSource, get types.ChannelPrecisionGetter) int {
	if src.Mode != types.FiveHoleSourceDevice || src.DeviceID == "" {
		return types.FallbackChannelPrecision
	}
	return types.ResolveChannelPrecision(get, src.DeviceID, src.Channel)
}
