package five_hole

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"yx-daq/internal/types"
)

// DataProcessor 五孔数据处理器
// 负责多探针并行采集（每通道独立 DeviceID）+ 3σ 滤波 + 插值调度
type DataProcessor struct {
	batchGetter FiveHoleMultiDeviceBatchGetter
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
//   - 全局 PAtm/TAtm 一次读取（三根共用）
//   - 各探针 P1-P5 按设备分组并行读取
//   - lastTimestamps: 上次采样各设备的 timestamp，用于判断新帧（首次传 nil）
//   - 返回本次采样的 timestamps 供下次调用传入
func (dp *DataProcessor) ReadAllProbesRawData(
	probes []types.FiveHoleProbeConfig,
	pAtmDeviceID string, pAtmChannel int,
	tAtmDeviceID string, tAtmChannel int,
	lastTimestamps map[string]int64,
) ([]types.FiveHoleRawData, map[string]int64, error) {
	if dp.batchGetter == nil {
		return nil, nil, fmt.Errorf("batch getter not set")
	}

	currentTimestamps := make(map[string]int64)

	// 读取全局 PAtm/TAtm（同时返回 timestamp 用于后续去重）
	pAtmVal, pAtmTs, err := dp.readSingleChannel(pAtmDeviceID, pAtmChannel)
	if err != nil {
		return nil, nil, fmt.Errorf("读取大气压通道失败: %w", err)
	}
	tAtmVal, tAtmTs, err := dp.readSingleChannel(tAtmDeviceID, tAtmChannel)
	if err != nil {
		return nil, nil, fmt.Errorf("读取大气温度通道失败: %w", err)
	}
	if pAtmDeviceID != "" {
		currentTimestamps[pAtmDeviceID] = pAtmTs
	}
	if tAtmDeviceID != "" && tAtmDeviceID != pAtmDeviceID {
		currentTimestamps[tAtmDeviceID] = tAtmTs
	}

	// 各探针并行读取 P1-P5
	results := make([]types.FiveHoleRawData, len(probes))
	var mu sync.Mutex
	var wg sync.WaitGroup
	errChan := make(chan error, len(probes))

	for i, probe := range probes {
		if !probe.Enabled {
			continue
		}
		wg.Add(1)
		go func(idx int, p types.FiveHoleProbeConfig) {
			defer wg.Done()
			rawData, probeTimestamps, err := dp.readProbeRawData(p)
			if err != nil {
				errChan <- fmt.Errorf("探针%s 读取失败: %w", p.ProbeID, err)
				return
			}
			rawData.PAtm = pAtmVal
			rawData.TAtm = tAtmVal
			results[idx] = rawData
			mu.Lock()
			for did, ts := range probeTimestamps {
				if existing, ok := currentTimestamps[did]; !ok || ts > existing {
					currentTimestamps[did] = ts
				}
			}
			mu.Unlock()
		}(i, probe)
	}

	wg.Wait()
	close(errChan)
	var errs []string
	for err := range errChan {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return nil, nil, fmt.Errorf("读取探针数据失败: %s", strings.Join(errs, "; "))
	}

	return results, currentTimestamps, nil
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
// 返回 (rawData, 各设备timestamp, error)
func (dp *DataProcessor) readProbeRawData(probe types.FiveHoleProbeConfig) (types.FiveHoleRawData, map[string]int64, error) {
	var rawData types.FiveHoleRawData

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
	var errs []string
	for err := range errChan {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return rawData, nil, fmt.Errorf("读取探针%s 设备数据失败: %s", probe.ProbeID, strings.Join(errs, "; "))
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

	var ok bool
	if rawData.P1, ok = getValue(types.Role5H_P1); !ok {
		return rawData, nil, fmt.Errorf("P1 通道无数据")
	}
	if rawData.P2, ok = getValue(types.Role5H_P2); !ok {
		return rawData, nil, fmt.Errorf("P2 通道无数据")
	}
	if rawData.P3, ok = getValue(types.Role5H_P3); !ok {
		return rawData, nil, fmt.Errorf("P3 通道无数据")
	}
	if rawData.P4, ok = getValue(types.Role5H_P4); !ok {
		return rawData, nil, fmt.Errorf("P4 通道无数据")
	}
	if rawData.P5, ok = getValue(types.Role5H_P5); !ok {
		return rawData, nil, fmt.Errorf("P5 通道无数据")
	}

	return rawData, deviceTimestamps, nil
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
