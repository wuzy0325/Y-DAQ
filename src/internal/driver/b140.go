package driver

import (
	"bufio"
	"fmt"
	"log/slog"
	"math"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"yx-daq/internal/logger"
	"yx-daq/internal/types"
)

// B140Driver B140运动控制器TCP驱动
type B140Driver struct {
	mu        sync.Mutex
	address   string
	port      int
	timeoutMs int
	conn      net.Conn
	connected atomic.Bool
	reader    *bufio.Reader
	// deviceID 配置中的控制器 ID（仅用于通信日志可读性，由驱动工厂在创建后注入）
	deviceID string
}

// NewB140Driver 创建B140驱动
func NewB140Driver(address string, port, timeoutMs int) *B140Driver {
	if timeoutMs == 0 {
		timeoutMs = 5000
	}
	return &B140Driver{
		address:   address,
		port:      port,
		timeoutMs: timeoutMs,
	}
}

// SetDeviceID 设置控制器 ID（仅用于通信日志；由驱动工厂在创建后注入）
func (d *B140Driver) SetDeviceID(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.deviceID = id
}

// commFields 组装通信日志公共字段
func (d *B140Driver) commFields() []any {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.commFieldsLocked()
}

// commFieldsLocked 组装通信日志公共字段（调用方必须持有 d.mu）
func (d *B140Driver) commFieldsLocked() []any {
	return []any{"device", d.deviceID, "host", d.address, "port", d.port}
}

// Connect 建立TCP连接
func (d *B140Driver) Connect() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", d.address, d.port), time.Duration(d.timeoutMs)*time.Millisecond)
	if err != nil {
		return fmt.Errorf("connect to B140 %s:%d failed: %w", d.address, d.port, err)
	}

	d.conn = conn
	d.reader = bufio.NewReader(conn)
	d.connected.Store(true)

	return nil
}

// Disconnect 断开连接
func (d *B140Driver) Disconnect() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.connected.Store(false)
	if d.conn != nil {
		d.conn.Close()
		d.conn = nil
	}
}

// IsConnected 是否已连接
func (d *B140Driver) IsConnected() bool {
	return d.connected.Load()
}

// SendCommand 发送命令并等待响应（串行锁保护）
func (d *B140Driver) SendCommand(cmd string) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.connected.Load() {
		return "", fmt.Errorf("B140 not connected")
	}

	// 设置读超时
	d.conn.SetReadDeadline(time.Now().Add(time.Duration(d.timeoutMs) * time.Millisecond))

	if _, err := fmt.Fprintf(d.conn, "%s\r", cmd); err != nil {
		d.connected.Store(false)
		logger.CommError("B140 send command failed",
			append(d.commFieldsLocked(), "op", "send-command", "cmd", logger.Truncate(cmd, 256), "err", err)...)
		return "", fmt.Errorf("send command %q failed: %w", cmd, err)
	}

	// B140/Galil responses terminate with ':' on success or '?' on error.
	resp, err := d.readResponse()
	if err != nil {
		logger.CommError("B140 read response failed",
			append(d.commFieldsLocked(), "op", "send-command", "cmd", logger.Truncate(cmd, 256), "err", err)...)
		return "", fmt.Errorf("read response for %q failed: %w", cmd, err)
	}

	if strings.HasSuffix(resp, "?") {
		logger.CommError("B140 command rejected",
			append(d.commFieldsLocked(), "op", "send-command", "cmd", logger.Truncate(cmd, 256), "resp", logger.Truncate(resp, 256))...)
		return resp, fmt.Errorf("B140 command error: %s", cmd)
	}

	return resp, nil
}

func (d *B140Driver) readResponse() (string, error) {
	var b strings.Builder
	buf := make([]byte, 1)
	for {
		if _, err := d.conn.Read(buf); err != nil {
			return b.String(), err
		}
		b.WriteByte(buf[0])
		if buf[0] == ':' || buf[0] == '?' {
			return b.String(), nil
		}
	}
}

// B140MotionController B140运动控制器（高层接口）
type B140MotionController struct {
	driver *B140Driver
	// mu 保护 axes 与 *Signature 字段的并发访问（轮询/移动/配置更新分属不同 goroutine）
	mu                 sync.RWMutex
	axes               []types.AxisConfig
	directionSignature string // cached MT/CE signature
	softLimitSignature string // cached FL/BL signature
	// applyLimitMu 串行化软限位下发，避免并发下发命令交错
	applyLimitMu sync.Mutex
}

// NewB140MotionController 创建B140运动控制器
func NewB140MotionController(driver *B140Driver, axes []types.AxisConfig) *B140MotionController {
	return &B140MotionController{
		driver: driver,
		axes:   axes,
	}
}

// axesSnapshot 返回当前轴配置切片；切片整体替换、元素只读，调用方可安全遍历
func (c *B140MotionController) axesSnapshot() []types.AxisConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.axes
}

// softLimitSigSnapshot 读取软限位签名
func (c *B140MotionController) softLimitSigSnapshot() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.softLimitSignature
}

// resetSignatures 清空方向/软限位签名，使下次访问强制重新下发
func (c *B140MotionController) resetSignatures() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.directionSignature = ""
	c.softLimitSignature = ""
}

// Connect 连接并使能所有轴
func (c *B140MotionController) Connect() error {
	if err := c.driver.Connect(); err != nil {
		return err
	}

	// 使能所有轴
	if _, err := c.driver.SendCommand("SH"); err != nil {
		return fmt.Errorf("servo enable failed: %w", err)
	}

	// 配置各轴方向
	c.resetSignatures()
	if err := c.ensureAxisDirectionConfigured(); err != nil {
		slog.Warn("axis direction config warning", "err", err)
	}

	// 下发各轴软限位（FL/BL）
	c.ensureSoftLimitsConfigured()

	return nil
}

// Disconnect 断开连接
func (c *B140MotionController) Disconnect() {
	c.resetSignatures()
	c.driver.Disconnect()
}

// UpdateAxes applies edited axis parameters to an already connected controller.
func (c *B140MotionController) UpdateAxes(axes []types.AxisConfig) {
	c.mu.Lock()
	c.axes = axes
	c.directionSignature = ""
	c.softLimitSignature = ""
	c.mu.Unlock()
	if c.IsConnected() {
		c.ensureSoftLimitsConfigured()
	}
}

// IsConnected 是否已连接
func (c *B140MotionController) IsConnected() bool {
	return c.driver.IsConnected()
}

// resolveAxis 查找轴名映射并确保方向与软限位已配置
func (c *B140MotionController) resolveAxis(axis types.AxisName) (string, error) {
	bAxis, ok := types.AxisNameToB140[axis]
	if !ok {
		return "", fmt.Errorf("unknown axis: %s", axis)
	}
	if err := c.ensureAxisDirectionConfigured(); err != nil {
		slog.Warn("axis direction config warning", "err", err)
	}
	c.ensureSoftLimitsConfigured()
	return bAxis, nil
}

// MoveTo 绝对定位移动
func (c *B140MotionController) MoveTo(axis types.AxisName, position float64) error {
	bAxis, err := c.resolveAxis(axis)
	if err != nil {
		return err
	}

	if err := c.checkSoftLimit(axis, position); err != nil {
		return err
	}

	if err := c.applyConfiguredSpeed(axis, bAxis); err != nil {
		return err
	}

	pulse := c.engineeringToPulse(axis, position)
	cmd := fmt.Sprintf("PA%s=%d", bAxis, int(math.Round(pulse)))
	if _, err := c.driver.SendCommand(cmd); err != nil {
		return err
	}

	_, err = c.driver.SendCommand(fmt.Sprintf("BG%s", bAxis))
	return err
}

// MoveBy 相对增量移动
func (c *B140MotionController) MoveBy(axis types.AxisName, delta float64) error {
	bAxis, err := c.resolveAxis(axis)
	if err != nil {
		return err
	}

	if err := c.checkSoftLimitDelta(axis, bAxis, delta); err != nil {
		return err
	}

	pulse := c.engineeringToPulse(axis, delta)
	if int(math.Round(pulse)) == 0 {
		return nil
	}

	if err := c.applyConfiguredSpeed(axis, bAxis); err != nil {
		return err
	}

	cmd := fmt.Sprintf("PR%s=%d", bAxis, int(math.Round(pulse)))
	if _, err := c.driver.SendCommand(cmd); err != nil {
		return err
	}

	_, err = c.driver.SendCommand(fmt.Sprintf("BG%s", bAxis))
	return err
}

// Jog 点动 (moves 1 engineering unit per call)
func (c *B140MotionController) Jog(axis types.AxisName, direction int, distance float64, speed float64) error {
	bAxis, err := c.resolveAxis(axis)
	if err != nil {
		return err
	}

	stepEngineering := distance
	if direction < 0 {
		stepEngineering = -distance
	}
	if err := c.checkSoftLimitDelta(axis, bAxis, stepEngineering); err != nil {
		return err
	}

	ax := c.findAxis(axis)
	if ax != nil {
		maxSpeed := ax.MaxSpeed
		if maxSpeed <= 0 {
			maxSpeed = 1
		}
		if speed <= 0 || speed > maxSpeed {
			speed = maxSpeed
		}
	}
	if speed > 0 {
		pulseSpeed := c.engineeringToPulse(axis, speed)
		if _, err := c.driver.SendCommand(fmt.Sprintf("SP%s=%d", bAxis, int(math.Round(pulseSpeed)))); err != nil {
			return err
		}
	}

	stepPulse := c.engineeringToPulse(axis, stepEngineering)
	if _, err := c.driver.SendCommand(fmt.Sprintf("PR%s=%d", bAxis, int(math.Round(stepPulse)))); err != nil {
		return err
	}
	_, err = c.driver.SendCommand(fmt.Sprintf("BG%s", bAxis))
	return err
}

// Home 单轴回零
func (c *B140MotionController) Home(axis types.AxisName) error {
	bAxis, err := c.resolveAxis(axis)
	if err != nil {
		return err
	}
	if err := c.applyConfiguredSpeed(axis, bAxis); err != nil {
		return err
	}
	if _, err := c.driver.SendCommand(fmt.Sprintf("HM%s", bAxis)); err != nil {
		return err
	}
	_, err = c.driver.SendCommand(fmt.Sprintf("BG%s", bAxis))
	return err
}

// Stop 停止运动
func (c *B140MotionController) Stop(axis types.AxisName) error {
	bAxis, ok := types.AxisNameToB140[axis]
	if !ok {
		return fmt.Errorf("unknown axis: %s", axis)
	}
	_, err := c.driver.SendCommand(fmt.Sprintf("ST%s", bAxis))
	return err
}

// StopAll 停止所有轴
func (c *B140MotionController) StopAll() error {
	_, err := c.driver.SendCommand("ST")
	return err
}

// EmergencyStop 急停
func (c *B140MotionController) EmergencyStop() error {
	_, err := c.driver.SendCommand("AB")
	return err
}

// DefinePosition 置位（DP+DE）
func (c *B140MotionController) DefinePosition(axis types.AxisName, position float64) error {
	bAxis, err := c.resolveAxis(axis)
	if err != nil {
		return err
	}

	pulse := c.engineeringToPulse(axis, position)
	if _, err := c.driver.SendCommand(fmt.Sprintf("DP%s=%d", bAxis, int(math.Round(pulse)))); err != nil {
		return err
	}
	encoderCount := c.engineeringToEncoderCount(axis, position)
	_, err = c.driver.SendCommand(fmt.Sprintf("DE%s=%d", bAxis, int(math.Round(encoderCount))))
	return err
}

// GetAxisStatus 查询单轴状态（位置+运动状态+限位状态+回零）
func (c *B140MotionController) GetAxisStatus(axis types.AxisName) (types.AxisStatus, error) {
	bAxis, err := c.resolveAxis(axis)
	if err != nil {
		return types.AxisStatus{}, err
	}

	status := types.AxisStatus{Name: axis}
	ax := c.findAxis(axis)
	useEncoder := ax != nil && ax.EncoderCompensation.Enabled

	// 读取寄存器位置 (TD, 全轴)
	tdPositions, _ := c.readTD()

	// 读取运动状态 (TS)
	moving, _ := c.IsAxisMoving(axis)
	status.Moving = moving

	// 读取限位状态 (MG _LFA / MG _LRA)
	limitStatus, _ := c.GetLimitStatus(axis)
	status.PosLimit = limitStatus.PosLimit
	status.NegLimit = limitStatus.NegLimit

	// 位置读取
	if useEncoder {
		resp, err := c.driver.SendCommand(fmt.Sprintf("TP%s", bAxis))
		if err == nil {
			clean := strings.TrimSpace(resp)
			clean = strings.TrimSuffix(clean, ":")
			var count int
			if _, scanErr := fmt.Sscanf(clean, "%d", &count); scanErr == nil {
				status.Position = c.encoderCountToEngineering(axis, float64(count))
			}
		} else if pos, has := tdPositions[bAxis]; has {
			status.Position = c.pulseToEngineering(axis, pos)
		}
	} else {
		if pos, has := tdPositions[bAxis]; has {
			status.Position = c.pulseToEngineering(axis, pos)
		}
	}

	status.Homed = math.Abs(status.Position) < 0.001

	return status, nil
}

// GetAllAxisStatus 批量查询所有轴状态（TD+TS+限位一次读取）
func (c *B140MotionController) GetAllAxisStatus() ([]types.AxisStatus, error) {
	if err := c.ensureAxisDirectionConfigured(); err != nil {
		slog.Warn("axis direction config warning", "err", err)
	}

	tdPositions, err := c.readTD()
	if err != nil {
		return nil, fmt.Errorf("read TD failed: %w", err)
	}
	tsMoving, err := c.readTSMoving()
	if err != nil {
		return nil, fmt.Errorf("read TS failed: %w", err)
	}
	limitMap := c.readAllLimits()

	statuses := []types.AxisStatus{}
	for _, ax := range c.axesSnapshot() {
		if !ax.Enabled {
			continue
		}
		bAxis, ok := types.AxisNameToB140[ax.Name]
		if !ok {
			continue
		}

		useEncoder := ax.EncoderCompensation.Enabled
		var position float64

		if useEncoder {
			// 读取编码器位置 (TP, 逐轴)
			tpResp, tpErr := c.driver.SendCommand(fmt.Sprintf("TP%s", bAxis))
			if tpErr == nil {
				clean := strings.TrimSpace(tpResp)
				clean = strings.TrimSuffix(clean, ":")
				var count int
				if _, scanErr := fmt.Sscanf(clean, "%d", &count); scanErr == nil {
					position = c.encoderCountToEngineering(ax.Name, float64(count))
				}
			} else if pos, has := tdPositions[bAxis]; has {
				position = c.pulseToEngineering(ax.Name, pos)
			}
		} else {
			if pos, has := tdPositions[bAxis]; has {
				position = c.pulseToEngineering(ax.Name, pos)
			}
		}

		limits := limitMap[bAxis]

		statuses = append(statuses, types.AxisStatus{
			Name:     ax.Name,
			Position: position,
			Moving:   tsMoving[bAxis],
			PosLimit: limits.PosLimit,
			NegLimit: limits.NegLimit,
			Homed:    math.Abs(position) < 0.001,
		})
	}
	return statuses, nil
}

// readTSMoving 读取所有轴运动状态 (TS 命令)，返回 bAxis -> 是否运动中
func (c *B140MotionController) readTSMoving() (map[string]bool, error) {
	resp, err := c.driver.SendCommand("TS")
	if err != nil {
		return nil, err
	}
	clean := strings.TrimSpace(resp)
	clean = strings.TrimSuffix(clean, ":")
	parts := strings.Split(clean, ",")
	axes := []string{"A", "B", "C", "D"}
	result := make(map[string]bool, len(axes))
	for i, p := range parts {
		if i >= len(axes) {
			break
		}
		val, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			continue
		}
		result[axes[i]] = val&0x80 != 0
	}
	return result, nil
}

// readAllLimits 读取所有轴限位状态 (8 条 MG 命令)，错误时该限位保持 false
func (c *B140MotionController) readAllLimits() map[string]types.LimitStatus {
	result := make(map[string]types.LimitStatus, 4)
	for _, bAxis := range []string{"A", "B", "C", "D"} {
		ls := types.LimitStatus{}
		if rp, rpErr := c.driver.SendCommand(fmt.Sprintf("MG _LF%s", bAxis)); rpErr == nil {
			ls.PosLimit = parseMGBool(rp)
		}
		if rn, rnErr := c.driver.SendCommand(fmt.Sprintf("MG _LR%s", bAxis)); rnErr == nil {
			ls.NegLimit = parseMGBool(rn)
		}
		result[bAxis] = ls
	}
	return result
}

// readTD 读取所有轴寄存器位置 (TD命令)
func (c *B140MotionController) readTD() (map[string]float64, error) {
	resp, err := c.driver.SendCommand("TD")
	if err != nil {
		return nil, err
	}
	clean := strings.TrimSpace(resp)
	clean = strings.TrimSuffix(clean, ":")
	parts := strings.Split(clean, ",")
	if len(parts) < 4 {
		return nil, fmt.Errorf("TD response too short: %s", resp)
	}
	result := make(map[string]float64)
	axes := []string{"A", "B", "C", "D"}
	for i, p := range parts {
		if i >= len(axes) {
			break
		}
		val, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			continue
		}
		result[axes[i]] = val
	}
	return result, nil
}

// SetSpeed 设置速度
func (c *B140MotionController) SetSpeed(axis types.AxisName, speed float64) error {
	bAxis, ok := types.AxisNameToB140[axis]
	if !ok {
		return fmt.Errorf("unknown axis: %s", axis)
	}
	pulseSpeed := c.engineeringToPulse(axis, speed)
	_, err := c.driver.SendCommand(fmt.Sprintf("SP%s=%d", bAxis, int(math.Round(pulseSpeed))))
	return err
}

// applyConfiguredSpeed 在运动前下发轴配置的最大速度（SP 命令），
// 确保使用设定速度而非固件默认/上次残留速度。MaxSpeed 未配置时跳过。
func (c *B140MotionController) applyConfiguredSpeed(axis types.AxisName, bAxis string) error {
	ax := c.findAxis(axis)
	if ax == nil || ax.MaxSpeed <= 0 {
		return nil
	}
	pulseSpeed := c.engineeringToPulse(axis, ax.MaxSpeed)
	if _, err := c.driver.SendCommand(fmt.Sprintf("SP%s=%d", bAxis, int(math.Round(pulseSpeed)))); err != nil {
		return err
	}
	return nil
}

// engineeringToPulse 工程单位→脉冲
func (c *B140MotionController) engineeringToPulse(axis types.AxisName, position float64) float64 {
	ax := c.findAxis(axis)
	if ax == nil {
		return position
	}

	stepAngleDeg := ax.StepAngleDeg
	if stepAngleDeg == 0 {
		stepAngleDeg = 1.8
	}
	stepsPerRev := 360.0 / stepAngleDeg
	pulsesPerRev := stepsPerRev * float64(ax.MicroSteps)

	var pulsesPerUnit float64
	if ax.Kind == types.AxisKindLinear {
		lead := ax.Lead
		if lead == 0 {
			lead = 1
		}
		pulsesPerUnit = pulsesPerRev / lead
	} else {
		gearRatio := ax.GearRatio
		if gearRatio <= 0 {
			gearRatio = 1
		}
		pulsesPerUnit = (pulsesPerRev * gearRatio) / 360.0
	}

	return position * pulsesPerUnit
}

// pulseToEngineering 脉冲→工程单位
func (c *B140MotionController) pulseToEngineering(axis types.AxisName, pulses float64) float64 {
	ax := c.findAxis(axis)
	if ax == nil {
		return pulses
	}

	stepAngleDeg := ax.StepAngleDeg
	if stepAngleDeg == 0 {
		stepAngleDeg = 1.8
	}
	stepsPerRev := 360.0 / stepAngleDeg
	pulsesPerRev := stepsPerRev * float64(ax.MicroSteps)

	var pulsesPerUnit float64
	if ax.Kind == types.AxisKindLinear {
		lead := ax.Lead
		if lead == 0 {
			lead = 1
		}
		pulsesPerUnit = pulsesPerRev / lead
	} else {
		gearRatio := ax.GearRatio
		if gearRatio <= 0 {
			gearRatio = 1
		}
		pulsesPerUnit = (pulsesPerRev * gearRatio) / 360.0
	}

	if pulsesPerUnit == 0 {
		return 0
	}
	return pulses / pulsesPerUnit
}

// engineeringToEncoderCount 工程单位→编码器计数值
func (c *B140MotionController) engineeringToEncoderCount(axis types.AxisName, position float64) float64 {
	ax := c.findAxis(axis)
	if ax == nil {
		return position
	}
	scale := ax.EncoderScale
	if scale == 0 {
		scale = types.DefaultEncoderScale
	}
	return position / scale
}

// encoderCountToEngineering 编码器计数值→工程单位
func (c *B140MotionController) encoderCountToEngineering(axis types.AxisName, count float64) float64 {
	ax := c.findAxis(axis)
	if ax == nil {
		return count
	}
	scale := ax.EncoderScale
	if scale == 0 {
		scale = types.DefaultEncoderScale
	}
	return count * scale
}

// findAxis 查找轴配置。返回的指针指向不可变切片元素，仅可读取。
func (c *B140MotionController) findAxis(axis types.AxisName) *types.AxisConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for i := range c.axes {
		if c.axes[i].Name == axis {
			return &c.axes[i]
		}
	}
	return nil
}

// SetAcceleration B140 不支持 AC 命令，静默 no-op（保留接口兼容性）
func (c *B140MotionController) SetAcceleration(_ types.AxisName, _ float64) error { return nil }

// SetDeceleration B140 不支持 DC 命令，静默 no-op（保留接口兼容性）
func (c *B140MotionController) SetDeceleration(_ types.AxisName, _ float64) error { return nil }

// IsMoving 查询是否有轴在运动（TS命令）
func (c *B140MotionController) IsMoving() (bool, error) {
	resp, err := c.driver.SendCommand("TS")
	if err != nil {
		return false, err
	}
	clean := strings.TrimSpace(resp)
	clean = strings.TrimSuffix(clean, ":")
	parts := strings.Split(clean, ",")
	for _, part := range parts {
		val, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			continue
		}
		if val&0x80 != 0 {
			return true, nil
		}
	}
	return false, nil
}

// IsAxisMoving 查询单轴是否在运动（使用 TS 全轴命令）
func (c *B140MotionController) IsAxisMoving(axis types.AxisName) (bool, error) {
	axisIndex, ok := types.AxisNameToTSIndex[axis]
	if !ok {
		return false, fmt.Errorf("unknown axis: %s", axis)
	}
	resp, err := c.driver.SendCommand("TS")
	if err != nil {
		return false, err
	}
	clean := strings.TrimSpace(resp)
	clean = strings.TrimSuffix(clean, ":")
	parts := strings.Split(clean, ",")
	if axisIndex >= len(parts) {
		return false, nil
	}
	statusVal, err := strconv.Atoi(strings.TrimSpace(parts[axisIndex]))
	if err != nil {
		return false, nil
	}
	return statusVal&0x80 != 0, nil
}

// GetLimitStatus 查询轴限位状态（MG _LFX / MG _LRX）
func (c *B140MotionController) GetLimitStatus(axis types.AxisName) (types.LimitStatus, error) {
	bAxis, ok := types.AxisNameToB140[axis]
	if !ok {
		return types.LimitStatus{}, fmt.Errorf("unknown axis: %s", axis)
	}

	result := types.LimitStatus{}

	// 查询正向限位 MG _LF{axis}
	respPos, err := c.driver.SendCommand(fmt.Sprintf("MG _LF%s", bAxis))
	if err == nil {
		result.PosLimit = parseMGBool(respPos)
	}

	// 查询反向限位 MG _LR{axis}
	respNeg, err := c.driver.SendCommand(fmt.Sprintf("MG _LR%s", bAxis))
	if err == nil {
		result.NegLimit = parseMGBool(respNeg)
	}

	return result, nil
}

// parseMGBool 解析MG命令返回的限位状态值
// MG返回格式: 1.0000:\r (未触发) 或 0.0000:\r (已触发)
// 0.xxxx → limit IS triggered, 1.xxxx → limit NOT triggered
func parseMGBool(resp string) bool {
	clean := strings.TrimSpace(resp)
	clean = strings.TrimPrefix(clean, ":")
	clean = strings.TrimSuffix(clean, ":")
	if clean == "" {
		return false
	}
	val, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return false
	}
	return val < 0.5
}

// WaitForMotionComplete 等待运动完成
func (c *B140MotionController) WaitForMotionComplete(axis types.AxisName, timeoutMs int) error {
	if timeoutMs == 0 {
		timeoutMs = 60000
	}
	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)
	for {
		moving, err := c.IsAxisMoving(axis)
		if err != nil {
			return err
		}
		if !moving {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("motion complete timeout for axis %s", axis)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// MotorOff 减速停止所有轴（B140 不使用 MO 命令；伺服使能仅使用 SH，软停止使用 ST）
func (c *B140MotionController) MotorOff() error {
	_, err := c.driver.SendCommand("ST")
	return err
}

// SetAxisDirection 运行时设置单轴方向
func (c *B140MotionController) SetAxisDirection(axis types.AxisName, reverse bool) error {
	bAxis, ok := types.AxisNameToB140[axis]
	if !ok {
		return fmt.Errorf("unknown axis: %s", axis)
	}

	mtVal := 2
	if reverse {
		mtVal = -2
	}
	if _, err := c.driver.SendCommand(fmt.Sprintf("MT%s=%d", bAxis, mtVal)); err != nil {
		return err
	}

	ceVal := 0
	if reverse {
		ceVal = 2
	}
	_, err := c.driver.SendCommand(fmt.Sprintf("CE%s=%d", bAxis, ceVal))
	return err
}

// ensureAxisDirectionConfigured 配置各轴电机/编码器方向（签名缓存，仅变更时重发）
func (c *B140MotionController) ensureAxisDirectionConfigured() error {
	sig := c.buildDirectionSignature()
	c.mu.RLock()
	cached := c.directionSignature
	c.mu.RUnlock()
	if sig == cached {
		return nil
	}
	for _, ax := range c.axesSnapshot() {
		if !ax.Enabled {
			continue
		}
		if err := c.SetAxisDirection(ax.Name, ax.Inverted); err != nil {
			return err
		}
	}
	c.mu.Lock()
	c.directionSignature = sig
	c.mu.Unlock()
	return nil
}

// buildDirectionSignature 构建轴方向配置签名
func (c *B140MotionController) buildDirectionSignature() string {
	axes := c.axesSnapshot()
	parts := make([]string, 0, len(axes))
	for _, ax := range axes {
		inverted := 0
		if ax.Inverted {
			inverted = 1
		}
		parts = append(parts, fmt.Sprintf("%s:%d", ax.Name, inverted))
	}
	return strings.Join(parts, "|")
}
