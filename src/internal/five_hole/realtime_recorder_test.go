package five_hole

import (
	"bytes"
	"encoding/csv"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"yx-daq/internal/types"
)

// readRealtimeCSV5H 读取五孔实时录制 CSV 文件，自动跳过 UTF-8 BOM，返回解析后的所有记录
func readRealtimeCSV5H(t *testing.T, path string) [][]string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read csv file failed: %v", err)
	}
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	reader := csv.NewReader(bytes.NewReader(raw))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("parse csv failed: %v", err)
	}
	return records
}

// makeSampleRealtimeEvent 构造一个含 n 个探针的实时事件用于测试
func makeSampleRealtimeEvent(pointID, phase string, probeIDs ...string) types.FiveHoleTraversalRealtimeEvent {
	evt := types.FiveHoleTraversalRealtimeEvent{
		TaskID:  "task-rt",
		PointID: pointID,
		Phase:   phase,
	}
	for _, id := range probeIDs {
		evt.ProbeRealtime = append(evt.ProbeRealtime, types.FiveHoleProbeRealtimeItem{
			ProbeID: id,
			RawData: types.FiveHoleRawData{
				P1: 100.123456, P2: 101.654321, P3: 99.111111,
				P4: 100.222222, P5: 100.333333,
				PAtm: 101.325, TAtm: 20.5,
			},
			InterpResult: types.FiveHoleInterpolationResult{
				PtProbe:         102.5,
				PsProbe:         100.0,
				MachProbe:       0.45,
				AlphaProbe:      5.2,
				BetaProbe:       1.3,
				VelocityProbe:   150.0,
				CASProbe:        148.2,
				SATProbe:        293.15,
				DynamicPressure: 2500.0,
				Density:         1.225,
				VxProbe:         149.8,
				VyProbe:         5.0,
				VzProbe:         1.0,
			},
		})
	}
	return evt
}

// TestRealtimeRecorder_New_NotRecording 新建录制器初始状态应为未录制
func TestRealtimeRecorder_New_NotRecording(t *testing.T) {
	r := NewRealtimeRecorder()
	if r.IsRecording() {
		t.Error("新建录制器 IsRecording 应为 false")
	}
}

// TestRealtimeRecorder_Start_CreatesFileWithBOMAndHeader Start 应创建含 BOM 和 24 列表头的文件
func TestRealtimeRecorder_Start_CreatesFileWithBOMAndHeader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rt5h.csv")

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
	records := readRealtimeCSV5H(t, path)
	if len(records) < 1 {
		t.Fatal("应至少包含表头行")
	}
	expectedHeader := []string{
		"Timestamp", "PointID", "Phase", "ProbeID",
		"P1", "P2", "P3", "P4", "P5", "P∞", "T∞",
		"Pt", "Ps", "Ma", "α", "β", "V",
		"CAS", "SAT", "Qc", "ρ", "Vx", "Vy", "Vz",
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
	path := filepath.Join(dir, "nested", "deep", "rt5h.csv")

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
	path := filepath.Join(dir, "rt5h.csv")

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
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	invalidPath := filepath.Join(blocker, "rt5h.csv")

	r := NewRealtimeRecorder()
	err := r.Start(invalidPath)
	if err == nil {
		t.Error("无效路径 Start 应返回错误")
		r.Stop()
	}
}

// TestRealtimeRecorder_Record_WritesRowPerProbe 单事件含多探针时应写多行（每探针一行）
func TestRealtimeRecorder_Record_WritesRowPerProbe(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rt5h.csv")

	r := NewRealtimeRecorder()
	if err := r.Start(path); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	evt := makeSampleRealtimeEvent("p-001", "waiting", "probe1", "probe2", "probe3")
	r.Record(evt)
	r.Stop()

	records := readRealtimeCSV5H(t, path)
	if len(records) != 4 { // 表头 + 3 探针行
		t.Fatalf("应有 4 行（表头+3探针），实际 %d", len(records))
	}

	// 验证每行的 ProbeID
	for i, probeID := range []string{"probe1", "probe2", "probe3"} {
		row := records[i+1]
		if row[3] != probeID {
			t.Errorf("第 %d 行 ProbeID 应为 %s，实际 %s", i+1, probeID, row[3])
		}
		if row[2] != "waiting" {
			t.Errorf("第 %d 行 Phase 应为 waiting，实际 %s", i+1, row[2])
		}
		if row[1] != "p-001" {
			t.Errorf("第 %d 行 PointID 应为 p-001，实际 %s", i+1, row[1])
		}
	}
}

// TestRealtimeRecorder_Record_WritesCorrectFormat 验证单探针记录的数值格式
func TestRealtimeRecorder_Record_WritesCorrectFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rt5h.csv")

	r := NewRealtimeRecorder()
	if err := r.Start(path); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	evt := makeSampleRealtimeEvent("p-001", "acquiring", "probe1")
	r.Record(evt)
	r.Stop()

	records := readRealtimeCSV5H(t, path)
	if len(records) != 2 {
		t.Fatalf("应有 2 行（表头+1探针），实际 %d", len(records))
	}

	row := records[1]
	// P1: %.6f
	if row[4] != "100.123456" {
		t.Errorf("P1 应为 100.123456，实际 %s", row[4])
	}
	// TAtm: %.2f
	if row[10] != "20.50" {
		t.Errorf("TAtm 应为 20.50，实际 %s", row[10])
	}
	// α: %.4f
	if row[14] != "5.2000" {
		t.Errorf("α 应为 5.2000，实际 %s", row[14])
	}
	// β: %.4f
	if row[15] != "1.3000" {
		t.Errorf("β 应为 1.3000，实际 %s", row[15])
	}
	// ρ (Density): %.6f
	if row[20] != "1.225000" {
		t.Errorf("ρ 应为 1.225000，实际 %s", row[20])
	}
}

// TestRealtimeRecorder_Record_NotRecordingIsNoop 未录制时 Record 应静默无操作
func TestRealtimeRecorder_Record_NotRecordingIsNoop(t *testing.T) {
	r := NewRealtimeRecorder()
	evt := makeSampleRealtimeEvent("p-001", "waiting", "probe1")
	// 不应 panic
	r.Record(evt)
	if r.IsRecording() {
		t.Error("未 Start 的录制器不应变为录制状态")
	}
}

// TestRealtimeRecorder_Record_EmptyProbeRealtimeWritesNothing 空 ProbeRealtime 不写任何行
func TestRealtimeRecorder_Record_EmptyProbeRealtimeWritesNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rt5h.csv")

	r := NewRealtimeRecorder()
	if err := r.Start(path); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	evt := types.FiveHoleTraversalRealtimeEvent{
		TaskID:  "task-rt",
		PointID: "p-001",
		Phase:   "waiting",
		// ProbeRealtime 为空
	}
	r.Record(evt)
	r.Stop()

	records := readRealtimeCSV5H(t, path)
	if len(records) != 1 { // 仅表头
		t.Errorf("空 ProbeRealtime 应只写表头，实际 %d 行", len(records))
	}
}

// TestRealtimeRecorder_Record_MultipleEvents 多次事件各含多探针应写入正确总行数
func TestRealtimeRecorder_Record_MultipleEvents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rt5h.csv")

	r := NewRealtimeRecorder()
	if err := r.Start(path); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// 3 次事件，每次 2 探针 = 6 行数据
	for i := 0; i < 3; i++ {
		evt := makeSampleRealtimeEvent("p-001", "waiting", "probe1", "probe2")
		r.Record(evt)
	}
	r.Stop()

	records := readRealtimeCSV5H(t, path)
	if len(records) != 7 { // 表头 + 6 行
		t.Fatalf("应有 7 行（表头+6行数据），实际 %d", len(records))
	}
}

// TestRealtimeRecorder_Stop_ClosesFileAndResetsState Stop 后应关闭文件并重置状态
func TestRealtimeRecorder_Stop_ClosesFileAndResetsState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rt5h.csv")

	r := NewRealtimeRecorder()
	if err := r.Start(path); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	r.Stop()

	if r.IsRecording() {
		t.Error("Stop 后 IsRecording 应为 false")
	}

	// Stop 后 Record 应无操作
	r.Record(makeSampleRealtimeEvent("x", "waiting", "probe1"))

	records := readRealtimeCSV5H(t, path)
	if len(records) != 1 { // 仅表头
		t.Errorf("Stop 后不应再写入数据，应有 1 行（表头），实际 %d", len(records))
	}
}

// TestRealtimeRecorder_Stop_Idempotent 多次 Stop 不应 panic
func TestRealtimeRecorder_Stop_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rt5h.csv")

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
	r.Record(makeSampleRealtimeEvent("a", "waiting", "probe1"))
	r.Stop()

	// 第二轮
	if err := r.Start(path2); err != nil {
		t.Fatalf("第二次 Start 失败: %v", err)
	}
	r.Record(makeSampleRealtimeEvent("b", "waiting", "probe2"))
	r.Stop()

	rec1 := readRealtimeCSV5H(t, path1)
	if len(rec1) != 2 || rec1[1][3] != "probe1" {
		t.Errorf("文件1 数据错误: %v", rec1)
	}
	rec2 := readRealtimeCSV5H(t, path2)
	if len(rec2) != 2 || rec2[1][3] != "probe2" {
		t.Errorf("文件2 数据错误: %v", rec2)
	}
}

// TestRealtimeRecorder_ConcurrentRecord_NoPanic 并发 Record 不应 panic（验证 mutex 保护）
func TestRealtimeRecorder_ConcurrentRecord_NoPanic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rt5h.csv")

	r := NewRealtimeRecorder()
	if err := r.Start(path); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			r.Record(makeSampleRealtimeEvent("concurrent", "waiting", "probe1", "probe2"))
		}(i)
	}
	wg.Wait()
	r.Stop()

	records := readRealtimeCSV5H(t, path)
	if len(records) != 61 { // 表头 + 30 事件 × 2 探针 = 61
		t.Errorf("应有 61 行（表头+60行并发数据），实际 %d", len(records))
	}
}

// TestRealtimeRecorder_CSVHeader_Has24Columns CSV 表头应有 24 列
func TestRealtimeRecorder_CSVHeader_Has24Columns(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rt5h.csv")

	r := NewRealtimeRecorder()
	if err := r.Start(path); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	r.Stop()

	records := readRealtimeCSV5H(t, path)
	if len(records) == 0 {
		t.Fatal("无表头")
	}
	if len(records[0]) != 24 {
		t.Errorf("表头应有 24 列，实际 %d 列: %v", len(records[0]), records[0])
	}
}
