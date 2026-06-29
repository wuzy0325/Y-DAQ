package manager

import (
	"context"
	"sync"
	"testing"
	"time"

	"yx-daq/internal/types"
)

func TestAcquisitionHub_OnDataAndGetLatestData(t *testing.T) {
	h := NewAcquisitionHub()
	h.OnData(types.DataPayload{DeviceID: "d1", Channels: []float64{1.0, 2.0}})

	d, ok := h.GetLatestData("d1")
	if !ok {
		t.Fatal("expected d1 to be present")
	}
	if len(d.Channels) != 2 || d.Channels[0] != 1.0 || d.Channels[1] != 2.0 {
		t.Errorf("unexpected channels: %v", d.Channels)
	}

	if _, ok := h.GetLatestData("nonexistent"); ok {
		t.Error("nonexistent device should not be found")
	}
}

func TestAcquisitionHub_OnData_OverwritesLatest(t *testing.T) {
	h := NewAcquisitionHub()
	h.OnData(types.DataPayload{DeviceID: "d1", Channels: []float64{1.0}})
	h.OnData(types.DataPayload{DeviceID: "d1", Channels: []float64{2.0}})

	d, _ := h.GetLatestData("d1")
	if d.Channels[0] != 2.0 {
		t.Errorf("expected latest 2.0, got %v", d.Channels[0])
	}
}

func TestAcquisitionHub_GetLatestValue(t *testing.T) {
	h := NewAcquisitionHub()
	h.OnData(types.DataPayload{
		DeviceID:       "d1",
		Channels:       []float64{10.0, 20.0, 30.0},
		ChannelIndices: []int{5, 6, 7},
	})

	cases := []struct {
		name    string
		devID   string
		chIdx   int
		wantVal float64
		wantOK  bool
	}{
		{"existing ch5", "d1", 5, 10.0, true},
		{"existing ch7", "d1", 7, 30.0, true},
		{"missing ch 99", "d1", 99, 0, false},
		{"unknown device", "nope", 5, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			val, ok := h.GetLatestValue(tc.devID, tc.chIdx)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && val != tc.wantVal {
				t.Errorf("val = %v, want %v", val, tc.wantVal)
			}
		})
	}
}

func TestAcquisitionHub_GetSnapshot(t *testing.T) {
	h := NewAcquisitionHub()
	h.OnData(types.DataPayload{DeviceID: "d1"})
	h.OnData(types.DataPayload{DeviceID: "d2"})

	snap := h.GetSnapshot()
	if len(snap) != 2 {
		t.Errorf("expected 2 snapshots, got %d", len(snap))
	}

	// 空hub返回空切片（非nil）
	empty := NewAcquisitionHub().GetSnapshot()
	if len(empty) != 0 {
		t.Errorf("expected empty snapshot, got %d", len(empty))
	}
}

func TestAcquisitionHub_SetPublishHz_Clamps(t *testing.T) {
	h := NewAcquisitionHub()
	if h.GetPublishHz() != types.SnapshotPublishHz {
		t.Errorf("initial = %d, want %d", h.GetPublishHz(), types.SnapshotPublishHz)
	}

	h.SetPublishHz(0)
	if h.GetPublishHz() != 1 {
		t.Errorf("hz=0 should clamp to 1, got %d", h.GetPublishHz())
	}

	h.SetPublishHz(-10)
	if h.GetPublishHz() != 1 {
		t.Errorf("hz<0 should clamp to 1, got %d", h.GetPublishHz())
	}

	h.SetPublishHz(500)
	if h.GetPublishHz() != 100 {
		t.Errorf("hz>100 should clamp to 100, got %d", h.GetPublishHz())
	}

	h.SetPublishHz(50)
	if h.GetPublishHz() != 50 {
		t.Errorf("hz=50 should be 50, got %d", h.GetPublishHz())
	}
}

func TestAcquisitionHub_ClearDevice(t *testing.T) {
	h := NewAcquisitionHub()
	h.OnData(types.DataPayload{DeviceID: "d1"})
	h.OnData(types.DataPayload{DeviceID: "d2"})

	h.ClearDevice("d1")
	if _, ok := h.GetLatestData("d1"); ok {
		t.Error("d1 should be cleared")
	}
	if _, ok := h.GetLatestData("d2"); !ok {
		t.Error("d2 should still exist")
	}

	// ClearDevice 不存在的设备不应 panic
	h.ClearDevice("nonexistent")
}

func TestAcquisitionHub_SetOnSnapshot(t *testing.T) {
	h := NewAcquisitionHub()
	var got []types.DataPayload
	h.SetOnSnapshot(func(snapshots []types.DataPayload) {
		got = snapshots
	})

	h.OnData(types.DataPayload{DeviceID: "d1"})
	// 直接调用回调（不依赖 StartPublishing goroutine），通过加锁读取避免数据竞争
	if cb := h.getOnSnapshot(); cb != nil {
		cb(h.GetSnapshot())
	}
	if len(got) != 1 || got[0].DeviceID != "d1" {
		t.Errorf("snapshot callback got %v", got)
	}
}

// TestAcquisitionHub_Concurrent_OnData_And_GetSnapshot 验证并发写入与读取无数据竞争
// N 个 goroutine 并发 OnData + M 个 goroutine 并发 GetSnapshot/GetLatestValue，运行 500ms
func TestAcquisitionHub_Concurrent_OnData_And_GetSnapshot(t *testing.T) {
	h := NewAcquisitionHub()

	devices := []string{"d1", "d2", "d3", "d4"}
	stop := make(chan struct{})

	var wg sync.WaitGroup
	// N=8 写入者
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			devID := devices[idx%len(devices)]
			for {
				select {
				case <-stop:
					return
				default:
				}
				h.OnData(types.DataPayload{
					DeviceID:       devID,
					Channels:       []float64{float64(idx), float64(time.Now().UnixNano() % 1000)},
					ChannelIndices: []int{0, 1},
				})
			}
		}(i)
	}

	// M=8 读取者，混合 GetSnapshot / GetLatestValue / GetLatestData
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			devID := devices[idx%len(devices)]
			for {
				select {
				case <-stop:
					return
				default:
				}
				switch idx % 3 {
				case 0:
					_ = h.GetSnapshot()
				case 1:
					_, _ = h.GetLatestData(devID)
				case 2:
					_, _ = h.GetLatestValue(devID, 0)
				}
			}
		}(i)
	}

	time.Sleep(500 * time.Millisecond)
	close(stop)
	wg.Wait()
}

// TestAcquisitionHub_Concurrent_SetOnSnapshot_DuringPublish 验证发布期间并发切换回调无数据竞争
// 启动 StartPublishing，并发 SetOnSnapshot 切换回调，500ms 后 cancel 退出
func TestAcquisitionHub_Concurrent_SetOnSnapshot_DuringPublish(t *testing.T) {
	h := NewAcquisitionHub()
	h.SetPublishHz(50) // 50Hz 加快触发频率

	// 预填充数据，让 GetSnapshot 有内容
	h.OnData(types.DataPayload{DeviceID: "d1", Channels: []float64{1.0}})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go h.StartPublishing(ctx)

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// 4 个 goroutine 并发切换回调（含 nil 回调）
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
					h.SetOnSnapshot(func(snapshots []types.DataPayload) {
						// 回调内部不获取 h.mu，避免死锁；仅消费切片
						_ = len(snapshots)
					})
				} else {
					h.SetOnSnapshot(nil)
				}
			}
		}(i)
	}

	time.Sleep(500 * time.Millisecond)
	close(stop)
	wg.Wait()
	cancel()
}
