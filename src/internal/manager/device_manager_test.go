package manager

import (
	"errors"
	"strings"
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
	atmEnabled atomic.Bool
	adAvgCount atomic.Int32
	adAvgRead  atomic.Int32 // ReadADAveraging 返回的设备实际值
	// adAvgReadErr 非空时模拟读取 AD 平均失败
	adAvgReadErr error
	connectErr   error
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

func (m *mockDeviceDriver) IsConnected() bool                       { return m.connected.Load() }
func (m *mockDeviceDriver) IsAcquiring() bool                       { return m.acquiring.Load() }
func (m *mockDeviceDriver) SetDataCallback(cb types.DataCallback)   { m.onData = cb }
func (m *mockDeviceDriver) UpdateChannels(ch []types.ChannelConfig) { m.channels = ch }
func (m *mockDeviceDriver) GetChannels() []types.ChannelConfig      { return m.channels }

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

func (m *mockDeviceDriver) SetAtmEnabled(enabled bool) {
	m.atmEnabled.Store(enabled)
}

func (m *mockDeviceDriver) SetADAveraging(count int) error {
	m.adAvgCount.Store(int32(count))
	return nil
}

// ReadADAveraging 模拟连接后读取设备实际 AD 平均次数（adAvgReadErr 非空时模拟读取失败）
func (m *mockDeviceDriver) ReadADAveraging() (int, error) {
	if m.adAvgReadErr != nil {
		return 0, m.adAvgReadErr
	}
	return int(m.adAvgRead.Load()), nil
}

var errNotConnected = errors.New("device not connected")

func newTestManager() *DeviceManager {
	return NewDeviceManager()
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

func newAtmCapableProfile(id string, deviceType types.DeviceType) types.DeviceProfile {
	total := deviceType.TotalChannelCount()
	channels := make([]types.ChannelConfig, total)
	for i := range channels {
		channels[i] = types.ChannelConfig{Index: i, Name: "CH", Enabled: true}
	}
	return types.DeviceProfile{
		ID: id, Name: "压力设备", Type: deviceType,
		Host: "127.0.0.1", Port: 9000, StreamID: 1,
		AtmEnabled: true, Channels: channels,
	}
}

func injectMock(m *DeviceManager, id string, drv *mockDeviceDriver) {
	m.Lock()
	m.instances[id] = drv
	m.Unlock()
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

// ==================== 大气压/温度使能 ====================

func TestDeviceManager_UpdateProfile_SyncsAtmEnabledToConnectedDriver(t *testing.T) {
	m := newTestManager()
	p := newSimProfile("dev-1")
	p.AtmEnabled = false
	m.AddProfile(p)

	mock := &mockDeviceDriver{}
	mock.connected.Store(true)
	mock.atmEnabled.Store(false)
	injectMock(m, "dev-1", mock)

	p.AtmEnabled = true
	if err := m.UpdateProfile(p); err != nil {
		t.Fatalf("UpdateProfile failed: %v", err)
	}
	if !mock.atmEnabled.Load() {
		t.Error("expected driver atmEnabled updated to true")
	}
}

func TestDeviceManager_SetAtmEnabled(t *testing.T) {
	m := newTestManager()
	p := newSimProfile("dev-1")
	p.Type = types.DeviceTypeEA2516A
	p.AtmEnabled = true
	m.AddProfile(p)

	mock := &mockDeviceDriver{}
	mock.connected.Store(true)
	mock.atmEnabled.Store(true)
	injectMock(m, "dev-1", mock)

	if err := m.SetAtmEnabled("dev-1", false); err != nil {
		t.Fatalf("SetAtmEnabled failed: %v", err)
	}
	got := m.GetProfileByID("dev-1")
	if got == nil || got.AtmEnabled {
		t.Error("expected profile AtmEnabled=false")
	}
	if mock.atmEnabled.Load() {
		t.Error("expected driver atmEnabled=false")
	}
}

func TestDeviceManager_SetAtmEnabled_SyncsChannelFlags(t *testing.T) {
	m := newTestManager()
	p := newAtmCapableProfile("dev-1", types.DeviceTypeEA2516A)
	m.AddProfile(p)

	mock := &mockDeviceDriver{}
	mock.connected.Store(true)
	injectMock(m, "dev-1", mock)

	if err := m.SetAtmEnabled("dev-1", false); err != nil {
		t.Fatalf("SetAtmEnabled(false) failed: %v", err)
	}
	got := m.GetProfileByID("dev-1")
	if got == nil || got.AtmEnabled {
		t.Fatal("expected profile AtmEnabled=false")
	}
	for idx := 0; idx < 16; idx++ {
		if !got.Channels[idx].Enabled {
			t.Errorf("pressure channel %d should stay enabled", idx)
		}
	}
	for _, idx := range []int{16, 17} {
		if got.Channels[idx].Enabled {
			t.Errorf("atm channel %d should be disabled when atm disabled", idx)
		}
	}

	if err := m.SetAtmEnabled("dev-1", true); err != nil {
		t.Fatalf("SetAtmEnabled(true) failed: %v", err)
	}
	got = m.GetProfileByID("dev-1")
	for _, idx := range []int{16, 17} {
		if !got.Channels[idx].Enabled {
			t.Errorf("atm channel %d should be re-enabled when atm enabled", idx)
		}
	}
}

func TestDeviceManager_UpdateProfile_SyncsAtmChannelFlags(t *testing.T) {
	m := newTestManager()
	p := newAtmCapableProfile("dev-1", types.DeviceTypeEA2508A)
	m.AddProfile(p)

	mock := &mockDeviceDriver{}
	mock.connected.Store(true)
	injectMock(m, "dev-1", mock)

	p.AtmEnabled = false
	if err := m.UpdateProfile(p); err != nil {
		t.Fatalf("UpdateProfile failed: %v", err)
	}
	got := m.GetProfileByID("dev-1")
	if got == nil {
		t.Fatal("expected profile")
	}
	for _, idx := range []int{8, 9} {
		if got.Channels[idx].Enabled {
			t.Errorf("atm channel %d should be disabled", idx)
		}
	}
	if !got.Channels[0].Enabled {
		t.Error("pressure channel 0 should stay enabled")
	}
}

func TestDeviceManager_AddProfile_SyncsAtmChannelFlags(t *testing.T) {
	m := newTestManager()
	p := newAtmCapableProfile("dev-1", types.DeviceTypeEA2508A)
	p.AtmEnabled = false
	m.AddProfile(p)

	got := m.GetProfileByID("dev-1")
	for _, idx := range []int{8, 9} {
		if got.Channels[idx].Enabled {
			t.Errorf("atm channel %d should be disabled on add", idx)
		}
	}
}

func TestDeviceManager_SetAtmEnabled_UnsupportedType(t *testing.T) {
	m := newTestManager()
	m.AddProfile(newSimProfile("dev-1")) // SIMULATED 不支持
	if err := m.SetAtmEnabled("dev-1", false); err == nil {
		t.Fatal("expected error for unsupported device type")
	}
}

func TestDeviceManager_SetAtmEnabled_AcquiringRejected(t *testing.T) {
	m := newTestManager()
	p := newSimProfile("dev-1")
	p.Type = types.DeviceTypeEA2508A
	m.AddProfile(p)

	mock := &mockDeviceDriver{}
	mock.connected.Store(true)
	mock.acquiring.Store(true)
	injectMock(m, "dev-1", mock)

	if err := m.SetAtmEnabled("dev-1", false); err == nil {
		t.Fatal("expected error while acquiring")
	}
}

// ==================== AD 平均 ====================

func TestDeviceManager_SetADAveraging(t *testing.T) {
	m := newTestManager()
	m.AddProfile(newAtmCapableProfile("dev-1", types.DeviceTypeEA2516A))

	mock := &mockDeviceDriver{}
	mock.connected.Store(true)
	injectMock(m, "dev-1", mock)

	if err := m.SetADAveraging("dev-1", true, 16); err != nil {
		t.Fatalf("SetADAveraging failed: %v", err)
	}
	got := m.GetProfileByID("dev-1")
	if got == nil || !got.ADAveragingEnabled || got.ADAveragingCount != 16 {
		t.Fatalf("profile not updated: %+v", got)
	}
	if mock.adAvgCount.Load() != 16 {
		t.Errorf("driver adAvgCount = %d, want 16", mock.adAvgCount.Load())
	}

	// 关闭 → 下发固定最低值 4，且持久化的次数与硬件一致
	if err := m.SetADAveraging("dev-1", false, 8); err != nil {
		t.Fatalf("SetADAveraging(off) failed: %v", err)
	}
	if mock.adAvgCount.Load() != types.ADAveragingOffCount {
		t.Errorf("driver adAvgCount = %d, want %d (off)", mock.adAvgCount.Load(), types.ADAveragingOffCount)
	}
	if got := m.GetProfileByID("dev-1"); got == nil || got.ADAveragingCount != types.ADAveragingOffCount {
		t.Errorf("profile count not normalized: %+v", got)
	}
}

// TestDeviceManager_SetADAveraging_InvalidCountRejected 开启时非法次数应在下发前拒绝
func TestDeviceManager_SetADAveraging_InvalidCountRejected(t *testing.T) {
	m := newTestManager()
	m.AddProfile(newAtmCapableProfile("dev-1", types.DeviceTypeEA2516A))

	mock := &mockDeviceDriver{}
	mock.connected.Store(true)
	injectMock(m, "dev-1", mock)

	if err := m.SetADAveraging("dev-1", true, 7); err == nil {
		t.Fatal("expected error for illegal count 7")
	}
	if mock.adAvgCount.Load() != 0 {
		t.Errorf("driver must not be written on invalid count, got %d", mock.adAvgCount.Load())
	}
}

func TestDeviceManager_SetADAveraging_UnsupportedType(t *testing.T) {
	m := newTestManager()
	m.AddProfile(newSimProfile("dev-1")) // SIMULATED 不支持
	if err := m.SetADAveraging("dev-1", true, 8); err == nil {
		t.Fatal("expected error for unsupported device type")
	}
}

func TestDeviceManager_SetADAveraging_AcquiringRejected(t *testing.T) {
	m := newTestManager()
	p := newSimProfile("dev-1")
	p.Type = types.DeviceTypeEA2516A
	m.AddProfile(p)

	mock := &mockDeviceDriver{}
	mock.connected.Store(true)
	mock.acquiring.Store(true)
	injectMock(m, "dev-1", mock)

	if err := m.SetADAveraging("dev-1", true, 8); err == nil {
		t.Fatal("expected error while acquiring")
	}
}

// ==================== Connection lifecycle ====================

// withMockFactory 临时把某设备类型的驱动工厂替换为给定 mock，测试结束自动还原
func withMockFactory(t *testing.T, deviceType types.DeviceType, drv DeviceDriver) {
	t.Helper()
	orig, has := driverFactories[deviceType]
	driverFactories[deviceType] = func(types.DeviceProfile) DeviceDriver { return drv }
	t.Cleanup(func() {
		if has {
			driverFactories[deviceType] = orig
			return
		}
		delete(driverFactories, deviceType)
	})
}

// TestDeviceManager_Connect_SyncsADAveragingFromDevice 连接后以设备实际值为准：
// 读到 4 → 开关关闭；读到其他值 → 开关开启并显示实际值；读取失败保留原配置。
func TestDeviceManager_Connect_SyncsADAveragingFromDevice(t *testing.T) {
	cases := []struct {
		name        string
		deviceRead  int32
		readErr     error
		wantEnabled bool
		wantCount   int
		wantNotify  bool
	}{
		{"读到4视为关闭", 4, nil, false, types.ADAveragingOffCount, true},
		{"读到8开启8", 8, nil, true, 8, true},
		{"读到16开启16", 16, nil, true, 16, true},
		{"读到32开启32", 32, nil, true, 32, true},
		{"读取失败保留原配置", 0, errNotConnected, true, 8, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestManager()
			p := newAtmCapableProfile("dev-1", types.DeviceTypeEA2516A)
			// 原配置故意与设备不一致（enabled=8），验证以设备为准
			p.ADAveragingEnabled = true
			p.ADAveragingCount = 8
			m.AddProfile(p)

			mock := &mockDeviceDriver{channels: p.Channels}
			mock.adAvgRead.Store(tc.deviceRead)
			mock.adAvgReadErr = tc.readErr
			withMockFactory(t, types.DeviceTypeEA2516A, mock)

			var notified []string
			m.SetOnProfileChange(func(id string) { notified = append(notified, id) })

			if err := m.Connect("dev-1"); err != nil {
				t.Fatalf("Connect failed: %v", err)
			}

			got := m.GetProfileByID("dev-1")
			if got == nil {
				t.Fatal("profile missing after connect")
			}
			if got.ADAveragingEnabled != tc.wantEnabled || got.ADAveragingCount != tc.wantCount {
				t.Errorf("profile AD = (%v, %d), want (%v, %d)",
					got.ADAveragingEnabled, got.ADAveragingCount, tc.wantEnabled, tc.wantCount)
			}
			if tc.wantNotify {
				if len(notified) != 1 || notified[0] != "dev-1" {
					t.Errorf("profile change notify = %v, want [dev-1]", notified)
				}
			} else if len(notified) != 0 {
				t.Errorf("read failure must not notify, got %v", notified)
			}
			// 连接时只读不写：设备侧次数不得被配置值覆盖
			if mock.adAvgCount.Load() != 0 {
				t.Errorf("driver must not be written on connect, got %d", mock.adAvgCount.Load())
			}
		})
	}
}

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
		m.Lock()
		m.latestData["dev-1"] = payload
		m.Unlock()
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
		m.Lock()
		m.latestData["dev-1"] = payload
		m.Unlock()
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

// 采集过程中的设备禁止修改配置（SetUnit 应被拦截）
func TestDeviceManager_SetUnit_Acquiring(t *testing.T) {
	m := newTestManager()
	m.AddProfile(newSimProfile("dev-1"))

	mock := &mockDeviceDriver{}
	mock.connected.Store(true)
	mock.acquiring.Store(true)
	injectMock(m, "dev-1", mock)

	err := m.SetUnit("dev-1", "kPa")
	if err == nil {
		t.Fatal("expected error when acquiring")
	}
	if !strings.Contains(err.Error(), "采集进行中") {
		t.Errorf("expected acquiring-protection error, got %v", err)
	}
	if mock.unit != "" {
		t.Errorf("expected unit unchanged while acquiring, got %s", mock.unit)
	}
}

// 采集过程中禁止更新设备配置（UpdateProfile 应被拦截，配置与驱动通道均不变）
func TestDeviceManager_UpdateProfile_Acquiring(t *testing.T) {
	m := newTestManager()
	m.AddProfile(newSimProfile("dev-1"))

	mock := &mockDeviceDriver{}
	mock.connected.Store(true)
	mock.acquiring.Store(true)
	injectMock(m, "dev-1", mock)

	updated := newSimProfile("dev-1")
	updated.Channels = []types.ChannelConfig{{Index: 0, Name: "CHANGED", Enabled: false}}
	err := m.UpdateProfile(updated)
	if err == nil {
		t.Fatal("expected error when acquiring")
	}
	if !strings.Contains(err.Error(), "采集进行中") {
		t.Errorf("expected acquiring-protection error, got %v", err)
	}
	if p := m.GetProfileByID("dev-1"); p != nil && p.Channels[0].Name == "CHANGED" {
		t.Error("expected profile unchanged while acquiring")
	}
	if len(mock.channels) != 0 {
		t.Errorf("expected driver channels not hot-updated, got %v", mock.channels)
	}
}

// 采集过程中禁止删除设备配置（RemoveProfile 应被拦截，设备保持连接）
func TestDeviceManager_RemoveProfile_Acquiring(t *testing.T) {
	m := newTestManager()
	m.AddProfile(newSimProfile("dev-1"))

	mock := &mockDeviceDriver{}
	mock.connected.Store(true)
	mock.acquiring.Store(true)
	injectMock(m, "dev-1", mock)

	err := m.RemoveProfile("dev-1")
	if err == nil {
		t.Fatal("expected error when acquiring")
	}
	if !strings.Contains(err.Error(), "采集进行中") {
		t.Errorf("expected acquiring-protection error, got %v", err)
	}
	if m.GetProfileByID("dev-1") == nil {
		t.Error("expected profile preserved while acquiring")
	}
	if !mock.IsConnected() {
		t.Error("expected device still connected after rejected removal")
	}
}

// 未连接设备（instances 无实例）编辑配置不得 panic，且应正常持久化新配置。
// 回归：UpdateProfile 曾把 nil 驱动传给 ensureConfigMutable，触发 nil 解引用崩溃。
func TestDeviceManager_UpdateProfile_NotConnected(t *testing.T) {
	m := newTestManager()
	m.AddProfile(newSimProfile("dev-1"))

	updated := newSimProfile("dev-1")
	updated.Name = "改名设备"
	updated.Host = "192.168.1.50"
	if err := m.UpdateProfile(updated); err != nil {
		t.Fatalf("unexpected error updating disconnected device: %v", err)
	}
	p := m.GetProfileByID("dev-1")
	if p == nil {
		t.Fatal("expected profile to exist")
	}
	if p.Name != "改名设备" || p.Host != "192.168.1.50" {
		t.Errorf("profile not updated: name=%q host=%q", p.Name, p.Host)
	}
}
