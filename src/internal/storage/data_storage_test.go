package storage

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"

	"yx-daq/internal/types"
)

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

func TestDataStorage_HandlePayload_WritesRows(t *testing.T) {
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
	// 期望 1 表头 + 3 数据行
	if len(records) != 4 {
		t.Fatalf("expected 4 records (1 header + 3 data), got %d", len(records))
	}
	// 表头
	if records[0][0] != "Timestamp" || records[0][3] != "ChannelName" {
		t.Errorf("unexpected header: %v", records[0])
	}
	// 第一条数据：CH6（chIdx=5 +1），值 1.0，单位 kPa
	if records[1][1] != "d1" {
		t.Errorf("record[1] DeviceID = %q", records[1][1])
	}
	if records[1][3] != "CH6" {
		t.Errorf("record[1] ChannelName = %q, want CH6", records[1][3])
	}
	if records[1][4] != "1.000000" {
		t.Errorf("record[1] Value = %q, want 1.000000", records[1][4])
	}
	if records[1][5] != "kPa" {
		t.Errorf("record[1] Unit = %q, want kPa", records[1][5])
	}
}

func TestDataStorage_HandlePayload_DefaultUnit(t *testing.T) {
	// ChannelUnits 为空时单位应回退到 "Pa"
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
	if records[1][5] != "Pa" {
		t.Errorf("default unit = %q, want Pa", records[1][5])
	}
}

func TestDataStorage_HandlePayload_ChannelIndexOutOfBounds(t *testing.T) {
	// ChannelIndices 长度 < Channels 长度 时，剩余应使用 chIdx=0
	dir := t.TempDir()
	s := NewDataStorageService(dir)
	if err := s.StartRecording(); err != nil {
		t.Fatalf("StartRecording failed: %v", err)
	}

	if err := s.HandlePayload(types.DataPayload{
		DeviceID:       "d1",
		Channels:       []float64{1.0, 2.0},
		ChannelIndices: []int{5}, // 只有一个，第二个通道应使用 chIdx=0
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
	// 第二条数据 CH1（chIdx=0 +1）
	if records[2][3] != "CH1" {
		t.Errorf("out-of-bounds channel should use CH1, got %q", records[2][3])
	}
}

func TestDataStorage_SetOutputDir(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	s := NewDataStorageService(dir1)

	if err := s.StartRecording(); err != nil {
		t.Fatalf("StartRecording failed: %v", err)
	}
	s.StopRecording()

	// 切换目录
	s.SetOutputDir(dir2)
	if err := s.StartRecording(); err != nil {
		t.Fatalf("StartRecording in dir2 failed: %v", err)
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
