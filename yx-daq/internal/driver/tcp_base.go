package driver

import (
	"fmt"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"yx-daq/internal/types"
)

// TCPDriverBase TCP 驱动公共基座
type TCPDriverBase struct {
	mu             sync.Mutex
	Host           string
	Port           int
	Conn           net.Conn
	connected      atomic.Bool
	acquiring      atomic.Bool
	draining       atomic.Bool
	onData         types.DataCallback
	onStatusChange func() // 状态变更回调（断连/重连成功时通知 DeviceManager）
	RecvBuffer     []byte
	reconnectCount int
	stopReconnect  chan struct{}
	channels       []types.ChannelConfig
	CmdRespCh      chan []byte
	recvLoopRunning atomic.Bool
}

// NewTCPDriverBase 创建 TCP 驱动基座
func NewTCPDriverBase(host string, port int, channels []types.ChannelConfig) *TCPDriverBase {
	return &TCPDriverBase{
		Host:          host,
		Port:          port,
		channels:      channels,
		stopReconnect: make(chan struct{}),
		CmdRespCh:     make(chan []byte, 1),
	}
}

// SetDataCallback 设置数据回调
func (b *TCPDriverBase) SetDataCallback(cb types.DataCallback) {
	b.onData = cb
}

// SetOnStatusChange 设置状态变更回调（断连/重连成功/重连失败时触发）
func (b *TCPDriverBase) SetOnStatusChange(cb func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onStatusChange = cb
}

// getOnStatusChange 安全读取 onStatusChange 回调
func (b *TCPDriverBase) getOnStatusChange() func() {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.onStatusChange
}

// IsConnected 是否已连接
func (b *TCPDriverBase) IsConnected() bool {
	return b.connected.Load()
}

// IsAcquiring 是否采集中
func (b *TCPDriverBase) IsAcquiring() bool {
	return b.acquiring.Load()
}

// UpdateChannels 热更新通道配置
func (b *TCPDriverBase) UpdateChannels(channels []types.ChannelConfig) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.channels = channels
}

// GetChannels 返回当前通道配置副本
func (b *TCPDriverBase) GetChannels() []types.ChannelConfig {
	b.mu.Lock()
	defer b.mu.Unlock()
	channels := make([]types.ChannelConfig, len(b.channels))
	copy(channels, b.channels)
	return channels
}

// Channels 返回通道配置引用（内部使用，无需拷贝）
func (b *TCPDriverBase) Channels() []types.ChannelConfig {
	return b.channels
}

// DialConnect TCP 拨号连接
func (b *TCPDriverBase) DialConnect() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", b.Host, b.Port), 5*time.Second)
	if err != nil {
		return fmt.Errorf("connect to %s:%d failed: %w", b.Host, b.Port, err)
	}

	// 启用 KeepAlive
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(10 * time.Second)
	}

	b.Conn = conn
	b.connected.Store(true)
	b.reconnectCount = 0
	return nil
}

// CloseDisconnect 断开连接
func (b *TCPDriverBase) CloseDisconnect() {
	b.mu.Lock()
	defer b.mu.Unlock()

	// 非阻塞发送，避免在重连循环未运行时阻塞
	select {
	case b.stopReconnect <- struct{}{}:
	default:
	}
	b.acquiring.Store(false)
	b.connected.Store(false)

	if b.Conn != nil {
		b.Conn.Close()
		b.Conn = nil
	}
}

// StartReceiveLoop 启动数据接收协程
func (b *TCPDriverBase) StartReceiveLoop(processFunc func(data []byte)) {
	go b.receiveLoop(processFunc)
}

// receiveLoop 数据接收循环
func (b *TCPDriverBase) receiveLoop(processFunc func(data []byte)) {
	b.recvLoopRunning.Store(true)
	defer b.recvLoopRunning.Store(false)
	buf := make([]byte, 4096)
	for b.connected.Load() {
		n, err := b.Conn.Read(buf)
		if err != nil {
			if b.connected.Load() {
				b.connected.Store(false) // 先标记断连，确保 onStatusChange 回调读到正确状态
				slog.Error("TCP read error", "host", b.Host, "port", b.Port, "err", err)
				if cb := b.getOnStatusChange(); cb != nil {
					cb()
				}
				b.HandleDisconnect()
			}
			return
		}
		if n > 0 {
			b.RecvBuffer = append(b.RecvBuffer, buf[:n]...)
			processFunc(b.RecvBuffer)
		}
	}
}

// HandleDisconnect 处理断连（指数退避重连）
func (b *TCPDriverBase) HandleDisconnect() {
	b.connected.Store(false)
	b.acquiring.Store(false)

	for b.reconnectCount < types.MaxReconnectAttempts {
		delay := types.ReconnectBaseDelayMs * (1 << b.reconnectCount)
		if delay > types.ReconnectMaxDelayMs {
			delay = types.ReconnectMaxDelayMs
		}

		select {
		case <-b.stopReconnect:
			return
		case <-time.After(time.Duration(delay) * time.Millisecond):
		}

		b.reconnectCount++
		slog.Warn("TCP reconnecting", "host", b.Host, "port", b.Port, "attempt", b.reconnectCount, "max", types.MaxReconnectAttempts)

		if err := b.DialConnect(); err == nil {
			slog.Info("TCP reconnected successfully", "host", b.Host, "port", b.Port)
			if cb := b.getOnStatusChange(); cb != nil {
				cb()
			}
			return
		}
	}

	slog.Error("TCP max reconnect attempts reached", "host", b.Host, "port", b.Port)
	if cb := b.getOnStatusChange(); cb != nil {
		cb()
	}
}

// EmitData 发射数据到回调
func (b *TCPDriverBase) EmitData(payload types.DataPayload) {
	if b.onData != nil {
		b.onData(payload)
	}
}

// BuildDataPayload 构建数据载荷（映射到已启用通道）
func (b *TCPDriverBase) BuildDataPayload(values []float64, deviceID string) types.DataPayload {
	enabledValues := []float64{}
	enabledIndices := []int{}
	enabledUnits := []string{}
	for i, ch := range b.channels {
		if ch.Enabled && i < len(values) {
			enabledValues = append(enabledValues, values[i])
			enabledIndices = append(enabledIndices, i)
			enabledUnits = append(enabledUnits, ch.Unit)
		}
	}

	return types.DataPayload{
		DeviceID:       deviceID,
		Timestamp:      time.Now().UnixMilli(),
		Channels:       enabledValues,
		ChannelIndices: enabledIndices,
		ChannelUnits:   enabledUnits,
	}
}

// SendCommandDirect 直接发送命令并读取响应（用于 receiveLoop 未运行时）
func (b *TCPDriverBase) SendCommandDirect(cmd string, timeout time.Duration) (string, error) {
	if !b.connected.Load() || b.Conn == nil {
		return "", fmt.Errorf("device not connected")
	}

	if _, err := b.Conn.Write([]byte(cmd)); err != nil {
		return "", fmt.Errorf("send command %q failed: %w", cmd, err)
	}

	b.Conn.SetReadDeadline(time.Now().Add(timeout))
	buf := make([]byte, 1024)
	n, err := b.Conn.Read(buf)
	b.Conn.SetReadDeadline(time.Time{})
	if err != nil {
		return "", fmt.Errorf("read response for %q failed: %w", cmd, err)
	}

	return trimSpace(string(buf[:n])), nil
}

// SendCommandViaChannel 通过 cmdRespCh 发送命令并等待响应（用于 receiveLoop 运行时）
func (b *TCPDriverBase) SendCommandViaChannel(cmd string, timeout time.Duration) (string, error) {
	if !b.connected.Load() || b.Conn == nil {
		return "", fmt.Errorf("device not connected")
	}

	// 排空残留响应
	select {
	case <-b.CmdRespCh:
	default:
	}

	b.mu.Lock()
	if _, err := b.Conn.Write([]byte(cmd)); err != nil {
		b.mu.Unlock()
		return "", fmt.Errorf("send command %q failed: %w", cmd, err)
	}
	b.mu.Unlock()

	select {
	case payload := <-b.CmdRespCh:
		return string(payload), nil
	case <-time.After(timeout):
		return "", fmt.Errorf("command %q timeout", cmd)
	}
}

// WriteCommandOnly 仅写入命令，不等待响应
func (b *TCPDriverBase) WriteCommandOnly(cmd string) error {
	if !b.connected.Load() || b.Conn == nil {
		return fmt.Errorf("device not connected")
	}

	if _, err := b.Conn.Write([]byte(cmd)); err != nil {
		return fmt.Errorf("write command %q failed: %w", cmd, err)
	}
	return nil
}

// DrainConnection 排空连接中的残留数据
func (b *TCPDriverBase) DrainConnection(waitMs int) {
	time.Sleep(time.Duration(waitMs) * time.Millisecond)
	if b.Conn == nil {
		return
	}
	buf := make([]byte, 4096)
	for {
		b.Conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
		_, err := b.Conn.Read(buf)
		b.Conn.SetReadDeadline(time.Time{})
		if err != nil {
			break
		}
	}
	b.RecvBuffer = b.RecvBuffer[:0]
}

// ConsumeOptionalACK 消费可选的 ACK 响应
func (b *TCPDriverBase) ConsumeOptionalACK(timeoutMs int) {
	if b.Conn == nil {
		return
	}
	b.Conn.SetReadDeadline(time.Now().Add(time.Duration(timeoutMs) * time.Millisecond))
	buf := make([]byte, 1024)
	b.Conn.Read(buf)
	b.Conn.SetReadDeadline(time.Time{})
}

// RouteToCmdRespCh 将数据路由到命令响应通道（非阻塞）
func (b *TCPDriverBase) RouteToCmdRespCh(data []byte) {
	select {
	case b.CmdRespCh <- data:
	default:
	}
}
