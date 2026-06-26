package driver

import (
	"encoding/binary"
	"math"
	"testing"
)

// --- FrameParser 策略接口测试 ---

func TestDAQTBinaryParser_ValidFrame(t *testing.T) {
	frame := make([]byte, 64)
	// CH15=39.95°C at offset 0: float32 LE bits for 39.95
	bits := math.Float32bits(39.95)
	frame[0] = byte(bits)
	frame[1] = byte(bits >> 8)
	frame[2] = byte(bits >> 16)
	frame[3] = byte(bits >> 24)

	parser := &DAQTBinaryParser{}
	values, err := parser.Parse(frame)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(values) != 16 {
		t.Fatalf("expected 16 values, got %d", len(values))
	}
	// reverse 后 CH15 变为 index 15
	if values[15] < 39.9 || values[15] > 40.0 {
		t.Errorf("CH15 expected ~39.95, got %f", values[15])
	}
	for i := 0; i < 15; i++ {
		if values[i] != 0.0 {
			t.Errorf("CH%d expected 0.0, got %f", i, values[i])
		}
	}
}

func TestDAQTBinaryParser_ShortFrame(t *testing.T) {
	parser := &DAQTBinaryParser{}
	_, err := parser.Parse([]byte{0x00, 0x01, 0x02})
	if err == nil {
		t.Fatal("expected error for short frame")
	}
}

func TestDAQTASCIIParser_ValidFrame(t *testing.T) {
	raw := make([]byte, 192)
	offset := 0
	for i := 0; i < 16; i++ {
		var s string
		if i == 0 {
			s = "   39.952503"
		} else {
			s = "    0.000000"
		}
		copy(raw[offset:], []byte(s))
		offset += 12
	}

	parser := &DAQTASCIIParser{}
	values, err := parser.Parse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if values[15] < 39.9 || values[15] > 40.0 {
		t.Errorf("CH15 expected ~39.95, got %f", values[15])
	}
}

func TestDAQTMetadataParser_WithSeqAndTs(t *testing.T) {
	raw := []byte("0 1781600803.751855 0.000000 0.000000 0.000000 0.000000 0.000000 0.000000 0.000000 39.952503 0.000000 0.000000 0.000000 0.000000 0.000000 0.000000 0.000000 0.000000")

	parser := &DAQTMetadataParser{}
	values, err := parser.Parse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(values) != 16 {
		t.Fatalf("expected 16 values, got %d", len(values))
	}
	if values[8] < 39.9 || values[8] > 40.0 {
		t.Errorf("CH8 expected ~39.95, got %f", values[8])
	}
}

// --- 帧合法性校验 ---

func TestIsValidDAQTFrame(t *testing.T) {
	// 全零帧：0.0 在 -100~300 物理范围内，是合法帧
	allZero := make([]float64, 16)
	if !isValidDAQTFrame(allZero) {
		t.Error("all-zero frame should be valid (0.0 is in -100~300 range)")
	}

	valid := make([]float64, 16)
	for i := 0; i < 10; i++ {
		valid[i] = 25.0
	}
	if !isValidDAQTFrame(valid) {
		t.Error("frame with 10/16 valid channels should be valid")
	}

	// 超出范围的帧
	invalid := make([]float64, 16)
	for i := 0; i < 16; i++ {
		invalid[i] = 999.0 // 超出 -100~300
	}
	if isValidDAQTFrame(invalid) {
		t.Error("frame with all channels out of range should be invalid")
	}
}

// --- 帧读取器测试 ---

func TestDAQTFrameReader_BinaryMode(t *testing.T) {
	reader := NewDAQTFrameReader()
	reader.SetBinaryMode(true)

	frame := make([]byte, 64)
	bits := math.Float32bits(39.95)
	frame[0] = byte(bits)
	frame[1] = byte(bits >> 8)
	frame[2] = byte(bits >> 16)
	frame[3] = byte(bits >> 24)

	reader.Feed(frame)
	if !reader.HasCompleteFrame() {
		t.Fatal("expected complete frame")
	}
	result := reader.ReadFrame()
	if len(result) != 64 {
		t.Fatalf("expected 64 bytes, got %d", len(result))
	}
}

func TestDAQTFrameReader_PartialFrame(t *testing.T) {
	reader := NewDAQTFrameReader()
	reader.SetBinaryMode(true)

	reader.Feed(make([]byte, 32))
	if reader.HasCompleteFrame() {
		t.Fatal("should not have complete frame with 32 bytes")
	}

	reader.Feed(make([]byte, 32))
	if !reader.HasCompleteFrame() {
		t.Fatal("expected complete frame after 64 bytes")
	}
}

func TestDAQTFrameReader_Reset(t *testing.T) {
	reader := NewDAQTFrameReader()
	reader.SetBinaryMode(true)
	reader.Feed(make([]byte, 32))

	reader.Reset()
	if len(reader.buffer) != 0 {
		t.Fatal("buffer should be empty after reset")
	}
	if reader.frameSize != 64 {
		t.Fatal("frameSize should be preserved after reset")
	}
}

// --- 辅助函数测试 ---

func TestReverseFloat64(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5}
	reverseFloat64(values)
	if values[0] != 5 || values[4] != 1 {
		t.Errorf("reverse failed: %v", values)
	}
}

func TestIsParsableAsInt(t *testing.T) {
	if !isParsableAsInt("0") {
		t.Error("0 should be parsable as int")
	}
	if isParsableAsInt("1781600803.751855") {
		t.Error("float should not be parsable as int")
	}
}

func TestSplitWhitespace(t *testing.T) {
	tokens := splitWhitespace("  hello   world  ")
	if len(tokens) != 2 || tokens[0] != "hello" || tokens[1] != "world" {
		t.Errorf("unexpected tokens: %v", tokens)
	}
}

// --- DAQTSerialParser 测试 ---

func TestDAQTSerialParser_ValidFrame(t *testing.T) {
	frame := make([]byte, 46)
	// Header
	frame[0] = 0x55
	frame[1] = 0xAA
	// Temperature data at offset 8: CH0 = 250 (25.0°C)
	binary.BigEndian.PutUint16(frame[8:10], 250)
	// CH1 = -50 (-5.0°C)
	ch1Val := int16(-50)
	binary.BigEndian.PutUint16(frame[10:12], uint16(ch1Val))

	// Calculate and set checksum: sum(raw[0..44]) & 0xFF
	var sum byte
	for i := 0; i < 45; i++ {
		sum += frame[i]
	}
	frame[45] = sum

	parser := &DAQTSerialParser{}
	values, err := parser.Parse(frame)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(values) != 16 {
		t.Fatalf("expected 16 values, got %d", len(values))
	}
	if values[0] < 24.9 || values[0] > 25.1 {
		t.Errorf("CH0 expected 25.0, got %f", values[0])
	}
	if values[1] < -5.1 || values[1] > -4.9 {
		t.Errorf("CH1 expected -5.0, got %f", values[1])
	}
}

func TestDAQTSerialParser_ChecksumMismatch(t *testing.T) {
	frame := make([]byte, 46)
	frame[0] = 0x55
	frame[1] = 0xAA
	// Set some temperature data to make checksum non-trivial
	binary.BigEndian.PutUint16(frame[8:10], 1000)
	// Set wrong checksum (0x00 is very unlikely to be correct with 0x55+0xAA header)
	frame[45] = 0x00

	parser := &DAQTSerialParser{}
	_, err := parser.Parse(frame)
	if err == nil {
		t.Fatal("expected error for checksum mismatch")
	}
}

func TestDAQTSerialParser_InvalidHeader(t *testing.T) {
	frame := make([]byte, 46)
	parser := &DAQTSerialParser{}
	_, err := parser.Parse(frame)
	if err == nil {
		t.Fatal("expected error for invalid header")
	}
}

func TestDAQTSerialParser_ShortFrame(t *testing.T) {
	frame := make([]byte, 30)
	frame[0] = 0x55
	frame[1] = 0xAA
	parser := &DAQTSerialParser{}
	_, err := parser.Parse(frame)
	if err == nil {
		t.Fatal("expected error for short frame")
	}
}

// --- 温度单位转换测试 ---

func TestConvertTemperature(t *testing.T) {
	// 0°C = 32°F = 273.15K
	if v := convertTemperature(0, "°C", "°F"); v < 31.9 || v > 32.1 {
		t.Errorf("0°C → °F: expected 32, got %f", v)
	}
	if v := convertTemperature(0, "°C", "K"); v < 273.0 || v > 273.3 {
		t.Errorf("0°C → K: expected 273.15, got %f", v)
	}
	if v := convertTemperature(100, "°C", "°F"); v < 211.9 || v > 212.1 {
		t.Errorf("100°C → °F: expected 212, got %f", v)
	}
	// Identity
	if v := convertTemperature(25, "°C", "°C"); v != 25 {
		t.Errorf("identity conversion failed: got %f", v)
	}
	// Fahrenheit to Celsius
	if v := convertTemperature(32, "°F", "°C"); v < -0.1 || v > 0.1 {
		t.Errorf("32°F → °C: expected 0, got %f", v)
	}
	// Kelvin to Celsius
	if v := convertTemperature(273.15, "K", "°C"); v < -0.1 || v > 0.1 {
		t.Errorf("273.15K → °C: expected 0, got %f", v)
	}
}
