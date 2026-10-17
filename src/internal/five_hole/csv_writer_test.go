package five_hole

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"yx-daq/internal/types"
)

func TestFiveHoleCsvWriter_HeaderAndDataPoint(t *testing.T) {
	tmpDir := t.TempDir()
	w := NewFiveHoleCsvWriter()
	if err := w.Initialize(tmpDir, "test", "probe1", types.TraversalPatternLine); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer w.Close()

	// 验证文件存在
	filePath := filepath.Join(tmpDir, "test_probe1.csv")
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("csv file not created: %v", err)
	}

	// 写入数据点
	dp := types.FiveHoleTraversalDataPoint{
		PointID:       "pt-0",
		ProbeID:       "probe1",
		X:             10.5,
		Y:             20.3,
		XControllerName: "ctrl-α",
		XAxis:           "X",
		YControllerName: "ctrl-β",
		YAxis:           "Y",
		RawData: types.FiveHoleRawData{
			P1: 100.123456, P2: 101.654321, P3: 99.111111, P4: 100.222222, P5: 100.333333,
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
			Valid:           true,
		},
		SampleCount: 10,
		Timestamp:   time.Date(2024, 1, 2, 3, 4, 5, 678000000, time.Local).UnixMilli(),
	}
	if err := w.AppendPoint(dp); err != nil {
		t.Fatalf("AppendPoint failed: %v", err)
	}

	// flush 确保写入
	w.Close()

	// 读取文件验证内容
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read csv failed: %v", err)
	}
	str := string(content)

	// 验证 BOM
	if str[:3] != "\xEF\xBB\xBF" {
		t.Fatal("missing UTF-8 BOM")
	}

	// 验证表头含 β 列及富字段
	if !contains(str, "侧滑角Beta") {
		t.Fatal("header missing 侧滑角Beta")
	}
	if !contains(str, "X方向位移机构名") {
		t.Fatal("header missing X方向位移机构名")
	}
	if !contains(str, "Y方向位移机构名") {
		t.Fatal("header missing Y方向位移机构名")
	}
	if !contains(str, "P4") {
		t.Fatal("header missing P4")
	}
	if !contains(str, "P5") {
		t.Fatal("header missing P5")
	}
	if !contains(str, "校正空速CAS") {
		t.Fatal("header missing 校正空速CAS")
	}
	if !contains(str, "动压Qc") {
		t.Fatal("header missing 动压Qc")
	}
	if !contains(str, "速度Vz") {
		t.Fatal("header missing 速度Vz")
	}
	// 验证 T0（总温）列已移除
	if contains(str, "T0") {
		t.Fatal("header should not contain T0 (TTotal removed)")
	}
	// 验证旧字段已移除
	if contains(str, "迭代次数") {
		t.Fatal("header should not contain 迭代次数")
	}
	if contains(str, "收敛") {
		t.Fatal("header should not contain 收敛")
	}

	// 验证数据行
	if !contains(str, "probe1") {
		t.Fatal("data row missing probe1")
	}
	// 时间戳列为可读时间，且紧随点号列
	lines := strings.Split(strings.TrimRight(str, "\r\n"), "\n")
	headerFields := strings.Split(strings.TrimPrefix(lines[0], "\xEF\xBB\xBF"), ",")
	if len(headerFields) < 2 {
		t.Fatalf("header has too few columns: %v", headerFields)
	}
	if headerFields[0] != "点号" || headerFields[1] != "时间戳" {
		t.Fatalf("header should start with 点号,时间戳, got %s,%s", headerFields[0], headerFields[1])
	}
	if headerFields[len(headerFields)-1] != "采样数" {
		t.Fatalf("last header column should be 采样数, got %s", headerFields[len(headerFields)-1])
	}
	dataFields := strings.Split(lines[1], ",")
	if len(dataFields) != len(headerFields) {
		t.Fatalf("data field count %d != header field count %d", len(dataFields), len(headerFields))
	}
	if dataFields[0] != "pt-0" {
		t.Fatalf("first data column should be point id, got %s", dataFields[0])
	}
	if dataFields[1] != "2024-01-02 03:04:05.678" {
		t.Fatalf("timestamp should be readable, got %s", dataFields[1])
	}
	if !contains(str, "ctrl-α") {
		t.Fatal("data row missing alpha controller id")
	}
	if !contains(str, "ctrl-β") {
		t.Fatal("data row missing beta controller id")
	}
	if !contains(str, "5.2000") {
		t.Fatal("data row missing alpha value")
	}
	if !contains(str, "1.3000") {
		t.Fatal("data row missing beta value")
	}
	if !contains(str, "1.225000") {
		t.Fatal("data row missing density value")
	}
}

func TestFiveHoleCsvWriter_IndependentFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// 两个探针各一个 writer
	w1 := NewFiveHoleCsvWriter()
	if err := w1.Initialize(tmpDir, "test", "probe1", types.TraversalPatternLine); err != nil {
		t.Fatalf("Initialize probe1 failed: %v", err)
	}
	defer w1.Close()

	w2 := NewFiveHoleCsvWriter()
	if err := w2.Initialize(tmpDir, "test", "probe2", types.TraversalPatternLine); err != nil {
		t.Fatalf("Initialize probe2 failed: %v", err)
	}
	defer w2.Close()

	// 验证两个独立文件
	if _, err := os.Stat(filepath.Join(tmpDir, "test_probe1.csv")); err != nil {
		t.Fatalf("probe1 csv not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "test_probe2.csv")); err != nil {
		t.Fatalf("probe2 csv not created: %v", err)
	}
}

func TestFiveHoleCsvWriter_FanHeader(t *testing.T) {
	tmpDir := t.TempDir()
	w := NewFiveHoleCsvWriter()
	if err := w.Initialize(tmpDir, "test", "probe1", types.TraversalPatternFan); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer w.Close()

	w.Close()
	content, err := os.ReadFile(filepath.Join(tmpDir, "test_probe1.csv"))
	if err != nil {
		t.Fatalf("read csv failed: %v", err)
	}
	str := string(content)
	// 扇面模式点位列为 R(mm)/θ(°)，方向列标注 R/θ
	if !contains(str, "R(mm)") || !contains(str, "θ(°)") {
		t.Fatal("fan header missing R(mm)/θ(°) columns")
	}
	if !contains(str, "R方向位移机构名") || !contains(str, "θ方向位移机构名") {
		t.Fatal("fan header missing R/θ direction columns")
	}
	if contains(str, "X方向位移机构名") {
		t.Fatal("fan header should not contain X方向 columns")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
