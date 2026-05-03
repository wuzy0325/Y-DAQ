package app

import (
	"fmt"
	"log/slog"

	"yx-daq/internal/types"
)

// ==================== 设备管理 API ====================

// GetDeviceProfiles 获取所有设备配置
func (a *App) GetDeviceProfiles() []types.DeviceProfile {
	if a.deviceManager == nil {
		return nil
	}
	return a.deviceManager.GetProfiles()
}

// AddDeviceProfile 添加设备配置
func (a *App) AddDeviceProfile(profile types.DeviceProfile) {
	if a.deviceManager == nil {
		return
	}
	a.deviceManager.AddProfile(profile)
}

// UpdateDeviceProfile 更新设备配置
func (a *App) UpdateDeviceProfile(profile types.DeviceProfile) {
	if a.deviceManager == nil {
		return
	}
	a.deviceManager.UpdateProfile(profile)
}

// RemoveDeviceProfile 删除设备配置
func (a *App) RemoveDeviceProfile(id string) {
	if a.deviceManager == nil {
		return
	}
	a.deviceManager.RemoveProfile(id)
}

// ConnectDevice 连接设备
func (a *App) ConnectDevice(id string) error {
	if a.deviceManager == nil {
		return fmt.Errorf("device manager not initialized")
	}
	return a.deviceManager.Connect(id)
}

// DisconnectDevice 断开设备
func (a *App) DisconnectDevice(id string) {
	if a.deviceManager == nil {
		return
	}
	a.deviceManager.Disconnect(id)
}

// StartAcquisition 启动采集
func (a *App) StartAcquisition(id string) error {
	if a.deviceManager == nil {
		return fmt.Errorf("device manager not initialized")
	}
	periodMs := 50
	profile := a.deviceManager.GetProfileByID(id)
	if profile != nil && profile.PeriodMs > 0 {
		periodMs = profile.PeriodMs
	}
	return a.deviceManager.StartAcquisition(id, periodMs)
}

// StopAcquisition 停止采集
func (a *App) StopAcquisition(id string) error {
	if a.deviceManager == nil {
		return fmt.Errorf("device manager not initialized")
	}
	err := a.deviceManager.StopAcquisition(id)
	if err == nil {
		a.acquisitionHub.ClearDevice(id)
	}
	return err
}

// StartAcquisitionAll 批量启动采集
func (a *App) StartAcquisitionAll() int {
	if a.deviceManager == nil {
		return 0
	}
	return a.deviceManager.StartAcquisitionAll(50)
}

// StopAcquisitionAll 批量停止采集
func (a *App) StopAcquisitionAll() {
	if a.deviceManager == nil {
		return
	}
	statuses := a.deviceManager.GetStatusAll()
	a.deviceManager.StopAcquisitionAll()
	for _, s := range statuses {
		if s.Acquiring {
			a.acquisitionHub.ClearDevice(s.ID)
		}
	}
}

// GetDeviceStatusAll 获取所有设备状态
func (a *App) GetDeviceStatusAll() []types.DeviceStatus {
	if a.deviceManager == nil {
		return nil
	}
	return a.deviceManager.GetStatusAll()
}

// SetUnit 设置设备压力单位（写入硬件）
func (a *App) SetUnit(id string, unit string) error {
	if a.deviceManager == nil {
		return fmt.Errorf("device manager not initialized")
	}
	return a.deviceManager.SetUnit(id, unit)
}

// ScanDevices 扫描设备
func (a *App) ScanDevices() []types.DiscoveredDevice {
	if a.daqScanner == nil {
		return nil
	}
	devices, err := a.daqScanner.Scan(3000)
	if err != nil {
		slog.Error("scan devices failed", "err", err)
		return []types.DiscoveredDevice{}
	}
	return devices
}

// GetLatestData 获取最新数据快照
func (a *App) GetLatestData() []types.DataPayload {
	if a.acquisitionHub == nil {
		return nil
	}
	return a.acquisitionHub.GetSnapshot()
}
