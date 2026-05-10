package manager

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"yx-daq/internal/driver"
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

// MotionControllerManager 运动控制器管理器
type MotionControllerManager struct {
	mu          sync.RWMutex
	profiles    map[string]types.MotionControllerProfile
	instances   map[string]MotionController
	statuses    map[string][]types.AxisStatus
	pollCancel  chan struct{}
	pollRunning atomic.Bool
}

// NewMotionControllerManager 创建运动控制器管理器
func NewMotionControllerManager() *MotionControllerManager {
	return &MotionControllerManager{
		profiles:   make(map[string]types.MotionControllerProfile),
		instances:  make(map[string]MotionController),
		statuses:   make(map[string][]types.AxisStatus),
		pollCancel: make(chan struct{}),
	}
}

// AddProfile 添加控制器配置
func (m *MotionControllerManager) AddProfile(profile types.MotionControllerProfile) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.profiles[profile.ID] = profile
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
func (m *MotionControllerManager) Connect(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	profile, ok := m.profiles[id]
	if !ok {
		return fmt.Errorf("motion controller profile not found: %s", id)
	}
	if existing, ok := m.instances[id]; ok {
		existing.Disconnect()
		delete(m.instances, id)
	}

	var ctrl MotionController
	switch profile.Type {
	case types.MotionTypeB140:
		b140Drv := driver.NewB140Driver(profile.Address, profile.Port, profile.TimeoutMs)
		ctrl = driver.NewB140MotionController(b140Drv, profile.Axes)
	case types.MotionTypeSimulated:
		ctrl = driver.NewSimulatedMotionController(profile.Axes)
	default:
		return fmt.Errorf("unsupported motion controller type: %s", profile.Type)
	}

	if err := ctrl.Connect(); err != nil {
		return err
	}

	m.instances[id] = ctrl
	return nil
}

// Disconnect 断开控制器
func (m *MotionControllerManager) Disconnect(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ctrl, ok := m.instances[id]; ok {
		ctrl.Disconnect()
		delete(m.instances, id)
	}
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
				status.Status = types.StatusDisconnected
			}
			if axes, err := ctrl.GetAllAxisStatus(); err == nil {
				status.Axes = axes
			} else if axes, ok := cachedStatuses[id]; ok {
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
				status.Status = types.StatusDisconnected
			}
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

// Init 初始化（创建默认模拟控制器）
func (m *MotionControllerManager) Init() {
	axes := types.DefaultAxisConfigs()

	// B140 运动控制器（真实硬件）
	b140Profile := types.MotionControllerProfile{
		ID:        "b140-mc-1",
		Name:      "B140 运动控制器",
		Type:      types.MotionTypeB140,
		Address:   "192.168.1.101",
		Port:      5000,
		TimeoutMs: 5000,
		Axes:      axes,
	}

	simProfile := types.MotionControllerProfile{
		ID:        "sim-mc-1",
		Name:      "模拟运动控制器",
		Type:      types.MotionTypeSimulated,
		Address:   "127.0.0.1",
		Port:      5000,
		TimeoutMs: 5000,
		Axes:      axes,
	}

	m.AddProfile(b140Profile)
	m.AddProfile(simProfile)

	// 启动时自动连接一次（失败不重试，用户可通过 UI 手动连接）
	if err := m.Connect(simProfile.ID); err != nil {
		slog.Error("connect simulated motion controller failed", "err", err)
	}
	if err := m.Connect(b140Profile.ID); err != nil {
		slog.Warn("auto-connect B140 failed (manual connect available)", "id", b140Profile.ID, "address", b140Profile.Address, "err", err)
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
	defer m.mu.Unlock()
	m.profiles[profile.ID] = profile
	if ctrl, ok := m.instances[profile.ID]; ok {
		if updater, canUpdate := ctrl.(axisConfigUpdater); canUpdate {
			updater.UpdateAxes(profile.Axes)
		}
	}
}

// RemoveProfile 删除控制器配置
func (m *MotionControllerManager) RemoveProfile(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.profiles, id)
}
