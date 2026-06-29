package manager

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"yx-daq/internal/driver"
	"yx-daq/internal/types"
)

// mockMotionController implements MotionController for testing
type mockMotionController struct {
	mu         sync.Mutex
	connected  bool
	position   map[types.AxisName]float64
	moveErr    error
	axisStatus []types.AxisStatus
}

func newMockMotionController() *mockMotionController {
	return &mockMotionController{
		position: make(map[types.AxisName]float64),
		axisStatus: []types.AxisStatus{
			{Name: types.AxisX},
			{Name: types.AxisY},
		},
	}
}

func (m *mockMotionController) Connect() error {
	m.connected = true
	return nil
}
func (m *mockMotionController) Disconnect() { m.connected = false }
func (m *mockMotionController) IsConnected() bool { return m.connected }
func (m *mockMotionController) MoveTo(axis types.AxisName, pos float64) error {
	m.mu.Lock()
	m.position[axis] = pos
	m.mu.Unlock()
	return m.moveErr
}
func (m *mockMotionController) MoveBy(axis types.AxisName, delta float64) error {
	m.mu.Lock()
	m.position[axis] += delta
	m.mu.Unlock()
	return m.moveErr
}
func (m *mockMotionController) Jog(_ types.AxisName, _ int, _ float64, _ float64) error { return nil }
func (m *mockMotionController) Home(_ types.AxisName) error                   { return nil }
func (m *mockMotionController) Stop(_ types.AxisName) error                   { return nil }
func (m *mockMotionController) StopAll() error                                { return nil }
func (m *mockMotionController) EmergencyStop() error                          { return nil }
func (m *mockMotionController) DefinePosition(axis types.AxisName, pos float64) error {
	m.mu.Lock()
	m.position[axis] = pos
	m.mu.Unlock()
	return nil
}
func (m *mockMotionController) GetAxisStatus(_ types.AxisName) (types.AxisStatus, error) {
	return types.AxisStatus{}, nil
}
func (m *mockMotionController) GetAllAxisStatus() ([]types.AxisStatus, error) {
	return m.axisStatus, nil
}
func (m *mockMotionController) SetSpeed(_ types.AxisName, _ float64) error { return nil }
func (m *mockMotionController) SetAcceleration(_ types.AxisName, _ float64) error {
	return nil
}
func (m *mockMotionController) SetDeceleration(_ types.AxisName, _ float64) error {
	return nil
}
func (m *mockMotionController) IsMoving() (bool, error) { return false, nil }
func (m *mockMotionController) IsAxisMoving(_ types.AxisName) (bool, error) {
	return false, nil
}
func (m *mockMotionController) GetLimitStatus(_ types.AxisName) (types.LimitStatus, error) {
	return types.LimitStatus{}, nil
}
func (m *mockMotionController) WaitForMotionComplete(_ types.AxisName, _ int) error {
	return nil
}
func (m *mockMotionController) MotorOff() error { return nil }
func (m *mockMotionController) SetAxisDirection(_ types.AxisName, _ bool) error {
	return nil
}

func newTestMotionManager() *MotionControllerManager {
	return NewMotionControllerManager()
}

func newMCProfile(id string) types.MotionControllerProfile {
	return types.MotionControllerProfile{
		ID:   id,
		Name: "测试控制器",
		Type: types.MotionTypeSimulated,
		Axes: []types.AxisConfig{
			{Name: types.AxisX, Enabled: true},
			{Name: types.AxisY, Enabled: true},
		},
	}
}

func injectMockMC(m *MotionControllerManager, id string, ctrl *mockMotionController) {
	m.Lock()
	m.instances[id] = ctrl
	m.Unlock()
}

// ==================== Profile CRUD ====================

func TestMotionManager_AddAndGetProfiles(t *testing.T) {
	m := newTestMotionManager()
	m.AddProfile(newMCProfile("mc-1"))
	m.AddProfile(newMCProfile("mc-2"))

	profiles := m.GetProfiles()
	if len(profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(profiles))
	}
}

func TestMotionManager_RemoveProfile(t *testing.T) {
	m := newTestMotionManager()
	m.AddProfile(newMCProfile("mc-1"))

	m.RemoveProfile("mc-1")

	if len(m.GetProfiles()) != 0 {
		t.Error("expected profile removed")
	}
}

func TestMotionManager_UpdateProfile(t *testing.T) {
	m := newTestMotionManager()
	m.AddProfile(newMCProfile("mc-1"))

	updated := newMCProfile("mc-1")
	updated.Name = "更新后"
	m.UpdateProfile(updated)

	profiles := m.GetProfiles()
	if len(profiles) != 1 || profiles[0].Name != "更新后" {
		t.Error("expected profile name updated")
	}
}

func TestMotionManager_UpdateProfileSyncsConnectedSimulatedInstance(t *testing.T) {
	m := newTestMotionManager()
	profile := newMCProfile("mc-1")
	profile.Axes = []types.AxisConfig{{Name: types.AxisU, Enabled: true, Kind: types.AxisKindRotary, StepAngleDeg: 1.8, MicroSteps: 16, GearRatio: 1}}
	m.AddProfile(profile)
	if err := m.Connect("mc-1"); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	updated := profile
	updated.Axes = []types.AxisConfig{{Name: types.AxisU, Enabled: true, Kind: types.AxisKindRotary, StepAngleDeg: 1.8, MicroSteps: 16, GearRatio: 10}}
	m.UpdateProfile(updated)

	m.RLock()
	ctrl := m.instances["mc-1"]
	m.RUnlock()
	sim, ok := ctrl.(*driver.SimulatedMotionController)
	if !ok {
		t.Fatalf("expected simulated controller, got %T", ctrl)
	}
	statuses, err := sim.GetAllAxisStatus()
	if err != nil {
		t.Fatalf("GetAllAxisStatus failed: %v", err)
	}
	if len(statuses) != 1 || statuses[0].Name != types.AxisU {
		t.Fatalf("expected updated U axis only, got %+v", statuses)
	}
}

func TestMotionManager_ConnectDisconnectsExistingInstance(t *testing.T) {
	m := newTestMotionManager()
	m.AddProfile(newMCProfile("mc-1"))

	old := newMockMotionController()
	old.connected = true
	injectMockMC(m, "mc-1", old)

	if err := m.Connect("mc-1"); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	if old.IsConnected() {
		t.Fatal("expected previous instance to be disconnected before replacement")
	}
}

// ==================== Operations on nonexistent ====================

func TestMotionManager_MoveTo_NotConnected(t *testing.T) {
	m := newTestMotionManager()
	err := m.MoveTo("nonexistent", types.AxisX, 10.0)
	if err == nil {
		t.Fatal("expected error for nonexistent controller")
	}
}

func TestMotionManager_EmergencyStop_NotConnected(t *testing.T) {
	m := newTestMotionManager()
	err := m.EmergencyStop("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent controller")
	}
}

// ==================== Movement ====================

func TestMotionManager_MoveTo(t *testing.T) {
	m := newTestMotionManager()
	m.AddProfile(newMCProfile("mc-1"))

	mock := newMockMotionController()
	mock.connected = true
	injectMockMC(m, "mc-1", mock)

	if err := m.MoveTo("mc-1", types.AxisX, 42.5); err != nil {
		t.Fatalf("MoveTo failed: %v", err)
	}

	mock.mu.Lock()
	pos := mock.position[types.AxisX]
	mock.mu.Unlock()
	if pos != 42.5 {
		t.Errorf("expected position 42.5, got %f", pos)
	}
}

func TestMotionManager_MoveBy(t *testing.T) {
	m := newTestMotionManager()
	m.AddProfile(newMCProfile("mc-1"))

	mock := newMockMotionController()
	mock.connected = true
	mock.position[types.AxisY] = 10.0
	injectMockMC(m, "mc-1", mock)

	if err := m.MoveBy("mc-1", types.AxisY, 5.0); err != nil {
		t.Fatalf("MoveBy failed: %v", err)
	}

	mock.mu.Lock()
	pos := mock.position[types.AxisY]
	mock.mu.Unlock()
	if pos != 15.0 {
		t.Errorf("expected position 15.0, got %f", pos)
	}
}

// ==================== IsConnected ====================

func TestMotionManager_IsConnected(t *testing.T) {
	m := newTestMotionManager()
	m.AddProfile(newMCProfile("mc-1"))

	if m.IsConnected("mc-1") {
		t.Error("expected not connected before injecting mock")
	}

	mock := newMockMotionController()
	mock.connected = true
	injectMockMC(m, "mc-1", mock)

	if !m.IsConnected("mc-1") {
		t.Error("expected connected after injecting mock")
	}
}

// ==================== GetStatusAll ====================

func TestMotionManager_GetStatusAll(t *testing.T) {
	m := newTestMotionManager()
	m.AddProfile(newMCProfile("mc-1"))

	mock := newMockMotionController()
	mock.connected = true
	injectMockMC(m, "mc-1", mock)

	statuses := m.GetStatusAll()
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].Status != types.StatusConnected {
		t.Errorf("expected connected, got %s", statuses[0].Status)
	}
}

// TestMotionManager_StartPolling_Concurrent_StartStop 验证 StartPolling/StopPolling 并发调用无竞态
// 覆盖 R3 回归：CompareAndSwap 守护单实例 + context.WithCancel 取消
// 设计要点：
//   - 多个 goroutine 同时启动 StartPolling，CAS 应保证只有一个进入轮询循环，其余立即返回
//   - CAS 成功的 goroutine 阻塞在 StartPolling 内部，需外部 StopPolling 唤醒
//   - 不在 for 循环中反复调用 StartPolling，避免 CAS 成功者退出后再次 CAS 成功导致 StopPolling 漏唤醒
func TestMotionManager_StartPolling_Concurrent_StartStop(t *testing.T) {
	m := newTestMotionManager()
	// 注入一个已连接的模拟控制器，让 pollStatus 有内容可轮询
	m.AddProfile(newMCProfile("mc-1"))
	mock := newMockMotionController()
	mock.connected = true
	injectMockMC(m, "mc-1", mock)

	// 8 个 goroutine 同时调用 StartPolling（仅一次），CAS 竞争
	// 期望：1 个阻塞在轮询循环，7 个 CAS 失败立即返回
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.StartPolling()
		}()
	}

	// 运行 500ms，让轮询循环执行若干次 pollStatus
	time.Sleep(500 * time.Millisecond)

	// StopPolling 唤醒阻塞在 StartPolling 内部的 goroutine
	m.StopPolling()

	// 等待所有 goroutine 退出（最长 2 秒，避免死锁卡死测试）
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("StartPolling goroutines did not exit within 2s")
	}
}

// TestMotionManager_PollStatus_During_Connect_Remove 验证轮询期间并发 Connect/RemoveProfile/GetStatusAll 无竞态
// 覆盖 R4 回归：emitStatusChange 在 Connect/RemoveProfile 期间不产生数据竞争
func TestMotionManager_PollStatus_During_Connect_Remove(t *testing.T) {
	m := newTestMotionManager()

	// 注册状态变更回调，让 emitStatusChange 路径被触发
	var cbCount atomic.Int64
	m.SetOnStatusChange(func(statuses []types.MotionControllerStatus) {
		cbCount.Add(1)
	})

	// 预添加几个 profile，让 pollStatus 有内容
	for i := 0; i < 3; i++ {
		m.AddProfile(newMCProfile("mc-" + string(rune('A'+i))))
	}

	// 启动轮询（在 goroutine 中运行，测试结束时 StopPolling）
	go m.StartPolling()
	defer m.StopPolling()

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// 并发 Connect/Disconnect（模拟控制器，无 TCP 拨号）
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			id := "mc-" + string(rune('A'+idx%3))
			for {
				select {
				case <-stop:
					return
				default:
				}
				_ = m.Connect(id)
				m.Disconnect(id)
			}
		}(i)
	}

	// 并发 GetStatusAll（读取 profiles/instances/runtimeStatus）
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				_ = m.GetStatusAll()
				_ = m.GetCachedStatusAll()
			}
		}()
	}

	// 并发 AddProfile/RemoveProfile（修改 profiles map）
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				id := "dyn-" + string(rune('A'+idx))
				m.AddProfile(newMCProfile(id))
				m.RemoveProfile(id)
			}
		}(i)
	}

	time.Sleep(500 * time.Millisecond)
	close(stop)
	wg.Wait()
}
