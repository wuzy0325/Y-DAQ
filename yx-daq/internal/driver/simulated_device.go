package driver

import (
	"math"
	"math/rand"
	"sync/atomic"
	"time"

	"yx-daq/internal/types"
)

// SimulatedDevice 模拟设备（用于无硬件调试）
type SimulatedDevice struct {
	acquiring     atomic.Bool
	onData        types.DataCallback
	channels      []types.ChannelConfig
	pressureCount int // 压力通道数
	stopCh        chan struct{}
}

// NewSimulatedDevice 创建模拟设备
func NewSimulatedDevice(channels []types.ChannelConfig) *SimulatedDevice {
	// 从通道配置推断压力通道数：总通道数-2（大气压+大气温度）
	pressureCount := len(channels) - 2
	if pressureCount < 1 {
		pressureCount = 16
	}
	return &SimulatedDevice{
		channels:      channels,
		pressureCount: pressureCount,
		stopCh:        make(chan struct{}),
	}
}

// SetDataCallback 设置数据回调
func (s *SimulatedDevice) SetDataCallback(cb types.DataCallback) {
	s.onData = cb
}

// Connect 模拟连接
func (s *SimulatedDevice) Connect() error {
	return nil
}

// Disconnect 模拟断开
func (s *SimulatedDevice) Disconnect() {
	s.acquiring.Store(false)
	// 非阻塞发送，避免在未采集时阻塞
	select {
	case s.stopCh <- struct{}{}:
	default:
	}
}

// IsConnected 始终连接
func (s *SimulatedDevice) IsConnected() bool {
	return true
}

// IsAcquiring 是否采集中
func (s *SimulatedDevice) IsAcquiring() bool {
	return s.acquiring.Load()
}

// StartAcquisition 启动模拟采集
func (s *SimulatedDevice) StartAcquisition(periodMs int) error {
	if s.acquiring.Load() {
		return nil
	}
	s.acquiring.Store(true)
	go s.simulateData(periodMs)
	return nil
}

// StopAcquisition 停止模拟采集
func (s *SimulatedDevice) StopAcquisition() error {
	if !s.acquiring.Load() {
		return nil
	}
	s.acquiring.Store(false)
	select {
	case s.stopCh <- struct{}{}:
	default:
	}
	return nil
}

// UpdateChannels 更新通道配置
func (s *SimulatedDevice) UpdateChannels(channels []types.ChannelConfig) {
	s.channels = channels
}

// GetChannels 返回当前通道配置副本。
func (s *SimulatedDevice) GetChannels() []types.ChannelConfig {
	channels := make([]types.ChannelConfig, len(s.channels))
	copy(channels, s.channels)
	return channels
}

// simulateData 模拟数据生成
func (s *SimulatedDevice) simulateData(periodMs int) {
	ticker := time.NewTicker(time.Duration(periodMs) * time.Millisecond)
	defer ticker.Stop()

	basePressure := 101325.0 // Pa 大气压基准
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			if !s.acquiring.Load() {
				return
			}

			values := []float64{}
			indices := []int{}
			for i, ch := range s.channels {
				if ch.Enabled {
					noise := rand.Float64()*2 - 1 // -1 ~ 1
					var val float64
					switch {
					case i < s.pressureCount:
						val = basePressure + float64(i)*5 + noise*2
					case i == s.pressureCount:
						val = basePressure + noise*0.3
					default:
						val = 25.0 + noise*0.5
					}
					values = append(values, math.Round(val*1000)/1000)
					indices = append(indices, i)
				}
			}

			units := make([]string, len(indices))
			for j, idx := range indices {
				if idx < len(s.channels) {
					units[j] = s.channels[idx].Unit
				}
			}
			payload := types.DataPayload{
				DeviceID:       "simulated",
				Timestamp:      time.Now().UnixMilli(),
				Channels:       values,
				ChannelIndices: indices,
				ChannelUnits:   units,
			}

			if s.onData != nil {
				s.onData(payload)
			}
		}
	}
}
