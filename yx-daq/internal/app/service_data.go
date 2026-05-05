package app

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"yx-daq/internal/types"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// DataService 数据录制与导出服务
type DataService struct {
	Core *Core
}

// GetDataDir 获取数据存储目录路径
func (s *DataService) GetDataDir() string {
	return s.Core.GetDataDir()
}

// SetPublishRate 设置数据发布频率
func (s *DataService) SetPublishRate(hz int) {
	if s.Core.AcquisitionHub == nil {
		return
	}
	s.Core.AcquisitionHub.SetPublishHz(hz)
}

// GetPublishRate 获取数据发布频率
func (s *DataService) GetPublishRate() int {
	if s.Core.AcquisitionHub == nil {
		return 0
	}
	return s.Core.AcquisitionHub.GetPublishHz()
}

// StartRecording 开始录制
func (s *DataService) StartRecording() error {
	if s.Core.DataStorage == nil {
		return fmt.Errorf("data storage not initialized")
	}
	s.Core.DataStorage.SetOutputDir(s.Core.GetDataDir())
	return s.Core.DataStorage.StartRecording()
}

// StopRecording 停止录制
func (s *DataService) StopRecording() {
	if s.Core.DataStorage == nil {
		return
	}
	s.Core.DataStorage.StopRecording()
}

// IsRecording 是否正在录制
func (s *DataService) IsRecording() bool {
	if s.Core.DataStorage == nil {
		return false
	}
	return s.Core.DataStorage.IsRecording()
}

// ExportCalibrationPDF 导出校准PDF报告
func (s *DataService) ExportCalibrationPDF() error {
	if s.Core.CalibService == nil {
		return fmt.Errorf("calibration service not initialized")
	}
	status := s.Core.CalibService.GetStatus()
	if len(status.DataPoints) == 0 {
		return fmt.Errorf("no calibration data to export")
	}

	dlg := s.Core.App.Dialog.SaveFile()
	dlg.SetOptions(&application.SaveFileDialogOptions{
		Title:    "导出校准PDF报告",
		Filename: fmt.Sprintf("calibration-report-%s.pdf", time.Now().Format("2006-01-02-15-04-05")),
	})
	dlg.AddFilter("PDF文件", "*.pdf")
	filePath, err := dlg.PromptForSingleSelection()
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

	if s.Core.PdfReport == nil {
		return fmt.Errorf("pdf report service not initialized")
	}
	return s.Core.PdfReport.ExportCalibrationReport(status.DataPoints, config, filePath)
}

// LoadCSVFile 加载CSV文件用于回放
func (s *DataService) LoadCSVFile() (string, error) {
	filePath, err := s.Core.App.Dialog.OpenFile().
		SetTitle("选择CSV数据文件").
		AddFilter("CSV文件", "*.csv").
		AddFilter("所有文件", "*.*").
		PromptForSingleSelection()
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
func (s *DataService) ListRecordingFiles() []string {
	dataDir := s.Core.GetDataDir()
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Printf("mkdir for data dir failed: %v", err)
	}

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
func (s *DataService) ReadRecordingFile(fileName string) (string, error) {
	if filepath.IsAbs(fileName) || !filepath.IsLocal(fileName) {
		return "", fmt.Errorf("invalid file name: %s", fileName)
	}

	dataDir := s.Core.GetDataDir()
	filePath := filepath.Join(dataDir, fileName)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read file failed: %w", err)
	}

	return string(data), nil
}
