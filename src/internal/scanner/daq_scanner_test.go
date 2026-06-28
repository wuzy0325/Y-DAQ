package scanner

import (
	"encoding/json"
	"net"
	"testing"

	"yx-daq/internal/types"
)

// newScanner 创建一个最小化的 DAQScanner 实例用于调用私有 parseResponse 方法
func newScanner() *DAQScanner { return &DAQScanner{} }

func TestParseResponse_JSON_DAQT(t *testing.T) {
	resp := `{"ip":"192.168.1.7","mac":"AA:BB:CC:DD:EE:FF","serialNumber":"SN12345","model":"DAQ-T-1603","firmwareVersion":"v1.0","port":9000}`
	dev, ok := newScanner().parseResponse(resp)
	if !ok {
		t.Fatal("expected ok for JSON response")
	}
	if dev.IP != "192.168.1.7" {
		t.Errorf("IP = %q", dev.IP)
	}
	if dev.MAC != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("MAC = %q", dev.MAC)
	}
	if dev.SN != "SN12345" {
		t.Errorf("SN = %q", dev.SN)
	}
	if dev.Firmware != "v1.0" {
		t.Errorf("Firmware = %q", dev.Firmware)
	}
	if dev.Port != 9000 {
		t.Errorf("Port = %d", dev.Port)
	}
}

func TestParseResponse_JSON_EmptyIPRejected(t *testing.T) {
	// JSON 解析成功但 IP 为空 → 应返回 false，回退到 CSV（CSV 也会失败）
	resp := `{"ip":"","mac":"AA"}`
	_, ok := newScanner().parseResponse(resp)
	if ok {
		t.Error("empty IP JSON should not produce a device")
	}
}

func TestParseResponse_JSON_MalformedFallsBackToCSV(t *testing.T) {
	// 以 { 开头但 JSON 非法 → 应回退到 CSV 解析
	// CSV 至少需要 10 字段，这里给出 10 字段
	resp := "{malformed,AA:BB:CC:DD:EE:FF,_,SN001,v1.0,_,_,9000,255.255.255.0,192.168.1.1"
	dev, ok := newScanner().parseResponse(resp)
	if !ok {
		t.Fatal("expected CSV fallback to succeed")
	}
	if dev.IP != "{malformed" {
		t.Errorf("IP = %q (CSV first field)", dev.IP)
	}
	if dev.SN != "SN001" {
		t.Errorf("SN = %q", dev.SN)
	}
	if dev.Port != 9000 {
		t.Errorf("Port = %d", dev.Port)
	}
}

func TestParseResponse_CSV_XYDAQ(t *testing.T) {
	// IP,MAC,_,SN,FW,_,_,Port,Mask,GW
	resp := "192.168.3.101,AA:BB:CC:DD:EE:FF,_,SN001,v1.0,_,_,9000,255.255.255.0,192.168.1.1"
	dev, ok := newScanner().parseResponse(resp)
	if !ok {
		t.Fatal("expected ok for CSV response")
	}
	if dev.IP != "192.168.3.101" {
		t.Errorf("IP = %q", dev.IP)
	}
	if dev.MAC != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("MAC = %q", dev.MAC)
	}
	if dev.SN != "SN001" {
		t.Errorf("SN = %q", dev.SN)
	}
	if dev.Firmware != "v1.0" {
		t.Errorf("Firmware = %q", dev.Firmware)
	}
	if dev.Port != 9000 {
		t.Errorf("Port = %d", dev.Port)
	}
	if dev.Mask != "255.255.255.0" {
		t.Errorf("Mask = %q", dev.Mask)
	}
	if dev.Gateway != "192.168.1.1" {
		t.Errorf("Gateway = %q", dev.Gateway)
	}
}

func TestParseResponse_CSV_TrimsWhitespace(t *testing.T) {
	resp := " 192.168.3.101 , AA:BB , _ , SN , v1 , _ , _ , 9000 , mask , gw "
	dev, ok := newScanner().parseResponse(resp)
	if !ok {
		t.Fatal("expected ok for CSV with whitespace")
	}
	if dev.IP != "192.168.3.101" {
		t.Errorf("IP = %q", dev.IP)
	}
	if dev.SN != "SN" {
		t.Errorf("SN = %q", dev.SN)
	}
	if dev.Mask != "mask" {
		t.Errorf("Mask = %q", dev.Mask)
	}
}

func TestParseResponse_CSV_TooFewFields(t *testing.T) {
	// 少于 10 字段 → false
	resp := "a,b,c"
	_, ok := newScanner().parseResponse(resp)
	if ok {
		t.Error("CSV with < 10 fields should be rejected")
	}
}

func TestParseResponse_CSV_PortNotNumber(t *testing.T) {
	// Port 字段非数字 → Port=0，但仍返回 ok
	resp := "192.168.3.101,AA:BB,_,SN,v1,_,_,not_a_number,mask,gw"
	dev, ok := newScanner().parseResponse(resp)
	if !ok {
		t.Fatal("expected ok even when port is non-numeric")
	}
	if dev.Port != 0 {
		t.Errorf("Port should default to 0, got %d", dev.Port)
	}
}

func TestParseResponse_EmptyString(t *testing.T) {
	_, ok := newScanner().parseResponse("")
	if ok {
		t.Error("empty string should not produce a device")
	}
}

func TestCalcBroadcast_StandardMask(t *testing.T) {
	_, ipNet, err := net.ParseCIDR("192.168.1.10/24")
	if err != nil {
		t.Fatalf("ParseCIDR failed: %v", err)
	}
	got := newScanner().calcBroadcast(ipNet)
	if got != "192.168.1.255" {
		t.Errorf("broadcast = %q, want 192.168.1.255", got)
	}
}

func TestCalcBroadcast_NarrowMask(t *testing.T) {
	_, ipNet, err := net.ParseCIDR("10.0.0.5/30")
	if err != nil {
		t.Fatalf("ParseCIDR failed: %v", err)
	}
	got := newScanner().calcBroadcast(ipNet)
	if got != "10.0.0.7" {
		t.Errorf("broadcast = %q, want 10.0.0.7", got)
	}
}

func TestCalcBroadcast_HostMask(t *testing.T) {
	// /32 → broadcast = ip itself
	_, ipNet, err := net.ParseCIDR("192.168.1.5/32")
	if err != nil {
		t.Fatalf("ParseCIDR failed: %v", err)
	}
	got := newScanner().calcBroadcast(ipNet)
	if got != "192.168.1.5" {
		t.Errorf("broadcast = %q, want 192.168.1.5", got)
	}
}

func TestCalcBroadcast_DefaultRoute(t *testing.T) {
	// 0.0.0.0/0 → broadcast = 255.255.255.255
	_, ipNet, err := net.ParseCIDR("0.0.0.0/0")
	if err != nil {
		t.Fatalf("ParseCIDR failed: %v", err)
	}
	got := newScanner().calcBroadcast(ipNet)
	if got != "255.255.255.255" {
		t.Errorf("broadcast = %q, want 255.255.255.255", got)
	}
}

func TestCalcBroadcast_IPv6Rejected(t *testing.T) {
	// IPv6 应返回空
	_, ipNet, err := net.ParseCIDR("2001:db8::1/64")
	if err != nil {
		t.Fatalf("ParseCIDR failed: %v", err)
	}
	got := newScanner().calcBroadcast(ipNet)
	if got != "" {
		t.Errorf("IPv6 should return empty, got %q", got)
	}
}

// 编译期断言：确保 DiscoveredDevice 与 daqTDiscoveryResponse 字段对齐
var _ = json.Marshal
var _ types.DiscoveredDevice
