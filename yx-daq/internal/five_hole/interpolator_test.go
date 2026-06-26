package five_hole

import (
	"os"
	"path/filepath"
	"testing"

	"yx-daq/internal/types"
)

func TestFiveHoleInterpolator_NotLoaded(t *testing.T) {
	i := NewFiveHoleInterpolator()
	if i.IsLoaded() {
		t.Fatal("new interpolator should not be loaded")
	}
	result := i.Calculate(types.FiveHoleRawData{P1: 100, P2: 101, P3: 99, P4: 100, P5: 100, PAtm: 101.3, TAtm: 20})
	if result.Valid {
		t.Fatal("not loaded should return invalid result")
	}
	if result.ErrorMsg != "校准文件未载入" {
		t.Fatalf("expected 校准文件未载入, got %s", result.ErrorMsg)
	}
}

func TestFiveHoleInterpolator_LoadAndCalculate_NotImplemented(t *testing.T) {
	// 创建临时 .prb 测试文件
	tmpDir := t.TempDir()
	prbPath := filepath.Join(tmpDir, "test.prb")
	content := `0.5
0.1 0.2 1.0 0.9 5.0 1.0
0.2 0.3 1.1 0.95 10.0 2.0
0.3 0.4 1.2 1.0 15.0 3.0
`
	if err := os.WriteFile(prbPath, []byte(content), 0644); err != nil {
		t.Fatalf("write test prb failed: %v", err)
	}

	i := NewFiveHoleInterpolator()
	if err := i.LoadCalibFiles([]string{prbPath}); err != nil {
		t.Fatalf("load calib files failed: %v", err)
	}
	if !i.IsLoaded() {
		t.Fatal("should be loaded after LoadCalibFiles")
	}

	// 校验 GetCalibInfo
	infos := i.GetCalibInfo()
	if len(infos) != 1 {
		t.Fatalf("expected 1 calib info, got %d", len(infos))
	}
	if infos[0].CMa != 0.5 {
		t.Fatalf("expected CMa=0.5, got %f", infos[0].CMa)
	}

	// 占位 Calculate 应返回 NotImplemented
	result := i.Calculate(types.FiveHoleRawData{P1: 100, P2: 101, P3: 99, P4: 100, P5: 100, PAtm: 101.3, TAtm: 20})
	if result.Valid {
		t.Fatal("placeholder Calculate should return invalid")
	}
	if result.ErrorMsg != "五孔插值算法未实现（待提供）" {
		t.Fatalf("expected NotImplemented error, got %s", result.ErrorMsg)
	}
}

func TestFiveHoleInterpolator_LoadEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	prbPath := filepath.Join(tmpDir, "empty.prb")
	// 仅 CMa 无数据行
	content := `0.5
`
	if err := os.WriteFile(prbPath, []byte(content), 0644); err != nil {
		t.Fatalf("write test prb failed: %v", err)
	}

	i := NewFiveHoleInterpolator()
	err := i.LoadCalibFiles([]string{prbPath})
	if err == nil {
		t.Fatal("expected error for empty entries file")
	}
}

func TestFiveHoleInterpolator_LoadMalformedFile(t *testing.T) {
	tmpDir := t.TempDir()
	prbPath := filepath.Join(tmpDir, "bad.prb")
	// 数据行列数不足
	content := `0.5
0.1 0.2 1.0
`
	if err := os.WriteFile(prbPath, []byte(content), 0644); err != nil {
		t.Fatalf("write test prb failed: %v", err)
	}

	i := NewFiveHoleInterpolator()
	err := i.LoadCalibFiles([]string{prbPath})
	if err == nil {
		t.Fatal("expected error for malformed file")
	}
}
