package manager

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"yx-daq/internal/types"
)

// mockDeviceDriver implements DeviceDriver for testing
type mockDeviceDriver struct {
	mu         sync.Mutex
	connected  atomic.Bool
	acquiring  atomic.Bool
	channels   []types.ChannelConfig
	onData     types.DataCallback
	unit       string
	connectErr error
}

func (m *mockDeviceDriver) Connect() error {
	if m.connectErr != nil {
		return m.connectErr
	}
	m.connected.Store(true)
	return nil
}

func (m *mockDeviceDriver) Disconnect() {
	m.connected.Store(false)
	m.acquiring.Store(false)
}

func (m *mockDeviceDriver) IsConnected() bool    { return m.connected.Load() }
func (m *mockDeviceDriver) IsAcquiring() bool     { return m.acquiring.Load() }
func (m *mockDeviceDriver) SetDataCallback(cb types.DataCallback) { m.onData = cb }
func (m *mockDeviceDriver) UpdateChannels(ch []types.ChannelConfig) { m.channels = ch }
func (m *mockDeviceDriver) GetChannels() []types.ChannelConfig { return m.channels }

func (m *mockDeviceDriver) StartAcquisition(_ int) error {
	if !m.connected.Load() {
		return errNotConnected
	}
	m.acquiring.Store(true)
	return nil
}

func (m *mockDeviceDriver) StopAcquisition() error {
	m.acquiring.Store(false)
	return nil
}

func (m *mockDeviceDriver) SetUnit(unit string) error {
	m.unit = unit
	return nil
}

var errNotConnected = errors.New("device not connected")

func newTestManager() *DeviceManager {
	return &DeviceManager{
		profiles:   make(map[string]types.DeviceProfile),
		instances:  make(map[string]DeviceDriver),
		latestData: make(map[string]types.DataPayload),
	}
}

func newSimProfile(id string) types.DeviceProfile {
	return types.DeviceProfile{
		ID:       id,
		Name:     "测试设备",
		Type:     types.DeviceTypeSimulated,
		Host:     "127.0.0.1",
		Port:     9000,
		StreamID: 1,
		Channels: []types.ChannelConfig{
			{Index: 0, Name: "CH1", Enabled: true},
		},
	}
}

func injectMock(m *DeviceManager, id string, drv *mockDeviceDriver) {
	m.mu.Lock()
	m.instances[id] = drv
	m.mu.Unlock()
}

// ==================== Profile CRUD ====================

func TestDeviceManager_AddProfile(t *testing.T) {
	m := newTestManager()
	p := newSimProfile("dev-1")
	m.AddProfile(p)

	profiles := m.GetProfiles()
	if len(profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(profiles))
	}
	if profiles[0].ID != "dev-1" {
		t.Errorf("expected profile ID dev-1, got %s", profiles[0].ID)
	}
}

func TestDeviceManager_RemoveProfile_DisconnectsFirst(t *testing.T) {
	m := newTestManager()
	p := newSimProfile("dev-1")
	m.AddProfile(p)

	mock := &mockDeviceDriver{}
	mock.connected.Store(true)
	injectMock(m, "dev-1", mock)

	m.RemoveProfile("dev-1")

	if mock.IsConnected() {
		t.Error("expected driver to be disconnected after RemoveProfile")
	}
	if len(m.GetProfiles()) != 0 {
		t.Error("expected profile to be removed")
	}
}

func TestDeviceManager_GetProfileByID_NotFound(t *testing.T) {
	m := newTestManager()
	if p := m.GetProfileByID("nonexistent"); p != nil {
		t.Error("expected nil for nonexistent profile")
	}
}

func TestDeviceManager_UpdateProfile_SyncsChannelsToConnectedDriver(t *testing.T) {
	m := newTestManager()
	p := newSimProfile("dev-1")
	m.AddProfile(p)

	mock := &mockDeviceDriver{}
	mock.connected.Store(true)
	injectMock(m, "dev-1", mock)

	newChannels := []types.ChannelConfig{
		{Index: 0, Name: "Updated", Enabled: true},
	}
	p.Channels = newChannels
	m.UpdateProfile(p)

	if len(mock.channels) != 1 || mock.channels[0].Name != "Updated" {
		t.Errorf("expected driver channels updated, got %v", mock.channels)
	}
}

// ==================== Connection lifecycle ====================

func TestDeviceManager_Connect_ProfileNotFound(t *testing.T) {
	m := newTestManager()
	err := m.Connect("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent profile")
	}
}

func TestDeviceManager_Disconnect_Idempotent(t *testing.T) {
	m := newTestManager()
	m.Disconnect("nonexistent") // should not panic
}

func TestDeviceManager_Connect_SyncsDriverChannelsToProfile(t *testing.T) {
	m := newTestManager()
	p := newSimProfile("dev-1")
	p.Type = types.DeviceTypeSimulated
	p.Channels = []types.ChannelConfig{
		{Index: 0, Name: "CH1", Enabled: true, Unit: "kPa"},
		{Index: 1, Name: "CH2", Enabled: true, Unit: "kPa"},
		{Index: 2, Name: "大气压", Enabled: true, Unit: "Pa"},
		{Index: 3, Name: "大气温度", Enabled: true, Unit: "°C"},
	}
	m.AddProfile(p)

	if err := m.Connect("dev-1"); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	profile := m.GetProfileByID("dev-1")
	if profile == nil {
		t.Fatal("expected profile after connect")
	}
	if len(profile.Channels) != len(p.Channels) {
		t.Fatalf("expected %d channels, got %d", len(p.Channels), len(profile.Channels))
	}
	if profile.Channels[0].Unit != "kPa" {
		t.Errorf("expected connected driver channel unit kPa, got %s", profile.Channels[0].Unit)
	}
}

// ==================== Acquisition ====================

func TestDeviceManager_StartAcquisition_NotConnected(t *testing.T) {
	m := newTestManager()
	err := m.StartAcquisition("nonexistent", 50)
	if err == nil {
		t.Fatal("expected error for disconnected device")
	}
}

func TestDeviceManager_StartAndStopAcquisition(t *testing.T) {
	m := newTestManager()
	m.AddProfile(newSimProfile("dev-1"))

	mock := &mockDeviceDriver{}
	mock.connected.Store(true)
	injectMock(m, "dev-1", mock)

	if err := m.StartAcquisition("dev-1", 50); err != nil {
		t.Fatalf("StartAcquisition failed: %v", err)
	}
	if !mock.IsAcquiring() {
		t.Error("expected driver to be acquiring")
	}
	if !m.IsAcquiring("dev-1") {
		t.Error("expected manager to report acquiring")
	}

	if err := m.StopAcquisition("dev-1"); err != nil {
		t.Fatalf("StopAcquisition failed: %v", err)
	}
	if mock.IsAcquiring() {
		t.Error("expected driver to stop acquiring")
	}
}

// ==================== Status ====================

func TestDeviceManager_GetStatusAll(t *testing.T) {
	m := newTestManager()
	m.AddProfile(newSimProfile("dev-1"))
	m.AddProfile(newSimProfile("dev-2"))

	mock := &mockDeviceDriver{}
	mock.connected.Store(true)
	mock.acquiring.Store(true)
	injectMock(m, "dev-1", mock)

	statuses := m.GetStatusAll()
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}

	found := map[string]types.DeviceStatus{}
	for _, s := range statuses {
		found[s.ID] = s
	}

	if found["dev-1"].Status != types.StatusConnected {
		t.Errorf("expected dev-1 connected, got %s", found["dev-1"].Status)
	}
	if !found["dev-1"].Acquiring {
		t.Error("expected dev-1 acquiring")
	}
	if found["dev-2"].Status != types.StatusDisconnected {
		t.Errorf("expected dev-2 disconnected, got %s", found["dev-2"].Status)
	}
}

// ==================== Data flow ====================

func TestDeviceManager_GetLatestData(t *testing.T) {
	m := newTestManager()

	var capturedPayload types.DataPayload
	m.SetDataSink(func(p types.DataPayload) {
		capturedPayload = p
	})

	m.AddProfile(newSimProfile("dev-1"))
	mock := &mockDeviceDriver{}
	mock.connected.Store(true)
	injectMock(m, "dev-1", mock)

	// Wire up the callback manually (normally done in Connect)
	mock.SetDataCallback(func(payload types.DataPayload) {
		payload.DeviceID = "dev-1"
		m.mu.Lock()
		m.latestData["dev-1"] = payload
		m.mu.Unlock()
		if m.dataSink != nil {
			m.dataSink(payload)
		}
	})

	testPayload := types.DataPayload{
		DeviceID:       "dev-1",
		Timestamp:      12345,
		Channels:       []float64{1.0, 2.0},
		ChannelIndices: []int{0, 1},
	}
	mock.onData(testPayload)

	data, ok := m.GetLatestData("dev-1")
	if !ok {
		t.Fatal("expected latest data")
	}
	if data.DeviceID != "dev-1" {
		t.Errorf("expected device ID dev-1, got %s", data.DeviceID)
	}
	if capturedPayload.DeviceID != "dev-1" {
		t.Error("expected dataSink to receive payload")
	}
}

func TestDeviceManager_GetChannelValue(t *testing.T) {
	m := newTestManager()
	m.AddProfile(newSimProfile("dev-1"))
	mock := &mockDeviceDriver{}
	mock.connected.Store(true)
	injectMock(m, "dev-1", mock)

	// Wire up the callback manually
	mock.SetDataCallback(func(payload types.DataPayload) {
		payload.DeviceID = "dev-1"
		m.mu.Lock()
		m.latestData["dev-1"] = payload
		m.mu.Unlock()
	})

	mock.onData(types.DataPayload{
		DeviceID:       "dev-1",
		Channels:       []float64{10.5, 20.3},
		ChannelIndices: []int{0, 5},
	})

	val, ok := m.GetChannelValue("dev-1", 5)
	if !ok {
		t.Fatal("expected channel value")
	}
	if val != 20.3 {
		t.Errorf("expected 20.3, got %f", val)
	}

	_, ok = m.GetChannelValue("dev-1", 99)
	if ok {
		t.Error("expected no value for nonexistent channel")
	}
}

// ==================== SetUnit ====================

func TestDeviceManager_SetUnit(t *testing.T) {
	m := newTestManager()
	m.AddProfile(newSimProfile("dev-1"))

	mock := &mockDeviceDriver{}
	mock.connected.Store(true)
	injectMock(m, "dev-1", mock)

	if err := m.SetUnit("dev-1", "kPa"); err != nil {
		t.Fatalf("SetUnit failed: %v", err)
	}
	if mock.unit != "kPa" {
		t.Errorf("expected unit kPa, got %s", mock.unit)
	}
}

func TestDeviceManager_SetUnit_NotConnected(t *testing.T) {
	m := newTestManager()
	err := m.SetUnit("nonexistent", "kPa")
	if err == nil {
		t.Fatal("expected error for nonexistent device")
	}
}
