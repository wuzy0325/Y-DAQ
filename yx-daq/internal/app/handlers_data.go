package app

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"yx-daq/internal/types"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ==================== 数据发布 API ====================

// SetPublishRate 设置数据发布频率
func (a *App) SetPublishRate(hz int) {
	if a.acquisitionHub == nil {
		return
	}
	a.acquisitionHub.SetPublishHz(hz)
}

// GetPublishRate 获取数据发布频率
func (a *App) GetPublishRate() int {
	if a.acquisitionHub == nil {
		return 0
	}
	return a.acquisitionHub.GetPublishHz()
}

// ==================== 录制 API ====================

// StartRecording 开始录制
func (a *App) StartRecording() error {
	if a.dataStorage == nil {
		return fmt.Errorf("data storage not initialized")
	}
	a.dataStorage.SetOutputDir(a.GetDataDir())
	return a.dataStorage.StartRecording()
}

// StopRecording 停止录制
func (a *App) StopRecording() {
	if a.dataStorage == nil {
		return
	}
	a.dataStorage.StopRecording()
}

// IsRecording 是否正在录制
func (a *App) IsRecording() bool {
	if a.dataStorage == nil {
		return false
	}
	return a.dataStorage.IsRecording()
}

// ==================== 报告导出 API ====================

// ExportCalibrationPDF 导出校准PDF报告
func (a *App) ExportCalibrationPDF() error {
	if a.calibService == nil {
		return fmt.Errorf("calibration service not initialized")
	}
	status := a.calibService.GetStatus()
	if len(status.DataPoints) == 0 {
		return fmt.Errorf("no calibration data to export")
	}

	filePath, err := wailsRuntime.SaveFileDialog(a.ctx, wailsRuntime.SaveDialogOptions{
		DefaultFilename: fmt.Sprintf("calibration-report-%s.pdf", time.Now().Format("2006-01-02-15-04-05")),
		Title:           "导出校准PDF报告",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "PDF文件", Pattern: "*.pdf"},
		},
	})
	if err != nil {
		return err
	}
	if filePath == "" {
		return nil
	}

	config := types.CalibrationConfig{
		Type:            types.CalibrationTypeFiveHole,
		DeviceID:        status.TaskID,
		AlphaAxis:       "X",
		BetaAxis:        "Y",
		DwellTimeMs:     500,
		SamplesPerPoint: 10,
	}

	if a.pdfReport == nil {
		return fmt.Errorf("pdf report service not initialized")
	}
	return a.pdfReport.ExportCalibrationReport(status.DataPoints, config, filePath)
}

// ==================== 数据回放 API ====================

// LoadCSVFile 加载CSV文件用于回放
func (a *App) LoadCSVFile() (string, error) {
	filePath, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "选择CSV数据文件",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "CSV文件", Pattern: "*.csv"},
			{DisplayName: "所有文件", Pattern: "*.*"},
		},
	})
	if err != nil {
		return "", err
	}
	if filePath == "" {
		return "", nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read file failed: %w", err)
	}

	return string(data), nil
}

// ListRecordingFiles 列出录制文件
func (a *App) ListRecordingFiles() []string {
	dataDir := a.GetDataDir()
	os.MkdirAll(dataDir, 0755) // ignore error: 目录已存在或后续ReadDir会报错

	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return []string{}
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".csv" {
			files = append(files, entry.Name())
		}
	}
	return files
}

// ReadRecordingFile 按文件名直接读取录制文件内容
func (a *App) ReadRecordingFile(fileName string) (string, error) {
	if filepath.IsAbs(fileName) || !filepath.IsLocal(fileName) {
		return "", fmt.Errorf("invalid file name: %s", fileName)
	}

	dataDir := a.GetDataDir()
	filePath := filepath.Join(dataDir, fileName)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read file failed: %w", err)
	}

	return string(data), nil
}
