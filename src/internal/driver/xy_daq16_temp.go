package driver

import (
	"fmt"
	"log/slog"

	"yx-daq/internal/types"
)

// EA2508A 温度通道配置（参考 LabVIEW 协议）：
//   - @16<12|13|14> 温度传感器来源：12=内部温度传感器 13=外界热电偶传感器 14=外接PT100传感器
//   - @17<T|K|J|E|S> 热电偶类型（仅外界热电偶来源时有意义）
// 温度通道为最后一个通道，即 index = totalChannels - 1 = pressureCount + 1（大气压之后的通道）。

// tempChannelIndex 返回温度通道索引（最后一个通道）
func (d *XYDAQDriver) tempChannelIndex() int {
	return d.totalChannels - 1
}

// getTempChannelConfig 读取温度通道配置快照（加锁）
func (d *XYDAQDriver) getTempChannelConfig() (types.ChannelConfig, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	idx := d.tempChannelIndex()
	for i := range d.channels {
		if d.channels[i].Index == idx {
			return d.channels[i], true
		}
	}
	return types.ChannelConfig{}, false
}

// SetTempSource 设置温度通道传感器来源（@16 命令，写入硬件）
// source: internal / thermocouple / pt100。设备响应须为 A，否则返回错误。
func (d *XYDAQDriver) SetTempSource(source string) error {
	code, ok := types.TempSource(source).CmdCode()
	if !ok {
		return fmt.Errorf("不支持的温度源: %s（支持: internal/thermocouple/pt100）", source)
	}

	resp, err := d.sendUnitCommand(fmt.Sprintf("@16%d", code))
	if err != nil {
		return fmt.Errorf("设置温度源失败: %w", err)
	}
	if resp != "A" {
		return fmt.Errorf("设置温度源被设备拒绝: %s", resp)
	}

	d.mu.Lock()
	for i := range d.channels {
		if d.channels[i].Index == d.tempChannelIndex() {
			d.channels[i].TempSource = source
		}
	}
	d.mu.Unlock()
	slog.Info("XY-DAQ temp source set", "host", d.Host, "port", d.Port, "source", source)
	return nil
}

// SetTempThermocoupleType 设置温度通道热电偶类型（@17 命令，写入硬件）
// tcType: T/K/J/E/S。内部温度传感器来源下无意义，由上层拦截。
func (d *XYDAQDriver) SetTempThermocoupleType(tcType string) error {
	if !types.EA2508ATempThermocoupleTypes[tcType] {
		return fmt.Errorf("不支持的热电偶类型: %s（支持: T/K/J/E/S）", tcType)
	}

	resp, err := d.sendUnitCommand("@17" + tcType)
	if err != nil {
		return fmt.Errorf("设置热电偶类型失败: %w", err)
	}
	if resp != "A" {
		return fmt.Errorf("设置热电偶类型被设备拒绝: %s", resp)
	}

	d.mu.Lock()
	for i := range d.channels {
		if d.channels[i].Index == d.tempChannelIndex() {
			d.channels[i].ThermocoupleType = tcType
		}
	}
	d.mu.Unlock()
	slog.Info("XY-DAQ temp thermocouple type set", "host", d.Host, "port", d.Port, "type", tcType)
	return nil
}

// applyTempChannelConfig 连接后按持久化配置下发温度通道配置（@16 → @17）。
// 失败仅告警不阻断连接；@17 仅在来源为外界热电偶时下发。
func (d *XYDAQDriver) applyTempChannelConfig() {
	cfg, ok := d.getTempChannelConfig()
	if !ok {
		return
	}

	if cfg.TempSource != "" {
		if err := d.SetTempSource(cfg.TempSource); err != nil {
			slog.Warn("XY-DAQ apply temp source on connect failed", "host", d.Host, "port", d.Port, "err", err)
		}
	}
	if cfg.TempSource == string(types.TempSourceThermocouple) && cfg.ThermocoupleType != "" {
		if err := d.SetTempThermocoupleType(cfg.ThermocoupleType); err != nil {
			slog.Warn("XY-DAQ apply temp thermocouple type on connect failed", "host", d.Host, "port", d.Port, "err", err)
		}
	}
}
