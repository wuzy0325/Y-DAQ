package manager

import (
	"fmt"
	"log/slog"
	"sort"

	"yx-daq/internal/driver"
	"yx-daq/internal/types"
)

// DriverFactory 驱动工厂函数类型
type DriverFactory func(profile types.DeviceProfile) DeviceDriver

// driverFactories 驱动工厂注册表 — 新增设备类型只需在此注册工厂函数
var driverFactories = map[types.DeviceType]DriverFactory{
	types.DeviceTypeEA2508A:   newXYDAQDriver,
	types.DeviceTypeEA2516A:   newXYDAQDriver,
	types.DeviceTypeEA2516T:   newYXDAQTDriver,
	types.DeviceTypeSimulated: newSimulatedDriver,
}

func newXYDAQDriver(profile types.DeviceProfile) DeviceDriver {
	return driver.NewXYDAQDriver(profile.Host, profile.Port, profile.StreamID, profile.Channels, profile.Type)
}

func newYXDAQTDriver(profile types.DeviceProfile) DeviceDriver {
	return driver.NewYXDAQTDriver(profile.Host, profile.Port, profile.Channels)
}

func newSimulatedDriver(profile types.DeviceProfile) DeviceDriver {
	return driver.NewSimulatedDevice(profile.Channels)
}

// DeviceDriver 设备驱动接口
type DeviceDriver interface {
	Connect() error
	Disconnect()
	IsConnected() bool
	IsAcquiring() bool
	StartAcquisition(periodMs int) error
	StopAcquisition() error
	SetDataCallback(cb types.DataCallback)
	UpdateChannels(channels []types.ChannelConfig)
	GetChannels() []types.ChannelConfig
}

// DeviceManager 设备管理器
type DeviceManager struct {
	BaseProfileManager[types.DeviceProfile]
	instances   map[string]DeviceDriver
	dataSink    func(payload types.DataPayload)
	latestData  map[string]types.DataPayload
	runtimeStatus map[string]types.ConnectionStatus
	onStatusChange func(statuses []types.DeviceStatus)
	onTempCalibChange func(deviceID string)
}

// NewDeviceManager 创建设备管理器
func NewDeviceManager() *DeviceManager {
	m := &DeviceManager{
		instances:     make(map[string]DeviceDriver),
		latestData:    make(map[string]types.DataPayload),
		runtimeStatus: make(map[string]types.ConnectionStatus),
	}
	m.initProfiles()
	return m
}

// SetDataSink 设置数据下沉回调
func (m *DeviceManager) SetDataSink(sink func(payload types.DataPayload)) {
	m.dataSink = sink
}

// SetOnStatusChange 设置状态变更回调（由 Core 层桥接到 Wails 事件系统）
func (m *DeviceManager) SetOnStatusChange(cb func(statuses []types.DeviceStatus)) {
	m.Lock()
	defer m.Unlock()
	m.onStatusChange = cb
}

// emitStatusChange 发射状态变更事件
func (m *DeviceManager) emitStatusChange() {
	m.RLock()
	cb := m.onStatusChange
	m.RUnlock()
	if cb != nil {
		cb(m.GetStatusAll())
	}
}

// AddProfile 添加设备配置
func (m *DeviceManager) AddProfile(profile types.DeviceProfile) {
	m.addProfile(profile.ID, profile)
	m.saveProfilesWithLog("device")
}

// UpdateProfile 更新设备配置
func (m *DeviceManager) UpdateProfile(profile types.DeviceProfile) {
	m.Lock()
	if _, ok := m.profiles[profile.ID]; ok {
		m.profiles[profile.ID] = profile
	}
	if drv, ok := m.instances[profile.ID]; ok {
		drv.UpdateChannels(profile.Channels)
	}
	m.Unlock()
	m.saveProfilesWithLog("device")
}

// RemoveProfile 删除设备配置（同时断开连接和停止采集）
func (m *DeviceManager) RemoveProfile(id string) {
	m.Lock()
	if drv, ok := m.instances[id]; ok {
		drv.Disconnect()
		delete(m.instances, id)
	}
	delete(m.profiles, id)
	delete(m.latestData, id)
	delete(m.runtimeStatus, id)
	m.Unlock()
	m.saveProfilesWithLog("device")
	m.emitStatusChange()
}

// GetProfileByID 根据ID获取设备配置
func (m *DeviceManager) GetProfileByID(id string) *types.DeviceProfile {
	p, ok := m.getProfile(id)
	if !ok {
		return nil
	}
	return &p
}

// Connect 连接设备
func (m *DeviceManager) Connect(id string) error {
	m.RLock()
	profile, ok := m.profiles[id]
	m.RUnlock()
	if !ok {
		return fmt.Errorf("device profile not found: %s", id)
	}

	m.Lock()
	m.runtimeStatus[id] = types.StatusConnecting
	m.Unlock()
	m.emitStatusChange()

	factory, ok := driverFactories[profile.Type]
	if !ok {
		m.Lock()
		m.runtimeStatus[id] = types.StatusError
		m.Unlock()
		m.emitStatusChange()
		return fmt.Errorf("unsupported device type: %s", profile.Type)
	}
	drv := factory(profile)

	dataSink := m.dataSink
	drv.SetDataCallback(func(payload types.DataPayload) {
		payload.DeviceID = id
		m.applyZeroOffset(id, &payload)
		m.Lock()
		m.latestData[id] = payload
		m.Unlock()
		if dataSink != nil {
			dataSink(payload)
		}
	})

	if notifier, ok := drv.(ConfigSyncNotifier); ok {
		notifier.OnConfigSynced(func(_ driver.DAQTHardwareConfig) {
			updatedChannels := drv.GetChannels()
			m.Lock()
			if profile, exists := m.profiles[id]; exists {
				profile.Channels = updatedChannels
				m.profiles[id] = profile
			}
			m.Unlock()
			m.saveProfilesWithLog("device")
			m.emitStatusChange()
		})
	}

	if err := drv.Connect(); err != nil {
		m.Lock()
		if m.runtimeStatus[id] == types.StatusDisconnected {
			m.Unlock()
			return fmt.Errorf("connection cancelled")
		}
		m.runtimeStatus[id] = types.StatusError
		m.Unlock()
		m.emitStatusChange()
		return err
	}

	if notifier, ok := drv.(StatusChangeNotifier); ok {
		notifier.SetOnStatusChange(func() {
			m.Lock()
			if drv.IsConnected() {
				m.runtimeStatus[id] = types.StatusConnected
			} else {
				m.runtimeStatus[id] = types.StatusError
			}
			m.Unlock()
			m.emitStatusChange()
		})
	}

	m.Lock()
	if m.runtimeStatus[id] == types.StatusDisconnected {
		m.Unlock()
		drv.Disconnect()
		return fmt.Errorf("connection cancelled")
	}
	m.instances[id] = drv
	m.runtimeStatus[id] = types.StatusConnected
	profile.Channels = drv.GetChannels()
	m.profiles[id] = profile
	m.Unlock()
	m.saveProfilesWithLog("device")
	m.emitStatusChange()
	return nil
}

// Disconnect 断开设备
func (m *DeviceManager) Disconnect(id string) {
	m.Lock()
	if drv, ok := m.instances[id]; ok {
		drv.Disconnect()
		delete(m.instances, id)
	}
	m.runtimeStatus[id] = types.StatusDisconnected
	m.Unlock()
	m.emitStatusChange()
}

// GetStatusAll 获取所有设备状态
func (m *DeviceManager) GetStatusAll() []types.DeviceStatus {
	m.RLock()
	profiles := make(map[string]types.DeviceProfile, len(m.profiles))
	for id, p := range m.profiles {
		profiles[id] = p
	}
	instances := make(map[string]DeviceDriver, len(m.instances))
	for id, drv := range m.instances {
		instances[id] = drv
	}
	runtimeStatus := make(map[string]types.ConnectionStatus, len(m.runtimeStatus))
	for id, s := range m.runtimeStatus {
		runtimeStatus[id] = s
	}
	m.RUnlock()

	statuses := []types.DeviceStatus{}
	for id, profile := range profiles {
		status := types.DeviceStatus{
			ID:   id,
			Name: profile.Name,
			Type: profile.Type,
		}
		if drv, ok := instances[id]; ok {
			if drv.IsConnected() {
				status.Status = types.StatusConnected
			} else {
				status.Status = types.StatusError
			}
			status.Acquiring = drv.IsAcquiring()
		} else if rs, ok := runtimeStatus[id]; ok {
			status.Status = rs
		} else {
			status.Status = types.StatusDisconnected
		}
		statuses = append(statuses, status)
	}
	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].ID < statuses[j].ID
	})
	return statuses
}

// GetLatestData 获取指定设备最新数据
func (m *DeviceManager) GetLatestData(deviceID string) (types.DataPayload, bool) {
	m.RLock()
	defer m.RUnlock()
	data, ok := m.latestData[deviceID]
	return data, ok
}

// GetChannelValue 获取指定通道值
func (m *DeviceManager) GetChannelValue(deviceID string, channelIndex int) (float64, bool) {
	m.RLock()
	defer m.RUnlock()
	data, ok := m.latestData[deviceID]
	if !ok {
		return 0, false
	}
	for i, idx := range data.ChannelIndices {
		if idx == channelIndex && i < len(data.Channels) {
			return data.Channels[i], true
		}
	}
	return 0, false
}

// GetAllLatestData 获取所有设备最新数据快照
func (m *DeviceManager) GetAllLatestData() []types.DataPayload {
	m.RLock()
	defer m.RUnlock()
	snapshots := make([]types.DataPayload, 0, len(m.latestData))
	for _, data := range m.latestData {
		snapshots = append(snapshots, data)
	}
	return snapshots
}

// UnitSetter 单位设置接口（仅XY-DAQ驱动实现）
type UnitSetter interface {
	SetUnit(unit string) error
}

// ThermocoupleTypeSetter 热电偶类型设置接口（仅 EA2516T 驱动实现）
type ThermocoupleTypeSetter interface {
	SetThermocoupleType(tcTypes string) error
	SetSingleThermocoupleType(channelIndex int, tcType string) error
}

// ValveController 校准阀控制接口（仅 EA2516A 压力驱动 + 模拟设备实现）
type ValveController interface {
	ReadValveState() (types.ValveState, error)
	SetValveState(state types.ValveState) error
}

// TempChannelConfigurator EA2508A 温度通道配置接口（@16 来源 + @17 热电偶类型，仅 EA2508A 驱动实现）
type TempChannelConfigurator interface {
	SetTempSource(source string) error
	SetTempThermocoupleType(tcType string) error
}

// ConfigSyncNotifier 配置同步通知能力接口（仅 EA2516T 驱动实现）
type ConfigSyncNotifier interface {
	OnConfigSynced(cb func(driver.DAQTHardwareConfig))
}

// StatusChangeNotifier 状态变更通知能力接口（仅 TCP 驱动实现）
type StatusChangeNotifier interface {
	SetOnStatusChange(func())
}

// Init 初始化（从配置文件加载设备，若无则创建默认模拟设备）
func (m *DeviceManager) Init() {
	loaded := false
	if m.configStore != nil {
		profiles := m.configStore.Get()
		if len(profiles) > 0 {
			migrated := 0
			for i := range profiles {
				p := &profiles[i]
				if newType, changed := types.MigrateDeviceType(p.Type); changed {
					slog.Info("migrate legacy device type", "id", p.ID, "old", p.Type, "new", newType)
					p.Type = newType
					migrated++
				}
				m.Lock()
				m.profiles[p.ID] = *p
				m.Unlock()
			}
			loaded = true
			slog.Info("loaded device profiles from config", "count", len(profiles), "migrated", migrated)
			if migrated > 0 {
				m.saveProfilesWithLog("device")
			}
		}
	}

	if !loaded {
		pressureCount := types.DeviceTypeEA2516A.PressureChannelCount()
		totalChannels := types.DeviceTypeEA2516A.TotalChannelCount()
		defaultChannels := make([]types.ChannelConfig, totalChannels)
		for i := 0; i < totalChannels; i++ {
			name := fmt.Sprintf("CH%d", i+1)
			if i == pressureCount {
				name = "大气压"
			} else if i == pressureCount+1 {
				name = "大气温度"
			}
			unit := "kPa"
			if i == pressureCount {
				unit = "Pa"
			} else if i == pressureCount+1 {
				unit = "°C"
			}
			defaultChannels[i] = types.ChannelConfig{
				Index:     i,
				Name:      name,
				Enabled:   true,
				Unit:      unit,
				Precision: 3,
			}
		}

		simProfile := types.DeviceProfile{
			ID:          "sim-1",
			Name:        "模拟设备",
			Type:        types.DeviceTypeSimulated,
			Host:        "127.0.0.1",
			Port:        9000,
			StreamID:    1,
			AutoConnect: true,
			Channels:    defaultChannels,
		}

		m.Lock()
		m.profiles[simProfile.ID] = simProfile
		m.Unlock()
		m.saveProfilesWithLog("device")
	}

	m.AutoConnect()
}

// AutoConnect 自动连接所有启用了自动连接的设备
func (m *DeviceManager) AutoConnect() {
	m.RLock()
	type kv struct {
		id   string
		auto bool
	}
	pairs := make([]kv, 0, len(m.profiles))
	for id, p := range m.profiles {
		pairs = append(pairs, kv{id, p.AutoConnect})
	}
	m.RUnlock()

	for _, pair := range pairs {
		if !pair.auto {
			continue
		}
		if !m.IsConnected(pair.id) {
			if err := m.Connect(pair.id); err != nil {
				slog.Error("auto connect device failed", "id", pair.id, "err", err)
			}
		}
	}
}
