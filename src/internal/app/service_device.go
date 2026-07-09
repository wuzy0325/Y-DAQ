package app

import (
	"log/slog"

	"yx-daq/internal/types"
)

// DeviceService 设备管理服务
type DeviceService struct {
	Core *Core
}

// GetDeviceProfiles 获取所有设备配置
func (s *DeviceService) GetDeviceProfiles() []types.DeviceProfile {
	return s.Core.DeviceManager.GetProfiles()
}

// AddDeviceProfile 添加设备配置
func (s *DeviceService) AddDeviceProfile(profile types.DeviceProfile) {
	s.Core.DeviceManager.AddProfile(profile)
}

// UpdateDeviceProfile 更新设备配置
func (s *DeviceService) UpdateDeviceProfile(profile types.DeviceProfile) {
	s.Core.DeviceManager.UpdateProfile(profile)
}

// RemoveDeviceProfile 删除设备配置
func (s *DeviceService) RemoveDeviceProfile(id string) {
	s.Core.DeviceManager.RemoveProfile(id)
}

// ConnectDevice 连接设备
func (s *DeviceService) ConnectDevice(id string) error {
	return s.Core.DeviceManager.Connect(id)
}

// DisconnectDevice 断开设备
func (s *DeviceService) DisconnectDevice(id string) {
	s.Core.DeviceManager.Disconnect(id)
}

// StartAcquisition 启动采集
func (s *DeviceService) StartAcquisition(id string) error {
	periodMs := 50
	if profile := s.Core.DeviceManager.GetProfileByID(id); profile != nil && profile.PeriodMs > 0 {
		periodMs = profile.PeriodMs
	}
	return s.Core.DeviceManager.StartAcquisition(id, periodMs)
}

// StopAcquisition 停止采集
func (s *DeviceService) StopAcquisition(id string) error {
	err := s.Core.DeviceManager.StopAcquisition(id)
	if err == nil {
		s.Core.AcquisitionHub.ClearDevice(id)
	}
	return err
}

// StartAcquisitionAll 批量启动采集
func (s *DeviceService) StartAcquisitionAll() int {
	return s.Core.DeviceManager.StartAcquisitionAll()
}

// StopAcquisitionAll 批量停止采集
func (s *DeviceService) StopAcquisitionAll() {
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
	return s.Core.DeviceManager.GetStatusAll()
}

// SetUnit 设置设备压力单位
func (s *DeviceService) SetUnit(id string, unit string) error {
	return s.Core.DeviceManager.SetUnit(id, unit)
}

// SetThermocoupleType 设置设备热电偶类型（全通道批量设置）
func (s *DeviceService) SetThermocoupleType(id string, tcTypes string) error {
	return s.Core.DeviceManager.SetThermocoupleType(id, tcTypes)
}

// SetSingleThermocoupleType 设置单个通道的热电偶类型
func (s *DeviceService) SetSingleThermocoupleType(id string, channelIndex int, tcType string) error {
	return s.Core.DeviceManager.SetSingleThermocoupleType(id, channelIndex, tcType)
}

// ReadValveState 读取设备校准阀位（每次从设备读取，不持久化）
func (s *DeviceService) ReadValveState(id string) (types.ValveState, error) {
	return s.Core.DeviceManager.ReadValveState(id)
}

// SetValveState 切换设备校准阀位（采集进行中会被拒绝）
func (s *DeviceService) SetValveState(id string, state types.ValveState) error {
	return s.Core.DeviceManager.SetValveState(id, state)
}

// ZeroCalibrate 对指定设备的所有启用压力通道执行零位校准
func (s *DeviceService) ZeroCalibrate(id string) error {
	return s.Core.DeviceManager.ZeroCalibrate(id)
}

// ZeroCalibrateAll 对所有已连接且正在采集的压力采集设备批量执行零位校准
func (s *DeviceService) ZeroCalibrateAll() []types.ZeroCalibrateResult {
	return s.Core.DeviceManager.ZeroCalibrateAll()
}

// ZeroCalibrateChannel 对指定设备的单个通道执行零位校准
func (s *DeviceService) ZeroCalibrateChannel(id string, channelIndex int) error {
	return s.Core.DeviceManager.ZeroCalibrateChannel(id, channelIndex)
}

// ClearZeroOffset 清除指定通道的零位偏移
func (s *DeviceService) ClearZeroOffset(id string, channelIndex int) error {
	return s.Core.DeviceManager.ClearZeroOffset(id, channelIndex)
}

// ClearAllZeroOffsets 清除指定设备所有通道的零位偏移
func (s *DeviceService) ClearAllZeroOffsets(id string) error {
	return s.Core.DeviceManager.ClearAllZeroOffsets(id)
}

// ScanDevices 扫描设备
func (s *DeviceService) ScanDevices() []types.DiscoveredDevice {
	devices, err := s.Core.DaqScanner.Scan(3000)
	if err != nil {
		slog.Error("scan devices failed", "err", err)
		return []types.DiscoveredDevice{}
	}
	return devices
}

// GetLatestData 获取最新数据快照
func (s *DeviceService) GetLatestData() []types.DataPayload {
	return s.Core.AcquisitionHub.GetSnapshot()
}
