package manager

import (
	"context"
	"sync"
	"time"

	"yx-daq/internal/logger"
	"yx-daq/internal/types"
)

// AcquisitionHub 数据采集中枢
type AcquisitionHub struct {
	mu          sync.RWMutex
	latestData  map[string]types.DataPayload
	frameSeq    map[string]uint64 // 每设备帧序号（单调递增，只增不清）
	publishHz   int
	onSnapshot  func(snapshots []types.DataPayload)
	subscribers map[uint64]chan struct{}
	nextSubID   uint64
}

// NewAcquisitionHub 创建数据采集中枢
func NewAcquisitionHub() *AcquisitionHub {
	return &AcquisitionHub{
		latestData:  make(map[string]types.DataPayload),
		frameSeq:    make(map[string]uint64),
		publishHz:   types.SnapshotPublishHz,
		subscribers: make(map[uint64]chan struct{}),
	}
}

// OnData 数据接收回调
// 为每帧分配该设备的单调帧序号（见 types.DataPayload.FrameSeq：墙钟 Timestamp 在 Windows 上
// 约 1ms 一档，同一毫秒内到达的多帧无法区分），随后向所有订阅者发送一次"新帧到达"信号。
func (h *AcquisitionHub) OnData(data types.DataPayload) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.frameSeq[data.DeviceID]++
	data.FrameSeq = h.frameSeq[data.DeviceID]
	h.latestData[data.DeviceID] = data
	// 非阻塞投递：通道容量 1，信号允许合并（消费者收到信号后必须重新读取帧序号判断是否真有新帧），
	// 慢消费者不会阻塞采集链路（receiveLoop）。
	for _, ch := range h.subscribers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// Subscribe 订阅"设备新帧到达"通知。
// 每收到一帧设备数据（OnData）都会向返回的通道投递一次信号；通道容量为 1，
// 高频到达的信号会被合并，因此消费者不能假设"一个信号 = 一帧"，必须收到信号后
// 重新读取该设备的 FrameSeq 再判断。返回的 cancel 用于移除订阅（幂等，可在 defer 中调用）；
// 通道由本 Hub 持有，永远不会被关闭（可安全地在 nil/已取消订阅上永久阻塞等待）。
func (h *AcquisitionHub) Subscribe() (<-chan struct{}, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.nextSubID++
	id := h.nextSubID
	ch := make(chan struct{}, 1)
	h.subscribers[id] = ch

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			delete(h.subscribers, id)
		})
	}
	return ch, cancel
}

// GetLatestData 获取指定设备最新数据
func (h *AcquisitionHub) GetLatestData(deviceID string) (types.DataPayload, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	data, ok := h.latestData[deviceID]
	return data, ok
}

// GetLatestValue 获取指定设备/通道的最新值
func (h *AcquisitionHub) GetLatestValue(deviceID string, channelIndex int) (float64, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	data, ok := h.latestData[deviceID]
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

// GetSnapshot 获取所有设备最新快照
func (h *AcquisitionHub) GetSnapshot() []types.DataPayload {
	h.mu.RLock()
	defer h.mu.RUnlock()
	snapshots := make([]types.DataPayload, 0, len(h.latestData))
	for _, data := range h.latestData {
		snapshots = append(snapshots, data)
	}
	return snapshots
}

// SetPublishHz 设置发布频率
func (h *AcquisitionHub) SetPublishHz(hz int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if hz < 1 {
		hz = 1
	}
	if hz > 100 {
		hz = 100
	}
	h.publishHz = hz
}

// GetPublishHz 获取发布频率
func (h *AcquisitionHub) GetPublishHz() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.publishHz
}

// SetOnSnapshot 设置快照回调
// 必须加锁：StartPublishing goroutine 会并发读取 h.onSnapshot
func (h *AcquisitionHub) SetOnSnapshot(cb func(snapshots []types.DataPayload)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onSnapshot = cb
}

// getOnSnapshot 加锁读取 onSnapshot 回调（与 emitStatusChange 风格一致：
// 锁内取指针，锁外执行回调，避免回调内部再获取 h.mu 导致死锁）
func (h *AcquisitionHub) getOnSnapshot() func(snapshots []types.DataPayload) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.onSnapshot
}

// ClearDevice 清除指定设备的最新数据（停止采集时调用，避免继续发布旧数据）
// 有意不清零 frameSeq：采样端保存的新帧基线是帧序号，清零会让重启采集后的序号回退到
// 基线以下，导致长时间判不出新帧（误报"数据停滞"）。
func (h *AcquisitionHub) ClearDevice(deviceID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.latestData, deviceID)
}

// StartPublishing 启动定时发布
// ctx 取消时退出循环（与 MotionControllerManager.StartPolling 的 context 风格一致）
func (h *AcquisitionHub) StartPublishing(ctx context.Context) {
	defer logger.Recover("acquisition-publish")
	ticker := time.NewTicker(time.Duration(1000/h.GetPublishHz()) * time.Millisecond)
	defer ticker.Stop()

	lastHz := h.GetPublishHz()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 检测频率变化，动态更新ticker
			curHz := h.GetPublishHz()
			if curHz != lastHz {
				lastHz = curHz
				ticker.Reset(time.Duration(1000/curHz) * time.Millisecond)
			}
			if cb := h.getOnSnapshot(); cb != nil {
				cb(h.GetSnapshot())
			}
		}
	}
}
