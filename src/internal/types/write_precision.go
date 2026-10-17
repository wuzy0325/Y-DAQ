package types

// ==================== 数据落盘小数位（CSV 保存/导出） ====================
//
// 约定：保存文件的小数位与界面显示一致 —— 界面显示几位，文件就写几位。
//   - 原始采集列（P1..P5 / P∞ / T∞）按对应设备通道配置的 Precision 落盘
//   - 插值列按界面显示的固定位数落盘（见下方常量注释）
//   - 界面未显示的列（CAS/SAT/Qc/ρ/Vx/Vy/Vz）与点位坐标保持历史精度，避免无谓丢精度
//
// 前端对应实现：src/frontend/src/utils/writePrecision.ts（两侧规则必须同步修改）

const (
	// FallbackChannelPrecision 通道精度解析不到时的回退小数位
	// （与前端 FiveHoleTestView/ThreeHoleTestView 的 getChPrecision 默认值 3 一致）
	FallbackChannelPrecision = 3
	// maxChannelPrecision 设备通道精度上限（设备页 el-input-number :max=6）
	maxChannelPrecision = 6
)

// 插值列落盘小数位（与界面显示一致）
const (
	WritePrecPt       = 3 // 总压 Pt（界面 3 位）
	WritePrecPs       = 3 // 静压 Ps（界面 3 位）
	WritePrecMach     = 4 // 马赫数 Ma（界面 4 位）
	WritePrecAlpha    = 2 // 攻角 Alpha（界面 2 位）
	WritePrecBeta     = 2 // 侧滑角 Beta（界面 2 位）
	WritePrecVelocity = 2 // 速度 V（界面 2 位）
)

// 界面未显示的列：保持历史精度
const (
	WritePrecCAS     = 4 // 校正空速 CAS
	WritePrecSAT     = 4 // 静温 SAT
	WritePrecQc      = 6 // 动压 Qc
	WritePrecDensity = 6 // 密度 ρ
	WritePrecVxyz    = 4 // 三向速度分量 Vx/Vy/Vz
	// WritePrecPosition 点位坐标 X/Y（界面无对应数值列）
	WritePrecPosition = 4
)

// ChannelPrecisionGetter 返回指定设备通道配置的采集精度（小数位）。
// 设备/通道不存在时返回 ok=false，由 ResolveChannelPrecision 回退默认精度。
type ChannelPrecisionGetter func(deviceID string, channel int) (int, bool)

// ResolveChannelPrecision 解析通道落盘小数位：未注入 getter、未命中设备/通道，
// 或配置值越界（不在 0-maxChannelPrecision 内）时回退 FallbackChannelPrecision
func ResolveChannelPrecision(get ChannelPrecisionGetter, deviceID string, channel int) int {
	if get != nil && channel >= 0 {
		if p, ok := get(deviceID, channel); ok && p >= 0 && p <= maxChannelPrecision {
			return p
		}
	}
	return FallbackChannelPrecision
}
