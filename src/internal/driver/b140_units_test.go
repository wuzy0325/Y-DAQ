package driver

import (
	"math"
	"testing"

	"yx-daq/internal/types"
)

// newB140WithAxes 构造一个仅含 axes 配置的 B140MotionController（driver=nil）
// 用于测试纯计算方法 engineeringToPulse / pulseToEngineering / etc.
func newB140WithAxes(axes []types.AxisConfig) *B140MotionController {
	return &B140MotionController{
		driver: nil,
		axes:   axes,
	}
}

func TestB140_EngineeringToPulse_Linear(t *testing.T) {
	// 默认 linear 轴: StepAngleDeg=1.8, MicroSteps=16, Lead=5
	// stepsPerRev = 360/1.8 = 200
	// pulsesPerRev = 200 * 16 = 3200
	// pulsesPerUnit = 3200/5 = 640
	// 1 unit => 640 pulses
	axes := []types.AxisConfig{
		{Name: types.AxisX, Kind: types.AxisKindLinear, StepAngleDeg: 1.8, MicroSteps: 16, Lead: 5},
	}
	c := newB140WithAxes(axes)

	got := c.engineeringToPulse(types.AxisX, 1.0)
	if math.Abs(got-640.0) > 0.0001 {
		t.Errorf("linear 1.0 unit => %v pulses, want 640", got)
	}
	got = c.engineeringToPulse(types.AxisX, 2.5)
	if math.Abs(got-1600.0) > 0.0001 {
		t.Errorf("linear 2.5 unit => %v pulses, want 1600", got)
	}
}

func TestB140_EngineeringToPulse_Rotary(t *testing.T) {
	// 默认 rotary 轴: StepAngleDeg=1.8, MicroSteps=16, GearRatio=2
	// stepsPerRev = 200, pulsesPerRev = 3200
	// pulsesPerUnit = (3200 * 2) / 360 = 17.7778
	axes := []types.AxisConfig{
		{Name: types.AxisU, Kind: types.AxisKindRotary, StepAngleDeg: 1.8, MicroSteps: 16, GearRatio: 2},
	}
	c := newB140WithAxes(axes)

	got := c.engineeringToPulse(types.AxisU, 180.0)
	want := 3200.0 * 2.0 / 360.0 * 180.0
	if math.Abs(got-want) > 0.0001 {
		t.Errorf("rotary 180 => %v, want %v", got, want)
	}
}

func TestB140_EngineeringToPulse_Defaults(t *testing.T) {
	// StepAngleDeg=0 应回退到 1.8；Lead=0 应回退到 1；GearRatio<=0 应回退到 1
	axes := []types.AxisConfig{
		{Name: types.AxisX, Kind: types.AxisKindLinear, StepAngleDeg: 0, MicroSteps: 16, Lead: 0},
	}
	c := newB140WithAxes(axes)
	// stepsPerRev = 360/1.8 = 200, pulsesPerRev = 3200, pulsesPerUnit = 3200/1 = 3200
	got := c.engineeringToPulse(types.AxisX, 1.0)
	if math.Abs(got-3200.0) > 0.0001 {
		t.Errorf("linear default 1.0 => %v, want 3200", got)
	}
}

func TestB140_EngineeringToPulse_AxisNotFound(t *testing.T) {
	c := newB140WithAxes(nil)
	// 未找到轴时应原样返回 position
	got := c.engineeringToPulse(types.AxisX, 42.0)
	if got != 42.0 {
		t.Errorf("unknown axis should pass-through, got %v, want 42", got)
	}
}

func TestB140_PulseToEngineering_Linear(t *testing.T) {
	axes := []types.AxisConfig{
		{Name: types.AxisX, Kind: types.AxisKindLinear, StepAngleDeg: 1.8, MicroSteps: 16, Lead: 5},
	}
	c := newB140WithAxes(axes)
	// 640 pulses => 1 unit
	got := c.pulseToEngineering(types.AxisX, 640.0)
	if math.Abs(got-1.0) > 0.0001 {
		t.Errorf("640 pulses => %v units, want 1.0", got)
	}
}

func TestB140_PulseToEngineering_AxisNotFound(t *testing.T) {
	c := newB140WithAxes(nil)
	got := c.pulseToEngineering(types.AxisX, 100.0)
	if got != 100.0 {
		t.Errorf("unknown axis should pass-through, got %v, want 100", got)
	}
}

func TestB140_PulseToEngineering_RoundTrip(t *testing.T) {
	axes := []types.AxisConfig{
		{Name: types.AxisX, Kind: types.AxisKindLinear, StepAngleDeg: 1.8, MicroSteps: 16, Lead: 5},
		{Name: types.AxisU, Kind: types.AxisKindRotary, StepAngleDeg: 1.8, MicroSteps: 16, GearRatio: 2},
	}
	c := newB140WithAxes(axes)

	for _, v := range []float64{0.0, 1.0, 3.14, -2.5, 100.0} {
		pulses := c.engineeringToPulse(types.AxisX, v)
		back := c.pulseToEngineering(types.AxisX, pulses)
		if math.Abs(back-v) > 0.0001 {
			t.Errorf("linear round-trip %v => %v => %v", v, pulses, back)
		}
	}
	for _, v := range []float64{0.0, 45.0, 90.0, -30.0} {
		pulses := c.engineeringToPulse(types.AxisU, v)
		back := c.pulseToEngineering(types.AxisU, pulses)
		if math.Abs(back-v) > 0.0001 {
			t.Errorf("rotary round-trip %v => %v => %v", v, pulses, back)
		}
	}
}

func TestB140_EngineeringToEncoderCount(t *testing.T) {
	axes := []types.AxisConfig{
		{Name: types.AxisX, EncoderScale: 0.005},
	}
	c := newB140WithAxes(axes)
	// scale=0.005 => count = position/scale = 1.0/0.005 = 200
	got := c.engineeringToEncoderCount(types.AxisX, 1.0)
	if math.Abs(got-200.0) > 0.0001 {
		t.Errorf("1.0 / 0.005 = %v, want 200", got)
	}
}

func TestB140_EngineeringToEncoderCount_DefaultScale(t *testing.T) {
	// EncoderScale=0 应回退到 DefaultEncoderScale=0.005
	axes := []types.AxisConfig{
		{Name: types.AxisX, EncoderScale: 0},
	}
	c := newB140WithAxes(axes)
	got := c.engineeringToEncoderCount(types.AxisX, 1.0)
	if math.Abs(got-200.0) > 0.0001 {
		t.Errorf("default scale 1.0/0.005 = %v, want 200", got)
	}
}

func TestB140_EncoderCountToEngineering(t *testing.T) {
	axes := []types.AxisConfig{
		{Name: types.AxisX, EncoderScale: 0.005},
	}
	c := newB140WithAxes(axes)
	// 200 counts * 0.005 = 1.0
	got := c.encoderCountToEngineering(types.AxisX, 200.0)
	if math.Abs(got-1.0) > 0.0001 {
		t.Errorf("200 * 0.005 = %v, want 1.0", got)
	}
}

func TestB140_EncoderRoundTrip(t *testing.T) {
	axes := []types.AxisConfig{
		{Name: types.AxisX, EncoderScale: 0.005},
	}
	c := newB140WithAxes(axes)
	for _, v := range []float64{0.0, 1.0, 3.14, -2.5} {
		count := c.engineeringToEncoderCount(types.AxisX, v)
		back := c.encoderCountToEngineering(types.AxisX, count)
		if math.Abs(back-v) > 0.0001 {
			t.Errorf("encoder round-trip %v => %v => %v", v, count, back)
		}
	}
}

func TestB140_Encoder_AxisNotFound(t *testing.T) {
	c := newB140WithAxes(nil)
	if got := c.engineeringToEncoderCount(types.AxisX, 42.0); got != 42.0 {
		t.Errorf("unknown axis pass-through, got %v", got)
	}
	if got := c.encoderCountToEngineering(types.AxisX, 42.0); got != 42.0 {
		t.Errorf("unknown axis pass-through, got %v", got)
	}
}

func TestParseMGBool(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		// 0.xxxx → limit IS triggered (true)
		{"0.0000:", true},
		{"0.1234", true},
		{"  0.4000  ", true},
		// 1.xxxx → limit NOT triggered (false)
		{"1.0000:", false},
		{"1.5000", false},
		// 边界：0.5 判定为 false（< 0.5 才 true）
		{"0.5000", false},
		// 非法/空
		{"", false},
		{":", false},
		{"abc", false},
		{":1.0000:", false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got := parseMGBool(tc.in)
			if got != tc.want {
				t.Errorf("parseMGBool(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
