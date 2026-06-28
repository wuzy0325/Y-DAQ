package interpolation

import (
	"fmt"
	"math"
	"testing"
)

// TestCalInterpolatorLoadAndCalculate 验证单文件 CAL 插值器能加载 13×13 合成数据并产出有效结果
func TestCalInterpolatorLoadAndCalculate(t *testing.T) {
	interpolator := NewCalInterpolator()
	if err := interpolator.LoadCalLines(syntheticCalLines(0.05, 0.01), "0.3Ma.cal"); err != nil {
		t.Fatalf("LoadCalLines returned error: %v", err)
	}
	if !interpolator.IsLoaded() {
		t.Fatal("expected IsLoaded true after LoadCalLines")
	}

	// 输入 Ka=0,Kb=0 → 中心网格点 (alpha=0,beta=0)
	result, err := interpolator.Calculate(InterpolationInput{
		P1: 100, P2: 300, P3: 100, P4: 100, P5: 100,
		PAtm: 101325, TAtm: 20,
	})
	if err != nil {
		t.Fatalf("Calculate returned error: %v", err)
	}
	if !result.IsValid {
		t.Fatalf("expected valid result, got warning %q", result.Warning)
	}
	if math.Abs(result.Alpha) > 1 {
		t.Fatalf("expected alpha near 0, got %v", result.Alpha)
	}
	if math.Abs(result.Beta) > 1 {
		t.Fatalf("expected beta near 0, got %v", result.Beta)
	}
	if result.MachNumber <= 0 {
		t.Fatalf("expected positive Mach, got %v", result.MachNumber)
	}
	if result.TotalPressure <= 0 {
		t.Fatalf("expected positive total pressure, got %v", result.TotalPressure)
	}
}

// TestCalInterpolatorRejectsBadHeader 验证非 13×13 表头报错
func TestCalInterpolatorRejectsBadHeader(t *testing.T) {
	interpolator := NewCalInterpolator()
	bad := append([]string{"12 12"}, syntheticCalLines(0.05, 0.01)[1:]...)
	if err := interpolator.LoadCalLines(bad, "bad.cal"); err == nil {
		t.Fatal("expected error for bad header")
	}
}

// TestCalInterpolatorRejectsMalformedRow 验证数据行列数错误报错
func TestCalInterpolatorRejectsMalformedRow(t *testing.T) {
	interpolator := NewCalInterpolator()
	lines := []string{"13 13", "0.1 0.2 1.0"}
	// 补齐剩余行凑足 169 行
	for i := 0; i < expectedGridCount-1; i++ {
		lines = append(lines, "0.000000 0.000000 0.050000 0.010000 0 0")
	}
	if err := interpolator.LoadCalLines(lines, "bad.cal"); err == nil {
		t.Fatal("expected error for malformed row")
	}
}

// TestMultiCalLinearKeepsCasSatAndMachRange 验证多 CAL 线性插值保留 CAS/SAT 并生成区间警告
func TestMultiCalLinearKeepsCasSatAndMachRange(t *testing.T) {
	low := NewCalInterpolator()
	if err := low.LoadCalLines(syntheticCalLines(0.05, 0.01), "low.cal"); err != nil {
		t.Fatalf("load low CAL: %v", err)
	}
	high := NewCalInterpolator()
	if err := high.LoadCalLines(syntheticCalLines(0.15, 0.02), "high.cal"); err != nil {
		t.Fatalf("load high CAL: %v", err)
	}
	multi := &MultiCalInterpolator{
		calFiles: []calFileWithInterpolator{
			{MachNumber: 0.2, FileInfo: CalFileInfo{ValidRange: CalValidRange{MachMin: 0.2, MachMax: 0.2}}, Interpolator: low},
			{MachNumber: 0.4, FileInfo: CalFileInfo{ValidRange: CalValidRange{MachMin: 0.4, MachMax: 0.4}}, Interpolator: high},
		},
		sortedMachNumbers: []float64{0.2, 0.4},
		loaded:            true,
		mode:              ModeLinear,
	}
	result, err := multi.calculateWithLinear(0.3, InterpolationInput{
		P1: 100, P2: 300, P3: 100, P4: 100, P5: 100,
		PAtm: 101325, TAtm: 20,
	})
	if err != nil {
		t.Fatalf("calculateWithLinear returned error: %v", err)
	}
	if result.CAS <= 0 {
		t.Fatalf("expected interpolated CAS to be preserved, got %v", result.CAS)
	}
	if result.SAT <= 0 {
		t.Fatalf("expected interpolated SAT to be preserved, got %v", result.SAT)
	}
	validRange := multi.GetValidRange()
	assertNear(t, "MachMin", validRange.MachMin, 0.2, 1e-12)
	assertNear(t, "MachMax", validRange.MachMax, 0.4, 1e-12)
}

// TestCalculateVelocityComponentsUsesProbeAxisConvention 验证速度分量坐标系约定
func TestCalculateVelocityComponentsUsesProbeAxisConvention(t *testing.T) {
	v := 100.0
	alphaDeg := 30.0
	betaDeg := 20.0
	vx, vy, vz := calculateVelocityComponents(v, alphaDeg, betaDeg)
	alpha := alphaDeg * math.Pi / 180
	beta := betaDeg * math.Pi / 180
	assertNear(t, "vx", vx, v*math.Cos(beta)*math.Sin(alpha), 1e-12)
	assertNear(t, "vy", vy, v*math.Sin(beta), 1e-12)
	assertNear(t, "vz", vz, v*math.Cos(beta)*math.Cos(alpha), 1e-12)
}

// syntheticCalLines 生成 13×13 合成 CAL 文本行（首行 "13 13"，169 行 ka kb cpt cps alpha beta）
func syntheticCalLines(cpt, cps float64) []string {
	lines := []string{"13 13"}
	for alpha := -30.0; alpha <= 30; alpha += 5 {
		for beta := -30.0; beta <= 30; beta += 5 {
			lines = append(lines, fmt.Sprintf("%.6f %.6f %.6f %.6f %.0f %.0f",
				alpha/100, beta/100, cpt, cps, alpha, beta))
		}
	}
	return lines
}

func assertNear(t *testing.T, name string, got, want, tolerance float64) {
	t.Helper()
	if math.Abs(got-want) > tolerance {
		t.Fatalf("%s = %v, want %v +/- %v", name, got, want, tolerance)
	}
}
