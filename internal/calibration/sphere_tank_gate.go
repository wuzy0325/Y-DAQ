package calibration

import (
	"log"
	"math"
	"time"

	"yx-daq/internal/types"
)

// SphereTankGate 球罐稳定门控
type SphereTankGate struct {
	config      types.SphereTankGateConfig
	dataGetter  func(deviceID string, channelIndex int) (float64, bool)
}

// NewSphereTankGate 创建球罐门控
func NewSphereTankGate(config types.SphereTankGateConfig, dataGetter func(deviceID string, channelIndex int) (float64, bool)) *SphereTankGate {
	return &SphereTankGate{
		config:     config,
		dataGetter: dataGetter,
	}
}

// WaitForStable 等待球罐压力稳定
// 在驻留时间内持续监测，压力变化率低于阈值时判定稳定
// 返回: 是否稳定, 实际等待时间(ms)
func (g *SphereTankGate) WaitForStable(deviceID string, maxWaitMs int) (bool, int) {
	if !g.config.Enabled {
		return true, 0
	}

	if g.dataGetter == nil {
		return true, 0
	}

	startTime := time.Now()
	maxDuration := time.Duration(maxWaitMs) * time.Millisecond
	sampleInterval := 100 * time.Millisecond
	stableRequiredMs := g.config.StableTimeMs
	thresholdRate := g.config.ThresholdRate

	var prevValue float64
	var hasPrev bool
	stableStart := time.Time{}

	ticker := time.NewTicker(sampleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			elapsed := time.Since(startTime)
			if elapsed > maxDuration {
				log.Printf("SphereTankGate: timeout after %v", elapsed)
				return false, int(elapsed.Milliseconds())
			}

			// 读取当前压力值
			currentValue, ok := g.dataGetter(deviceID, g.config.ChannelIndex)
			if !ok {
				continue
			}

			if !hasPrev {
				prevValue = currentValue
				hasPrev = true
				continue
			}

			// 计算变化率 (单位/ms)
			rate := math.Abs(currentValue-prevValue) / sampleInterval.Seconds()
			prevValue = currentValue

			if rate <= thresholdRate {
				// 变化率低于阈值
				if stableStart.IsZero() {
					stableStart = time.Now()
				}
				stableDuration := time.Since(stableStart)
				if stableDuration >= time.Duration(stableRequiredMs)*time.Millisecond {
					log.Printf("SphereTankGate: stable after %v (rate=%.6f)", elapsed, rate)
					return true, int(elapsed.Milliseconds())
				}
			} else {
				// 变化率超过阈值，重置稳定计时
				stableStart = time.Time{}
			}
		}
	}
}

// IsStable 快速检查当前是否稳定（不等待）
func (g *SphereTankGate) IsStable(deviceID string, recentValues []float64) bool {
	if !g.config.Enabled || len(recentValues) < 2 {
		return true
	}

	// 检查最近几个采样点的变化率
	for i := 1; i < len(recentValues); i++ {
		rate := math.Abs(recentValues[i]-recentValues[i-1]) / 0.1 // 假设100ms间隔
		if rate > g.config.ThresholdRate {
			return false
		}
	}
	return true
}
