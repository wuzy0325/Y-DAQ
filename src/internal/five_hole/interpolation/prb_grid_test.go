package interpolation

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

// buildSyntheticPrbLines 按给定 α/β 网格线合成 PRB 文本行。
//
// 系数 Kα=α/30、Kβ=β/30、CPT=-0.5、CPS=0.1——线性可逆，
// 保证逆双线性定位能精确还原角度，测试可做数值断言。
func buildSyntheticPrbLines(alphas, betas []float64) []string {
	lines := make([]string, 0, len(alphas)*len(betas)+1)
	lines = append(lines, fmt.Sprintf("%d %d", len(alphas), len(betas)))
	for _, alpha := range alphas {
		for _, beta := range betas {
			lines = append(lines, fmt.Sprintf("%.6f %.6f %.6f %.6f %s %s",
				alpha/30, beta/30, -0.5, 0.1,
				formatGridAngle(alpha), formatGridAngle(beta)))
		}
	}
	return lines
}

// formatGridAngle 角度输出为最短文本（整数不带小数点，支持 2.5 等小数布点）。
func formatGridAngle(v float64) string {
	return strings.TrimSuffix(fmt.Sprintf("%.4f", v), ".0000")
}

// TestPrbLoadArbitraryGrid 验证非 13×13 / 非 ±30° 网格可加载并正确插值：
// α 5 线（-10..10 步 5）× β 3 线（-15,0,15），共 15 点、非方形。
func TestPrbLoadArbitraryGrid(t *testing.T) {
	alphas := []float64{-10, -5, 0, 5, 10}
	betas := []float64{-15, 0, 15}

	interpolator := NewPrbInterpolator()
	if err := interpolator.LoadPrbLines(buildSyntheticPrbLines(alphas, betas), "arbitrary.prb"); err != nil {
		t.Fatalf("加载非标准网格失败: %v", err)
	}

	vr := interpolator.GetValidRange()
	if vr.AlphaMin != -10 || vr.AlphaMax != 10 || vr.BetaMin != -15 || vr.BetaMax != 15 {
		t.Fatalf("有效范围 = [%v,%v]×[%v,%v]，期望 [-10,10]×[-15,15]",
			vr.AlphaMin, vr.AlphaMax, vr.BetaMin, vr.BetaMax)
	}

	// 构造 (α=2.5, β=7.5) 的孔压（系数定义见 buildSyntheticPrbLines），
	// 断言逆解角度精确还原——同时覆盖非方形网格的单元定位与输出插值路径。
	// 孔压全整数以消除浮点构造噪声：avg=100、Δ=48，Kβ=0.25→P3-P1=12，
	// Kα=1/12→P4-P5=4。
	res, err := interpolator.Calculate(InterpolationInput{
		P1: 94, P2: 148, P3: 106, P4: 102, P5: 98,
		PAtm: 101325, TAtm: 20,
	})
	if err != nil {
		t.Fatalf("插值计算失败: %v", err)
	}
	// 容差 1e-4：PRB 格式系数仅存 6 位小数（%.6f），5/30→0.166667 的
	// 截断使逆解角度带有 ~1e-5 量级的固有误差（真实标定表同此精度）。
	if math.Abs(res.Alpha-2.5) > 1e-4 || math.Abs(res.Beta-7.5) > 1e-4 {
		t.Fatalf("逆解角度 = (α=%v, β=%v)，期望 (2.5, 7.5)", res.Alpha, res.Beta)
	}
}

// TestPrbLoadNonUniformGrid 验证非等差网格线（中心密、边缘疏）可加载——
// 网格约束是"完整矩形"而非"等差步长"。
func TestPrbLoadNonUniformGrid(t *testing.T) {
	alphas := []float64{-20, -10, -5, 0, 5, 10, 20}
	betas := []float64{-20, -10, 0, 10, 20}

	interpolator := NewPrbInterpolator()
	if err := interpolator.LoadPrbLines(buildSyntheticPrbLines(alphas, betas), "nonuniform.prb"); err != nil {
		t.Fatalf("加载非等差网格失败: %v", err)
	}
	if !interpolator.IsLoaded() {
		t.Fatal("插值器未进入已加载状态")
	}
}

// TestPrbLoadFractionalStepGrid 验证小数步长（2.5°）网格：角度列以
// "12.5" 这类小数文本出现，gridPointKey 必须全精度区分（历史 %.0f
// 会把 12.5 和 13 舍入到同一键）。
func TestPrbLoadFractionalStepGrid(t *testing.T) {
	alphas := []float64{-12.5, -10, -7.5, -5, -2.5, 0, 2.5, 5, 7.5, 10, 12.5}
	betas := []float64{-5, -2.5, 0, 2.5, 5}

	interpolator := NewPrbInterpolator()
	if err := interpolator.LoadPrbLines(buildSyntheticPrbLines(alphas, betas), "fractional.prb"); err != nil {
		t.Fatalf("加载小数步长网格失败: %v", err)
	}

	// 网格中心 (α=2.5, β=2.5)：Kα=Kβ=2.5/30=1/12。
	// 全整数孔压：avg=100、Δ=48 → P3-P1=4、P4-P5=4。
	res, err := interpolator.Calculate(InterpolationInput{
		P1: 98, P2: 148, P3: 102, P4: 102, P5: 98,
		PAtm: 101325, TAtm: 20,
	})
	if err != nil {
		t.Fatalf("插值计算失败: %v", err)
	}
	if math.Abs(res.Alpha-2.5) > 1e-6 || math.Abs(res.Beta-2.5) > 1e-6 {
		t.Fatalf("逆解角度 = (α=%v, β=%v)，期望 (2.5, 2.5)", res.Alpha, res.Beta)
	}
}

// TestPrbRejectsInconsistentGrid 验证表头声明与数据不符、维度不足、
// 矩形缺角的文件被拒绝——放宽网格不等于放弃完整性校验。
func TestPrbRejectsInconsistentGrid(t *testing.T) {
	alphas := []float64{-10, -5, 0, 5, 10}
	betas := []float64{-15, 0, 15}
	full := buildSyntheticPrbLines(alphas, betas)

	// 表头声明 5 3，实际抽掉一行（14 行）→ 矩形缺角。
	dropped := append([]string{full[0]}, full[1:len(full)-1]...)

	// 表头声明 4 3，实际 15 行 → 声明与数据不符。
	misdeclared := append([]string{"4 3"}, full[1:]...)

	// 单线维度（1 5）→ 无法构成单元。
	single := []string{"1 5", "-10 -10 -10 -10 -10 -10", "10 10 10 10 10 10"}

	cases := []struct {
		name   string
		lines  []string
		wantEr string
	}{
		{"矩形缺角", dropped, "行数"},
		{"声明不符", misdeclared, "实际 15 行"},
		{"维度不足", single, "≥2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			interpolator := NewPrbInterpolator()
			err := interpolator.LoadPrbLines(tc.lines, "bad.prb")
			if err == nil {
				t.Fatal("期望加载失败，实际成功")
			}
			if !strings.Contains(err.Error(), tc.wantEr) {
				t.Fatalf("错误信息 %q 未包含 %q", err.Error(), tc.wantEr)
			}
		})
	}
}

// TestResolveOutputCellAxis 单元定位的边界等价性：新实现（数据网格线）
// 在历史标准网格上与旧实现（固定步长 ±30）行为一致。
func TestResolveOutputCellAxis(t *testing.T) {
	lines := make([]float64, 13)
	for i := range lines {
		lines[i] = -30 + 5*float64(i)
	}
	cases := []struct {
		angle  float64
		wantLo float64
		wantHi float64
	}{
		{-30, -30, -25}, // 下边界
		{-28, -30, -25}, // 负数区间
		{0, 0, 5},       // 零点
		{5, 5, 10},      // 恰在线上
		{12.3, 10, 15},  // 一般点
		{29.9, 25, 30},  // 上边界内
		{30, 25, 30},    // 上边界
	}
	for _, tc := range cases {
		lo, hi := resolveOutputCellAxis(lines, tc.angle)
		if lo != tc.wantLo || hi != tc.wantHi {
			t.Fatalf("angle=%v: got (%v,%v), want (%v,%v)", tc.angle, lo, hi, tc.wantLo, tc.wantHi)
		}
	}
}
