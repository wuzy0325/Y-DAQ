package three_hole

import (
	"bytes"
	"encoding/csv"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"yx-daq/internal/types"
)

// readRealtimeCSV 读取实时录制 CSV 文件，自动跳过 UTF-8 BOM，返回解析后的所有记录
func readRealtimeCSV(t *testing.T, path string) [][]string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read csv file failed: %v", err)
	}
	// 跳过 UTF-8 BOM（0xEF 0xBB 0xBF）
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	reader := csv.NewReader(bytes.NewReader(raw))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("parse csv failed: %v", err)
	}
	return records
}

// TestNewRealtimeRecorder_NotRecording 新建录制器初始状态应为未录制
func TestNewRealtimeRecorder_NotRecording(t *testing.T) {
	r := NewRealtimeRecorder()
	if r.IsRecording() {
		t.Error("新建录制器 IsRecording 应为 false")
	}
}

// TestRealtimeRecorder_Start_CreatesFileWithBOMAndHeader Start 应创建含 BOM 和表头的文件
func TestRealtimeRecorder_Start_CreatesFileWithBOMAndHeader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rt.csv")

	r := NewRealtimeRecorder()
	if err := r.Start(path); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer r.Stop()

	if !r.IsRecording() {
		t.Error("Start 后 IsRecording 应为 true")
	}

	// 验证文件存在
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("CSV 文件未创建")
	}

	// 验证 BOM
	raw, _ := os.ReadFile(path)
	if len(raw) < 3 || raw[0] != 0xEF || raw[1] != 0xBB || raw[2] != 0xBF {
		t.Error("文件应以 UTF-8 BOM 开头")
	}

	// 验证表头
	records := readRealtimeCSV(t, path)
	if len(records) < 1 {
		t.Fatal("应至少包含表头行")
	}
	expectedHeader := []string{
		"Timestamp", "PointID",
		"P1", "P2", "P3", "P∞", "T∞",
		"Pt", "Ps", "Ma", "α", "V",
		"Iterations",
	}
	if len(records[0]) != len(expectedHeader) {
		t.Fatalf("表头列数应为 %d，实际 %d", len(expectedHeader), len(records[0]))
	}
	for i, h := range expectedHeader {
		if records[0][i] != h {
			t.Errorf("表头第 %d 列应为 %q，实际 %q", i, h, records[0][i])
		}
	}
}

// TestRealtimeRecorder_Start_NestedDirAutoCreated Start 应自动创建嵌套目录
func TestRealtimeRecorder_Start_NestedDirAutoCreated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "rt.csv")

	r := NewRealtimeRecorder()
	if err := r.Start(path); err != nil {
		t.Fatalf("Start 嵌套目录失败: %v", err)
	}
	r.Stop()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("嵌套目录下的文件未创建")
	}
}

// TestRealtimeRecorder_Start_AlreadyRecordingReturnsError 已在录制时 Start 应返回错误
func TestRealtimeRecorder_Start_AlreadyRecordingReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rt.csv")

	r := NewRealtimeRecorder()
	if err := r.Start(path); err != nil {
		t.Fatalf("首次 Start 失败: %v", err)
	}
	defer r.Stop()

	err := r.Start(path)
	if err == nil {
		t.Error("重复 Start 应返回错误")
	}
}

// TestRealtimeRecorder_Start_InvalidPathReturnsError 无效路径 Start 应返回错误
func TestRealtimeRecorder_Start_InvalidPathReturnsError(t *testing.T) {
	dir := t.TempDir()
	// 用一个已存在的文件作为父目录，MkdirAll 应失败
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	invalidPath := filepath.Join(blocker, "rt.csv")

	r := NewRealtimeRecorder()
	err := r.Start(invalidPath)
	if err == nil {
		t.Error("无效路径 Start 应返回错误")
		r.Stop()
	}
}

// TestRealtimeRecorder_Record_WritesRow Record 应写入一条格式正确的记录
func TestRealtimeRecorder_Record_WritesRow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rt.csv")

	r := NewRealtimeRecorder()
	if err := r.Start(path); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	evt := types.ThreeHoleTraversalRealtimeEvent{
		TaskID:  "task-1",
		PointID: "p-001",
		RawData: types.ThreeHoleRawData{
			P1: 100.123456, P2: 105.654321, P3: 102.0,
			PAtm: 101.325, TAtm: 25.5,
		},
		InterpResult: types.ThreeHoleInterpolationResult{
			PtProbe: 1000.0, PsProbe: 500.0, MachProbe: 0.8,
			AlphaProbe: 5.25, VelocityProbe: 250.5, IterationCount: 3,
		},
	}
	r.Record(evt)
	r.Stop()

	records := readRealtimeCSV(t, path)
	if len(records) != 2 { // 表头 + 1 行数据
		t.Fatalf("应有 2 行（表头+数据），实际 %d", len(records))
	}

	row := records[1]
	if row[1] != "p-001" {
		t.Errorf("PointID 应为 p-001，实际 %s", row[1])
	}
	if row[2] != "100.123456" {
		t.Errorf("P1 应为 100.123456，实际 %s", row[2])
	}
	if row[6] != "25.50" {
		t.Errorf("TAtm 应为 25.50（%.2f），实际 %s", 25.5, row[6])
	}
	if row[12] != "3" {
		t.Errorf("Iterations 应为 3，实际 %s", row[12])
	}
}

// TestRealtimeRecorder_Record_NotRecordingIsNoop 未录制时 Record 应静默无操作
func TestRealtimeRecorder_Record_NotRecordingIsNoop(t *testing.T) {
	r := NewRealtimeRecorder()
	evt := types.ThreeHoleTraversalRealtimeEvent{
		PointID: "p-001",
		RawData: types.ThreeHoleRawData{P1: 100, P2: 105, P3: 102, PAtm: 101.325, TAtm: 25},
	}
	// 不应 panic
	r.Record(evt)
	if r.IsRecording() {
		t.Error("未 Start 的录制器不应变为录制状态")
	}
}

// TestRealtimeRecorder_Record_MultipleRows 多次 Record 应写入多行
func TestRealtimeRecorder_Record_MultipleRows(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rt.csv")

	r := NewRealtimeRecorder()
	if err := r.Start(path); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	for i := 0; i < 10; i++ {
		evt := types.ThreeHoleTraversalRealtimeEvent{
			PointID: "p-001",
			RawData: types.ThreeHoleRawData{P1: float64(i), P2: 105, P3: 102, PAtm: 101.325, TAtm: 25},
			InterpResult: types.ThreeHoleInterpolationResult{
				IterationCount: i,
			},
		}
		r.Record(evt)
	}
	r.Stop()

	records := readRealtimeCSV(t, path)
	if len(records) != 11 { // 表头 + 10 行
		t.Fatalf("应有 11 行（表头+10行数据），实际 %d", len(records))
	}
}

// TestRealtimeRecorder_Stop_ClosesFileAndResetsState Stop 后应关闭文件并重置状态
func TestRealtimeRecorder_Stop_ClosesFileAndResetsState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rt.csv")

	r := NewRealtimeRecorder()
	if err := r.Start(path); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	r.Stop()

	if r.IsRecording() {
		t.Error("Stop 后 IsRecording 应为 false")
	}

	// Stop 后 Record 应无操作
	evt := types.ThreeHoleTraversalRealtimeEvent{PointID: "x"}
	r.Record(evt)

	records := readRealtimeCSV(t, path)
	if len(records) != 1 { // 仅表头
		t.Errorf("Stop 后不应再写入数据，应有 1 行（表头），实际 %d", len(records))
	}
}

// TestRealtimeRecorder_Stop_Idempotent 多次 Stop 不应 panic
func TestRealtimeRecorder_Stop_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rt.csv")

	r := NewRealtimeRecorder()
	if err := r.Start(path); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	r.Stop()
	r.Stop() // 不应 panic
	r.Stop() // 不应 panic
}

// TestRealtimeRecorder_Stop_NotStartedIsNoop 未 Start 时 Stop 应无操作
func TestRealtimeRecorder_Stop_NotStartedIsNoop(t *testing.T) {
	r := NewRealtimeRecorder()
	r.Stop() // 不应 panic
	if r.IsRecording() {
		t.Error("未 Start 的录制器 Stop 后不应为录制状态")
	}
}

// TestRealtimeRecorder_StartStopRecord_Cycle Stop 后可重新 Start 并 Record
func TestRealtimeRecorder_StartStopRecord_Cycle(t *testing.T) {
	dir := t.TempDir()
	path1 := filepath.Join(dir, "rt1.csv")
	path2 := filepath.Join(dir, "rt2.csv")

	r := NewRealtimeRecorder()

	// 第一轮
	if err := r.Start(path1); err != nil {
		t.Fatalf("第一次 Start 失败: %v", err)
	}
	r.Record(types.ThreeHoleTraversalRealtimeEvent{PointID: "a"})
	r.Stop()

	// 第二轮（不同文件）
	if err := r.Start(path2); err != nil {
		t.Fatalf("第二次 Start 失败: %v", err)
	}
	r.Record(types.ThreeHoleTraversalRealtimeEvent{PointID: "b"})
	r.Stop()

	// 验证两个文件各有 1 行数据
	rec1 := readRealtimeCSV(t, path1)
	if len(rec1) != 2 || rec1[1][1] != "a" {
		t.Errorf("文件1 数据错误: %v", rec1)
	}
	rec2 := readRealtimeCSV(t, path2)
	if len(rec2) != 2 || rec2[1][1] != "b" {
		t.Errorf("文件2 数据错误: %v", rec2)
	}
}

// TestRealtimeRecorder_ConcurrentRecord_NoPanic 并发 Record 不应 panic（验证 mutex 保护）
func TestRealtimeRecorder_ConcurrentRecord_NoPanic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rt.csv")

	r := NewRealtimeRecorder()
	if err := r.Start(path); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			r.Record(types.ThreeHoleTraversalRealtimeEvent{
				PointID: "concurrent",
				RawData: types.ThreeHoleRawData{P1: float64(n)},
			})
		}(i)
	}
	wg.Wait()
	r.Stop()

	records := readRealtimeCSV(t, path)
	if len(records) != 51 { // 表头 + 50 行
		t.Errorf("应有 51 行（表头+50行并发数据），实际 %d", len(records))
	}
}

// TestRealtimeRecorder_CSVHeader_Has13Columns CSV 表头应有 13 列
func TestRealtimeRecorder_CSVHeader_Has13Columns(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rt.csv")

	r := NewRealtimeRecorder()
	if err := r.Start(path); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	r.Stop()

	records := readRealtimeCSV(t, path)
	if len(records) == 0 {
		t.Fatal("无表头")
	}
	if len(records[0]) != 13 {
		t.Errorf("表头应有 13 列，实际 %d 列: %v", len(records[0]), records[0])
	}
}
