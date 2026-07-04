package driver

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
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
	valveMu       sync.Mutex         // 保护 valveState 的跨 goroutine 读写
	valveState    types.ValveState   // 模拟阀位，默认测量位
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
		valveState:    types.ValveStateMeasurement,
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
					var val float64
					switch {
					case i < s.pressureCount:
						if i < 8 {
							val = float64(rand.Intn(51) + 50)
						} else {
							val = float64(rand.Intn(301) + 500)
						}
					case i == s.pressureCount:
						val = basePressure + (rand.Float64()*2-1)*0.3
					default:
						val = 25.0 + (rand.Float64()*2-1)*0.5
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

// ReadValveState 返回模拟设备当前阀位（默认测量位）。
// 模拟设备不模拟设备拒绝，始终成功返回。
func (s *SimulatedDevice) ReadValveState() (types.ValveState, error) {
	s.valveMu.Lock()
	defer s.valveMu.Unlock()
	return s.valveState, nil
}

// SetValveState 设置模拟设备阀位。
// 仅接受 Calibration / Measurement，Unknown 拒绝（与真实驱动一致）。
func (s *SimulatedDevice) SetValveState(state types.ValveState) error {
	if state != types.ValveStateCalibration && state != types.ValveStateMeasurement {
		return fmt.Errorf("invalid valve state: %s", state)
	}
	s.valveMu.Lock()
	defer s.valveMu.Unlock()
	s.valveState = state
	return nil
}
