package manager

import (
	"fmt"
	"log/slog"
	"time"

	"yx-daq/internal/types"
)

// applyTempCalib 对数据帧应用温度线性校准 y = a*x + b（原地修改 payload.Channels）。
// 在数据回调中、applyZeroOffset 之后调用，串联顺序保持与压力校准一致。
// 注意：applyZeroOffset 内部对温度设备（EA2516T）跳过零位修正（IsPressureDevice 排除温度设备），
// 实际仅 applyTempCalib 生效，但保留串联顺序便于将来扩展。
// 仅对温度设备（EA2516T）生效，压力设备直接返回。
// 校准条件：TempCalibratedAt != 0（已校准）且通道启用。
func (m *DeviceManager) applyTempCalib(deviceID string, payload *types.DataPayload) {
	m.RLock()
	profile, ok := m.profiles[deviceID]
	m.RUnlock()
	if !ok || !profile.Type.IsTemperatureDevice() {
		return
	}
	// 锁外构建 channelIndex → (a, b) 映射，避免在循环中持锁
	type coeff struct{ a, b float64 }
	coeffs := make(map[int]coeff, len(profile.Channels))
	for _, ch := range profile.Channels {
		if !ch.Enabled || ch.TempCalibratedAt == 0 {
			continue
		}
		coeffs[ch.Index] = coeff{a: ch.TempCalibA, b: ch.TempCalibB}
	}
	if len(coeffs) == 0 {
		return
	}
	for i, idx := range payload.ChannelIndices {
		if i >= len(payload.Channels) {
			break
		}
		c, ok := coeffs[idx]
		if !ok {
			continue
		}
		payload.Channels[i] = c.a*payload.Channels[i] + c.b
	}
}

// SetOnTempCalibChange 设置温度校准变更回调（由 Core 层桥接到 Wails 事件系统）。
// 在 TempCalibWrite / ClearTempCalib / ClearAllTempCalib 成功后调用。
// 回调参数为设备 ID，前端监听后可刷新 profiles 以更新"已校准"徽标。
// 加锁 setter 符合"所有回调字段读写必须用 mu 保护"的并发规范。
func (m *DeviceManager) SetOnTempCalibChange(cb func(deviceID string)) {
	m.Lock()
	defer m.Unlock()
	m.onTempCalibChange = cb
}

// emitTempCalibChange 触发温度校准变更回调（锁外调用 cb，避免持锁调用外部函数）
func (m *DeviceManager) emitTempCalibChange(deviceID string) {
	m.RLock()
	cb := m.onTempCalibChange
	m.RUnlock()
	if cb != nil {
		cb(deviceID)
	}
}

// collectTempCalibTargets 在 RLock 内收集温度设备上需采点的启用通道
func (m *DeviceManager) collectTempCalibTargets(id string, channelIndices []int) ([]types.ChannelConfig, error) {
	m.RLock()
	drv, ok := m.instances[id]
	profile := m.profiles[id]
	m.RUnlock()
	if !ok {
		return nil, fmt.Errorf("设备未连接: %s", id)
	}
	if !profile.Type.IsTemperatureDevice() {
		return nil, fmt.Errorf("设备 %s 不是温度采集设备，不支持温度校准", id)
	}
	if !drv.IsAcquiring() {
		return nil, fmt.Errorf("设备未在采集: %s", id)
	}
	// 用户勾选的通道索引集合
	want := make(map[int]struct{}, len(channelIndices))
	for _, idx := range channelIndices {
		want[idx] = struct{}{}
	}
	targets := make([]types.ChannelConfig, 0, len(channelIndices))
	for _, ch := range profile.Channels {
		if !ch.Enabled {
			continue
		}
		if len(want) > 0 {
			if _, ok := want[ch.Index]; !ok {
				continue
			}
		}
		targets = append(targets, ch)
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("没有可采点的启用温度通道")
	}
	return targets, nil
}

// TempCalibSample 批量采点：对勾选通道各采 10 帧（复用 sampleFrames），返回每通道均值。
// channelIndices 为空时报错（与压力校零"全部启用通道"语义不同：温度校准点数有限，必须显式勾选）。
func (m *DeviceManager) TempCalibSample(id string, channelIndices []int) ([]types.TempCalibSampleResult, error) {
	if len(channelIndices) == 0 {
		return nil, fmt.Errorf("请至少勾选一个通道")
	}
	targets, err := m.collectTempCalibTargets(id, channelIndices)
	if err != nil {
		return nil, err
	}
	frames, err := m.sampleFrames(id, ZeroCalibrateSamples, zeroCalibTimeout)
	if err != nil {
		return nil, err
	}
	results := make([]types.TempCalibSampleResult, 0, len(targets))
	for _, t := range targets {
		mean, ok := computeChannelMean(frames, t.Index)
		if !ok {
			results = append(results, types.TempCalibSampleResult{
				ChannelIndex: t.Index,
				ChannelName:  t.Name,
				Error:        "采样失败：未获取到该通道数据",
			})
			continue
		}
		results = append(results, types.TempCalibSampleResult{
			ChannelIndex: t.Index,
			ChannelName:  t.Name,
			MeanValue:    mean,
			SampleCount:  len(frames),
		})
	}
	slog.Info("temp calib sample done", "device", id, "channels", len(results))
	return results, nil
}

// TempCalibFit 对给定校准点做线性回归 y = a*x + b，返回 a/b/R²。
// 转发到 types.FitLinearRegression 纯函数，DeviceManager 不承担数学计算职责。
// 保留 DeviceManager 方法仅为 Service 层透传便利，避免直接暴露 types 包到前端 binding。
func (m *DeviceManager) TempCalibFit(points []types.TempCalibPoint) (types.TempCalibResult, error) {
	return types.FitLinearRegression(points)
}

// TempCalibWrite 将拟合结果写入指定通道的 ChannelConfig（5 个 TempCalib* 字段）。
// 写入后下一帧数据即应用新系数（applyTempCalib 在数据回调里实时读取 profile）。
// 写入成功后触发 onTempCalibChange 回调，由 Core 层广播 device:temp-calib-updated 事件。
func (m *DeviceManager) TempCalibWrite(id string, channelIndex int, result types.TempCalibResult) error {
	m.Lock()
	profile, ok := m.profiles[id]
	if !ok {
		m.Unlock()
		return fmt.Errorf("设备配置不存在: %s", id)
	}
	if !profile.Type.IsTemperatureDevice() {
		m.Unlock()
		return fmt.Errorf("设备 %s 不是温度采集设备", id)
	}
	found := false
	for i := range profile.Channels {
		if profile.Channels[i].Index != channelIndex {
			continue
		}
		profile.Channels[i].TempCalibA = result.A
		profile.Channels[i].TempCalibB = result.B
		profile.Channels[i].TempCalibR2 = result.R2
		profile.Channels[i].TempCalibPoints = result.Points
		profile.Channels[i].TempCalibratedAt = time.Now().UnixMilli()
		found = true
		break
	}
	if !found {
		m.Unlock()
		return fmt.Errorf("通道 %d 不存在于设备 %s", channelIndex, id)
	}
	m.profiles[id] = profile
	m.Unlock()
	m.saveProfilesWithLog("device")
	m.emitTempCalibChange(id)
	slog.Info("temp calib written", "device", id, "channel", channelIndex, "a", result.A, "b", result.B, "r2", result.R2, "points", result.Points)
	return nil
}

// ClearTempCalib 清空指定通道的温度校准（5 字段全部清零）。
// 清除成功后触发 onTempCalibChange 回调。
func (m *DeviceManager) ClearTempCalib(id string, channelIndex int) error {
	m.Lock()
	profile, ok := m.profiles[id]
	if !ok {
		m.Unlock()
		return fmt.Errorf("设备配置不存在: %s", id)
	}
	found := false
	changed := false
	for i := range profile.Channels {
		if profile.Channels[i].Index != channelIndex {
			continue
		}
		if profile.Channels[i].TempCalibratedAt == 0 {
			// 未校准也认为成功，但不触发变更事件
			found = true
			break
		}
		profile.Channels[i].TempCalibA = 0
		profile.Channels[i].TempCalibB = 0
		profile.Channels[i].TempCalibR2 = 0
		profile.Channels[i].TempCalibPoints = 0
		profile.Channels[i].TempCalibratedAt = 0
		found = true
		changed = true
		break
	}
	if !found {
		m.Unlock()
		return fmt.Errorf("通道 %d 不存在于设备 %s", channelIndex, id)
	}
	m.profiles[id] = profile
	m.Unlock()
	if changed {
		m.saveProfilesWithLog("device")
		m.emitTempCalibChange(id)
	}
	slog.Info("temp calib cleared", "device", id, "channel", channelIndex, "changed", changed)
	return nil
}

// ClearAllTempCalib 清空指定设备所有通道的温度校准。
// 清除成功后触发 onTempCalibChange 回调。
func (m *DeviceManager) ClearAllTempCalib(id string) error {
	m.Lock()
	profile, ok := m.profiles[id]
	if !ok {
		m.Unlock()
		return fmt.Errorf("设备配置不存在: %s", id)
	}
	changed := false
	for i := range profile.Channels {
		if profile.Channels[i].TempCalibratedAt == 0 {
			continue
		}
		profile.Channels[i].TempCalibA = 0
		profile.Channels[i].TempCalibB = 0
		profile.Channels[i].TempCalibR2 = 0
		profile.Channels[i].TempCalibPoints = 0
		profile.Channels[i].TempCalibratedAt = 0
		changed = true
	}
	m.profiles[id] = profile
	m.Unlock()
	if changed {
		m.saveProfilesWithLog("device")
		m.emitTempCalibChange(id)
	}
	slog.Info("temp calib cleared all", "device", id, "changed", changed)
	return nil
}
