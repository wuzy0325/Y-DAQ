package five_hole

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"yx-daq/internal/five_hole/interpolation"
	"yx-daq/internal/types"
)

// ==================== 五孔插值适配器 ====================
//
// 本文件是薄适配层：负责把 .cal 文件读成文本行，喂给 vendored 算法包
// `interpolation.MultiCalInterpolator`，并将其结果类型映射回
// `types.FiveHoleInterpolationResult`。
//
// .cal 文件格式：
//
//	第一行：网格尺寸 "13 13"
//	后续 169 行：ka kb cpt cps alpha beta
//
// 多文件加载时，每个文件对应一个校准马赫数（从文件名解析，如 "Ma0.5.cal"），
// 算法包内部采用中间 Ma 文件算初始 Ma → 相邻 Ma 文件线性插值的策略。

// FiveHoleInterpolator 五孔探针插值器（适配器）
type FiveHoleInterpolator struct {
	multiCal  *interpolation.MultiCalInterpolator
	calibInfo []types.FiveHoleCalibFileInfo
	loaded    bool
}

// NewFiveHoleInterpolator 创建五孔插值器
func NewFiveHoleInterpolator() *FiveHoleInterpolator {
	return &FiveHoleInterpolator{}
}

// LoadCalibFiles 加载多个 .cal 校准文件
// 马赫数由文件名解析（如 "Ma0.5.cal" → 0.5），算法包内部按马赫数排序与插值。
func (i *FiveHoleInterpolator) LoadCalibFiles(filePaths []string) error {
	if len(filePaths) == 0 {
		return fmt.Errorf("未指定校准文件")
	}

	fileData := make([]interpolation.CalFileData, 0, len(filePaths))
	for _, fp := range filePaths {
		lines, err := readCalFileLines(fp)
		if err != nil {
			return fmt.Errorf("读取校准文件 %s 失败: %w", fp, err)
		}
		fileData = append(fileData, interpolation.CalFileData{
			FilePath: fp,
			Lines:    lines,
		})
	}

	multi := interpolation.NewMultiCalInterpolator()
	result, err := multi.LoadCalData(fileData, nil)
	if err != nil {
		return fmt.Errorf("加载校准数据失败: %w", err)
	}

	infos := make([]types.FiveHoleCalibFileInfo, 0, len(result.Files))
	for idx, fi := range result.Files {
		// machNumbers 与 files 顺序对应（在算法包内尚未排序前的顺序）
		ma := 0.0
		if idx < len(result.MachNumbers) {
			ma = result.MachNumbers[idx]
		}
		infos = append(infos, types.FiveHoleCalibFileInfo{
			FilePath: fi.FilePath,
			FileName: fi.FileName,
			CMa:      ma,
			ValidRange: types.FiveHoleCalibRange{
				AlphaMin: fi.ValidRange.AlphaMin,
				AlphaMax: fi.ValidRange.AlphaMax,
				BetaMin:  fi.ValidRange.BetaMin,
				BetaMax:  fi.ValidRange.BetaMax,
				MachMin:  fi.ValidRange.MachMin,
				MachMax:  fi.ValidRange.MachMax,
			},
		})
	}

	i.multiCal = multi
	i.calibInfo = infos
	i.loaded = true
	return nil
}

// IsLoaded 是否已加载校准数据
func (i *FiveHoleInterpolator) IsLoaded() bool {
	return i.loaded
}

// GetCalibInfo 获取校准文件信息
func (i *FiveHoleInterpolator) GetCalibInfo() []types.FiveHoleCalibFileInfo {
	if i.calibInfo == nil {
		return nil
	}
	out := make([]types.FiveHoleCalibFileInfo, len(i.calibInfo))
	copy(out, i.calibInfo)
	return out
}

// Calculate 执行五孔插值计算
// 输入：FiveHoleRawData（P1-P5 + PAtm + TAtm）
// 输出：types.FiveHoleInterpolationResult（对齐 vendored InterpolationResult）
func (i *FiveHoleInterpolator) Calculate(rawData types.FiveHoleRawData) types.FiveHoleInterpolationResult {
	if !i.loaded || i.multiCal == nil {
		return types.FiveHoleInterpolationResult{
			Valid:    false,
			ErrorMsg: "校准文件未载入",
		}
	}

	input := interpolation.InterpolationInput{
		P1:     rawData.P1,
		P2:     rawData.P2,
		P3:     rawData.P3,
		P4:     rawData.P4,
		P5:     rawData.P5,
		PAtm:   rawData.PAtm,
		TAtm:   rawData.TAtm,
		TTotal: rawData.TTotal,
	}

	result, err := i.multiCal.Calculate(input)
	if err != nil {
		return types.FiveHoleInterpolationResult{
			Valid:    false,
			ErrorMsg: fmt.Sprintf("插值计算失败: %v", err),
		}
	}
	return mapInterpolationResult(result)
}

// mapInterpolationResult 将 vendored InterpolationResult 映射为业务类型
// 字段语义对齐：
//   - VelocityProbe ← InterpolationResult.Velocity（TAS，与 V 同源）
//   - PtProbe ← TotalPressure，PsProbe ← StaticPressure
//   - MachProbe ← MachNumber，AlphaProbe ← Alpha，BetaProbe ← Beta
func mapInterpolationResult(r interpolation.InterpolationResult) types.FiveHoleInterpolationResult {
	return types.FiveHoleInterpolationResult{
		PtProbe:         r.TotalPressure,
		PsProbe:         r.StaticPressure,
		MachProbe:       r.MachNumber,
		AlphaProbe:      r.Alpha,
		BetaProbe:       r.Beta,
		VelocityProbe:   r.Velocity,
		CASProbe:        r.CAS,
		SATProbe:        r.SAT,
		DynamicPressure: r.DynamicPressure,
		Density:         r.Density,
		VxProbe:         r.Vx,
		VyProbe:         r.Vy,
		VzProbe:         r.Vz,
		Valid:           r.IsValid,
		ErrorMsg:        r.Warning,
	}
}

// readCalFileLines 读取 .cal 文件全部非空文本行（不去注释、不去表头）
// 算法包内部会处理空行与表头解析，适配器只负责读文本。
func readCalFileLines(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r\n")
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("扫描文件失败: %w", err)
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("文件为空")
	}
	return lines, nil
}
