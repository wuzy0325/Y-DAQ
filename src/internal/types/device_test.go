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
		{DeviceTypeXYDAQ8, 8, 10, 45, true, false},
		{DeviceTypeXYDAQ16, 16, 18, 77, true, false},
		{DeviceTypeYXDAQT, 16, 16, 0, true, true},
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
	if info.Type != DeviceTypeXYDAQ16 {
		t.Errorf("expected fallback to XY-DAQ16, got %q", info.Type)
	}
}

func TestDeviceType_Methods(t *testing.T) {
	if DeviceTypeXYDAQ16.PressureChannelCount() != 16 {
		t.Error("XY-DAQ16 PressureChannelCount should be 16")
	}
	if DeviceTypeXYDAQ16.TotalChannelCount() != 18 {
		t.Error("XY-DAQ16 TotalChannelCount should be 18")
	}
	if DeviceTypeXYDAQ16.StreamFrameSize() != 77 {
		t.Error("XY-DAQ16 StreamFrameSize should be 77")
	}
	if !DeviceTypeXYDAQ16.IsDAQDevice() {
		t.Error("XY-DAQ16 should be DAQ device")
	}
	if DeviceTypeXYDAQ16.IsTemperatureDevice() {
		t.Error("XY-DAQ16 should not be temperature device")
	}
	if !DeviceTypeYXDAQT.IsTemperatureDevice() {
		t.Error("YX-DAQ-T should be temperature device")
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
	for _, want := range []DeviceType{DeviceTypeSimulated, DeviceTypeXYDAQ8, DeviceTypeXYDAQ16, DeviceTypeYXDAQT} {
		if !seen[want] {
			t.Errorf("AllDeviceTypes missing %q", want)
		}
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
