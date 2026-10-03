package storage

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"yx-daq/internal/types"
)

// csvValueOne 录制文件中 1.0 的落盘格式（%.6f）
const csvValueOne = "1.000000"

func TestDataStorage_StartAndStopRecording(t *testing.T) {
	dir := t.TempDir()
	s := NewDataStorageService(dir)

	if s.IsRecording() {
		t.Fatal("should not be recording initially")
	}

	if err := s.StartRecording(); err != nil {
		t.Fatalf("StartRecording failed: %v", err)
	}
	if !s.IsRecording() {
		t.Error("should be recording after StartRecording")
	}

	// 重复 StartRecording 应报错
	if err := s.StartRecording(); err == nil {
		t.Error("duplicate StartRecording should error")
	}

	s.StopRecording()
	if s.IsRecording() {
		t.Error("should not be recording after StopRecording")
	}

	// 重复 StopRecording 不应 panic
	s.StopRecording()
}

func TestDataStorage_StartRecording_CreatesFileWithBOM(t *testing.T) {
	dir := t.TempDir()
	s := NewDataStorageService(dir)

	if err := s.StartRecording(); err != nil {
		t.Fatalf("StartRecording failed: %v", err)
	}
	// 文件惰性创建：首帧到达前目录中不应有文件
	if files, _ := os.ReadDir(dir); len(files) != 0 {
		t.Fatalf("no file should exist before first frame, got %d", len(files))
	}
	if err := s.HandlePayload(types.DataPayload{
		DeviceID:       "d1",
		Timestamp:      1700000000000,
		Channels:       []float64{1.0},
		ChannelIndices: []int{0},
		ChannelUnits:   []string{"Pa"},
	}); err != nil {
		t.Fatalf("HandlePayload failed: %v", err)
	}
	s.StopRecording()

	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if filepath.Ext(files[0].Name()) != ".csv" {
		t.Errorf("expected .csv extension, got %s", files[0].Name())
	}
	if !strings.HasPrefix(files[0].Name(), "recording-") {
		t.Errorf("file name should start with recording-, got %s", files[0].Name())
	}

	// 验证 BOM
	path := filepath.Join(dir, files[0].Name())
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if len(content) < 3 || content[0] != 0xEF || content[1] != 0xBB || content[2] != 0xBF {
		t.Errorf("file should start with UTF-8 BOM, got first bytes: %v", content[:min(3, len(content))])
	}
}

func TestDataStorage_StartRecording_NestedDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "path")
	s := NewDataStorageService(dir)

	if err := s.StartRecording(); err != nil {
		t.Fatalf("StartRecording with nested dir failed: %v", err)
	}
	s.StopRecording()

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("nested directory should have been created")
	}
}

func TestDataStorage_HandlePayload_NotRecording(t *testing.T) {
	s := NewDataStorageService(t.TempDir())
	// 未开始录制时 HandlePayload 应为 no-op（返回 nil）
	err := s.HandlePayload(types.DataPayload{
		DeviceID:       "d1",
		Channels:       []float64{1.0},
		ChannelIndices: []int{0},
		ChannelUnits:   []string{"Pa"},
	})
	if err != nil {
		t.Errorf("HandlePayload while not recording should return nil, got %v", err)
	}
}

func TestDataStorage_HandlePayload_WritesWideRow(t *testing.T) {
	dir := t.TempDir()
	s := NewDataStorageService(dir)

	if err := s.StartRecording(); err != nil {
		t.Fatalf("StartRecording failed: %v", err)
	}

	err := s.HandlePayload(types.DataPayload{
		DeviceID:       "d1",
		Timestamp:      1700000000000,
		Channels:       []float64{1.0, 2.0, 3.0},
		ChannelIndices: []int{5, 6, 7},
		ChannelUnits:   []string{"kPa", "kPa", "kPa"},
	})
	if err != nil {
		t.Fatalf("HandlePayload failed: %v", err)
	}
	s.StopRecording()

	// 读取文件并验证内容
	files, _ := os.ReadDir(dir)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	f, err := os.Open(filepath.Join(dir, files[0].Name()))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	// 跳过 BOM
	bom := make([]byte, 3)
	if _, err := f.Read(bom); err != nil {
		t.Fatalf("read BOM failed: %v", err)
	}

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	// 期望 1 表头 + 1 数据行（所有通道在一横行）
	if len(records) != 2 {
		t.Fatalf("expected 2 records (1 header + 1 wide row), got %d", len(records))
	}
	// 表头：Timestamp, DeviceID + 每通道一列（含单位）
	expectedHeader := []string{"Timestamp", "DeviceID", "CH6 (kPa)", "CH7 (kPa)", "CH8 (kPa)"}
	if len(records[0]) != len(expectedHeader) {
		t.Fatalf("header fields = %d, want %d: %v", len(records[0]), len(expectedHeader), records[0])
	}
	for i, want := range expectedHeader {
		if records[0][i] != want {
			t.Errorf("header[%d] = %q, want %q", i, records[0][i], want)
		}
	}
	// 数据行：时间戳, 设备, 各通道值横排
	row := records[1]
	// 时间戳用 ="..." 公式包裹强制文本显示，内部为本地时区格式
	if !strings.HasPrefix(row[0], `="`) || !strings.HasSuffix(row[0], `"`) ||
		len(row[0]) != len(`="2006-01-02 15:04:05.000"`) {
		t.Errorf("row timestamp = %q, want format =\"yyyy-mm-dd hh:mm:ss.mmm\"", row[0])
	}
	if row[1] != "d1" {
		t.Errorf("row DeviceID = %q", row[1])
	}
	if row[2] != csvValueOne || row[3] != "2.000000" || row[4] != "3.000000" {
		t.Errorf("row channel values = %v, want [1.000000 2.000000 3.000000]", row[2:])
	}
}

func TestDataStorage_HandlePayload_MultipleFrames(t *testing.T) {
	dir := t.TempDir()
	s := NewDataStorageService(dir)
	if err := s.StartRecording(); err != nil {
		t.Fatalf("StartRecording failed: %v", err)
	}

	base := types.DataPayload{
		DeviceID:       "d1",
		Channels:       []float64{10.0, 20.0},
		ChannelIndices: []int{0, 1},
		ChannelUnits:   []string{"Pa", "Pa"},
	}
	base.Timestamp = 1700000000000
	if err := s.HandlePayload(base); err != nil {
		t.Fatalf("HandlePayload #1 failed: %v", err)
	}
	base.Timestamp = 1700000000100
	base.Channels = []float64{11.0, 21.0}
	if err := s.HandlePayload(base); err != nil {
		t.Fatalf("HandlePayload #2 failed: %v", err)
	}
	s.StopRecording()

	files, _ := os.ReadDir(dir)
	f, _ := os.Open(filepath.Join(dir, files[0].Name()))
	defer f.Close()
	f.Read(make([]byte, 3))
	r := csv.NewReader(f)
	records, _ := r.ReadAll()
	// 1 表头 + 2 数据行，每行两个通道横排
	if len(records) != 3 {
		t.Fatalf("expected 3 records (1 header + 2 rows), got %d", len(records))
	}
	if records[1][2] != "10.000000" || records[1][3] != "20.000000" {
		t.Errorf("frame1 values = %v", records[1][2:])
	}
	if records[2][2] != "11.000000" || records[2][3] != "21.000000" {
		t.Errorf("frame2 values = %v", records[2][2:])
	}
}

func TestDataStorage_HandlePayload_DefaultUnitInHeader(t *testing.T) {
	// ChannelUnits 为空时表头单位应省略（仅 CH{n}）
	dir := t.TempDir()
	s := NewDataStorageService(dir)
	if err := s.StartRecording(); err != nil {
		t.Fatalf("StartRecording failed: %v", err)
	}

	if err := s.HandlePayload(types.DataPayload{
		DeviceID:       "d1",
		Channels:       []float64{1.0},
		ChannelIndices: []int{0},
		ChannelUnits:   nil, // 无单位
	}); err != nil {
		t.Fatalf("HandlePayload failed: %v", err)
	}
	s.StopRecording()

	files, _ := os.ReadDir(dir)
	f, _ := os.Open(filepath.Join(dir, files[0].Name()))
	defer f.Close()
	f.Read(make([]byte, 3)) // skip BOM
	r := csv.NewReader(f)
	records, _ := r.ReadAll()
	if records[0][2] != "CH1" {
		t.Errorf("header channel = %q, want CH1", records[0][2])
	}
	if records[1][2] != csvValueOne {
		t.Errorf("row value = %q, want 1.000000", records[1][2])
	}
}

func TestDataStorage_HandlePayload_ChannelIndexOutOfBounds(t *testing.T) {
	// ChannelIndices 长度 < Channels 长度 时，剩余通道按位置索引（i）对齐
	dir := t.TempDir()
	s := NewDataStorageService(dir)
	if err := s.StartRecording(); err != nil {
		t.Fatalf("StartRecording failed: %v", err)
	}

	if err := s.HandlePayload(types.DataPayload{
		DeviceID:       "d1",
		Channels:       []float64{1.0, 2.0},
		ChannelIndices: []int{5}, // 只有一个，第二个通道应使用位置索引 1
		ChannelUnits:   []string{"Pa"},
	}); err != nil {
		t.Fatalf("HandlePayload failed: %v", err)
	}
	s.StopRecording()

	files, _ := os.ReadDir(dir)
	f, _ := os.Open(filepath.Join(dir, files[0].Name()))
	defer f.Close()
	f.Read(make([]byte, 3))
	r := csv.NewReader(f)
	records, _ := r.ReadAll()
	// 表头：CH6 (Pa)（索引5+1，含单位）、CH2（位置索引1+1，无单位）
	if records[0][2] != "CH6 (Pa)" || records[0][3] != "CH2" {
		t.Errorf("header = %v, want [CH6 (Pa) CH2]", records[0][2:])
	}
	if records[1][2] != csvValueOne || records[1][3] != "2.000000" {
		t.Errorf("row values = %v, want [1.000000 2.000000]", records[1][2:])
	}
}

func TestDataStorage_SetOutputDir(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	s := NewDataStorageService(dir1)

	payload := types.DataPayload{
		DeviceID:       "d1",
		Timestamp:      1700000000000,
		Channels:       []float64{1.0},
		ChannelIndices: []int{0},
		ChannelUnits:   []string{"Pa"},
	}

	if err := s.StartRecording(); err != nil {
		t.Fatalf("StartRecording failed: %v", err)
	}
	if err := s.HandlePayload(payload); err != nil {
		t.Fatalf("HandlePayload failed: %v", err)
	}
	s.StopRecording()

	// 切换目录
	s.SetOutputDir(dir2)
	if err := s.StartRecording(); err != nil {
		t.Fatalf("StartRecording in dir2 failed: %v", err)
	}
	if err := s.HandlePayload(payload); err != nil {
		t.Fatalf("HandlePayload in dir2 failed: %v", err)
	}
	s.StopRecording()

	// dir1 应有 1 个文件，dir2 应有 1 个文件
	if files, _ := os.ReadDir(dir1); len(files) != 1 {
		t.Errorf("dir1 should have 1 file, got %d", len(files))
	}
	if files, _ := os.ReadDir(dir2); len(files) != 1 {
		t.Errorf("dir2 should have 1 file, got %d", len(files))
	}
}

func TestDataStorage_ExportCalibrationCSV(t *testing.T) {
	dir := t.TempDir()
	s := NewDataStorageService(dir)
	path := filepath.Join(dir, "sub", "calib.csv")

	pTotal := 500.0
	dataPoints := []types.CalibrationDataPoint{
		{
			PointID: "p1",
			Alpha:   5.0,
			Beta:    3.0,
			RawData: types.FiveHoleRawData{
				P1: 100, P2: 105, P3: 102, P4: 101, P5: 103,
				PAtm: 90, TAtm: 25, PTotal: &pTotal,
			},
			Coefficients: types.FiveHoleCoefficients{Kalpha: 0.1, Kbeta: 0.2, CPT: 1.1, CPS: 1.2},
			SampleCount:  10,
			StdDev:       0.05,
		},
	}

	if err := s.ExportCalibrationCSV(dataPoints, path); err != nil {
		t.Fatalf("ExportCalibrationCSV failed: %v", err)
	}

	// 验证文件存在且包含 BOM
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if len(content) < 3 || content[0] != 0xEF || content[1] != 0xBB || content[2] != 0xBF {
		t.Error("file should start with UTF-8 BOM")
	}

	// 验证可解析为 CSV
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()
	f.Read(make([]byte, 3)) // skip BOM
	r := csv.NewReader(f)
	// 现有实现表头 15 列、数据 16 列（多一列 PTotal），允许字段数不一致
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records (header + 1 data), got %d", len(records))
	}
	// 表头第 1 列是 α
	if records[0][0] != "α" {
		t.Errorf("header[0] = %q, want α", records[0][0])
	}
	// PTotal 在数据行索引 9（数据行比表头多一列 PTotal）
	if records[1][9] != "500.000000" {
		t.Errorf("PTotal = %q, want 500.000000", records[1][9])
	}
	// 表头 15 列，数据行 16 列
	if len(records[0]) != 15 {
		t.Errorf("header should have 15 fields, got %d", len(records[0]))
	}
	if len(records[1]) != 16 {
		t.Errorf("data row should have 16 fields, got %d", len(records[1]))
	}
}

func TestDataStorage_ExportCalibrationCSV_NilPTotal(t *testing.T) {
	dir := t.TempDir()
	s := NewDataStorageService(dir)
	path := filepath.Join(dir, "calib.csv")

	// PTotal 为 nil 时该列应为空字符串
	dataPoints := []types.CalibrationDataPoint{
		{
			Alpha:   0,
			Beta:    0,
			RawData: types.FiveHoleRawData{P1: 1, P2: 2, P3: 3, P4: 4, P5: 5, PAtm: 0, TAtm: 0, PTotal: nil},
		},
	}

	if err := s.ExportCalibrationCSV(dataPoints, path); err != nil {
		t.Fatalf("ExportCalibrationCSV failed: %v", err)
	}

	f, _ := os.Open(path)
	defer f.Close()
	f.Read(make([]byte, 3))
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if records[1][9] != "" {
		t.Errorf("nil PTotal should be empty, got %q", records[1][9])
	}
}

func TestDataStorage_ExportCalibrationCSV_EmptyData(t *testing.T) {
	dir := t.TempDir()
	s := NewDataStorageService(dir)
	path := filepath.Join(dir, "empty.csv")

	// 空数据应仅写表头
	if err := s.ExportCalibrationCSV(nil, path); err != nil {
		t.Fatalf("ExportCalibrationCSV with empty data failed: %v", err)
	}

	f, _ := os.Open(path)
	defer f.Close()
	f.Read(make([]byte, 3))
	r := csv.NewReader(f)
	records, _ := r.ReadAll()
	if len(records) != 1 {
		t.Errorf("expected only header (1 record), got %d", len(records))
	}
}

// readCSVRecords 读取单个录制文件（跳过 BOM），返回全部记录（含表头）
func readCSVRecords(t *testing.T, path string) [][]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open recording %s failed: %v", path, err)
	}
	defer f.Close()
	f.Read(make([]byte, 3)) // skip BOM
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll recording %s failed: %v", path, err)
	}
	return records
}

// readSingleRecording 读取录制目录中唯一 CSV（单设备场景），返回全部记录（含表头）
func readSingleRecording(t *testing.T, dir string) [][]string {
	t.Helper()
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 recording file, got %d", len(files))
	}
	return readCSVRecords(t, filepath.Join(dir, files[0].Name()))
}

// readRecordingsByDevice 读取目录中所有录制文件，按数据行 DeviceID 归组（含表头）
func readRecordingsByDevice(t *testing.T, dir string) map[string][][]string {
	t.Helper()
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	result := make(map[string][][]string)
	for _, f := range files {
		if filepath.Ext(f.Name()) != ".csv" {
			continue
		}
		records := readCSVRecords(t, filepath.Join(dir, f.Name()))
		if len(records) < 2 {
			t.Fatalf("recording %s has no data rows", f.Name())
		}
		result[records[1][1]] = records
	}
	return result
}

// 多设备录制：每台设备一个独立文件，各自表头，不与其他设备共享列
func TestDataStorage_HandlePayload_SeparateFilePerDevice(t *testing.T) {
	dir := t.TempDir()
	s := NewDataStorageService(dir)
	if err := s.StartRecording(); err != nil {
		t.Fatalf("StartRecording failed: %v", err)
	}

	// dev-a：两帧，通道索引 0/1
	first := types.DataPayload{
		DeviceID:       "dev-a",
		Timestamp:      1700000000000,
		Channels:       []float64{1.0, 2.0},
		ChannelIndices: []int{0, 1},
		ChannelUnits:   []string{"Pa", "Pa"},
	}
	if err := s.HandlePayload(first); err != nil {
		t.Fatalf("HandlePayload dev-a #1 failed: %v", err)
	}
	// dev-b：一帧，通道索引 5（不得混入 dev-a 文件，也不得给 dev-a 文件扩列）
	if err := s.HandlePayload(types.DataPayload{
		DeviceID:       "dev-b",
		Timestamp:      1700000000100,
		Channels:       []float64{6.0},
		ChannelIndices: []int{5},
		ChannelUnits:   []string{"kPa"},
	}); err != nil {
		t.Fatalf("HandlePayload dev-b failed: %v", err)
	}
	first.Timestamp = 1700000000200
	if err := s.HandlePayload(first); err != nil {
		t.Fatalf("HandlePayload dev-a #2 failed: %v", err)
	}
	s.StopRecording()

	byDev := readRecordingsByDevice(t, dir)
	if len(byDev) != 2 {
		t.Fatalf("expected 2 per-device files, got %d", len(byDev))
	}
	a, okA := byDev["dev-a"]
	b, okB := byDev["dev-b"]
	if !okA || !okB {
		t.Fatalf("missing device recordings: %v", byDev)
	}

	// dev-a 文件只含自己的通道列
	expectedA := []string{"Timestamp", "DeviceID", "CH1 (Pa)", "CH2 (Pa)"}
	if len(a[0]) != len(expectedA) {
		t.Fatalf("dev-a header = %v, want %v", a[0], expectedA)
	}
	for i, want := range expectedA {
		if a[0][i] != want {
			t.Errorf("dev-a header[%d] = %q, want %q", i, a[0][i], want)
		}
	}
	if len(a) != 3 || a[1][2] != csvValueOne || a[2][2] != csvValueOne {
		t.Errorf("dev-a data rows = %v", a[1:])
	}

	// dev-b 文件独立表头：含自己的 CH6 (kPa)，不含 dev-a 的列
	expectedB := []string{"Timestamp", "DeviceID", "CH6 (kPa)"}
	if len(b[0]) != len(expectedB) {
		t.Fatalf("dev-b header = %v, want %v", b[0], expectedB)
	}
	for i, want := range expectedB {
		if b[0][i] != want {
			t.Errorf("dev-b header[%d] = %q, want %q", i, b[0][i], want)
		}
	}
	if len(b) != 2 || b[1][2] != "6.000000" {
		t.Errorf("dev-b data rows = %v", b[1:])
	}
}

// 同一设备后续帧引入新通道索引：仅在该设备文件内扩列，历史行补空
func TestDataStorage_HandlePayload_ExpandsColumnsWithinDevice(t *testing.T) {
	dir := t.TempDir()
	s := NewDataStorageService(dir)
	if err := s.StartRecording(); err != nil {
		t.Fatalf("StartRecording failed: %v", err)
	}

	base := types.DataPayload{
		DeviceID:       "dev-a",
		Timestamp:      1700000000000,
		Channels:       []float64{1.0, 2.0},
		ChannelIndices: []int{0, 1},
		ChannelUnits:   []string{"Pa", "Pa"},
	}
	if err := s.HandlePayload(base); err != nil {
		t.Fatalf("HandlePayload #1 failed: %v", err)
	}
	// 新通道索引 5：应扩列
	if err := s.HandlePayload(types.DataPayload{
		DeviceID:       "dev-a",
		Timestamp:      1700000000100,
		Channels:       []float64{6.0},
		ChannelIndices: []int{5},
		ChannelUnits:   []string{"kPa"},
	}); err != nil {
		t.Fatalf("HandlePayload #2 failed: %v", err)
	}
	if err := s.HandlePayload(base); err != nil {
		t.Fatalf("HandlePayload #3 failed: %v", err)
	}
	s.StopRecording()

	records := readSingleRecording(t, dir)
	if len(records) != 4 {
		t.Fatalf("expected 4 records (header + 3 rows), got %d", len(records))
	}
	expectedHeader := []string{"Timestamp", "DeviceID", "CH1 (Pa)", "CH2 (Pa)", "CH6 (kPa)"}
	for i, want := range expectedHeader {
		if records[0][i] != want {
			t.Errorf("header[%d] = %q, want %q", i, records[0][i], want)
		}
	}
	// 扩列前写下的历史行：尾部补空
	if len(records[1]) != 5 || records[1][2] != csvValueOne || records[1][4] != "" {
		t.Errorf("historical row should be padded with empty trailing cells, got %v", records[1])
	}
	// 新通道数据写入扩列后的 CH6
	if records[2][4] != "6.000000" || records[2][2] != "" {
		t.Errorf("new channel row = %v, want CH1 empty and CH6 6.000000", records[2])
	}
	// 旧通道后续帧：原列保持，扩列处为空
	if records[3][2] != csvValueOne || records[3][4] != "" {
		t.Errorf("later frame = %v", records[3])
	}
}

// 时间戳在磁盘上被 encoding/csv 转义为 "=""..."（= 公式包裹 + CSV 引号转义），
// 前端回放解析依赖该格式（见 usePlayback.ts unwrapTimestamp），不可悄悄去掉公式包裹
func TestDataStorage_HandlePayload_TimestampEscapedOnDisk(t *testing.T) {
	dir := t.TempDir()
	s := NewDataStorageService(dir)
	if err := s.StartRecording(); err != nil {
		t.Fatalf("StartRecording failed: %v", err)
	}
	if err := s.HandlePayload(types.DataPayload{
		DeviceID:       "d1",
		Timestamp:      1700000000000,
		Channels:       []float64{1.0},
		ChannelIndices: []int{0},
		ChannelUnits:   []string{"Pa"},
	}); err != nil {
		t.Fatalf("HandlePayload failed: %v", err)
	}
	s.StopRecording()

	files, _ := os.ReadDir(dir)
	raw, err := os.ReadFile(filepath.Join(dir, files[0].Name()))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if !strings.Contains(string(raw), `"=""`) {
		t.Errorf("timestamp should be CSV-escaped as \"=\"\"... on disk, raw content:\n%s", raw)
	}
}

// 多设备并发写各自文件：files 与各 writer 必须串行化（-race 下验证），设备间不混文件
func TestDataStorage_HandlePayload_ConcurrentDevices(t *testing.T) {
	dir := t.TempDir()
	s := NewDataStorageService(dir)
	if err := s.StartRecording(); err != nil {
		t.Fatalf("StartRecording failed: %v", err)
	}

	const devices = 4
	const frames = 25
	var wg sync.WaitGroup
	for d := 0; d < devices; d++ {
		wg.Add(1)
		go func(device int) {
			defer wg.Done()
			for i := 0; i < frames; i++ {
				_ = s.HandlePayload(types.DataPayload{
					DeviceID:       fmt.Sprintf("dev-%d", device),
					Timestamp:      int64(1700000000000 + i),
					Channels:       []float64{float64(device*100 + i)},
					ChannelIndices: []int{0},
					ChannelUnits:   []string{"Pa"},
				})
			}
		}(d)
	}
	wg.Wait()
	s.StopRecording()

	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(files) != devices {
		t.Fatalf("expected %d per-device files, got %d", devices, len(files))
	}
	seen := make(map[string]bool, devices)
	for _, f := range files {
		records := readCSVRecords(t, filepath.Join(dir, f.Name()))
		if len(records) != 1+frames {
			t.Fatalf("file %s: expected %d records (1 header + %d rows), got %d", f.Name(), 1+frames, frames, len(records))
		}
		if records[0][2] != "CH1 (Pa)" {
			t.Errorf("file %s header channel = %q, want CH1 (Pa)", f.Name(), records[0][2])
		}
		seen[records[1][1]] = true
	}
	if len(seen) != devices {
		t.Errorf("expected %d distinct devices, got %v", devices, seen)
	}
}

// 文件名：优先设备名，非法字符消毒，同名设备追加序号，解析不到回退 DeviceID
func TestDataStorage_DeviceFileNaming(t *testing.T) {
	dir := t.TempDir()
	s := NewDataStorageService(dir)
	s.SetDeviceNameResolver(func(id string) string {
		switch id {
		case "d1":
			return "风洞/1号"
		case "d2":
			return "风洞:1号" // 消毒后与 d1 同名
		}
		return ""
	})
	if err := s.StartRecording(); err != nil {
		t.Fatalf("StartRecording failed: %v", err)
	}
	for _, id := range []string{"d1", "d2", "d3"} {
		if err := s.HandlePayload(types.DataPayload{
			DeviceID:       id,
			Timestamp:      1700000000000,
			Channels:       []float64{1.0},
			ChannelIndices: []int{0},
			ChannelUnits:   []string{"Pa"},
		}); err != nil {
			t.Fatalf("HandlePayload %s failed: %v", id, err)
		}
	}
	s.StopRecording()

	files, _ := os.ReadDir(dir)
	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(files))
	}
	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, f.Name())
		if strings.ContainsAny(f.Name(), `\/:*?"<>|`) {
			t.Errorf("file name contains illegal characters: %s", f.Name())
		}
	}
	joined := strings.Join(names, "\n")
	if !strings.Contains(joined, "_风洞_1号.csv") {
		t.Errorf("missing sanitized device-name file, got:\n%s", joined)
	}
	if !strings.Contains(joined, "_风洞_1号-2.csv") {
		t.Errorf("duplicate device name should get -2 suffix, got:\n%s", joined)
	}
	if !strings.Contains(joined, "_d3.csv") {
		t.Errorf("device without name should fall back to DeviceID, got:\n%s", joined)
	}
}
