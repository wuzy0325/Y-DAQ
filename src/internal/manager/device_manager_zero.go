package manager

import (
	"fmt"
	"log/slog"
	"time"

	"yx-daq/internal/types"
)

// ZeroCalibrateSamples 零位校准固定采样帧数
const ZeroCalibrateSamples = 10

// zeroCalibTimeout 零位校准采样超时（10 帧 × 50ms ≈ 500ms，留 5 秒余量应对慢设备）
const zeroCalibTimeout = 5 * time.Second

// applyZeroOffset 对数据帧应用零位偏移修正（原地修改 payload.Channels）
// 在数据回调中、存储 latestData 之前调用，确保所有消费者获得修正后的值。
// 偏移逻辑：corrected = raw - offset_in_current_unit
func (m *DeviceManager) applyZeroOffset(deviceID string, payload *types.DataPayload) {
	m.RLock()
	profile, ok := m.profiles[deviceID]
	m.RUnlock()
	if !ok {
		return
	}
	if !profile.Type.IsDAQDevice() || profile.Type.IsTemperatureDevice() {
		return
	}
	hasOffset := false
	for _, ch := range profile.Channels {
		if ch.ZeroOffset != 0 && ch.ZeroOffsetUnit != "" {
			hasOffset = true
			break
		}
	}
	if !hasOffset {
		return
	}
	type chInfo struct {
		unit           string
		zeroOffset     float64
		zeroOffsetUnit string
	}
	chMap := make(map[int]chInfo, len(profile.Channels))
	for _, ch := range profile.Channels {
		chMap[ch.Index] = chInfo{ch.Unit, ch.ZeroOffset, ch.ZeroOffsetUnit}
	}
	for i, idx := range payload.ChannelIndices {
		if i >= len(payload.Channels) {
			break
		}
		info, ok := chMap[idx]
		if !ok || info.zeroOffset == 0 || info.zeroOffsetUnit == "" {
			continue
		}
		offsetInCurrentUnit, err := types.ConvertPressureToUnit(info.zeroOffset, info.zeroOffsetUnit, info.unit)
		if err != nil {
			slog.Warn("apply zero offset: unit convert failed", "channel", idx, "err", err)
			continue
		}
		payload.Channels[i] -= offsetInCurrentUnit
	}
}

// sampleFrames 从指定设备采集 count 帧数据（按 timestamp 去重）
func (m *DeviceManager) sampleFrames(deviceID string, count int, timeout time.Duration) ([]types.DataPayload, error) {
	frames := make([]types.DataPayload, 0, count)
	lastTs := int64(0)
	deadline := time.Now().Add(timeout)
	for len(frames) < count {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("采样超时：已收集 %d/%d 帧", len(frames), count)
		}
		data, ok := m.GetLatestData(deviceID)
		if !ok || data.Timestamp == lastTs {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		frames = append(frames, data)
		lastTs = data.Timestamp
	}
	return frames, nil
}

// computeChannelMean 计算指定通道在采样帧中的均值
func computeChannelMean(frames []types.DataPayload, channelIndex int) (float64, bool) {
	sum := 0.0
	count := 0
	for _, frame := range frames {
		for i, idx := range frame.ChannelIndices {
			if idx == channelIndex && i < len(frame.Channels) {
				sum += frame.Channels[i]
				count++
				break
			}
		}
	}
	if count == 0 {
		return 0, false
	}
	return sum / float64(count), true
}

// zeroCalibTarget 校零目标通道的瞬时快照
type zeroCalibTarget struct {
	index         int
	unit          string
	oldOffset     float64
	oldOffsetUnit string
}

// collectZeroCalibTargets 在 RLock 内收集需要校零的通道快照，并完成单位白名单校验
func (m *DeviceManager) collectZeroCalibTargets(id string, onlyIndex int) ([]zeroCalibTarget, error) {
	m.RLock()
	drv, ok := m.instances[id]
	profile := m.profiles[id]
	m.RUnlock()
	if !ok {
		return nil, fmt.Errorf("设备未连接: %s", id)
	}
	if !profile.Type.IsDAQDevice() || profile.Type.IsTemperatureDevice() {
		return nil, fmt.Errorf("设备 %s 不是压力扫描阀，不支持校零", id)
	}
	if !drv.IsAcquiring() {
		return nil, fmt.Errorf("设备未在采集: %s", id)
	}

	pressureCount := profile.Type.PressureChannelCount()
	targets := make([]zeroCalibTarget, 0)
	for i := range profile.Channels {
		ch := &profile.Channels[i]
		if ch.Index >= pressureCount || !ch.Enabled {
			continue
		}
		if onlyIndex >= 0 && ch.Index != onlyIndex {
			continue
		}
		if !types.IsZeroCalibrationSupported(ch.Unit) {
			return nil, fmt.Errorf("通道 %d 单位 %s 不支持校零，请先切换到 %v", ch.Index, ch.Unit, types.ZeroCalibrationSupportedUnits)
		}
		targets = append(targets, zeroCalibTarget{
			index:         ch.Index,
			unit:          ch.Unit,
			oldOffset:     ch.ZeroOffset,
			oldOffsetUnit: ch.ZeroOffsetUnit,
		})
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("没有可校零的启用压力通道")
	}
	return targets, nil
}

// applyZeroCalibResult 写回校零结果
func (m *DeviceManager) applyZeroCalibResult(id string, targets []zeroCalibTarget, frames []types.DataPayload) error {
	now := time.Now().UnixMilli()
	changed := false
	m.Lock()
	profile, exists := m.profiles[id]
	if !exists {
		m.Unlock()
		return fmt.Errorf("设备配置不存在: %s", id)
	}
	for _, t := range targets {
		for i := range profile.Channels {
			if profile.Channels[i].Index != t.index {
				continue
			}
			cur := &profile.Channels[i]
			curUnit := cur.Unit
			curOldOffset := cur.ZeroOffset
			curOldOffsetUnit := cur.ZeroOffsetUnit

			correctedMean, ok := computeChannelMean(frames, t.index)
			if !ok {
				continue
			}
			oldOffsetInCurrentUnit := 0.0
			if curOldOffset != 0 && curOldOffsetUnit != "" {
				converted, err := types.ConvertPressureToUnit(curOldOffset, curOldOffsetUnit, curUnit)
				if err != nil {
					slog.Warn("zero calibrate: convert old offset failed", "channel", t.index, "err", err)
				} else {
					oldOffsetInCurrentUnit = converted
				}
			}
			cur.ZeroOffset = correctedMean + oldOffsetInCurrentUnit
			cur.ZeroOffsetUnit = curUnit
			cur.ZeroCalibratedAt = now
			changed = true
			break
		}
	}
	m.profiles[id] = profile
	m.Unlock()
	if changed {
		m.saveProfilesWithLog("device")
	}
	return nil
}

// ZeroCalibrate 对指定设备的所有启用压力通道执行零位校准
func (m *DeviceManager) ZeroCalibrate(id string) error {
	targets, err := m.collectZeroCalibTargets(id, -1)
	if err != nil {
		return err
	}

	frames, err := m.sampleFrames(id, ZeroCalibrateSamples, zeroCalibTimeout)
	if err != nil {
		return err
	}

	if err := m.applyZeroCalibResult(id, targets, frames); err != nil {
		return err
	}
	slog.Info("zero calibrate done", "device", id, "channels", len(targets))
	return nil
}

// ZeroCalibrateChannel 对指定设备的单个通道执行零位校准
func (m *DeviceManager) ZeroCalibrateChannel(id string, channelIndex int) error {
	targets, err := m.collectZeroCalibTargets(id, channelIndex)
	if err != nil {
		return err
	}

	frames, err := m.sampleFrames(id, ZeroCalibrateSamples, zeroCalibTimeout)
	if err != nil {
		return err
	}

	if _, ok := computeChannelMean(frames, channelIndex); !ok {
		return fmt.Errorf("通道 %d 采样失败", channelIndex)
	}

	if err := m.applyZeroCalibResult(id, targets, frames); err != nil {
		return err
	}
	slog.Info("zero calibrate channel done", "device", id, "channel", channelIndex)
	return nil
}

// ClearZeroOffset 清除指定通道的零位偏移
func (m *DeviceManager) ClearZeroOffset(id string, channelIndex int) error {
	m.Lock()
	profile, ok := m.profiles[id]
	if !ok {
		m.Unlock()
		return fmt.Errorf("设备配置不存在: %s", id)
	}
	for i := range profile.Channels {
		if profile.Channels[i].Index == channelIndex {
			profile.Channels[i].ZeroOffset = 0
			profile.Channels[i].ZeroOffsetUnit = ""
			profile.Channels[i].ZeroCalibratedAt = 0
			break
		}
	}
	m.profiles[id] = profile
	m.Unlock()
	m.saveProfilesWithLog("device")
	return nil
}

// ClearAllZeroOffsets 清除指定设备所有通道的零位偏移
func (m *DeviceManager) ClearAllZeroOffsets(id string) error {
	m.Lock()
	profile, ok := m.profiles[id]
	if !ok {
		m.Unlock()
		return fmt.Errorf("设备配置不存在: %s", id)
	}
	for i := range profile.Channels {
		profile.Channels[i].ZeroOffset = 0
		profile.Channels[i].ZeroOffsetUnit = ""
		profile.Channels[i].ZeroCalibratedAt = 0
	}
	m.profiles[id] = profile
	m.Unlock()
	m.saveProfilesWithLog("device")
	return nil
}
