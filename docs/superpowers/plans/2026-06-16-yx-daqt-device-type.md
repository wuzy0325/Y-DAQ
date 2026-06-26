# YX-DAQ-T (DAQ-T-1603) 热电偶采集设备 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增 YX-DAQ-T (DAQ-T-1603) 热电偶温度采集设备类型，支持 ASCII 文本命令协议（@e3/@f0/@f1/@f3/@fd/@fe）、TCP 二进制/ASCII 数据帧解析、配置同步和完整生命周期管理。同时重构现有代码，消除 OCP 违反和重复代码。

**Architecture:** 采用三项 OOP 改进：(1) **设备类型注册表** — 用 `DeviceTypeInfo` 数据结构替代 switch/if，新增设备类型只需加一行注册；(2) **TCP 驱动基座** — 提取 `TCPDriverBase` 消除 XY-DAQ 和 DAQ-T 驱动间的 ~150 行重复代码；(3) **帧解析策略接口** — `FrameParser` 接口解耦解析逻辑，可独立测试和替换。DAQ-T 驱动嵌入 `TCPDriverBase`，组合 `FrameParser` 策略，实现 `DeviceDriver` 接口。

**Tech Stack:** Go 1.23 (后端驱动)、Vue 3 + TypeScript + Element Plus (前端)、Wails v3 (桌面框架)

---

## 文件结构

| 操作 | 文件路径 | 职责 |
|------|----------|------|
| 修改 | `yx-daq/internal/types/device.go` | 注册表替代 switch，新增 DeviceTypeInfo |
| 修改 | `yx-daq/internal/types/constants.go` | 新增 DAQ-T 相关常量 |
| 创建 | `yx-daq/internal/driver/tcp_base.go` | TCP 驱动公共基座（提取自 xy_daq16.go） |
| 修改 | `yx-daq/internal/driver/xy_daq16.go` | 重构为嵌入 TCPDriverBase |
| 创建 | `yx-daq/internal/driver/frame_parser.go` | FrameParser 策略接口及实现 |
| 创建 | `yx-daq/internal/driver/yx_daqt.go` | DAQ-T 驱动核心（嵌入 TCPDriverBase） |
| 创建 | `yx-daq/internal/driver/yx_daqt_config.go` | DAQ-T 配置同步与命令系统 |
| 创建 | `yx-daq/internal/driver/yx_daqt_test.go` | DAQ-T 驱动 + 帧解析单元测试 |
| 创建 | `yx-daq/internal/driver/tcp_base_test.go` | TCPDriverBase 单元测试 |
| 修改 | `yx-daq/internal/manager/device_manager.go` | 用注册表驱动工厂替代 switch |
| 修改 | `yx-daq/internal/scanner/daq_scanner.go` | 支持 DAQ-T 设备发现 |
| 修改 | `yx-daq/frontend/src/api/enums.ts` | 新增 YX_DAQT 设备类型枚举 |
| 修改 | `yx-daq/frontend/src/views/DeviceView.vue` | 新增 YX-DAQ-T 设备类型选项及温度通道配置 |

---

### Task 1: 设备类型注册表（替代 switch 方法）

**Files:**
- 修改: `yx-daq/internal/types/device.go`
- 修改: `yx-daq/internal/types/constants.go`

**设计动机：** 当前每新增设备类型需修改 `PressureChannelCount()`、`TotalChannelCount()`、`StreamFrameSize()`、`IsDAQDevice()` 等 4+ 个 switch 方法，违反开闭原则。用注册表替代后，新增设备类型只需在 registry 加一行。

- [ ] **Step 1: 重写 device.go — 新增 DeviceTypeInfo 注册表**

```go
package types

// DeviceType 设备类型标识
type DeviceType string

const (
	DeviceTypeSimulated DeviceType = "SIMULATED"
	DeviceTypeXYDAQ8    DeviceType = "XY-DAQ8"
	DeviceTypeXYDAQ16   DeviceType = "XY-DAQ16"
	DeviceTypeYXDAQT    DeviceType = "YX-DAQ-T"
)

// DeviceTypeInfo 设备类型元数据（注册表驱动，新增设备类型只需加一行）
type DeviceTypeInfo struct {
	Type            DeviceType
	Label           string
	PressureChCount int  // 压力/主通道数
	TotalChCount    int  // 总通道数
	FrameSize       int  // 数据帧大小（0 = 驱动自定义）
	IsTemperature   bool // 是否为温度采集设备
	IsRealDAQ       bool // 是否为真实 DAQ 设备（非模拟）
	DefaultHost     string
	DefaultPort     int
	DefaultUnit     string // 主通道默认单位
}

// deviceTypeRegistry 设备类型注册表 — 新增设备类型只需在此添加一行
var deviceTypeRegistry = map[DeviceType]DeviceTypeInfo{
	DeviceTypeXYDAQ8: {
		Type: "XY-DAQ8", Label: "XY-DAQ8",
		PressureChCount: 8, TotalChCount: 10, FrameSize: 45,
		IsRealDAQ: true, DefaultHost: "192.168.3.101", DefaultPort: 9000, DefaultUnit: "kPa",
	},
	DeviceTypeXYDAQ16: {
		Type: "XY-DAQ16", Label: "XY-DAQ16",
		PressureChCount: 16, TotalChCount: 18, FrameSize: 77,
		IsRealDAQ: true, DefaultHost: "192.168.3.101", DefaultPort: 9000, DefaultUnit: "kPa",
	},
	DeviceTypeYXDAQT: {
		Type: "YX-DAQ-T", Label: "DAQ-T-1603",
		PressureChCount: 16, TotalChCount: 16, FrameSize: 0,
		IsTemperature: true, IsRealDAQ: true,
		DefaultHost: "192.168.1.7", DefaultPort: 9000, DefaultUnit: "°C",
	},
	DeviceTypeSimulated: {
		Type: "SIMULATED", Label: "模拟设备",
		PressureChCount: 16, TotalChCount: 18, FrameSize: 77,
		DefaultUnit: "kPa",
	},
}

// Info 返回该设备类型的元数据（注册表查询，无需 switch）
func (t DeviceType) Info() DeviceTypeInfo {
	if info, ok := deviceTypeRegistry[t]; ok {
		return info
	}
	return deviceTypeRegistry[DeviceTypeXYDAQ16] // 默认
}

// 以下方法委托给 Info()，保持向后兼容

// PressureChannelCount 返回该设备类型的压力通道数
func (t DeviceType) PressureChannelCount() int {
	return t.Info().PressureChCount
}

// TotalChannelCount 返回该设备类型的总通道数
func (t DeviceType) TotalChannelCount() int {
	return t.Info().TotalChCount
}

// StreamFrameSize 返回该设备类型的数据帧大小（字节）
func (t DeviceType) StreamFrameSize() int {
	return t.Info().FrameSize
}

// IsDAQDevice 是否为真实DAQ设备（非模拟）
func (t DeviceType) IsDAQDevice() bool {
	return t.Info().IsRealDAQ
}

// IsTemperatureDevice 是否为温度采集设备（热电偶）
func (t DeviceType) IsTemperatureDevice() bool {
	return t.Info().IsTemperature
}

// AllDeviceTypes 返回所有已注册设备类型（供前端枚举使用）
func AllDeviceTypes() []DeviceTypeInfo {
	result := make([]DeviceTypeInfo, 0, len(deviceTypeRegistry))
	for _, info := range deviceTypeRegistry {
		result = append(result, info)
	}
	return result
}
```

- [ ] **Step 2: 在 constants.go 中新增 DAQ-T 相关常量**

```go
// DAQ-T-1603 热电偶采集设备常量
const (
	DAQTDefaultHost       = "192.168.1.7"
	DAQTDefaultPort       = 9000
	DAQTDiscoveryPort     = 7000
	DAQTChannelCount      = 16
	DAQTBinaryFrameSize   = 64  // BIN=1: 16 × float32 LE
	DAQTASCIIFrameSize    = 192 // BIN=0: 16 × 12字符定宽
	DAQTSerialFrameSize   = 46  // 串口帧
	DAQTConfigSyncDelayMs = 300 // 连接后配置同步延迟
	DAQTCmdTerminator     = "\n" // ASCII 命令终止符
	DAQTACKTimeoutMs      = 200 // ACK 超时
)
```

- [ ] **Step 3: 运行编译检查**

Run: `cd yx-daq && go build ./...`
Expected: 编译通过（向后兼容方法签名未变）

- [ ] **Step 4: 运行现有测试**

Run: `cd yx-daq && go test ./internal/... -v`
Expected: 全部 PASS

- [ ] **Step 5: Commit**

```bash
git add yx-daq/internal/types/device.go yx-daq/internal/types/constants.go
git commit -m "refactor: replace DeviceType switch methods with registry pattern"
```

---

### Task 2: 提取 TCP 驱动公共基座

**Files:**
- 创建: `yx-daq/internal/driver/tcp_base.go`
- 创建: `yx-daq/internal/driver/tcp_base_test.go`
- 修改: `yx-daq/internal/driver/xy_daq16.go`

**设计动机：** `XYDAQDriver` 和 `YXDAQTDriver` 共享 ~150 行重复代码：连接管理、断连重连、接收循环、通道配置、命令响应通道等。提取 `TCPDriverBase` 后，两个驱动只需实现各自差异化的协议逻辑。

- [ ] **Step 1: 编写 TCPDriverBase 测试**

```go
package driver

import (
	"testing"
)

func TestTCPDriverBase_SetDataCallback(t *testing.T) {
	base := NewTCPDriverBase("127.0.0.1", 9000, nil)
	called := false
	base.SetDataCallback(func(_ DataPayload) {
		called = true
	})
	if base.onData == nil {
		t.Fatal("callback should be set")
	}
}

func TestTCPDriverBase_UpdateAndGetChannels(t *testing.T) {
	channels := []ChannelConfig{
		{Index: 0, Name: "CH1", Enabled: true, Unit: "kPa"},
		{Index: 1, Name: "CH2", Enabled: true, Unit: "kPa"},
	}
	base := NewTCPDriverBase("127.0.0.1", 9000, channels)

	got := base.GetChannels()
	if len(got) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(got))
	}
	// 修改返回值不应影响内部
	got[0].Name = "MODIFIED"
	original := base.GetChannels()
	if original[0].Name == "MODIFIED" {
		t.Error("GetChannels should return a copy")
	}
}

func TestTCPDriverBase_ConnectedState(t *testing.T) {
	base := NewTCPDriverBase("127.0.0.1", 9000, nil)
	if base.IsConnected() {
		t.Error("should not be connected initially")
	}
	if base.IsAcquiring() {
		t.Error("should not be acquiring initially")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd yx-daq && go test ./internal/driver/ -run TestTCPDriverBase -v`
Expected: 编译失败

- [ ] **Step 3: 实现 tcp_base.go**

```go
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
// 封装连接管理、断连重连、接收循环、通道配置、命令响应通道等通用逻辑。
// 具体驱动（XY-DAQ、DAQ-T）嵌入此结构体，只需实现各自差异化的协议逻辑。
type TCPDriverBase struct {
	mu             sync.Mutex
	Host           string
	Port           int
	Conn           net.Conn
	connected      atomic.Bool
	acquiring      atomic.Bool
	draining       atomic.Bool
	onData         types.DataCallback
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

// Channels 返回通道配置引用（内部使用，不拷贝）
func (b *TCPDriverBase) Channels() []types.ChannelConfig {
	return b.channels
}

// DialConnect 建立 TCP 连接（通用实现，子驱动在 Connect 中调用）
func (b *TCPDriverBase) DialConnect() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", b.Host, b.Port), 5*time.Second)
	if err != nil {
		return fmt.Errorf("connect to %s:%d failed: %w", b.Host, b.Port, err)
	}

	b.Conn = conn
	b.connected.Store(true)
	b.reconnectCount = 0

	// 启用 TCP KeepAlive
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(10 * time.Second)
	}

	return nil
}

// CloseDisconnect 断开连接（通用实现，子驱动在 Disconnect 中调用）
func (b *TCPDriverBase) CloseDisconnect() {
	b.mu.Lock()
	defer b.mu.Unlock()

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
// processFunc: 子驱动自定义的数据处理函数（区分命令响应和数据帧）
func (b *TCPDriverBase) StartReceiveLoop(processFunc func(data []byte)) {
	go b.receiveLoop(processFunc)
}

func (b *TCPDriverBase) receiveLoop(processFunc func(data []byte)) {
	b.recvLoopRunning.Store(true)
	defer b.recvLoopRunning.Store(false)

	buf := make([]byte, 4096)
	for b.connected.Load() {
		n, err := b.Conn.Read(buf)
		if err != nil {
			if b.connected.Load() {
				slog.Error("TCP driver read error", "host", b.Host, "err", err)
				b.HandleDisconnect()
			}
			return
		}
		if n > 0 {
			processFunc(buf[:n])
		}
	}
}

// HandleDisconnect 处理断连（指数退避重连）
// 子驱动的 reconnectFunc 负责重新建立连接和初始化
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
		slog.Warn("TCP driver reconnecting", "host", b.Host, "attempt", b.reconnectCount, "max", types.MaxReconnectAttempts)

		if err := b.DialConnect(); err == nil {
			slog.Info("TCP driver reconnected successfully", "host", b.Host)
			return
		}
	}

	slog.Error("TCP driver max reconnect attempts reached", "host", b.Host)
}

// EmitData 发出数据回调
func (b *TCPDriverBase) EmitData(payload types.DataPayload) {
	if b.onData != nil {
		b.onData(payload)
	}
}

// BuildDataPayload 构建数据帧（映射到已启用通道）
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

// SendCommandDirect 直接发送命令并读取响应（receiveLoop 未运行时使用）
func (b *TCPDriverBase) SendCommandDirect(cmd string, timeout time.Duration) (string, error) {
	if !b.connected.Load() || b.Conn == nil {
		return "", fmt.Errorf("device not connected")
	}

	b.mu.Lock()
	_, err := b.Conn.Write([]byte(cmd))
	b.mu.Unlock()
	if err != nil {
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

// SendCommandViaChannel 通过 cmdRespCh 发送命令并获取响应（receiveLoop 运行时使用）
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
	_, err := b.Conn.Write([]byte(cmd))
	b.mu.Unlock()
	if err != nil {
		return "", fmt.Errorf("send command %q failed: %w", cmd, err)
	}

	select {
	case resp := <-b.CmdRespCh:
		return trimSpace(string(resp)), nil
	case <-time.After(timeout):
		return "", fmt.Errorf("command %q timeout", cmd)
	}
}

// WriteCommandOnly 仅发送命令，不等待响应
func (b *TCPDriverBase) WriteCommandOnly(cmd string) error {
	if !b.connected.Load() || b.Conn == nil {
		return fmt.Errorf("device not connected")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	_, err := b.Conn.Write([]byte(cmd))
	return err
}

// DrainConnection 排空 TCP 缓冲区
func (b *TCPDriverBase) DrainConnection(waitMs int) {
	b.Conn.SetReadDeadline(time.Now().Add(time.Duration(waitMs) * time.Millisecond))
	buf := make([]byte, 4096)
	for {
		_, err := b.Conn.Read(buf)
		if err != nil {
			break
		}
	}
	b.Conn.SetReadDeadline(time.Time{})
	b.RecvBuffer = b.RecvBuffer[:0]
}

// ConsumeOptionalACK 消费可选的 ACK 前导
func (b *TCPDriverBase) ConsumeOptionalACK(timeoutMs int) {
	b.Conn.SetReadDeadline(time.Now().Add(time.Duration(timeoutMs) * time.Millisecond))
	buf := make([]byte, 16)
	b.Conn.Read(buf)
	b.Conn.SetReadDeadline(time.Time{})
}

// RouteToCmdRespCh 将数据路由到命令响应通道（非采集状态下使用）
func (b *TCPDriverBase) RouteToCmdRespCh(data []byte) {
	select {
	case b.CmdRespCh <- data:
	default:
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd yx-daq && go test ./internal/driver/ -run TestTCPDriverBase -v`
Expected: PASS

- [ ] **Step 5: 重构 xy_daq16.go — 嵌入 TCPDriverBase**

将 `XYDAQDriver` 重构为嵌入 `TCPDriverBase`，删除重复代码，只保留 XY-DAQ 特有逻辑：

```go
package driver

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"time"

	"yx-daq/internal/types"
)

// XYDAQDriver XY-DAQ TCP驱动（支持DAQ8/DAQ16）
type XYDAQDriver struct {
	*TCPDriverBase
	streamID      int
	pressureCount int
	totalChannels int
	frameSize     int
}

// NewXYDAQDriver 创建XY-DAQ驱动（DAQ8/DAQ16通用）
func NewXYDAQDriver(host string, port, streamID int, channels []types.ChannelConfig, deviceType types.DeviceType) *XYDAQDriver {
	pressureCount := deviceType.PressureChannelCount()
	totalChannels := deviceType.TotalChannelCount()
	return &XYDAQDriver{
		TCPDriverBase: NewTCPDriverBase(host, port, channels),
		streamID:      streamID,
		pressureCount: pressureCount,
		totalChannels: totalChannels,
		frameSize:     deviceType.StreamFrameSize(),
	}
}

// Connect 建立TCP连接
func (d *XYDAQDriver) Connect() error {
	if err := d.DialConnect(); err != nil {
		return err
	}

	// 启用2字节长度前缀模式
	if _, err := d.Conn.Write([]byte("w1601\r")); err != nil {
		d.Conn.Close()
		return fmt.Errorf("send w1601 failed: %w", err)
	}
	time.Sleep(50 * time.Millisecond)

	// 读取设备EU单位并更新通道配置
	d.readAndUpdateEUUnit()

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

	streamTag := fmt.Sprintf("%d", d.streamID)

	cmd1 := fmt.Sprintf("c 00 %s FFFF 1 %d 7 0\r", streamTag, periodMs)
	if _, err := d.Conn.Write([]byte(cmd1)); err != nil {
		return fmt.Errorf("configure stream failed: %w", err)
	}
	time.Sleep(100 * time.Millisecond)

	cmd2 := fmt.Sprintf("c 05 %s 0810\r", streamTag)
	if _, err := d.Conn.Write([]byte(cmd2)); err != nil {
		return fmt.Errorf("configure stream content failed: %w", err)
	}
	time.Sleep(100 * time.Millisecond)

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

// processData XY-DAQ 特有的数据处理（2字节长度前缀拆包）
func (d *XYDAQDriver) processData(data []byte) {
	if d.draining.Load() {
		return
	}
	d.RecvBuffer = append(d.RecvBuffer, data...)
	d.processBuffer()
}

// processBuffer 处理接收缓冲区（2字节长度前缀拆包）
func (d *XYDAQDriver) processBuffer() {
	for len(d.RecvBuffer) >= 2 {
		frameLen := int(binary.BigEndian.Uint16(d.RecvBuffer[:2]))
		if frameLen < 2 || len(d.RecvBuffer) < frameLen {
			break
		}

		frame := d.RecvBuffer[:frameLen]
		d.RecvBuffer = d.RecvBuffer[frameLen:]

		payload := frame[2:]
		if len(payload) > 0 && payload[0] < 0x20 {
			d.handleStreamFrame(payload)
		} else {
			d.RouteToCmdRespCh(payload)
		}
	}
}

// handleStreamFrame 处理二进制数据流帧
func (d *XYDAQDriver) handleStreamFrame(frame []byte) {
	if !d.acquiring.Load() {
		return
	}
	if len(frame) < d.frameSize {
		return
	}

	values := make([]float64, d.totalChannels)
	for i := 0; i < d.totalChannels; i++ {
		offset := types.StreamFrameHeaderSize + i*4
		bits := binary.BigEndian.Uint32(frame[offset : offset+4])
		values[i] = float64(math.Float32frombits(bits))
	}

	for i := 0; i < d.pressureCount/2; i++ {
		j := d.pressureCount - 1 - i
		values[i], values[j] = values[j], values[i]
	}

	deviceID := fmt.Sprintf("%s:%d", d.Host, d.Port)
	d.EmitData(d.BuildDataPayload(values, deviceID))
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

// sendUnitCommand 发送单位命令
func (d *XYDAQDriver) sendUnitCommand(cmd string) (string, error) {
	if !d.connected.Load() || d.Conn == nil {
		return "", fmt.Errorf("device not connected")
	}

	if d.recvLoopRunning.Load() {
		// 排空残留
		select {
		case <-d.CmdRespCh:
		default:
		}
		d.mu.Lock()
		if _, err := d.Conn.Write([]byte(cmd)); err != nil {
			d.mu.Unlock()
			return "", fmt.Errorf("send unit command %q failed: %w", cmd, err)
		}
		d.mu.Unlock()
		select {
		case payload := <-d.CmdRespCh:
			return parseUnitPayload(payload)
		case <-time.After(3 * time.Second):
			return "", fmt.Errorf("unit command %q timeout", cmd)
		}
	}

	if _, err := d.Conn.Write([]byte(cmd)); err != nil {
		return "", fmt.Errorf("send unit command %q failed: %w", cmd, err)
	}
	d.Conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 1024)
	n, err := d.Conn.Read(buf)
	d.Conn.SetReadDeadline(time.Time{})
	if err != nil {
		return "", fmt.Errorf("read unit command response failed: %w", err)
	}
	return parseUnitPayload(buf[:n])
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

// parseUnitPayload, coeffToUnit, unitToCoeff, trimSpace 保留不变
// （这些函数已在 xy_daq16.go 中定义，无需修改）
```

- [ ] **Step 6: 运行编译检查**

Run: `cd yx-daq && go build ./...`
Expected: 编译通过

- [ ] **Step 7: 运行全部测试**

Run: `cd yx-daq && go test ./internal/... -v`
Expected: 全部 PASS

- [ ] **Step 8: Commit**

```bash
git add yx-daq/internal/driver/tcp_base.go yx-daq/internal/driver/tcp_base_test.go yx-daq/internal/driver/xy_daq16.go
git commit -m "refactor: extract TCPDriverBase, slim down XYDAQDriver"
```

---

### Task 3: 帧解析策略接口

**Files:**
- 创建: `yx-daq/internal/driver/frame_parser.go`

**设计动机：** DAQ-T 驱动中帧解析用 if/else 分发（二进制/ASCII/变长），违反策略模式。提取 `FrameParser` 接口后，解析逻辑可独立测试、替换，驱动代码不再关心具体帧格式。

- [ ] **Step 1: 实现 frame_parser.go**

```go
package driver

import (
	"encoding/binary"
	"fmt"
	"math"
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

func NewDAQTFrameReader() *DAQTFrameReader {
	return &DAQTFrameReader{frameSize: 64}
}

func (r *DAQTFrameReader) SetBinaryMode(isBinary bool) {
	if isBinary {
		r.frameSize = 64
	} else {
		r.frameSize = 192
	}
}

func (r *DAQTFrameReader) SetMetadataMode(enabled bool) {
	r.metadataMode = enabled
}

func (r *DAQTFrameReader) Feed(data []byte) {
	r.buffer = append(r.buffer, data...)
}

func (r *DAQTFrameReader) HasCompleteFrame() bool {
	if r.metadataMode {
		return r.hasVariableFrame()
	}
	return len(r.buffer) >= r.frameSize
}

func (r *DAQTFrameReader) ReadFrame() []byte {
	if r.metadataMode {
		return r.readVariableFrame()
	}
	return r.readFixedFrame()
}

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

// isValidDAQTFrame 校验帧数据合法性：至少 50% 通道值在物理范围内
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

func isParsableAsInt(s string) bool {
	var v int
	_, err := fmt.Sscanf(s, "%d", &v)
	return err == nil
}

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
```

- [ ] **Step 2: 运行编译检查**

Run: `cd yx-daq && go build ./...`
Expected: 编译通过

- [ ] **Step 3: Commit**

```bash
git add yx-daq/internal/driver/frame_parser.go
git commit -m "feat: add FrameParser strategy interface and DAQ-T parsers"
```

---

### Task 4: 帧解析器单元测试

**Files:**
- 创建: `yx-daq/internal/driver/yx_daqt_test.go`

- [ ] **Step 1: 编写帧解析器测试**

```go
package driver

import (
	"testing"
)

// --- FrameParser 策略接口测试 ---

func TestDAQTBinaryParser_ValidFrame(t *testing.T) {
	frame := make([]byte, 64)
	// CH15=39.95℃ at offset 0: float32 LE = C3 F5 21 42
	frame[0] = 0xC3
	frame[1] = 0xF5
	frame[2] = 0x21
	frame[3] = 0x42

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
	if values[7] < 39.9 || values[7] > 40.0 {
		t.Errorf("CH7 expected ~39.95, got %f", values[7])
	}
}

// --- 帧合法性校验 ---

func TestIsValidDAQTFrame(t *testing.T) {
	allZero := make([]float64, 16)
	if isValidDAQTFrame(allZero) {
		t.Error("all-zero frame should be invalid")
	}

	valid := make([]float64, 16)
	for i := 0; i < 10; i++ {
		valid[i] = 25.0
	}
	if !isValidDAQTFrame(valid) {
		t.Error("frame with 10/16 valid channels should be valid")
	}
}

// --- 帧读取器测试 ---

func TestDAQTFrameReader_BinaryMode(t *testing.T) {
	reader := NewDAQTFrameReader()
	reader.SetBinaryMode(true)

	frame := make([]byte, 64)
	frame[0] = 0xC3
	frame[1] = 0xF5
	frame[2] = 0x21
	frame[3] = 0x42

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
```

- [ ] **Step 2: 运行测试**

Run: `cd yx-daq && go test ./internal/driver/ -v`
Expected: 全部 PASS

- [ ] **Step 3: Commit**

```bash
git add yx-daq/internal/driver/yx_daqt_test.go
git commit -m "test: add FrameParser strategy and DAQ-T frame reader tests"
```

---

### Task 5: DAQ-T 配置同步与命令系统

**Files:**
- 创建: `yx-daq/internal/driver/yx_daqt_config.go`

- [ ] **Step 1: 实现 yx_daqt_config.go**

```go
package driver

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"yx-daq/internal/types"
)

// DAQTHardwareConfig DAQ-T 硬件配置（从设备读取）
type DAQTHardwareConfig struct {
	ThermocoupleTypes string
	ChannelMask       string
	SamplingRate      int
	BinaryFormat      bool
	ShowTimestamp     bool
	ShowSequence      bool
	AverageCount      int
	TriggerMode       int
	TriggerEdge       int
	TriggerCount      int
	IsTempModel       bool // temp 型号固件不支持 BIN=1
}

// sendCommand 发送 ASCII 命令并等待响应（自动选择直接/通道模式）
func (d *YXDAQTDriver) sendCommand(cmd string) (string, error) {
	fullCmd := cmd + DAQTCmdTerminator
	if d.recvLoopRunning.Load() {
		return d.SendCommandViaChannel(fullCmd, 3*time.Second)
	}
	return d.SendCommandDirect(fullCmd, 3*time.Second)
}

// sendCommandExact 发送命令并直接读取响应（配置同步阶段使用）
func (d *YXDAQTDriver) sendCommandExact(cmd string, _ int) (string, error) {
	fullCmd := cmd + DAQTCmdTerminator
	return d.SendCommandDirect(fullCmd, 3*time.Second)
}

// writeCmdOnly 仅发送命令，不等待响应
func (d *YXDAQTDriver) writeCmdOnly(cmd string) error {
	return d.WriteCommandOnly(cmd + DAQTCmdTerminator)
}

// syncHardwareConfig 连接后同步硬件配置
func (d *YXDAQTDriver) syncHardwareConfig() {
	config := DAQTHardwareConfig{}

	if resp, err := d.sendCommandExact("@e3", 16); err == nil {
		config.ThermocoupleTypes = resp
	}
	if resp, err := d.sendCommand("@fd MCH"); err == nil {
		config.ChannelMask = resp
	}
	if resp, err := d.sendCommand("@fd SPS"); err == nil {
		if v, e := strconv.Atoi(trimSpace(resp)); e == nil {
			config.SamplingRate = v
		}
	}
	if resp, err := d.sendCommandExact("@fd BIN", 1); err == nil {
		config.BinaryFormat = trimSpace(resp) == "1"
	}
	if resp, err := d.sendCommandExact("@fd TIME", 1); err == nil {
		config.ShowTimestamp = trimSpace(resp) == "1"
	}
	if resp, err := d.sendCommandExact("@fd HEAD", 1); err == nil {
		config.ShowSequence = trimSpace(resp) == "1"
	}
	if resp, err := d.sendCommand("@fd AVG"); err == nil {
		if v, e := strconv.Atoi(trimSpace(resp)); e == nil {
			config.AverageCount = v
		}
	}
	if resp, err := d.sendCommandExact("@fd TYPE", 1); err == nil {
		if v, e := strconv.Atoi(trimSpace(resp)); e == nil {
			config.TriggerMode = v
		}
	}
	if resp, err := d.sendCommandExact("@fd TRIG", 1); err == nil {
		if v, e := strconv.Atoi(trimSpace(resp)); e == nil {
			config.TriggerEdge = v
		}
	}
	if resp, err := d.sendCommand("@fd TNUM"); err == nil {
		if v, e := strconv.Atoi(trimSpace(resp)); e == nil {
			config.TriggerCount = v
		}
	}

	// 检测 temp 型号
	if !config.BinaryFormat {
		d.writeCmdOnly("@fe BIN 1")
		time.Sleep(100 * time.Millisecond)
		if resp, err := d.sendCommandExact("@fd BIN", 1); err == nil {
			if trimSpace(resp) == "1" {
				config.BinaryFormat = true
			} else {
				config.IsTempModel = true
				config.BinaryFormat = false
			}
		}
	}

	d.hwConfig = config
	d.frameReader.SetBinaryMode(config.BinaryFormat)
	d.frameReader.SetMetadataMode(config.ShowTimestamp || config.ShowSequence)
	d.configSyncDone = true
}

// applyNormalizedConfig 采集启动前归一化配置
func (d *YXDAQTDriver) applyNormalizedConfig() error {
	if d.hwConfig.BinaryFormat {
		d.writeCmdOnly("@fe BIN 1")
		time.Sleep(50 * time.Millisecond)
	}
	if d.hwConfig.ShowTimestamp {
		d.writeCmdOnly("@fe TIME 0")
		time.Sleep(50 * time.Millisecond)
		d.hwConfig.ShowTimestamp = false
	}
	if d.hwConfig.ShowSequence {
		d.writeCmdOnly("@fe HEAD 0")
		time.Sleep(50 * time.Millisecond)
		d.hwConfig.ShowSequence = false
	}
	d.frameReader.SetBinaryMode(d.hwConfig.BinaryFormat)
	d.frameReader.SetMetadataMode(false)

	// 根据配置选择帧解析策略
	d.selectFrameParser()
	return nil
}

// selectFrameParser 根据当前配置选择帧解析策略
func (d *YXDAQTDriver) selectFrameParser() {
	if d.hwConfig.BinaryFormat {
		d.frameParser = &DAQTBinaryParser{}
	} else if d.hwConfig.ShowTimestamp || d.hwConfig.ShowSequence {
		d.frameParser = &DAQTMetadataParser{}
	} else {
		d.frameParser = &DAQTASCIIParser{}
	}
}

// SetThermocoupleType 设置热电偶类型
func (d *YXDAQTDriver) SetThermocoupleType(tcTypes string) error {
	if len(tcTypes) != 16 {
		return fmt.Errorf("thermocouple types must be 16 characters, got %d", len(tcTypes))
	}
	cmd := fmt.Sprintf("@f3 0%s0", tcTypes)
	resp, err := d.sendCommand(cmd)
	if err != nil {
		return fmt.Errorf("set thermocouple type failed: %w", err)
	}
	if strings.ToUpper(trimSpace(resp)) == "E" {
		return fmt.Errorf("device rejected thermocouple type command")
	}
	return nil
}
```

- [ ] **Step 2: 运行编译检查**

Run: `cd yx-daq && go build ./...`
Expected: 编译失败（YXDAQTDriver 未定义），但语法正确

- [ ] **Step 3: Commit**

```bash
git add yx-daq/internal/driver/yx_daqt_config.go
git commit -m "feat: add DAQ-T config sync and command system"
```

---

### Task 6: DAQ-T 驱动核心

**Files:**
- 创建: `yx-daq/internal/driver/yx_daqt.go`

- [ ] **Step 1: 实现 yx_daqt.go — 嵌入 TCPDriverBase，组合 FrameParser**

```go
package driver

import (
	"fmt"
	"log/slog"
	"time"

	"yx-daq/internal/types"
)

// YXDAQTDriver DAQ-T-1603 热电偶采集设备驱动
// 嵌入 TCPDriverBase 复用连接/重连/接收循环等通用逻辑
// 组合 FrameParser 策略实现可替换的帧解析
type YXDAQTDriver struct {
	*TCPDriverBase
	frameReader    *DAQTFrameReader
	frameParser    FrameParser
	hwConfig       DAQTHardwareConfig
	configSyncDone bool
}

// NewYXDAQTDriver 创建 DAQ-T-1603 驱动
func NewYXDAQTDriver(host string, port int, channels []types.ChannelConfig) *YXDAQTDriver {
	return &YXDAQTDriver{
		TCPDriverBase: NewTCPDriverBase(host, port, channels),
		frameReader:   NewDAQTFrameReader(),
		frameParser:   &DAQTBinaryParser{}, // 默认，syncHardwareConfig 后会重新选择
	}
}

// Connect 建立 TCP 连接
func (d *YXDAQTDriver) Connect() error {
	if err := d.DialConnect(); err != nil {
		return err
	}

	d.configSyncDone = false

	// 延迟 300ms 后自动执行配置同步
	go func() {
		time.Sleep(DAQTConfigSyncDelayMs * time.Millisecond)
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

	// 等待配置同步完成
	for !d.configSyncDone {
		d.mu.Unlock()
		time.Sleep(50 * time.Millisecond)
		d.mu.Lock()
		if !d.connected.Load() {
			return fmt.Errorf("device disconnected during config sync")
		}
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

	d.ConsumeOptionalACK(DAQTACKTimeoutMs)
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

// processData DAQ-T 特有的数据处理（帧读取器 + 策略解析器）
func (d *YXDAQTDriver) processData(data []byte) {
	if d.draining.Load() {
		return
	}

	if !d.acquiring.Load() {
		d.RouteToCmdRespCh(data)
		return
	}

	d.frameReader.Feed(data)

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
```

- [ ] **Step 2: 运行编译检查**

Run: `cd yx-daq && go build ./...`
Expected: 编译通过

- [ ] **Step 3: Commit**

```bash
git add yx-daq/internal/driver/yx_daqt.go
git commit -m "feat: add YXDAQTDriver with TCPDriverBase and FrameParser strategy"
```

---

### Task 7: 注册驱动工厂（注册表驱动）

**Files:**
- 修改: `yx-daq/internal/manager/device_manager.go`

**设计动机：** 用 `DeviceTypeInfo` 驱动工厂选择，替代 switch。新增设备类型只需在 `deviceTypeRegistry` 加一行 + 在此加一个工厂函数注册。

- [ ] **Step 1: 新增驱动工厂注册表**

在 `device_manager.go` 中新增工厂类型和注册表：

```go
// DriverFactory 驱动工厂函数类型
type DriverFactory func(profile types.DeviceProfile) DeviceDriver

// driverFactories 驱动工厂注册表
var driverFactories = map[types.DeviceType]DriverFactory{
	types.DeviceTypeXYDAQ8:  newXYDAQDriver,
	types.DeviceTypeXYDAQ16: newXYDAQDriver,
	types.DeviceTypeYXDAQT:  newYXDAQTDriver,
	types.DeviceTypeSimulated: newSimulatedDriver,
}

func newXYDAQDriver(profile types.DeviceProfile) DeviceDriver {
	return driver.NewXYDAQDriver(profile.Host, profile.Port, profile.StreamID, profile.Channels, profile.Type)
}

func newYXDAQTDriver(profile types.DeviceProfile) DeviceDriver {
	return driver.NewYXDAQTDriver(profile.Host, profile.Port, profile.Channels)
}

func newSimulatedDriver(profile types.DeviceProfile) DeviceDriver {
	return driver.NewSimulatedDevice(profile.Channels)
}
```

- [ ] **Step 2: 简化 Connect 方法**

```go
// Connect 连接设备
func (m *DeviceManager) Connect(id string) error {
	m.mu.RLock()
	profile, ok := m.profiles[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("device profile not found: %s", id)
	}

	factory, ok := driverFactories[profile.Type]
	if !ok {
		return fmt.Errorf("unsupported device type: %s", profile.Type)
	}
	drv := factory(profile)

	dataSink := m.dataSink
	drv.SetDataCallback(func(payload types.DataPayload) {
		payload.DeviceID = id
		m.mu.Lock()
		m.latestData[id] = payload
		m.mu.Unlock()
		if dataSink != nil {
			dataSink(payload)
		}
	})

	if err := drv.Connect(); err != nil {
		return err
	}

	m.mu.Lock()
	m.instances[id] = drv
	profile.Channels = drv.GetChannels()
	m.profiles[id] = profile
	m.mu.Unlock()
	m.saveProfiles()
	return nil
}
```

- [ ] **Step 3: 运行编译检查**

Run: `cd yx-daq && go build ./...`
Expected: 编译通过

- [ ] **Step 4: 运行测试**

Run: `cd yx-daq && go test ./internal/... -v`
Expected: 全部 PASS

- [ ] **Step 5: Commit**

```bash
git add yx-daq/internal/manager/device_manager.go
git commit -m "refactor: replace driver switch with factory registry in DeviceManager"
```

---

### Task 8: 前端新增 YX-DAQ-T 设备类型

**Files:**
- 修改: `yx-daq/frontend/src/api/enums.ts`
- 修改: `yx-daq/frontend/src/views/DeviceView.vue`

- [ ] **Step 1: 在 enums.ts 中新增 YX_DAQT 类型**

```typescript
export const DeviceType = {
  SIMULATED: 'SIMULATED',
  XY_DAQ8: 'XY-DAQ8',
  XY_DAQ16: 'XY-DAQ16',
  YX_DAQT: 'YX-DAQ-T',
} as const

export type DeviceTypeValue = typeof DeviceType[keyof typeof DeviceType]

export const DeviceTypeLabels: Record<DeviceTypeValue, string> = {
  [DeviceType.SIMULATED]: '模拟设备',
  [DeviceType.XY_DAQ8]: 'XY-DAQ8',
  [DeviceType.XY_DAQ16]: 'XY-DAQ16',
  [DeviceType.YX_DAQT]: 'DAQ-T-1603',
}

// 设备类型元数据注册表（与后端 DeviceTypeInfo 对应）
export interface DeviceTypeInfo {
  type: DeviceTypeValue
  label: string
  pressureChCount: number
  totalChCount: number
  isTemperature: boolean
  defaultHost: string
  defaultPort: number
  defaultUnit: string
}

export const deviceTypeRegistry: Record<DeviceTypeValue, DeviceTypeInfo> = {
  [DeviceType.XY_DAQ8]: {
    type: 'XY-DAQ8', label: 'XY-DAQ8',
    pressureChCount: 8, totalChCount: 10, isTemperature: false,
    defaultHost: '192.168.3.101', defaultPort: 9000, defaultUnit: 'kPa',
  },
  [DeviceType.XY_DAQ16]: {
    type: 'XY-DAQ16', label: 'XY-DAQ16',
    pressureChCount: 16, totalChCount: 18, isTemperature: false,
    defaultHost: '192.168.3.101', defaultPort: 9000, defaultUnit: 'kPa',
  },
  [DeviceType.YX_DAQT]: {
    type: 'YX-DAQ-T', label: 'DAQ-T-1603',
    pressureChCount: 16, totalChCount: 16, isTemperature: true,
    defaultHost: '192.168.1.7', defaultPort: 9000, defaultUnit: '°C',
  },
  [DeviceType.SIMULATED]: {
    type: 'SIMULATED', label: '模拟设备',
    pressureChCount: 16, totalChCount: 18, isTemperature: false,
    defaultHost: '127.0.0.1', defaultPort: 9000, defaultUnit: 'kPa',
  },
}

export function getDeviceInfo(type: DeviceTypeValue): DeviceTypeInfo {
  return deviceTypeRegistry[type] || deviceTypeRegistry[DeviceType.XY_DAQ16]
}

export function getTotalChannelCount(type: DeviceTypeValue): number {
  return getDeviceInfo(type).totalChCount
}
```

- [ ] **Step 2: 在 DeviceView.vue 添加设备类型选项**

```html
<el-select v-model="newDevice.type" style="width: 100%">
  <el-option label="XY-DAQ8" value="XY-DAQ8" />
  <el-option label="XY-DAQ16" value="XY-DAQ16" />
  <el-option label="DAQ-T-1603 (热电偶)" value="YX-DAQ-T" />
  <el-option label="模拟设备" value="SIMULATED" />
</el-select>
```

- [ ] **Step 3: 用注册表替代硬编码函数**

替换 `getPressureCount` 和 `getTotalChannels` 为注册表查询：

```typescript
import { getDeviceInfo } from '../api/enums'

// 删除旧的 getPressureCount / getTotalChannels 函数
// 改用：
function getPressureCount(type: string): number {
  return getDeviceInfo(type as DeviceTypeValue).pressureChCount
}
function getTotalChannels(type: string): number {
  return getDeviceInfo(type as DeviceTypeValue).totalChCount
}
```

- [ ] **Step 4: 修改 addDevice 中的通道初始化**

```typescript
const info = getDeviceInfo(newDevice.value.type as DeviceTypeValue)
const channels = []
for (let i = 0; i < info.totalChCount; i++) {
  if (info.isTemperature) {
    channels.push({
      index: i, name: `CH${i+1}`, enabled: true,
      unit: '°C', precision: newDevice.value.precision,
      rangeMin: -100, rangeMax: 300,
    })
  } else {
    const isAtmPressure = i === info.pressureChCount
    const isAtmTemp = i === info.pressureChCount + 1
    channels.push({
      index: i,
      name: i < info.pressureChCount ? `CH${i+1}` : (isAtmPressure ? '大气压' : '大气温度'),
      enabled: true,
      unit: isAtmPressure ? 'Pa' : (isAtmTemp ? '°C' : newDevice.value.unit),
      precision: newDevice.value.precision,
      rangeMin: 0, rangeMax: 200,
    })
  }
}
```

- [ ] **Step 5: 监听类型变化，自动更新默认参数**

```typescript
watch(() => newDevice.value.type, (newType) => {
  const info = getDeviceInfo(newType as DeviceTypeValue)
  newDevice.value.host = info.defaultHost
  newDevice.value.port = info.defaultPort
  newDevice.value.unit = info.defaultUnit
})
```

- [ ] **Step 6: 修改编辑对话框通道提示**

```typescript
const editProfileType = ref('')

function openEditDialog(id: string) {
  // ...existing code...
  editProfileType.value = profile.type
  // ...
}
```

```html
<div class="channel-hint" v-if="editProfileType !== 'YX-DAQ-T'">
  0-{{ editPressureCount - 1 }}: 压力通道 | {{ editPressureCount }}: 大气压 | {{ editPressureCount + 1 }}: 大气温度
</div>
<div class="channel-hint" v-else>
  0-15: 温度通道（热电偶）
</div>
```

- [ ] **Step 7: 修改编辑保存时的通道单位逻辑**

```typescript
const isTempDevice = editProfileType.value === 'YX-DAQ-T'
const updatedChannels = channelsSnapshot.map(c => {
  let unit: string
  if (isTempDevice) {
    unit = '°C'
  } else {
    unit = c.index === pc ? 'Pa' : (c.index === pc + 1 ? '°C' : formSnapshot.unit)
  }
  return { index: c.index, name: c.name, enabled: c.enabled, unit, precision: formSnapshot.precision, rangeMin: c.rangeMin, rangeMax: c.rangeMax }
})
```

- [ ] **Step 8: 运行前端构建**

Run: `cd yx-daq/frontend && npm run build`
Expected: 构建通过

- [ ] **Step 9: Commit**

```bash
git add yx-daq/frontend/src/api/enums.ts yx-daq/frontend/src/views/DeviceView.vue
git commit -m "feat: add YX-DAQ-T device type in frontend with registry pattern"
```

---

### Task 9: 支持 DAQ-T 设备扫描

**Files:**
- 修改: `yx-daq/internal/scanner/daq_scanner.go`

- [ ] **Step 1: 扫描器兼容 DAQ-T 设备**

当前 `DAQScanner` 已使用 UDP 7000 端口广播，广播消息 `psi9000`。DAQ-T 使用相同发现端口，无需修改广播消息。确认现有逻辑兼容即可。

如果需要区分设备类型，可在 `DiscoveredDevice` 中新增 `DeviceModel` 字段，从响应中解析。当前阶段保持不变。

- [ ] **Step 2: 运行编译检查**

Run: `cd yx-daq && go build ./...`
Expected: 编译通过

- [ ] **Step 3: Commit（如有修改）**

```bash
git add yx-daq/internal/scanner/daq_scanner.go
git commit -m "feat: confirm DAQ-T device discovery compatibility"
```

---

### Task 10: 端到端验证

- [ ] **Step 1: 运行 Go 全量测试**

Run: `cd yx-daq && go test ./internal/... -v`
Expected: 全部 PASS

- [ ] **Step 2: 运行 Go lint**

Run: `cd yx-daq && golangci-lint run ./internal/...`
Expected: 无错误

- [ ] **Step 3: 运行前端 lint**

Run: `cd yx-daq/frontend && npm run lint`
Expected: 无错误

- [ ] **Step 4: 运行前端测试**

Run: `cd yx-daq/frontend && npm run test`
Expected: 全部 PASS

- [ ] **Step 5: 运行完整构建**

Run: `cd yx-daq && wails3 task build`
Expected: 构建成功

- [ ] **Step 6: Commit（如有修复）**

```bash
git add -A
git commit -m "fix: address lint and build issues from DAQ-T integration"
```

---

## 设计决策总结

| 问题 | 旧设计 | 新设计 | 收益 |
|------|--------|--------|------|
| OCP 违反 | 6 处 switch/if | 注册表 `DeviceTypeInfo` + `deviceTypeRegistry` | 新增设备类型零修改已有代码 |
| 代码重复 | XY-DAQ 和 DAQ-T 各自 ~150 行相同代码 | `TCPDriverBase` 公共基座 | 消除重复，单一维护点 |
| 帧解析耦合 | if/else 硬编码分发 | `FrameParser` 策略接口 | 可独立测试、替换、扩展 |
| 驱动工厂 | switch 分支 | `DriverFactory` 注册表 | 新增驱动只需注册工厂函数 |
| 前端类型 | 硬编码函数 | `deviceTypeRegistry` 注册表 | 与后端注册表对称，单一数据源 |

## 自检清单

### 1. Spec 覆盖度

| 协议特性 | 对应 Task |
|----------|-----------|
| 设备类型注册表 | Task 1 |
| 二进制帧解析 (BIN=1, 64B) | Task 3 |
| ASCII 定长帧解析 (BIN=0, 192B) | Task 3 |
| 变长帧解析 (TIME/HEAD) | Task 3 |
| 帧读取器 (FrameReader) | Task 3 |
| 帧合法性校验 | Task 3 |
| ASCII 命令系统 (@e3/@f0/@f1/@f3/@fd/@fe) | Task 5 |
| 配置同步 (syncHardwareConfig) | Task 5 |
| temp 型号兼容 (BIN=1 不生效) | Task 5 |
| 连接生命周期 (Connect/Disconnect) | Task 6 |
| 采集生命周期 (Start/Stop) | Task 6 |
| 归一化配置 (applyNormalizedConfig) | Task 5 |
| 启动前准备 (Preamble) | Task 6 |
| ACK 消费 | Task 5 |
| 驱动工厂注册 | Task 7 |
| 前端设备类型选项 | Task 8 |
| 前端通道配置 (16 温度通道) | Task 8 |
| 设备扫描兼容 | Task 9 |
| 单元测试 | Task 4 |
| 端到端验证 | Task 10 |

### 2. 占位符扫描

无 TBD/TODO/实现后补充等占位符。

### 3. 类型一致性

- `FrameParser.Parse` 返回 `([]float64, error)` — Task 3 定义，Task 6 使用 ✓
- `DAQTBinaryParser` / `DAQTASCIIParser` / `DAQTMetadataParser` 实现 `FrameParser` — Task 3 定义，Task 5/6 使用 ✓
- `isValidDAQTFrame` 接受 `[]float64` — Task 3 定义，Task 6 使用 ✓
- `DAQTFrameReader` 方法签名 — Task 3 定义，Task 5/6 使用 ✓
- `DAQTHardwareConfig` 结构体 — Task 5 定义，Task 6 使用 ✓
- `DeviceTypeYXDAQT` 常量值 `"YX-DAQ-T"` — Task 1 定义，Task 7/8 使用 ✓
- `TCPDriverBase` 方法签名 — Task 2 定义，Task 5/6 使用 ✓
- `DeviceTypeInfo` 注册表 — Task 1 定义，Task 7/8 使用 ✓
- `DriverFactory` 注册表 — Task 7 定义 ✓
- 前端 `deviceTypeRegistry` — Task 8 定义，与后端 `DeviceTypeInfo` 对称 ✓
