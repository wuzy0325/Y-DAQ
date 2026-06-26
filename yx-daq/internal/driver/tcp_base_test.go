package driver

import (
	"testing"

	"yx-daq/internal/types"
)

func TestTCPDriverBase_SetDataCallback(t *testing.T) {
	base := NewTCPDriverBase("127.0.0.1", 9000, nil)

	if base.onData != nil {
		t.Fatal("initial onData should be nil")
	}

	called := false
	cb := func(payload types.DataPayload) {
		called = true
	}
	base.SetDataCallback(cb)

	if base.onData == nil {
		t.Fatal("onData should not be nil after SetDataCallback")
	}

	// 验证回调可被调用
	base.onData(types.DataPayload{DeviceID: "test"})
	if !called {
		t.Fatal("callback was not called")
	}
}

func TestTCPDriverBase_UpdateAndGetChannels(t *testing.T) {
	channels := []types.ChannelConfig{
		{Index: 0, Name: "CH1", Enabled: true, Unit: "kPa"},
		{Index: 1, Name: "CH2", Enabled: false, Unit: "kPa"},
	}
	base := NewTCPDriverBase("127.0.0.1", 9000, channels)

	// GetChannels 应返回副本
	got := base.GetChannels()
	if len(got) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(got))
	}

	// 修改返回的副本不应影响原始数据
	got[0].Name = "MODIFIED"
	original := base.GetChannels()
	if original[0].Name == "MODIFIED" {
		t.Fatal("GetChannels should return a copy, not a reference")
	}

	// UpdateChannels 应更新通道
	newChannels := []types.ChannelConfig{
		{Index: 0, Name: "NEW1", Enabled: true, Unit: "Pa"},
		{Index: 1, Name: "NEW2", Enabled: true, Unit: "Pa"},
		{Index: 2, Name: "NEW3", Enabled: false, Unit: "Pa"},
	}
	base.UpdateChannels(newChannels)

	updated := base.GetChannels()
	if len(updated) != 3 {
		t.Fatalf("expected 3 channels after update, got %d", len(updated))
	}
	if updated[0].Name != "NEW1" {
		t.Fatalf("expected channel name NEW1, got %s", updated[0].Name)
	}
}

func TestTCPDriverBase_ConnectedState(t *testing.T) {
	base := NewTCPDriverBase("127.0.0.1", 9000, nil)

	// 初始状态：未连接、未采集
	if base.IsConnected() {
		t.Fatal("should not be connected initially")
	}
	if base.IsAcquiring() {
		t.Fatal("should not be acquiring initially")
	}

	// connected 和 acquiring 应为 false
	if base.connected.Load() {
		t.Fatal("connected flag should be false initially")
	}
	if base.acquiring.Load() {
		t.Fatal("acquiring flag should be false initially")
	}
}
