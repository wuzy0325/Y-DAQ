package manager

import (
	"fmt"
	"log/slog"

	"yx-daq/internal/types"
)

// syncADAveragingFromDevice 连接后读取设备实际 AD 平均次数，并以设备为准同步配置：
// 读到 4 → 开关记为关闭；读到其他值 → 开关记为开启且保留实际次数（界面显示实际值）。
// 读取失败仅告警并保留原配置（部分固件不支持 q05，不能因此阻断连接）。
// 同步成功后广播 profiles 变更，前端据此刷新 AD 列显示。
func (m *DeviceManager) syncADAveragingFromDevice(id string, drv DeviceDriver) {
	reader, ok := drv.(ADAveragingReader)
	if !ok {
		return
	}
	actual, err := reader.ReadADAveraging()
	if err != nil {
		slog.Warn("sync AD averaging from device failed, keep profile",
			"device", id, "err", err)
		return
	}
	enabled, count := types.ADAveragingFromDevice(actual)

	m.Lock()
	profile, exists := m.profiles[id]
	if exists {
		profile.ADAveragingEnabled = enabled
		profile.ADAveragingCount = count
		m.profiles[id] = profile
	}
	m.Unlock()
	if !exists {
		return
	}
	m.saveProfilesWithLog("device")
	m.emitProfileChange(id)
}

// SetADAveraging 设置压力扫描阀 AD 平均（启用开关 + 平均次数）。
// 关闭时下发固定最低值 4；开启时下发 8/16/32（非法值回退 8）。
// 持久化到设备配置，已连接时立即下发 w10 命令（采集进行中拒绝修改）。
// 连接时则反向：以设备实际值同步配置（见 syncADAveragingFromDevice）。
func (m *DeviceManager) SetADAveraging(id string, enabled bool, count int) error {
	m.RLock()
	profile, exists := m.profiles[id]
	drv, connected := m.instances[id]
	m.RUnlock()
	if !exists {
		return fmt.Errorf("device profile not found: %s", id)
	}
	if !profile.Type.SupportsADAveraging() {
		return fmt.Errorf("设备不支持AD平均配置: %s（仅 EA2508A/EA2516A）", id)
	}
	// 开启时必须为合法次数，尽早拒绝，避免把非法值下发硬件或持久化
	if enabled && !types.IsAllowedADAveragingCount(count) {
		return fmt.Errorf("AD平均次数非法: %d（开启时可选 %v）", count, types.ADAveragingAllowedCounts)
	}
	if connected {
		if err := m.ensureConfigMutable(id, drv); err != nil {
			return err
		}
		if _, ok := drv.(ADAveragingConfigurator); !ok {
			return fmt.Errorf("设备不支持AD平均配置: %s", id)
		}
	}

	// 归一化后的实际次数：硬件与持久化必须使用同一值，避免 profile/UI/设备不一致
	effective := types.EffectiveADAveragingCount(enabled, count)

	// 先下发硬件，成功后再持久化，避免硬件失败却留下不一致的配置
	if connected {
		if cfg, ok := drv.(ADAveragingConfigurator); ok {
			if err := cfg.SetADAveraging(effective); err != nil {
				return err
			}
		}
	}

	// 在写锁内重新读取最新 profile 后修改，避免并发更新丢失
	m.Lock()
	profile, exists = m.profiles[id]
	if !exists {
		m.Unlock()
		return fmt.Errorf("device profile not found: %s", id)
	}
	profile.ADAveragingEnabled = enabled
	profile.ADAveragingCount = effective
	m.profiles[id] = profile
	m.Unlock()

	m.saveProfilesWithLog("device")
	return nil
}
