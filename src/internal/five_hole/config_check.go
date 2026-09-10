package five_hole

import (
	"fmt"

	"yx-daq/internal/types"
)

// CheckMotionConflict 检查五孔配置中各探针位移机构轴是否冲突
// （同一控制器的同一轴不能被多个探针同时使用）
func CheckMotionConflict(config types.FiveHoleTraversalConfig) error {
	axisMap := make(map[string]string) // key: "controllerID:axis" -> probeID
	for _, probe := range config.Probes {
		if !probe.Enabled {
			continue
		}
		xKey := probe.MotionX.ControllerID + ":" + string(probe.MotionX.Axis)
		if owner, exists := axisMap[xKey]; exists {
			return fmt.Errorf("位移机构 %s 的 %s 轴同时被探针 %s 和 %s 使用",
				probe.MotionX.ControllerID, probe.MotionX.Axis, owner, probe.ProbeID)
		}
		axisMap[xKey] = probe.ProbeID
		yKey := probe.MotionY.ControllerID + ":" + string(probe.MotionY.Axis)
		if owner, exists := axisMap[yKey]; exists {
			return fmt.Errorf("位移机构 %s 的 %s 轴同时被探针 %s 和 %s 使用",
				probe.MotionY.ControllerID, probe.MotionY.Axis, owner, probe.ProbeID)
		}
		axisMap[yKey] = probe.ProbeID
	}
	return nil
}

// CheckDeviceChannelOverlap 检查五孔配置中同一采集设备通道是否冲突。
// 返回冲突警告文案；无冲突返回空串（仅警告不拦截启动）。
func CheckDeviceChannelOverlap(config types.FiveHoleTraversalConfig) string {
	// 收集所有启用通道：key: "deviceID:channel" -> role（含探针级 PAtm/TAtm 数据源）
	chMap := make(map[string]string)
	// registerAtmChannel 登记探针级大气数据源占用的通道；与其他角色冲突时返回警告文案
	registerAtmChannel := func(role, name string, src types.FiveHoleAtmSource) string {
		if src.Mode == types.FiveHoleSourceManual || src.DeviceID == "" {
			return "" // 手动模式或未配置设备，不占用通道
		}
		key := fmt.Sprintf("%s:%d", src.DeviceID, src.Channel)
		if existing, exists := chMap[key]; exists && existing != role {
			return fmt.Sprintf("警告: 采集设备 %s 的通道 %d 同时被映射为 %s 和 %s，数据冲突",
				src.DeviceID, src.Channel, existing, name)
		}
		chMap[key] = role
		return ""
	}

	for _, probe := range config.Probes {
		if !probe.Enabled {
			continue
		}
		for _, ch := range probe.ProbeChannels {
			if !ch.Enabled {
				continue
			}
			key := fmt.Sprintf("%s:%d", ch.DeviceID, ch.Channel)
			if existing, exists := chMap[key]; exists && existing != string(ch.Role) {
				return fmt.Sprintf("警告: 采集设备 %s 的通道 %d 同时被映射为 %s 和 %s，数据冲突",
					ch.DeviceID, ch.Channel, existing, string(ch.Role))
			}
			chMap[key] = string(ch.Role)
		}
		// 探针级 PAtm/TAtm 数据源（device 模式时参与冲突检查）
		pRole := probe.ProbeID + "/PAtm"
		tRole := probe.ProbeID + "/TAtm"
		if msg := registerAtmChannel(pRole, "大气压", probe.PAtmSource); msg != "" {
			return msg
		}
		if msg := registerAtmChannel(tRole, "气流温度", probe.TAtmSource); msg != "" {
			return msg
		}
	}
	return ""
}
