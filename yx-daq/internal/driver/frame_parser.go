package driver

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
)

// FrameParser 帧解析策略接口
type FrameParser interface {
	// Parse 解析一帧原始数据为通道值数组
	Parse(frame []byte) ([]float64, error)
}

// --- DAQ-T 帧解析器实现 ---

// DAQTBinaryParser DAQ-T 二进制帧解析器（BIN=1, 64字节 float32 LE）
type DAQTBinaryParser struct{}

func (p *DAQTBinaryParser) Parse(frame []byte) ([]float64, error) {
	if len(frame) < 64 {
		return nil, fmt.Errorf("binary frame too short: %d bytes, need 64", len(frame))
	}
	values := make([]float64, 16)
	for i := 0; i < 16; i++ {
		bits := binary.LittleEndian.Uint32(frame[i*4 : i*4+4])
		values[i] = float64(math.Float32frombits(bits))
	}
	reverseFloat64(values)
	return values, nil
}

// DAQTASCIIParser DAQ-T ASCII 定长帧解析器（BIN=0, 192字节）
type DAQTASCIIParser struct{}

func (p *DAQTASCIIParser) Parse(frame []byte) ([]float64, error) {
	s := string(frame)
	values := make([]float64, 16)
	for i := 0; i < 16; i++ {
		start := i * 12
		end := start + 12
		if end > len(s) {
			return nil, fmt.Errorf("ASCII frame too short at field %d", i)
		}
		var val float64
		fmt.Sscanf(s[start:end], "%f", &val)
		values[i] = val
	}
	reverseFloat64(values)
	return values, nil
}

// DAQTSerialParser DAQ-T 串口帧解析器（46字节二进制帧）
// 帧结构: 2B header + 1B length + 1B frameCount + 1B invFrameCount + 1B status + 2B version + 32B temp data + 4B reserved + 1B checksum + 1B(?) = 46 bytes
type DAQTSerialParser struct{}

func (p *DAQTSerialParser) Parse(frame []byte) ([]float64, error) {
	if len(frame) < 46 {
		return nil, fmt.Errorf("serial frame too short: %d bytes, need 46", len(frame))
	}
	// Validate header
	if frame[0] != 0x55 || frame[1] != 0xAA {
		return nil, fmt.Errorf("serial frame invalid header: %02X %02X", frame[0], frame[1])
	}
	// Validate checksum: sum(raw[0..43]) & 0xFF == raw[44]
	var sum byte
	for i := 0; i < 45; i++ {
		sum += frame[i]
	}
	if sum != frame[45] {
		return nil, fmt.Errorf("serial frame checksum mismatch: expected %02X, got %02X", sum, frame[45])
	}
	values := make([]float64, 16)
	for i := 0; i < 16; i++ {
		offset := 8 + i*2
		rawVal := int16(binary.BigEndian.Uint16(frame[offset : offset+2]))
		values[i] = float64(rawVal) * 0.1
	}
	// 串口帧已经是 CH0→CH15 顺序，无需 reverse
	return values, nil
}

// DAQTMetadataParser DAQ-T 变长 ASCII 帧解析器（TIME=1 或 HEAD=1）
type DAQTMetadataParser struct{}

func (p *DAQTMetadataParser) Parse(frame []byte) ([]float64, error) {
	s := trimSpace(string(frame))
	tokens := splitWhitespace(s)
	if len(tokens) < 16 {
		return nil, fmt.Errorf("metadata frame too few tokens: %d", len(tokens))
	}

	offset := 0
	if isParsableAsInt(tokens[0]) {
		offset = 1
		if len(tokens) > 17 {
			offset = 2
		}
	} else {
		offset = 1
	}

	if len(tokens) < offset+16 {
		return nil, fmt.Errorf("metadata frame not enough value tokens")
	}

	values := make([]float64, 16)
	for i := 0; i < 16; i++ {
		fmt.Sscanf(tokens[offset+i], "%f", &values[i])
	}
	reverseFloat64(values)
	return values, nil
}

// --- DAQ-T 帧读取器 ---

// DAQTFrameReader DAQ-T 帧读取器（缓冲 + 拆帧）
type DAQTFrameReader struct {
	buffer       []byte
	frameSize    int
	metadataMode bool
}

// NewDAQTFrameReader 创建 DAQ-T 帧读取器（默认二进制模式）
func NewDAQTFrameReader() *DAQTFrameReader {
	return &DAQTFrameReader{frameSize: 64}
}

// SetBinaryMode 设置二进制/ASCII 定长模式
func (r *DAQTFrameReader) SetBinaryMode(isBinary bool) {
	if isBinary {
		r.frameSize = 64
	} else {
		r.frameSize = 192
	}
}

// SetMetadataMode 设置变长元数据模式
func (r *DAQTFrameReader) SetMetadataMode(enabled bool) {
	r.metadataMode = enabled
}

// SetSerialMode 设置串口协议模式（46字节帧）
func (r *DAQTFrameReader) SetSerialMode(enabled bool) {
	if enabled {
		r.frameSize = 46
	}
}

// Feed 向缓冲区追加数据
func (r *DAQTFrameReader) Feed(data []byte) {
	r.buffer = append(r.buffer, data...)
}

// HasCompleteFrame 检查缓冲区中是否有完整帧
func (r *DAQTFrameReader) HasCompleteFrame() bool {
	if r.metadataMode {
		return r.hasVariableFrame()
	}
	return len(r.buffer) >= r.frameSize
}

// ReadFrame 读取一帧数据（从缓冲区移除）
func (r *DAQTFrameReader) ReadFrame() []byte {
	if r.metadataMode {
		return r.readVariableFrame()
	}
	return r.readFixedFrame()
}

// Reset 清空缓冲区（保留 frameSize 配置）
func (r *DAQTFrameReader) Reset() {
	r.buffer = r.buffer[:0]
}

func (r *DAQTFrameReader) readFixedFrame() []byte {
	if len(r.buffer) < r.frameSize {
		return nil
	}
	frame := make([]byte, r.frameSize)
	copy(frame, r.buffer[:r.frameSize])
	r.buffer = r.buffer[r.frameSize:]
	return frame
}

func (r *DAQTFrameReader) hasVariableFrame() bool {
	for _, need := range []int{18, 17} {
		if findFieldEnd(r.buffer, need) >= 0 {
			return true
		}
	}
	return false
}

func (r *DAQTFrameReader) readVariableFrame() []byte {
	for _, need := range []int{18, 17} {
		end := findFieldEnd(r.buffer, need)
		if end >= 0 {
			frame := make([]byte, end)
			copy(frame, r.buffer[:end])
			r.buffer = r.buffer[end:]
			return frame
		}
	}
	return nil
}

// --- 通用辅助函数 ---

// isValidDAQTFrame 校验帧数据合法性：至少 50% 通道值在物理范围内（-100~300°C）
func isValidDAQTFrame(values []float64) bool {
	validCount := 0
	for _, v := range values {
		if v >= -100 && v <= 300 {
			validCount++
		}
	}
	return validCount >= 8
}

// reverseFloat64 原地反转 float64 切片
func reverseFloat64(values []float64) {
	for i := 0; i < len(values)/2; i++ {
		j := len(values) - 1 - i
		values[i], values[j] = values[j], values[i]
	}
}

// isParsableAsInt 判断字符串是否可完整解析为整数
func isParsableAsInt(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

// splitWhitespace 按空白字符分割字符串
func splitWhitespace(s string) []string {
	var tokens []string
	start := -1
	for i, c := range s {
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			if start >= 0 {
				tokens = append(tokens, s[start:i])
				start = -1
			}
		} else {
			if start < 0 {
				start = i
			}
		}
	}
	if start >= 0 {
		tokens = append(tokens, s[start:])
	}
	return tokens
}

// findFieldEnd 查找缓冲区中第 n 个字段的结束位置
func findFieldEnd(buf []byte, n int) int {
	count := 0
	inField := false
	for i, b := range buf {
		if b == ' ' || b == '\t' || b == '\r' || b == '\n' {
			if inField {
				count++
				if count == n {
					return i
				}
			}
			inField = false
		} else {
			inField = true
		}
	}
	if inField {
		count++
		if count == n {
			return len(buf)
		}
	}
	return -1
}
