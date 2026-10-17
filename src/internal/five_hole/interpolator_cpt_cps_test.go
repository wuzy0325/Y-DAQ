package five_hole

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"yx-daq/internal/types"
)

// angleTol 合成校准表把 ka/kb 写成 6 位小数，反解角度带 ~1e-5 量化误差，
// 因此角度断言用 1e-3 容差；Pt/Ps 断言仍用 floatEq(1e-6)。
const angleTol = 1e-3

// TestFiveHoleInterpolator_AppliesCptCpsForNegativeSmallAngles 回归用例。
//
// 历史缺陷：α 或 β 落在 (-5°, 0°) 时，输出值插值的网格键被格式化成 "-0"
// （负零），与校准表里的 "0" 键不匹配 → 查表失败 → cpt/cps 静默取 0，
// 于是 Pt 退化成中心孔原压 P2、Ps 退化成四孔算术平均，整个校准修正丢失，
// 且结果仍然 Valid=true，不报任何告警。
//
// 本用例锁定“CPT/CPS 必须真正生效”，覆盖 (-5°,0°) 区间内外。
func TestFiveHoleInterpolator_AppliesCptCpsForNegativeSmallAngles(t *testing.T) {
	tmpDir := t.TempDir()
	prbPath := filepath.Join(tmpDir, "Ma0.7.prb")
	if err := os.WriteFile(prbPath, []byte(syntheticCalLines()), 0644); err != nil {
		t.Fatalf("write test prb failed: %v", err)
	}

	i := NewFiveHoleInterpolator()
	if err := i.LoadCalibFiles([]string{prbPath}); err != nil {
		t.Fatalf("load prb failed: %v", err)
	}

	// syntheticCalLinesBody 把每个网格点写成
	//   ka = alpha/30, kb = beta/30, cpt = 0.5, cps = 0.3
	// 因此令 (Ka,Kb) = (alpha/30, beta/30) 即可把点打进指定角度所在单元。
	const (
		wantCPT = 0.5
		wantCPS = 0.3
		base    = 100.0 // P1 = P4 基准
		delta   = 48.0  // delta = P2 - 四孔均值
	)

	cases := []struct {
		name  string
		alpha float64
		beta  float64
	}{
		{"αβ 均在 (-5,0) 中部", -2, -2},
		{"αβ 贴近 -5° 网格线", -4.5, -4.5},
		{"αβ 贴近 0° 网格线", -0.5, -0.5},
		{"αβ 均为小而负且不对称", -3, -1},
		{"对照：正角度区间", 2, 2},
		{"对照：仅 α 为负", -2, 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p1, p4 := base, base
			p3 := base + (tc.beta/30)*delta  // Kb = (P3-P1)/delta = beta/30
			p5 := base - (tc.alpha/30)*delta // Ka = (P4-P5)/delta = alpha/30
			avg := (p1 + p3 + p4 + p5) * 0.25
			p2 := avg + delta

			res := i.Calculate(types.FiveHoleRawData{
				P1: p1, P2: p2, P3: p3, P4: p4, P5: p5,
				PAtm: 101325, TAtm: 20,
			})
			if !res.Valid {
				t.Fatalf("结果无效: %s", res.ErrorMsg)
			}

			if math.Abs(res.AlphaProbe-tc.alpha) > angleTol || math.Abs(res.BetaProbe-tc.beta) > angleTol {
				t.Fatalf("角度反解错误: 期望 (%.4f, %.4f)，实际 (%.4f, %.4f)",
					tc.alpha, tc.beta, res.AlphaProbe, res.BetaProbe)
			}

			// 核心断言：CPT/CPS 必须真的参与计算
			wantPt := p2 - wantCPT*delta
			wantPs := avg - wantCPS*delta
			if !floatEq(res.PtProbe, wantPt) {
				t.Fatalf("总压未按 CPT 修正: 期望 %.6f (= P2 - %.1f*delta)，实际 %.6f (P2=%.6f)",
					wantPt, wantCPT, res.PtProbe, p2)
			}
			if !floatEq(res.PsProbe, wantPs) {
				t.Fatalf("静压未按 CPS 修正: 期望 %.6f (= 四孔均值 - %.1f*delta)，实际 %.6f (均值=%.6f)",
					wantPs, wantCPS, res.PsProbe, avg)
			}

			// 显式防止退化成“未修正”形态
			if floatEq(res.PtProbe, p2) {
				t.Fatalf("总压退化为中心孔原压 P2=%.6f，说明 CPT 被当成 0（网格键查表失败）", p2)
			}
			if floatEq(res.PsProbe, avg) {
				t.Fatalf("静压退化为四孔均值 %.6f，说明 CPS 被当成 0（网格键查表失败）", avg)
			}
		})
	}
}
