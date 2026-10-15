package manager

import (
	"fmt"

	"yx-daq/internal/types"
)

// SetADAveraging 设置压力扫描阀 AD 平均（启用开关 + 平均次数）。
// 关闭时下发固定最低值 4；开启时下发 8/16/32/64（非法值回退 8）。
// 持久化到设备配置，已连接时立即下发 w05 命令（采集进行中拒绝修改）。
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
