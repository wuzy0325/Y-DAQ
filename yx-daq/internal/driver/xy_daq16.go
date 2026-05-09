package driver

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"yx-daq/internal/types"
)

// XYDAQDriver XY-DAQ TCP驱动（支持DAQ8/DAQ16）
type XYDAQDriver struct {
	mu             sync.Mutex
	host           string
	port           int
	streamID       int
	conn           net.Conn
	connected      atomic.Bool
	acquiring      atomic.Bool
	draining       atomic.Bool
	onData         types.DataCallback
	recvBuffer     []byte
	reconnectCount int
	stopReconnect  chan struct{}
	channels       []types.ChannelConfig
	pressureCount  int // 压力通道数（8或16）
	totalChannels  int // 总通道数（压力+大气压+大气温度）
	frameSize      int // 数据帧大小（字节）
	// 命令响应通道
	cmdRespCh           chan []byte
	recvLoopRunning     atomic.Bool
}

// NewXYDAQDriver 创建XY-DAQ驱动（DAQ8/DAQ16通用）
func NewXYDAQDriver(host string, port, streamID int, channels []types.ChannelConfig, deviceType types.DeviceType) *XYDAQDriver {
	pressureCount := deviceType.PressureChannelCount()
	totalChannels := deviceType.TotalChannelCount()
	return &XYDAQDriver{
		host:          host,
		port:          port,
		streamID:      streamID,
		channels:      channels,
		pressureCount: pressureCount,
		totalChannels: totalChannels,
		frameSize:     deviceType.StreamFrameSize(),
		stopReconnect: make(chan struct{}),
		cmdRespCh:     make(chan []byte, 1),
	}
}

// SetDataCallback 设置数据回调
func (d *XYDAQDriver) SetDataCallback(cb types.DataCallback) {
	d.onData = cb
}

// Connect 建立TCP连接
func (d *XYDAQDriver) Connect() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", d.host, d.port), 5*time.Second)
	if err != nil {
		return fmt.Errorf("connect to %s:%d failed: %w", d.host, d.port, err)
	}

	d.conn = conn
	d.connected.Store(true)
	d.reconnectCount = 0

	// 启用2字节长度前缀模式
	if _, err := conn.Write([]byte("w1601\r")); err != nil {
		conn.Close()
		return fmt.Errorf("send w1601 failed: %w", err)
	}
	time.Sleep(50 * time.Millisecond)

	// 读取设备EU单位并更新通道配置
	d.readAndUpdateEUUnit()

	// 启动数据接收协程
	go d.receiveLoop()

	return nil
}

// Disconnect 断开连接
func (d *XYDAQDriver) Disconnect() {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 非阻塞发送，避免在重连循环未运行时阻塞
	select {
	case d.stopReconnect <- struct{}{}:
	default:
	}
	d.acquiring.Store(false)
	d.connected.Store(false)

	if d.conn != nil {
		d.conn.Close()
		d.conn = nil
	}
}

// IsConnected 是否已连接
func (d *XYDAQDriver) IsConnected() bool {
	return d.connected.Load()
}

// IsAcquiring 是否采集中
func (d *XYDAQDriver) IsAcquiring() bool {
	return d.acquiring.Load()
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

	streamTag := fmt.Sprintf("%d", d.streamID)

	// 配置数据流参数: 内部时钟/大端/连续
	cmd1 := fmt.Sprintf("c 00 %s FFFF 1 %d 7 0\r", streamTag, periodMs)
	if _, err := d.conn.Write([]byte(cmd1)); err != nil {
		return fmt.Errorf("configure stream failed: %w", err)
	}
	time.Sleep(100 * time.Millisecond)

	// 配置返回内容: 压力+大气压+温度
	cmd2 := fmt.Sprintf("c 05 %s 0810\r", streamTag)
	if _, err := d.conn.Write([]byte(cmd2)); err != nil {
		return fmt.Errorf("configure stream content failed: %w", err)
	}
	time.Sleep(100 * time.Millisecond)

	// 启动数据流
	cmd3 := fmt.Sprintf("c 01 %s\r", streamTag)
	if _, err := d.conn.Write([]byte(cmd3)); err != nil {
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
	if _, err := d.conn.Write([]byte(cmd)); err != nil {
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

// UpdateChannels 热更新通道配置
func (d *XYDAQDriver) UpdateChannels(channels []types.ChannelConfig) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.channels = channels
}

// GetChannels 返回当前通道配置副本。
func (d *XYDAQDriver) GetChannels() []types.ChannelConfig {
	d.mu.Lock()
	defer d.mu.Unlock()
	channels := make([]types.ChannelConfig, len(d.channels))
	copy(channels, d.channels)
	return channels
}

// receiveLoop 数据接收循环
func (d *XYDAQDriver) receiveLoop() {
	d.recvLoopRunning.Store(true)
	defer d.recvLoopRunning.Store(false)
	buf := make([]byte, 4096)
	for d.connected.Load() {
		n, err := d.conn.Read(buf)
		if err != nil {
			if d.connected.Load() {
				slog.Error("XY-DAQ read error", "err", err)
				d.handleDisconnect()
			}
			return
		}
		if n > 0 {
			d.recvBuffer = append(d.recvBuffer, buf[:n]...)
			d.processBuffer()
		}
	}
}

// processBuffer 处理接收缓冲区（2字节长度前缀拆包）
func (d *XYDAQDriver) processBuffer() {
	for len(d.recvBuffer) >= 2 {
		// 2字节大端长度前缀
		frameLen := int(binary.BigEndian.Uint16(d.recvBuffer[:2]))
		if frameLen < 2 || len(d.recvBuffer) < frameLen {
			break
		}

		frame := d.recvBuffer[:frameLen]
		d.recvBuffer = d.recvBuffer[frameLen:]

		// 判断帧类型
		payload := frame[2:] // 去掉长度前缀
		if len(payload) > 0 && payload[0] < 0x20 {
			// 二进制帧
			d.handleStreamFrame(payload)
		} else {
			// ASCII帧路由到命令响应通道
			select {
			case d.cmdRespCh <- payload:
			default:
			}
		}
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
	if len(frame) < d.frameSize {
		return
	}

	values := make([]float64, d.totalChannels)
	for i := 0; i < d.totalChannels; i++ {
		offset := types.StreamFrameHeaderSize + i*4
		bits := binary.BigEndian.Uint32(frame[offset : offset+4])
		values[i] = float64(math.Float32frombits(bits))
	}

	// 反转压力通道顺序：硬件按 CHn→CH1 逆序发送，需反转为 CH1→CHn
	for i := 0; i < d.pressureCount/2; i++ {
		j := d.pressureCount - 1 - i
		values[i], values[j] = values[j], values[i]
	}

	// 映射到已启用通道
	enabledValues := []float64{}
	enabledIndices := []int{}
	enabledUnits := []string{}
	for i, ch := range d.channels {
		if ch.Enabled && i < len(values) {
			enabledValues = append(enabledValues, values[i])
			enabledIndices = append(enabledIndices, i)
			enabledUnits = append(enabledUnits, ch.Unit)
		}
	}

	payload := types.DataPayload{
		DeviceID:       fmt.Sprintf("%s:%d", d.host, d.port),
		Timestamp:      time.Now().UnixMilli(),
		Channels:       enabledValues,
		ChannelIndices: enabledIndices,
		ChannelUnits:   enabledUnits,
	}

	if d.onData != nil {
		d.onData(payload)
	}
}

// handleDisconnect 处理断连（指数退避重连）
func (d *XYDAQDriver) handleDisconnect() {
	d.connected.Store(false)
	d.acquiring.Store(false)

	for d.reconnectCount < types.MaxReconnectAttempts {
		delay := types.ReconnectBaseDelayMs * (1 << d.reconnectCount)
		if delay > types.ReconnectMaxDelayMs {
			delay = types.ReconnectMaxDelayMs
		}

		select {
		case <-d.stopReconnect:
			return
		case <-time.After(time.Duration(delay) * time.Millisecond):
		}

		d.reconnectCount++
		slog.Warn("XY-DAQ reconnecting", "attempt", d.reconnectCount, "max", types.MaxReconnectAttempts)

		if err := d.Connect(); err == nil {
			slog.Info("XY-DAQ reconnected successfully")
			return
		}
	}

	slog.Error("XY-DAQ max reconnect attempts reached")
}

// SendCommand 发送命令并等待ASCII响应（用于查询类命令）
// 注意：此方法在receiveLoop启动前调用，直接从conn读取响应
func (d *XYDAQDriver) SendCommand(cmd string) (string, error) {
	if !d.connected.Load() || d.conn == nil {
		return "", fmt.Errorf("device not connected")
	}

	// 发送命令
	if _, err := d.conn.Write([]byte(cmd + "\r")); err != nil {
		return "", fmt.Errorf("send command %q failed: %w", cmd, err)
	}

	// 读取响应（带超时）
	d.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1024)
	n, err := d.conn.Read(buf)
	d.conn.SetReadDeadline(time.Time{}) // 清除超时
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
	if !d.connected.Load() || d.conn == nil {
		return "", fmt.Errorf("device not connected")
	}

	// receiveLoop 运行时通过 cmdRespCh 获取响应，避免与 receiveLoop 竞争 conn.Read
	if d.recvLoopRunning.Load() {
		// 排空残留响应
		select {
		case <-d.cmdRespCh:
		default:
		}

		d.mu.Lock()
		if _, err := d.conn.Write([]byte(cmd)); err != nil {
			d.mu.Unlock()
			return "", fmt.Errorf("send unit command %q failed: %w", cmd, err)
		}
		d.mu.Unlock()

		select {
		case payload := <-d.cmdRespCh:
			return parseUnitPayload(payload)
		case <-time.After(3 * time.Second):
			return "", fmt.Errorf("unit command %q timeout", cmd)
		}
	}

	// receiveLoop 未运行时直接读写（Connect 初始化阶段）
	if _, err := d.conn.Write([]byte(cmd)); err != nil {
		return "", fmt.Errorf("send unit command %q failed: %w", cmd, err)
	}

	d.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 1024)
	n, err := d.conn.Read(buf)
	d.conn.SetReadDeadline(time.Time{})
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

	unit := coeffToUnit(resp)
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
	coeff, ok := unitToCoeff(unit)
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

// coeffToUnit EU转换系数/返回值 → 单位字符串
func coeffToUnit(raw string) string {
	raw = trimSpace(raw)

	switch raw {
	case "0":
		return "kgf/cm²"
	case "1":
		return "psi"
	case "6":
		return "kPa"
	case "6894", "6894.76":
		return "Pa"
	}

	var val float64
	if _, err := fmt.Sscanf(raw, "%f", &val); err != nil {
		return ""
	}

	type coeffUnit struct {
		coeff float64
		unit  string
	}
	table := []coeffUnit{
		{1, "psi"},
		{0.07031, "kgf/cm²"},
		{0.0689476, "bar"},
		{68.9476, "mbar"},
		{6.89476, "kPa"},
		{0.00689476, "MPa"},
		{6894.76, "Pa"},
		{51.7149, "mmHg"},
		{0.068046, "atm"},
	}

	for _, entry := range table {
		if math.Abs(val-entry.coeff)/entry.coeff < 0.01 {
			return entry.unit
		}
	}

	return ""
}

// unitToCoeff 单位字符串 → EU转换系数
func unitToCoeff(unit string) (string, bool) {
	switch unit {
	case "psi":
		return "1", true
	case "kgf/cm²":
		return "0.07031", true
	case "bar":
		return "0.0689476", true
	case "mbar":
		return "68.9476", true
	case "kPa":
		return "6.89476", true
	case "MPa":
		return "0.00689476", true
	case "Pa":
		return "6894.76", true
	case "mmHg":
		return "51.7149", true
	case "atm":
		return "0.068046", true
	default:
		return "", false
	}
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
