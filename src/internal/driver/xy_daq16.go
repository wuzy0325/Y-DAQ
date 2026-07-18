package driver

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	"yx-daq/internal/types"
)

// XYDAQDriver XY-DAQ TCP驱动（支持DAQ8/DAQ16）
type XYDAQDriver struct {
	*TCPDriverBase
	streamID      int
	pressureCount int // 压力通道数（8或16）
	totalChannels int // 总通道数（压力+大气压+大气温度）
	frameSize     int // 数据帧大小（字节）
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
	// 注册重连 hook，之后 HandleDisconnect 重连成功时会自动调用 initAfterConnect
	d.SetOnReconnect(d.initAfterConnect)
	return d.initAfterConnect()
}

// initAfterConnect 连接建立后的初始化逻辑（首次连接与重连均调用）
// 包含：w1601 模式切换、EU 单位读取、启动数据接收协程
func (d *XYDAQDriver) initAfterConnect() error {
	// 启用2字节长度前缀模式
	if _, err := d.Conn.Write([]byte("w1601\r")); err != nil {
		d.Conn.Close()
		d.connected.Store(false)
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

	// 配置数据流参数: 内部时钟/大端/连续
	cmd1 := fmt.Sprintf("c 00 %s FFFF 1 %d 7 0\r", streamTag, periodMs)
	if _, err := d.Conn.Write([]byte(cmd1)); err != nil {
		return fmt.Errorf("configure stream failed: %w", err)
	}
	time.Sleep(100 * time.Millisecond)

	// 配置返回内容: 压力+大气压+温度
	cmd2 := fmt.Sprintf("c 05 %s 0810\r", streamTag)
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
func (d *XYDAQDriver) processData(_ []byte) {
	for len(d.RecvBuffer) >= 2 {
		// 2字节大端长度前缀
		frameLen := int(binary.BigEndian.Uint16(d.RecvBuffer[:2]))
		if frameLen < 2 || len(d.RecvBuffer) < frameLen {
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

	deviceID := fmt.Sprintf("%s:%d", d.Host, d.Port)
	payload := d.BuildDataPayload(values, deviceID)
	d.EmitData(payload)
}

// SendCommand 发送命令并等待ASCII响应（用于查询类命令）
// 注意：此方法在receiveLoop启动前调用，直接从conn读取响应
func (d *XYDAQDriver) SendCommand(cmd string) (string, error) {
	if !d.connected.Load() || d.Conn == nil {
		return "", fmt.Errorf("device not connected")
	}

	// 发送命令
	if _, err := d.Conn.Write([]byte(cmd + "\r")); err != nil {
		return "", fmt.Errorf("send command %q failed: %w", cmd, err)
	}

	// 读取响应（带超时）
	d.Conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1024)
	n, err := d.Conn.Read(buf)
	d.Conn.SetReadDeadline(time.Time{}) // 清除超时
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
	if !d.connected.Load() || d.Conn == nil {
		return "", fmt.Errorf("device not connected")
	}

	// receiveLoop 运行时通过 cmdRespCh 获取响应，避免与 receiveLoop 竞争 conn.Read
	if d.recvLoopRunning.Load() {
		// 排空残留响应
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
			// processData 已剥离 2 字节长度前缀，payload 为纯响应数据，无需再走 parseUnitPayload
			return trimSpace(string(payload)), nil
		case <-time.After(3 * time.Second):
			return "", fmt.Errorf("unit command %q timeout", cmd)
		}
	}

	// receiveLoop 未运行时直接读写（Connect 初始化阶段）
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

// ReadValveState 读取校准阀位（命令 `@01  0`）。
// 返回 ValveStateCalibration(=1) / ValveStateMeasurement(=2/3) / ValveStateUnknown(=0)。
// 设备拒绝（Nxx）作为错误抛出，避免上层误把错误码当成阀位。
func (d *XYDAQDriver) ReadValveState() (types.ValveState, error) {
	resp, err := d.sendUnitCommand("@01  0")
	if err != nil {
		return types.ValveStateUnknown, fmt.Errorf("read valve state: %w", err)
	}
	return parseValveReadResponse(resp)
}

// SetValveState 切换校准阀位。Calibration→w0C01，Measurement→w0C00。
// 采集进行中严禁切阀（压力瞬变会损坏数据/传感器），由调用方（DeviceManager）拦截。
// 设备拒绝（Nxx）作为专属错误返回，便于前端给出可读提示。
func (d *XYDAQDriver) SetValveState(state types.ValveState) error {
	cmd, err := valveSetCommandFor(state)
	if err != nil {
		return err
	}
	resp, err := d.sendUnitCommand(cmd)
	if err != nil {
		return fmt.Errorf("set valve state: %w", err)
	}
	return interpretValveSetResponse(cmd, resp)
}

// parseValveReadResponse 把硬件读阀响应映射为统一阀位三态。
// 抽为纯函数便于单测覆盖所有分支（NACK / 数字 0~3 / 文本同义词 / 未识别）。
func parseValveReadResponse(resp string) (types.ValveState, error) {
	raw := strings.TrimSpace(resp)
	// 设备拒绝命令：以 N 开头并跟两位数字
	if isNACK(raw) {
		return types.ValveStateUnknown, fmt.Errorf("device rejected read valve: %s", raw)
	}
	val := strings.TrimSpace(strings.TrimPrefix(raw, "A"))
	if val == "" {
		val = raw
	}
	if num, parseErr := strconv.Atoi(strings.TrimSpace(val)); parseErr == nil {
		switch num {
		case 1:
			return types.ValveStateCalibration, nil
		case 2, 3:
			// 现场兼容：部分固件在 RUN/测量态返回 3
			return types.ValveStateMeasurement, nil
		case 0:
			// 0 在不同固件下可能表示「测量态」或「未初始化」，
			// 没有现场固件文档前不武断归类为 measurement
			return types.ValveStateUnknown, nil
		}
	}
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "calibration", "calibrate":
		return types.ValveStateCalibration, nil
	case "measurement", "measure":
		return types.ValveStateMeasurement, nil
	default:
		return types.ValveStateUnknown, nil
	}
}

// valveSetCommandFor 把业务态映射到具体协议命令字。
func valveSetCommandFor(state types.ValveState) (string, error) {
	switch state {
	case types.ValveStateCalibration:
		return "w0C01", nil
	case types.ValveStateMeasurement:
		return "w0C00", nil
	default:
		return "", fmt.Errorf("invalid valve state: %s", state)
	}
}

// interpretValveSetResponse 把写阀响应分类：成功 / 设备拒绝 / 协议异常。
func interpretValveSetResponse(cmd, resp string) error {
	trimmed := strings.TrimSpace(resp)
	if isNACK(trimmed) {
		// 固件拒绝（如 N09 拒绝校准），明确告知用户「设备拒绝」
		return fmt.Errorf("device rejected valve command %s: %s", cmd, trimmed)
	}
	if trimmed != "A" {
		return fmt.Errorf("set valve state failed: response %q", trimmed)
	}
	return nil
}

// isNACK 判定响应是否为设备拒绝错误码（Nxx）。
// 协议中 N 开头后跟两位数字（如 N09、N03）表示固件拒绝。
func isNACK(resp string) bool {
	if len(resp) < 2 || resp[0] != 'N' {
		return false
	}
	for i := 1; i < len(resp); i++ {
		if resp[i] < '0' || resp[i] > '9' {
			return false
		}
	}
	return true
}
