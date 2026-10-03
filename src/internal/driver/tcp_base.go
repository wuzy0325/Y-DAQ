package driver

import (
	"fmt"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"yx-daq/internal/logger"
	"yx-daq/internal/types"
)

// TCPDriverBase TCP 驱动公共基座
type TCPDriverBase struct {
	mu              sync.Mutex
	Host            string
	Port            int
	Conn            net.Conn
	connected       atomic.Bool
	acquiring       atomic.Bool
	draining        atomic.Bool
	onData          types.DataCallback
	onStatusChange  func() // 状态变更回调（断连/重连成功时通知 DeviceManager）
	RecvBuffer      []byte
	reconnectCount  int
	stopReconnect   chan struct{}
	channels        []types.ChannelConfig
	CmdRespCh       chan []byte
	recvLoopRunning atomic.Bool
	// deviceID 配置中的设备 ID（仅用于通信日志可读性，由管理器的驱动工厂注入）
	deviceID string
	// userDisconnected 用户主动断开标记（CloseDisconnect 置位，终态不复位）。
	// stopReconnect 通道只能中断一次退避等待；拨号/hook 执行期间收不到信号，
	// 必须以此标志为准停止重连，避免用户断开后设备自行重连产生幽灵连接。
	userDisconnected atomic.Bool
	onReconnect      func() error // 重连成功后的初始化 hook（子类通过 SetOnReconnect 注册）
	onResumeAcquire  func() error // 重连成功后恢复采集的 hook（子类通过 SetOnResumeAcquire 注册）
	// beforeClose 关闭连接前的回调（子类注册发送设备停止命令，如 DAQT 的 @f1），
	// 避免设备保持推流状态、连接槽位释放缓慢
	beforeClose func(net.Conn)
	// recvLoopDone 当前接收循环的退出信号（StartReceiveLoop 创建，循环退出时 close）
	recvLoopDone chan struct{}
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

// SetDeviceID 设置设备 ID（仅用于通信日志；由驱动工厂在创建后注入）
func (b *TCPDriverBase) SetDeviceID(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.deviceID = id
}

// GetDeviceID 返回设备 ID
func (b *TCPDriverBase) GetDeviceID() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.deviceID
}

// commFields 组装通信日志公共字段
func (b *TCPDriverBase) commFields() []any {
	return []any{"device", b.GetDeviceID(), "host", b.Host, "port", b.Port}
}

// SetDataCallback 设置数据回调
// 必须加锁：receiveLoop goroutine 通过 EmitData 并发读取 b.onData
func (b *TCPDriverBase) SetDataCallback(cb types.DataCallback) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onData = cb
}

// getOnData 安全读取 onData 回调（与 getOnStatusChange 风格一致）
func (b *TCPDriverBase) getOnData() types.DataCallback {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.onData
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

// SetOnReconnect 注册重连后初始化 hook
// 子类在首次 Connect 成功后调用此方法注册，之后 HandleDisconnect 重连成功时会自动调用
// hook 内通常包含：发送设备初始化命令、读取配置、启动 receiveLoop
func (b *TCPDriverBase) SetOnReconnect(fn func() error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onReconnect = fn
}

// getOnReconnect 安全读取 onReconnect hook
func (b *TCPDriverBase) getOnReconnect() func() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.onReconnect
}

// SetOnResumeAcquire 注册采集恢复 hook
// 断连前正在采集（acquiring=true）时，重连成功后自动调用 hook 恢复采集；
// hook 返回错误仅记录并通过状态回调上报，不触发新一轮重连
func (b *TCPDriverBase) SetOnResumeAcquire(fn func() error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onResumeAcquire = fn
}

// getOnResumeAcquire 安全读取采集恢复 hook
func (b *TCPDriverBase) getOnResumeAcquire() func() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.onResumeAcquire
}

// SetOnBeforeClose 注册连接关闭前回调（子类用于发送设备停止命令，如 @f1）。
// 回调在 CloseDisconnect 中、conn 关闭前执行；实现必须自行设置写超时，
// 且失败不得阻塞断开流程（CloseDisconnect 会忽略回调结果）。
func (b *TCPDriverBase) SetOnBeforeClose(fn func(net.Conn)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.beforeClose = fn
}

// getOnBeforeClose 安全读取关闭前回调
func (b *TCPDriverBase) getOnBeforeClose() func(net.Conn) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.beforeClose
}

// connRef 加锁获取当前连接快照。
// receiveLoop / 重连 hook 等多 goroutine 环境下禁止裸读 b.Conn（与 CloseDisconnect
// 的 b.Conn = nil 存在竞态，可能 nil 解引用崩溃），必须通过本方法取快照后使用；
// 连接被并发关闭时操作返回错误，由调用方处理。
func (b *TCPDriverBase) connRef() net.Conn {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Conn
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

// CloseDisconnect 断开连接。
//
// 设备（DAQ-T）只允许单连接，历史问题：直接 conn.Close() 时若接收协程阻塞在
// Read 且机器安全软件 hook winsock，Close 可能永久阻塞 → 设备侧连接槽位泄漏
// → 之后所有重连被设备拒绝。整改（参考 wista 可用实现）：
//  1. 先执行子类注册的 beforeClose 回调（DAQT 发送 @f1 停止推流）；
//  2. CloseWrite 发送 FIN（不依赖读侧状态，通常立即返回），Close 放后台；
//  3. 等待接收循环退出，避免下一次 Connect 与旧循环并发读；
//  4. 留出设备处理 FIN、释放单连接槽位的时间，避免立即重连被拒。
func (b *TCPDriverBase) CloseDisconnect() {
	// 取连接快照，锁外关闭，避免持锁调用外部函数 Close
	conn := b.connRef()

	b.mu.Lock()
	// 非阻塞发送，避免在重连循环未运行时阻塞
	select {
	case b.stopReconnect <- struct{}{}:
	default:
	}
	// 标记用户主动断开：拨号/hook 执行期间的断开请求无法通过 stopReconnect
	// 通道送达，重连循环以本标志为准终止，避免设备自行重连产生幽灵连接
	b.userDisconnected.Store(true)
	b.acquiring.Store(false)
	b.connected.Store(false)
	b.Conn = nil
	b.mu.Unlock()

	if conn != nil {
		// 关闭前回调：子类发送设备停止命令（如 DAQT @f1），失败不影响断开
		if hook := b.getOnBeforeClose(); hook != nil {
			hook(conn)
		}
		// FIN 让设备立即感知并释放单连接槽位；Close 后台执行防止 UI 阻塞
		abortConnection(conn)
		// 设备（DAQ-T）接收 FIN 后释放连接槽位需要时间，立即重连会被拒绝
		// （wista 实机复现，等待 200ms 后重拨）
		time.Sleep(200 * time.Millisecond)
	}

	// 等旧接收循环完全退出，避免新旧循环并发读同一设备连接
	if !b.joinReceiveLoop(2 * time.Second) {
		slog.Warn("TCP receive loop did not exit during disconnect", "host", b.Host, "port", b.Port)
	}
}

// ResetUserDisconnected 清除用户主动断开标记。
// 仅由子类显式连接入口（Connect）调用；重连循环内部不得调用，
// 否则会清除用户在拨号期间新置的断开标记，产生幽灵重连。
func (b *TCPDriverBase) ResetUserDisconnected() {
	b.userDisconnected.Store(false)
}

// abortConnection 先 CloseWrite 发送 FIN（对端可立即释放资源，如单连接设备
// 的连接槽位），再在后台 Close。故障 Windows 机上挂起 Read 时 Close 可能
// 永久阻塞（安全软件 hook winsock），不能在调用方 goroutine 同步执行。
func abortConnection(conn net.Conn) {
	if conn == nil {
		return
	}
	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.CloseWrite()
	}
	go func() { _ = conn.Close() }()
}

// joinReceiveLoop 等待当前接收循环退出；timeout<=0 时立即返回当前状态
func (b *TCPDriverBase) joinReceiveLoop(timeout time.Duration) bool {
	b.mu.Lock()
	done := b.recvLoopDone
	b.mu.Unlock()
	if done == nil {
		return true
	}
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

// StartReceiveLoop 启动数据接收协程。
// 若旧接收循环尚未退出（异常路径），最多等待 2s 再启动新循环，
// 避免两个循环并发读同一连接导致字节流竞争、命令响应被抢读。
func (b *TCPDriverBase) StartReceiveLoop(processFunc func(data []byte)) {
	b.mu.Lock()
	oldDone := b.recvLoopDone
	done := make(chan struct{})
	b.recvLoopDone = done
	b.mu.Unlock()

	if oldDone != nil {
		select {
		case <-oldDone:
		case <-time.After(2 * time.Second):
			slog.Warn("TCP old receive loop still running, starting new loop", "host", b.Host, "port", b.Port)
		}
	}
	go b.receiveLoop(processFunc, done)
}

// receiveLoop 数据接收循环
func (b *TCPDriverBase) receiveLoop(processFunc func(data []byte), loopDone chan struct{}) {
	b.recvLoopRunning.Store(true)
	// defer 为 LIFO：退出时先复位 recvLoopRunning，再关闭 loopDone，
	// 保证 HandleDisconnect 被唤醒时旧接收循环已完全退出
	defer close(loopDone)
	defer b.recvLoopRunning.Store(false)
	// panic 退出时不能只记日志：否则 connected 残留 true，界面显示在线但
	// 所有命令超时且不会触发重连（假连接）。此处按读错误路径补发断连并重连。
	// 本 defer 最先执行：HandleDisconnect 在独立 goroutine 中等 loopDone 关闭后继续
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		logger.ReportPanic("tcp-receive-loop", r)
		if b.connected.Load() {
			b.connected.Store(false)
			logger.CommError("TCP receive loop panic", append(b.commFields(), "op", "receive-loop", "err", fmt.Sprint(r))...)
			if cb := b.getOnStatusChange(); cb != nil {
				cb()
			}
			go b.HandleDisconnect(loopDone)
		}
	}()
	buf := make([]byte, 4096)
	for b.connected.Load() {
		// 加锁取连接快照：与 CloseDisconnect 的 b.Conn = nil 竞态，禁止裸读
		conn := b.connRef()
		if conn == nil {
			break
		}
		n, err := conn.Read(buf)
		if err != nil {
			if b.connected.Load() {
				b.connected.Store(false) // 先标记断连，确保 onStatusChange 回调读到正确状态
				logger.CommError("TCP read error", append(b.commFields(), "op", "receive-loop", "err", err)...)
				if cb := b.getOnStatusChange(); cb != nil {
					cb()
				}
				// 在独立 goroutine 中执行重连：本协程立即返回并复位 recvLoopRunning，
				// 避免旧协程的 defer 在新接收循环启动后把 recvLoopRunning 覆盖回 false，
				// 导致 sendUnitCommand 误判接收循环未运行而直接读 conn、与新循环并发抢数据
				go b.HandleDisconnect(loopDone)
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
// 必须在独立 goroutine 中运行（receiveLoop 出错时启动）：先等待旧 receiveLoop
// 完全退出（recvLoopRunning 已复位），确保重连 hook 中的直读命令路径
// 不会与新 receiveLoop 并发读同一连接。
// 子类通过 SetOnReconnect 注册 hook 后，重连成功后会调用 hook 重跑初始化
// （如 w1601、配置同步、重启 receiveLoop）；若断连前正在采集且注册了
// SetOnResumeAcquire，重连成功后自动恢复采集。
func (b *TCPDriverBase) HandleDisconnect(loopDone chan struct{}) {
	defer logger.Recover("tcp-reconnect")
	if loopDone != nil {
		<-loopDone
	}

	// 关闭已断开的旧连接：Read 报错后 socket 可能仍处于半开状态占用设备
	// 单连接槽位。先 CloseWrite(FIN)+Close，并留出设备释放槽位的时间再拨号，
	// 否则立即重连会被设备拒绝（DAQ-T 实机复现）
	b.mu.Lock()
	stale := b.Conn
	b.Conn = nil
	b.mu.Unlock()
	if stale != nil {
		abortConnection(stale)
		time.Sleep(200 * time.Millisecond)
	}

	wasAcquiring := b.acquiring.Load()
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

		// 用户已主动断开（信号可能在拨号/hook 期间已产生），停止重连
		if b.userDisconnected.Load() {
			return
		}

		b.reconnectCount++
		slog.Warn("TCP reconnecting", "host", b.Host, "port", b.Port, "attempt", b.reconnectCount, "max", types.MaxReconnectAttempts)

		if err := b.DialConnect(); err == nil {
			// 用户在拨号等待期间主动断开：清理新连接并放弃重连
			if b.userDisconnected.Load() {
				b.closeConnLocked()
				b.connected.Store(false)
				return
			}
			// 清空旧连接残留的半帧数据，避免新连接数据帧错位导致数据流永久停摆
			b.mu.Lock()
			b.RecvBuffer = b.RecvBuffer[:0]
			b.mu.Unlock()

			// 重连成功后执行子类初始化 hook（如设备初始化命令、配置同步、重启 receiveLoop）
			if hook := b.getOnReconnect(); hook != nil {
				if err := hook(); err != nil {
					slog.Warn("TCP reconnect init hook failed", "host", b.Host, "port", b.Port, "err", err)
					// 防御性清理：无论 hook 内部是否清理过 Conn，此处确保连接被关闭、状态被重置，
					// 避免子类实现不一致导致 b.Conn 残留、b.connected 错误为 true。
					b.closeConnLocked()
					b.connected.Store(false)
					// hook 失败视为本次重连失败，继续下一轮重试
					continue
				}
			}
			slog.Info("TCP reconnected successfully", "host", b.Host, "port", b.Port)

			// 断连前正在采集：重连成功后自动恢复采集。
			// 恢复失败仅记录错误并通过状态回调上报（连接正常但未采集），由用户决定是否手动重启
			if wasAcquiring {
				if resume := b.getOnResumeAcquire(); resume != nil {
					if err := resume(); err != nil {
						slog.Error("TCP resume acquisition after reconnect failed", "host", b.Host, "port", b.Port, "err", err)
					}
				}
			}

			if cb := b.getOnStatusChange(); cb != nil {
				cb()
			}
			return
		}
	}

	logger.CommError("TCP max reconnect attempts reached",
		append(b.commFields(), "op", "reconnect", "attempts", types.MaxReconnectAttempts)...)
	if cb := b.getOnStatusChange(); cb != nil {
		cb()
	}
}

// closeConnLocked 关闭并清空当前连接（不修改 connected 标志）。
// FIN+Close 通过 abortConnection 执行：故障 Windows 机挂起 Read 时
// 同步 Close 可能永久阻塞，不能卡住调用方。
func (b *TCPDriverBase) closeConnLocked() {
	conn := b.connRef()
	b.mu.Lock()
	b.Conn = nil
	b.mu.Unlock()
	if conn != nil {
		abortConnection(conn)
	}
}

// EmitData 发射数据到回调
// 由 receiveLoop goroutine 调用，必须通过 getOnData 加锁读取，避免与 SetDataCallback 竞争
func (b *TCPDriverBase) EmitData(payload types.DataPayload) {
	if cb := b.getOnData(); cb != nil {
		cb(payload)
	}
}

// BuildDataPayload 构建数据载荷（映射到已启用通道）。
// 按 ch.Index 取值而非切片位置：values 以硬件通道号（= ChannelConfig.Index）为下标，
// 通道配置切片顺序不参与映射，避免配置顺序变化时通道串位。
func (b *TCPDriverBase) BuildDataPayload(values []float64, deviceID string) types.DataPayload {
	enabledValues := []float64{}
	enabledIndices := []int{}
	enabledUnits := []string{}
	for _, ch := range b.channels {
		if !ch.Enabled || ch.Index < 0 || ch.Index >= len(values) {
			continue
		}
		enabledValues = append(enabledValues, values[ch.Index])
		enabledIndices = append(enabledIndices, ch.Index)
		enabledUnits = append(enabledUnits, ch.Unit)
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
	if !b.connected.Load() {
		return "", fmt.Errorf("device not connected")
	}
	conn := b.connRef()
	if conn == nil {
		return "", fmt.Errorf("device not connected")
	}

	if _, err := conn.Write([]byte(cmd)); err != nil {
		logger.CommError("TCP send command failed",
			append(b.commFields(), "op", "send-command", "cmd", logger.Truncate(cmd, 256), "err", err)...)
		return "", fmt.Errorf("send command %q failed: %w", cmd, err)
	}

	conn.SetReadDeadline(time.Now().Add(timeout))
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	conn.SetReadDeadline(time.Time{})
	if err != nil {
		logger.CommError("TCP read response failed",
			append(b.commFields(), "op", "send-command", "cmd", logger.Truncate(cmd, 256), "err", err)...)
		return "", fmt.Errorf("read response for %q failed: %w", cmd, err)
	}

	return trimSpace(string(buf[:n])), nil
}

// SendCommandViaChannel 通过 cmdRespCh 发送命令并等待响应（用于 receiveLoop 运行时）
func (b *TCPDriverBase) SendCommandViaChannel(cmd string, timeout time.Duration) (string, error) {
	if !b.connected.Load() {
		return "", fmt.Errorf("device not connected")
	}
	conn := b.connRef()
	if conn == nil {
		return "", fmt.Errorf("device not connected")
	}

	// 排空残留响应
	select {
	case <-b.CmdRespCh:
	default:
	}

	if _, err := conn.Write([]byte(cmd)); err != nil {
		logger.CommError("TCP send command failed",
			append(b.commFields(), "op", "send-command-channel", "cmd", logger.Truncate(cmd, 256), "err", err)...)
		return "", fmt.Errorf("send command %q failed: %w", cmd, err)
	}

	select {
	case payload := <-b.CmdRespCh:
		return string(payload), nil
	case <-time.After(timeout):
		logger.CommError("TCP command timeout",
			append(b.commFields(), "op", "send-command-channel", "cmd", logger.Truncate(cmd, 256), "timeout_ms", timeout.Milliseconds())...)
		return "", fmt.Errorf("command %q timeout", cmd)
	}
}

// WriteCommandOnly 仅写入命令，不等待响应
func (b *TCPDriverBase) WriteCommandOnly(cmd string) error {
	if !b.connected.Load() {
		return fmt.Errorf("device not connected")
	}
	conn := b.connRef()
	if conn == nil {
		return fmt.Errorf("device not connected")
	}

	if _, err := conn.Write([]byte(cmd)); err != nil {
		logger.CommError("TCP write command failed",
			append(b.commFields(), "op", "write-only", "cmd", logger.Truncate(cmd, 256), "err", err)...)
		return fmt.Errorf("write command %q failed: %w", cmd, err)
	}
	return nil
}

// DrainConnection 排空连接中的残留数据
func (b *TCPDriverBase) DrainConnection(waitMs int) {
	time.Sleep(time.Duration(waitMs) * time.Millisecond)
	conn := b.connRef()
	if conn == nil {
		return
	}
	buf := make([]byte, 4096)
	for {
		conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
		_, err := conn.Read(buf)
		conn.SetReadDeadline(time.Time{})
		if err != nil {
			break
		}
	}
	b.RecvBuffer = b.RecvBuffer[:0]
}

// ConsumeOptionalACK 消费可选的 ACK 响应
func (b *TCPDriverBase) ConsumeOptionalACK(timeoutMs int) {
	conn := b.connRef()
	if conn == nil {
		return
	}
	conn.SetReadDeadline(time.Now().Add(time.Duration(timeoutMs) * time.Millisecond))
	buf := make([]byte, 1024)
	conn.Read(buf)
	conn.SetReadDeadline(time.Time{})
}

// RouteToCmdRespCh 将数据路由到命令响应通道（非阻塞）
func (b *TCPDriverBase) RouteToCmdRespCh(data []byte) {
	select {
	case b.CmdRespCh <- data:
	default:
	}
}
