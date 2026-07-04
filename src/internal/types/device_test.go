package types

import "testing"

func TestDeviceType_Info_KnownTypes(t *testing.T) {
	cases := []struct {
		t                DeviceType
		wantPressure     int
		wantTotal        int
		wantFrame        int
		wantIsDAQ        bool
		wantIsTemp       bool
	}{
		{DeviceTypeEA2508A, 8, 10, 45, true, false},
		{DeviceTypeEA2516A, 16, 18, 77, true, false},
		{DeviceTypeEA2516T, 16, 16, 0, true, true},
		{DeviceTypeSimulated, 16, 18, 77, false, false},
	}
	for _, tc := range cases {
		t.Run(string(tc.t), func(t *testing.T) {
			info := tc.t.Info()
			if info.Type != tc.t {
				t.Errorf("Type = %q, want %q", info.Type, tc.t)
			}
			if info.PressureChCount != tc.wantPressure {
				t.Errorf("PressureChCount = %d, want %d", info.PressureChCount, tc.wantPressure)
			}
			if info.TotalChCount != tc.wantTotal {
				t.Errorf("TotalChCount = %d, want %d", info.TotalChCount, tc.wantTotal)
			}
			if info.FrameSize != tc.wantFrame {
				t.Errorf("FrameSize = %d, want %d", info.FrameSize, tc.wantFrame)
			}
			if info.IsRealDAQ != tc.wantIsDAQ {
				t.Errorf("IsRealDAQ = %v, want %v", info.IsRealDAQ, tc.wantIsDAQ)
			}
			if info.IsTemperature != tc.wantIsTemp {
				t.Errorf("IsTemperature = %v, want %v", info.IsTemperature, tc.wantIsTemp)
			}
		})
	}
}

func TestDeviceType_Info_UnknownFallsBackToDAQ16(t *testing.T) {
	info := DeviceType("UNKNOWN").Info()
	if info.Type != DeviceTypeEA2516A {
		t.Errorf("expected fallback to EA2516A, got %q", info.Type)
	}
}

func TestDeviceType_Methods(t *testing.T) {
	if DeviceTypeEA2516A.PressureChannelCount() != 16 {
		t.Error("EA2516A PressureChannelCount should be 16")
	}
	if DeviceTypeEA2516A.TotalChannelCount() != 18 {
		t.Error("EA2516A TotalChannelCount should be 18")
	}
	if DeviceTypeEA2516A.StreamFrameSize() != 77 {
		t.Error("EA2516A StreamFrameSize should be 77")
	}
	if !DeviceTypeEA2516A.IsDAQDevice() {
		t.Error("EA2516A should be DAQ device")
	}
	if DeviceTypeEA2516A.IsTemperatureDevice() {
		t.Error("EA2516A should not be temperature device")
	}
	if !DeviceTypeEA2516T.IsTemperatureDevice() {
		t.Error("EA2516T should be temperature device")
	}
	if DeviceTypeSimulated.IsDAQDevice() {
		t.Error("SIMULATED should not be DAQ device")
	}
}

func TestAllDeviceTypes_ReturnsAllRegistered(t *testing.T) {
	all := AllDeviceTypes()
	if len(all) != 4 {
		t.Errorf("expected 4 device types, got %d", len(all))
	}
	seen := make(map[DeviceType]bool)
	for _, info := range all {
		seen[info.Type] = true
	}
	for _, want := range []DeviceType{DeviceTypeSimulated, DeviceTypeEA2508A, DeviceTypeEA2516A, DeviceTypeEA2516T} {
		if !seen[want] {
			t.Errorf("AllDeviceTypes missing %q", want)
		}
	}
}

// TestMigrateDeviceType 旧型号到新型号的迁移映射。
// 存量用户配置文件中保存的 "XY-DAQ8" / "XY-DAQ16" / "YX-DAQ-T" 必须能正确迁移，
// 否则 8 通道硬件会被 fallback 到 EA2516A（16 通道）导致通道配置错位、驱动无法连接。
func TestMigrateDeviceType(t *testing.T) {
	cases := []struct {
		name        string
		input       DeviceType
		wantMapped  DeviceType
		wantChanged bool
	}{
		{"XY-DAQ8 → EA2508A", "XY-DAQ8", DeviceTypeEA2508A, true},
		{"XY-DAQ16 → EA2516A", "XY-DAQ16", DeviceTypeEA2516A, true},
		{"YX-DAQ-T → EA2516T", "YX-DAQ-T", DeviceTypeEA2516T, true},
		{"EA2508A 不变", DeviceTypeEA2508A, DeviceTypeEA2508A, false},
		{"EA2516A 不变", DeviceTypeEA2516A, DeviceTypeEA2516A, false},
		{"EA2516T 不变", DeviceTypeEA2516T, DeviceTypeEA2516T, false},
		{"SIMULATED 不变", DeviceTypeSimulated, DeviceTypeSimulated, false},
		{"未知型号不迁移", "UNKNOWN-XYZ", "UNKNOWN-XYZ", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, changed := MigrateDeviceType(tc.input)
			if got != tc.wantMapped {
				t.Errorf("mapped = %q, want %q", got, tc.wantMapped)
			}
			if changed != tc.wantChanged {
				t.Errorf("changed = %v, want %v", changed, tc.wantChanged)
			}
		})
	}
}

// TestMigrateMotionControllerType B140-MC → EA25MC04 迁移
func TestMigrateMotionControllerType(t *testing.T) {
	cases := []struct {
		name        string
		input       MotionControllerType
		wantMapped  MotionControllerType
		wantChanged bool
	}{
		{"B140-MC → EA25MC04", "B140-MC", MotionTypeEA25MC04, true},
		{"EA25MC04 不变", MotionTypeEA25MC04, MotionTypeEA25MC04, false},
		{"SIMULATED-MC 不变", MotionTypeSimulated, MotionTypeSimulated, false},
		{"未知型号不迁移", "UNKNOWN-MC", "UNKNOWN-MC", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, changed := MigrateMotionControllerType(tc.input)
			if got != tc.wantMapped {
				t.Errorf("mapped = %q, want %q", got, tc.wantMapped)
			}
			if changed != tc.wantChanged {
				t.Errorf("changed = %v, want %v", changed, tc.wantChanged)
			}
		})
	}
}

func TestDefaultAxisConfigs(t *testing.T) {
	configs := DefaultAxisConfigs()
	if len(configs) != 4 {
		t.Fatalf("expected 4 axis configs, got %d", len(configs))
	}
	wantAxes := map[AxisName]bool{AxisX: false, AxisY: false, AxisZ: false, AxisU: false}
	for _, ac := range configs {
		if _, ok := wantAxes[ac.Name]; !ok {
			t.Errorf("unexpected axis %q", ac.Name)
		}
		wantAxes[ac.Name] = true
		if !ac.Enabled {
			t.Errorf("axis %q should be enabled by default", ac.Name)
		}
	}
	for axis, seen := range wantAxes {
		if !seen {
			t.Errorf("DefaultAxisConfigs missing axis %q", axis)
		}
	}
	// U 轴应为 ROTARY，其余 LINEAR
	for _, ac := range configs {
		if ac.Name == AxisU && ac.Kind != AxisKindRotary {
			t.Errorf("U axis should be ROTARY, got %q", ac.Kind)
		}
		if ac.Name != AxisU && ac.Kind != AxisKindLinear {
			t.Errorf("axis %q should be LINEAR, got %q", ac.Name, ac.Kind)
		}
	}
}
