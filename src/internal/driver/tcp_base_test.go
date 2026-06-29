package driver

import (
	"sync"
	"testing"
	"time"

	"yx-daq/internal/types"
)

func TestTCPDriverBase_SetDataCallback(t *testing.T) {
	base := NewTCPDriverBase("127.0.0.1", 9000, nil)

	if base.getOnData() != nil {
		t.Fatal("initial onData should be nil")
	}

	called := false
	cb := func(payload types.DataPayload) {
		called = true
	}
	base.SetDataCallback(cb)

	if base.getOnData() == nil {
		t.Fatal("onData should not be nil after SetDataCallback")
	}

	// 验证回调可被调用（通过 EmitData 走加锁路径）
	base.EmitData(types.DataPayload{DeviceID: "test"})
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

// TestTCPDriverBase_Concurrent_SetDataCallback_And_EmitData 验证并发设置回调与发射数据无数据竞争
// N 个 goroutine 并发 SetDataCallback（含 nil）+ M 个 goroutine 并发 EmitData，运行 500ms
// 覆盖 R2 回归：receiveLoop goroutine 调用 EmitData 读 b.onData 必须与 SetDataCallback 写互斥
func TestTCPDriverBase_Concurrent_SetDataCallback_And_EmitData(t *testing.T) {
	base := NewTCPDriverBase("127.0.0.1", 9000, nil)

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// 4 个 goroutine 并发切换回调（含 nil 回调，模拟重连/配置变更场景）
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if idx%2 == 0 {
					base.SetDataCallback(func(payload types.DataPayload) {
						// 回调内部仅消费 payload，不访问 base 字段，避免与 getOnData 锁顺序冲突
						_ = payload.DeviceID
					})
				} else {
					base.SetDataCallback(nil)
				}
			}
		}(i)
	}

	// 8 个 goroutine 并发 EmitData，模拟 receiveLoop 高频调用
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				base.EmitData(types.DataPayload{
					DeviceID: "dev",
					Channels: []float64{float64(idx)},
				})
			}
		}(i)
	}

	time.Sleep(500 * time.Millisecond)
	close(stop)
	wg.Wait()
}
