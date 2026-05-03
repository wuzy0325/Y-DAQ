package storage

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"yx-daq/internal/types"
)

// DataStorageService 数据存储服务
type DataStorageService struct {
	recording   bool
	outputDir   string
	currentFile *os.File
	writer      *csv.Writer
}

// NewDataStorageService 创建数据存储服务
func NewDataStorageService(outputDir string) *DataStorageService {
	return &DataStorageService{
		outputDir: outputDir,
	}
}

// StartRecording 开始录制
func (s *DataStorageService) StartRecording() error {
	if s.recording {
		return fmt.Errorf("already recording")
	}

	os.MkdirAll(s.outputDir, 0755) // ignore error: 目录已存在或后续Create会报错
	filename := fmt.Sprintf("recording-%s.csv", time.Now().Format("2006-01-02-15-04-05"))
	filePath := filepath.Join(s.outputDir, filename)

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}

	// 写入 UTF-8 BOM
	file.Write([]byte{0xEF, 0xBB, 0xBF}) // ignore error: BOM写入失败不影响后续CSV写入

	s.currentFile = file
	s.writer = csv.NewWriter(file)
	s.recording = true

	// 写入表头
	header := []string{"Timestamp", "DeviceID", "ChannelIndex", "ChannelName", "Value", "Unit"}
	return s.writer.Write(header)
}

// StopRecording 停止录制
func (s *DataStorageService) StopRecording() {
	if !s.recording {
		return
	}
	s.recording = false
	if s.writer != nil {
		s.writer.Flush()
	}
	if s.currentFile != nil {
		s.currentFile.Close()
		s.currentFile = nil
	}
}

// IsRecording 是否录制中
func (s *DataStorageService) IsRecording() bool {
	return s.recording
}

// SetOutputDir 设置输出目录
func (s *DataStorageService) SetOutputDir(dir string) {
	s.outputDir = dir
}

// HandlePayload 处理数据帧
func (s *DataStorageService) HandlePayload(payload types.DataPayload) error {
	if !s.recording || s.writer == nil {
		return nil
	}

	timestamp := time.UnixMilli(payload.Timestamp).Format("2006-01-02 15:04:05.000")

	for i, val := range payload.Channels {
		chIdx := 0
		if i < len(payload.ChannelIndices) {
			chIdx = payload.ChannelIndices[i]
		}
		record := []string{
			timestamp,
			payload.DeviceID,
			fmt.Sprintf("%d", chIdx),
			fmt.Sprintf("CH%d", chIdx+1),
			fmt.Sprintf("%.6f", val),
			"kPa",
		}
		if err := s.writer.Write(record); err != nil {
			return err
		}
	}

	s.writer.Flush()
	return nil
}

// ExportCalibrationCSV 导出校准数据为CSV
func (s *DataStorageService) ExportCalibrationCSV(dataPoints []types.CalibrationDataPoint, filePath string) error {
	os.MkdirAll(filepath.Dir(filePath), 0755) // ignore error: 目录已存在或后续Create会报错

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// UTF-8 BOM
	file.Write([]byte{0xEF, 0xBB, 0xBF}) // ignore error: BOM写入失败不影响后续CSV写入

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 表头
	header := []string{"α", "β", "P1", "P2", "P3", "P4", "P5", "P∞", "T∞", "Kα", "Kβ", "CPT", "CPS", "采样数", "标准差"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// 数据行
	for _, dp := range dataPoints {
		pTotalStr := ""
		if dp.RawData.PTotal != nil {
			pTotalStr = fmt.Sprintf("%.6f", *dp.RawData.PTotal)
		}
		record := []string{
			fmt.Sprintf("%.4f", dp.Alpha),
			fmt.Sprintf("%.4f", dp.Beta),
			fmt.Sprintf("%.6f", dp.RawData.P1),
			fmt.Sprintf("%.6f", dp.RawData.P2),
			fmt.Sprintf("%.6f", dp.RawData.P3),
			fmt.Sprintf("%.6f", dp.RawData.P4),
			fmt.Sprintf("%.6f", dp.RawData.P5),
			fmt.Sprintf("%.6f", dp.RawData.PAtm),
			fmt.Sprintf("%.6f", dp.RawData.TAtm),
			pTotalStr,
			fmt.Sprintf("%.6f", dp.Coefficients.Kalpha),
			fmt.Sprintf("%.6f", dp.Coefficients.Kbeta),
			fmt.Sprintf("%.6f", dp.Coefficients.CPT),
			fmt.Sprintf("%.6f", dp.Coefficients.CPS),
			fmt.Sprintf("%d", dp.SampleCount),
			fmt.Sprintf("%.6f", dp.StdDev),
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}
