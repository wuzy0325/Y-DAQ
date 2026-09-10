package driver

import (
	"fmt"
	"log/slog"
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

// sendCommandACK 发送设置类命令（@fe/@f3 等）并校验单字节 ACK 响应。
// 设备返回 'A' 表示成功，'E' 表示拒绝（sendCommandWithType 已处理），
// 其他字节视为协议错误。
// 调用方不得持有 d.mu（sendCommandWithType 内部加锁 d.mu，会导致自死锁）。
func (d *YXDAQTDriver) sendCommandACK(cmd string) error {
	resp, err := d.sendCommandExact(cmd, 1)
	if err != nil {
		return err
	}
	if trimSpace(resp) != "A" {
		return fmt.Errorf("命令 %s 的 ACK 应答无效: %q（应为 A）", cmd, resp)
	}
	return nil
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
// 参考实机协议（device-lab/skills/daq-t1603/SKILL.md）：
//   - @e3 响应 = 16 字节类型数据 + 单个 LF = 17 字节
//   - @fd MCH 读回值是设备端持久化的历史掩码，不代表应用配置，不写回
//   - @fe BIN 1 必须用 ACK 校验 + readback 确认（temp 固件 ACK 但不切换）
//   - @fe TIME 按当前配置设置（FrameReader 支持 72 字节带时间戳帧）
//   - @fe HEAD 0 强制关闭序号帧（FrameReader 不支持 68 字节序号帧）
func (d *YXDAQTDriver) syncHardwareConfig() {
	config := DAQTHardwareConfig{}

	// @e3: 实机响应 16 字节类型数据 + 单个 LF，共 17 字节
	if resp, err := d.sendCommandExact("@e3", 17); err == nil {
		value := strings.TrimSuffix(resp, "\n")
		value = trimSpace(value)
		if len(value) == 16 {
			config.ThermocoupleTypes = value
		}
	}

	// @fd MCH: 不写回 config.ChannelMask。
	// 读回值是设备端持久化的历史掩码（如出厂遗留 "0000"），不代表应用配置；
	// 若写回 "0000" 会导致后续 @f0 0000 2 零通道采集。
	// 仍读取以维持协议响应边界与后续命令时序。
	if _, err := d.sendCommandExact("@fd MCH", 4); err == nil {
		// 故意不写回 ChannelMask
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
	if resp, err := d.sendCommandSilence("@fd SPS"); err == nil {
		if v, e := strconv.Atoi(trimSpace(resp)); e == nil {
			config.SamplingRate = v
		}
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

	// 强制 BIN=1 启用二进制采集模式（16×float32 LE，64 字节帧，效率最高）。
	// @fe 命令返回单字节 ACK（A=成功/E=拒绝），用 sendCommandACK 读取校验。
	if err := d.sendCommandACK("@fe BIN 1"); err != nil {
		slog.Warn("DAQ-T: 强制 BIN=1 失败", "host", d.Host, "port", d.Port, "err", err)
	} else {
		// readback 验证：temp 型号固件对 @fe BIN 1 仍回 ACK 'A' 但实际不切换，
		// 必须读 @fd BIN 确认实际状态，否则 FrameReader 按 64 字节二进制解析
		// 设备实际发送的 ASCII 帧会导致帧错位。
		time.Sleep(50 * time.Millisecond)
		if resp, err := d.sendCommandExact("@fd BIN", 1); err == nil {
			val := trimSpace(resp)
			switch val {
			case "1":
				config.BinaryFormat = true
			case "0":
				config.IsTempModel = true
				config.BinaryFormat = false
				slog.Warn("DAQ-T: BIN=1 未生效（temp 固件？），回退 ASCII 模式",
					"host", d.Host, "port", d.Port)
			default:
				slog.Warn("DAQ-T: @fd BIN readback 返回非法值",
					"host", d.Host, "port", d.Port, "resp", val)
			}
		}
	}

	// 强制 TIME 按当前配置设置（FrameReader 支持 BIN=1+TIME=1 的 72 字节帧）。
	desiredTime := 0
	if config.ShowTimestamp {
		desiredTime = 1
	}
	if err := d.sendCommandACK(fmt.Sprintf("@fe TIME %d", desiredTime)); err != nil {
		slog.Warn("DAQ-T: 设置 TIME 失败", "host", d.Host, "port", d.Port, "err", err)
	}

	// 强制 HEAD=0：当前 FrameReader 不支持 BIN=1+HEAD=1 的 68 字节序号帧，
	// 设备持久化的 HEAD=1 会导致 readBinaryTimestampFrame 按 72 字节解析 68 字节帧→错位。
	if err := d.sendCommandACK("@fe HEAD 0"); err != nil {
		slog.Warn("DAQ-T: 强制 HEAD=0 失败", "host", d.Host, "port", d.Port, "err", err)
	} else {
		config.ShowSequence = false
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
// periodMs: 采集周期(毫秒)，用于设置硬件采样率 SPS = 1000/periodMs
func (d *YXDAQTDriver) applyNormalizedConfig(periodMs int) error {
	if d.hwConfig.BinaryFormat {
		d.writeCmdOnly("@fe BIN 1")
		time.Sleep(50 * time.Millisecond)
	}

	// 根据 periodMs 设置硬件采样率（SPS = 1000 / periodMs）
	if periodMs > 0 {
		sps := 1000 / periodMs
		if sps < 1 {
			sps = 1
		}
		if sps > 1000 {
			sps = 1000
		}
		if err := d.writeCmdOnly(fmt.Sprintf("@fe SPS %d", sps)); err != nil {
			return fmt.Errorf("设置采样频率失败: %w", err)
		}
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
// @f3 命令返回单字节 ACK（A=成功/E=拒绝），用 sendCommandACK 读取校验。
// 发送后需 readback 验证（读 @e3 比对），不匹配时返回错误并同步 hwConfig.ThermocoupleTypes。
// 调用方不得持有 d.mu（sendCommandACK 内部加锁 d.mu，会导致自死锁）。
func (d *YXDAQTDriver) SetThermocoupleType(tcTypes string) error {
	if len(tcTypes) != 16 {
		return fmt.Errorf("thermocouple types must be 16 characters, got %d", len(tcTypes))
	}
	cmd := fmt.Sprintf("@f3 0%s0", tcTypes)
	if err := d.sendCommandACK(cmd); err != nil {
		return fmt.Errorf("set thermocouple type failed: %w", err)
	}

	// readback 验证：发 @f3 命令后读 @e3 比对，确保设备实际接受了配置。
	// 某些固件可能 ACK 'A' 但不实际切换，readback 可暴露这种静默失败。
	time.Sleep(50 * time.Millisecond)
	if resp, err := d.sendCommandExact("@e3", 17); err == nil {
		value := strings.TrimSuffix(resp, "\n")
		value = trimSpace(value)
		if len(value) == 16 && value != tcTypes {
			// readback 不匹配：同步 hwConfig 为设备实际值，让上层感知真实状态
			d.mu.Lock()
			d.hwConfig.ThermocoupleTypes = value
			d.mu.Unlock()
			return fmt.Errorf("热电偶类型回读不一致: 已发送 %q, 设备返回 %q", tcTypes, value)
		}
		if len(value) == 16 {
			d.mu.Lock()
			d.hwConfig.ThermocoupleTypes = value
			d.mu.Unlock()
		}
	}
	return nil
}

// SetSingleThermocoupleType 设置单个通道的热电偶类型
// channelIndex: 0-15, tcType: 热电偶类型字符（K/J/T/E/N/S/R/B）
func (d *YXDAQTDriver) SetSingleThermocoupleType(channelIndex int, tcType string) error {
	if channelIndex < 0 || channelIndex > 15 {
		return fmt.Errorf("channel index must be 0-15, got %d", channelIndex)
	}
	validTypes := map[string]bool{"K": true, "J": true, "T": true, "E": true, "N": true, "S": true, "R": true, "B": true}
	if !validTypes[tcType] {
		return fmt.Errorf("unsupported thermocouple type: %s (supported: K, J, T, E, N, S, R, B)", tcType)
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

// SetSamplingRate 设置硬件采样率（SPS = 1000/periodMs，即采集间隔毫秒）。
// 采集中拒绝设置以避免帧错位/缓冲区污染。
// @fe SPS <value> 返回单字节 ACK，用 sendCommandACK 校验；
// 发送后 readback @fd SPS 确认实际值，不一致时返回错误或同步实际值。
// 调用方不得持有 d.mu（sendCommandACK 内部加锁 d.mu，会导致自死锁）。
func (d *YXDAQTDriver) SetSamplingRate(periodMs int) error {
	if d.IsAcquiring() {
		return fmt.Errorf("采集进行中，不允许设置采样频率")
	}
	if periodMs <= 0 {
		return fmt.Errorf("采样周期必须为正数，当前 %d", periodMs)
	}
	sps := 1000 / periodMs
	if sps < 1 {
		sps = 1
	}
	if sps > 1000 {
		sps = 1000
	}

	if err := d.sendCommandACK(fmt.Sprintf("@fe SPS %d", sps)); err != nil {
		return fmt.Errorf("设置采样频率失败: %w", err)
	}

	// readback 验证：读 @fd SPS 确认设备实际采样率。
	// @fd SPS 响应无分隔符且长度可变，用静默窗口读取。
	time.Sleep(50 * time.Millisecond)
	if resp, err := d.sendCommandSilence("@fd SPS"); err == nil {
		if v, e := strconv.Atoi(trimSpace(resp)); e == nil {
			d.mu.Lock()
			d.hwConfig.SamplingRate = v
			d.mu.Unlock()
			if v != sps {
				return fmt.Errorf("采样频率回读不一致: 已发送 %d, 设备返回 %d", sps, v)
			}
		}
	}
	return nil
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
