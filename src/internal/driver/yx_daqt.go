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

// YXDAQTDriver EA2516T 热电偶采集设备驱动
// 嵌入 TCPDriverBase 复用连接/重连/接收循环等通用逻辑
// 组合 FrameParser 策略实现可替换的帧解析
type YXDAQTDriver struct {
	*TCPDriverBase
	frameReader    *DAQTFrameReader
	frameParser    FrameParser
	hwConfig       DAQTHardwareConfig
	configSyncDone bool
	configSyncCond *sync.Cond // 配置同步完成条件变量
	pending        *PendingResponses
	respBuffer     []byte                   // accumulates response bytes between dispatches
	silenceTimer   *time.Timer              // for variable-length silence window
	lastPeriodMs   atomic.Int32             // 最近一次采集周期（重连恢复采集时使用）
	onConfigSynced func(DAQTHardwareConfig) // 配置同步完成回调
}

// NewYXDAQTDriver 创建 EA2516T 驱动
func NewYXDAQTDriver(host string, port int, channels []types.ChannelConfig) *YXDAQTDriver {
	d := &YXDAQTDriver{
		TCPDriverBase: NewTCPDriverBase(host, port, channels),
		frameReader:   NewDAQTFrameReader(),
		frameParser:   &DAQTBinaryParser{}, // 默认，syncHardwareConfig 后会重新选择
		pending:       NewPendingResponses(),
	}
	d.configSyncCond = sync.NewCond(&d.mu)
	return d
}

// Connect 建立TCP连接
func (d *YXDAQTDriver) Connect() error {
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
	// 断开/重连前先发 @f1 停止设备推流：设备 TCP 连接后自动持续流数据，
	// 不停止会让设备保持推流状态，且关闭连接后设备侧单连接槽位释放变慢
	d.SetOnBeforeClose(func(conn net.Conn) {
		_ = conn.SetWriteDeadline(time.Now().Add(500 * time.Millisecond))
		_, _ = conn.Write([]byte("@f1"))
		_ = conn.SetWriteDeadline(time.Time{})
	})
	return d.initAfterConnect()
}

// initAfterConnect 连接建立后的初始化逻辑（首次连接与重连均调用）
func (d *YXDAQTDriver) initAfterConnect() error {
	d.mu.Lock()
	d.configSyncDone = false
	// 清空旧连接残留的帧缓冲/响应缓冲，避免新连接数据错位（与 StopAcquisition 的清理对称）
	d.frameReader.Reset()
	d.respBuffer = d.respBuffer[:0]
	if d.silenceTimer != nil {
		d.silenceTimer.Stop()
		d.silenceTimer = nil
	}
	d.mu.Unlock()

	// 清空旧连接遗留的 pending 响应期望：断连时未完成的命令响应不会再到达，
	// 残留 entry 会被新连接的首个响应误匹配（队列错位）；用空响应唤醒等待方
	for _, entry := range d.pending.Clear() {
		select {
		case entry.RespCh <- "":
		default:
		}
	}

	// 设备 TCP 连接后自动持续流数据，必须先用 @f1 停止，
	// 否则后续 syncHardwareConfig 的命令响应会被数据帧污染。
	// 停止命令失败仅记录警告：设备可能本就处于停止态，连接不应因此失败
	if err := d.writeCmdOnly("@f1"); err != nil {
		slog.Warn("DAQ-T: 连接后发送 @f1 停止命令失败", "host", d.Host, "port", d.Port, "err", err)
	}

	// 启动数据接收协程
	d.StartReceiveLoop(d.processData)

	// 延迟后自动执行配置同步（此时设备已停止流数据，响应纯净）
	go func() {
		time.Sleep(time.Duration(types.DAQTConfigSyncDelayMs) * time.Millisecond)
		d.syncHardwareConfig()
	}()

	return nil
}

// Disconnect 断开连接
func (d *YXDAQTDriver) Disconnect() {
	d.CloseDisconnect()
}

// StartAcquisition 启动采集
//
// 锁约束：writeCmdOnly/sendCommandACK 内部会获取 d.mu（base 的 connRef），
// Go 的 sync.Mutex 不可重入，因此本方法在发送任何命令时必须不持有 d.mu，
// 仅在读写共享状态时短暂持锁（参考 wista 可用实现的状态锁/写路径分离）。
func (d *YXDAQTDriver) StartAcquisition(periodMs int) error {
	if !d.connected.Load() {
		return fmt.Errorf("device not connected")
	}
	if d.acquiring.Load() {
		return nil
	}

	// 记录采集周期，断连重连后用于恢复采集
	d.lastPeriodMs.Store(int32(periodMs))

	// 等待配置同步完成（syncHardwareConfig 异步执行）。
	// sync.Cond.Wait 会释放 d.mu，不能用 defer 持锁整个函数
	const configSyncWaitTimeout = 10 * time.Second
	d.mu.Lock()
	deadline := time.Now().Add(configSyncWaitTimeout)
	timer := time.AfterFunc(configSyncWaitTimeout, func() {
		d.mu.Lock()
		d.configSyncCond.Broadcast()
		d.mu.Unlock()
	})
	defer timer.Stop()

	for !d.configSyncDone {
		if !d.connected.Load() {
			d.mu.Unlock()
			return fmt.Errorf("device disconnected during config sync")
		}
		if time.Now().After(deadline) {
			d.mu.Unlock()
			slog.Warn("DAQ-T config sync timeout, abort start acquisition", "host", d.Host, "port", d.Port)
			return fmt.Errorf("配置同步超时，请重新连接设备后再开始采集")
		}
		d.configSyncCond.Wait()
	}
	d.mu.Unlock()

	// 锁外下发归一化配置（@fe 命令带 ACK 消费，避免 ACK 字节污染 @f0 数据边界）
	if err := d.applyNormalizedConfig(periodMs); err != nil {
		return fmt.Errorf("apply normalized config failed: %w", err)
	}

	if !d.connected.Load() {
		return fmt.Errorf("device disconnected during config sync")
	}

	d.mu.Lock()
	d.frameReader.Reset()
	d.frameReader.SetBinaryMode(d.hwConfig.BinaryFormat)
	d.frameReader.SetMetadataMode(d.hwConfig.ShowTimestamp, d.hwConfig.ShowSequence)
	d.mu.Unlock()

	// 发送开始采集命令。
	// 用 writeCmdOnly 不注册 pending entry：设备固件在 @f0 后并行发送 ACK 与数据流，
	// 顺序不保证（约 10~15% 数据帧先到），若注册 pending 会导致数据帧被误消费为 ACK。
	// 迟到的 ACK 'A'(0x41) 由 FrameReader 的偏移对齐机制自动丢弃（见 hasAlignedFixedFrame）。
	if err := d.writeCmdOnly("@f0 FFFF 2"); err != nil {
		return fmt.Errorf("start acquisition failed: %w", err)
	}

	// 等待设备开始流数据。此时 acquiring=false，数据走 handleCommandResponse，
	// 但 pending 为空会被直接清空 respBuffer。
	// 切换到采集模式后，FrameReader 的偏移对齐会处理迟到的 ACK。
	time.Sleep(150 * time.Millisecond)

	d.mu.Lock()
	d.frameReader.Reset()
	d.respBuffer = d.respBuffer[:0]
	d.acquiring.Store(true)
	d.mu.Unlock()
	return nil
}

// StopAcquisition 停止采集
// 锁约束同 StartAcquisition：写命令必须在锁外执行
func (d *YXDAQTDriver) StopAcquisition() error {
	if !d.connected.Load() {
		return fmt.Errorf("device not connected")
	}
	if !d.acquiring.Load() {
		return nil
	}

	// 先发硬件停止命令，否则设备会持续流数据，导致后续响应被污染
	// 命令失败仍继续清理本地状态，避免 UI 卡死
	if err := d.writeCmdOnly("@f1"); err != nil {
		slog.Warn("DAQ-T: 停止采集命令 @f1 发送失败", "host", d.Host, "port", d.Port, "err", err)
	}

	// 切换到非采集模式，让后续到达的数据走 handleCommandResponse
	d.mu.Lock()
	d.acquiring.Store(false)
	d.mu.Unlock()

	// 静默窗口确认数据流停止：@f1 发送后设备可能仍在发送已排队的帧 + ACK。
	// 等待 150ms 静默（无新数据到达），确认停止完成，再清空缓冲区，
	// 避免残留帧/ACK 字节污染后续命令响应。
	time.Sleep(150 * time.Millisecond)

	d.mu.Lock()
	d.frameReader.Reset()
	d.respBuffer = d.respBuffer[:0]
	if d.silenceTimer != nil {
		d.silenceTimer.Stop()
		d.silenceTimer = nil
	}
	d.mu.Unlock()
	return nil
}

// GetHardwareConfig 获取硬件配置（只读）
func (d *YXDAQTDriver) GetHardwareConfig() DAQTHardwareConfig {
	return d.hwConfig
}

// OnConfigSynced 注册配置同步完成回调
func (d *YXDAQTDriver) OnConfigSynced(cb func(DAQTHardwareConfig)) {
	d.onConfigSynced = cb
}

// processData DAQ-T 特有的数据处理（帧读取器 + 策略解析器）
func (d *YXDAQTDriver) processData(data []byte) {
	if !d.acquiring.Load() {
		// Non-acquiring mode: route to response handler
		d.handleCommandResponse(data)
		d.RecvBuffer = d.RecvBuffer[:0]
		return
	}

	// frameReader 必须在 d.mu 内操作：StopAcquisition/StartAcquisition 会在锁内
	// Reset，无锁访问会与 Reset 竞态（slice 越界 panic）。
	// 解析结果收集到锁外投递：EmitData 内部会取同一把锁（getOnData），持锁调用会自锁。
	deviceID := fmt.Sprintf("%s:%d", d.Host, d.Port)
	var payloads []types.DataPayload
	d.mu.Lock()
	d.frameReader.Feed(data)
	d.RecvBuffer = d.RecvBuffer[:0]

	for d.frameReader.HasCompleteFrame() {
		frame := d.frameReader.ReadFrame()
		if frame == nil {
			continue
		}

		// 使用策略模式解析帧
		values, err := d.frameParser.Parse(frame)
		if err != nil {
			slog.Warn("DAQ-T frame parse error", "err", err)
			continue
		}

		if !isValidDAQTFrame(values) {
			continue
		}

		payloads = append(payloads, d.BuildDataPayload(values, deviceID))
	}
	d.mu.Unlock()

	for _, payload := range payloads {
		d.EmitData(payload)
	}
}

// handleCommandResponse processes received data as a command response
// 由 receiveLoop goroutine 调用。持锁阶段只解析 respBuffer/pending/silenceTimer，
// 完成的响应收集到 sends 后在锁外统一投递（dispatchPending），
// 避免持有 d.mu 时阻塞发送 channel 造成死锁
func (d *YXDAQTDriver) handleCommandResponse(data []byte) {
	var sends []pendingDispatch
	d.mu.Lock()
	defer func() {
		d.mu.Unlock()
		dispatchPending(sends)
	}()

	d.respBuffer = append(d.respBuffer, data...)

	// Remove expired entries（超时信号在锁外投递）
	for _, expired := range d.pending.RemoveExpired() {
		sends = append(sends, pendingDispatch{entry: expired})
	}

	// 循环处理缓冲区中的所有完整响应，避免递归导致栈溢出
	for {
		if d.pending.IsEmpty() {
			d.respBuffer = d.respBuffer[:0]
			return
		}

		front := d.pending.Front()
		if front == nil {
			d.respBuffer = d.respBuffer[:0]
			return
		}

		switch front.RespType {
		case ResponseNewline:
			// Check if buffer contains \n
			found := false
			for i, b := range d.respBuffer {
				if b == '\n' {
					resp := string(d.respBuffer[:i])
					d.respBuffer = d.respBuffer[i+1:]
					entry := d.pending.Pop()
					sends = append(sends, pendingDispatch{entry: entry, resp: trimSpace(resp)})
					found = true
					break
				}
			}
			if !found {
				return // 等待更多数据
			}

		case ResponseFixedLength:
			if len(d.respBuffer) < front.ExpectedLen {
				return // 等待更多数据
			}
			resp := string(d.respBuffer[:front.ExpectedLen])
			d.respBuffer = d.respBuffer[front.ExpectedLen:]
			entry := d.pending.Pop()
			sends = append(sends, pendingDispatch{entry: entry, resp: trimSpace(resp)})

		case ResponseSilenceWindow:
			// Reset silence timer on each data arrival
			if d.silenceTimer != nil {
				d.silenceTimer.Stop()
			}
			d.silenceTimer = time.AfterFunc(30*time.Millisecond, func() {
				var tsends []pendingDispatch
				d.mu.Lock()
				if !d.pending.IsEmpty() {
					entry := d.pending.Pop()
					if entry != nil {
						resp := string(d.respBuffer)
						d.respBuffer = d.respBuffer[:0]
						tsends = append(tsends, pendingDispatch{entry: entry, resp: trimSpace(resp)})
					}
				}
				d.mu.Unlock()
				dispatchPending(tsends)
			})
			return // 静默窗口模式不能立即处理，等待定时器
		}
	}
}
