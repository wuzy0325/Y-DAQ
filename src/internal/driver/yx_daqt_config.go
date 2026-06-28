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
	ThermocoupleTypes  string
	ChannelMask        string
	SamplingRate       int
	BinaryFormat       bool
	ShowTimestamp      bool
	ShowSequence       bool
	AverageCount       int
	TriggerMode        int
	TriggerEdge        int
	TriggerCount       int
	IsTempModel        bool // temp 型号固件不支持 BIN=1
	OpenCircuitCheck   string
}

// sendCommand sends a command and waits for response using the pending queue
func (d *YXDAQTDriver) sendCommand(cmd string) (string, error) {
	return d.sendCommandWithType(cmd, ResponseNewline, 0)
}

// sendCommandExact sends a command expecting a fixed-length response
func (d *YXDAQTDriver) sendCommandExact(cmd string, expectedLen int) (string, error) {
	return d.sendCommandWithType(cmd, ResponseFixedLength, expectedLen)
}

// sendCommandSilence sends a command expecting a variable-length response with silence window
func (d *YXDAQTDriver) sendCommandSilence(cmd string) (string, error) {
	return d.sendCommandWithType(cmd, ResponseSilenceWindow, 0)
}

// sendCommandWithType sends a command and registers a pending response expectation
func (d *YXDAQTDriver) sendCommandWithType(cmd string, respType ResponseType, expectedLen int) (string, error) {
	if !d.connected.Load() || d.Conn == nil {
		return "", fmt.Errorf("device not connected")
	}

	fullCmd := cmd + types.DAQTCmdTerminator
	respCh := make(chan string, 1)

	entry := &PendingEntry{
		Cmd:         cmd,
		RespType:    respType,
		ExpectedLen: expectedLen,
		SilenceMs:   30,
		RespCh:      respCh,
		Deadline:    time.Now().Add(5 * time.Second),
	}

	d.pending.Push(entry)

	d.mu.Lock()
	_, err := d.Conn.Write([]byte(fullCmd))
	d.mu.Unlock()

	if err != nil {
		// Remove the entry we just pushed (match by cmd to avoid FIFO disorder)
		d.pending.RemoveByCmd(cmd)
		return "", fmt.Errorf("send command %q failed: %w", cmd, err)
	}

	select {
	case resp := <-respCh:
		if resp == "" {
			return "", fmt.Errorf("command %q timeout", cmd)
		}
		// ★ 错误响应 E：终止当前操作并上报错误，不应重试同一命令
		if strings.ToUpper(resp) == "E" {
			return "", fmt.Errorf("device rejected command %q (error response E)", cmd)
		}
		return resp, nil
	case <-time.After(5 * time.Second):
		return "", fmt.Errorf("command %q timeout", cmd)
	}
}

// writeCmdOnly 仅发送命令，不等待响应
func (d *YXDAQTDriver) writeCmdOnly(cmd string) error {
	return d.WriteCommandOnly(cmd + types.DAQTCmdTerminator)
}

// syncHardwareConfig 连接后同步硬件配置
func (d *YXDAQTDriver) syncHardwareConfig() {
	config := DAQTHardwareConfig{}

	if resp, err := d.sendCommandExact("@e3", 16); err == nil {
		config.ThermocoupleTypes = resp
	}
	if resp, err := d.sendCommandExact("@fd MCH", 4); err == nil {
		config.ChannelMask = resp
	}
	if resp, err := d.sendCommandSilence("@fd SPS"); err == nil {
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
	if resp, err := d.sendCommandSilence("@fd AVG"); err == nil {
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
	if resp, err := d.sendCommandSilence("@fd TNUM"); err == nil {
		if v, e := strconv.Atoi(trimSpace(resp)); e == nil {
			config.TriggerCount = v
		}
	}
	if resp, err := d.sendCommandExact("@fd CHECK", 4); err == nil {
		config.OpenCircuitCheck = resp
	}

	// 连接后主动设置 BIN=1，启用二进制采集模式
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

	d.hwConfig = config
	d.frameReader.SetBinaryMode(config.BinaryFormat)
	d.frameReader.SetMetadataMode(config.ShowTimestamp || config.ShowSequence)

	// 将热电偶类型同步到通道配置
	if len(config.ThermocoupleTypes) == 16 {
		d.mu.Lock()
		for i := range d.channels {
			if i < 16 {
				d.channels[i].ThermocoupleType = string(config.ThermocoupleTypes[i])
			}
		}
		d.mu.Unlock()
	}

	// 必须在 d.mu 保护下设置标志并广播，否则会与 StartAcquisition 的 Wait()
	// 产生丢失唤醒竞态，导致开始采集永久卡死
	d.mu.Lock()
	d.configSyncDone = true
	d.configSyncCond.Broadcast()
	d.mu.Unlock()

	// 通知外部配置已就绪
	if d.onConfigSynced != nil {
		d.onConfigSynced(config)
	}
}

// applyNormalizedConfig 采集启动前归一化配置
// ★ 只确保 BIN=1（如果设备支持），不自动修改 TIME/HEAD，保持用户配置不变
func (d *YXDAQTDriver) applyNormalizedConfig() error {
	if d.hwConfig.BinaryFormat {
		d.writeCmdOnly("@fe BIN 1")
		time.Sleep(50 * time.Millisecond)
	}
	// ★ 注意：不自动发 @fe TIME/HEAD，保持用户配置不变
	d.frameReader.SetBinaryMode(d.hwConfig.BinaryFormat)
	d.frameReader.SetMetadataMode(d.hwConfig.ShowTimestamp || d.hwConfig.ShowSequence)

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

// SetSingleThermocoupleType 设置单个通道的热电偶类型
// channelIndex: 0-15, tcType: 热电偶类型字符
func (d *YXDAQTDriver) SetSingleThermocoupleType(channelIndex int, tcType string) error {
	if channelIndex < 0 || channelIndex > 15 {
		return fmt.Errorf("channel index must be 0-15, got %d", channelIndex)
	}

	// 读取当前所有通道的热电偶类型
	current := d.hwConfig.ThermocoupleTypes
	if len(current) != 16 {
		current = "KKKKKKKKKKKKKKKK" // 默认全 K
	}

	// 修改指定通道
	runes := []rune(current)
	runes[channelIndex] = []rune(tcType)[0]
	newTypes := string(runes)

	// 发送完整的热电偶类型命令
	return d.SetThermocoupleType(newTypes)
}

// SetTemperatureUnit 设置温度单位（℃/℉/K）
// 更新所有温度通道的单位，并重新计算量程
func (d *YXDAQTDriver) SetTemperatureUnit(unit string) error {
	validUnits := map[string]bool{"°C": true, "°F": true, "K": true}
	if !validUnits[unit] {
		return fmt.Errorf("unsupported temperature unit: %s (supported: °C, °F, K)", unit)
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	for i := range d.channels {
		oldUnit := d.channels[i].Unit
		if oldUnit == "°C" || oldUnit == "°F" || oldUnit == "K" {
			// Convert range bounds
			d.channels[i].RangeMin = convertTemperature(d.channels[i].RangeMin, oldUnit, unit)
			d.channels[i].RangeMax = convertTemperature(d.channels[i].RangeMax, oldUnit, unit)
			d.channels[i].Unit = unit
		}
	}
	return nil
}

// convertTemperature converts a temperature value between units
func convertTemperature(value float64, from, to string) float64 {
	if from == to {
		return value
	}
	// First convert to Celsius
	var celsius float64
	switch from {
	case "°C":
		celsius = value
	case "°F":
		celsius = (value - 32) * 5 / 9
	case "K":
		celsius = value - 273.15
	}
	// Then convert from Celsius to target
	switch to {
	case "°C":
		return celsius
	case "°F":
		return celsius*9/5 + 32
	case "K":
		return celsius + 273.15
	}
	return value
}
