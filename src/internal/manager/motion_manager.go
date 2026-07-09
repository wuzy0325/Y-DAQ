package manager

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
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

// MotionControllerFactory 运动控制器工厂函数类型
type MotionControllerFactory func(profile types.MotionControllerProfile) MotionController

// controllerFactories 控制器工厂注册表 — 新增控制器类型只需在此注册工厂函数
var controllerFactories = map[types.MotionControllerType]MotionControllerFactory{
	types.MotionTypeEA25MC04: func(p types.MotionControllerProfile) MotionController {
		b140Drv := driver.NewB140Driver(p.Address, p.Port, p.TimeoutMs)
		return driver.NewB140MotionController(b140Drv, p.Axes)
	},
	types.MotionTypeSimulated: func(p types.MotionControllerProfile) MotionController {
		return driver.NewSimulatedMotionController(p.Axes)
	},
}

// MotionControllerManager 运动控制器管理器
type MotionControllerManager struct {
	BaseProfileManager[types.MotionControllerProfile]
	instances    map[string]MotionController
	statuses     map[string][]types.AxisStatus
	pollMu       sync.Mutex
	pollCancelFn context.CancelFunc
	pollRunning  atomic.Bool
	// 运行时连接状态（独立于控制器实例，用于在实例创建前/断连后仍可查询）
	runtimeStatus map[string]types.ConnectionStatus
	// 状态变更回调（由 Core 层桥接到 Wails 事件系统）
	onStatusChange func(statuses []types.MotionControllerStatus)
}

// NewMotionControllerManager 创建运动控制器管理器
func NewMotionControllerManager() *MotionControllerManager {
	m := &MotionControllerManager{
		instances:     make(map[string]MotionController),
		statuses:      make(map[string][]types.AxisStatus),
		runtimeStatus: make(map[string]types.ConnectionStatus),
	}
	m.initProfiles()
	return m
}

// SetOnStatusChange 设置状态变更回调（由 Core 层桥接到 Wails 事件系统）
// 必须在应用启动时调用，之后不再变更
func (m *MotionControllerManager) SetOnStatusChange(cb func(statuses []types.MotionControllerStatus)) {
	m.Lock()
	defer m.Unlock()
	m.onStatusChange = cb
}

// emitStatusChange 发射状态变更事件
// 注意：必须在未持有锁时调用，否则 GetStatusAll 中的 RLock 会死锁
func (m *MotionControllerManager) emitStatusChange() {
	m.RLock()
	cb := m.onStatusChange
	m.RUnlock()
	if cb != nil {
		cb(m.GetStatusAll())
	}
}

// AddProfile 添加控制器配置
func (m *MotionControllerManager) AddProfile(profile types.MotionControllerProfile) {
	m.addProfile(profile.ID, profile)
	m.saveProfilesWithLog("motion")
}

// IsConnected 检查控制器是否连接
func (m *MotionControllerManager) IsConnected(id string) bool {
	m.RLock()
	ctrl, ok := m.instances[id]
	m.RUnlock()
	if !ok {
		return false
	}
	return ctrl.IsConnected()
}

// Connect 连接控制器
// 采用分段锁：仅在访问共享状态时持锁，TCP 拨号在锁外执行，
// 通过 runtimeStatus 标记 Connecting/Error/Connected 让前端可见连接过程。
func (m *MotionControllerManager) Connect(id string) error {
	m.Lock()
	profile, ok := m.profiles[id]
	if !ok {
		m.Unlock()
		return fmt.Errorf("motion controller profile not found: %s", id)
	}
	if existing, ok := m.instances[id]; ok {
		existing.Disconnect()
		delete(m.instances, id)
	}
	m.runtimeStatus[id] = types.StatusConnecting
	m.Unlock()
	m.emitStatusChange()

	factory, ok := controllerFactories[profile.Type]
	if !ok {
		m.Lock()
		m.runtimeStatus[id] = types.StatusError
		m.Unlock()
		m.emitStatusChange()
		return fmt.Errorf("unsupported motion controller type: %s", profile.Type)
	}
	ctrl := factory(profile)

	if err := ctrl.Connect(); err != nil {
		m.Lock()
		// 检查连接过程中是否被 RemoveProfile 取消：
		// RemoveProfile 用 delete(m.runtimeStatus, id) 清理，读取得到零值 ""（非 "Disconnected"），
		// 故必须用 profile 存在性判断取消；runtimeStatus == StatusDisconnected 判断 Disconnect 显式断开。
		if _, ok := m.profiles[id]; !ok {
			m.Unlock()
			ctrl.Disconnect()
			return fmt.Errorf("connection cancelled")
		}
		if m.runtimeStatus[id] == types.StatusDisconnected {
			m.Unlock()
			ctrl.Disconnect()
			return fmt.Errorf("connection cancelled")
		}
		m.runtimeStatus[id] = types.StatusError
		m.Unlock()
		m.emitStatusChange()
		return err
	}

	m.Lock()
	// 再次检查连接过程中是否被 RemoveProfile / Disconnect 取消
	if _, ok := m.profiles[id]; !ok {
		m.Unlock()
		ctrl.Disconnect() // 清理已建立的连接
		return fmt.Errorf("connection cancelled")
	}
	if m.runtimeStatus[id] == types.StatusDisconnected {
		m.Unlock()
		ctrl.Disconnect() // 清理已建立的连接
		return fmt.Errorf("connection cancelled")
	}
	m.instances[id] = ctrl
	m.runtimeStatus[id] = types.StatusConnected
	m.Unlock()
	m.emitStatusChange()
	return nil
}

// Disconnect 断开控制器
func (m *MotionControllerManager) Disconnect(id string) {
	m.Lock()
	if ctrl, ok := m.instances[id]; ok {
		ctrl.Disconnect()
		delete(m.instances, id)
	}
	m.runtimeStatus[id] = types.StatusDisconnected
	m.Unlock()
	m.emitStatusChange()
}

// MoveTo 绝对定位
func (m *MotionControllerManager) MoveTo(id string, axis types.AxisName, position float64) error {
	m.RLock()
	ctrl, ok := m.instances[id]
	m.RUnlock()
	if !ok {
		return fmt.Errorf("motion controller not connected: %s", id)
	}
	return ctrl.MoveTo(axis, position)
}

// MoveBy 相对移动
func (m *MotionControllerManager) MoveBy(id string, axis types.AxisName, delta float64) error {
	m.RLock()
	ctrl, ok := m.instances[id]
	m.RUnlock()
	if !ok {
		return fmt.Errorf("motion controller not connected: %s", id)
	}
	return ctrl.MoveBy(axis, delta)
}

// Jog 点动
func (m *MotionControllerManager) Jog(id string, axis types.AxisName, direction int, distance float64, speed float64) error {
	m.RLock()
	ctrl, ok := m.instances[id]
	m.RUnlock()
	if !ok {
		return fmt.Errorf("motion controller not connected: %s", id)
	}
	return ctrl.Jog(axis, direction, distance, speed)
}

// Home 回零
func (m *MotionControllerManager) Home(id string, axis types.AxisName) error {
	m.RLock()
	ctrl, ok := m.instances[id]
	m.RUnlock()
	if !ok {
		return fmt.Errorf("motion controller not connected: %s", id)
	}
	return ctrl.Home(axis)
}

// Stop 停止
func (m *MotionControllerManager) Stop(id string, axis types.AxisName) error {
	m.RLock()
	ctrl, ok := m.instances[id]
	m.RUnlock()
	if !ok {
		return fmt.Errorf("motion controller not connected: %s", id)
	}
	return ctrl.Stop(axis)
}

// StopAll 停止所有轴
func (m *MotionControllerManager) StopAll(id string) error {
	m.RLock()
	ctrl, ok := m.instances[id]
	m.RUnlock()
	if !ok {
		return fmt.Errorf("motion controller not connected: %s", id)
	}
	return ctrl.StopAll()
}

// EmergencyStop 急停
func (m *MotionControllerManager) EmergencyStop(id string) error {
	m.RLock()
	ctrl, ok := m.instances[id]
	m.RUnlock()
	if !ok {
		return fmt.Errorf("motion controller not connected: %s", id)
	}
	return ctrl.EmergencyStop()
}

// DefinePosition 置位
func (m *MotionControllerManager) DefinePosition(id string, axis types.AxisName, position float64) error {
	m.RLock()
	ctrl, ok := m.instances[id]
	m.RUnlock()
	if !ok {
		return fmt.Errorf("motion controller not connected: %s", id)
	}
	return ctrl.DefinePosition(axis, position)
}

// GetStatusAll 获取所有控制器状态（实时查询已连接控制器）
func (m *MotionControllerManager) GetStatusAll() []types.MotionControllerStatus {
	return m.buildStatusAll(true)
}

// GetCachedStatusAll 获取所有控制器的缓存状态（无实时查询）
func (m *MotionControllerManager) GetCachedStatusAll() []types.MotionControllerStatus {
	return m.buildStatusAll(false)
}

// buildStatusAll 构建所有控制器状态（includeLive=true 时实时查询已连接控制器）
func (m *MotionControllerManager) buildStatusAll(includeLive bool) []types.MotionControllerStatus {
	m.RLock()
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
	m.RUnlock()

	statuses := make([]types.MotionControllerStatus, 0, len(profiles))
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
			if includeLive {
				if axes, err := ctrl.GetAllAxisStatus(); err == nil {
					status.Axes = axes
				} else if axes, ok := cachedStatuses[id]; ok {
					status.Axes = axes
				}
			} else if axes, ok := cachedStatuses[id]; ok {
				status.Axes = axes
			}
		} else if rs, ok := runtimeStatus[id]; ok {
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
	// 按 ID 稳定排序：避免遍历 map 导致返回顺序随机，
	// 否则前端 v-for 会反复重排 DOM 造成列表上下跳动，
	// 同时确保 broadcastStatus 的 JSON 变化检测不会因顺序不同而误判。
	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].ID < statuses[j].ID
	})
	return statuses
}

// StartPolling 启动状态轮询（CompareAndSwap 守护单实例）
func (m *MotionControllerManager) StartPolling() {
	if !m.pollRunning.CompareAndSwap(false, true) {
		return
	}
	defer m.pollRunning.Store(false)

	ctx, cancel := context.WithCancel(context.Background())
	m.pollMu.Lock()
	m.pollCancelFn = cancel
	m.pollMu.Unlock()
	defer cancel()

	ticker := time.NewTicker(time.Duration(types.MotionPollIntervalMs) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.pollStatus()
		}
	}
}

// StopPolling 停止轮询（取消当前 ctx 触发 StartPolling 退出）
func (m *MotionControllerManager) StopPolling() {
	m.pollMu.Lock()
	cancel := m.pollCancelFn
	m.pollCancelFn = nil
	m.pollMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// pollStatus 轮询所有控制器状态
func (m *MotionControllerManager) pollStatus() {
	m.RLock()
	// 在读锁下收集需要轮询的控制器
	toPoll := make(map[string]MotionController)
	for id, ctrl := range m.instances {
		if ctrl.IsConnected() {
			toPoll[id] = ctrl
		}
	}
	m.RUnlock()

	// 在锁外执行轮询（可能耗时）
	results := make(map[string][]types.AxisStatus)
	for id, ctrl := range toPoll {
		if axes, err := ctrl.GetAllAxisStatus(); err == nil {
			results[id] = axes
		}
	}

	// 用写锁更新状态
	m.Lock()
	for id, axes := range results {
		m.statuses[id] = axes
	}
	m.Unlock()
}

// Init 初始化（从配置文件加载控制器，若无则创建默认模拟控制器）
func (m *MotionControllerManager) Init() {
	loaded := false
	if m.configStore != nil {
		profiles := m.configStore.Get()
		if len(profiles) > 0 {
			migrated := 0
			for i := range profiles {
				p := &profiles[i]
				if newType, changed := types.MigrateMotionControllerType(p.Type); changed {
					slog.Info("migrate legacy motion controller type", "id", p.ID, "old", p.Type, "new", newType)
					p.Type = newType
					migrated++
				}
			}
			m.Lock()
			for _, p := range profiles {
				m.profiles[p.ID] = p
			}
			m.Unlock()
			loaded = true
			slog.Info("loaded motion controller profiles from config", "count", len(profiles), "migrated", migrated)
			if migrated > 0 {
				m.saveProfilesWithLog("motion")
			}
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
		m.Lock()
		m.profiles[defaultProfile.ID] = defaultProfile
		m.Unlock()
		m.saveProfilesWithLog("motion")
	}

	// 自动连接：SIMULATED 同步，B140 异步（避免 TCP 拨号超时阻塞应用启动）
	m.RLock()
	type profileEntry struct {
		id   string
		kind types.MotionControllerType
	}
	entries := make([]profileEntry, 0, len(m.profiles))
	for id, p := range m.profiles {
		entries = append(entries, profileEntry{id, p.Type})
	}
	m.RUnlock()

	for _, e := range entries {
		if e.kind == types.MotionTypeEA25MC04 {
			pid := e.id
			go func() {
				if err := m.Connect(pid); err != nil {
					slog.Warn("auto-connect EA25MC04 failed (manual connect available)", "id", pid, "err", err)
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
	m.RLock()
	ctrl, ok := m.instances[id]
	m.RUnlock()
	if !ok {
		return fmt.Errorf("motion controller not connected: %s", id)
	}
	return ctrl.SetAcceleration(axis, accel)
}

// SetDeceleration 设置减速度
func (m *MotionControllerManager) SetDeceleration(id string, axis types.AxisName, decel float64) error {
	m.RLock()
	ctrl, ok := m.instances[id]
	m.RUnlock()
	if !ok {
		return fmt.Errorf("motion controller not connected: %s", id)
	}
	return ctrl.SetDeceleration(axis, decel)
}

// IsMoving 查询是否有轴在运动
func (m *MotionControllerManager) IsMoving(id string) (bool, error) {
	m.RLock()
	ctrl, ok := m.instances[id]
	m.RUnlock()
	if !ok {
		return false, fmt.Errorf("motion controller not connected: %s", id)
	}
	return ctrl.IsMoving()
}

// IsAxisMoving 查询单轴是否在运动
func (m *MotionControllerManager) IsAxisMoving(id string, axis types.AxisName) (bool, error) {
	m.RLock()
	ctrl, ok := m.instances[id]
	m.RUnlock()
	if !ok {
		return false, fmt.Errorf("motion controller not connected: %s", id)
	}
	return ctrl.IsAxisMoving(axis)
}

// GetLimitStatus 查询轴限位状态
func (m *MotionControllerManager) GetLimitStatus(id string, axis types.AxisName) (types.LimitStatus, error) {
	m.RLock()
	ctrl, ok := m.instances[id]
	m.RUnlock()
	if !ok {
		return types.LimitStatus{}, fmt.Errorf("motion controller not connected: %s", id)
	}
	return ctrl.GetLimitStatus(axis)
}

// WaitForMotionComplete 等待运动完成
func (m *MotionControllerManager) WaitForMotionComplete(id string, axis types.AxisName, timeoutMs int) error {
	m.RLock()
	ctrl, ok := m.instances[id]
	m.RUnlock()
	if !ok {
		return fmt.Errorf("motion controller not connected: %s", id)
	}
	return ctrl.WaitForMotionComplete(axis, timeoutMs)
}

// GetAxisPosition 获取指定控制器指定轴的当前位置（实时查询控制器）
// 用于五孔测试开始前保存初始位置、测试结束后回到初始位置
func (m *MotionControllerManager) GetAxisPosition(id string, axis types.AxisName) (float64, error) {
	m.RLock()
	ctrl, ok := m.instances[id]
	m.RUnlock()
	if !ok {
		return 0, fmt.Errorf("motion controller not connected: %s", id)
	}
	status, err := ctrl.GetAxisStatus(axis)
	if err != nil {
		return 0, err
	}
	return status.Position, nil
}

// MotorOff 关闭电机
func (m *MotionControllerManager) MotorOff(id string) error {
	m.RLock()
	ctrl, ok := m.instances[id]
	m.RUnlock()
	if !ok {
		return fmt.Errorf("motion controller not connected: %s", id)
	}
	return ctrl.MotorOff()
}

// SetAxisDirection 设置轴方向
func (m *MotionControllerManager) SetAxisDirection(id string, axis types.AxisName, reverse bool) error {
	m.RLock()
	ctrl, ok := m.instances[id]
	m.RUnlock()
	if !ok {
		return fmt.Errorf("motion controller not connected: %s", id)
	}
	return ctrl.SetAxisDirection(axis, reverse)
}

// UpdateProfile 更新控制器配置
func (m *MotionControllerManager) UpdateProfile(profile types.MotionControllerProfile) {
	m.Lock()
	m.profiles[profile.ID] = profile
	if ctrl, ok := m.instances[profile.ID]; ok {
		if updater, canUpdate := ctrl.(axisConfigUpdater); canUpdate {
			updater.UpdateAxes(profile.Axes)
		}
	}
	m.Unlock()
	m.saveProfilesWithLog("motion")
}

// RemoveProfile 删除控制器配置（同时断开连接并清理运行时状态）
func (m *MotionControllerManager) RemoveProfile(id string) {
	m.Lock()
	if ctrl, ok := m.instances[id]; ok {
		ctrl.Disconnect()
		delete(m.instances, id)
	}
	delete(m.profiles, id)
	delete(m.statuses, id)
	delete(m.runtimeStatus, id)
	m.Unlock()
	m.saveProfilesWithLog("motion")
	m.emitStatusChange()
}
