package three_hole

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"

	"yx-daq/internal/types"
)

// ==================== 常量 ====================

const (
	maxIterations    = 20          // 最大迭代次数
	convergenceTol  = 1e-4        // 收敛容差
	dpZeroThreshold = 1e-6        // ΔP 判零阈值
	gamma           = 1.4         // 比热比
	gammaRatio      = (gamma - 1) / gamma // 0.2857
	machCoeff       = 2 / (gamma - 1)     // 5
)

// ==================== 插值器 ====================

// ThreeHoleInterpolator 三孔探针插值器
type ThreeHoleInterpolator struct {
	calibData []types.ThreeHoleCalibData // 所有校准数据
	alphaOri  []float64                  // 攻角序列（共用）
	initMa    float64                    // 初始马赫数（所有CMa平均值）
	minMa     float64                    // 马赫数下限
	maxMa     float64                    // 马赫数上限
	loaded    bool
}

// NewThreeHoleInterpolator 创建三孔插值器
func NewThreeHoleInterpolator() *ThreeHoleInterpolator {
	return &ThreeHoleInterpolator{}
}

// LoadCalibFiles 加载多个校准文件
func (interp *ThreeHoleInterpolator) LoadCalibFiles(filePaths []string) error {
	var allData []types.ThreeHoleCalibData

	for _, fp := range filePaths {
		data, err := parseCalibFile(fp)
		if err != nil {
			return fmt.Errorf("parse calib file %s failed: %w", fp, err)
		}
		allData = append(allData, *data)
	}

	if len(allData) == 0 {
		return fmt.Errorf("no calibration data loaded")
	}

	// 按 CMa 排序
	sort.Slice(allData, func(i, j int) bool {
		return allData[i].CMa < allData[j].CMa
	})

	// 校验所有校准文件的攻角序列一致
	refEntries := allData[0].Entries
	for k := 1; k < len(allData); k++ {
		if len(allData[k].Entries) != len(refEntries) {
			return fmt.Errorf("calib file %d has %d entries, expected %d (same as file 0)", k, len(allData[k].Entries), len(refEntries))
		}
		for i := range refEntries {
			if math.Abs(allData[k].Entries[i].Alpha-refEntries[i].Alpha) > 1e-6 {
				return fmt.Errorf("calib file %d entry %d alpha=%.6f differs from file 0 alpha=%.6f", k, i, allData[k].Entries[i].Alpha, refEntries[i].Alpha)
			}
		}
	}

	interp.calibData = allData

	// 提取攻角序列（所有文件共用，已校验一致）
	interp.alphaOri = make([]float64, len(refEntries))
	for i, e := range refEntries {
		interp.alphaOri[i] = e.Alpha
	}

	// 计算初始马赫数和范围
	sumMa := 0.0
	interp.minMa = allData[0].CMa
	interp.maxMa = allData[0].CMa
	for _, d := range allData {
		sumMa += d.CMa
		if d.CMa < interp.minMa {
			interp.minMa = d.CMa
		}
		if d.CMa > interp.maxMa {
			interp.maxMa = d.CMa
		}
	}
	interp.initMa = sumMa / float64(len(allData))
	interp.loaded = true

	return nil
}

// IsLoaded 是否已加载校准数据
func (interp *ThreeHoleInterpolator) IsLoaded() bool {
	return interp.loaded
}

// GetCalibInfo 获取校准文件信息
func (interp *ThreeHoleInterpolator) GetCalibInfo() []types.ThreeHoleCalibFileInfo {
	infos := make([]types.ThreeHoleCalibFileInfo, len(interp.calibData))
	for i, d := range interp.calibData {
		infos[i] = types.ThreeHoleCalibFileInfo{
			CMa: d.CMa,
		}
	}
	return infos
}

// Calculate 执行三孔插值计算
// 输入：三孔原始压力数据
// 输出：插值结果（总压、静压、马赫数、攻角）
func (interp *ThreeHoleInterpolator) Calculate(rawData types.ThreeHoleRawData) types.ThreeHoleInterpolationResult {
	if !interp.loaded {
		return types.ThreeHoleInterpolationResult{Valid: false}
	}

	P1 := rawData.P1
	P2 := rawData.P2
	P3 := rawData.P3
	Pa := rawData.PAtm

	// 步骤1：计算压力差分 ΔP 和压力系数比 Kb_temp
	deltaP := 2*P2 - P1 - P3

	// ΔP 接近零的异常处理：ΔP≈0 只代表攻角接近零，无法通过Kb反查，标记结果无效
	if math.Abs(deltaP) < dpZeroThreshold {
		return types.ThreeHoleInterpolationResult{
			PtProbe:        P2,
			PsProbe:        P2,
			MachProbe:      0,
			AlphaProbe:     0,
			IterationCount: 0,
			Valid:          false,
		}
	}

	kbTemp := (P3 - P1) / deltaP

	// Kb_temp 为无穷大或非数值
	if math.IsInf(kbTemp, 0) || math.IsNaN(kbTemp) {
		return types.ThreeHoleInterpolationResult{
			PtProbe:        P2,
			PsProbe:        P2,
			MachProbe:      interp.initMa,
			AlphaProbe:     0,
			IterationCount: 0,
			Valid:          false,
		}
	}

	// 步骤2：迭代求解马赫数和攻角
	maCurrent := interp.initMa
	iterationCount := 0

	for i := 0; i < maxIterations; i++ {
		iterationCount = i + 1

		// 子步骤A：二维插值查找
		_, kt, sb, ok := interp.interpolate2D(kbTemp, maCurrent)
		if !ok {
			// 插值失败，回退
			return types.ThreeHoleInterpolationResult{
				PtProbe:        P2,
				PsProbe:        P2,
				MachProbe:      maCurrent,
				AlphaProbe:     0,
				IterationCount: iterationCount,
				Valid:          true,
			}
		}

		// 子步骤B：计算总压和静压
		pt := P2 + kt*deltaP
		ps := pt - sb*deltaP

		// 子步骤C：计算新的马赫数
		maNew := calculateMachNumber(pt, ps, Pa)

		// 子步骤D：收敛判断
		if math.Abs(maNew-maCurrent) < convergenceTol {
			maCurrent = clampMa(maNew, interp.minMa, interp.maxMa)
			break
		}

		// 子步骤E：限制范围并更新
		maCurrent = clampMa(maNew, interp.minMa, interp.maxMa)
	}

	// 步骤3：用最终马赫数做一次最终插值
	alphaFinal, ktFinal, sbFinal, ok := interp.interpolate2D(kbTemp, maCurrent)
	if !ok {
		return types.ThreeHoleInterpolationResult{
			PtProbe:        P2,
			PsProbe:        P2,
			MachProbe:      maCurrent,
			AlphaProbe:     0,
			IterationCount: iterationCount,
			Valid:          true,
		}
	}

	ptProbe := P2 + ktFinal*deltaP
	psProbe := ptProbe - sbFinal*deltaP
	machProbe := calculateMachNumber(ptProbe, psProbe, Pa)

	return types.ThreeHoleInterpolationResult{
		PtProbe:        ptProbe,
		PsProbe:        psProbe,
		MachProbe:      machProbe,
		AlphaProbe:     alphaFinal,
		IterationCount: iterationCount,
		Valid:          true,
	}
}

// ==================== 二维插值核心算法 ====================

// interpolate2D 核心二维插值：在给定马赫数下，根据 Kb 值反查攻角和系数
func (interp *ThreeHoleInterpolator) interpolate2D(kbTemp float64, maCurrent float64) (alpha, kt, sb float64, ok bool) {
	if len(interp.calibData) == 0 {
		return 0, 0, 0, false
	}

	// 第一步：选取两个最近的校准马赫数
	calib1, calib2 := interp.findNearestTwoCalib(maCurrent)

	// 第二步：在马赫数方向线性插值，构建当前马赫数下的系数表
	kbAlphaMap := interp.interpolateInMaDirection(calib1, calib2, maCurrent)

	if len(kbAlphaMap) == 0 {
		return 0, 0, 0, false
	}

	// 第三步：按 Kb 值升序排序
	sort.Slice(kbAlphaMap, func(i, j int) bool {
		return kbAlphaMap[i].Kb < kbAlphaMap[j].Kb
	})

	// 第四步：在 Kb 方向插值反查攻角和系数
	return interp.interpolateInKbDirection(kbAlphaMap, kbTemp)
}

// findNearestTwoCalib 找到包围当前马赫数的两个校准数据
// 保证 calib1.CMa <= calib2.CMa，且 maCurrent 落在 [calib1.CMa, calib2.CMa] 区间内
// 超出校准范围时，取最近的两个相邻校准点
func (interp *ThreeHoleInterpolator) findNearestTwoCalib(maCurrent float64) (calib1, calib2 types.ThreeHoleCalibData) {
	n := len(interp.calibData)
	if n == 1 {
		return interp.calibData[0], interp.calibData[0]
	}

	// calibData 已按 CMa 升序排列，找到 maCurrent 落在哪两个相邻校准马赫数之间
	for i := 0; i < n-1; i++ {
		if maCurrent <= interp.calibData[i+1].CMa {
			return interp.calibData[i], interp.calibData[i+1]
		}
	}

	// maCurrent 超出最大校准马赫数，取最后两个
	return interp.calibData[n-2], interp.calibData[n-1]
}

// kbAlphaEntry 马赫数插值后的映射条目
type kbAlphaEntry struct {
	Kb    float64
	Alpha float64
	Kt    float64
	Sb    float64
}

// interpolateInMaDirection 在马赫数方向线性插值
func (interp *ThreeHoleInterpolator) interpolateInMaDirection(calib1, calib2 types.ThreeHoleCalibData, maCurrent float64) []kbAlphaEntry {
	n := len(calib1.Entries)
	if n == 0 {
		return nil
	}

	// 计算插值比例
	ratio := 0.0
	if math.Abs(calib2.CMa-calib1.CMa) > dpZeroThreshold {
		ratio = (maCurrent - calib1.CMa) / (calib2.CMa - calib1.CMa)
		ratio = math.Max(0, math.Min(1, ratio))
	}

	result := make([]kbAlphaEntry, n)
	for i := 0; i < n; i++ {
		e1 := calib1.Entries[i]
		// 如果 calib2 的条目数不够，用 calib1 的
		var e2 types.ThreeHoleCalibEntry
		if i < len(calib2.Entries) {
			e2 = calib2.Entries[i]
		} else {
			e2 = e1
		}

		result[i] = kbAlphaEntry{
			Kb:    e1.Kb + ratio*(e2.Kb-e1.Kb),
			Alpha: e1.Alpha + ratio*(e2.Alpha-e1.Alpha),
			Kt:    e1.Kt + ratio*(e2.Kt-e1.Kt),
			Sb:    e1.Sb + ratio*(e2.Sb-e1.Sb),
		}
	}

	return result
}

// interpolateInKbDirection 在 Kb 方向插值反查攻角和系数
func (interp *ThreeHoleInterpolator) interpolateInKbDirection(kbAlphaMap []kbAlphaEntry, kbTemp float64) (alpha, kt, sb float64, ok bool) {
	n := len(kbAlphaMap)
	if n == 0 {
		return 0, 0, 0, false
	}

	// 边界处理：Kb_temp 小于等于最小值
	if kbTemp <= kbAlphaMap[0].Kb {
		e := kbAlphaMap[0]
		return e.Alpha, e.Kt, e.Sb, true
	}

	// 边界处理：Kb_temp 大于等于最大值
	if kbTemp >= kbAlphaMap[n-1].Kb {
		e := kbAlphaMap[n-1]
		return e.Alpha, e.Kt, e.Sb, true
	}

	// 线性搜索找到相邻两点
	for j := 0; j < n-1; j++ {
		if kbAlphaMap[j].Kb <= kbTemp && kbTemp <= kbAlphaMap[j+1].Kb {
			j0 := kbAlphaMap[j]
			j1 := kbAlphaMap[j+1]

			denom := j1.Kb - j0.Kb
			if math.Abs(denom) < dpZeroThreshold {
				return j0.Alpha, j0.Kt, j0.Sb, true
			}

			r := (kbTemp - j0.Kb) / denom
			alpha = j0.Alpha + r*(j1.Alpha-j0.Alpha)
			kt = j0.Kt + r*(j1.Kt-j0.Kt)
			sb = j0.Sb + r*(j1.Sb-j0.Sb)
			return alpha, kt, sb, true
		}
	}

	// 未找到（不应到达此处）
	e := kbAlphaMap[n-1]
	return e.Alpha, e.Kt, e.Sb, true
}

// ==================== 辅助函数 ====================

// calculateMachNumber 根据总压、静压和大气压计算马赫数
// Ma = sqrt(5 * |((Pt+Pa)/(Ps+Pa))^0.2857 - 1|)
func calculateMachNumber(pt, ps, pa float64) float64 {
	psAbs := ps + pa
	if math.Abs(psAbs) < dpZeroThreshold {
		return 0
	}

	ptAbs := pt + pa
	ratio := ptAbs / psAbs
	if ratio <= 0 {
		return 0
	}

	maSq := machCoeff * (math.Pow(ratio, gammaRatio) - 1)
	// 防御性处理：使用绝对值
	return math.Sqrt(math.Abs(maSq))
}

// clampMa 将马赫数限制在校准范围内
func clampMa(ma, minMa, maxMa float64) float64 {
	if ma < minMa {
		return minMa
	}
	if ma > maxMa {
		return maxMa
	}
	return ma
}

// ==================== 校准文件解析 ====================

// parseCalibFile 解析单个校准文件
// 格式：
//   第1行：CMa（校准马赫数）
//   第2行：Nalpha（攻角数据条数）
//   第3~Nalpha+2行：Kb  Kt  Sb  Alpha（每行4列）
func parseCalibFile(filePath string) (*types.ThreeHoleCalibData, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open file failed: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	// 读取 CMa
	if !scanner.Scan() {
		return nil, fmt.Errorf("empty file")
	}
	lineNum++
	cMa, err := strconv.ParseFloat(strings.TrimSpace(scanner.Text()), 64)
	if err != nil {
		return nil, fmt.Errorf("line %d: parse CMa failed: %w", lineNum, err)
	}

	// 读取 Nalpha
	if !scanner.Scan() {
		return nil, fmt.Errorf("missing Nalpha line")
	}
	lineNum++
	nAlpha, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
	if err != nil {
		return nil, fmt.Errorf("line %d: parse Nalpha failed: %w", lineNum, err)
	}

	// 读取数据行
	entries := make([]types.ThreeHoleCalibEntry, 0, nAlpha)
	for i := 0; i < nAlpha; i++ {
		if !scanner.Scan() {
			return nil, fmt.Errorf("line %d: missing data row %d", lineNum+1, i+1)
		}
		lineNum++

		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 {
			return nil, fmt.Errorf("line %d: expected 4 fields, got %d", lineNum, len(fields))
		}

		kb, err := strconv.ParseFloat(fields[0], 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: parse Kb failed: %w", lineNum, err)
		}
		kt, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: parse Kt failed: %w", lineNum, err)
		}
		sb, err := strconv.ParseFloat(fields[2], 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: parse Sb failed: %w", lineNum, err)
		}
		alpha, err := strconv.ParseFloat(fields[3], 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: parse Alpha failed: %w", lineNum, err)
		}

		entries = append(entries, types.ThreeHoleCalibEntry{
			Kb:    kb,
			Kt:    kt,
			Sb:    sb,
			Alpha: alpha,
		})
	}

	return &types.ThreeHoleCalibData{
		CMa:    cMa,
		Entries: entries,
	}, nil
}
