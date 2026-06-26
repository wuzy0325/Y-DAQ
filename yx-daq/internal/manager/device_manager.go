package manager

import (
	"fmt"
	"log/slog"
	"sync"

	"yx-daq/internal/driver"
	"yx-daq/internal/storage"
	"yx-daq/internal/types"
)

// DriverFactory 驱动工厂函数类型
type DriverFactory func(profile types.DeviceProfile) DeviceDriver

// driverFactories 驱动工厂注册表 — 新增设备类型只需在此注册工厂函数
var driverFactories = map[types.DeviceType]DriverFactory{
	types.DeviceTypeXYDAQ8:    newXYDAQDriver,
	types.DeviceTypeXYDAQ16:   newXYDAQDriver,
	types.DeviceTypeYXDAQT:    newYXDAQTDriver,
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
	mu          sync.RWMutex
	profiles    map[string]types.DeviceProfile
	instances   map[string]DeviceDriver
	dataSink    func(payload types.DataPayload)
	latestData  map[string]types.DataPayload
	configStore *storage.ConfigStore[[]types.DeviceProfile]
	// 运行时连接状态（独立于驱动实例，用于在驱动创建前/断连后仍可查询）
	runtimeStatus map[string]types.ConnectionStatus
	// 状态变更回调（由 Core 层桥接到 Wails 事件系统）
	onStatusChange func(statuses []types.DeviceStatus)
}

// NewDeviceManager 创建设备管理器
func NewDeviceManager() *DeviceManager {
	return &DeviceManager{
		profiles:      make(map[string]types.DeviceProfile),
		instances:     make(map[string]DeviceDriver),
		latestData:    make(map[string]types.DataPayload),
		runtimeStatus: make(map[string]types.ConnectionStatus),
	}
}

// SetConfigStore 设置配置存储（用于持久化）
func (m *DeviceManager) SetConfigStore(store *storage.ConfigStore[[]types.DeviceProfile]) {
	m.configStore = store
}

// saveProfiles 持久化设备配置到磁盘
func (m *DeviceManager) saveProfiles() {
	if m.configStore == nil {
		return
	}
	profiles := m.GetProfiles()
	if err := m.configStore.Set(profiles); err != nil {
		slog.Error("save device profiles failed", "err", err)
	}
}

// SetDataSink 设置数据下沉回调
func (m *DeviceManager) SetDataSink(sink func(payload types.DataPayload)) {
	m.dataSink = sink
}

// SetOnStatusChange 设置状态变更回调（由 Core 层桥接到 Wails 事件系统）
// 必须在应用启动时调用，之后不再变更
func (m *DeviceManager) SetOnStatusChange(cb func(statuses []types.DeviceStatus)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onStatusChange = cb
}

// emitStatusChange 发射状态变更事件
// 注意：必须在未持有 m.mu 时调用，否则 GetStatusAll 中的 RLock 会死锁
func (m *DeviceManager) emitStatusChange() {
	m.mu.RLock()
	cb := m.onStatusChange
	m.mu.RUnlock()
	if cb != nil {
		cb(m.GetStatusAll())
	}
}

// AddProfile 添加设备配置
func (m *DeviceManager) AddProfile(profile types.DeviceProfile) {
	m.mu.Lock()
	m.profiles[profile.ID] = profile
	m.mu.Unlock()
	m.saveProfiles()
}

// UpdateProfile 更新设备配置
func (m *DeviceManager) UpdateProfile(profile types.DeviceProfile) {
	m.mu.Lock()
	if _, ok := m.profiles[profile.ID]; ok {
		m.profiles[profile.ID] = profile
	}
	// 同步通道配置到已连接驱动
	if drv, ok := m.instances[profile.ID]; ok {
		drv.UpdateChannels(profile.Channels)
	}
	m.mu.Unlock()
	m.saveProfiles()
}

// RemoveProfile 删除设备配置（同时断开连接和停止采集）
func (m *DeviceManager) RemoveProfile(id string) {
	m.mu.Lock()
	// 先断开连接（会停止采集）
	if drv, ok := m.instances[id]; ok {
		drv.Disconnect()
		delete(m.instances, id)
	}
	delete(m.profiles, id)
	delete(m.latestData, id)
	delete(m.runtimeStatus, id)
	m.mu.Unlock()
	m.saveProfiles()
	m.emitStatusChange()
}

// GetProfiles 获取所有设备配置
func (m *DeviceManager) GetProfiles() []types.DeviceProfile {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]types.DeviceProfile, 0, len(m.profiles))
	for _, p := range m.profiles {
		result = append(result, p)
	}
	return result
}

// GetProfileByID 根据ID获取设备配置
func (m *DeviceManager) GetProfileByID(id string) *types.DeviceProfile {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if p, ok := m.profiles[id]; ok {
		return &p
	}
	return nil
}

// Connect 连接设备
func (m *DeviceManager) Connect(id string) error {
	m.mu.RLock()
	profile, ok := m.profiles[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("device profile not found: %s", id)
	}

	// 设置 Connecting 状态
	m.mu.Lock()
	m.runtimeStatus[id] = types.StatusConnecting
	m.mu.Unlock()
	m.emitStatusChange()

	factory, ok := driverFactories[profile.Type]
	if !ok {
		m.mu.Lock()
		m.runtimeStatus[id] = types.StatusError
		m.mu.Unlock()
		m.emitStatusChange()
		return fmt.Errorf("unsupported device type: %s", profile.Type)
	}
	drv := factory(profile)

	dataSink := m.dataSink
	drv.SetDataCallback(func(payload types.DataPayload) {
		payload.DeviceID = id
		m.mu.Lock()
		m.latestData[id] = payload
		m.mu.Unlock()
		if dataSink != nil {
			dataSink(payload)
		}
	})

	// DAQ-T 设备：注册配置同步回调，连接后自动读取热电偶类型并更新 profile
	if tcDrv, ok := drv.(*driver.YXDAQTDriver); ok {
		tcDrv.OnConfigSynced(func(_ driver.DAQTHardwareConfig) {
			updatedChannels := tcDrv.GetChannels()
			m.mu.Lock()
			if profile, exists := m.profiles[id]; exists {
				profile.Channels = updatedChannels
				m.profiles[id] = profile
			}
			m.mu.Unlock()
			m.saveProfiles()
			m.emitStatusChange()
		})
	}

	if err := drv.Connect(); err != nil {
		m.mu.Lock()
		// 检查是否在连接过程中被 Disconnect 取消
		if m.runtimeStatus[id] == types.StatusDisconnected {
			m.mu.Unlock()
			return fmt.Errorf("connection cancelled")
		}
		m.runtimeStatus[id] = types.StatusError
		m.mu.Unlock()
		m.emitStatusChange()
		return err
	}

	// 连接成功后注册状态变更回调（TCP驱动支持）
	if notifier, ok := drv.(interface{ SetOnStatusChange(func()) }); ok {
		notifier.SetOnStatusChange(func() {
			m.mu.Lock()
			if drv.IsConnected() {
				m.runtimeStatus[id] = types.StatusConnected
			} else {
				m.runtimeStatus[id] = types.StatusError
			}
			m.mu.Unlock()
			m.emitStatusChange()
		})
	}

	m.mu.Lock()
	// 再次检查是否在连接过程中被 Disconnect 取消
	if m.runtimeStatus[id] == types.StatusDisconnected {
		m.mu.Unlock()
		drv.Disconnect() // 清理已建立的连接
		return fmt.Errorf("connection cancelled")
	}
	m.instances[id] = drv
	m.runtimeStatus[id] = types.StatusConnected
	profile.Channels = drv.GetChannels()
	m.profiles[id] = profile
	m.mu.Unlock()
	m.saveProfiles()
	m.emitStatusChange()
	return nil
}

// Disconnect 断开设备
func (m *DeviceManager) Disconnect(id string) {
	m.mu.Lock()
	if drv, ok := m.instances[id]; ok {
		drv.Disconnect()
		delete(m.instances, id)
	}
	m.runtimeStatus[id] = types.StatusDisconnected
	m.mu.Unlock()
	m.emitStatusChange()
}

// StartAcquisition 启动采集
func (m *DeviceManager) StartAcquisition(id string, periodMs int) error {
	m.mu.RLock()
	drv, ok := m.instances[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("device not connected: %s", id)
	}
	return drv.StartAcquisition(periodMs)
}

// StopAcquisition 停止采集
func (m *DeviceManager) StopAcquisition(id string) error {
	m.mu.RLock()
	drv, ok := m.instances[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("device not connected: %s", id)
	}
	return drv.StopAcquisition()
}

// StartAcquisitionAll 批量启动采集（使用各设备配置的periodMs）
func (m *DeviceManager) StartAcquisitionAll() int {
	m.mu.RLock()
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
	m.mu.RUnlock()

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
	m.mu.RLock()
	instances := make(map[string]DeviceDriver, len(m.instances))
	for id, drv := range m.instances {
		instances[id] = drv
	}
	m.mu.RUnlock()

	for id, drv := range instances {
		if err := drv.StopAcquisition(); err != nil {
			slog.Error("stop acquisition failed", "id", id, "err", err)
		}
	}
}

// GetStatusAll 获取所有设备状态
func (m *DeviceManager) GetStatusAll() []types.DeviceStatus {
	m.mu.RLock()
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
	m.mu.RUnlock()

	statuses := []types.DeviceStatus{}
	for id, profile := range profiles {
		status := types.DeviceStatus{
			ID:   id,
			Name: profile.Name,
			Type: profile.Type,
		}
		if drv, ok := instances[id]; ok {
			// 驱动实例存在，以驱动实际状态为准
			if drv.IsConnected() {
				status.Status = types.StatusConnected
			} else {
				status.Status = types.StatusError
			}
			status.Acquiring = drv.IsAcquiring()
		} else if rs, ok := runtimeStatus[id]; ok {
			// 无驱动实例但有运行时状态记录（Connecting/Error/Disconnected）
			status.Status = rs
		} else {
			status.Status = types.StatusDisconnected
		}
		statuses = append(statuses, status)
	}
	return statuses
}

// GetLatestData 获取指定设备最新数据
func (m *DeviceManager) GetLatestData(deviceID string) (types.DataPayload, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.latestData[deviceID]
	return data, ok
}

// GetChannelValue 获取指定通道值
func (m *DeviceManager) GetChannelValue(deviceID string, channelIndex int) (float64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
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
	m.mu.RLock()
	defer m.mu.RUnlock()
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

// ThermocoupleTypeSetter 热电偶类型设置接口（仅 YX-DAQ-T 驱动实现）
type ThermocoupleTypeSetter interface {
	SetThermocoupleType(tcTypes string) error
	SetSingleThermocoupleType(channelIndex int, tcType string) error
}

// SetUnit 设置设备压力单位（写入硬件）
func (m *DeviceManager) SetUnit(id string, unit string) error {
	m.mu.RLock()
	drv, ok := m.instances[id]
	m.mu.RUnlock()
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

	m.mu.Lock()
	if profile, exists := m.profiles[id]; exists {
		for i := range profile.Channels {
			if profile.Channels[i].Index < profile.Type.PressureChannelCount() {
				profile.Channels[i].Unit = unit
			}
		}
		m.profiles[id] = profile
	}
	m.mu.Unlock()
	m.saveProfiles()

	return nil
}

// SetThermocoupleType 设置设备热电偶类型（全通道批量设置，写入硬件）
func (m *DeviceManager) SetThermocoupleType(id string, tcTypes string) error {
	m.mu.RLock()
	drv, ok := m.instances[id]
	m.mu.RUnlock()
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

	// 更新 profile 中的通道热电偶类型
	runes := []rune(tcTypes)
	m.mu.Lock()
	if profile, exists := m.profiles[id]; exists {
		if len(runes) == 16 {
			for i := range profile.Channels {
				if i < 16 {
					profile.Channels[i].ThermocoupleType = string(runes[i])
				}
			}
		}
		m.profiles[id] = profile
	}
	m.mu.Unlock()
	m.saveProfiles()

	return nil
}

// SetSingleThermocoupleType 设置单个通道的热电偶类型（写入硬件）
func (m *DeviceManager) SetSingleThermocoupleType(id string, channelIndex int, tcType string) error {
	m.mu.RLock()
	drv, ok := m.instances[id]
	m.mu.RUnlock()
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

	// 更新 profile 中对应通道的热电偶类型
	m.mu.Lock()
	if profile, exists := m.profiles[id]; exists {
		for i := range profile.Channels {
			if profile.Channels[i].Index == channelIndex {
				profile.Channels[i].ThermocoupleType = tcType
				break
			}
		}
		m.profiles[id] = profile
	}
	m.mu.Unlock()
	m.saveProfiles()

	return nil
}

// IsAcquiring 检查指定设备是否正在采集
func (m *DeviceManager) IsAcquiring(id string) bool {
	m.mu.RLock()
	drv, ok := m.instances[id]
	m.mu.RUnlock()
	if !ok {
		return false
	}
	return drv.IsAcquiring()
}

// IsConnected 检查设备是否连接
func (m *DeviceManager) IsConnected(id string) bool {
	m.mu.RLock()
	drv, ok := m.instances[id]
	m.mu.RUnlock()
	if !ok {
		return false
	}
	return drv.IsConnected()
}

// Init 初始化（从配置文件加载设备，若无则创建默认模拟设备）
func (m *DeviceManager) Init() {
	loaded := false
	if m.configStore != nil {
		profiles := m.configStore.Get()
		if len(profiles) > 0 {
			for _, p := range profiles {
				m.mu.Lock()
				m.profiles[p.ID] = p
				m.mu.Unlock()
			}
			loaded = true
			slog.Info("loaded device profiles from config", "count", len(profiles))
		}
	}

	if !loaded {
		// 无配置文件，创建默认模拟设备（默认DAQ16通道规格）
		pressureCount := types.DeviceTypeXYDAQ16.PressureChannelCount()
		totalChannels := types.DeviceTypeXYDAQ16.TotalChannelCount()
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

		m.mu.Lock()
		m.profiles[simProfile.ID] = simProfile
		m.mu.Unlock()
		m.saveProfiles()
	}

	// 自动连接所有设备
	m.AutoConnect()
}

// AutoConnect 自动连接所有启用了自动连接的设备
func (m *DeviceManager) AutoConnect() {
	m.mu.RLock()
	type kv struct {
		id   string
		auto bool
	}
	pairs := make([]kv, 0, len(m.profiles))
	for id, p := range m.profiles {
		pairs = append(pairs, kv{id, p.AutoConnect})
	}
	m.mu.RUnlock()

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
