package driver

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"yx-daq/internal/types"
)

// YXDAQTDriver DAQ-T-1603 热电偶采集设备驱动
// 嵌入 TCPDriverBase 复用连接/重连/接收循环等通用逻辑
// 组合 FrameParser 策略实现可替换的帧解析
type YXDAQTDriver struct {
	*TCPDriverBase
	frameReader     *DAQTFrameReader
	frameParser     FrameParser
	hwConfig        DAQTHardwareConfig
	configSyncDone  bool
	configSyncCond  *sync.Cond // 配置同步完成条件变量
	pending         *PendingResponses
	respBuffer      []byte          // accumulates response bytes between dispatches
	silenceTimer    *time.Timer     // for variable-length silence window
	onConfigSynced  func(DAQTHardwareConfig) // 配置同步完成回调
}

// NewYXDAQTDriver 创建 DAQ-T-1603 驱动
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

// Connect 建立 TCP 连接
func (d *YXDAQTDriver) Connect() error {
	if err := d.DialConnect(); err != nil {
		return err
	}

	d.configSyncDone = false

	// 延迟后自动执行配置同步
	go func() {
		time.Sleep(time.Duration(types.DAQTConfigSyncDelayMs) * time.Millisecond)
		d.syncHardwareConfig()
	}()

	// 启动数据接收协程
	d.StartReceiveLoop(d.processData)

	return nil
}

// Disconnect 断开连接
func (d *YXDAQTDriver) Disconnect() {
	d.CloseDisconnect()
}

// StartAcquisition 启动采集
func (d *YXDAQTDriver) StartAcquisition(periodMs int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.connected.Load() {
		return fmt.Errorf("device not connected")
	}
	if d.acquiring.Load() {
		return nil
	}

	// 等待配置同步完成（带超时保护，避免因丢失唤醒或设备无响应导致 UI 永久卡死）
	const configSyncWaitTimeout = 10 * time.Second
	deadline := time.Now().Add(configSyncWaitTimeout)
	timer := time.AfterFunc(configSyncWaitTimeout, func() {
		d.mu.Lock()
		d.configSyncCond.Broadcast() // 超时强制唤醒，打破可能的丢失唤醒
		d.mu.Unlock()
	})
	defer timer.Stop()

	for !d.configSyncDone {
		if !d.connected.Load() {
			return fmt.Errorf("device disconnected during config sync")
		}
		if time.Now().After(deadline) {
			slog.Warn("DAQ-T config sync timeout, abort start acquisition", "host", d.Host, "port", d.Port)
			return fmt.Errorf("配置同步超时，请重新连接设备后再开始采集")
		}
		d.configSyncCond.Wait()
	}

	// 配置归一化
	if err := d.applyNormalizedConfig(); err != nil {
		return fmt.Errorf("apply normalized config failed: %w", err)
	}

	// 启动前准备
	d.writeCmdOnly("@f1")
	time.Sleep(100 * time.Millisecond)
	d.DrainConnection(100)
	d.frameReader.Reset()
	d.frameReader.SetBinaryMode(d.hwConfig.BinaryFormat)
	d.frameReader.SetMetadataMode(d.hwConfig.ShowTimestamp || d.hwConfig.ShowSequence)

	// 发送开始采集命令
	if err := d.writeCmdOnly("@f0 FFFF 2"); err != nil {
		return fmt.Errorf("start acquisition failed: %w", err)
	}

	d.ConsumeOptionalACK(types.DAQTACKTimeoutMs)
	d.acquiring.Store(true)
	return nil
}

// StopAcquisition 停止采集
func (d *YXDAQTDriver) StopAcquisition() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.connected.Load() {
		return fmt.Errorf("device not connected")
	}
	if !d.acquiring.Load() {
		return nil
	}

	d.acquiring.Store(false)
	d.draining.Store(true)
	d.writeCmdOnly("@f1")

	go func() {
		time.Sleep(200 * time.Millisecond)
		d.draining.Store(false)
	}()

	d.frameReader.Reset()
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
	if d.draining.Load() {
		d.RecvBuffer = d.RecvBuffer[:0]
		return
	}

	if !d.acquiring.Load() {
		// Non-acquiring mode: route to response handler
		d.handleCommandResponse(data)
		d.RecvBuffer = d.RecvBuffer[:0]
		return
	}

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

		deviceID := fmt.Sprintf("%s:%d", d.Host, d.Port)
		d.EmitData(d.BuildDataPayload(values, deviceID))
	}
}

// handleCommandResponse processes received data as a command response
func (d *YXDAQTDriver) handleCommandResponse(data []byte) {
	d.respBuffer = append(d.respBuffer, data...)

	// Remove expired entries
	for _, expired := range d.pending.RemoveExpired() {
		expired.RespCh <- "" // signal timeout
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
					entry.RespCh <- trimSpace(resp)
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
			entry.RespCh <- trimSpace(resp)

		case ResponseSilenceWindow:
			// Reset silence timer on each data arrival
			if d.silenceTimer != nil {
				d.silenceTimer.Stop()
			}
			d.silenceTimer = time.AfterFunc(30*time.Millisecond, func() {
				if d.pending.IsEmpty() {
					return
				}
				entry := d.pending.Pop()
				if entry != nil {
					resp := string(d.respBuffer)
					d.respBuffer = d.respBuffer[:0]
					entry.RespCh <- trimSpace(resp)
				}
			})
			return // 静默窗口模式不能立即处理，等待定时器
		}
	}
}
