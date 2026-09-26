package manager

import (
	"fmt"

	"yx-daq/internal/types"
)

// syncAtmChannelEnabled 令大气压/大气温度通道的启用状态与 AtmEnabled 主开关一致：
// 关闭时强制禁用（数据流不含），启用时恢复启用。返回是否发生变更。
// 主开关切换会覆盖用户在通道表对这两个特殊通道的单独开关。
func syncAtmChannelEnabled(profile *types.DeviceProfile) bool {
	if !profile.Type.IsAtmCapable() {
		return false
	}
	pressureCount := profile.Type.PressureChannelCount()
	channels := make([]types.ChannelConfig, len(profile.Channels))
	copy(channels, profile.Channels)

	changed := false
	for i := range channels {
		if channels[i].Index >= pressureCount && channels[i].Enabled != profile.AtmEnabled {
			channels[i].Enabled = profile.AtmEnabled
			changed = true
		}
	}
	if changed {
		profile.Channels = channels
	}
	return changed
}

// SetAtmEnabled 设置压力设备大气压/温度采集使能（EA2508A/EA2516A）。
// 持久化并同步特殊通道启用标志；已连接时热更新驱动，c 05 命令在下次启动采集时下发。
// 采集进行中拒绝修改（与 ensureConfigMutable 保护一致）。
func (m *DeviceManager) SetAtmEnabled(id string, enabled bool) error {
	m.RLock()
	profile, exists := m.profiles[id]
	drv, connected := m.instances[id]
	m.RUnlock()
	if !exists {
		return fmt.Errorf("device profile not found: %s", id)
	}
	if !profile.Type.IsAtmCapable() {
		return fmt.Errorf("设备不支持大气压/温度采集开关: %s（仅 EA2508A/EA2516A）", id)
	}
	if connected {
		if err := m.ensureConfigMutable(id, drv); err != nil {
			return err
		}
		if _, ok := drv.(AtmChannelConfigurator); !ok {
			return fmt.Errorf("设备不支持大气压/温度采集配置: %s", id)
		}
	}

	// 在写锁内重新读取最新 profile 后修改：若用校验前的快照写回，
	// 窗口期内并发 Connect/SetUnit 的更新会被静默覆盖（lost update）
	m.Lock()
	profile, exists = m.profiles[id]
	if !exists {
		m.Unlock()
		return fmt.Errorf("device profile not found: %s", id)
	}
	profile.AtmEnabled = enabled
	syncAtmChannelEnabled(&profile)
	m.profiles[id] = profile
	m.Unlock()

	if connected {
		drv.UpdateChannels(profile.Channels)
		if cfg, ok := drv.(AtmChannelConfigurator); ok {
			cfg.SetAtmEnabled(enabled)
		}
	}
	m.saveProfilesWithLog("device")
	return nil
}
