package five_hole

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"yx-daq/internal/types"
)

// TestFiveHoleInterpolator_NotLoaded 未加载时 Calculate 应返回 invalid
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

// TestFiveHoleInterpolator_SingleCalFile 单个 .cal 文件加载与计算
// 合成 13×13 网格（α/β ∈ [-30, 30]，步长 5），文件名含 Ma0.5 触发马赫数解析。
func TestFiveHoleInterpolator_SingleCalFile(t *testing.T) {
	tmpDir := t.TempDir()
	calPath := filepath.Join(tmpDir, "Ma0.5.cal")
	if err := os.WriteFile(calPath, []byte(syntheticCalLines()), 0644); err != nil {
		t.Fatalf("write test cal failed: %v", err)
	}

	i := NewFiveHoleInterpolator()
	if err := i.LoadCalibFiles([]string{calPath}); err != nil {
		t.Fatalf("load calib files failed: %v", err)
	}
	if !i.IsLoaded() {
		t.Fatal("should be loaded after LoadCalibFiles")
	}

	// 校验 GetCalibInfo：CMa 从文件名解析为 0.5，ValidRange 覆盖 [-30,30]
	infos := i.GetCalibInfo()
	if len(infos) != 1 {
		t.Fatalf("expected 1 calib info, got %d", len(infos))
	}
	if !floatEq(infos[0].CMa, 0.5) {
		t.Fatalf("expected CMa=0.5, got %f", infos[0].CMa)
	}
	if !floatEq(infos[0].ValidRange.AlphaMin, -30) || !floatEq(infos[0].ValidRange.AlphaMax, 30) {
		t.Fatalf("unexpected alpha range: %+v", infos[0].ValidRange)
	}
	if !floatEq(infos[0].ValidRange.MachMin, 0.5) || !floatEq(infos[0].ValidRange.MachMax, 0.5) {
		t.Fatalf("unexpected mach range: %+v", infos[0].ValidRange)
	}

	// 中心网格点 (0,0) 上对称压力（P1=P3, P4=P5）→ Ka=0,Kb=0，应解析为 α=0,β=0
	// P2 高于四孔均值以避免 delta≈0 警告并保证 pt>ps
	result := i.Calculate(types.FiveHoleRawData{
		P1: 95, P2: 110, P3: 95, P4: 95, P5: 95,
		PAtm: 101.325, TAtm: 20,
	})
	if !result.Valid {
		t.Fatalf("expected valid result, got invalid: %s", result.ErrorMsg)
	}
	if !floatEq(result.AlphaProbe, 0) || !floatEq(result.BetaProbe, 0) {
		t.Fatalf("expected alpha=0 beta=0, got alpha=%f beta=%f", result.AlphaProbe, result.BetaProbe)
	}
}

// TestFiveHoleInterpolator_MultiCalFile 多个 .cal 文件加载与计算
// 两个文件 Ma0.3.cal / Ma0.7.cal，验证加载成功并返回有效结果。
func TestFiveHoleInterpolator_MultiCalFile(t *testing.T) {
	tmpDir := t.TempDir()
	path03 := filepath.Join(tmpDir, "Ma0.3.cal")
	path07 := filepath.Join(tmpDir, "Ma0.7.cal")
	if err := os.WriteFile(path03, []byte(syntheticCalLines()), 0644); err != nil {
		t.Fatalf("write Ma0.3.cal failed: %v", err)
	}
	if err := os.WriteFile(path07, []byte(syntheticCalLines()), 0644); err != nil {
		t.Fatalf("write Ma0.7.cal failed: %v", err)
	}

	i := NewFiveHoleInterpolator()
	if err := i.LoadCalibFiles([]string{path03, path07}); err != nil {
		t.Fatalf("load multi calib files failed: %v", err)
	}
	if !i.IsLoaded() {
		t.Fatal("should be loaded after LoadCalibFiles")
	}

	infos := i.GetCalibInfo()
	if len(infos) != 2 {
		t.Fatalf("expected 2 calib infos, got %d", len(infos))
	}

	// 中心点应解析为 α=0,β=0
	result := i.Calculate(types.FiveHoleRawData{
		P1: 95, P2: 110, P3: 95, P4: 95, P5: 95,
		PAtm: 101.325, TAtm: 20,
	})
	if !result.Valid {
		t.Fatalf("expected valid result, got invalid: %s", result.ErrorMsg)
	}
	if !floatEq(result.AlphaProbe, 0) || !floatEq(result.BetaProbe, 0) {
		t.Fatalf("expected alpha=0 beta=0, got alpha=%f beta=%f", result.AlphaProbe, result.BetaProbe)
	}
}

// TestFiveHoleInterpolator_EmptyFile 空文件应报错
func TestFiveHoleInterpolator_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	calPath := filepath.Join(tmpDir, "Ma0.5.cal")
	if err := os.WriteFile(calPath, []byte(""), 0644); err != nil {
		t.Fatalf("write empty cal failed: %v", err)
	}

	i := NewFiveHoleInterpolator()
	err := i.LoadCalibFiles([]string{calPath})
	if err == nil {
		t.Fatal("expected error for empty file")
	}
}

// TestFiveHoleInterpolator_BadHeader 错误表头应报错
func TestFiveHoleInterpolator_BadHeader(t *testing.T) {
	tmpDir := t.TempDir()
	calPath := filepath.Join(tmpDir, "Ma0.5.cal")
	// 表头不是 13 13
	content := "12 12\n" + syntheticCalLinesBody()
	if err := os.WriteFile(calPath, []byte(content), 0644); err != nil {
		t.Fatalf("write bad header cal failed: %v", err)
	}

	i := NewFiveHoleInterpolator()
	err := i.LoadCalibFiles([]string{calPath})
	if err == nil {
		t.Fatal("expected error for bad header")
	}
}

// TestFiveHoleInterpolator_MalformedRow 数据行列数错误应报错
func TestFiveHoleInterpolator_MalformedRow(t *testing.T) {
	tmpDir := t.TempDir()
	calPath := filepath.Join(tmpDir, "Ma0.5.cal")
	// 表头正确，但首行数据只有 3 列
	content := "13 13\n0.1 0.2 1.0\n"
	if err := os.WriteFile(calPath, []byte(content), 0644); err != nil {
		t.Fatalf("write malformed cal failed: %v", err)
	}

	i := NewFiveHoleInterpolator()
	err := i.LoadCalibFiles([]string{calPath})
	if err == nil {
		t.Fatal("expected error for malformed row")
	}
}

// TestFiveHoleInterpolator_NoMachInFileName 文件名无法解析马赫数应报错
func TestFiveHoleInterpolator_NoMachInFileName(t *testing.T) {
	tmpDir := t.TempDir()
	// 文件名没有 Ma 标记，算法包会跳过该文件并最终返回错误
	calPath := filepath.Join(tmpDir, "unknown.cal")
	if err := os.WriteFile(calPath, []byte(syntheticCalLines()), 0644); err != nil {
		t.Fatalf("write unknown.cal failed: %v", err)
	}

	i := NewFiveHoleInterpolator()
	err := i.LoadCalibFiles([]string{calPath})
	if err == nil {
		t.Fatal("expected error when mach number cannot be parsed from file name")
	}
}

// syntheticCalLines 生成 13×13 合成 .cal 文件内容（表头 + 169 行）
// 每行 ka kb cpt cps alpha beta；alpha/beta 遍历 [-30,30] 步长 5
func syntheticCalLines() string {
	return "13 13\n" + syntheticCalLinesBody()
}

func syntheticCalLinesBody() string {
	const (
		gridMin  = -30
		gridMax  = 30
		gridStep = 5
	)
	var buf []byte
	for a := gridMin; a <= gridMax; a += gridStep {
		for b := gridMin; b <= gridMax; b += gridStep {
			// 简单合成：ka = alpha/30, kb = beta/30, cpt = 0.5, cps = 0.3
			line := formatCalRow(float64(a)/30, float64(b)/30, 0.5, 0.3, float64(a), float64(b))
			buf = append(buf, []byte(line)...)
		}
	}
	return string(buf)
}

// formatCalRow 格式化一行 .cal 数据（ka kb cpt cps alpha beta，6 列空格分隔）
// 使用 'f' 格式 + -1 精度，让整数无小数点（-30）、小数尽量短（0.166667）。
func formatCalRow(ka, kb, cpt, cps, alpha, beta float64) string {
	return ftoStr(ka) + " " + ftoStr(kb) + " " + ftoStr(cpt) + " " + ftoStr(cps) + " " +
		ftoStr(alpha) + " " + ftoStr(beta) + "\n"
}

// ftoStr 紧凑格式化 float：整数无小数点，小数保留 6 位
func ftoStr(f float64) string {
	if f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', 6, 64)
}

// floatEq 容差比较（1e-6）
func floatEq(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 1e-6
}
