package manager

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"yx-daq/internal/driver"
	"yx-daq/internal/storage"
	"yx-daq/internal/types"
)

// MotionController 运动控制器接口
type MotionController interface {
	Connect() error
	Disconnect()
	IsConnected() bool
	MoveTo(axis types.AxisName, position float64) error
	MoveBy(axis types.AxisName, delta float64) error
	Jog(axis types.AxisName, direction int, distance float64, speed float64) error
	Home(axis types.AxisName) error
	Stop(axis types.AxisName) error
	StopAll() error
	EmergencyStop() error
	DefinePosition(axis types.AxisName, position float64) error
	GetAxisStatus(axis types.AxisName) (types.AxisStatus, error)
	GetAllAxisStatus() ([]types.AxisStatus, error)
	SetSpeed(axis types.AxisName, speed float64) error
	SetAcceleration(axis types.AxisName, accel float64) error
	SetDeceleration(axis types.AxisName, decel float64) error
	IsMoving() (bool, error)
	IsAxisMoving(axis types.AxisName) (bool, error)
	GetLimitStatus(axis types.AxisName) (types.LimitStatus, error)
	WaitForMotionComplete(axis types.AxisName, timeoutMs int) error
	MotorOff() error
	SetAxisDirection(axis types.AxisName, reverse bool) error
}

type axisConfigUpdater interface {
	UpdateAxes([]types.AxisConfig)
}

// MotionControllerFactory 运动控制器工厂函数类型
type MotionControllerFactory func(profile types.MotionControllerProfile) MotionController

// controllerFactories 控制器工厂注册表 — 新增控制器类型只需在此注册工厂函数
var controllerFactories = map[types.MotionControllerType]MotionControllerFactory{
	types.MotionTypeB140: func(p types.MotionControllerProfile) MotionController {
		b140Drv := driver.NewB140Driver(p.Address, p.Port, p.TimeoutMs)
		return driver.NewB140MotionController(b140Drv, p.Axes)
	},
	types.MotionTypeSimulated: func(p types.MotionControllerProfile) MotionController {
		return driver.NewSimulatedMotionController(p.Axes)
	},
}

// MotionControllerManager 运动控制器管理器
type MotionControllerManager struct {
	mu          sync.RWMutex
	profiles    map[string]types.MotionControllerProfile
	instances   map[string]MotionController
	statuses    map[string][]types.AxisStatus
	pollCancel  chan struct{}
	pollRunning atomic.Bool
	configStore *storage.ConfigStore[[]types.MotionControllerProfile]
	// 运行时连接状态（独立于控制器实例，用于在实例创建前/断连后仍可查询）
	runtimeStatus map[string]types.ConnectionStatus
	// 状态变更回调（由 Core 层桥接到 Wails 事件系统）
	onStatusChange func(statuses []types.MotionControllerStatus)
}

// NewMotionControllerManager 创建运动控制器管理器
func NewMotionControllerManager() *MotionControllerManager {
	return &MotionControllerManager{
		profiles:      make(map[string]types.MotionControllerProfile),
		instances:     make(map[string]MotionController),
		statuses:      make(map[string][]types.AxisStatus),
		pollCancel:    make(chan struct{}),
		runtimeStatus: make(map[string]types.ConnectionStatus),
	}
}

// SetConfigStore 设置配置存储（用于持久化）
func (m *MotionControllerManager) SetConfigStore(store *storage.ConfigStore[[]types.MotionControllerProfile]) {
	m.configStore = store
}

// saveProfiles 持久化运动控制器配置到磁盘
func (m *MotionControllerManager) saveProfiles() {
	if m.configStore == nil {
		return
	}
	profiles := m.GetProfiles()
	if err := m.configStore.Set(profiles); err != nil {
		slog.Error("save motion profiles failed", "err", err)
	}
}

// SetOnStatusChange 设置状态变更回调（由 Core 层桥接到 Wails 事件系统）
// 必须在应用启动时调用，之后不再变更
func (m *MotionControllerManager) SetOnStatusChange(cb func(statuses []types.MotionControllerStatus)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onStatusChange = cb
}

// emitStatusChange 发射状态变更事件
// 注意：必须在未持有 m.mu 时调用，否则 GetStatusAll 中的 RLock 会死锁
func (m *MotionControllerManager) emitStatusChange() {
	m.mu.RLock()
	cb := m.onStatusChange
	m.mu.RUnlock()
	if cb != nil {
		cb(m.GetStatusAll())
	}
}

// AddProfile 添加控制器配置
func (m *MotionControllerManager) AddProfile(profile types.MotionControllerProfile) {
	m.mu.Lock()
	m.profiles[profile.ID] = profile
	m.mu.Unlock()
	m.saveProfiles()
}

// GetProfiles 获取所有控制器配置
func (m *MotionControllerManager) GetProfiles() []types.MotionControllerProfile {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]types.MotionControllerProfile, 0, len(m.profiles))
	for _, p := range m.profiles {
		result = append(result, p)
	}
	return result
}

// IsConnected 检查控制器是否连接
func (m *MotionControllerManager) IsConnected(id string) bool {
	m.mu.RLock()
	ctrl, ok := m.instances[id]
	m.mu.RUnlock()
	if !ok {
		return false
	}
	return ctrl.IsConnected()
}

// Connect 连接控制器
// 采用分段锁：仅在访问共享状态时持锁，TCP 拨号在锁外执行，
// 通过 runtimeStatus 标记 Connecting/Error/Connected 让前端可见连接过程。
func (m *MotionControllerManager) Connect(id string) error {
	m.mu.Lock()
	profile, ok := m.profiles[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("motion controller profile not found: %s", id)
	}
	if existing, ok := m.instances[id]; ok {
		existing.Disconnect()
		delete(m.instances, id)
	}
	m.runtimeStatus[id] = types.StatusConnecting
	m.mu.Unlock()
	m.emitStatusChange()

	factory, ok := controllerFactories[profile.Type]
	if !ok {
		m.mu.Lock()
		m.runtimeStatus[id] = types.StatusError
		m.mu.Unlock()
		m.emitStatusChange()
		return fmt.Errorf("unsupported motion controller type: %s", profile.Type)
	}
	ctrl := factory(profile)

	if err := ctrl.Connect(); err != nil {
		m.mu.Lock()
		// 检查连接过程中是否被 RemoveProfile 取消：
		// RemoveProfile 用 delete(m.runtimeStatus, id) 清理，读取得到零值 ""（非 "Disconnected"），
		// 故必须用 profile 存在性判断取消；runtimeStatus == StatusDisconnected 判断 Disconnect 显式断开。
		if _, ok := m.profiles[id]; !ok {
			m.mu.Unlock()
			ctrl.Disconnect()
			return fmt.Errorf("connection cancelled")
		}
		if m.runtimeStatus[id] == types.StatusDisconnected {
			m.mu.Unlock()
			ctrl.Disconnect()
			return fmt.Errorf("connection cancelled")
		}
		m.runtimeStatus[id] = types.StatusError
		m.mu.Unlock()
		m.emitStatusChange()
		return err
	}

	m.mu.Lock()
	// 再次检查连接过程中是否被 RemoveProfile / Disconnect 取消
	if _, ok := m.profiles[id]; !ok {
		m.mu.Unlock()
		ctrl.Disconnect() // 清理已建立的连接
		return fmt.Errorf("connection cancelled")
	}
	if m.runtimeStatus[id] == types.StatusDisconnected {
		m.mu.Unlock()
		ctrl.Disconnect() // 清理已建立的连接
		return fmt.Errorf("connection cancelled")
	}
	m.instances[id] = ctrl
	m.runtimeStatus[id] = types.StatusConnected
	m.mu.Unlock()
	m.emitStatusChange()
	return nil
}

// Disconnect 断开控制器
func (m *MotionControllerManager) Disconnect(id string) {
	m.mu.Lock()
	if ctrl, ok := m.instances[id]; ok {
		ctrl.Disconnect()
		delete(m.instances, id)
	}
	m.runtimeStatus[id] = types.StatusDisconnected
	m.mu.Unlock()
	m.emitStatusChange()
}

// MoveTo 绝对定位
func (m *MotionControllerManager) MoveTo(id string, axis types.AxisName, position float64) error {
	m.mu.RLock()
	ctrl, ok := m.instances[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("motion controller not connected: %s", id)
	}
	return ctrl.MoveTo(axis, position)
}

// MoveBy 相对移动
func (m *MotionControllerManager) MoveBy(id string, axis types.AxisName, delta float64) error {
	m.mu.RLock()
	ctrl, ok := m.instances[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("motion controller not connected: %s", id)
	}
	return ctrl.MoveBy(axis, delta)
}

// Jog 点动
func (m *MotionControllerManager) Jog(id string, axis types.AxisName, direction int, distance float64, speed float64) error {
	m.mu.RLock()
	ctrl, ok := m.instances[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("motion controller not connected: %s", id)
	}
	return ctrl.Jog(axis, direction, distance, speed)
}

// Home 回零
func (m *MotionControllerManager) Home(id string, axis types.AxisName) error {
	m.mu.RLock()
	ctrl, ok := m.instances[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("motion controller not connected: %s", id)
	}
	return ctrl.Home(axis)
}

// Stop 停止
func (m *MotionControllerManager) Stop(id string, axis types.AxisName) error {
	m.mu.RLock()
	ctrl, ok := m.instances[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("motion controller not connected: %s", id)
	}
	return ctrl.Stop(axis)
}

// StopAll 停止所有轴
func (m *MotionControllerManager) StopAll(id string) error {
	m.mu.RLock()
	ctrl, ok := m.instances[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("motion controller not connected: %s", id)
	}
	return ctrl.StopAll()
}

// EmergencyStop 急停
func (m *MotionControllerManager) EmergencyStop(id string) error {
	m.mu.RLock()
	ctrl, ok := m.instances[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("motion controller not connected: %s", id)
	}
	return ctrl.EmergencyStop()
}

// DefinePosition 置位
func (m *MotionControllerManager) DefinePosition(id string, axis types.AxisName, position float64) error {
	m.mu.RLock()
	ctrl, ok := m.instances[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("motion controller not connected: %s", id)
	}
	return ctrl.DefinePosition(axis, position)
}

// GetStatusAll 获取所有控制器状态
func (m *MotionControllerManager) GetStatusAll() []types.MotionControllerStatus {
	m.mu.RLock()
	profiles := make(map[string]types.MotionControllerProfile, len(m.profiles))
	for id, p := range m.profiles {
		profiles[id] = p
	}
	instances := make(map[string]MotionController, len(m.instances))
	for id, ctrl := range m.instances {
		instances[id] = ctrl
	}
	cachedStatuses := make(map[string][]types.AxisStatus, len(m.statuses))
	for id, axes := range m.statuses {
		cachedStatuses[id] = axes
	}
	runtimeStatus := make(map[string]types.ConnectionStatus, len(m.runtimeStatus))
	for id, s := range m.runtimeStatus {
		runtimeStatus[id] = s
	}
	m.mu.RUnlock()

	statuses := []types.MotionControllerStatus{}
	for id, profile := range profiles {
		status := types.MotionControllerStatus{
			ID:   id,
			Name: profile.Name,
			Type: profile.Type,
		}
		if ctrl, ok := instances[id]; ok {
			// 控制器实例存在，以实例实际状态为准
			if ctrl.IsConnected() {
				status.Status = types.StatusConnected
			} else {
				status.Status = types.StatusError
			}
			if axes, err := ctrl.GetAllAxisStatus(); err == nil {
				status.Axes = axes
			} else if axes, ok := cachedStatuses[id]; ok {
				status.Axes = axes
			}
		} else if rs, ok := runtimeStatus[id]; ok {
			// 无实例但有运行时状态记录（Connecting/Error/Disconnected）
			status.Status = rs
			if axes, ok := cachedStatuses[id]; ok {
				status.Axes = axes
			}
		} else {
			status.Status = types.StatusDisconnected
			if axes, ok := cachedStatuses[id]; ok {
				status.Axes = axes
			}
		}
		statuses = append(statuses, status)
	}
	return statuses
}

// GetCachedStatusAll 获取所有控制器的缓存状态（无实时查询）
func (m *MotionControllerManager) GetCachedStatusAll() []types.MotionControllerStatus {
	m.mu.RLock()
	profiles := make(map[string]types.MotionControllerProfile, len(m.profiles))
	for id, p := range m.profiles {
		profiles[id] = p
	}
	instances := make(map[string]MotionController, len(m.instances))
	for id, ctrl := range m.instances {
		instances[id] = ctrl
	}
	cachedStatuses := make(map[string][]types.AxisStatus, len(m.statuses))
	for id, axes := range m.statuses {
		cachedStatuses[id] = axes
	}
	runtimeStatus := make(map[string]types.ConnectionStatus, len(m.runtimeStatus))
	for id, s := range m.runtimeStatus {
		runtimeStatus[id] = s
	}
	m.mu.RUnlock()

	statuses := []types.MotionControllerStatus{}
	for id, profile := range profiles {
		status := types.MotionControllerStatus{
			ID:   id,
			Name: profile.Name,
			Type: profile.Type,
		}
		if ctrl, ok := instances[id]; ok {
			if ctrl.IsConnected() {
				status.Status = types.StatusConnected
			} else {
				status.Status = types.StatusError
			}
		} else if rs, ok := runtimeStatus[id]; ok {
			status.Status = rs
		} else {
			status.Status = types.StatusDisconnected
		}
		if axes, ok := cachedStatuses[id]; ok {
			status.Axes = axes
		}
		statuses = append(statuses, status)
	}
	return statuses
}

// StartPolling 启动状态轮询
func (m *MotionControllerManager) StartPolling() {
	if m.pollRunning.Load() {
		return
	}
	m.pollRunning.Store(true)
	defer m.pollRunning.Store(false)

	ticker := time.NewTicker(time.Duration(types.MotionPollIntervalMs) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-m.pollCancel:
			return
		case <-ticker.C:
			m.pollStatus()
		}
	}
}

// StopPolling 停止轮询
func (m *MotionControllerManager) StopPolling() {
	select {
	case m.pollCancel <- struct{}{}:
	default:
	}
}

// pollStatus 轮询所有控制器状态
func (m *MotionControllerManager) pollStatus() {
	m.mu.RLock()
	// 在读锁下收集需要轮询的控制器
	toPoll := make(map[string]MotionController)
	for id, ctrl := range m.instances {
		if ctrl.IsConnected() {
			toPoll[id] = ctrl
		}
	}
	m.mu.RUnlock()

	// 在锁外执行轮询（可能耗时）
	results := make(map[string][]types.AxisStatus)
	for id, ctrl := range toPoll {
		if axes, err := ctrl.GetAllAxisStatus(); err == nil {
			results[id] = axes
		}
	}

	// 用写锁更新状态
	m.mu.Lock()
	for id, axes := range results {
		m.statuses[id] = axes
	}
	m.mu.Unlock()
}

// Init 初始化（从配置文件加载控制器，若无则创建默认模拟控制器）
func (m *MotionControllerManager) Init() {
	loaded := false
	if m.configStore != nil {
		profiles := m.configStore.Get()
		if len(profiles) > 0 {
			m.mu.Lock()
			for _, p := range profiles {
				m.profiles[p.ID] = p
			}
			m.mu.Unlock()
			loaded = true
			slog.Info("loaded motion controller profiles from config", "count", len(profiles))
		}
	}

	if !loaded {
		// 无配置文件，创建默认模拟控制器
		defaultProfile := types.MotionControllerProfile{
			ID:        "sim-mc-default",
			Name:      "模拟运动控制器",
			Type:      types.MotionTypeSimulated,
			Address:   "127.0.0.1",
			Port:      5000,
			TimeoutMs: 5000,
			Axes:      types.DefaultAxisConfigs(),
		}
		m.mu.Lock()
		m.profiles[defaultProfile.ID] = defaultProfile
		m.mu.Unlock()
		m.saveProfiles()
	}

	// 自动连接：SIMULATED 同步，B140 异步（避免 TCP 拨号超时阻塞应用启动）
	m.mu.RLock()
	type profileEntry struct {
		id   string
		kind types.MotionControllerType
	}
	entries := make([]profileEntry, 0, len(m.profiles))
	for id, p := range m.profiles {
		entries = append(entries, profileEntry{id, p.Type})
	}
	m.mu.RUnlock()

	for _, e := range entries {
		if e.kind == types.MotionTypeB140 {
			pid := e.id
			go func() {
				if err := m.Connect(pid); err != nil {
					slog.Warn("auto-connect B140 failed (manual connect available)", "id", pid, "err", err)
				}
			}()
		} else {
			if err := m.Connect(e.id); err != nil {
				slog.Error("auto-connect motion controller failed", "id", e.id, "err", err)
			}
		}
	}

	go m.StartPolling()
}

// SetAcceleration 设置加速度
func (m *MotionControllerManager) SetAcceleration(id string, axis types.AxisName, accel float64) error {
	m.mu.RLock()
	ctrl, ok := m.instances[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("motion controller not connected: %s", id)
	}
	return ctrl.SetAcceleration(axis, accel)
}

// SetDeceleration 设置减速度
func (m *MotionControllerManager) SetDeceleration(id string, axis types.AxisName, decel float64) error {
	m.mu.RLock()
	ctrl, ok := m.instances[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("motion controller not connected: %s", id)
	}
	return ctrl.SetDeceleration(axis, decel)
}

// IsMoving 查询是否有轴在运动
func (m *MotionControllerManager) IsMoving(id string) (bool, error) {
	m.mu.RLock()
	ctrl, ok := m.instances[id]
	m.mu.RUnlock()
	if !ok {
		return false, fmt.Errorf("motion controller not connected: %s", id)
	}
	return ctrl.IsMoving()
}

// IsAxisMoving 查询单轴是否在运动
func (m *MotionControllerManager) IsAxisMoving(id string, axis types.AxisName) (bool, error) {
	m.mu.RLock()
	ctrl, ok := m.instances[id]
	m.mu.RUnlock()
	if !ok {
		return false, fmt.Errorf("motion controller not connected: %s", id)
	}
	return ctrl.IsAxisMoving(axis)
}

// GetLimitStatus 查询轴限位状态
func (m *MotionControllerManager) GetLimitStatus(id string, axis types.AxisName) (types.LimitStatus, error) {
	m.mu.RLock()
	ctrl, ok := m.instances[id]
	m.mu.RUnlock()
	if !ok {
		return types.LimitStatus{}, fmt.Errorf("motion controller not connected: %s", id)
	}
	return ctrl.GetLimitStatus(axis)
}

// WaitForMotionComplete 等待运动完成
func (m *MotionControllerManager) WaitForMotionComplete(id string, axis types.AxisName, timeoutMs int) error {
	m.mu.RLock()
	ctrl, ok := m.instances[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("motion controller not connected: %s", id)
	}
	return ctrl.WaitForMotionComplete(axis, timeoutMs)
}

// MotorOff 关闭电机
func (m *MotionControllerManager) MotorOff(id string) error {
	m.mu.RLock()
	ctrl, ok := m.instances[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("motion controller not connected: %s", id)
	}
	return ctrl.MotorOff()
}

// SetAxisDirection 设置轴方向
func (m *MotionControllerManager) SetAxisDirection(id string, axis types.AxisName, reverse bool) error {
	m.mu.RLock()
	ctrl, ok := m.instances[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("motion controller not connected: %s", id)
	}
	return ctrl.SetAxisDirection(axis, reverse)
}

// UpdateProfile 更新控制器配置
func (m *MotionControllerManager) UpdateProfile(profile types.MotionControllerProfile) {
	m.mu.Lock()
	m.profiles[profile.ID] = profile
	if ctrl, ok := m.instances[profile.ID]; ok {
		if updater, canUpdate := ctrl.(axisConfigUpdater); canUpdate {
			updater.UpdateAxes(profile.Axes)
		}
	}
	m.mu.Unlock()
	m.saveProfiles()
}

// RemoveProfile 删除控制器配置（同时断开连接并清理运行时状态）
func (m *MotionControllerManager) RemoveProfile(id string) {
	m.mu.Lock()
	if ctrl, ok := m.instances[id]; ok {
		ctrl.Disconnect()
		delete(m.instances, id)
	}
	delete(m.profiles, id)
	delete(m.statuses, id)
	delete(m.runtimeStatus, id)
	m.mu.Unlock()
	m.saveProfiles()
	m.emitStatusChange()
}
