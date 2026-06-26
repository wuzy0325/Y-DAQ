package five_hole

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"yx-daq/internal/types"
)

// ==================== 插值器（占位骨架） ====================

// FiveHoleInterpolator 五孔探针插值器
// 算法待用户提供，当前为占位骨架：
// - LoadCalibFiles 解析 .prb 文件（占位格式，参考文档六列 ka kb cpt cps alpha beta）
// - Calculate 返回 NotImplemented 错误
type FiveHoleInterpolator struct {
	calibData []types.FiveHoleCalibData
	loaded    bool
}

// NewFiveHoleInterpolator 创建五孔插值器
func NewFiveHoleInterpolator() *FiveHoleInterpolator {
	return &FiveHoleInterpolator{}
}

// LoadCalibFiles 加载多个 .prb 校准文件
// 占位解析器：按参考文档六列格式 ka kb cpt cps alpha beta 解析
// 实际格式待算法确认后调整
func (i *FiveHoleInterpolator) LoadCalibFiles(filePaths []string) error {
	var allData []types.FiveHoleCalibData

	for _, fp := range filePaths {
		data, err := parsePrbFile(fp)
		if err != nil {
			return fmt.Errorf("parse prb file %s failed: %w", fp, err)
		}
		data.FilePath = fp
		data.FileName = fp
		if idx := strings.LastIndexAny(fp, "/\\"); idx >= 0 {
			data.FileName = fp[idx+1:]
		}
		allData = append(allData, *data)
	}

	if len(allData) == 0 {
		return fmt.Errorf("no calibration data loaded")
	}

	i.calibData = allData
	i.loaded = true
	return nil
}

// IsLoaded 是否已加载校准数据
func (i *FiveHoleInterpolator) IsLoaded() bool {
	return i.loaded
}

// GetCalibInfo 获取校准文件信息
func (i *FiveHoleInterpolator) GetCalibInfo() []types.FiveHoleCalibFileInfo {
	infos := make([]types.FiveHoleCalibFileInfo, len(i.calibData))
	for idx, d := range i.calibData {
		infos[idx] = types.FiveHoleCalibFileInfo{
			FilePath: d.FilePath,
			FileName: d.FileName,
			CMa:      d.CMa,
		}
	}
	return infos
}

// Calculate 执行五孔插值计算
// 输入：P1-P5 + PAtm + TAtm
// 输出：α、β、Ma、V、Pt、Ps
// 【占位】算法待用户提供
func (i *FiveHoleInterpolator) Calculate(rawData types.FiveHoleRawData) types.FiveHoleInterpolationResult {
	if !i.loaded {
		return types.FiveHoleInterpolationResult{
			Valid:    false,
			ErrorMsg: "校准文件未载入",
		}
	}
	// 占位：算法未实现
	_ = rawData
	return types.FiveHoleInterpolationResult{
		Valid:    false,
		ErrorMsg: "五孔插值算法未实现（待提供）",
	}
}

// parsePrbFile 解析 .prb 校准文件（占位格式）
// 参考文档：13×13 网格，每行六列 ka kb cpt cps alpha beta
// 首行 CMa（校准马赫数）
// 后续行数据条目
// 实际格式待算法确认后调整
func parsePrbFile(filePath string) (*types.FiveHoleCalibData, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open file failed: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNo := 0
	data := &types.FiveHoleCalibData{}

	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if lineNo == 1 {
			// 首行：CMa
			cMa, err := strconv.ParseFloat(fields[0], 64)
			if err != nil {
				return nil, fmt.Errorf("line %d: parse CMa failed: %w", lineNo, err)
			}
			data.CMa = cMa
			continue
		}

		// 数据行：ka kb cpt cps alpha beta
		if len(fields) < 6 {
			return nil, fmt.Errorf("line %d: expected 6 fields, got %d", lineNo, len(fields))
		}
		ka, err1 := strconv.ParseFloat(fields[0], 64)
		kb, err2 := strconv.ParseFloat(fields[1], 64)
		cpt, err3 := strconv.ParseFloat(fields[2], 64)
		cps, err4 := strconv.ParseFloat(fields[3], 64)
		alpha, err5 := strconv.ParseFloat(fields[4], 64)
		beta, err6 := strconv.ParseFloat(fields[5], 64)
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil || err5 != nil || err6 != nil {
			return nil, fmt.Errorf("line %d: parse fields failed", lineNo)
		}

		data.Entries = append(data.Entries, types.FiveHoleCalibEntry{
			Ka:    ka,
			Kb:    kb,
			Cpt:   cpt,
			Cps:   cps,
			Alpha: alpha,
			Beta:  beta,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan file failed: %w", err)
	}

	if len(data.Entries) == 0 {
		return nil, fmt.Errorf("no calibration entries in file")
	}

	return data, nil
}
