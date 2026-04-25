package manager

import (
	"sync"
	"time"

	"yx-daq/internal/types"
)

// AcquisitionHub 数据采集中枢
type AcquisitionHub struct {
	mu         sync.RWMutex
	latestData map[string]types.DataPayload
	publishHz  int
	snapshots  []types.DataPayload
	onSnapshot func(snapshots []types.DataPayload)
}

// NewAcquisitionHub 创建数据采集中枢
func NewAcquisitionHub() *AcquisitionHub {
	return &AcquisitionHub{
		latestData: make(map[string]types.DataPayload),
		publishHz:  types.SnapshotPublishHz,
	}
}

// OnData 数据接收回调
func (h *AcquisitionHub) OnData(data types.DataPayload) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.latestData[data.DeviceID] = data
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
func (h *AcquisitionHub) SetOnSnapshot(cb func(snapshots []types.DataPayload)) {
	h.onSnapshot = cb
}

// ClearDevice 清除指定设备的最新数据（停止采集时调用，避免继续发布旧数据）
func (h *AcquisitionHub) ClearDevice(deviceID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.latestData, deviceID)
}

// StartPublishing 启动定时发布
func (h *AcquisitionHub) StartPublishing(cancel <-chan struct{}) {
	ticker := time.NewTicker(time.Duration(1000/h.GetPublishHz()) * time.Millisecond)
	defer ticker.Stop()

	lastHz := h.GetPublishHz()
	for {
		select {
		case <-cancel:
			return
		case <-ticker.C:
			// 检测频率变化，动态更新ticker
			curHz := h.GetPublishHz()
			if curHz != lastHz {
				lastHz = curHz
				ticker.Reset(time.Duration(1000/curHz) * time.Millisecond)
			}
			if h.onSnapshot != nil {
				h.onSnapshot(h.GetSnapshot())
			}
		}
	}
}
