package driver

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"sync/atomic"
	"time"

	"yx-daq/internal/types"
)

// XYDAQDriver XY-DAQ TCP驱动（支持DAQ8/DAQ16）
type XYDAQDriver struct {
	*TCPDriverBase
	streamID      int
	deviceType    types.DeviceType
	pressureCount int          // 压力通道数（8或16）
	totalChannels int          // 总通道数（压力+大气压+大气温度）
	frameSize     int          // 数据帧大小（字节）
	lastPeriodMs  atomic.Int32 // 最近一次采集周期（重连恢复采集时使用）
	atmEnabled    atomic.Bool  // 大气压/大气温度采集使能（c 05 位图 0x0800）
}

// maxXYFrameLen XY-DAQ 帧长度合法性上限。
// 正常数据帧 ≤ 79 字节（77+2 前缀），ASCII 响应帧更短；
// 超过该值的前缀必为残杂字节导致的错位，应触发重新对齐
const maxXYFrameLen = 512

// NewXYDAQDriver 创建XY-DAQ驱动（DAQ8/DAQ16通用）。
// atmEnabled 控制数据流是否包含大气压/大气温度（对应 c 05 位图 0x0800）。
func NewXYDAQDriver(host string, port, streamID int, channels []types.ChannelConfig, deviceType types.DeviceType, atmEnabled bool) *XYDAQDriver {
	pressureCount := deviceType.PressureChannelCount()
	totalChannels := deviceType.TotalChannelCount()
	d := &XYDAQDriver{
		TCPDriverBase: NewTCPDriverBase(host, port, channels),
		streamID:      streamID,
		deviceType:    deviceType,
		pressureCount: pressureCount,
		totalChannels: totalChannels,
		frameSize:     deviceType.StreamFrameSize(),
	}
	d.atmEnabled.Store(atmEnabled)
	return d
}

// Connect 建立TCP连接
func (d *XYDAQDriver) Connect() error {
	// 显式连接清空上次用户主动断开留下的终止标记（重连循环内部不复位该标记）
	d.ResetUserDisconnected()
	if err := d.DialConnect(); err != nil {
		return err
	}
	// 注册重连 hook，之后 HandleDisconnect 重连成功时会自动调用 initAfterConnect
	d.SetOnReconnect(d.initAfterConnect)
	// 注册采集恢复 hook：断连前正在采集时，重连成功后自动恢复采集
	d.SetOnResumeAcquire(func() error {
		periodMs := int(d.lastPeriodMs.Load())
		if periodMs <= 0 {
			periodMs = 50
		}
		return d.StartAcquisition(periodMs)
	})
	return d.initAfterConnect()
}

// initAfterConnect 连接建立后的初始化逻辑（首次连接与重连均调用）
// 包含：w1601 模式切换、EU 单位读取、启动数据接收协程
func (d *XYDAQDriver) initAfterConnect() error {
	// 启用2字节长度前缀模式
	if err := d.WriteCommandOnly("w1601\r"); err != nil {
		d.closeConnLocked()
		d.connected.Store(false)
		return fmt.Errorf("send w1601 failed: %w", err)
	}
	time.Sleep(50 * time.Millisecond)
	// 排空 w1601 的 ACK 响应，避免残留字节被后续直读命令误当作响应
	d.ConsumeOptionalACK(50)

	// 读取设备EU单位并更新通道配置
	d.readAndUpdateEUUnit()

	// EA2508A：按持久化配置下发温度通道配置（@16 来源 + @17 热电偶类型）
	if d.deviceType == types.DeviceTypeEA2508A {
		d.applyTempChannelConfig()
	}

	// 启动数据接收协程
	d.StartReceiveLoop(d.processData)
	return nil
}

// Disconnect 断开连接
func (d *XYDAQDriver) Disconnect() {
	d.CloseDisconnect()
}

// StartAcquisition 启动采集
func (d *XYDAQDriver) StartAcquisition(periodMs int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.connected.Load() {
		return fmt.Errorf("device not connected")
	}

	if d.acquiring.Load() {
		return nil
	}

	// 记录采集周期，断连重连后用于恢复采集
	d.lastPeriodMs.Store(int32(periodMs))

	streamTag := fmt.Sprintf("%d", d.streamID)

	// 配置数据流参数: 内部时钟/大端/连续
	cmd1 := fmt.Sprintf("c 00 %s FFFF 1 %d 7 0\r", streamTag, periodMs)
	if _, err := d.Conn.Write([]byte(cmd1)); err != nil {
		return fmt.Errorf("configure stream failed: %w", err)
	}
	time.Sleep(100 * time.Millisecond)

	// 配置返回内容: 压力+大气压+温度（0810）或仅压力（0010）
	cmd2 := fmt.Sprintf("c 05 %s %s\r", streamTag, streamContentMask(d.atmEnabled.Load()))
	if _, err := d.Conn.Write([]byte(cmd2)); err != nil {
		return fmt.Errorf("configure stream content failed: %w", err)
	}
	time.Sleep(100 * time.Millisecond)

	// 启动数据流
	cmd3 := fmt.Sprintf("c 01 %s\r", streamTag)
	if _, err := d.Conn.Write([]byte(cmd3)); err != nil {
		return fmt.Errorf("start stream failed: %w", err)
	}

	d.acquiring.Store(true)
	return nil
}

// StopAcquisition 停止采集
func (d *XYDAQDriver) StopAcquisition() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.connected.Load() {
		return fmt.Errorf("device not connected")
	}

	if !d.acquiring.Load() {
		return nil
	}

	streamTag := fmt.Sprintf("%d", d.streamID)
	cmd := fmt.Sprintf("c 02 %s\r", streamTag)
	if _, err := d.Conn.Write([]byte(cmd)); err != nil {
		return fmt.Errorf("stop stream failed: %w", err)
	}

	d.acquiring.Store(false)
	d.draining.Store(true)
	go func() {
		time.Sleep(200 * time.Millisecond)
		d.draining.Store(false)
	}()
	return nil
}

// processData 处理接收缓冲区数据（2字节长度前缀拆包）
// 遇到垃圾长度前缀（残杂字节/半帧残留导致）时逐字节丢弃重新对齐，自愈帧错位，
// 避免等待不可能达到的字节数导致数据流永久停摆
func (d *XYDAQDriver) processData(_ []byte) {
	dropped := 0
	for len(d.RecvBuffer) >= 2 {
		// 2字节大端长度前缀
		frameLen := int(binary.BigEndian.Uint16(d.RecvBuffer[:2]))
		if frameLen < 2 || frameLen > maxXYFrameLen {
			// 非法长度前缀：丢弃首字节重新对齐
			d.RecvBuffer = d.RecvBuffer[1:]
			dropped++
			continue
		}
		if len(d.RecvBuffer) < frameLen {
			break
		}

		frame := d.RecvBuffer[:frameLen]
		d.RecvBuffer = d.RecvBuffer[frameLen:]

		// 判断帧类型
		payload := frame[2:] // 去掉长度前缀
		if len(payload) > 0 && payload[0] < 0x20 {
			// 二进制帧
			d.handleStreamFrame(payload)
		} else {
			// ASCII帧路由到命令响应通道
			d.RouteToCmdRespCh(payload)
		}
	}
	if dropped > 0 {
		slog.Warn("XY-DAQ frame desync, dropped bytes to realign", "host", d.Host, "port", d.Port, "dropped", dropped)
	}
}

// handleStreamFrame 处理二进制数据流帧
func (d *XYDAQDriver) handleStreamFrame(frame []byte) {
	if d.draining.Load() {
		return
	}
	if !d.acquiring.Load() {
		return
	}
	// 帧结构: 头(5B) + CH1(4B float32 BE) + ... + CHn(4B)
	channelCount, frameSize := d.frameSpec()
	if len(frame) < frameSize {
		// 容错：atm 使能但设备未应用 0810 位图时按仅压力帧解析，避免静默无数据
		if !d.atmEnabled.Load() || len(frame) != d.deviceType.PressureOnlyFrameSize() {
			return
		}
		channelCount = d.pressureCount
	}

	values := make([]float64, channelCount)
	for i := 0; i < channelCount; i++ {
		offset := types.StreamFrameHeaderSize + i*4
		bits := binary.BigEndian.Uint32(frame[offset : offset+4])
		values[i] = float64(math.Float32frombits(bits))
	}

	// 反转压力通道顺序：硬件按 CHn→CH1 逆序发送，需反转为 CH1→CHn
	for i := 0; i < d.pressureCount/2; i++ {
		j := d.pressureCount - 1 - i
		values[i], values[j] = values[j], values[i]
	}

	deviceID := fmt.Sprintf("%s:%d", d.Host, d.Port)
	payload := d.BuildDataPayload(values, deviceID)
	d.EmitData(payload)
}

// SendCommand 发送命令并等待ASCII响应（用于查询类命令）
// 注意：此方法在receiveLoop启动前调用，直接从conn读取响应
func (d *XYDAQDriver) SendCommand(cmd string) (string, error) {
	if !d.connected.Load() {
		return "", fmt.Errorf("device not connected")
	}
	conn := d.connRef()
	if conn == nil {
		return "", fmt.Errorf("device not connected")
	}

	// 发送命令
	if _, err := conn.Write([]byte(cmd + "\r")); err != nil {
		return "", fmt.Errorf("send command %q failed: %w", cmd, err)
	}

	// 读取响应（带超时）
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	conn.SetReadDeadline(time.Time{}) // 清除超时
	if err != nil {
		return "", fmt.Errorf("read response for %q failed: %w", cmd, err)
	}

	// 响应可能带2字节长度前缀，去掉
	resp := buf[:n]
	if n >= 2 {
		frameLen := int(binary.BigEndian.Uint16(resp[:2]))
		if frameLen >= 2 && frameLen <= n {
			resp = resp[2:frameLen]
		}
	}

	return string(resp), nil
}

// sendUnitCommand sends unit read/write commands via length-prefix protocol
func (d *XYDAQDriver) sendUnitCommand(cmd string) (string, error) {
	if !d.connected.Load() {
		return "", fmt.Errorf("device not connected")
	}

	// receiveLoop 运行时通过 cmdRespCh 获取响应，避免与 receiveLoop 竞争 conn.Read
	if d.recvLoopRunning.Load() {
		// 排空残留响应
		select {
		case <-d.CmdRespCh:
		default:
		}

		conn := d.connRef()
		if conn == nil {
			return "", fmt.Errorf("device not connected")
		}
		if _, err := conn.Write([]byte(cmd)); err != nil {
			return "", fmt.Errorf("send unit command %q failed: %w", cmd, err)
		}

		select {
		case payload := <-d.CmdRespCh:
			// processData 已剥离 2 字节长度前缀，payload 为纯响应数据，无需再走 parseUnitPayload
			return trimSpace(string(payload)), nil
		case <-time.After(3 * time.Second):
			return "", fmt.Errorf("unit command %q timeout", cmd)
		}
	}

	// receiveLoop 未运行时直接读写（Connect 初始化阶段 / 重连 hook 阶段）
	conn := d.connRef()
	if conn == nil {
		return "", fmt.Errorf("device not connected")
	}
	if _, err := conn.Write([]byte(cmd)); err != nil {
		return "", fmt.Errorf("send unit command %q failed: %w", cmd, err)
	}

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	conn.SetReadDeadline(time.Time{})
	if err != nil {
		return "", fmt.Errorf("read unit command response failed: %w", err)
	}
	return parseUnitPayload(buf[:n])
}

// parseUnitPayload 解析单位命令响应帧
func parseUnitPayload(raw []byte) (string, error) {
	if len(raw) < 2 {
		return "", fmt.Errorf("unit command response too short: %d bytes", len(raw))
	}

	frameLen := int(binary.BigEndian.Uint16(raw[:2]))
	if frameLen < 2 || frameLen > len(raw) {
		return "", fmt.Errorf("invalid unit response frame: len=%d, data=%d", frameLen, len(raw))
	}

	resp := string(raw[2:frameLen])
	for i := 0; i < len(resp); i++ {
		if resp[i] == 0 {
			resp = resp[:i]
			break
		}
	}
	return trimSpace(resp), nil
}

// readAndUpdateEUUnit 连接后读取设备EU单位并更新通道配置
func (d *XYDAQDriver) readAndUpdateEUUnit() {
	resp, err := d.sendUnitCommand("u01101")
	if err != nil {
		slog.Warn("XY-DAQ read EU unit failed", "err", err)
		return
	}

	unit := types.CoeffToUnit(resp)
	if unit != "" {
		for i := range d.channels {
			if d.channels[i].Index < d.pressureCount {
				d.channels[i].Unit = unit
			}
		}
		slog.Info("XY-DAQ EU unit from device", "unit", unit, "coeff", resp)
	}
}

// SetUnit 设置设备压力单位（写入硬件）
func (d *XYDAQDriver) SetUnit(unit string) error {
	coeff, ok := types.UnitToCoeff(unit)
	if !ok {
		return fmt.Errorf("unsupported unit: %s", unit)
	}

	cmd := fmt.Sprintf("v01101 %s", coeff)
	resp, err := d.sendUnitCommand(cmd)
	if err != nil {
		return fmt.Errorf("send set unit command failed: %w", err)
	}

	if resp != "A" {
		return fmt.Errorf("set unit rejected by device: %s", resp)
	}

	for i := range d.channels {
		if d.channels[i].Index < d.pressureCount {
			d.channels[i].Unit = unit
		}
	}
	slog.Info("XY-DAQ unit set", "unit", unit, "coeff", coeff)
	return nil
}

// trimSpace 去除字符串首尾空白和换行
func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\r' || s[0] == '\n') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\r' || s[len(s)-1] == '\n') {
		s = s[:len(s)-1]
	}
	return s
}
