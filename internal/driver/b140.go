package driver

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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
		return "", fmt.Errorf("send command %q failed: %w", cmd, err)
	}

	// 读取响应（以\r结尾）
	resp, err := d.reader.ReadString('\r')
	if err != nil {
		return "", fmt.Errorf("read response for %q failed: %w", cmd, err)
	}

	// 检查B140响应格式: ':'成功, '?'错误
	if len(resp) > 0 && resp[0] == '?' {
		return resp, fmt.Errorf("B140 command error: %s", cmd)
	}

	return resp, nil
}

// B140MotionController B140运动控制器（高层接口）
type B140MotionController struct {
	driver   *B140Driver
	axes     []types.AxisConfig
	axisDir  map[types.AxisName]bool // true=已配置方向
}

// NewB140MotionController 创建B140运动控制器
func NewB140MotionController(driver *B140Driver, axes []types.AxisConfig) *B140MotionController {
	return &B140MotionController{
		driver:  driver,
		axes:    axes,
		axisDir: make(map[types.AxisName]bool),
	}
}

// Connect 连接并使能所有轴
func (mc *B140MotionController) Connect() error {
	if err := mc.driver.Connect(); err != nil {
		return err
	}

	// 使能所有轴
	if _, err := mc.driver.SendCommand("SH"); err != nil {
		return fmt.Errorf("servo enable failed: %w", err)
	}

	// 配置各轴方向
	if err := mc.ensureAxisDirectionConfigured(); err != nil {
		log.Printf("axis direction config warning: %v", err)
	}

	return nil
}

// Disconnect 断开连接
func (mc *B140MotionController) Disconnect() {
	mc.driver.Disconnect()
}

// IsConnected 是否已连接
func (mc *B140MotionController) IsConnected() bool {
	return mc.driver.IsConnected()
}

// MoveTo 绝对定位移动
func (mc *B140MotionController) MoveTo(axis types.AxisName, position float64) error {
	bAxis, ok := types.AxisNameToB140[axis]
	if !ok {
		return fmt.Errorf("unknown axis: %s", axis)
	}

	pulse := mc.engineeringToPulse(axis, position)
	cmd := fmt.Sprintf("PA%s=%d", bAxis, int(pulse))
	if _, err := mc.driver.SendCommand(cmd); err != nil {
		return err
	}

	_, err := mc.driver.SendCommand(fmt.Sprintf("BG%s", bAxis))
	return err
}

// MoveBy 相对增量移动
func (mc *B140MotionController) MoveBy(axis types.AxisName, delta float64) error {
	bAxis, ok := types.AxisNameToB140[axis]
	if !ok {
		return fmt.Errorf("unknown axis: %s", axis)
	}

	pulse := mc.engineeringToPulse(axis, delta)
	cmd := fmt.Sprintf("PR%s=%d", bAxis, int(pulse))
	if _, err := mc.driver.SendCommand(cmd); err != nil {
		return err
	}

	_, err := mc.driver.SendCommand(fmt.Sprintf("BG%s", bAxis))
	return err
}

// Jog 点动
func (mc *B140MotionController) Jog(axis types.AxisName, direction int, speed float64) error {
	bAxis, ok := types.AxisNameToB140[axis]
	if !ok {
		return fmt.Errorf("unknown axis: %s", axis)
	}

	if speed > 0 {
		pulseSpeed := mc.engineeringToPulse(axis, speed)
		if _, err := mc.driver.SendCommand(fmt.Sprintf("SP%s=%d", bAxis, int(pulseSpeed))); err != nil {
			return err
		}
	}

	if direction > 0 {
		_, err := mc.driver.SendCommand(fmt.Sprintf("PR%s=10000", bAxis))
		return err
	}
	_, err := mc.driver.SendCommand(fmt.Sprintf("PR%s=-10000", bAxis))
	return err
}

// Home 单轴回零
func (mc *B140MotionController) Home(axis types.AxisName) error {
	bAxis, ok := types.AxisNameToB140[axis]
	if !ok {
		return fmt.Errorf("unknown axis: %s", axis)
	}
	_, err := mc.driver.SendCommand(fmt.Sprintf("HM%s", bAxis))
	return err
}

// Stop 停止运动
func (mc *B140MotionController) Stop(axis types.AxisName) error {
	bAxis, ok := types.AxisNameToB140[axis]
	if !ok {
		return fmt.Errorf("unknown axis: %s", axis)
	}
	_, err := mc.driver.SendCommand(fmt.Sprintf("ST%s", bAxis))
	return err
}

// StopAll 停止所有轴
func (mc *B140MotionController) StopAll() error {
	_, err := mc.driver.SendCommand("ST")
	return err
}

// EmergencyStop 急停
func (mc *B140MotionController) EmergencyStop() error {
	_, err := mc.driver.SendCommand("AB")
	return err
}

// DefinePosition 置位（DP+DE）
func (mc *B140MotionController) DefinePosition(axis types.AxisName, position float64) error {
	bAxis, ok := types.AxisNameToB140[axis]
	if !ok {
		return fmt.Errorf("unknown axis: %s", axis)
	}

	pulse := mc.engineeringToPulse(axis, position)
	if _, err := mc.driver.SendCommand(fmt.Sprintf("DP%s=%d", bAxis, int(pulse))); err != nil {
		return err
	}
	_, err := mc.driver.SendCommand(fmt.Sprintf("DE%s=%d", bAxis, int(pulse)))
	return err
}

// GetAxisStatus 查询单轴状态（位置+运动状态+限位状态）
func (mc *B140MotionController) GetAxisStatus(axis types.AxisName) (types.AxisStatus, error) {
	bAxis, ok := types.AxisNameToB140[axis]
	if !ok {
		return types.AxisStatus{}, fmt.Errorf("unknown axis: %s", axis)
	}

	status := types.AxisStatus{Name: axis}

	// 读取编码器位置 (TP)
	resp, err := mc.driver.SendCommand(fmt.Sprintf("TP%s", bAxis))
	if err == nil {
		var pulse int
		fmt.Sscanf(resp, ":%d", &pulse)
		status.Position = mc.pulseToEngineering(axis, float64(pulse))
	}

	// 读取运动状态 (TS)
	moving, _ := mc.IsAxisMoving(axis)
	status.Moving = moving

	// 读取限位状态 (MG _LFX / MG _LRX)
	limitStatus, _ := mc.GetLimitStatus(axis)
	status.PosLimit = limitStatus.PosLimit
	status.NegLimit = limitStatus.NegLimit

	return status, nil
}

// GetAllAxisStatus 查询所有轴状态
func (mc *B140MotionController) GetAllAxisStatus() ([]types.AxisStatus, error) {
	statuses := []types.AxisStatus{}
	for _, ax := range mc.axes {
		if !ax.Enabled {
			continue
		}
		s, err := mc.GetAxisStatus(ax.Name)
		if err != nil {
			statuses = append(statuses, types.AxisStatus{Name: ax.Name})
			continue
		}
		statuses = append(statuses, s)
	}
	return statuses, nil
}

// SetSpeed 设置速度
func (mc *B140MotionController) SetSpeed(axis types.AxisName, speed float64) error {
	bAxis, ok := types.AxisNameToB140[axis]
	if !ok {
		return fmt.Errorf("unknown axis: %s", axis)
	}
	pulseSpeed := mc.engineeringToPulse(axis, speed)
	_, err := mc.driver.SendCommand(fmt.Sprintf("SP%s=%d", bAxis, int(pulseSpeed)))
	return err
}

// engineeringToPulse 工程单位→脉冲
func (mc *B140MotionController) engineeringToPulse(axis types.AxisName, position float64) float64 {
	ax := mc.findAxis(axis)
	if ax == nil {
		return position
	}

	stepsPerRev := 360.0 / ax.StepAngleDeg
	pulsesPerRev := stepsPerRev * float64(ax.MicroSteps)

	var pulsesPerUnit float64
	if ax.Kind == types.AxisKindLinear {
		pulsesPerUnit = pulsesPerRev / ax.Lead
	} else {
		pulsesPerUnit = pulsesPerRev / 360.0
	}

	return position * pulsesPerUnit
}

// pulseToEngineering 脉冲→工程单位
func (mc *B140MotionController) pulseToEngineering(axis types.AxisName, pulses float64) float64 {
	ax := mc.findAxis(axis)
	if ax == nil {
		return pulses
	}

	stepsPerRev := 360.0 / ax.StepAngleDeg
	pulsesPerRev := stepsPerRev * float64(ax.MicroSteps)

	var pulsesPerUnit float64
	if ax.Kind == types.AxisKindLinear {
		pulsesPerUnit = pulsesPerRev / ax.Lead
	} else {
		pulsesPerUnit = pulsesPerRev / 360.0
	}

	if pulsesPerUnit == 0 {
		return 0
	}
	return pulses / pulsesPerUnit
}

// findAxis 查找轴配置
func (mc *B140MotionController) findAxis(axis types.AxisName) *types.AxisConfig {
	for i := range mc.axes {
		if mc.axes[i].Name == axis {
			return &mc.axes[i]
		}
	}
	return nil
}

// SetAcceleration 设置加速度
func (mc *B140MotionController) SetAcceleration(axis types.AxisName, accel float64) error {
	bAxis, ok := types.AxisNameToB140[axis]
	if !ok {
		return fmt.Errorf("unknown axis: %s", axis)
	}
	pulseAccel := mc.engineeringToPulse(axis, accel)
	_, err := mc.driver.SendCommand(fmt.Sprintf("AC%s=%d", bAxis, int(pulseAccel)))
	return err
}

// SetDeceleration 设置减速度
func (mc *B140MotionController) SetDeceleration(axis types.AxisName, decel float64) error {
	bAxis, ok := types.AxisNameToB140[axis]
	if !ok {
		return fmt.Errorf("unknown axis: %s", axis)
	}
	pulseDecel := mc.engineeringToPulse(axis, decel)
	_, err := mc.driver.SendCommand(fmt.Sprintf("DC%s=%d", bAxis, int(pulseDecel)))
	return err
}

// IsMoving 查询是否有轴在运动（TS命令）
func (mc *B140MotionController) IsMoving() (bool, error) {
	resp, err := mc.driver.SendCommand("TS")
	if err != nil {
		return false, err
	}
	// TS返回格式: :AABBCCDD\r 其中每2位是一个轴的状态
	// 运动位是每个轴状态字的bit7 (0x80)
	clean := strings.TrimSpace(resp)
	clean = strings.TrimPrefix(clean, ":")
	if len(clean) < 2 {
		return false, nil
	}
	// 检查每个轴状态
	for i := 0; i+1 < len(clean); i += 2 {
		statusStr := clean[i : i+1]
		statusVal, err := strconv.ParseInt(statusStr, 16, 8)
		if err != nil {
			continue
		}
		if statusVal&0x08 != 0 { // bit3 = 运动中
			return true, nil
		}
	}
	return false, nil
}

// IsAxisMoving 查询单轴是否在运动
func (mc *B140MotionController) IsAxisMoving(axis types.AxisName) (bool, error) {
	bAxis, ok := types.AxisNameToB140[axis]
	if !ok {
		return false, fmt.Errorf("unknown axis: %s", axis)
	}
	resp, err := mc.driver.SendCommand(fmt.Sprintf("TS%s", bAxis))
	if err != nil {
		return false, err
	}
	clean := strings.TrimSpace(resp)
	clean = strings.TrimPrefix(clean, ":")
	if len(clean) == 0 {
		return false, nil
	}
	statusVal, err := strconv.ParseInt(string(clean[0]), 16, 8)
	if err != nil {
		return false, nil
	}
	return statusVal&0x08 != 0, nil
}

// GetLimitStatus 查询轴限位状态（MG _LFX / MG _LRX）
func (mc *B140MotionController) GetLimitStatus(axis types.AxisName) (types.LimitStatus, error) {
	bAxis, ok := types.AxisNameToB140[axis]
	if !ok {
		return types.LimitStatus{}, fmt.Errorf("unknown axis: %s", axis)
	}

	result := types.LimitStatus{}

	// 查询正向限位 MG _LF{axis}
	respPos, err := mc.driver.SendCommand(fmt.Sprintf("MG _LF%s", bAxis))
	if err == nil {
		result.PosLimit = parseMGBool(respPos)
	}

	// 查询反向限位 MG _LR{axis}
	respNeg, err := mc.driver.SendCommand(fmt.Sprintf("MG _LR%s", bAxis))
	if err == nil {
		result.NegLimit = parseMGBool(respNeg)
	}

	return result, nil
}

// parseMGBool 解析MG命令返回的布尔值
// MG返回格式如: 1.0000\r 或 0.0000\r
func parseMGBool(resp string) bool {
	clean := strings.TrimSpace(resp)
	clean = strings.TrimPrefix(clean, ":")
	val, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return false
	}
	return val != 0
}

// WaitForMotionComplete 等待运动完成
func (mc *B140MotionController) WaitForMotionComplete(axis types.AxisName, timeoutMs int) error {
	if timeoutMs == 0 {
		timeoutMs = 60000
	}
	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)
	for {
		moving, err := mc.IsAxisMoving(axis)
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

// MotorOff 关闭所有轴电机（MO命令）
func (mc *B140MotionController) MotorOff() error {
	_, err := mc.driver.SendCommand("MO")
	return err
}

// SetAxisDirection 运行时设置单轴方向
func (mc *B140MotionController) SetAxisDirection(axis types.AxisName, reverse bool) error {
	bAxis, ok := types.AxisNameToB140[axis]
	if !ok {
		return fmt.Errorf("unknown axis: %s", axis)
	}

	mtVal := 2
	if reverse {
		mtVal = -2
	}
	if _, err := mc.driver.SendCommand(fmt.Sprintf("MT%s=%d", bAxis, mtVal)); err != nil {
		return err
	}

	ceVal := 0
	if reverse {
		ceVal = 2
	}
	_, err := mc.driver.SendCommand(fmt.Sprintf("CE%s=%d", bAxis, ceVal))
	return err
}

// ensureAxisDirectionConfigured 配置各轴电机/编码器方向
func (mc *B140MotionController) ensureAxisDirectionConfigured() error {
	for _, ax := range mc.axes {
		if !ax.Enabled {
			continue
		}
		if err := mc.SetAxisDirection(ax.Name, ax.Inverted); err != nil {
			return err
		}
		mc.axisDir[ax.Name] = true
	}
	return nil
}
