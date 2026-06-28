package interpolation

import (
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// MultiCalInterpolator 多CAL文件插值器
// 支持加载多个不同马赫数的CAL文件，根据实时Ma值自动选择或插值
type MultiCalInterpolator struct {
	calFiles          []calFileWithInterpolator
	sortedMachNumbers []float64
	loaded            bool
	mode              MultiCalInterpolationMode
}

type calFileWithInterpolator struct {
	FileInfo     CalFileInfo
	Interpolator *CalInterpolator
	MachNumber   float64
}

type CalFileData struct {
	FilePath string
	Lines    []string
}

// MultiCalLoadResult 多CAL文件加载结果
type MultiCalLoadResult struct {
	Files       []CalFileInfo // 已加载文件信息
	MachNumbers []float64     // 马赫数列表
	Warnings    []string      // 警告信息
}

// NewMultiCalInterpolator 创建多CAL插值器
func NewMultiCalInterpolator() *MultiCalInterpolator {
	return &MultiCalInterpolator{
		mode: ModeLinear,
	}
}

// LoadCalFiles is kept for source compatibility. File I/O belongs in adapters.
func (m *MultiCalInterpolator) LoadCalFiles(filePaths []string, machNumbers []float64) (*MultiCalLoadResult, error) {
	return nil, fmt.Errorf("load CAL files through an adapter and call LoadCalData")
}

// LoadCalData loads multiple CAL datasets from already-read text lines.
func (m *MultiCalInterpolator) LoadCalData(fileData []CalFileData, machNumbers []float64) (*MultiCalLoadResult, error) {
	m.clearState()

	var files []CalFileInfo
	var loadedMachNumbers []float64
	var warnings []string

	for i, data := range fileData {
		filePath := data.FilePath
		fileName := filepath.Base(filePath)

		interpolator := NewCalInterpolator()
		if err := interpolator.LoadCalLines(data.Lines, filePath); err != nil {
			warnings = append(warnings, fmt.Sprintf("加载CAL文件失败: %s - %v", fileName, err))
			continue
		}

		// 确定马赫数
		var machNumber float64
		if machNumbers != nil && i < len(machNumbers) && isFinite(machNumbers[i]) {
			machNumber = machNumbers[i]
		} else {
			parsed := parseMachFromFileName(filePath)
			if parsed == 0 {
				warnings = append(warnings, fmt.Sprintf("无法从文件名解析马赫数: %s, 已跳过", fileName))
				continue
			}
			machNumber = parsed
		}

		// 检查马赫数重复
		if containsFloat(loadedMachNumbers, machNumber) {
			warnings = append(warnings, fmt.Sprintf("马赫数 %.3f 重复, 已跳过文件: %s", machNumber, fileName))
			continue
		}

		fileInfo := CalFileInfo{
			FilePath:   filePath,
			FileName:   fileName,
			LoadedAt:   0, // 可用 time.Now().UnixMilli()
			ValidRange: interpolator.GetValidRange(),
		}
		fileInfo.ValidRange.MachMin = machNumber
		fileInfo.ValidRange.MachMax = machNumber

		m.calFiles = append(m.calFiles, calFileWithInterpolator{
			FileInfo:     fileInfo,
			Interpolator: interpolator,
			MachNumber:   machNumber,
		})

		files = append(files, fileInfo)
		loadedMachNumbers = append(loadedMachNumbers, machNumber)
	}

	// 按马赫数排序
	sort.Slice(m.calFiles, func(i, j int) bool {
		return m.calFiles[i].MachNumber < m.calFiles[j].MachNumber
	})
	m.sortedMachNumbers = make([]float64, len(m.calFiles))
	for i, f := range m.calFiles {
		m.sortedMachNumbers[i] = f.MachNumber
	}

	if len(m.calFiles) == 0 {
		return nil, fmt.Errorf("没有成功加载任何CAL文件")
	}

	if len(m.calFiles) == 1 {
		warnings = append(warnings, "只加载了一个CAL文件，多马赫数插值功能将退化为单文件模式")
	}

	m.loaded = true

	return &MultiCalLoadResult{
		Files:       files,
		MachNumbers: loadedMachNumbers,
		Warnings:    warnings,
	}, nil
}

// LoadCalFile 兼容单文件加载接口
func (m *MultiCalInterpolator) LoadCalFile(filePath string) error {
	_, err := m.LoadCalFiles([]string{filePath}, nil)
	return err
}

// IsLoaded 检查是否已加载
func (m *MultiCalInterpolator) IsLoaded() bool {
	return m.loaded
}

// GetValidRange 获取有效范围（所有CAL文件范围的并集）
func (m *MultiCalInterpolator) GetValidRange() CalValidRange {
	if len(m.calFiles) == 0 {
		return CalValidRange{
			AlphaMin: -30, AlphaMax: 30,
			BetaMin: -30, BetaMax: 30,
		}
	}

	vr := m.calFiles[0].FileInfo.ValidRange
	alphaMin, alphaMax := vr.AlphaMin, vr.AlphaMax
	betaMin, betaMax := vr.BetaMin, vr.BetaMax
	machMin, machMax := vr.MachMin, vr.MachMax

	for _, f := range m.calFiles[1:] {
		r := f.FileInfo.ValidRange
		alphaMin = math.Min(alphaMin, r.AlphaMin)
		alphaMax = math.Max(alphaMax, r.AlphaMax)
		betaMin = math.Min(betaMin, r.BetaMin)
		betaMax = math.Max(betaMax, r.BetaMax)
		machMin = math.Min(machMin, r.MachMin)
		machMax = math.Max(machMax, r.MachMax)
	}

	return CalValidRange{
		AlphaMin: alphaMin, AlphaMax: alphaMax,
		BetaMin: betaMin, BetaMax: betaMax,
		MachMin: machMin, MachMax: machMax,
	}
}

// Calculate 执行插值计算
func (m *MultiCalInterpolator) Calculate(input InterpolationInput) (InterpolationResult, error) {
	if !m.loaded || len(m.calFiles) == 0 {
		return InterpolationResult{}, fmt.Errorf("CAL文件未加载")
	}

	// 单文件模式
	if len(m.calFiles) == 1 {
		return m.calFiles[0].Interpolator.Calculate(input)
	}

	// 第一步：用中间马赫数的CAL计算初始Ma值
	middleIndex := len(m.calFiles) / 2
	initialResult, err := m.calFiles[middleIndex].Interpolator.Calculate(input)
	if err != nil {
		return InterpolationResult{}, err
	}

	if !initialResult.IsValid || !isFinite(initialResult.MachNumber) {
		return initialResult, nil
	}

	targetMa := initialResult.MachNumber

	// 第二步：根据插值模式计算
	if m.mode == ModeNearest {
		return m.calculateWithNearest(targetMa, input)
	}
	return m.calculateWithLinear(targetMa, input)
}

// calculateWithNearest 最近邻插值
func (m *MultiCalInterpolator) calculateWithNearest(targetMa float64, input InterpolationInput) (InterpolationResult, error) {
	nearestIdx := m.findNearestMachIndex(targetMa)
	result, err := m.calFiles[nearestIdx].Interpolator.Calculate(input)
	if err != nil {
		return InterpolationResult{}, err
	}

	selectedMa := m.calFiles[nearestIdx].MachNumber
	maDiff := math.Abs(targetMa - selectedMa)

	if maDiff > 0.01 {
		warning := fmt.Sprintf("目标 Ma=%.3f 使用 Ma=%.3f 的CAL文件插值 (偏差=%.3f)", targetMa, selectedMa, maDiff)
		if result.Warning != "" {
			result.Warning = result.Warning + "; " + warning
		} else {
			result.Warning = warning
		}
	}

	return result, nil
}

// calculateWithLinear 线性插值
func (m *MultiCalInterpolator) calculateWithLinear(targetMa float64, input InterpolationInput) (InterpolationResult, error) {
	lower, upper := m.findMachInterval(targetMa)

	// 范围外，使用最近邻
	if lower == upper {
		return m.calculateWithNearest(targetMa, input)
	}

	lowerResult, err := m.calFiles[lower].Interpolator.Calculate(input)
	if err != nil {
		return InterpolationResult{}, err
	}
	upperResult, err := m.calFiles[upper].Interpolator.Calculate(input)
	if err != nil {
		return InterpolationResult{}, err
	}

	// 任一结果无效，退化为最近邻
	if !lowerResult.IsValid || !upperResult.IsValid {
		return m.calculateWithNearest(targetMa, input)
	}

	// 线性插值权重
	lowerMa := m.calFiles[lower].MachNumber
	upperMa := m.calFiles[upper].MachNumber
	weight := (targetMa - lowerMa) / (upperMa - lowerMa)

	result := InterpolationResult{
		Alpha:           angleLerp(lowerResult.Alpha, upperResult.Alpha, weight),
		Beta:            angleLerp(lowerResult.Beta, upperResult.Beta, weight),
		MachNumber:      lerp(lowerResult.MachNumber, upperResult.MachNumber, weight),
		V:               lerp(lowerResult.V, upperResult.V, weight),
		Vx:              lerp(lowerResult.Vx, upperResult.Vx, weight),
		Vy:              lerp(lowerResult.Vy, upperResult.Vy, weight),
		Vz:              lerp(lowerResult.Vz, upperResult.Vz, weight),
		Velocity:        lerp(lowerResult.Velocity, upperResult.Velocity, weight),
		CAS:             lerp(lowerResult.CAS, upperResult.CAS, weight),
		SAT:             lerp(lowerResult.SAT, upperResult.SAT, weight),
		DynamicPressure: lerp(lowerResult.DynamicPressure, upperResult.DynamicPressure, weight),
		Density:         lerp(lowerResult.Density, upperResult.Density, weight),
		TotalPressure:   lerp(lowerResult.TotalPressure, upperResult.TotalPressure, weight),
		StaticPressure:  lerp(lowerResult.StaticPressure, upperResult.StaticPressure, weight),
		IsValid:         true,
		Warning: fmt.Sprintf("Ma=%.3f 在 Ma=%.3f 和 Ma=%.3f 之间线性插值 (权重=%.3f)",
			targetMa, lowerMa, upperMa, weight),
	}
	result.Warning = mergeWarnings(result.Warning, lowerResult.Warning, upperResult.Warning)

	return result, nil
}

// SetInterpolationMode 设置插值模式
func (m *MultiCalInterpolator) SetInterpolationMode(mode MultiCalInterpolationMode) {
	m.mode = mode
}

// GetMachRange 获取马赫数范围
func (m *MultiCalInterpolator) GetMachRange() (min, max float64) {
	if len(m.sortedMachNumbers) == 0 {
		return 0, 0
	}
	return m.sortedMachNumbers[0], m.sortedMachNumbers[len(m.sortedMachNumbers)-1]
}

// GetMachNumbers 获取已加载的马赫数列表
func (m *MultiCalInterpolator) GetMachNumbers() []float64 {
	result := make([]float64, len(m.sortedMachNumbers))
	copy(result, m.sortedMachNumbers)
	return result
}

// ==================== 内部辅助函数 ====================

func (m *MultiCalInterpolator) clearState() {
	m.calFiles = nil
	m.sortedMachNumbers = nil
	m.loaded = false
}

func (m *MultiCalInterpolator) findNearestMachIndex(targetMa float64) int {
	nearestIdx := 0
	minDiff := math.Abs(targetMa - m.sortedMachNumbers[0])

	for i := 1; i < len(m.sortedMachNumbers); i++ {
		diff := math.Abs(targetMa - m.sortedMachNumbers[i])
		if diff < minDiff {
			minDiff = diff
			nearestIdx = i
		}
	}
	return nearestIdx
}

func (m *MultiCalInterpolator) findMachInterval(targetMa float64) (lower, upper int) {
	n := len(m.sortedMachNumbers)

	if targetMa <= m.sortedMachNumbers[0] {
		return 0, 0
	}
	if targetMa >= m.sortedMachNumbers[n-1] {
		return n - 1, n - 1
	}

	for i := 0; i < n-1; i++ {
		if targetMa >= m.sortedMachNumbers[i] && targetMa <= m.sortedMachNumbers[i+1] {
			return i, i + 1
		}
	}
	return 0, 0
}

func lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}

// angleLerp 角度线性插值，处理符号不一致时的最短路径插值
// 当两个角度符号不一致时，直接 lerp 可能走长路径（如 -170° 到 170° 应经过 180° 而非 0°）
func angleLerp(a, b, t float64) float64 {
	diff := b - a
	// 取最短角度路径
	if diff > 180 {
		diff -= 360
	} else if diff < -180 {
		diff += 360
	}
	result := a + diff*t
	// 归一化到 [-180, 180)
	for result <= -180 {
		result += 360
	}
	for result > 180 {
		result -= 360
	}
	return result
}

func containsFloat(slice []float64, val float64) bool {
	for _, v := range slice {
		if math.Abs(v-val) < 1e-9 {
			return true
		}
	}
	return false
}

func mergeWarnings(warnings ...string) string {
	seen := make(map[string]bool)
	var parts []string
	for _, warning := range warnings {
		for _, part := range strings.Split(warning, ";") {
			part = strings.TrimSpace(part)
			if part == "" || seen[part] {
				continue
			}
			seen[part] = true
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, "; ")
}

// parseMachFromFileName 从文件名解析马赫数（包级函数，供CalInterpolator和MultiCalInterpolator共用）
func parseMachFromFileName(filePath string) float64 {
	// 提取文件名
	parts := strings.Split(filePath, string(filepath.Separator))
	fileName := parts[len(parts)-1]

	// 匹配格式: "0.5Ma.cal", "Ma0.5.cal" 等
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?)Ma`),
		regexp.MustCompile(`(?i)Ma([0-9]+(?:\.[0-9]+)?)`),
	}

	for _, re := range patterns {
		match := re.FindStringSubmatch(fileName)
		if match != nil {
			ma, err := parseFloat(match[1])
			if err == nil {
				return ma
			}
		}
	}
	return 0
}
