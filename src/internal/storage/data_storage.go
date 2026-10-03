package storage

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"yx-daq/internal/types"
)

// deviceRecording 单台设备的录制文件（每台设备独立 CSV，避免多设备数据混入同列）
type deviceRecording struct {
	deviceID       string
	file           *os.File
	writer         *csv.Writer
	path           string
	headerWritten  bool  // 表头是否已写入（该设备首个数据帧到达时写入）
	headerChannels []int // 表头各列对应的通道索引

	// 扩列失败保护：文件被其他程序占用（如 Excel）时 os.Rename 失败，
	// 逐帧重试会阻塞该设备接收循环。记录待扩列签名并限制重试频率，
	// 期间的帧按现有表头写入（新通道值留空），文件可写后自动补齐。
	expandDeferred    bool
	expandPendingSig  string
	lastExpandAttempt time.Time
}

// DataStorageService 数据存储服务。
// 多设备并行采集时每台设备一个独立文件 recording-<会话时间>_<设备名>.csv，
// 行=帧（宽表：Timestamp, DeviceID, CH1, CH2...），通道布局以该设备首帧为准。
// HandlePayload 由各驱动接收协程并发调用，files 与各 deviceRecording 必须加锁串行化
type DataStorageService struct {
	mu           sync.Mutex
	recording    bool
	outputDir    string
	sessionTag   string
	files        map[string]*deviceRecording // DeviceID → 该设备的录制文件
	usedNames    map[string]bool             // 本次会话已占用的文件名（同名设备去重）
	nameResolver func(deviceID string) string
}

// expandRetryInterval 扩列失败后的重试间隔（避免逐帧重试反复阻塞采集链路）
const expandRetryInterval = 3 * time.Second

// NewDataStorageService 创建数据存储服务
func NewDataStorageService(outputDir string) *DataStorageService {
	return &DataStorageService{
		outputDir: outputDir,
		files:     make(map[string]*deviceRecording),
		usedNames: make(map[string]bool),
	}
}

// SetDeviceNameResolver 设置 DeviceID → 设备名解析器（生成可读文件名用）；
// 返回空或未设置时回退用 DeviceID。应在开始录制前调用。
func (s *DataStorageService) SetDeviceNameResolver(fn func(deviceID string) string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nameResolver = fn
}

// StartRecording 开始录制。
// 文件在各设备首帧到达时惰性创建（此时才知道实际参与录制的设备），
// 文件名格式 recording-<会话时间>_<设备名>.csv，多设备各自独立。
func (s *DataStorageService) StartRecording() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.recording {
		return fmt.Errorf("already recording")
	}

	if err := os.MkdirAll(s.outputDir, 0755); err != nil {
		return fmt.Errorf("create recording dir failed: %w", err)
	}

	s.recording = true
	s.sessionTag = time.Now().Format("2006-01-02-15-04-05")
	s.files = make(map[string]*deviceRecording)
	s.usedNames = make(map[string]bool)
	return nil
}

// StopRecording 停止录制：flush 并关闭所有设备文件
func (s *DataStorageService) StopRecording() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.recording {
		return
	}
	s.recording = false
	for _, rec := range s.files {
		if rec.writer != nil {
			rec.writer.Flush()
			if err := rec.writer.Error(); err != nil {
				slog.Warn("flush recording on stop failed", "file", rec.path, "err", err)
			}
		}
		if rec.file != nil {
			if err := rec.file.Close(); err != nil {
				slog.Warn("close recording file failed", "file", rec.path, "err", err)
			}
		}
	}
	s.files = make(map[string]*deviceRecording)
	s.usedNames = make(map[string]bool)
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

// HandlePayload 处理数据帧：写入该设备自己的录制文件，每帧一行、所有通道横排为列
func (s *DataStorageService) HandlePayload(payload types.DataPayload) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.recording {
		return nil
	}

	rec := s.files[payload.DeviceID]
	if rec == nil {
		var err error
		rec, err = s.createDeviceFileLocked(payload.DeviceID)
		if err != nil {
			return err
		}
		s.files[payload.DeviceID] = rec
	}

	if err := s.ensureChannelLayoutLocked(rec, payload); err != nil {
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
	for _, idx := range rec.headerChannels {
		if val, ok := valueByIndex[idx]; ok {
			record = append(record, fmt.Sprintf("%.6f", val))
		} else {
			record = append(record, "")
		}
	}
	if err := rec.writer.Write(record); err != nil {
		return err
	}

	rec.writer.Flush()
	// Flush 的错误不会由 Write 返回：磁盘满/句柄失效等延迟错误在此暴露，
	// 不能静默吞掉（否则录制缺行而 IsRecording 仍为 true）
	if err := rec.writer.Error(); err != nil {
		return fmt.Errorf("write recording row for device %s failed: %w", payload.DeviceID, err)
	}
	return nil
}

// createDeviceFileLocked 为设备创建独立录制文件（其首帧到达时调用，调用方须持锁）。
// 文件名用设备名（解析不到时用 DeviceID），非法字符消毒；同名/同秒重复会话追加序号避免覆盖。
func (s *DataStorageService) createDeviceFileLocked(deviceID string) (*deviceRecording, error) {
	base := fmt.Sprintf("recording-%s_%s", s.sessionTag, s.deviceFileLabelLocked(deviceID))
	name := base + ".csv"
	for i := 2; s.usedNames[name] || fileExists(filepath.Join(s.outputDir, name)); i++ {
		name = fmt.Sprintf("%s-%d.csv", base, i)
	}

	path := filepath.Join(s.outputDir, name)
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create recording file for device %s failed: %w", deviceID, err)
	}
	// 写入 UTF-8 BOM
	if _, err := file.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		file.Close()
		return nil, fmt.Errorf("write BOM to recording file failed: %w", err)
	}

	s.usedNames[name] = true
	return &deviceRecording{
		deviceID: deviceID,
		file:     file,
		writer:   csv.NewWriter(file),
		path:     path,
	}, nil
}

// deviceFileLabelLocked 生成文件名的设备标签：设备名优先，解析不到回退 DeviceID，并消毒非法字符
func (s *DataStorageService) deviceFileLabelLocked(deviceID string) string {
	label := ""
	if s.nameResolver != nil {
		label = s.nameResolver(deviceID)
	}
	if strings.TrimSpace(label) == "" {
		label = deviceID
	}
	return sanitizeFileName(label)
}

// sanitizeFileName 替换 Windows 文件名非法字符与控制字符，去除首尾空白/点；空则回退 device
func sanitizeFileName(name string) string {
	name = strings.Map(func(r rune) rune {
		switch r {
		case '\\', '/', ':', '*', '?', '"', '<', '>', '|':
			return '_'
		}
		if r < 0x20 {
			return '_'
		}
		return r
	}, name)
	name = strings.Trim(name, " .")
	if name == "" {
		return "device"
	}
	return name
}

// fileExists 判断文件是否已存在
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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

// ensureChannelLayoutLocked 保证该设备当前数据帧的所有通道在其文件表头中都有对应列。
// 设备首个数据帧定义初始列；同一设备后续帧引入新通道索引（如运行中启用通道）时扩列
// （重写该文件表头并给历史行补空），避免新通道数据被静默丢弃。
func (s *DataStorageService) ensureChannelLayoutLocked(rec *deviceRecording, payload types.DataPayload) error {
	indices := payloadChannelIndices(payload)
	if !rec.headerWritten {
		return s.writeHeaderLocked(rec, indices, payload)
	}

	seen := make(map[int]bool, len(rec.headerChannels)+len(indices))
	for _, idx := range rec.headerChannels {
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
		return nil
	}

	// 与上次失败同一批新增通道且未到重试时间：本帧按现有表头写入（新通道留空），
	// 避免每帧全量重写阻塞该设备接收循环、持续丢弃整帧
	sig := fmt.Sprint(newIndices)
	if rec.expandDeferred && sig == rec.expandPendingSig && time.Since(rec.lastExpandAttempt) < expandRetryInterval {
		return nil
	}

	rec.lastExpandAttempt = time.Now()
	if err := s.expandColumnsLocked(rec, newIndices, newLabels); err != nil {
		if rec.writer == nil {
			// 文件无法恢复（被占用且重开失败）：摘除该设备文件，
			// 后续帧会新建文件继续录制，不影响其他设备
			delete(s.files, rec.deviceID)
			return err
		}
		rec.expandDeferred = true
		rec.expandPendingSig = sig
		slog.Warn("recording column expansion deferred", "device", rec.deviceID, "new_channels", len(newIndices), "err", err)
		// 不返回错误：本帧仍按现有表头落盘，已有通道数据不丢
		return nil
	}
	rec.expandDeferred = false
	rec.expandPendingSig = ""
	return nil
}

// writeHeaderLocked 根据该设备首个数据帧的通道布局写入宽表表头：Timestamp, DeviceID, CH{n} (unit)...
func (s *DataStorageService) writeHeaderLocked(rec *deviceRecording, indices []int, payload types.DataPayload) error {
	header := []string{"Timestamp", "DeviceID"}
	channels := make([]int, 0, len(indices))
	for pos, idx := range indices {
		header = append(header, channelLabel(idx, pos, payload))
		channels = append(channels, idx)
	}
	if err := rec.writer.Write(header); err != nil {
		return err
	}
	rec.headerChannels = channels
	rec.headerWritten = true
	return nil
}

// expandColumnsLocked 在该设备文件末尾追加新列：重写表头，历史行补空保持列对齐。
// 调用方必须已持有 s.mu。失败时保证 rec.writer 要么可用（已重开原文件），要么为 nil（不可恢复）。
func (s *DataStorageService) expandColumnsLocked(rec *deviceRecording, newIndices []int, newLabels []string) error {
	if rec.file == nil || rec.writer == nil {
		return fmt.Errorf("recording file not open")
	}
	rec.writer.Flush()
	if err := rec.writer.Error(); err != nil {
		return fmt.Errorf("flush recording before column expansion failed: %w", err)
	}

	path := rec.path
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

	if err := rec.file.Close(); err != nil {
		slog.Warn("close recording before column expansion failed", "file", path, "err", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		// 尝试恢复原文件，避免该设备录制彻底中断
		if f, reopenErr := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644); reopenErr == nil {
			rec.file = f
			rec.writer = csv.NewWriter(f)
			return fmt.Errorf("replace recording with expanded header failed: %w", err)
		}
		rec.file = nil
		rec.writer = nil
		return fmt.Errorf("replace recording with expanded header failed: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		rec.file = nil
		rec.writer = nil
		return fmt.Errorf("reopen recording after column expansion failed: %w", err)
	}
	rec.file = f
	rec.writer = csv.NewWriter(f)
	rec.headerChannels = append(rec.headerChannels, newIndices...)
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
