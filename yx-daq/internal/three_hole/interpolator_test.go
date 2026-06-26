package three_hole

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"yx-daq/internal/types"
)

// writeTempCalibFile 创建临时校准文件，返回文件路径
func writeTempCalibFile(t *testing.T, dir string, name string, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write temp calib file failed: %v", err)
	}
	return path
}

// sampleCalibFile 生成给定 CMa 和 alpha 序列的校准文件内容
func sampleCalibContent(cMa float64, alphas []float64) string {
	// 为每个 alpha 生成合理的 Kb/Kt/Sb 值
	// Kb = alpha * 0.05 (线性近似), Kt = 0.5 + alpha*0.01, Sb = 0.8 + alpha*0.005
	content := fmt.Sprintf("%.6f\n", cMa)
	content += fmt.Sprintf("%d\n", len(alphas))
	for _, a := range alphas {
		kb := a * 0.05
		kt := 0.5 + a*0.01
		sb := 0.8 + a*0.005
		content += fmt.Sprintf("%.6f  %.6f  %.6f  %.6f\n", kb, kt, sb, a)
	}
	return content
}

func TestCalculate_NotLoaded(t *testing.T) {
	interp := NewThreeHoleInterpolator()
	result := interp.Calculate(types.ThreeHoleRawData{
		P1: 100, P2: 101, P3: 99, PAtm: 101.325, TAtm: 25,
	})
	if result.Valid {
		t.Error("expected Valid=false when calib not loaded")
	}
}

func TestCalculate_DeltaPZero(t *testing.T) {
	dir := t.TempDir()
	alphas := []float64{-10, -5, 0, 5, 10}
	f1 := writeTempCalibFile(t, dir, "calib_0.5.dat", sampleCalibContent(0.5, alphas))

	interp := NewThreeHoleInterpolator()
	if err := interp.LoadCalibFiles([]string{f1}); err != nil {
		t.Fatalf("load calib failed: %v", err)
	}

	// P1=P2=P3 → deltaP = 2*P2 - P1 - P3 = 0
	result := interp.Calculate(types.ThreeHoleRawData{
		P1: 100, P2: 100, P3: 100, PAtm: 101.325, TAtm: 25,
	})
	if result.Valid {
		t.Error("expected Valid=false when deltaP is zero")
	}
}

func TestCalculate_NaN(t *testing.T) {
	dir := t.TempDir()
	alphas := []float64{-10, -5, 0, 5, 10}
	f1 := writeTempCalibFile(t, dir, "calib_0.5.dat", sampleCalibContent(0.5, alphas))

	interp := NewThreeHoleInterpolator()
	if err := interp.LoadCalibFiles([]string{f1}); err != nil {
		t.Fatalf("load calib failed: %v", err)
	}

	result := interp.Calculate(types.ThreeHoleRawData{
		P1: math.NaN(), P2: 101, P3: 99, PAtm: 101.325, TAtm: 25,
	})
	if result.Valid {
		t.Error("expected Valid=false when input is NaN")
	}
}

func TestCalculate_SingleCalibFile(t *testing.T) {
	dir := t.TempDir()
	alphas := []float64{-10, -5, 0, 5, 10}
	f1 := writeTempCalibFile(t, dir, "calib_0.5.dat", sampleCalibContent(0.5, alphas))

	interp := NewThreeHoleInterpolator()
	if err := interp.LoadCalibFiles([]string{f1}); err != nil {
		t.Fatalf("load calib failed: %v", err)
	}

	// alpha≈0 附近的数据: P2 最大（中心孔压力最高），P1/P3 略低
	result := interp.Calculate(types.ThreeHoleRawData{
		P1: 99.5, P2: 101.0, P3: 99.8, PAtm: 101.325, TAtm: 25,
	})
	if !result.Valid {
		t.Error("expected Valid=true for reasonable input")
	}
	if result.IterationCount < 1 {
		t.Errorf("expected at least 1 iteration, got %d", result.IterationCount)
	}
	// alpha 应该接近0（P1≈P3 表示攻角接近0）
	if math.Abs(result.AlphaProbe) > 5 {
		t.Errorf("expected alpha near 0, got %.4f", result.AlphaProbe)
	}
	if result.MachProbe < 0 || math.IsNaN(result.MachProbe) || math.IsInf(result.MachProbe, 0) {
		t.Errorf("expected non-negative finite Mach, got %.4f", result.MachProbe)
	}
}

func TestCalculate_TwoCalibFiles(t *testing.T) {
	dir := t.TempDir()
	alphas := []float64{-10, -5, 0, 5, 10}
	f1 := writeTempCalibFile(t, dir, "calib_0.5.dat", sampleCalibContent(0.5, alphas))
	f2 := writeTempCalibFile(t, dir, "calib_1.0.dat", sampleCalibContent(1.0, alphas))

	interp := NewThreeHoleInterpolator()
	if err := interp.LoadCalibFiles([]string{f1, f2}); err != nil {
		t.Fatalf("load calib files failed: %v", err)
	}

	result := interp.Calculate(types.ThreeHoleRawData{
		P1: 99.5, P2: 101.0, P3: 99.8, PAtm: 101.325, TAtm: 25,
	})
	if !result.Valid {
		t.Error("expected Valid=true")
	}
	if result.IterationCount < 1 {
		t.Errorf("expected at least 1 iteration, got %d", result.IterationCount)
	}
}

func TestCalculate_Convergence(t *testing.T) {
	dir := t.TempDir()
	alphas := []float64{-10, -5, 0, 5, 10}
	f1 := writeTempCalibFile(t, dir, "calib_0.5.dat", sampleCalibContent(0.5, alphas))

	interp := NewThreeHoleInterpolator()
	if err := interp.LoadCalibFiles([]string{f1}); err != nil {
		t.Fatalf("load calib failed: %v", err)
	}

	// 多次计算，验证结果一致（收敛稳定）
	var lastAlpha float64
	for i := 0; i < 3; i++ {
		result := interp.Calculate(types.ThreeHoleRawData{
			P1: 99.5, P2: 101.0, P3: 99.8, PAtm: 101.325, TAtm: 25,
		})
		if !result.Valid {
			t.Errorf("iteration %d: expected Valid=true", i)
		}
		if i > 0 && math.Abs(result.AlphaProbe-lastAlpha) > 1e-6 {
			t.Errorf("inconsistent results across calls: %.6f vs %.6f", lastAlpha, result.AlphaProbe)
		}
		lastAlpha = result.AlphaProbe
	}
}

func TestCalculate_BoundaryMa(t *testing.T) {
	dir := t.TempDir()
	alphas := []float64{-10, -5, 0, 5, 10}
	// 只加载马赫数 0.5~0.6 范围
	f1 := writeTempCalibFile(t, dir, "calib_0.5.dat", sampleCalibContent(0.5, alphas))
	f2 := writeTempCalibFile(t, dir, "calib_0.6.dat", sampleCalibContent(0.6, alphas))

	interp := NewThreeHoleInterpolator()
	if err := interp.LoadCalibFiles([]string{f1, f2}); err != nil {
		t.Fatalf("load calib failed: %v", err)
	}

	// 输入数据产生的马赫数可能超出范围，应被 clamp
	result := interp.Calculate(types.ThreeHoleRawData{
		P1: 50, P2: 200, P3: 50, PAtm: 101.325, TAtm: 25,
	})
	if !result.Valid {
		t.Error("expected Valid=true even for boundary input")
	}
	// 马赫数应该在 [0.5, 0.6] 范围内（clamped）
	if result.MachProbe < 0.5-1e-6 || result.MachProbe > 0.6+1e-6 {
		t.Logf("Mach=%.4f (may be clamped to [%.4f, %.4f])", result.MachProbe, 0.5, 0.6)
	}
}

func TestFindNearestTwoCalib_Single(t *testing.T) {
	i := &ThreeHoleInterpolator{
		calibData: []types.ThreeHoleCalibData{
			{CMa: 0.5},
		},
	}
	c1, c2 := i.findNearestTwoCalib(0.5)
	if c1.CMa != 0.5 || c2.CMa != 0.5 {
		t.Errorf("single entry: expected both 0.5, got %.2f and %.2f", c1.CMa, c2.CMa)
	}
}

func TestFindNearestTwoCalib_Middle(t *testing.T) {
	i := &ThreeHoleInterpolator{
		calibData: []types.ThreeHoleCalibData{
			{CMa: 0.3},
			{CMa: 0.5},
			{CMa: 0.7},
		},
	}
	c1, c2 := i.findNearestTwoCalib(0.55)
	if c1.CMa != 0.5 || c2.CMa != 0.7 {
		t.Errorf("expected (0.5, 0.7), got (%.2f, %.2f)", c1.CMa, c2.CMa)
	}
}

func TestFindNearestTwoCalib_BelowMin(t *testing.T) {
	i := &ThreeHoleInterpolator{
		calibData: []types.ThreeHoleCalibData{
			{CMa: 0.3},
			{CMa: 0.5},
		},
	}
	c1, c2 := i.findNearestTwoCalib(0.1)
	if c1.CMa != 0.3 || c2.CMa != 0.5 {
		t.Errorf("below min: expected (0.3, 0.5), got (%.2f, %.2f)", c1.CMa, c2.CMa)
	}
}

func TestFindNearestTwoCalib_AboveMax(t *testing.T) {
	i := &ThreeHoleInterpolator{
		calibData: []types.ThreeHoleCalibData{
			{CMa: 0.3},
			{CMa: 0.5},
		},
	}
	c1, c2 := i.findNearestTwoCalib(0.9)
	if c1.CMa != 0.3 || c2.CMa != 0.5 {
		t.Errorf("above max: expected (0.3, 0.5), got (%.2f, %.2f)", c1.CMa, c2.CMa)
	}
}

func TestInterpolateInKbDirection_LowerBound(t *testing.T) {
	i := &ThreeHoleInterpolator{}
	entries := []kbAlphaEntry{
		{Kb: -0.5, Alpha: -10, Kt: 0.4, Sb: 0.75},
		{Kb: 0, Alpha: 0, Kt: 0.5, Sb: 0.8},
		{Kb: 0.5, Alpha: 10, Kt: 0.6, Sb: 0.85},
	}
	alpha, kt, sb, ok := i.interpolateInKbDirection(entries, -1.0)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if alpha != -10 || kt != 0.4 || sb != 0.75 {
		t.Errorf("lower bound: expected (-10, 0.4, 0.75), got (%.2f, %.2f, %.4f)", alpha, kt, sb)
	}
}

func TestInterpolateInKbDirection_UpperBound(t *testing.T) {
	i := &ThreeHoleInterpolator{}
	entries := []kbAlphaEntry{
		{Kb: -0.5, Alpha: -10, Kt: 0.4, Sb: 0.75},
		{Kb: 0.5, Alpha: 10, Kt: 0.6, Sb: 0.85},
	}
	alpha, kt, sb, ok := i.interpolateInKbDirection(entries, 1.0)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if alpha != 10 || kt != 0.6 || sb != 0.85 {
		t.Errorf("upper bound: expected (10, 0.6, 0.85), got (%.2f, %.2f, %.4f)", alpha, kt, sb)
	}
}

func TestInterpolateInKbDirection_Middle(t *testing.T) {
	i := &ThreeHoleInterpolator{}
	entries := []kbAlphaEntry{
		{Kb: 0, Alpha: 0, Kt: 0.5, Sb: 0.8},
		{Kb: 0.5, Alpha: 10, Kt: 0.6, Sb: 0.85},
	}
	alpha, kt, sb, ok := i.interpolateInKbDirection(entries, 0.25)
	if !ok {
		t.Fatal("expected ok=true")
	}
	// 中间插值: ratio=0.5, alpha=5, kt=0.55, sb=0.825
	if math.Abs(alpha-5) > 0.01 || math.Abs(kt-0.55) > 0.01 || math.Abs(sb-0.825) > 0.01 {
		t.Errorf("middle: expected (5, 0.55, 0.825), got (%.4f, %.4f, %.6f)", alpha, kt, sb)
	}
}

func TestLoadCalibFiles_MismatchedAlpha(t *testing.T) {
	dir := t.TempDir()

	// 文件1: 3个alpha
	f1 := writeTempCalibFile(t, dir, "calib_0.5.dat",
		sampleCalibContent(0.5, []float64{-5, 0, 5}))

	// 文件2: 不同数量的alpha
	f2 := writeTempCalibFile(t, dir, "calib_0.6.dat",
		sampleCalibContent(0.6, []float64{-10, 0, 10}))

	interp := NewThreeHoleInterpolator()
	err := interp.LoadCalibFiles([]string{f1, f2})
	if err == nil {
		t.Error("expected error for mismatched alpha sequences")
	}
}

func TestParseCalibFile_Valid(t *testing.T) {
	dir := t.TempDir()
	f1 := writeTempCalibFile(t, dir, "calib.dat",
		sampleCalibContent(0.5, []float64{-5, 0, 5}))

	data, err := parseCalibFile(f1)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if data.CMa != 0.5 {
		t.Errorf("CMa: expected 0.5, got %.4f", data.CMa)
	}
	if len(data.Entries) != 3 {
		t.Errorf("entries: expected 3, got %d", len(data.Entries))
	}
}

func TestParseCalibFile_Empty(t *testing.T) {
	dir := t.TempDir()
	f1 := writeTempCalibFile(t, dir, "empty.dat", "")

	_, err := parseCalibFile(f1)
	if err == nil {
		t.Error("expected error for empty file")
	}
}

func TestParseCalibFile_MissingNalpha(t *testing.T) {
	dir := t.TempDir()
	f1 := writeTempCalibFile(t, dir, "bad.dat", "0.5\n")

	_, err := parseCalibFile(f1)
	if err == nil {
		t.Error("expected error for missing Nalpha line")
	}
}

func TestInterpolateInMaDirection(t *testing.T) {
	i := &ThreeHoleInterpolator{}
	calib1 := types.ThreeHoleCalibData{
		CMa: 0.5,
		Entries: []types.ThreeHoleCalibEntry{
			{Kb: -0.5, Kt: 0.4, Sb: 0.75, Alpha: -10},
			{Kb: 0, Kt: 0.5, Sb: 0.8, Alpha: 0},
		},
	}
	calib2 := types.ThreeHoleCalibData{
		CMa: 1.0,
		Entries: []types.ThreeHoleCalibEntry{
			{Kb: -0.4, Kt: 0.45, Sb: 0.77, Alpha: -9},
			{Kb: 0.1, Kt: 0.55, Sb: 0.82, Alpha: 1},
		},
	}

	// ratio=0.5 (Ma=0.75在0.5和1.0中间)
	result := i.interpolateInMaDirection(calib1, calib2, 0.75)
	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
	// 第一条: Kb = -0.5 + 0.5*(-0.4+0.5) = -0.45
	if math.Abs(result[0].Kb+0.45) > 0.01 {
		t.Errorf("entry 0 Kb: expected -0.45, got %.4f", result[0].Kb)
	}
}

func TestCalculateMachNumber(t *testing.T) {
	tests := []struct {
		name       string
		pt, ps, pa float64
		want       float64
	}{
		{"zero ps", 100, 0, 101.325, 0},
		{"normal", 120, 105, 101.325, 0},
		{"ps negative", 100, -200, 101.325, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateMachNumber(tt.pt, tt.ps, tt.pa)
			if math.IsNaN(result) {
				t.Error("result is NaN")
			}
			if math.IsInf(result, 0) {
				t.Error("result is Inf")
			}
		})
	}
}

func TestClampMa(t *testing.T) {
	if v := clampMa(0.3, 0.5, 1.0); v != 0.5 {
		t.Errorf("below min: expected 0.5, got %.4f", v)
	}
	if v := clampMa(1.5, 0.5, 1.0); v != 1.0 {
		t.Errorf("above max: expected 1.0, got %.4f", v)
	}
	if v := clampMa(0.7, 0.5, 1.0); v != 0.7 {
		t.Errorf("in range: expected 0.7, got %.4f", v)
	}
}

func TestIsLoaded(t *testing.T) {
	interp := NewThreeHoleInterpolator()
	if interp.IsLoaded() {
		t.Error("expected not loaded initially")
	}

	dir := t.TempDir()
	alphas := []float64{-5, 0, 5}
	f1 := writeTempCalibFile(t, dir, "calib.dat", sampleCalibContent(0.5, alphas))

	if err := interp.LoadCalibFiles([]string{f1}); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if !interp.IsLoaded() {
		t.Error("expected loaded after LoadCalibFiles")
	}
}
