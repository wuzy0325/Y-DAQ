package three_hole

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"yx-daq/internal/types"
)

// DataProcessor 数据处理器
type DataProcessor struct {
	batchGetter      ThreeHoleBatchGetter
	motionCtrl       ThreeHoleMotionController
	motionWaiter     ThreeHoleMotionWaiter
	testManager      *TestManager
	interpolator     *ThreeHoleInterpolator
	eventPublisher   ThreeHoleEventPublisher

	// 实时监控相关
	monitorRunning atomic.Bool
	monitorCtx     context.Context
	monitorCancel  context.CancelFunc
	// 标记是否正在执行测试，避免监控和测试数据冲突
	testRunning    atomic.Bool
	// 节流：readRawData 缺数据时只打一次 WARN，数据恢复后重置
	noDataWarned   atomic.Bool

	// 实时数据录制
	realtimeRecorder *RealtimeRecorder
}

// NewDataProcessor 创建数据处理器
func NewDataProcessor(testManager *TestManager, interpolator *ThreeHoleInterpolator, publisher ThreeHoleEventPublisher) *DataProcessor {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 初始状态为已取消，StartRealtimeMonitor 时重新创建
	return &DataProcessor{
		testManager:      testManager,
		interpolator:     interpolator,
		eventPublisher:   publisher,
		monitorCtx:       ctx,
		monitorCancel:    cancel,
		realtimeRecorder: NewRealtimeRecorder(),
	}
}

// SetBatchGetter 设置批量数据获取函数
func (dp *DataProcessor) SetBatchGetter(getter ThreeHoleBatchGetter) {
	dp.batchGetter = getter
}

// SetMotionController 设置运动控制函数
func (dp *DataProcessor) SetMotionController(ctrl ThreeHoleMotionController) {
	dp.motionCtrl = ctrl
}

// SetMotionWaiter 设置运动等待函数
func (dp *DataProcessor) SetMotionWaiter(waiter ThreeHoleMotionWaiter) {
	dp.motionWaiter = waiter
}

// StartRealtimeMonitor 启动实时数据监控
func (dp *DataProcessor) StartRealtimeMonitor(config types.ThreeHoleTraversalConfig) {
	if dp.monitorRunning.Load() {
		return
	}
	dp.testManager.config = config
	dp.monitorCtx, dp.monitorCancel = context.WithCancel(context.Background())
	dp.monitorRunning.Store(true)

	go dp.runRealtimeMonitor()
}

// StopRealtimeMonitor 停止实时数据监控
func (dp *DataProcessor) StopRealtimeMonitor() {
	if dp.monitorRunning.Load() && dp.monitorCancel != nil {
		dp.monitorCancel()
	}
}

// StartRealtimeRecording 开始实时数据录制
func (dp *DataProcessor) StartRealtimeRecording(savePath string) error {
	return dp.realtimeRecorder.Start(savePath)
}

// StopRealtimeRecording 停止实时数据录制
func (dp *DataProcessor) StopRealtimeRecording() {
	dp.realtimeRecorder.Stop()
}

// IsRealtimeRecording 是否正在录制实时数据
func (dp *DataProcessor) IsRealtimeRecording() bool {
	return dp.realtimeRecorder.IsRecording()
}

// runRealtimeMonitor 实时监控协程
func (dp *DataProcessor) runRealtimeMonitor() {
	defer dp.monitorRunning.Store(false)

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-dp.monitorCtx.Done():
			return
		case <-ticker.C:
		}

		// 如果正在执行测试，监控不推送数据（避免重复）
		if dp.testRunning.Load() {
			continue
		}

		// 读取原始数据
		rawData := dp.readRawData()
		if rawData == nil || dp.eventPublisher == nil {
			continue
		}

		// 计算插值（如果校准文件已加载）
		interpResult := dp.interpolator.Calculate(*rawData)

		evt := types.ThreeHoleTraversalRealtimeEvent{
			TaskID:       "monitor",
			PointID:      "realtime",
			RawData:      *rawData,
			InterpResult: interpResult,
		}
		dp.eventPublisher.EmitRealtime(evt)
		dp.realtimeRecorder.Record(evt)
	}
}

// RunSinglePoint 执行单点测试
func (dp *DataProcessor) RunSinglePoint(point types.TraversalPoint) (types.ThreeHoleTraversalDataPoint, error) {
	if err := dp.testManager.CheckCancelled(); err != nil {
		return types.ThreeHoleTraversalDataPoint{}, err
	}

	if err := dp.MoveToPoint(point); err != nil {
		return types.ThreeHoleTraversalDataPoint{}, err
	}

	dp.testManager.WaitForResume()
	if err := dp.testManager.CheckCancelled(); err != nil {
		return types.ThreeHoleTraversalDataPoint{}, err
	}

	dp.DwellWithRealtimeUpdate(point)

	return dp.AcquireAndInterpolate(point)
}

// MoveToPoint 运动到指定点位
func (dp *DataProcessor) MoveToPoint(point types.TraversalPoint) error {
	if dp.testManager == nil {
		return fmt.Errorf("test manager not initialized")
	}

	if dp.motionCtrl == nil {
		return fmt.Errorf("motion controller not set")
	}

	// 发送运动开始事件
	dp.testManager.EmitProgress(dp.testManager.status.TaskID, dp.testManager.status.TotalPoints,
		dp.testManager.status.CompletedPoints, dp.testManager.status.Progress, point.X, point.Y, "moving")

	// 并行控制Alpha和Beta轴
	var wg sync.WaitGroup
	errChan := make(chan error, 2)

	// Alpha轴控制
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := dp.motionCtrl(dp.testManager.config.MotionAlpha.Axis, point.X); err != nil {
			errChan <- fmt.Errorf("move α axis to %.2f failed: %w", point.X, err)
		}
	}()

	// Beta轴控制
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := dp.motionCtrl(dp.testManager.config.MotionBeta.Axis, point.Y); err != nil {
			errChan <- fmt.Errorf("move β axis to %.2f failed: %w", point.Y, err)
		}
	}()

	wg.Wait()
	close(errChan)

	// 检查运动错误
	for err := range errChan {
		return err
	}

	// 并行等待运动完成
	if dp.motionWaiter != nil {
		motionTimeout := dp.testManager.config.MotionTimeoutMs
		if motionTimeout <= 0 {
			motionTimeout = 30000
		}

		wg = sync.WaitGroup{}
		wg.Add(2)

		// Alpha轴等待
		go func() {
			defer wg.Done()
			if err := dp.motionWaiter(dp.testManager.config.MotionAlpha.Axis, motionTimeout); err != nil {
				dp.testManager.EmitPointError(fmt.Sprintf("α轴运动超时: %v", err))
				slog.Warn("motion waiter α axis timeout", "axis", dp.testManager.config.MotionAlpha.Axis, "err", err)
			}
		}()

		// Beta轴等待
		go func() {
			defer wg.Done()
			if err := dp.motionWaiter(dp.testManager.config.MotionBeta.Axis, motionTimeout); err != nil {
				dp.testManager.EmitPointError(fmt.Sprintf("β轴运动超时: %v", err))
				slog.Warn("motion waiter β axis timeout", "axis", dp.testManager.config.MotionBeta.Axis, "err", err)
			}
		}()

		wg.Wait()
	}

	return nil
}

// DwellWithRealtimeUpdate 在驻留等待期间持续推送实时数据
func (dp *DataProcessor) DwellWithRealtimeUpdate(point types.TraversalPoint) {
	dp.testManager.EmitProgress(dp.testManager.status.TaskID, dp.testManager.status.TotalPoints,
		dp.testManager.status.CompletedPoints, dp.testManager.status.Progress, point.X, point.Y, "waiting")

	dwellDuration := time.Duration(dp.testManager.config.DwellTimeMs) * time.Millisecond
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	deadline := time.Now().Add(dwellDuration)

	for {
		select {
		case <-dp.testManager.ctx.Done():
			return
		case <-ticker.C:
		}

		pauseStart := time.Time{}
		for dp.testManager.paused.Load() {
			if pauseStart.IsZero() {
				pauseStart = time.Now()
			}
			select {
			case <-dp.testManager.ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
			}
		}

		// 如果暂停过，调整deadline
		if !pauseStart.IsZero() {
			pausedDuration := time.Since(pauseStart)
			deadline = deadline.Add(pausedDuration)
			pauseStart = time.Time{}
		}

		// 推送实时数据
		if dp.eventPublisher != nil {
			rawData := dp.readRawData()
			if rawData != nil {
				interpResult := dp.interpolator.Calculate(*rawData)
				evt := types.ThreeHoleTraversalRealtimeEvent{
					TaskID:       dp.testManager.status.TaskID,
					PointID:      point.ID,
					RawData:      *rawData,
					InterpResult: interpResult,
				}
				dp.eventPublisher.EmitRealtime(evt)
				dp.realtimeRecorder.Record(evt)
			}
		}

		if time.Now().After(deadline) {
			return
		}
	}
}

// AcquireAndInterpolate 采集数据并执行插值
func (dp *DataProcessor) AcquireAndInterpolate(point types.TraversalPoint) (types.ThreeHoleTraversalDataPoint, error) {
	samples := []types.ThreeHoleRawData{}

	// 发送采集开始事件
	dp.testManager.EmitProgress(dp.testManager.status.TaskID, dp.testManager.status.TotalPoints,
		dp.testManager.status.CompletedPoints, dp.testManager.status.Progress, point.X, point.Y, "acquiring")

	for i := 0; i < dp.testManager.config.SamplesPerPoint; i++ {
		if err := dp.testManager.CheckCancelled(); err != nil {
			return types.ThreeHoleTraversalDataPoint{}, err
		}

		dp.testManager.WaitForResume()
		if err := dp.testManager.CheckCancelled(); err != nil {
			return types.ThreeHoleTraversalDataPoint{}, err
		}

		// 读取原始数据
		rawData := dp.readRawData()
		if rawData == nil {
			continue
		}
		samples = append(samples, *rawData)

		// 推送实时数据
		if dp.eventPublisher != nil {
			interpResult := dp.interpolator.Calculate(*rawData)
			evt := types.ThreeHoleTraversalRealtimeEvent{
				TaskID:       dp.testManager.status.TaskID,
				PointID:      point.ID,
				RawData:      *rawData,
				InterpResult: interpResult,
			}
			dp.eventPublisher.EmitRealtime(evt)
			dp.realtimeRecorder.Record(evt)
		}

		// 采样间隔
		intervalMs := dp.testManager.config.SampleIntervalMs
		if intervalMs <= 0 {
			intervalMs = 50
		}
		time.Sleep(time.Duration(intervalMs) * time.Millisecond)
	}

	if len(samples) == 0 {
		return types.ThreeHoleTraversalDataPoint{
			PointID:   point.ID,
			X:         point.X,
			Y:         point.Y,
			Timestamp: time.Now().UnixMilli(),
		}, fmt.Errorf("no samples collected")
	}

	// 计算平均原始数据（3σ异常值剔除）
	avgData := calculateThreeHoleAverage(samples)

	// 对平均数据执行插值
	interpResult := dp.interpolator.Calculate(avgData)

	dataPoint := types.ThreeHoleTraversalDataPoint{
		PointID:      point.ID,
		X:            point.X,
		Y:            point.Y,
		RawData:      avgData,
		InterpResult: interpResult,
		SampleCount:  len(samples),
		Timestamp:    time.Now().UnixMilli(),
	}

	// 更新到测试管理器
	dp.testManager.UpdateDataPoint(dataPoint)

	return dataPoint, nil
}

// readRawData 读取三孔原始数据
func (dp *DataProcessor) readRawData() *types.ThreeHoleRawData {
	if dp.batchGetter == nil {
		return nil
	}

	data, err := dp.batchGetter(dp.testManager.config.ProbeChannels)
	if err != nil {
		slog.Warn("readRawData batch getter failed", "err", err)
		return nil
	}

	result := &types.ThreeHoleRawData{}
	gotP1, gotP2, gotP3, gotPAtm := false, false, false, false
	missingChannels := make([]int, 0, len(dp.testManager.config.ProbeChannels))
	for _, ch := range dp.testManager.config.ProbeChannels {
		if !ch.Enabled {
			continue
		}
		val, ok := data[ch.Channel]
		if !ok {
			missingChannels = append(missingChannels, ch.Channel)
			continue
		}
		switch ch.Role {
		case types.Role3H_P1:
			result.P1 = val
			gotP1 = true
		case types.Role3H_P2:
			result.P2 = val
			gotP2 = true
		case types.Role3H_P3:
			result.P3 = val
			gotP3 = true
		case types.Role3H_PAtm:
			result.PAtm = val
			gotPAtm = true
		case types.Role3H_TAtm:
			result.TAtm = val
		}
	}

	if !gotP1 || !gotP2 || !gotP3 || !gotPAtm {
		// 节流：首次缺数据打一次 WARN（含缺失通道列表），之后完全静默；数据恢复后重置
		if dp.noDataWarned.CompareAndSwap(false, true) {
			slog.Warn("readRawData: missing required channel data (subsequent warnings suppressed until recovered)",
				"P1", gotP1, "P2", gotP2, "P3", gotP3, "PAtm", gotPAtm,
				"missingChannels", missingChannels)
		}
		return nil
	}

	// 数据完整，重置告警标记，下次再失败可再打一次 WARN
	dp.noDataWarned.Store(false)
	return result
}

// calculateThreeHoleAverage 计算多次采样的平均值（3σ 异常值剔除）
func calculateThreeHoleAverage(samples []types.ThreeHoleRawData) types.ThreeHoleRawData {
	if len(samples) == 0 {
		return types.ThreeHoleRawData{}
	}
	if len(samples) < 4 {
		return simpleAverage(samples)
	}

	// 对每个字段做 3σ 剔除后取均值
	return types.ThreeHoleRawData{
		P1:   outlierFilteredAvg(mapField(samples, func(s types.ThreeHoleRawData) float64 { return s.P1 })),
		P2:   outlierFilteredAvg(mapField(samples, func(s types.ThreeHoleRawData) float64 { return s.P2 })),
		P3:   outlierFilteredAvg(mapField(samples, func(s types.ThreeHoleRawData) float64 { return s.P3 })),
		PAtm: outlierFilteredAvg(mapField(samples, func(s types.ThreeHoleRawData) float64 { return s.PAtm })),
		TAtm: outlierFilteredAvg(mapField(samples, func(s types.ThreeHoleRawData) float64 { return s.TAtm })),
	}
}

func simpleAverage(samples []types.ThreeHoleRawData) types.ThreeHoleRawData {
	n := float64(len(samples))
	result := types.ThreeHoleRawData{}
	for _, s := range samples {
		result.P1 += s.P1
		result.P2 += s.P2
		result.P3 += s.P3
		result.PAtm += s.PAtm
		result.TAtm += s.TAtm
	}
	result.P1 /= n
	result.P2 /= n
	result.P3 /= n
	result.PAtm /= n
	result.TAtm /= n
	return result
}

func mapField(samples []types.ThreeHoleRawData, fn func(types.ThreeHoleRawData) float64) []float64 {
	vals := make([]float64, len(samples))
	for i, s := range samples {
		vals[i] = fn(s)
	}
	return vals
}

// outlierFilteredAvg 3σ 异常值剔除后取均值，若剔除后不足2个则回退到全量均值
func outlierFilteredAvg(vals []float64) float64 {
	n := len(vals)
	if n == 0 {
		return 0
	}

	// 计算均值
	mean := 0.0
	for _, v := range vals {
		mean += v
	}
	mean /= float64(n)

	// 计算标准差
	variance := 0.0
	for _, v := range vals {
		d := v - mean
		variance += d * d
	}
	variance /= float64(n)
	stdDev := 0.0
	if variance > 0 {
		stdDev = math.Sqrt(variance)
	}

	// 剔除异常值
	lo := mean - 3*stdDev
	hi := mean + 3*stdDev

	sum := 0.0
	cnt := 0
	for _, v := range vals {
		if v >= lo && v <= hi {
			sum += v
			cnt++
		}
	}

	// 如果剔除后不足2个，回退到全量均值
	if cnt < 2 {
		return mean
	}
	return sum / float64(cnt)
}