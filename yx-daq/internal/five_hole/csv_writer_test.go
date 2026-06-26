package five_hole

import (
	"os"
	"path/filepath"
	"testing"

	"yx-daq/internal/types"
)

func TestFiveHoleCsvWriter_HeaderAndDataPoint(t *testing.T) {
	tmpDir := t.TempDir()
	w := NewFiveHoleCsvWriter()
	if err := w.Initialize(tmpDir, "test", "probe1"); err != nil {
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
		PointID: "pt-0",
		ProbeID: "probe1",
		X:       10.5,
		Y:       20.3,
		RawData: types.FiveHoleRawData{
			P1: 100.123456, P2: 101.654321, P3: 99.111111, P4: 100.222222, P5: 100.333333,
			PAtm: 101.325, TAtm: 20.5,
		},
		InterpResult: types.FiveHoleInterpolationResult{
			PtProbe:        102.5,
			PsProbe:        100.0,
			MachProbe:      0.45,
			AlphaProbe:     5.2,
			BetaProbe:      1.3,
			VelocityProbe:  150.0,
			IterationCount: 5,
			Converged:      true,
			Valid:          true,
		},
		SampleCount: 10,
		Timestamp:   1700000000,
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

	// 验证表头含 β 列
	if !contains(str, "侧滑角Beta") {
		t.Fatal("header missing 侧滑角Beta")
	}
	if !contains(str, "P4") {
		t.Fatal("header missing P4")
	}
	if !contains(str, "P5") {
		t.Fatal("header missing P5")
	}

	// 验证数据行
	if !contains(str, "probe1") {
		t.Fatal("data row missing probe1")
	}
	if !contains(str, "5.2000") {
		t.Fatal("data row missing alpha value")
	}
	if !contains(str, "1.3000") {
		t.Fatal("data row missing beta value")
	}
}

func TestFiveHoleCsvWriter_IndependentFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// 两个探针各一个 writer
	w1 := NewFiveHoleCsvWriter()
	if err := w1.Initialize(tmpDir, "test", "probe1"); err != nil {
		t.Fatalf("Initialize probe1 failed: %v", err)
	}
	defer w1.Close()

	w2 := NewFiveHoleCsvWriter()
	if err := w2.Initialize(tmpDir, "test", "probe2"); err != nil {
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

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
