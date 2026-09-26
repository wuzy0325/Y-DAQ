package types

import "testing"

func TestNormalizeSoftLimit(t *testing.T) {
	cases := []struct {
		name     string
		axis     AxisConfig
		wantMin  float64
		wantMax  float64
		wantDone bool
	}{
		{"linear 0/0 -> defaults", AxisConfig{Kind: AxisKindLinear}, -100, 100, true},
		{"rotary 0/0 -> defaults", AxisConfig{Kind: AxisKindRotary}, -360, 360, true},
		{"existing range preserved", AxisConfig{Kind: AxisKindLinear, SoftLimit: SoftLimitConfig{Min: -5, Max: 50}}, -5, 50, false},
		{"zero min with max preserved", AxisConfig{Kind: AxisKindLinear, SoftLimit: SoftLimitConfig{Min: 0, Max: 50}}, 0, 50, false},
		{"negative min with zero max preserved", AxisConfig{Kind: AxisKindRotary, SoftLimit: SoftLimitConfig{Min: -5, Max: 0}}, -5, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.axis.NormalizeSoftLimit()
			if got != tc.wantDone {
				t.Fatalf("NormalizeSoftLimit() = %v, want %v", got, tc.wantDone)
			}
			if tc.axis.SoftLimit.Min != tc.wantMin || tc.axis.SoftLimit.Max != tc.wantMax {
				t.Fatalf("range = [%v, %v], want [%v, %v]", tc.axis.SoftLimit.Min, tc.axis.SoftLimit.Max, tc.wantMin, tc.wantMax)
			}
		})
	}
}
