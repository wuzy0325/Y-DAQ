package five_hole

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"yx-daq/internal/types"
)

// ==================== 服务 ====================

// FiveHoleTraversalService 五孔移位测试服务
// 照三孔 ThreeHoleTraversalService，但支持多探针：
// - interpolators map[probeID]*FiveHoleInterpolator（每探针独立校准）
// - calibInfos map[probeID][]FiveHoleCalibFileInfo
// - 实时监控协程（100ms ticker）读所有探针实时数据 + 插值 + 发射 realtime 事件
type FiveHoleTraversalService struct {
	mu sync.RWMutex

	interpolators map[string]*FiveHoleInterpolator
	calibInfos   map[string][]types.FiveHoleCalibFileInfo

	motionCoordinator *MotionCoordinator
	probeAxisMover    FiveHoleProbeAxisMover
	probeAxisWaiter   FiveHoleProbeAxisWaiter
	dataProcessor     *DataProcessor
	eventHandler      *EventHandler
	testManager       *TestManager
	realtimeRecorder  *RealtimeRecorder

	// 实时监控相关
	monitorRunning atomic.Bool
	monitorCtx     context.Context
	monitorCancel  context.CancelFunc
	monitorConfig  types.FiveHoleTraversalConfig
	monitorWg      sync.WaitGroup // 等待监控 goroutine 退出，避免 Stop+Start 快速切换时 race
	// 标记是否正在执行测试，避免监控和测试数据冲突
	testRunning    atomic.Bool

	eventPublisher FiveHoleEventPublisher
}

// NewFiveHoleTraversalService 创建五孔移位测试服务
// 初始化 testManager、dataProcessor、realtimeRecorder、空 interpolators map
func NewFiveHoleTraversalService(publisher FiveHoleEventPublisher) *FiveHoleTraversalService {
	testManager := NewTestManager(publisher)
	dataProcessor := NewDataProcessor()
	realtimeRecorder := NewRealtimeRecorder()

	// 占位运动协调器（mover/waiter 为 nil，SetProbeAxisMover/SetProbeAxisWaiter 时重建）
	motionCoordinator := NewMotionCoordinator(nil, nil)

	service := &FiveHoleTraversalService{
		interpolators:     make(map[string]*FiveHoleInterpolator),
		calibInfos:        make(map[string][]types.FiveHoleCalibFileInfo),
		testManager:       testManager,
		dataProcessor:     dataProcessor,
		realtimeRecorder:  realtimeRecorder,
		motionCoordinator: motionCoordinator,
		eventPublisher:    publisher,
	}

	// 初始化事件处理器
	service.eventHandler = NewEventHandler(testManager, dataProcessor, publisher)

	// 实时监控 ctx 初始为已取消
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	service.monitorCtx = ctx
	service.monitorCancel = cancel

	return service
}

// SetMultiDeviceBatchGetter 设置多设备批量通道数据获取函数（委托 dataProcessor）
func (s *FiveHoleTraversalService) SetMultiDeviceBatchGetter(getter FiveHoleMultiDeviceBatchGetter) {
	s.dataProcessor.SetBatchGetter(getter)
}

// SetProbeAxisMover 设置探针单轴运动控制函数（创建 MotionCoordinator 并保存）
// 注意：仅可在服务初始化阶段（runTestLoop 启动前）调用。
// 运行期热替换未对 motionCoordinator 字段做同步保护，运行中调用会与 runTestLoop 产生 data race。
func (s *FiveHoleTraversalService) SetProbeAxisMover(mover FiveHoleProbeAxisMover) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.probeAxisMover = mover
	s.motionCoordinator = NewMotionCoordinator(mover, s.probeAxisWaiter)
}

// SetProbeAxisWaiter 设置探针单轴运动等待函数（更新 MotionCoordinator）
// 注意：仅可在服务初始化阶段（runTestLoop 启动前）调用。
// 运行期热替换未对 motionCoordinator 字段做同步保护，运行中调用会与 runTestLoop 产生 data race。
func (s *FiveHoleTraversalService) SetProbeAxisWaiter(waiter FiveHoleProbeAxisWaiter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.probeAxisWaiter = waiter
	s.motionCoordinator = NewMotionCoordinator(s.probeAxisMover, waiter)
}

// SetEventPublisher 设置事件发布器（可选，便于测试替换）
func (s *FiveHoleTraversalService) SetEventPublisher(publisher FiveHoleEventPublisher) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventPublisher = publisher
	s.testManager.eventPublisher = publisher
	s.eventHandler.eventPublisher = publisher
}

// LoadCalibFiles 为指定探针创建/获取 interpolator 并加载校准文件
func (s *FiveHoleTraversalService) LoadCalibFiles(probeID string, filePaths []string) error {
	if probeID == "" {
		return fmt.Errorf("probeID 不能为空")
	}

	s.mu.Lock()
	interpolator, ok := s.interpolators[probeID]
	if !ok {
		interpolator = NewFiveHoleInterpolator()
		s.interpolators[probeID] = interpolator
	}
	s.mu.Unlock()

	if err := interpolator.LoadCalibFiles(filePaths); err != nil {
		return fmt.Errorf("探针%s 加载校准文件失败: %w", probeID, err)
	}

	s.mu.Lock()
	s.calibInfos[probeID] = interpolator.GetCalibInfo()
	s.mu.Unlock()

	return nil
}

// IsCalibLoaded 指定探针的校准文件是否已加载
func (s *FiveHoleTraversalService) IsCalibLoaded(probeID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	interpolator, ok := s.interpolators[probeID]
	if !ok {
		return false
	}
	return interpolator.IsLoaded()
}

// GetCalibInfo 获取指定探针的校准文件信息
func (s *FiveHoleTraversalService) GetCalibInfo(probeID string) []types.FiveHoleCalibFileInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.calibInfos[probeID]
}

// ==================== 实时监控（测试未运行时也推送实时数据） ====================

// StartRealtimeMonitor 启动实时数据监控
// 启动 100ms ticker 协程读所有探针实时数据 + 插值 + 发射 realtime 事件（含所有探针数据）
// 已在运行时仅更新配置；否则等待旧 goroutine 退出后再启动新的，避免新旧 goroutine 并发发射事件
func (s *FiveHoleTraversalService) StartRealtimeMonitor(config types.FiveHoleTraversalConfig) {
	// 等待旧 goroutine 完全退出，避免 Stop+Start 快速切换时新旧 goroutine 短暂并存
	s.monitorWg.Wait()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.monitorRunning.Load() {
		// 已在运行：仅更新配置（旧 goroutine 下次迭代自动用新配置）
		s.monitorConfig = config
		return
	}

	s.monitorConfig = config
	s.monitorCtx, s.monitorCancel = context.WithCancel(context.Background())
	s.monitorRunning.Store(true)
	s.monitorWg.Add(1)
	go s.runRealtimeMonitor()
}

// StopRealtimeMonitor 停止实时数据监控
func (s *FiveHoleTraversalService) StopRealtimeMonitor() {
	s.mu.Lock()
	if s.monitorCancel != nil {
		s.monitorCancel()
	}
	s.mu.Unlock()
	// 等待 goroutine 退出，确保下次 Start 时旧 goroutine 已彻底清理
	s.monitorWg.Wait()
}

// runRealtimeMonitor 实时监控协程
func (s *FiveHoleTraversalService) runRealtimeMonitor() {
	defer s.monitorWg.Done()
	defer s.monitorRunning.Store(false)

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.monitorCtx.Done():
			return
		case <-ticker.C:
		}

		// 如果正在执行测试，监控不推送数据（避免重复）
		if s.testRunning.Load() {
			continue
		}

		s.mu.RLock()
		config := s.monitorConfig
		s.mu.RUnlock()

		// 配置未完成（PAtm/TAtm 设备未选）时静默跳过，避免每 100ms 刷错误日志
		// 这是用户尚未完成配置的正常状态，不应视为错误
		if config.PAtmDeviceID == "" || config.TAtmDeviceID == "" {
			continue
		}
		// 没有启用探针也跳过
		hasEnabled := false
		for _, p := range config.Probes {
			if p.Enabled {
				hasEnabled = true
				break
			}
		}
		if !hasEnabled {
			continue
		}

		s.emitRealtimeForAllProbes("monitor", "realtime", "", config)
	}
}

// StartRealtimeRecording 开始实时数据录制
func (s *FiveHoleTraversalService) StartRealtimeRecording(savePath string) error {
	return s.realtimeRecorder.Start(savePath)
}

// StopRealtimeRecording 停止实时数据录制
func (s *FiveHoleTraversalService) StopRealtimeRecording() {
	s.realtimeRecorder.Stop()
}

// IsRealtimeRecording 是否正在录制实时数据
func (s *FiveHoleTraversalService) IsRealtimeRecording() bool {
	return s.realtimeRecorder.IsRecording()
}

// ==================== 测试生命周期 ====================

// Start 启动测试
// 校验 config.Validate()、每探针 IsCalibLoaded、初始化 csvWriters、testManager.Start、go runTestLoop
func (s *FiveHoleTraversalService) Start(config types.FiveHoleTraversalConfig) (string, error) {
	// 验证配置
	if err := config.Validate(); err != nil {
		return "", fmt.Errorf("配置验证失败: %w", err)
	}

	// 检查每探针校准文件是否已加载
	for _, p := range config.Probes {
		if !p.Enabled {
			continue
		}
		if !s.IsCalibLoaded(p.ProbeID) {
			return "", fmt.Errorf("探针%s 校准文件未载入", p.ProbeID)
		}
	}

	// 标记测试开始
	s.testRunning.Store(true)

	// 初始化 CSV 写入器
	if err := s.eventHandler.OnTestStart(config); err != nil {
		s.testRunning.Store(false)
		return "", err
	}

	// 启动测试
	taskID, err := s.testManager.Start(config)
	if err != nil {
		// 回滚 CSV 文件：OnTestStart 已为每个启用探针创建 csvWriter，
		// testManager.Start 失败时测试循环不会启动，需要手动关闭避免空 CSV 残留
		s.eventHandler.CloseCSVWriters()
		s.eventHandler.OnFatalError(fmt.Sprintf("启动测试失败: %v", err))
		s.testRunning.Store(false)
		return "", err
	}

	// 启动测试协程
	doneCloseOnce := &sync.Once{}
	go func() {
		s.runTestLoop(taskID, config)
		doneCloseOnce.Do(func() {
			s.testManager.CloseDoneCh()
		})
	}()

	return taskID, nil
}

// Pause 暂停测试
func (s *FiveHoleTraversalService) Pause() {
	s.testManager.Pause()
}

// Resume 恢复测试
func (s *FiveHoleTraversalService) Resume() {
	s.testManager.Resume()
}

// Stop 停止测试
func (s *FiveHoleTraversalService) Stop() {
	s.testManager.Stop()

	time.Sleep(100 * time.Millisecond)

	s.testRunning.Store(false)
	s.StopRealtimeMonitor()
}

// GetStatus 获取测试状态
func (s *FiveHoleTraversalService) GetStatus() types.FiveHoleTraversalTaskStatus {
	return s.testManager.GetStatus()
}

// GetConfig 获取当前测试配置
func (s *FiveHoleTraversalService) GetConfig() types.FiveHoleTraversalConfig {
	return s.testManager.GetConfig()
}

// ==================== 测试主循环 ====================

// runTestLoop 测试主循环
// 对每个布点：
//  1. MotionCoordinator.MoveAllProbesToPoint 移动所有探针
//  2. 发射 progress（phase=moving）
//  3. 驻留 config.DwellTimeMs，期间每 100ms 发射 realtime（phase=waiting）
//  4. 采样 config.SamplesPerPoint 次，每次间隔 config.SampleIntervalMs
//  5. 每探针 OutlierFilteredAvg 各压力通道 → FiveHoleRawData → Calculate → DataPoint
//  6. 每探针 csvWriter.AppendPoint + eventHandler.OnDataPointAcquired
//  7. 更新统一进度 +1，发射 progress（phase=completed）
//  8. 检查 Pause/Cancel
//  完成后 eventHandler.OnTestComplete
func (s *FiveHoleTraversalService) runTestLoop(taskID string, config types.FiveHoleTraversalConfig) {
	points, err := generatePoints(config.Layout)
	if err != nil {
		s.eventHandler.OnFatalError(fmt.Sprintf("生成布点失败: %v", err))
		return
	}
	totalPoints := len(points)

	s.testManager.EmitProgress(taskID, totalPoints, 0, 0, 0, 0, "starting")

	defer func() { s.eventHandler.OnTestComplete(taskID, s.testManager.GetStatus().Status) }()

	// 保存当前代际号，用于检测是否被新测试取代
	myGen := s.testManager.testGen.Load()

	// 预计算启用探针索引
	enabledIndices := make([]int, 0, len(config.Probes))
	for i, p := range config.Probes {
		if p.Enabled {
			enabledIndices = append(enabledIndices, i)
		}
	}

	completed := 0
	for _, point := range points {
		if s.testManager.ctx.Err() != nil {
			return
		}

		// 检测是否已被新 Start() 取代
		if s.testManager.testGen.Load() != myGen {
			return
		}

		// 处理暂停状态
		for s.testManager.paused.Load() {
			select {
			case <-s.testManager.ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
			}
		}

		// 更新当前点位信息
		s.testManager.mu.Lock()
		s.testManager.status.CurrentPoint = &point
		s.testManager.mu.Unlock()

		// 更新探针 phase=moving
		for _, idx := range enabledIndices {
			s.testManager.UpdateProbeStatus(config.Probes[idx].ProbeID, "moving", point.X, point.Y)
		}

		// 1. 移动所有启用探针到指定点位
		if err := s.motionCoordinator.MoveAllProbesToPoint(point, config.Probes, config.Layout, config.MotionTimeoutMs); err != nil {
			if !s.testManager.running.Load() {
				return
			}
			s.eventHandler.OnTestError(point.ID, err)
			continue
		}

		// 检查取消
		if s.testManager.ctx.Err() != nil {
			return
		}

		// 2. 发射 progress（phase=moving）
		s.testManager.EmitProgress(taskID, totalPoints, completed, s.testManager.GetProgress(), point.X, point.Y, "moving")

		// 3. 驻留 config.DwellTimeMs，期间每 100ms 发射 realtime（phase=waiting）
		s.dwellWithRealtime(taskID, point, config)

		// 检查取消
		if s.testManager.ctx.Err() != nil {
			return
		}

		// 4. 采样 config.SamplesPerPoint 次
		probeSamples := make(map[string][]types.FiveHoleRawData)
		var lastTimestamps map[string]int64 // 首次采样不等待新帧

		for i := 0; i < config.SamplesPerPoint; i++ {
			if err := s.testManager.CheckCancelled(); err != nil {
				return
			}
			// 暂停期间持续推送 realtime 让用户观察设备实时状态以判断故障原因
			s.waitForResumeWithRealtime(taskID, point, config)
			if err := s.testManager.CheckCancelled(); err != nil {
				return
			}

			// 首次采样立即取当前帧；后续采样等待所有设备产生新帧（2 秒超时）
			if i > 0 && lastTimestamps != nil {
				deviceIDs := collectDeviceIDs(config)
				newTs, err := s.dataProcessor.WaitForFreshData(deviceIDs, lastTimestamps, 2*time.Second)
				if err != nil {
					if errors.Is(err, ErrDataStagnant) {
						// 设备数据停滞：自动暂停，等待用户恢复后重新采样当前点位
						s.testManager.Pause()
						s.eventHandler.OnTestError(point.ID, fmt.Errorf("点位(%.2f,%.2f) 采集设备数据停滞，已自动暂停，请检查设备后点击恢复: %w", point.X, point.Y, err))
						// 暂停期间持续推送 realtime 让用户观察设备状态以判断故障原因
						s.waitForResumeWithRealtime(taskID, point, config)
						if err := s.testManager.CheckCancelled(); err != nil {
							return
						}
						// 恢复后重新从第一次采样开始当前点位
						probeSamples = make(map[string][]types.FiveHoleRawData)
						lastTimestamps = nil
						i = -1 // for 循环 i++ 后变为 0
						continue
					}
					s.eventHandler.OnTestError(point.ID, err)
					break
				}
				lastTimestamps = newTs
			}

			// 读取所有探针原始数据
			rawDatas, currentTs, err := s.dataProcessor.ReadAllProbesRawData(
				config.Probes,
				config.PAtmDeviceID, config.PAtmChannel,
				config.TAtmDeviceID, config.TAtmChannel,
				lastTimestamps,
			)
			if err != nil {
				s.eventHandler.OnTestError(point.ID, err)
				break
			}
			lastTimestamps = currentTs

			// 每探针收集样本 + 构建 realtime items
			realtimeItems := make([]types.FiveHoleProbeRealtimeItem, 0, len(enabledIndices))
			for _, idx := range enabledIndices {
				probe := config.Probes[idx]
				rawData := rawDatas[idx]
				probeSamples[probe.ProbeID] = append(probeSamples[probe.ProbeID], rawData)

				// 插值（占位，结果 invalid）
				interp := s.calculateForProbe(probe.ProbeID, rawData)
				realtimeItems = append(realtimeItems, types.FiveHoleProbeRealtimeItem{
					ProbeID:      probe.ProbeID,
					RawData:      rawData,
					InterpResult: interp,
				})
				// 更新探针实时数据
				s.testManager.UpdateProbeData(probe.ProbeID, &rawData, &interp)
			}

			// 每次采样发射 realtime（phase=acquiring）
			s.emitAndRecordRealtime(types.FiveHoleTraversalRealtimeEvent{
				TaskID:        taskID,
				PointID:       point.ID,
				Phase:         "acquiring",
				ProbeRealtime: realtimeItems,
			})

			// 采样间隔（仅作节流上限，实际节奏由 WaitForFreshData 决定）
			intervalMs := config.SampleIntervalMs
			if intervalMs <= 0 {
				intervalMs = 50
			}
			time.Sleep(time.Duration(intervalMs) * time.Millisecond)
		}

		// 5. 每探针：OutlierFilteredAvg 各压力通道 → FiveHoleRawData → Calculate → DataPoint
		for _, idx := range enabledIndices {
			probe := config.Probes[idx]
			samples := probeSamples[probe.ProbeID]
			if len(samples) == 0 {
				s.eventHandler.OnTestError(point.ID, fmt.Errorf("探针%s 无有效样本", probe.ProbeID))
				continue
			}

			avgData := types.FiveHoleRawData{
				P1:   OutlierFilteredAvg(mapField5H(samples, func(s types.FiveHoleRawData) float64 { return s.P1 })),
				P2:   OutlierFilteredAvg(mapField5H(samples, func(s types.FiveHoleRawData) float64 { return s.P2 })),
				P3:   OutlierFilteredAvg(mapField5H(samples, func(s types.FiveHoleRawData) float64 { return s.P3 })),
				P4:   OutlierFilteredAvg(mapField5H(samples, func(s types.FiveHoleRawData) float64 { return s.P4 })),
				P5:   OutlierFilteredAvg(mapField5H(samples, func(s types.FiveHoleRawData) float64 { return s.P5 })),
				PAtm: OutlierFilteredAvg(mapField5H(samples, func(s types.FiveHoleRawData) float64 { return s.PAtm })),
				TAtm: OutlierFilteredAvg(mapField5H(samples, func(s types.FiveHoleRawData) float64 { return s.TAtm })),
			}

			// 对平均数据执行插值（占位，结果 invalid）
			interp := s.calculateForProbe(probe.ProbeID, avgData)

			dataPoint := types.FiveHoleTraversalDataPoint{
				PointID:      point.ID,
				ProbeID:      probe.ProbeID,
				X:            point.X,
				Y:            point.Y,
				RawData:      avgData,
				InterpResult: interp,
				SampleCount:  len(samples),
				Timestamp:    time.Now().UnixMilli(),
			}

			// 6. csvWriter.AppendPoint + eventHandler.OnDataPointAcquired
			if err := s.eventHandler.OnDataPointAcquired(probe.ProbeID, dataPoint); err != nil {
				slog.Error("处理数据点失败", "probe", probe.ProbeID, "point", point.ID, "err", err)
				// 如果是致命错误，停止测试
				if s.testManager.running.Load() {
					s.testManager.EmitFatalError(fmt.Sprintf("处理数据点失败: %v", err))
				}
				return
			}
		}

		// 7. 更新统一进度 +1
		completed++
		progress := float64(completed) / float64(totalPoints) * 100
		s.testManager.UpdateProgress(completed, totalPoints, &point)

		// 更新探针 phase=completed
		for _, idx := range enabledIndices {
			s.testManager.UpdateProbeStatus(config.Probes[idx].ProbeID, "completed", point.X, point.Y)
		}
		s.testManager.EmitProgress(taskID, totalPoints, completed, progress, point.X, point.Y, "completed")

		// 8. 检查 Pause/Cancel
		if s.testManager.ctx.Err() != nil {
			return
		}
	}
}

// dwellWithRealtime 驻留等待期间持续推送实时数据（phase=waiting）
func (s *FiveHoleTraversalService) dwellWithRealtime(taskID string, point types.TraversalPoint, config types.FiveHoleTraversalConfig) {
	// 更新探针 phase=waiting
	for _, p := range config.Probes {
		if !p.Enabled {
			continue
		}
		s.testManager.UpdateProbeStatus(p.ProbeID, "waiting", point.X, point.Y)
	}
	s.testManager.EmitProgress(taskID, s.testManager.GetTotalPoints(), s.testManager.GetCompletedPoints(), s.testManager.GetProgress(), point.X, point.Y, "waiting")

	dwellDuration := time.Duration(config.DwellTimeMs) * time.Millisecond
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	deadline := time.Now().Add(dwellDuration)

	for {
		select {
		case <-s.testManager.ctx.Done():
			return
		case <-ticker.C:
		}

		// 处理暂停（暂停期间延长 deadline）
		pauseStart := time.Time{}
		for s.testManager.paused.Load() {
			if pauseStart.IsZero() {
				pauseStart = time.Now()
			}
			select {
			case <-s.testManager.ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
			}
		}
		if !pauseStart.IsZero() {
			deadline = deadline.Add(time.Since(pauseStart))
			pauseStart = time.Time{}
		}

		// 推送实时数据
		s.emitRealtimeForAllProbes(taskID, point.ID, "waiting", config)

		if time.Now().After(deadline) {
			return
		}
	}
}

// waitForResumeWithRealtime 阻塞等待暂停解除，期间持续推送 realtime（phase=paused）
// 用于采样阶段的暂停（用户主动暂停或数据停滞自动暂停），让 UI 能实时观察设备状态以判断故障原因
func (s *FiveHoleTraversalService) waitForResumeWithRealtime(taskID string, point types.TraversalPoint, config types.FiveHoleTraversalConfig) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for s.testManager.paused.Load() {
		select {
		case <-s.testManager.ctx.Done():
			return
		case <-ticker.C:
			s.emitRealtimeForAllProbes(taskID, point.ID, "paused", config)
		}
	}
}

// emitRealtimeForAllProbes 读取所有探针实时数据 + 插值 + 发射 realtime 事件（含所有探针数据）
// 实时监控场景不使用 timestamp 去重，每次都取当前最新帧
func (s *FiveHoleTraversalService) emitRealtimeForAllProbes(taskID, pointID, phase string, config types.FiveHoleTraversalConfig) {
	rawDatas, _, err := s.dataProcessor.ReadAllProbesRawData(
		config.Probes,
		config.PAtmDeviceID, config.PAtmChannel,
		config.TAtmDeviceID, config.TAtmChannel,
		nil,
	)
	if err != nil {
		// 实时监控场景下瞬时读取失败（设备未连接/通道暂无数据）较常见，
		// 降级为 debug 日志避免刷屏；测试主循环中的读取失败由调用方处理
		slog.Debug("realtime read raw data skipped", "err", err)
		return
	}

	items := make([]types.FiveHoleProbeRealtimeItem, 0, len(config.Probes))
	for i, p := range config.Probes {
		if !p.Enabled {
			continue
		}
		rawData := rawDatas[i]
		interp := s.calculateForProbe(p.ProbeID, rawData)
		items = append(items, types.FiveHoleProbeRealtimeItem{
			ProbeID:      p.ProbeID,
			RawData:      rawData,
			InterpResult: interp,
		})
	}

	evt := types.FiveHoleTraversalRealtimeEvent{
		TaskID:        taskID,
		PointID:       pointID,
		Phase:         phase,
		ProbeRealtime: items,
	}
	s.emitAndRecordRealtime(evt)
}

// emitAndRecordRealtime 发射 realtime 事件并录制
func (s *FiveHoleTraversalService) emitAndRecordRealtime(evt types.FiveHoleTraversalRealtimeEvent) {
	s.testManager.EmitRealtime(evt)
	s.realtimeRecorder.Record(evt)
}

// calculateForProbe 对指定探针执行插值（占位算法，结果 invalid）
func (s *FiveHoleTraversalService) calculateForProbe(probeID string, rawData types.FiveHoleRawData) types.FiveHoleInterpolationResult {
	s.mu.RLock()
	interpolator, ok := s.interpolators[probeID]
	s.mu.RUnlock()
	if !ok || interpolator == nil {
		return types.FiveHoleInterpolationResult{
			Valid:    false,
			ErrorMsg: "探针校准未载入",
		}
	}
	return interpolator.Calculate(rawData)
}

// mapField5H 提取样本切片中指定字段为 float64 切片（用于 OutlierFilteredAvg）
func mapField5H(samples []types.FiveHoleRawData, fn func(types.FiveHoleRawData) float64) []float64 {
	vals := make([]float64, len(samples))
	for i, s := range samples {
		vals[i] = fn(s)
	}
	return vals
}

// collectDeviceIDs 收集配置中涉及的所有设备 ID（去重），用于 WaitForFreshData
func collectDeviceIDs(config types.FiveHoleTraversalConfig) []string {
	seen := make(map[string]struct{})
	var ids []string
	add := func(id string) {
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	add(config.PAtmDeviceID)
	add(config.TAtmDeviceID)
	for _, probe := range config.Probes {
		if !probe.Enabled {
			continue
		}
		for _, ch := range probe.ProbeChannels {
			if ch.Enabled {
				add(ch.DeviceID)
			}
		}
	}
	return ids
}
