// PRB 网格推导与定位工具。
//
// 历史实现把 PRB 网格写死为 13×13（-30..30 步长 5），导致校准布点不是
// 标准 ±30° 时无法生成/加载标定表。本文件把网格约束放宽为"任意完整
// 矩形网格"：α/β 各自的网格线由数据自身推导（distinct 值升序），
// 仅要求每轴 ≥2 条线、所有 α×β 组合恰好各出现一次。
// 历史 13×13 文件是该定义的特例，行为不变。
package interpolation

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// prbHeader PRB 文件表头：两条网格轴各自的点数。
type prbHeader struct {
	alphaCount int
	betaCount  int
}

// parsePrbHeader 解析表头行 "<alphaCount> <betaCount>"。
//
// 历史文件均为 "13 13"；语义约定为 α 轴点数在前、β 轴点数在后
// （方形网格下二者相等，不存在歧义；生成侧 windlabx4 按同一约定写出）。
func parsePrbHeader(line string) (prbHeader, error) {
	parts := strings.Fields(line)
	if len(parts) != 2 {
		return prbHeader{}, fmt.Errorf("PRB文件表头必须包含两个网格维度")
	}
	var width, height int
	if _, err := fmt.Sscanf(parts[0], "%d", &width); err != nil {
		return prbHeader{}, fmt.Errorf("PRB文件表头α维度解析失败")
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &height); err != nil {
		return prbHeader{}, fmt.Errorf("PRB文件表头β维度解析失败")
	}
	if width < 2 || height < 2 {
		return prbHeader{}, fmt.Errorf("PRB文件表头维度必须≥2（实际 %d %d）", width, height)
	}
	return prbHeader{alphaCount: width, betaCount: height}, nil
}

// deriveGridLines 从数据行推导 α/β 网格线（distinct 值升序）。
//
// 网格线值直接来自文件文本解析结果（同值字符串解析为同一 float64），
// 因此用精确相等收集 distinct 是安全的；不依赖等差步长——
// 非均匀布点（如密集中心 + 稀疏边缘）同样可加载。
func deriveGridLines(table []probeTableRow) (alphas, betas []float64) {
	alphaSet := make(map[float64]struct{}, len(table))
	betaSet := make(map[float64]struct{}, len(table))
	for _, row := range table {
		alphaSet[row.Alpha] = struct{}{}
		betaSet[row.Beta] = struct{}{}
	}
	alphas = sortedGridValues(alphaSet)
	betas = sortedGridValues(betaSet)
	return alphas, betas
}

func sortedGridValues(set map[float64]struct{}) []float64 {
	out := make([]float64, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	sort.Float64s(out)
	return out
}

// validateGridCompleteness 校验网格线与数据行构成完整矩形。
//
// 条件：线数与表头声明一致；α线数×β线数==行数。配合调用方的
// "无重复网格点"校验，这三条共同保证每个 α×β 组合恰好出现一次
// （无缺漏、无多余）。
func validateGridCompleteness(table []probeTableRow, header prbHeader, alphas, betas []float64) error {
	if len(alphas) != header.alphaCount {
		return fmt.Errorf("PRB表头声明α维度 %d，实际数据α线数 %d", header.alphaCount, len(alphas))
	}
	if len(betas) != header.betaCount {
		return fmt.Errorf("PRB表头声明β维度 %d，实际数据β线数 %d", header.betaCount, len(betas))
	}
	if want := header.alphaCount * header.betaCount; len(table) != want {
		return fmt.Errorf("PRB数据行数 %d 与网格规模 %d×%d=%d 不符",
			len(table), header.alphaCount, header.betaCount, want)
	}
	return nil
}

// resolveOutputCellAxis 在升序网格线中定位 angle 所在单元的 [下线, 上线]。
//
// 越界（超出首/末线）时钳制到最外层单元：输出值插值（CPT/CPS）不外推，
// 与"网格内钳制"的既有策略一致。angle 恒由 resolveRegion9 的双线性
// 参数（已钳制到 [0,1]）重建，正常路径不会越界；此处钳制仅是防御。
func resolveOutputCellAxis(lines []float64, angle float64) (float64, float64) {
	last := len(lines) - 1
	if angle <= lines[0] {
		return lines[0], lines[1]
	}
	if angle >= lines[last] {
		return lines[last-1], lines[last]
	}
	// 二分找最大 i 使 lines[i] <= angle（此时必有 lines[i+1] > angle）。
	lo, hi := 0, last-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if lines[mid] <= angle {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lines[lo], lines[lo+1]
}

// gridPointKey 生成网格点的 map 键（全精度，支持非整数角度布点）。
//
// 历史实现用 %.0f 隐含要求整数角度；布点布局允许一位小数精度
// （如 ±25° 步长 2.5°），两个不同角度可能被舍入到同一键导致错配。
// 全精度格式化对整数角度输出不变（"-30"、"5"），对小数角度输出
// 精确表示（"12.5"），历史文件键值不受影响。
func gridPointKey(alpha, beta float64) string {
	// 归一化负零：FormatFloat 对 -0 输出 "-0"，会与正零键 "0" 错配。
	if alpha == 0 {
		alpha = 0
	}
	if beta == 0 {
		beta = 0
	}
	return strconv.FormatFloat(alpha, 'f', -1, 64) + "," + strconv.FormatFloat(beta, 'f', -1, 64)
}
