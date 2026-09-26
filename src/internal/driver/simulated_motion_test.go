package driver

import (
	"testing"

	"yx-daq/internal/types"
)

func simulatedAxisWithSoftLimit(min, max float64) types.AxisConfig {
	return types.AxisConfig{
		Name: types.AxisX, Enabled: true, Kind: types.AxisKindLinear,
		SoftLimit: types.SoftLimitConfig{Enabled: true, Min: min, Max: max},
	}
}

func TestSimulatedSoftLimit_MoveTo(t *testing.T) {
	c := NewSimulatedMotionController([]types.AxisConfig{simulatedAxisWithSoftLimit(-10, 10)})

	if err := c.MoveTo(types.AxisX, 20); err == nil {
		t.Fatal("MoveTo beyond max should be rejected")
	}
	if err := c.MoveTo(types.AxisX, 10); err != nil {
		t.Fatalf("MoveTo at max should be allowed, got %v", err)
	}
	if err := c.WaitForMotionComplete(types.AxisX, 3000); err != nil {
		t.Fatalf("wait failed: %v", err)
	}
	st, _ := c.GetAxisStatus(types.AxisX)
	if st.Position != 10 {
		t.Fatalf("position = %v, want 10", st.Position)
	}
}

func TestSimulatedSoftLimit_MoveBy(t *testing.T) {
	c := NewSimulatedMotionController([]types.AxisConfig{simulatedAxisWithSoftLimit(-10, 10)})

	if err := c.MoveBy(types.AxisX, 11); err == nil {
		t.Fatal("MoveBy beyond max should be rejected")
	}
	if err := c.MoveBy(types.AxisX, 10); err != nil {
		t.Fatalf("MoveBy to max should be allowed, got %v", err)
	}
	if err := c.MoveBy(types.AxisX, 1); err == nil {
		t.Fatal("MoveBy from max should be rejected")
	}
	if err := c.MoveBy(types.AxisX, -20); err != nil {
		t.Fatalf("MoveBy within range should be allowed, got %v", err)
	}
	st, _ := c.GetAxisStatus(types.AxisX)
	if st.Position != -10 {
		t.Fatalf("position = %v, want -10", st.Position)
	}
}

func TestSimulatedSoftLimit_Jog(t *testing.T) {
	c := NewSimulatedMotionController([]types.AxisConfig{simulatedAxisWithSoftLimit(-10, 10)})

	if err := c.Jog(types.AxisX, 1, 11, 0); err == nil {
		t.Fatal("Jog beyond max should be rejected")
	}
	if err := c.Jog(types.AxisX, 1, 5, 0); err != nil {
		t.Fatalf("Jog within range should be allowed, got %v", err)
	}
	if err := c.WaitForMotionComplete(types.AxisX, 3000); err != nil {
		t.Fatalf("wait failed: %v", err)
	}
	if err := c.Jog(types.AxisX, -1, 20, 0); err == nil {
		t.Fatal("Jog beyond min should be rejected")
	}
	if err := c.Jog(types.AxisX, -1, 15, 0); err != nil {
		t.Fatalf("Jog to min should be allowed, got %v", err)
	}
}

func TestSimulatedSoftLimit_DisabledPasses(t *testing.T) {
	ax := simulatedAxisWithSoftLimit(-10, 10)
	ax.SoftLimit.Enabled = false
	c := NewSimulatedMotionController([]types.AxisConfig{ax})

	if err := c.MoveTo(types.AxisX, 1000); err != nil {
		t.Fatalf("disabled soft limit should pass, got %v", err)
	}
}

func TestSimulatedSoftLimit_InvalidRange(t *testing.T) {
	c := NewSimulatedMotionController([]types.AxisConfig{simulatedAxisWithSoftLimit(10, 10)})
	if err := c.MoveTo(types.AxisX, 0); err == nil {
		t.Fatal("max <= min should be rejected")
	}
}

func TestSimulatedSoftLimit_ChecksTargetAxisNotFirst(t *testing.T) {
	axX := types.AxisConfig{Name: types.AxisX, Enabled: true, Kind: types.AxisKindLinear}
	axY := types.AxisConfig{
		Name: types.AxisY, Enabled: true, Kind: types.AxisKindLinear,
		SoftLimit: types.SoftLimitConfig{Enabled: true, Min: -10, Max: 10},
	}
	c := NewSimulatedMotionController([]types.AxisConfig{axX, axY})

	if err := c.MoveTo(types.AxisY, 20); err == nil {
		t.Fatal("Y axis beyond max should be rejected even when not first in slice")
	}
	if err := c.MoveTo(types.AxisY, 10); err != nil {
		t.Fatalf("Y axis at max should be allowed, got %v", err)
	}
}

func TestSimulatedSoftLimit_HomeBypassesLimit(t *testing.T) {
	// 限位区间不含 0：回零目标 0 越限，但应与 B140 一致地放行
	c := NewSimulatedMotionController([]types.AxisConfig{simulatedAxisWithSoftLimit(1, 10)})
	if err := c.DefinePosition(types.AxisX, 5); err != nil {
		t.Fatalf("DefinePosition failed: %v", err)
	}
	if err := c.Home(types.AxisX); err != nil {
		t.Fatalf("Home should bypass soft limits, got %v", err)
	}
	if err := c.WaitForMotionComplete(types.AxisX, 3000); err != nil {
		t.Fatalf("wait failed: %v", err)
	}
	st, _ := c.GetAxisStatus(types.AxisX)
	if st.Position != 0 {
		t.Fatalf("position = %v, want 0 after home", st.Position)
	}
}
