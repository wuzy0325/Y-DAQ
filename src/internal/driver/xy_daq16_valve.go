package driver

import (
	"fmt"
	"strconv"
	"strings"

	"yx-daq/internal/types"
)

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
