package driver

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"yx-daq/internal/types"
)

// DataCallback 数据回调函数类型
type DataCallback func(payload types.DataPayload)

// XYDAQ16Driver XY-DAQ16 TCP驱动
type XYDAQ16Driver struct {
	mu             sync.Mutex
	host           string
	port           int
	streamID       int
	conn           net.Conn
	connected      atomic.Bool
	acquiring      atomic.Bool
	onData         DataCallback
	recvBuffer     []byte
	reconnectCount int
	stopReconnect  chan struct{}
	channels       []types.ChannelConfig
	// 命令响应通道
	cmdRespCh      chan []byte
}

// NewXYDAQ16Driver 创建XY-DAQ16驱动
func NewXYDAQ16Driver(host string, port, streamID int, channels []types.ChannelConfig) *XYDAQ16Driver {
	return &XYDAQ16Driver{
		host:      host,
		port:      port,
		streamID:  streamID,
		channels:  channels,
		stopReconnect: make(chan struct{}),
		cmdRespCh: make(chan []byte, 1),
	}
}

// SetDataCallback 设置数据回调
func (d *XYDAQ16Driver) SetDataCallback(cb DataCallback) {
	d.onData = cb
}

// Connect 建立TCP连接
func (d *XYDAQ16Driver) Connect() error {
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
func (d *XYDAQ16Driver) Disconnect() {
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
func (d *XYDAQ16Driver) IsConnected() bool {
	return d.connected.Load()
}

// IsAcquiring 是否采集中
func (d *XYDAQ16Driver) IsAcquiring() bool {
	return d.acquiring.Load()
}

// StartAcquisition 启动采集
func (d *XYDAQ16Driver) StartAcquisition(periodMs int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.connected.Load() {
		return fmt.Errorf("device not connected")
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
func (d *XYDAQ16Driver) StopAcquisition() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.connected.Load() {
		return fmt.Errorf("device not connected")
	}

	streamTag := fmt.Sprintf("%d", d.streamID)
	cmd := fmt.Sprintf("c 02 %s\r", streamTag)
	if _, err := d.conn.Write([]byte(cmd)); err != nil {
		return fmt.Errorf("stop stream failed: %w", err)
	}

	d.acquiring.Store(false)
	return nil
}

// SendRawCommand 发送原始命令
func (d *XYDAQ16Driver) SendRawCommand(command string) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.connected.Load() {
		return "", fmt.Errorf("device not connected")
	}

	if _, err := d.conn.Write([]byte(command + "\r")); err != nil {
		return "", err
	}

	return "", nil
}

// UpdateChannels 热更新通道配置
func (d *XYDAQ16Driver) UpdateChannels(channels []types.ChannelConfig) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.channels = channels
}

// receiveLoop 数据接收循环
func (d *XYDAQ16Driver) receiveLoop() {
	buf := make([]byte, 4096)
	for d.connected.Load() {
		n, err := d.conn.Read(buf)
		if err != nil {
			if d.connected.Load() {
				log.Printf("XY-DAQ16 read error: %v", err)
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
func (d *XYDAQ16Driver) processBuffer() {
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
		}
		// ASCII帧忽略（命令响应）
	}
}

// handleStreamFrame 处理二进制数据流帧
func (d *XYDAQ16Driver) handleStreamFrame(frame []byte) {
	// 帧结构: 头(5B) + CH1(4B float32 BE) + ... + CH18(4B)
	if len(frame) < types.StreamFrameSize {
		return
	}

	values := make([]float64, types.MaxDaqChannels)
	for i := 0; i < types.MaxDaqChannels; i++ {
		offset := types.StreamFrameHeaderSize + i*4
		bits := binary.BigEndian.Uint32(frame[offset : offset+4])
		values[i] = float64(math.Float32frombits(bits))
	}

	// 映射到已启用通道
	enabledValues := []float64{}
	enabledIndices := []int{}
	for i, ch := range d.channels {
		if ch.Enabled && i < len(values) {
			enabledValues = append(enabledValues, values[i])
			enabledIndices = append(enabledIndices, i)
		}
	}

	payload := types.DataPayload{
		DeviceID:       fmt.Sprintf("%s:%d", d.host, d.port),
		Timestamp:      time.Now().UnixMilli(),
		Channels:       enabledValues,
		ChannelIndices: enabledIndices,
	}

	if d.onData != nil {
		d.onData(payload)
	}
}

// handleDisconnect 处理断连（指数退避重连）
func (d *XYDAQ16Driver) handleDisconnect() {
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
		log.Printf("XY-DAQ16 reconnecting... attempt %d/%d", d.reconnectCount, types.MaxReconnectAttempts)

		if err := d.Connect(); err == nil {
			log.Printf("XY-DAQ16 reconnected successfully")
			return
		}
	}

	log.Printf("XY-DAQ16 max reconnect attempts reached")
}

// SendCommand 发送命令并等待ASCII响应（用于查询类命令）
// 注意：此方法在receiveLoop启动前调用，直接从conn读取响应
func (d *XYDAQ16Driver) SendCommand(cmd string) (string, error) {
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

// readAndUpdateEUUnit 连接后读取设备EU单位并更新通道配置
func (d *XYDAQ16Driver) readAndUpdateEUUnit() {
	// 读取EU转换系数: u01101 → 返回EU压力转换系数
	// 系数索引02对应EU转换系数c0，其中包含单位信息
	// 先读取传感器1的量程代码: u0010A
	resp, err := d.SendCommand("u0010A")
	if err != nil {
		log.Printf("XY-DAQ16 read range code failed: %v", err)
		return
	}

	unit := rangeCodeToUnit(resp)
	if unit == "" {
		// 尝试通过EU系数推断单位
		resp2, err2 := d.SendCommand("u01102")
		if err2 != nil {
			log.Printf("XY-DAQ16 read EU coeff failed: %v", err2)
			return
		}
		unit = euCoeffToUnit(resp2)
	}

	if unit != "" {
		// 更新CH0-15的单位（大气压固定kPa，大气温度固定°C）
		for i := range d.channels {
			if d.channels[i].Index < 16 {
				d.channels[i].Unit = unit
			}
		}
		log.Printf("XY-DAQ16 EU unit from device: %s", unit)
	}
}

// rangeCodeToUnit 量程代码转单位
// 1604设备量程代码对应不同的压力范围和单位
func rangeCodeToUnit(code string) string {
	// 去除空白
	code = trimSpace(code)
	switch code {
	case "1", "01":
		return "psi"
	case "2", "02":
		return "kPa"
	case "3", "03":
		return "kPa"
	case "4", "04":
		return "kPa"
	case "5", "05":
		return "bar"
	case "6", "06":
		return "mbar"
	case "7", "07":
		return "Pa"
	case "8", "08":
		return "mmHg"
	case "9", "09":
		return "MPa"
	default:
		return ""
	}
}

// euCoeffToUnit 通过EU转换系数推断单位（备用方案）
func euCoeffToUnit(coeff string) string {
	coeff = trimSpace(coeff)
	// 如果无法从系数推断，返回空字符串，使用默认配置
	return ""
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
