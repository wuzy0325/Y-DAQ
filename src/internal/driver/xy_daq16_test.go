package driver

import (
	"encoding/binary"
	"math"
	"testing"

	"yx-daq/internal/types"
)

// TestStreamContentMask 验证 c 05 位图选择：大气压/温度使能决定 0810/0010
func TestStreamContentMask(t *testing.T) {
	if got := streamContentMask(true); got != streamMaskWithAtm {
		t.Errorf("atm enabled mask = %q, want %q", got, streamMaskWithAtm)
	}
	if got := streamContentMask(false); got != streamMaskPressureOnly {
		t.Errorf("atm disabled mask = %q, want %q", got, streamMaskPressureOnly)
	}
}

// TestXYDAQDriver_FrameSpec 验证帧规格随大气压/温度使能切换
func TestXYDAQDriver_FrameSpec(t *testing.T) {
	cases := []struct {
		name     string
		device   types.DeviceType
		atm      bool
		wantCh   int
		wantSize int
	}{
		{"EA2516A atm on", types.DeviceTypeEA2516A, true, 18, 77},
		{"EA2516A atm off", types.DeviceTypeEA2516A, false, 16, types.DeviceTypeEA2516A.PressureOnlyFrameSize()},
		{"EA2508A atm on", types.DeviceTypeEA2508A, true, 10, 45},
		{"EA2508A atm off", types.DeviceTypeEA2508A, false, 8, types.DeviceTypeEA2508A.PressureOnlyFrameSize()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			drv := NewXYDAQDriver("127.0.0.1", 9000, 1, nil, tc.device, tc.atm)
			ch, size := drv.frameSpec()
			if ch != tc.wantCh || size != tc.wantSize {
				t.Errorf("frameSpec() = (%d, %d), want (%d, %d)", ch, size, tc.wantCh, tc.wantSize)
			}
		})
	}
}

// TestXYDAQDriver_SetAtmEnabled 验证运行时切换后帧规格即时更新
func TestXYDAQDriver_SetAtmEnabled(t *testing.T) {
	drv := NewXYDAQDriver("127.0.0.1", 9000, 1, nil, types.DeviceTypeEA2516A, true)
	drv.SetAtmEnabled(false)
	if ch, size := drv.frameSpec(); ch != 16 || size != types.DeviceTypeEA2516A.PressureOnlyFrameSize() {
		t.Fatalf("after disable: frameSpec() = (%d, %d), want (16, 69)", ch, size)
	}
	drv.SetAtmEnabled(true)
	if ch, size := drv.frameSpec(); ch != 18 || size != 77 {
		t.Fatalf("after enable: frameSpec() = (%d, %d), want (18, 77)", ch, size)
	}
}

// buildPressureFrame 构造压力帧：硬件按 CHn→CH1 逆序发送
func buildPressureFrame(pressureCount int, valueAt func(ch int) float64) []byte {
	frame := make([]byte, types.StreamFrameHeaderSize+pressureCount*4)
	for pos := 0; pos < pressureCount; pos++ {
		ch := pressureCount - pos
		bits := math.Float32bits(float32(valueAt(ch)))
		binary.BigEndian.PutUint32(frame[types.StreamFrameHeaderSize+pos*4:], bits)
	}
	return frame
}

func newTestXYChannels(total int) []types.ChannelConfig {
	channels := make([]types.ChannelConfig, total)
	for i := range channels {
		channels[i] = types.ChannelConfig{Index: i, Name: "CH", Enabled: true, Unit: "kPa"}
	}
	return channels
}

// TestXYDAQDriver_HandleStreamFrame_AtmDisabled 大气压/温度关闭时只解析压力通道，
// 大气压/大气温度通道不出现在数据载荷中
func TestXYDAQDriver_HandleStreamFrame_AtmDisabled(t *testing.T) {
	drv := NewXYDAQDriver("127.0.0.1", 9000, 1, newTestXYChannels(18), types.DeviceTypeEA2516A, false)
	var got types.DataPayload
	drv.SetDataCallback(func(p types.DataPayload) { got = p })
	drv.acquiring.Store(true)

	drv.handleStreamFrame(buildPressureFrame(16, func(ch int) float64 { return float64(ch) + 0.5 }))

	if len(got.Channels) != 16 {
		t.Fatalf("atm disabled: got %d channels, want 16", len(got.Channels))
	}
	for i, idx := range got.ChannelIndices {
		if idx != i {
			t.Fatalf("channel index[%d] = %d, want %d", i, idx, i)
		}
	}
	// 硬件逆序发送，驱动反转后 CH1=1.5、CH16=16.5
	if got.Channels[0] != 1.5 || got.Channels[15] != 16.5 {
		t.Errorf("pressure values after reverse = [%v ... %v], want [1.5 ... 16.5]", got.Channels[0], got.Channels[15])
	}
}

// TestXYDAQDriver_HandleStreamFrame_AtmEnabled 大气压/温度开启时解析全部通道，
// 且大气压/大气温度位于压力通道之后
func TestXYDAQDriver_HandleStreamFrame_AtmEnabled(t *testing.T) {
	drv := NewXYDAQDriver("127.0.0.1", 9000, 1, newTestXYChannels(18), types.DeviceTypeEA2516A, true)
	var got types.DataPayload
	drv.SetDataCallback(func(p types.DataPayload) { got = p })
	drv.acquiring.Store(true)

	frame := buildPressureFrame(16, func(ch int) float64 { return float64(ch) + 0.5 })
	frame = append(frame, 0, 0, 0, 0) // CH17 大气压
	binary.BigEndian.PutUint32(frame[types.StreamFrameHeaderSize+16*4:], math.Float32bits(101325))
	frame = append(frame, 0, 0, 0, 0) // CH18 大气温度
	binary.BigEndian.PutUint32(frame[types.StreamFrameHeaderSize+17*4:], math.Float32bits(25.5))

	drv.handleStreamFrame(frame)

	if len(got.Channels) != 18 {
		t.Fatalf("atm enabled: got %d channels, want 18", len(got.Channels))
	}
	if got.Channels[16] != 101325 || got.Channels[17] != 25.5 {
		t.Errorf("atm channels = [%v, %v], want [101325, 25.5]", got.Channels[16], got.Channels[17])
	}
}

// TestXYDAQDriver_HandleStreamFrame_AtmEnabledAcceptsPressureOnlyFrame
// atm 使能但设备未应用 0810 位图（返回仅压力帧）时回退解析，避免静默无数据；
// 其他非预期帧长仍丢弃
func TestXYDAQDriver_HandleStreamFrame_AtmEnabledAcceptsPressureOnlyFrame(t *testing.T) {
	drv := NewXYDAQDriver("127.0.0.1", 9000, 1, newTestXYChannels(18), types.DeviceTypeEA2516A, true)
	var got types.DataPayload
	drv.SetDataCallback(func(p types.DataPayload) { got = p })
	drv.acquiring.Store(true)

	drv.handleStreamFrame(buildPressureFrame(16, func(ch int) float64 { return float64(ch) + 0.5 }))
	if len(got.Channels) != 16 {
		t.Fatalf("fallback: got %d channels, want 16", len(got.Channels))
	}
	if got.Channels[0] != 1.5 || got.Channels[15] != 16.5 {
		t.Errorf("fallback pressure values = [%v ... %v], want [1.5 ... 16.5]", got.Channels[0], got.Channels[15])
	}

	got = types.DataPayload{}
	drv.handleStreamFrame(buildPressureFrame(14, func(int) float64 { return 1 }))
	if len(got.Channels) != 0 {
		t.Fatalf("unexpected frame length should be dropped, got %d channels", len(got.Channels))
	}
}

// TestXYDAQDriver_HandleStreamFrame_AtmDisabledShortFrame 大气压关闭时
// 收到压力帧长度（69B）即可解析；不足则丢弃
func TestXYDAQDriver_HandleStreamFrame_AtmDisabledShortFrame(t *testing.T) {
	drv := NewXYDAQDriver("127.0.0.1", 9000, 1, newTestXYChannels(18), types.DeviceTypeEA2516A, true)
	drv.SetAtmEnabled(false)
	var got types.DataPayload
	drv.SetDataCallback(func(p types.DataPayload) { got = p })
	drv.acquiring.Store(true)

	// 压力帧（69B）：应可解析
	drv.handleStreamFrame(buildPressureFrame(16, func(int) float64 { return 1 }))
	if len(got.Channels) != 16 {
		t.Fatalf("69B frame with atm disabled: got %d channels, want 16", len(got.Channels))
	}
	// 更短帧：应被丢弃（保持上一帧数据不变）
	got = types.DataPayload{}
	drv.handleStreamFrame(buildPressureFrame(14, func(int) float64 { return 1 }))
	if len(got.Channels) != 0 {
		t.Fatalf("short frame should be dropped, got %d channels", len(got.Channels))
	}
}

// TestXYDAQDriver_ProcessData_RealignsMisalignedBinaryFrame 验证错位长度前缀
// 落在合法区间但不匹配设备帧长时，不按错误长度消费字节而是逐字节重对齐，
// 后续真实帧仍能完整解析（回归：帧错位解析出 0.x 伪压力值）
func TestXYDAQDriver_ProcessData_RealignsMisalignedBinaryFrame(t *testing.T) {
	drv := NewXYDAQDriver("127.0.0.1", 9000, 1, newTestXYChannels(18), types.DeviceTypeEA2516A, true)
	var got []types.DataPayload
	drv.SetDataCallback(func(p types.DataPayload) { got = append(got, p) })
	drv.acquiring.Store(true)

	// 错位区：前缀 0x0049=73 → 载荷 71 字节（合法区间内但非 77/69），
	// 载荷首字节 0x00 会被误判为二进制帧
	garbage := make([]byte, 73)
	garbage[1] = 0x49

	// 真实帧：2B 前缀 + 77B 载荷
	payload := buildPressureFrame(16, func(ch int) float64 { return float64(ch) + 0.5 })
	payload = append(payload, 0, 0, 0, 0)
	binary.BigEndian.PutUint32(payload[types.StreamFrameHeaderSize+16*4:], math.Float32bits(101325))
	payload = append(payload, 0, 0, 0, 0)
	binary.BigEndian.PutUint32(payload[types.StreamFrameHeaderSize+17*4:], math.Float32bits(25.5))
	wire := make([]byte, 2+len(payload))
	binary.BigEndian.PutUint16(wire[:2], uint16(len(wire)))
	copy(wire[2:], payload)

	drv.RecvBuffer = append(drv.RecvBuffer, garbage...)
	drv.RecvBuffer = append(drv.RecvBuffer, wire...)
	drv.processData(nil)

	if len(got) != 1 {
		t.Fatalf("emitted %d frames, want 1 (misaligned bytes must be dropped)", len(got))
	}
	if got[0].Channels[0] != 1.5 || got[0].Channels[15] != 16.5 || got[0].Channels[16] != 101325 || got[0].Channels[17] != 25.5 {
		t.Fatalf("frame corrupted after realign: %v", got[0].Channels)
	}
	if len(drv.RecvBuffer) != 0 {
		t.Fatalf("buffer should be drained, got %d bytes", len(drv.RecvBuffer))
	}
}

// TestXYDAQDriver_ProcessData_AsciiResponseStillRouted 验证加固后
// ASCII 命令响应仍正常路由（二进制长度校验不得误伤 ASCII 帧）
func TestXYDAQDriver_ProcessData_AsciiResponseStillRouted(t *testing.T) {
	drv := NewXYDAQDriver("127.0.0.1", 9000, 1, newTestXYChannels(18), types.DeviceTypeEA2516A, true)

	wire := []byte{0, 3, 'A'}
	drv.RecvBuffer = append(drv.RecvBuffer, wire...)
	drv.processData(nil)

	select {
	case resp := <-drv.CmdRespCh:
		if string(resp) != "A" {
			t.Fatalf("routed response = %q, want %q", string(resp), "A")
		}
	default:
		t.Fatal("ASCII response was not routed to command response channel")
	}
}

// TestXYDAQDriver_HandleStreamFrame_RejectsInvalidValues 验证 NaN/Inf/超量级
// 帧整帧丢弃，不进入显示/存储/零位校准采样
func TestXYDAQDriver_HandleStreamFrame_RejectsInvalidValues(t *testing.T) {
	cases := []struct {
		name string
		bits uint32
	}{
		{"NaN", 0x7FC00001},
		{"+Inf", 0x7F800000},
		{"absurd magnitude", 0x7F000000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			drv := NewXYDAQDriver("127.0.0.1", 9000, 1, newTestXYChannels(16), types.DeviceTypeEA2516A, false)
			var got types.DataPayload
			drv.SetDataCallback(func(p types.DataPayload) { got = p })
			drv.acquiring.Store(true)

			frame := buildPressureFrame(16, func(int) float64 { return 1 })
			binary.BigEndian.PutUint32(frame[types.StreamFrameHeaderSize:], tc.bits)
			drv.handleStreamFrame(frame)

			if len(got.Channels) != 0 {
				t.Fatalf("invalid frame must be dropped, got %v", got.Channels)
			}
		})
	}
}
