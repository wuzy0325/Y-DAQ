package app

import (
	"fmt"
	"log/slog"

	"yx-daq/internal/types"
)

// DeviceService 设备管理服务
type DeviceService struct {
	Core *Core
}

// GetDeviceProfiles 获取所有设备配置
func (s *DeviceService) GetDeviceProfiles() []types.DeviceProfile {
	if s.Core.DeviceManager == nil {
		return nil
	}
	return s.Core.DeviceManager.GetProfiles()
}

// AddDeviceProfile 添加设备配置
func (s *DeviceService) AddDeviceProfile(profile types.DeviceProfile) {
	if s.Core.DeviceManager == nil {
		return
	}
	s.Core.DeviceManager.AddProfile(profile)
}

// UpdateDeviceProfile 更新设备配置
func (s *DeviceService) UpdateDeviceProfile(profile types.DeviceProfile) {
	if s.Core.DeviceManager == nil {
		return
	}
	s.Core.DeviceManager.UpdateProfile(profile)
}

// RemoveDeviceProfile 删除设备配置
func (s *DeviceService) RemoveDeviceProfile(id string) {
	if s.Core.DeviceManager == nil {
		return
	}
	s.Core.DeviceManager.RemoveProfile(id)
}

// ConnectDevice 连接设备
func (s *DeviceService) ConnectDevice(id string) error {
	if s.Core.DeviceManager == nil {
		return fmt.Errorf("device manager not initialized")
	}
	return s.Core.DeviceManager.Connect(id)
}

// DisconnectDevice 断开设备
func (s *DeviceService) DisconnectDevice(id string) {
	if s.Core.DeviceManager == nil {
		return
	}
	s.Core.DeviceManager.Disconnect(id)
}

// StartAcquisition 启动采集
func (s *DeviceService) StartAcquisition(id string) error {
	if s.Core.DeviceManager == nil {
		return fmt.Errorf("device manager not initialized")
	}
	periodMs := 50
	if profile := s.Core.DeviceManager.GetProfileByID(id); profile != nil && profile.PeriodMs > 0 {
		periodMs = profile.PeriodMs
	}
	return s.Core.DeviceManager.StartAcquisition(id, periodMs)
}

// StopAcquisition 停止采集
func (s *DeviceService) StopAcquisition(id string) error {
	if s.Core.DeviceManager == nil {
		return fmt.Errorf("device manager not initialized")
	}
	err := s.Core.DeviceManager.StopAcquisition(id)
	if err == nil {
		s.Core.AcquisitionHub.ClearDevice(id)
	}
	return err
}

// StartAcquisitionAll 批量启动采集
func (s *DeviceService) StartAcquisitionAll() int {
	if s.Core.DeviceManager == nil {
		return 0
	}
	return s.Core.DeviceManager.StartAcquisitionAll()
}

// StopAcquisitionAll 批量停止采集
func (s *DeviceService) StopAcquisitionAll() {
	if s.Core.DeviceManager == nil {
		return
	}
	statuses := s.Core.DeviceManager.GetStatusAll()
	s.Core.DeviceManager.StopAcquisitionAll()
	for _, st := range statuses {
		if st.Acquiring {
			s.Core.AcquisitionHub.ClearDevice(st.ID)
		}
	}
}

// GetDeviceStatusAll 获取所有设备状态
func (s *DeviceService) GetDeviceStatusAll() []types.DeviceStatus {
	if s.Core.DeviceManager == nil {
		return nil
	}
	return s.Core.DeviceManager.GetStatusAll()
}

// SetUnit 设置设备压力单位
func (s *DeviceService) SetUnit(id string, unit string) error {
	if s.Core.DeviceManager == nil {
		return fmt.Errorf("device manager not initialized")
	}
	return s.Core.DeviceManager.SetUnit(id, unit)
}

// SetThermocoupleType 设置设备热电偶类型（全通道批量设置）
func (s *DeviceService) SetThermocoupleType(id string, tcTypes string) error {
	if s.Core.DeviceManager == nil {
		return fmt.Errorf("device manager not initialized")
	}
	return s.Core.DeviceManager.SetThermocoupleType(id, tcTypes)
}

// SetSingleThermocoupleType 设置单个通道的热电偶类型
func (s *DeviceService) SetSingleThermocoupleType(id string, channelIndex int, tcType string) error {
	if s.Core.DeviceManager == nil {
		return fmt.Errorf("device manager not initialized")
	}
	return s.Core.DeviceManager.SetSingleThermocoupleType(id, channelIndex, tcType)
}

// ScanDevices 扫描设备
func (s *DeviceService) ScanDevices() []types.DiscoveredDevice {
	if s.Core.DaqScanner == nil {
		return nil
	}
	devices, err := s.Core.DaqScanner.Scan(3000)
	if err != nil {
		slog.Error("scan devices failed", "err", err)
		return []types.DiscoveredDevice{}
	}
	return devices
}

// GetLatestData 获取最新数据快照
func (s *DeviceService) GetLatestData() []types.DataPayload {
	if s.Core.AcquisitionHub == nil {
		return nil
	}
	return s.Core.AcquisitionHub.GetSnapshot()
}
