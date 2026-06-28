package three_hole

import (
	"math"
	"testing"

	"yx-daq/internal/types"
)

func TestCalculateThreeHoleAverage_Empty(t *testing.T) {
	got := calculateThreeHoleAverage(nil)
	if got != (types.ThreeHoleRawData{}) {
		t.Errorf("empty samples should return zero value, got %v", got)
	}
}

func TestCalculateThreeHoleAverage_LessThan4Samples_SimpleAverage(t *testing.T) {
	// <4 个样本走 simpleAverage 路径
	samples := []types.ThreeHoleRawData{
		{P1: 10, P2: 20, P3: 30, PAtm: 100, TAtm: 25},
		{P1: 20, P2: 30, P3: 40, PAtm: 100, TAtm: 25},
	}
	got := calculateThreeHoleAverage(samples)
	if got.P1 != 15 {
		t.Errorf("P1 = %v, want 15", got.P1)
	}
	if got.P2 != 25 {
		t.Errorf("P2 = %v, want 25", got.P2)
	}
	if got.P3 != 35 {
		t.Errorf("P3 = %v, want 35", got.P3)
	}
	if got.PAtm != 100 {
		t.Errorf("PAtm = %v, want 100", got.PAtm)
	}
	if got.TAtm != 25 {
		t.Errorf("TAtm = %v, want 25", got.TAtm)
	}
}

func TestCalculateThreeHoleAverage_4OrMoreSamples_OutlierFiltering(t *testing.T) {
	// 4+ 样本走 outlierFilteredAvg 路径
	// 5 个样本：[10, 10, 10, 10, 1000]，1000 是异常值应被剔除
	samples := []types.ThreeHoleRawData{
		{P1: 10},
		{P1: 10},
		{P1: 10},
		{P1: 10},
		{P1: 1000}, // outlier
	}
	got := calculateThreeHoleAverage(samples)
	// mean = 208, stdDev ≈ 398, lo = 208-1194 = -986, hi = 208+1194 = 1402
	// 1000 在 [-986, 1402] 范围内 → 不会被剔除
	// 所以结果就是 mean = 208
	want := (10.0 + 10.0 + 10.0 + 10.0 + 1000.0) / 5.0
	if math.Abs(got.P1-want) > 0.0001 {
		t.Errorf("P1 = %v, want %v (no outlier removal since within 3σ)", got.P1, want)
	}
}

func TestCalculateThreeHoleAverage_OutlierRemoved(t *testing.T) {
	// 构造一个 1000 远超 3σ 的样本集
	// 10 个 100 + 一个 1000000
	samples := make([]types.ThreeHoleRawData, 11)
	for i := 0; i < 10; i++ {
		samples[i] = types.ThreeHoleRawData{P1: 100}
	}
	samples[10] = types.ThreeHoleRawData{P1: 1000000}
	got := calculateThreeHoleAverage(samples)
	// mean ≈ 90909, stdDev ≈ 287480, lo ≈ -772531, hi ≈ 954349
	// 1000000 在 3σ 之外 → 剔除，剩余 10 个 100 → avg = 100
	if math.Abs(got.P1-100.0) > 0.0001 {
		t.Errorf("P1 = %v, want 100 (outlier removed)", got.P1)
	}
}

func TestSimpleAverage(t *testing.T) {
	samples := []types.ThreeHoleRawData{
		{P1: 1, P2: 2, P3: 3, PAtm: 4, TAtm: 5},
		{P1: 3, P2: 4, P3: 5, PAtm: 6, TAtm: 7},
	}
	got := simpleAverage(samples)
	if got.P1 != 2 {
		t.Errorf("P1 = %v, want 2", got.P1)
	}
	if got.P2 != 3 {
		t.Errorf("P2 = %v, want 3", got.P2)
	}
	if got.P3 != 4 {
		t.Errorf("P3 = %v, want 4", got.P3)
	}
	if got.PAtm != 5 {
		t.Errorf("PAtm = %v, want 5", got.PAtm)
	}
	if got.TAtm != 6 {
		t.Errorf("TAtm = %v, want 6", got.TAtm)
	}
}

func TestOutlierFilteredAvg_Empty(t *testing.T) {
	if got := outlierFilteredAvg(nil); got != 0 {
		t.Errorf("empty should return 0, got %v", got)
	}
}

func TestOutlierFilteredAvg_SingleValue(t *testing.T) {
	if got := outlierFilteredAvg([]float64{42.0}); got != 42.0 {
		t.Errorf("single value should return itself, got %v", got)
	}
}

func TestOutlierFilteredAvg_AllSame(t *testing.T) {
	vals := []float64{5, 5, 5, 5, 5}
	if got := outlierFilteredAvg(vals); got != 5 {
		t.Errorf("all-same should return 5, got %v", got)
	}
}

func TestOutlierFilteredAvg_OutlierRemoved(t *testing.T) {
	// 构造一个真正能被 3σ 剔除的场景：10 个相同正常值 + 1 个极端 outlier
	// mean ≈ 90909, stdDev ≈ 287480, lo ≈ -772531, hi ≈ 954349
	// 1000000 在 [lo, hi] 之外 → 剔除，剩 10 个 100 → avg = 100
	vals := []float64{100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 1000000}
	got := outlierFilteredAvg(vals)
	if math.Abs(got-100.0) > 0.0001 {
		t.Errorf("after outlier removal should be 100, got %v", got)
	}
}

func TestOutlierFilteredAvg_TwoValuesWithin3Sigma(t *testing.T) {
	// 两个不同值：mean=50, stdDev=50, lo=-100, hi=200，两值都在范围内 → cnt=2, avg=50
	vals := []float64{0, 100}
	got := outlierFilteredAvg(vals)
	if math.Abs(got-50.0) > 0.0001 {
		t.Errorf("both values within 3σ → avg = 50, got %v", got)
	}
}

func TestMapField(t *testing.T) {
	samples := []types.ThreeHoleRawData{
		{P1: 1, P2: 10},
		{P1: 2, P2: 20},
		{P1: 3, P2: 30},
	}
	p1Vals := mapField(samples, func(s types.ThreeHoleRawData) float64 { return s.P1 })
	if len(p1Vals) != 3 || p1Vals[0] != 1 || p1Vals[1] != 2 || p1Vals[2] != 3 {
		t.Errorf("mapField P1 = %v", p1Vals)
	}
	p2Vals := mapField(samples, func(s types.ThreeHoleRawData) float64 { return s.P2 })
	if len(p2Vals) != 3 || p2Vals[0] != 10 || p2Vals[1] != 20 || p2Vals[2] != 30 {
		t.Errorf("mapField P2 = %v", p2Vals)
	}
}
