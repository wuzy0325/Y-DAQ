package manager

import (
	"fmt"
	"log/slog"

	"yx-daq/internal/types"
)

// StartAcquisition 启动采集
func (m *DeviceManager) StartAcquisition(id string, periodMs int) error {
	m.RLock()
	drv, ok := m.instances[id]
	m.RUnlock()
	if !ok {
		return fmt.Errorf("device not connected: %s", id)
	}
	return drv.StartAcquisition(periodMs)
}

// StopAcquisition 停止采集
func (m *DeviceManager) StopAcquisition(id string) error {
	m.RLock()
	drv, ok := m.instances[id]
	m.RUnlock()
	if !ok {
		return fmt.Errorf("device not connected: %s", id)
	}
	return drv.StopAcquisition()
}

// StartAcquisitionAll 批量启动采集（使用各设备配置的periodMs）
func (m *DeviceManager) StartAcquisitionAll() int {
	m.RLock()
	type instanceInfo struct {
		id       string
		drv      DeviceDriver
		periodMs int
	}
	items := make([]instanceInfo, 0, len(m.instances))
	for id, drv := range m.instances {
		periodMs := 50
		if p, ok := m.profiles[id]; ok && p.PeriodMs > 0 {
			periodMs = p.PeriodMs
		}
		items = append(items, instanceInfo{id, drv, periodMs})
	}
	m.RUnlock()

	count := 0
	for _, item := range items {
		if err := item.drv.StartAcquisition(item.periodMs); err != nil {
			slog.Error("start acquisition failed", "id", item.id, "err", err)
		} else {
			count++
		}
	}
	return count
}

// StopAcquisitionAll 批量停止采集
func (m *DeviceManager) StopAcquisitionAll() {
	m.RLock()
	instances := make(map[string]DeviceDriver, len(m.instances))
	for id, drv := range m.instances {
		instances[id] = drv
	}
	m.RUnlock()

	for id, drv := range instances {
		if err := drv.StopAcquisition(); err != nil {
			slog.Error("stop acquisition failed", "id", id, "err", err)
		}
	}
}

// SetUnit 设置设备压力单位（写入硬件）
func (m *DeviceManager) SetUnit(id string, unit string) error {
	m.RLock()
	drv, ok := m.instances[id]
	m.RUnlock()
	if !ok {
		return fmt.Errorf("device not connected: %s", id)
	}

	setter, ok := drv.(UnitSetter)
	if !ok {
		return fmt.Errorf("device does not support SetUnit: %s", id)
	}

	if err := setter.SetUnit(unit); err != nil {
		return err
	}

	m.Lock()
	if profile, exists := m.profiles[id]; exists {
		for i := range profile.Channels {
			if profile.Channels[i].Index < profile.Type.PressureChannelCount() {
				profile.Channels[i].Unit = unit
			}
		}
		m.profiles[id] = profile
	}
	m.Unlock()
	m.saveProfilesWithLog("device")

	return nil
}

// SetThermocoupleType 设置设备热电偶类型（全通道批量设置，写入硬件）
func (m *DeviceManager) SetThermocoupleType(id string, tcTypes string) error {
	m.RLock()
	drv, ok := m.instances[id]
	m.RUnlock()
	if !ok {
		return fmt.Errorf("device not connected: %s", id)
	}

	setter, ok := drv.(ThermocoupleTypeSetter)
	if !ok {
		return fmt.Errorf("device does not support SetThermocoupleType: %s", id)
	}

	if err := setter.SetThermocoupleType(tcTypes); err != nil {
		return err
	}

	m.Lock()
	if profile, exists := m.profiles[id]; exists {
		if len(tcTypes) == 16 {
			runes := []rune(tcTypes)
			for i := range profile.Channels {
				if i < 16 {
					profile.Channels[i].ThermocoupleType = string(runes[i])
				}
			}
		}
		m.profiles[id] = profile
	}
	m.Unlock()
	m.saveProfilesWithLog("device")

	return nil
}

// SetSingleThermocoupleType 设置单个通道的热电偶类型（写入硬件）
func (m *DeviceManager) SetSingleThermocoupleType(id string, channelIndex int, tcType string) error {
	m.RLock()
	drv, ok := m.instances[id]
	m.RUnlock()
	if !ok {
		return fmt.Errorf("device not connected: %s", id)
	}

	setter, ok := drv.(ThermocoupleTypeSetter)
	if !ok {
		return fmt.Errorf("device does not support SetThermocoupleType: %s", id)
	}

	if err := setter.SetSingleThermocoupleType(channelIndex, tcType); err != nil {
		return err
	}

	m.Lock()
	if profile, exists := m.profiles[id]; exists {
		for i := range profile.Channels {
			if profile.Channels[i].Index == channelIndex {
				profile.Channels[i].ThermocoupleType = tcType
				break
			}
		}
		m.profiles[id] = profile
	}
	m.Unlock()
	m.saveProfilesWithLog("device")

	return nil
}

// ReadValveState 读取设备校准阀位（每次从设备读取，不持久化）
func (m *DeviceManager) ReadValveState(id string) (types.ValveState, error) {
	m.RLock()
	drv, ok := m.instances[id]
	m.RUnlock()
	if !ok {
		return types.ValveStateUnknown, fmt.Errorf("device not connected: %s", id)
	}

	ctrl, ok := drv.(ValveController)
	if !ok {
		return types.ValveStateUnknown, fmt.Errorf("device does not support valve control: %s", id)
	}
	return ctrl.ReadValveState()
}

// SetValveState 切换设备校准阀位。
func (m *DeviceManager) SetValveState(id string, state types.ValveState) error {
	m.RLock()
	drv, ok := m.instances[id]
	m.RUnlock()
	if !ok {
		return fmt.Errorf("device not connected: %s", id)
	}

	if drv.IsAcquiring() {
		return fmt.Errorf("采集进行中，不允许切换阀位")
	}

	ctrl, ok := drv.(ValveController)
	if !ok {
		return fmt.Errorf("device does not support valve control: %s", id)
	}
	return ctrl.SetValveState(state)
}

// IsAcquiring 检查指定设备是否正在采集
func (m *DeviceManager) IsAcquiring(id string) bool {
	m.RLock()
	drv, ok := m.instances[id]
	m.RUnlock()
	if !ok {
		return false
	}
	return drv.IsAcquiring()
}

// IsConnected 检查设备是否连接
func (m *DeviceManager) IsConnected(id string) bool {
	m.RLock()
	drv, ok := m.instances[id]
	m.RUnlock()
	if !ok {
		return false
	}
	return drv.IsConnected()
}
