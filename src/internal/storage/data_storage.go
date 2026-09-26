package storage

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"yx-daq/internal/types"
)

// DataStorageService 数据存储服务
// 多设备并行采集时 HandlePayload 由各驱动接收协程并发调用，内部状态必须加锁串行化
type DataStorageService struct {
	mu             sync.Mutex
	recording      bool
	outputDir      string
	currentFile    *os.File
	writer         *csv.Writer
	headerWritten  bool  // 表头是否已写入（首个数据帧到达时写入）
	headerChannels []int // 表头各列对应的通道索引

	// 扩列失败保护：文件被其他程序占用（如 Excel）时 os.Rename 失败，
	// 逐帧重试会让所有设备接收循环反复阻塞。记录待扩列签名并限制重试频率，
	// 期间的帧按现有表头写入（新通道值留空），文件可写后自动补齐。
	expandDeferred    bool
	expandPendingSig  string
	lastExpandAttempt time.Time
}

// expandRetryInterval 扩列失败后的重试间隔（避免逐帧重试反复阻塞采集链路）
const expandRetryInterval = 3 * time.Second

// NewDataStorageService 创建数据存储服务
func NewDataStorageService(outputDir string) *DataStorageService {
	return &DataStorageService{
		outputDir: outputDir,
	}
}

// StartRecording 开始录制
func (s *DataStorageService) StartRecording() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.recording {
		return fmt.Errorf("already recording")
	}

	if err := os.MkdirAll(s.outputDir, 0755); err != nil {
		slog.Error("mkdir for recording failed", "err", err)
	}
	filename := fmt.Sprintf("recording-%s.csv", time.Now().Format("2006-01-02-15-04-05"))
	filePath := filepath.Join(s.outputDir, filename)

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}

	// 写入 UTF-8 BOM
	if _, err := file.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		file.Close()
		return fmt.Errorf("write BOM to recording file failed: %w", err)
	}

	s.currentFile = file
	s.writer = csv.NewWriter(file)
	s.recording = true
	// 表头不在此处写入：通道布局以首个数据帧为准（不同设备通道数/启用通道不同），
	// 由 HandlePayload 收到首帧时懒写入，保证"所有通道在一横行"的列定义与数据对齐。
	s.headerWritten = false
	s.headerChannels = nil
	s.expandDeferred = false
	s.expandPendingSig = ""
	s.lastExpandAttempt = time.Time{}
	return nil
}

// StopRecording 停止录制
func (s *DataStorageService) StopRecording() {
	s.mu.Lock()
	defer s.mu.Unlock()

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
	s.writer = nil
	s.headerWritten = false
	s.headerChannels = nil
}

// IsRecording 是否录制中
func (s *DataStorageService) IsRecording() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.recording
}

// SetOutputDir 设置输出目录
func (s *DataStorageService) SetOutputDir(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.outputDir = dir
}

// HandlePayload 处理数据帧：每帧一行，所有通道横排为列（宽表格式）
func (s *DataStorageService) HandlePayload(payload types.DataPayload) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.recording || s.writer == nil {
		return nil
	}

	if err := s.ensureChannelLayoutLocked(payload); err != nil {
		return err
	}

	// Excel 打开 CSV 时会把带毫秒的时间自动解析成日期值，但默认显示格式会截成
	// "24:28.1" 这类残缺形式（日期不可见）。CSV 无法携带单元格格式，用 ="..."
	// 公式包裹可让 Excel/WPS 按文本原样显示完整时间（单引号前缀仅在手工输入
	// 单元格时有效，写在 CSV 里会被原样显示，不可用）。
	// 注意：encoding/csv 会把该字段转义为 "=""..."（前端回放解析需兼容，
	// 见 usePlayback.ts unwrapTimestamp）。
	timestamp := fmt.Sprintf(`="%s"`, time.UnixMilli(payload.Timestamp).Format("2006-01-02 15:04:05.000"))

	// 按通道索引映射值，随后对齐到表头各列（缺帧通道留空）
	valueByIndex := make(map[int]float64, len(payload.Channels))
	for i, idx := range payloadChannelIndices(payload) {
		valueByIndex[idx] = payload.Channels[i]
	}

	record := []string{timestamp, payload.DeviceID}
	for _, idx := range s.headerChannels {
		if val, ok := valueByIndex[idx]; ok {
			record = append(record, fmt.Sprintf("%.6f", val))
		} else {
			record = append(record, "")
		}
	}
	if err := s.writer.Write(record); err != nil {
		return err
	}

	s.writer.Flush()
	// Flush 的错误不会由 Write 返回：磁盘满/句柄失效等延迟错误在此暴露，
	// 不能静默吞掉（否则录制缺行而 IsRecording 仍为 true）
	if err := s.writer.Error(); err != nil {
		return fmt.Errorf("write recording row failed: %w", err)
	}
	return nil
}

// payloadChannelIndices 返回帧内每个通道对应的物理通道索引（索引缺失时回退到位置索引）
func payloadChannelIndices(payload types.DataPayload) []int {
	indices := make([]int, len(payload.Channels))
	for i := range payload.Channels {
		indices[i] = i
		if i < len(payload.ChannelIndices) {
			indices[i] = payload.ChannelIndices[i]
		}
	}
	return indices
}

// channelLabel 生成宽表列名：CH{n} 或 CH{n} (unit)（unit 为空时省略）
func channelLabel(idx, pos int, payload types.DataPayload) string {
	if pos < len(payload.ChannelUnits) && payload.ChannelUnits[pos] != "" {
		return fmt.Sprintf("CH%d (%s)", idx+1, payload.ChannelUnits[pos])
	}
	return fmt.Sprintf("CH%d", idx+1)
}

// ensureChannelLayoutLocked 保证当前数据帧的所有通道在表头中都有对应列。
// 首个数据帧定义初始列；多设备并行录制时后到设备可能引入新通道索引，
// 此时扩列（重写文件头并给历史行补空），避免新通道数据被静默丢弃。
func (s *DataStorageService) ensureChannelLayoutLocked(payload types.DataPayload) error {
	indices := payloadChannelIndices(payload)
	if !s.headerWritten {
		return s.writeHeaderLocked(indices, payload)
	}

	seen := make(map[int]bool, len(s.headerChannels)+len(indices))
	for _, idx := range s.headerChannels {
		seen[idx] = true
	}
	newIndices := []int{}
	newLabels := []string{}
	for pos, idx := range indices {
		if seen[idx] {
			continue
		}
		seen[idx] = true
		newIndices = append(newIndices, idx)
		newLabels = append(newLabels, channelLabel(idx, pos, payload))
	}
	if len(newIndices) == 0 {
		// 不清空 expandDeferred：多设备交替来帧时，无新增通道的帧不应让
		// 另一台设备待扩列的状态失效，否则每帧都会重试全量重写
		return nil
	}

	// 与上次失败同一批新增通道且未到重试时间：本帧按现有表头写入（新通道留空），
	// 避免每帧全量重写阻塞所有设备接收循环、持续丢弃该设备整帧
	sig := fmt.Sprint(newIndices)
	if s.expandDeferred && sig == s.expandPendingSig && time.Since(s.lastExpandAttempt) < expandRetryInterval {
		return nil
	}

	s.lastExpandAttempt = time.Now()
	if err := s.expandColumnsLocked(newIndices, newLabels); err != nil {
		s.expandDeferred = true
		s.expandPendingSig = sig
		slog.Warn("recording column expansion deferred", "new_channels", len(newIndices), "err", err)
		// 不返回错误：本帧仍按现有表头落盘，已有通道数据不丢
		return nil
	}
	s.expandDeferred = false
	s.expandPendingSig = ""
	return nil
}

// writeHeaderLocked 根据首个数据帧的通道布局写入宽表表头：Timestamp, DeviceID, CH{n} (unit)...
func (s *DataStorageService) writeHeaderLocked(indices []int, payload types.DataPayload) error {
	header := []string{"Timestamp", "DeviceID"}
	channels := make([]int, 0, len(indices))
	for pos, idx := range indices {
		header = append(header, channelLabel(idx, pos, payload))
		channels = append(channels, idx)
	}
	if err := s.writer.Write(header); err != nil {
		return err
	}
	s.headerChannels = channels
	s.headerWritten = true
	return nil
}

// expandColumnsLocked 在文件末尾追加新列：重写表头，历史行补空保持列对齐。
// 调用方必须已持有 s.mu。
func (s *DataStorageService) expandColumnsLocked(newIndices []int, newLabels []string) error {
	if s.currentFile == nil || s.writer == nil {
		return fmt.Errorf("recording file not open")
	}
	s.writer.Flush()
	if err := s.writer.Error(); err != nil {
		return fmt.Errorf("flush recording before column expansion failed: %w", err)
	}

	path := s.currentFile.Name()
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read recording for column expansion failed: %w", err)
	}
	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("parse recording for column expansion failed: %w", err)
	}
	if len(records) == 0 {
		return fmt.Errorf("recording file missing header")
	}

	header := append(append([]string{}, records[0]...), newLabels...)
	tmpPath := path + ".expand.tmp"
	if err := writeCSVFile(tmpPath, header, records[1:]); err != nil {
		os.Remove(tmpPath)
		return err
	}

	if err := s.currentFile.Close(); err != nil {
		slog.Warn("close recording before column expansion failed", "err", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		// 尝试恢复原文件，避免录制彻底中断
		if f, reopenErr := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644); reopenErr == nil {
			s.currentFile = f
			s.writer = csv.NewWriter(f)
			return fmt.Errorf("replace recording with expanded header failed: %w", err)
		}
		s.recording = false
		s.currentFile = nil
		s.writer = nil
		return fmt.Errorf("replace recording with expanded header failed: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		s.recording = false
		s.currentFile = nil
		s.writer = nil
		return fmt.Errorf("reopen recording after column expansion failed: %w", err)
	}
	s.currentFile = f
	s.writer = csv.NewWriter(f)
	s.headerChannels = append(s.headerChannels, newIndices...)
	return nil
}

// writeCSVFile 写含 UTF-8 BOM 的 CSV 文件，历史行不足表头列数时补空
func writeCSVFile(path string, header []string, rows [][]string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create csv file failed: %w", err)
	}
	defer f.Close()

	if _, err := f.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return fmt.Errorf("write BOM failed: %w", err)
	}
	w := csv.NewWriter(f)
	if err := w.Write(header); err != nil {
		return fmt.Errorf("write csv header failed: %w", err)
	}
	for _, row := range rows {
		padded := make([]string, len(header))
		copy(padded, row)
		if err := w.Write(padded); err != nil {
			return fmt.Errorf("write csv row failed: %w", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return fmt.Errorf("flush csv file failed: %w", err)
	}
	return nil
}

// ExportCalibrationCSV 导出校准数据为CSV
func (s *DataStorageService) ExportCalibrationCSV(dataPoints []types.CalibrationDataPoint, filePath string) error {
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		slog.Error("mkdir for csv export failed", "err", err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// UTF-8 BOM
	if _, err := file.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return fmt.Errorf("write BOM to csv export failed: %w", err)
	}

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
