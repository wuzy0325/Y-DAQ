package three_hole

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"yx-daq/internal/types"
)

// ==================== 依赖接口 ====================

// ThreeHoleBatchGetter 批量通道数据获取
type ThreeHoleBatchGetter func(channels []types.ThreeHoleProbeChannelConfig) (map[int]float64, error)

// ThreeHoleMotionController 运动控制函数
type ThreeHoleMotionController func(axis types.AxisName, position float64) error

// ThreeHoleEventPublisher 事件发布接口
type ThreeHoleEventPublisher interface {
	EmitProgress(event types.ThreeHoleTraversalProgressEvent)
	EmitRealtime(event types.ThreeHoleTraversalRealtimeEvent)
	EmitComplete(event types.ThreeHoleTraversalCompleteEvent)
	EmitError(event types.ThreeHoleTraversalErrorEvent)
}

// ==================== 服务 ====================

// ThreeHoleTraversalService 三孔移位测试服务
type ThreeHoleTraversalService struct {
	mu             sync.Mutex
	status         types.ThreeHoleTraversalTaskStatus
	running        atomic.Bool
	paused         atomic.Bool
	testGen        atomic.Int64 // 每 Start() 递增，旧 goroutine 退出时检测以防止干扰新 goroutine
	cancelCh       chan struct{}
	pauseCh        chan struct{}
	resumeCh       chan struct{}
	doneCh         chan struct{}

	config         types.ThreeHoleTraversalConfig
	interpolator   *ThreeHoleInterpolator
	batchGetter    ThreeHoleBatchGetter
	motionCtrl     ThreeHoleMotionController
	eventPublisher ThreeHoleEventPublisher

	// 实时监控（测试未运行时也推送实时数据）
	monitorRunning atomic.Bool
	monitorCancel  chan struct{}
}

// NewThreeHoleTraversalService 创建三孔移位测试服务
func NewThreeHoleTraversalService(publisher ThreeHoleEventPublisher) *ThreeHoleTraversalService {
	return &ThreeHoleTraversalService{
		interpolator:   NewThreeHoleInterpolator(),
		eventPublisher: publisher,
		cancelCh:      make(chan struct{}),
		pauseCh:       make(chan struct{}),
		resumeCh:      make(chan struct{}),
	}
}

// SetBatchGetter 设置批量数据获取函数
func (s *ThreeHoleTraversalService) SetBatchGetter(getter ThreeHoleBatchGetter) {
	s.batchGetter = getter
}

// SetMotionController 设置运动控制函数
func (s *ThreeHoleTraversalService) SetMotionController(ctrl ThreeHoleMotionController) {
	s.motionCtrl = ctrl
}

// LoadCalibFiles 加载校准文件
func (s *ThreeHoleTraversalService) LoadCalibFiles(filePaths []string) error {
	return s.interpolator.LoadCalibFiles(filePaths)
}

// IsCalibLoaded 校准文件是否已加载
func (s *ThreeHoleTraversalService) IsCalibLoaded() bool {
	return s.interpolator.IsLoaded()
}

// GetCalibInfo 获取校准文件信息
func (s *ThreeHoleTraversalService) GetCalibInfo() []types.ThreeHoleCalibFileInfo {
	return s.interpolator.GetCalibInfo()
}

// ==================== 实时监控（测试未运行时也推送实时数据） ====================

// StartRealtimeMonitor 启动实时数据监控
// 当采集设备在运行但测试未启动时，持续读取原始数据并推送 three-hole:realtime 事件
func (s *ThreeHoleTraversalService) StartRealtimeMonitor(config types.ThreeHoleTraversalConfig) {
	if s.monitorRunning.Load() {
		return
	}
	s.config = config
	s.monitorCancel = make(chan struct{})
	s.monitorRunning.Store(true)
	go s.runRealtimeMonitor()
}

// StopRealtimeMonitor 停止实时数据监控
func (s *ThreeHoleTraversalService) StopRealtimeMonitor() {
	if !s.monitorRunning.Load() {
		return
	}
	s.monitorRunning.Store(false)
	select {
	case s.monitorCancel <- struct{}{}:
	default:
	}
}

// runRealtimeMonitor 实时监控协程
func (s *ThreeHoleTraversalService) runRealtimeMonitor() {
	defer s.monitorRunning.Store(false)

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.monitorCancel:
			return
		case <-ticker.C:
		}

		// 测试运行期间不需要监控推送（测试自身会推送）
		if s.running.Load() {
			continue
		}

		// 读取原始数据
		rawData := s.readRawData()
		if rawData == nil || s.eventPublisher == nil {
			continue
		}

		// 计算插值（如果校准文件已加载）
		interpResult := s.interpolator.Calculate(*rawData)

		s.eventPublisher.EmitRealtime(types.ThreeHoleTraversalRealtimeEvent{
			TaskID:      "monitor",
			PointID:     "realtime",
			RawData:     *rawData,
			InterpResult: interpResult,
		})
	}
}

// ==================== 测试生命周期 ====================

// Start 启动测试
func (s *ThreeHoleTraversalService) Start(config types.ThreeHoleTraversalConfig) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch s.status.Status {
	case types.TraversalStatusRunning, types.TraversalStatusPaused:
		return "", fmt.Errorf("test already running")
	}

	if !s.interpolator.IsLoaded() {
		return "", fmt.Errorf("calibration files not loaded")
	}

	taskID := fmt.Sprintf("3h-traversal-%d", time.Now().UnixMilli())
	s.config = config
	cancelCh := make(chan struct{}, 1)
	s.cancelCh = cancelCh
	s.pauseCh = make(chan struct{})
	s.resumeCh = make(chan struct{})
	doneCh := make(chan struct{})
	s.doneCh = doneCh

	points := generatePoints(config.Layout)

	s.status = types.ThreeHoleTraversalTaskStatus{
		TaskID:      taskID,
		Status:      types.TraversalStatusRunning,
		TotalPoints: len(points),
		DataPoints:  []types.ThreeHoleTraversalDataPoint{},
	}

	s.running.Store(true)
	s.paused.Store(false)

	myGen := s.testGen.Add(1)
	go s.runTestLoop(points, cancelCh, doneCh, myGen)

	return taskID, nil
}

// Pause 暂停测试
func (s *ThreeHoleTraversalService) Pause() {
	s.mu.Lock()
	if s.status.Status != types.TraversalStatusRunning {
		s.mu.Unlock()
		return
	}
	s.status.Status = types.TraversalStatusPaused
	s.mu.Unlock()
	s.paused.Store(true)
}

// Resume 恢复测试
func (s *ThreeHoleTraversalService) Resume() {
	s.mu.Lock()
	if s.status.Status != types.TraversalStatusPaused {
		s.mu.Unlock()
		return
	}
	s.status.Status = types.TraversalStatusRunning
	s.mu.Unlock()
	s.paused.Store(false)
	select {
	case s.resumeCh <- struct{}{}:
	default:
	}
}

// Stop 停止测试
func (s *ThreeHoleTraversalService) Stop() {
	s.mu.Lock()
	switch s.status.Status {
	case types.TraversalStatusRunning, types.TraversalStatusPaused, types.TraversalStatusError, types.TraversalStatusCompleted:
	default:
		s.mu.Unlock()
		return
	}
	doneCh := s.doneCh
	cancelCh := s.cancelCh
	s.mu.Unlock()

	// 先置 running=false 让 goroutine 的 error 路径能立即退出
	s.running.Store(false)
	s.paused.Store(false)

	// buffered cancelCh 保底：即使 goroutine 正在阻塞调用，信号也不会丢失
	select {
	case cancelCh <- struct{}{}:
	default:
	}

	if doneCh != nil {
		select {
		case <-doneCh:
		case <-time.After(5 * time.Second):
			log.Printf("warning: test goroutine did not exit within 5s")
		}
	}

	s.mu.Lock()
	s.status.Status = types.TraversalStatusIdle
	s.mu.Unlock()
}

// GetStatus 获取测试状态
func (s *ThreeHoleTraversalService) GetStatus() types.ThreeHoleTraversalTaskStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// GetConfig 获取当前测试配置
func (s *ThreeHoleTraversalService) GetConfig() types.ThreeHoleTraversalConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config
}

// ==================== 测试主循环 ====================

// runTestLoop 测试主循环
func (s *ThreeHoleTraversalService) runTestLoop(points []types.TraversalPoint, cancelCh chan struct{}, doneCh chan struct{}, myGen int64) {
	defer func() {
		// 只有当前 goroutine 的 gen 与服务级 gen 一致时才清除 running 标志
		// 防止 Stop() 超时后旧 goroutine 干扰新 goroutine 的运行标志
		if s.testGen.Load() == myGen {
			s.running.Store(false)
		}
		close(doneCh)
	}()

	csvWriter := NewThreeHoleCsvWriter()
	if err := csvWriter.Initialize(s.config.SavePath, s.config.SaveFileName); err != nil {
		log.Printf("csv init failed: %v", err)
		s.emitFatalError(fmt.Sprintf("CSV初始化失败: %v", err))
		return
	}
	defer csvWriter.Close()

	s.mu.Lock()
	taskID := s.status.TaskID
	s.mu.Unlock()

	for i, point := range points {
		select {
		case <-cancelCh:
			return
		default:
		}

		// 检测是否已被新 Start() 取代（Stop 超时后旧 goroutine 仍可能存活）
		if s.testGen.Load() != myGen {
			return
		}

		for s.paused.Load() {
			rawData := s.readRawData()
			if rawData != nil && s.eventPublisher != nil {
				interpResult := s.interpolator.Calculate(*rawData)
				s.eventPublisher.EmitRealtime(types.ThreeHoleTraversalRealtimeEvent{
					TaskID:      taskID,
					PointID:     point.ID,
					RawData:     *rawData,
					InterpResult: interpResult,
				})
			}
			select {
			case <-cancelCh:
				return
			case <-s.resumeCh:
				continue
			case <-time.After(100 * time.Millisecond):
			}
		}

		dataPoint, err := s.runSinglePoint(point, cancelCh)
		if err != nil {
			if !s.running.Load() {
				return
			}
			log.Printf("point %s failed: %v", point.ID, err)
			s.emitPointError(fmt.Sprintf("点位 %s 测试失败: %v", point.ID, err))
			continue
		}

		if err := csvWriter.AppendPoint(dataPoint); err != nil {
			log.Printf("csv write point %s failed: %v", point.ID, err)
		}

		s.mu.Lock()
		s.status.DataPoints = append(s.status.DataPoints, dataPoint)
		s.status.CompletedPoints = i + 1
		s.status.Progress = float64(i+1) / float64(s.status.TotalPoints) * 100
		s.status.CurrentPoint = &point
		s.mu.Unlock()

		if s.eventPublisher != nil {
			s.eventPublisher.EmitProgress(types.ThreeHoleTraversalProgressEvent{
				TaskID:          taskID,
				TotalPoints:     s.status.TotalPoints,
				CompletedPoints: s.status.CompletedPoints,
				Progress:        s.status.Progress,
				CurrentX:        point.X,
				CurrentY:        point.Y,
			})
		}
	}

	// 确保不是被新 Start() 取代的旧 goroutine
	if s.testGen.Load() != myGen {
		return
	}

	s.mu.Lock()
	s.status.Status = types.TraversalStatusCompleted
	s.mu.Unlock()

	if s.eventPublisher != nil {
		s.eventPublisher.EmitComplete(types.ThreeHoleTraversalCompleteEvent{
			TaskID:     taskID,
			Status:     types.TraversalStatusCompleted,
			DataPoints: s.status.DataPoints,
		})
	}
}

// emitPointPhase 推送点位阶段进度事件
func (s *ThreeHoleTraversalService) emitPointPhase(point types.TraversalPoint, phase string) {
	if s.eventPublisher == nil {
		return
	}
	s.mu.Lock()
	taskID := s.status.TaskID
	completedPoints := s.status.CompletedPoints
	totalPoints := s.status.TotalPoints
	progress := s.status.Progress
	s.mu.Unlock()

	s.eventPublisher.EmitProgress(types.ThreeHoleTraversalProgressEvent{
		TaskID:          taskID,
		TotalPoints:     totalPoints,
		CompletedPoints: completedPoints,
		Progress:        progress,
		CurrentX:        point.X,
		CurrentY:        point.Y,
		Phase:           phase,
	})
}

// checkCancelled 检查是否已被取消
func (s *ThreeHoleTraversalService) checkCancelled(cancelCh chan struct{}) error {
	select {
	case <-cancelCh:
		return fmt.Errorf("cancelled")
	default:
		return nil
	}
}

// waitWhilePaused 阻塞等待直到暂停解除（不监听 resumeCh，仅通过轮询 paused flag 退出）
// 仅供 runSinglePoint 使用，避免与 runTestLoop 外层 / dwell 竞争 resumeCh
func (s *ThreeHoleTraversalService) waitWhilePaused(cancelCh chan struct{}) {
	for s.paused.Load() {
		select {
		case <-cancelCh:
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// runSinglePoint 执行单点测试
func (s *ThreeHoleTraversalService) runSinglePoint(point types.TraversalPoint, cancelCh chan struct{}) (types.ThreeHoleTraversalDataPoint, error) {
	if err := s.checkCancelled(cancelCh); err != nil {
		return types.ThreeHoleTraversalDataPoint{}, err
	}

	s.emitPointPhase(point, "moving")
	if s.motionCtrl != nil {
		targetX := resolveTargetPosition(point.X, s.config.MotionX)
		if err := s.motionCtrl(s.config.MotionX.Axis, targetX); err != nil {
			return types.ThreeHoleTraversalDataPoint{}, fmt.Errorf("move X to %.2f failed: %w", targetX, err)
		}
		targetY := resolveTargetPosition(point.Y, s.config.MotionY)
		if err := s.motionCtrl(s.config.MotionY.Axis, targetY); err != nil {
			return types.ThreeHoleTraversalDataPoint{}, fmt.Errorf("move Y to %.2f failed: %w", targetY, err)
		}
	}

	s.waitWhilePaused(cancelCh)
	if err := s.checkCancelled(cancelCh); err != nil {
		return types.ThreeHoleTraversalDataPoint{}, err
	}

	s.emitPointPhase(point, "waiting")
	s.dwellWithRealtimeUpdate(point, cancelCh)

	s.emitPointPhase(point, "acquiring")
	return s.acquireAndInterpolate(point, cancelCh)
}

// dwellWithRealtimeUpdate 在驻留等待期间持续推送实时数据，避免UI卡顿
func (s *ThreeHoleTraversalService) dwellWithRealtimeUpdate(point types.TraversalPoint, cancelCh chan struct{}) {
	dwellDuration := time.Duration(s.config.DwellTimeMs) * time.Millisecond
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	s.mu.Lock()
	taskID := s.status.TaskID
	s.mu.Unlock()

	deadline := time.Now().Add(dwellDuration)
	for {
		select {
		case <-cancelCh:
			return
		case <-ticker.C:
		}

		pauseStart := time.Time{}
		for s.paused.Load() {
			if pauseStart.IsZero() {
				pauseStart = time.Now()
			}
			select {
			case <-cancelCh:
				return
			case <-s.resumeCh:
				if !pauseStart.IsZero() {
					pauseDur := time.Since(pauseStart)
					deadline = deadline.Add(pauseDur)
					pauseStart = time.Time{}
				}
			case <-time.After(100 * time.Millisecond):
			}
		}

		if s.eventPublisher != nil {
			rawData := s.readRawData()
			if rawData != nil {
				interpResult := s.interpolator.Calculate(*rawData)
				s.eventPublisher.EmitRealtime(types.ThreeHoleTraversalRealtimeEvent{
					TaskID:      taskID,
					PointID:     point.ID,
					RawData:     *rawData,
					InterpResult: interpResult,
				})
			}
		}

		if time.Now().After(deadline) {
			return
		}
	}
}

// resolveTargetPosition 角度→位置映射（含scale/offset）
func resolveTargetPosition(sourceValue float64, mapping types.MotionAxisMapping) float64 {
	return sourceValue*mapping.Scale + mapping.Offset
}

// acquireAndInterpolate 采集数据并执行插值
func (s *ThreeHoleTraversalService) acquireAndInterpolate(point types.TraversalPoint, cancelCh chan struct{}) (types.ThreeHoleTraversalDataPoint, error) {
	samples := []types.ThreeHoleRawData{}

	s.mu.Lock()
	taskID := s.status.TaskID
	s.mu.Unlock()

	for i := 0; i < s.config.SamplesPerPoint; i++ {
		if err := s.checkCancelled(cancelCh); err != nil {
			return types.ThreeHoleTraversalDataPoint{}, err
		}

		s.waitWhilePaused(cancelCh)
		if err := s.checkCancelled(cancelCh); err != nil {
			return types.ThreeHoleTraversalDataPoint{}, err
		}

		rawData := s.readRawData()
		if rawData == nil {
			continue
		}
		samples = append(samples, *rawData)

		if s.eventPublisher != nil {
			interpResult := s.interpolator.Calculate(*rawData)
			s.eventPublisher.EmitRealtime(types.ThreeHoleTraversalRealtimeEvent{
				TaskID:      taskID,
				PointID:     point.ID,
				RawData:     *rawData,
				InterpResult: interpResult,
			})
		}

		time.Sleep(50 * time.Millisecond)
	}

	if len(samples) == 0 {
		return types.ThreeHoleTraversalDataPoint{
			PointID:   point.ID,
			X:         point.X,
			Y:         point.Y,
			Timestamp: time.Now().UnixMilli(),
		}, fmt.Errorf("no samples collected")
	}

	// 计算平均原始数据
	avgData := calculateThreeHoleAverage(samples)

	// 对平均数据执行插值
	interpResult := s.interpolator.Calculate(avgData)

	return types.ThreeHoleTraversalDataPoint{
		PointID:      point.ID,
		X:            point.X,
		Y:            point.Y,
		RawData:      avgData,
		InterpResult: interpResult,
		SampleCount:  len(samples),
		Timestamp:    time.Now().UnixMilli(),
	}, nil
}

// readRawData 读取三孔原始数据
func (s *ThreeHoleTraversalService) readRawData() *types.ThreeHoleRawData {
	if s.batchGetter == nil {
		return nil
	}

	data, err := s.batchGetter(s.config.ProbeChannels)
	if err != nil {
		return nil
	}

	result := &types.ThreeHoleRawData{}
	for _, ch := range s.config.ProbeChannels {
		if !ch.Enabled {
			continue
		}
		val, ok := data[ch.Channel]
		if !ok {
			continue
		}
		switch ch.Role {
		case types.Role3H_P1:
			result.P1 = val
		case types.Role3H_P2:
			result.P2 = val
		case types.Role3H_P3:
			result.P3 = val
		case types.Role3H_PAtm:
			result.PAtm = val
		case types.Role3H_TAtm:
			result.TAtm = val
		}
	}

	return result
}

// calculateThreeHoleAverage 计算多次采样的平均值
func calculateThreeHoleAverage(samples []types.ThreeHoleRawData) types.ThreeHoleRawData {
	if len(samples) == 0 {
		return types.ThreeHoleRawData{}
	}

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

// emitPointError 记录点位错误但不中断测试（只更新 LastError，不改 Status）
func (s *ThreeHoleTraversalService) emitPointError(errMsg string) {
	s.mu.Lock()
	s.status.LastError = errMsg
	s.mu.Unlock()

	if s.eventPublisher != nil {
		s.eventPublisher.EmitError(types.ThreeHoleTraversalErrorEvent{
			TaskID:  s.status.TaskID,
			Error:   errMsg,
			IsFatal: false,
		})
	}
}

// emitFatalError 致命错误，停止测试
func (s *ThreeHoleTraversalService) emitFatalError(errMsg string) {
	s.mu.Lock()
	s.status.LastError = errMsg
	s.status.Status = types.TraversalStatusError
	s.mu.Unlock()

	s.running.Store(false)
	s.paused.Store(false)

	if s.eventPublisher != nil {
		s.eventPublisher.EmitError(types.ThreeHoleTraversalErrorEvent{
			TaskID:  s.status.TaskID,
			Error:   errMsg,
			IsFatal: true,
		})
	}
}

// ==================== 点位生成 ====================

// generatePoints 根据布点配置生成测试点位
func generatePoints(layout types.TraversalLayout) []types.TraversalPoint {
	switch layout.Pattern {
	case types.TraversalPatternLine:
		return generateLinePoints(layout.Line)
	case types.TraversalPatternRectangle:
		return generateRectanglePoints(layout.Rectangle)
	case types.TraversalPatternCustom:
		return layout.CustomPoints
	default:
		return []types.TraversalPoint{}
	}
}

// generateLinePoints 直线布点
func generateLinePoints(line *types.LineLayout) []types.TraversalPoint {
	if line == nil {
		return nil
	}

	var points []types.TraversalPoint
	id := 0

	// 生成X方向点位
	xValues := expandStepSegments(line.XSteps)
	yValues := expandStepSegments(line.YSteps)

	if len(xValues) == 0 && len(yValues) == 0 {
		// 如果没有分段步长，直接用起止点
		points = append(points, types.TraversalPoint{ID: fmt.Sprintf("pt-%d", id), X: line.StartX, Y: line.StartY})
		id++
		points = append(points, types.TraversalPoint{ID: fmt.Sprintf("pt-%d", id), X: line.EndX, Y: line.EndY})
		return points
	}

	if len(yValues) == 0 {
		yValues = []float64{line.StartY}
	}
	if len(xValues) == 0 {
		xValues = []float64{line.StartX}
	}

	for _, x := range xValues {
		for _, y := range yValues {
			points = append(points, types.TraversalPoint{
				ID: fmt.Sprintf("pt-%d", id),
				X:  x,
				Y:  y,
			})
			id++
		}
	}

	return points
}

// generateRectanglePoints 矩形布点
func generateRectanglePoints(rect *types.RectangleLayout) []types.TraversalPoint {
	if rect == nil {
		return nil
	}

	var points []types.TraversalPoint
	id := 0

	xValues := expandStepSegments(rect.XSteps)
	yValues := expandStepSegments(rect.YSteps)

	// 如果没有分段步长，使用默认步长
	if len(xValues) == 0 {
		xValues = []float64{rect.XMin, rect.XMax}
	}
	if len(yValues) == 0 {
		yValues = []float64{rect.YMin, rect.YMax}
	}

	for _, x := range xValues {
		for _, y := range yValues {
			points = append(points, types.TraversalPoint{
				ID: fmt.Sprintf("pt-%d", id),
				X:  x,
				Y:  y,
			})
			id++
		}
	}

	return points
}

// expandStepSegments 展开分段步长为具体数值列表
func expandStepSegments(segments []types.StepSegment) []float64 {
	var values []float64
	for _, seg := range segments {
		if seg.Step == 0 {
			values = append(values, seg.Start, seg.End)
			continue
		}
		for v := seg.Start; v <= seg.End+1e-9; v += seg.Step {
			values = append(values, v)
		}
	}
	return values
}
