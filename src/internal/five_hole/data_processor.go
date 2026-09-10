package five_hole

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"yx-daq/internal/types"
)

// DataProcessor 五孔数据处理器
// 负责多探针并行采集（每通道独立 DeviceID）+ 3σ 滤波 + 插值调度
type DataProcessor struct {
	batchGetter  FiveHoleMultiDeviceBatchGetter
	noDataWarned atomic.Bool // 参照三孔：数据缺失时仅首次打 WARN，恢复后重置
}

// NewDataProcessor 创建数据处理器
func NewDataProcessor() *DataProcessor {
	return &DataProcessor{}
}

// SetBatchGetter 设置批量数据获取函数
func (dp *DataProcessor) SetBatchGetter(getter FiveHoleMultiDeviceBatchGetter) {
	dp.batchGetter = getter
}

// ReadAllProbesRawData 读取所有启用探针的原始数据
//   - 各探针 P1-P5 按设备分组并行读取
//   - 各探针 PAtm/TAtm 按其数据源配置解析：device=设备读取（按探针独立）/ manual=手动固定值
//   - lastTimestamps: 上次采样各设备的 timestamp，用于判断新帧（首次传 nil）
//   - 返回本次采样的 timestamps 供下次调用传入
func (dp *DataProcessor) ReadAllProbesRawData(
	probes []types.FiveHoleProbeConfig,
	lastTimestamps map[string]int64,
) ([]*types.FiveHoleRawData, map[string]int64, error) {
	if dp.batchGetter == nil {
		return nil, nil, fmt.Errorf("batch getter not set")
	}

	currentTimestamps := make(map[string]int64)

	// 各探针并行读取 P1-P5 + 解析各自 PAtm/TAtm 数据源
	// 语义：
	//   - manual 模式：直接取 ManualValue，无需读取设备
	//   - device 模式：读取指定设备通道，失败时该值降级为 0 并回传 err
	results := make([]*types.FiveHoleRawData, len(probes))
	var mu sync.Mutex
	var wg sync.WaitGroup
	var atmErrs []error

	for i, probe := range probes {
		if !probe.Enabled {
			continue
		}
		wg.Add(1)
		go func(idx int, p types.FiveHoleProbeConfig) {
			defer wg.Done()
			rawData, probeTimestamps := dp.readProbeRawData(p)
			if rawData == nil {
				return // 数据缺失，静默跳过（WARN 已在 readProbeRawData 内限频）
			}
			// 解析该探针的 PAtm/TAtm 数据源（设备读取或手动值）
			pAtmVal, pAtmErr := dp.resolveAtmSource(p.ProbeID, "大气压", p.PAtmSource, probeTimestamps)
			tAtmVal, tAtmErr := dp.resolveAtmSource(p.ProbeID, "气流温度", p.TAtmSource, probeTimestamps)
			rawData.PAtm = pAtmVal
			rawData.TAtm = tAtmVal
			mu.Lock()
			results[idx] = rawData
			for did, ts := range probeTimestamps {
				if existing, ok := currentTimestamps[did]; !ok || ts > existing {
					currentTimestamps[did] = ts
				}
			}
			if pAtmErr != nil || tAtmErr != nil {
				atmErrs = append(atmErrs, errors.Join(pAtmErr, tAtmErr))
			}
			mu.Unlock()
		}(i, probe)
	}

	wg.Wait()
	// PAtm/TAtm 读取失败时回传 err，但 results 仍填充完整 P1-P5 数据：
	//   - samplePoint: err 视为致命，丢弃 probeSamples + break，由下轮 WaitForFreshData 触发自动暂停
	//   - emitRealtimeForAllProbes: 限频 WARN，仍用 results 推送（实现"谁配置谁更新"）
	return results, currentTimestamps, errors.Join(atmErrs...)
}

// resolveAtmSource 解析单探针的大气压/气流温度数据源
//   - manual：直接返回手动值
//   - device：读取指定设备通道，成功时记录 timestamp 到 probeTimestamps（供新帧去重）
//     未配置设备（DeviceID 为空）返回 0 不报错（实时监控阶段容忍未配置）
func (dp *DataProcessor) resolveAtmSource(probeID, name string, src types.FiveHoleAtmSource, probeTimestamps map[string]int64) (float64, error) {
	if src.Mode == types.FiveHoleSourceManual {
		return src.ManualValue, nil
	}
	// device 模式（Mode 为空视为 device，兼容旧配置缺省值）
	if src.DeviceID == "" {
		return 0, nil
	}
	val, ts, err := dp.readSingleChannel(src.DeviceID, src.Channel)
	if err != nil {
		return 0, fmt.Errorf("探针%s的%s读取失败: %w", probeID, name, err)
	}
	if probeTimestamps != nil {
		if existing, ok := probeTimestamps[src.DeviceID]; !ok || ts > existing {
			probeTimestamps[src.DeviceID] = ts
		}
	}
	return val, nil
}

// WaitForFreshData 等待任一指定设备产生新帧（timestamp 严格大于 lastTimestamps[did]）
// 超时返回 ErrDataStagnant，由调用方决定暂停或重试
func (dp *DataProcessor) WaitForFreshData(
	deviceIDs []string,
	lastTimestamps map[string]int64,
	timeout time.Duration,
) (map[string]int64, error) {
	if dp.batchGetter == nil {
		return nil, fmt.Errorf("batch getter not set")
	}

	deadline := time.Now().Add(timeout)
	pollInterval := 20 * time.Millisecond

	for time.Now().Before(deadline) {
		// 用空通道列表只取 timestamp（避免不必要的数据拷贝）
		allFresh := true
		currentTs := make(map[string]int64)
		for _, did := range deviceIDs {
			if did == "" {
				continue
			}
			_, ts, err := dp.batchGetter(did, nil)
			if err != nil {
				return nil, err
			}
			currentTs[did] = ts
			last, existed := lastTimestamps[did]
			if !existed || ts <= last {
				allFresh = false
			}
		}
		if allFresh {
			return currentTs, nil
		}
		time.Sleep(pollInterval)
	}
	return nil, ErrDataStagnant
}

// readSingleChannel 读取单通道，返回 (数值, 设备timestamp, error)
func (dp *DataProcessor) readSingleChannel(deviceID string, channel int) (float64, int64, error) {
	values, ts, err := dp.batchGetter(deviceID, []int{channel})
	if err != nil {
		return 0, 0, err
	}
	val, ok := values[channel]
	if !ok {
		return 0, 0, fmt.Errorf("通道 %d 无数据", channel)
	}
	return val, ts, nil
}

// readProbeRawData 读取单根探针的 P1-P5（不含 PAtm/TAtm）
// 数据缺失时返回 nil，参照三孔模式：仅首次打 WARN，恢复后重置告警标记
func (dp *DataProcessor) readProbeRawData(probe types.FiveHoleProbeConfig) (*types.FiveHoleRawData, map[string]int64) {
	// 按设备分组通道，减少调用次数
	deviceChannels := make(map[string][]int)
	roleToChannel := make(map[types.FiveHoleChannelRole]int)
	roleToDevice := make(map[types.FiveHoleChannelRole]string)

	for _, ch := range probe.ProbeChannels {
		if !ch.Enabled {
			continue
		}
		deviceChannels[ch.DeviceID] = append(deviceChannels[ch.DeviceID], ch.Channel)
		roleToChannel[ch.Role] = ch.Channel
		roleToDevice[ch.Role] = ch.DeviceID
	}

	// 并行读取各设备
	var mu sync.Mutex
	deviceValues := make(map[string]map[int]float64)
	deviceTimestamps := make(map[string]int64)
	var wg sync.WaitGroup
	errChan := make(chan error, len(deviceChannels))

	for deviceID, channels := range deviceChannels {
		wg.Add(1)
		go func(did string, chs []int) {
			defer wg.Done()
			vals, ts, err := dp.batchGetter(did, chs)
			if err != nil {
				errChan <- err
				return
			}
			mu.Lock()
			deviceValues[did] = vals
			deviceTimestamps[did] = ts
			mu.Unlock()
		}(deviceID, channels)
	}

	wg.Wait()
	close(errChan)
	for err := range errChan {
		_ = err // batchGetter 错误已由调用方处理
	}

	// 按角色填充
	getValue := func(role types.FiveHoleChannelRole) (float64, bool) {
		ch, ok := roleToChannel[role]
		if !ok {
			return 0, false
		}
		did := roleToDevice[role]
		vals, ok := deviceValues[did]
		if !ok {
			return 0, false
		}
		v, ok := vals[ch]
		return v, ok
	}

	var rawData types.FiveHoleRawData
	var ok bool
	gotP1, gotP2, gotP3, gotP4, gotP5 := false, false, false, false, false
	missingChannels := make([]string, 0, 5)

	if rawData.P1, ok = getValue(types.Role5H_P1); !ok {
		missingChannels = append(missingChannels, "P1")
	} else {
		gotP1 = true
	}
	if rawData.P2, ok = getValue(types.Role5H_P2); !ok {
		missingChannels = append(missingChannels, "P2")
	} else {
		gotP2 = true
	}
	if rawData.P3, ok = getValue(types.Role5H_P3); !ok {
		missingChannels = append(missingChannels, "P3")
	} else {
		gotP3 = true
	}
	if rawData.P4, ok = getValue(types.Role5H_P4); !ok {
		missingChannels = append(missingChannels, "P4")
	} else {
		gotP4 = true
	}
	if rawData.P5, ok = getValue(types.Role5H_P5); !ok {
		missingChannels = append(missingChannels, "P5")
	} else {
		gotP5 = true
	}

	if !gotP1 || !gotP2 || !gotP3 || !gotP4 || !gotP5 {
		// 参照三孔：数据缺失时仅首次打 WARN，后续帧静默跳过
		if dp.noDataWarned.CompareAndSwap(false, true) {
			slog.Warn("五孔探针通道数据缺失（后续帧静默跳过，设备就绪后自动恢复）",
				"probeId", probe.ProbeID, "missing", missingChannels)
		}
		return nil, nil
	}

	// 数据完整，重置告警标记
	dp.noDataWarned.Store(false)
	return &rawData, deviceTimestamps
}

// OutlierFilteredAvg 3σ 滤波后取均值
// 复用三孔 outlierFilteredAvg 逻辑
func OutlierFilteredAvg(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	if len(values) == 1 {
		return values[0]
	}

	// 计算均值
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))

	// 计算标准差
	var sqSum float64
	for _, v := range values {
		diff := v - mean
		sqSum += diff * diff
	}
	stdDev := math.Sqrt(sqSum / float64(len(values)))

	// 3σ 滤波
	if stdDev == 0 {
		return mean
	}
	var filtered []float64
	for _, v := range values {
		if math.Abs(v - mean) <= 3*stdDev {
			filtered = append(filtered, v)
		}
	}
	if len(filtered) == 0 {
		return mean
	}

	filteredSum := 0.0
	for _, v := range filtered {
		filteredSum += v
	}
	return filteredSum / float64(len(filtered))
}
