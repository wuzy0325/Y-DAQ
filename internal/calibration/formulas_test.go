package calibration

import (
	"math"
	"testing"

	"yx-daq/internal/types"
)

func TestCalculateFiveHoleCoefficients(t *testing.T) {
	tests := []struct {
		name     string
		data     types.FiveHoleRawData
		expected types.FiveHoleCoefficients
	}{
		{
			name: "对称数据_Kalpha和Kbeta应为零",
			data: types.FiveHoleRawData{
				P1: 110, P2: 100, P3: 100, P4: 100, P5: 100,
				PAtm: 101.325, TAtm: 20,
			},
			expected: types.FiveHoleCoefficients{Kalpha: 0, Kbeta: 0},
		},
		{
			name: "P2>P3_正Kalpha",
			data: types.FiveHoleRawData{
				P1: 110, P2: 105, P3: 95, P4: 100, P5: 100,
				PAtm: 101.325, TAtm: 20,
			},
			expected: types.FiveHoleCoefficients{Kalpha: 1.0, Kbeta: 0},
		},
		{
			name: "P4>P5_正Kbeta",
			data: types.FiveHoleRawData{
				P1: 110, P2: 100, P3: 100, P4: 105, P5: 95,
				PAtm: 101.325, TAtm: 20,
			},
			expected: types.FiveHoleCoefficients{Kalpha: 0, Kbeta: 1.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateFiveHoleCoefficients(tt.data)

			if math.Abs(result.Kalpha-tt.expected.Kalpha) > 1e-6 {
				t.Errorf("Kalpha = %v, want %v", result.Kalpha, tt.expected.Kalpha)
			}
			if math.Abs(result.Kbeta-tt.expected.Kbeta) > 1e-6 {
				t.Errorf("Kbeta = %v, want %v", result.Kbeta, tt.expected.Kbeta)
			}
		})
	}
}

func TestCalculateFiveHoleCoefficients_WithPTotal(t *testing.T) {
	pTotal := 120.0
	data := types.FiveHoleRawData{
		P1: 110, P2: 100, P3: 100, P4: 100, P5: 100,
		PAtm: 101.325, TAtm: 20, PTotal: &pTotal,
	}

	result := CalculateFiveHoleCoefficients(data)

	// CPT = (P1 - PAtm) / (PTotal - PAtm) = (110 - 101.325) / (120 - 101.325)
	expectedCPT := (110.0 - 101.325) / (120.0 - 101.325)
	if math.Abs(result.CPT-expectedCPT) > 1e-6 {
		t.Errorf("CPT = %v, want %v", result.CPT, expectedCPT)
	}
}

func TestCalculateFiveHoleCoefficients_DivisionByZero(t *testing.T) {
	// P1 == Pavg, 分母为零
	data := types.FiveHoleRawData{
		P1: 100, P2: 100, P3: 100, P4: 100, P5: 100,
		PAtm: 101.325, TAtm: 20,
	}

	result := CalculateFiveHoleCoefficients(data)

	// 不应产生 NaN 或 Inf
	if math.IsNaN(result.Kalpha) || math.IsInf(result.Kalpha, 0) {
		t.Errorf("Kalpha should not be NaN/Inf, got %v", result.Kalpha)
	}
	if math.IsNaN(result.Kbeta) || math.IsInf(result.Kbeta, 0) {
		t.Errorf("Kbeta should not be NaN/Inf, got %v", result.Kbeta)
	}
}

func TestCalculateAverage(t *testing.T) {
	samples := []types.FiveHoleRawData{
		{P1: 100, P2: 101, P3: 102, P4: 103, P5: 104, PAtm: 101.3, TAtm: 20},
		{P1: 110, P2: 111, P3: 112, P4: 113, P5: 114, PAtm: 101.4, TAtm: 21},
	}

	result := CalculateAverage(samples)

	if math.Abs(result.P1-105) > 1e-6 {
		t.Errorf("P1 average = %v, want 105", result.P1)
	}
	if math.Abs(result.P2-106) > 1e-6 {
		t.Errorf("P2 average = %v, want 106", result.P2)
	}
	if math.Abs(result.PAtm-101.35) > 1e-6 {
		t.Errorf("PAtm average = %v, want 101.35", result.PAtm)
	}
}

func TestCalculateStdDev(t *testing.T) {
	values := []float64{100, 102, 98, 101, 99}

	result := CalculateStdDev(values)

	// 标准差应大于0
	if result <= 0 {
		t.Errorf("StdDev should be positive, got %v", result)
	}
}

func TestCalculateStdDev_Empty(t *testing.T) {
	result := CalculateStdDev([]float64{})
	if result != 0 {
		t.Errorf("StdDev of empty slice should be 0, got %v", result)
	}
}

func TestCalculateStdDev_SingleValue(t *testing.T) {
	result := CalculateStdDev([]float64{42})
	if result != 0 {
		t.Errorf("StdDev of single value should be 0, got %v", result)
	}
}

func TestMatchChannelByRole(t *testing.T) {
	tests := []struct {
		role    types.ProbeChannelRole
		name    string
		matches bool
	}{
		{types.RoleP1, "P1", true},
		{types.RoleP1, "孔1压力", true},
		{types.RoleP1, "CH1", false},
		{types.RoleP2, "P2", true},
		{types.RolePAtm, "大气压力", true},
		{types.RoleTAtm, "大气温度", true},
		{types.RolePTotal, "总压", true},
	}

	for _, tt := range tests {
		result := MatchChannelByRole(tt.role, tt.name)
		if result != tt.matches {
			t.Errorf("MatchChannelByRole(%v, %v) = %v, want %v", tt.role, tt.name, result, tt.matches)
		}
	}
}
