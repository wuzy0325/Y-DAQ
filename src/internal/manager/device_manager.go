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
	drv := driver.NewXYDAQDriver(profile.Host, profile.Port, profile.StreamID, profile.Channels, profile.Type, profile.AtmEnabled)
	drv.SetDeviceID(profile.ID)
	return drv
}

func newYXDAQTDriver(profile types.DeviceProfile) DeviceDriver {
	drv := driver.NewYXDAQTDriver(profile.Host, profile.Port, profile.Channels)
	drv.SetDeviceID(profile.ID)
	return drv
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
	syncAtmChannelEnabled(&profile)
	m.addProfile(profile.ID, profile)
	m.saveProfilesWithLog("device")
}

// UpdateProfile 更新设备配置。
// 采集过程中禁止修改配置：设备正在采集时拒绝更新，避免在途数据帧与通道配置错位。
// 已连接设备的通道热更新（UpdateChannels）在锁外调用，避免持锁调用驱动方法。
func (m *DeviceManager) UpdateProfile(profile types.DeviceProfile) error {
	m.RLock()
	_, exists := m.profiles[profile.ID]
	drv, connected := m.instances[profile.ID]
	m.RUnlock()
	if !exists {
		return fmt.Errorf("设备配置不存在: %s", profile.ID)
	}
	// 未连接设备 instances 中无驱动（drv 为 nil），不能做采集校验，直接允许修改
	if connected {
		if err := m.ensureConfigMutable(profile.ID, drv); err != nil {
			return err
		}
	}

	syncAtmChannelEnabled(&profile)

	m.Lock()
	m.profiles[profile.ID] = profile
	m.Unlock()
	if connected {
		drv.UpdateChannels(profile.Channels)
		// 大气压/温度使能热更新（下次启动采集时下发 c 05 命令）
		if cfg, ok := drv.(AtmChannelConfigurator); ok {
			cfg.SetAtmEnabled(profile.AtmEnabled)
		}
	}
	m.saveProfilesWithLog("device")
	return nil
}

// RemoveProfile 删除设备配置（同时断开连接和停止采集）。
// 采集过程中禁止删除：删除会断开连接并移除配置，属配置改变，正在采集时拒绝。
func (m *DeviceManager) RemoveProfile(id string) error {
	m.RLock()
	drv, ok := m.instances[id]
	m.RUnlock()
	if ok {
		if err := m.ensureConfigMutable(id, drv); err != nil {
			return err
		}
	}

	m.Lock()
	drv, hasDrv := m.instances[id]
	delete(m.instances, id)
	delete(m.profiles, id)
	delete(m.latestData, id)
	delete(m.runtimeStatus, id)
	m.Unlock()
	// 驱动关闭含 FIN/接收协程 join 等待，锁外执行避免阻塞管理器
	if hasDrv {
		drv.Disconnect()
	}
	m.saveProfilesWithLog("device")
	m.emitStatusChange()
	return nil
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
	// 并发点击连接防护：上一次连接流程未结束时再次连接会拨出第二条 TCP
	// 连接，被单连接设备直接拒绝（或两条连接并存导致数据错乱）
	if m.runtimeStatus[id] == types.StatusConnecting {
		m.Unlock()
		return fmt.Errorf("device is connecting")
	}
	// 设备只允许单连接：先摘除并断开同一 profile 的旧实例，
	// 避免拨出第二条 TCP 连接被设备拒绝，或两条连接并存导致数据错乱
	existing, hasExisting := m.instances[id]
	delete(m.instances, id)
	m.runtimeStatus[id] = types.StatusConnecting
	m.Unlock()
	if hasExisting {
		existing.Disconnect()
	}
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
		// 清理可能已建立的连接（拨号成功但初始化失败的半开连接），
		// 避免泄漏设备单连接槽位导致后续重连被设备拒绝
		drv.Disconnect()
		m.Lock()
		if _, exists := m.profiles[id]; !exists {
			// 拨号期间设备已被删除：不回写状态，否则删除后又冒出一条状态记录
			delete(m.runtimeStatus, id)
			m.Unlock()
			return fmt.Errorf("device profile removed during connect")
		}
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
	if _, exists := m.profiles[id]; !exists {
		// 拨号期间设备已被删除：断开刚建立的连接，不复活设备
		delete(m.runtimeStatus, id)
		m.Unlock()
		drv.Disconnect()
		return fmt.Errorf("device profile removed during connect")
	}
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
	drv, ok := m.instances[id]
	if ok {
		delete(m.instances, id)
	}
	m.runtimeStatus[id] = types.StatusDisconnected
	m.Unlock()
	// 驱动关闭含 FIN/接收协程 join 等待，锁外执行避免阻塞管理器
	if ok {
		drv.Disconnect()
	}
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

// Init 初始化（从配置文件加载设备，若无则创建默认模拟设备）
func (m *DeviceManager) Init() {
	loaded := false
	if m.configStore != nil {
		profiles := m.configStore.Get()
		if len(profiles) > 0 {
			migrated := 0
			normalized := 0
			for i := range profiles {
				p := &profiles[i]
				if newType, changed := types.MigrateDeviceType(p.Type); changed {
					slog.Info("migrate legacy device type", "id", p.ID, "old", p.Type, "new", newType)
					p.Type = newType
					migrated++
				}
				if syncAtmChannelEnabled(p) {
					normalized++
				}
				m.Lock()
				m.profiles[p.ID] = *p
				m.Unlock()
			}
			loaded = true
			slog.Info("loaded device profiles from config", "count", len(profiles), "migrated", migrated, "normalized", normalized)
			if migrated > 0 || normalized > 0 {
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
			AtmEnabled:  true,
			Channels:    defaultChannels,
		}

		m.Lock()
		m.profiles[simProfile.ID] = simProfile
		m.Unlock()
		m.saveProfilesWithLog("device")
	}

	m.AutoConnect()
}

// AutoConnect 自动连接所有启用了自动连接的设备。
// 每台设备在独立协程中连接：TCP 拨号需等待超时（离线设备约 5s），同步连接会阻塞
// Core.Startup；而 Wails v3 在所有 ServiceStartup 完成后才创建并显示窗口，
// 导致启动到出现画面被拨号超时拖慢。与 MotionManager 的异步自动连接策略一致。
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
		if !pair.auto || m.IsConnected(pair.id) {
			continue
		}
		id := pair.id
		go func() {
			if err := m.Connect(id); err != nil {
				slog.Error("auto connect device failed", "id", id, "err", err)
			}
		}()
	}
}
