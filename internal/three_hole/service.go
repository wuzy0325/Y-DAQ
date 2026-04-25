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
	cancelCh       chan struct{}
	pauseCh        chan struct{}
	resumeCh       chan struct{}

	config         types.ThreeHoleTraversalConfig
	interpolator   *ThreeHoleInterpolator
	batchGetter    ThreeHoleBatchGetter
	motionCtrl     ThreeHoleMotionController
	eventPublisher ThreeHoleEventPublisher
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

// ==================== 测试生命周期 ====================

// Start 启动测试
func (s *ThreeHoleTraversalService) Start(config types.ThreeHoleTraversalConfig) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running.Load() {
		return "", fmt.Errorf("test already running")
	}

	if !s.interpolator.IsLoaded() {
		return "", fmt.Errorf("calibration files not loaded")
	}

	taskID := fmt.Sprintf("3h-traversal-%d", time.Now().UnixMilli())
	s.config = config
	s.cancelCh = make(chan struct{})
	s.pauseCh = make(chan struct{})
	s.resumeCh = make(chan struct{})

	// 生成测试点位
	points := generatePoints(config.Layout)

	s.status = types.ThreeHoleTraversalTaskStatus{
		TaskID:      taskID,
		Status:      types.TraversalStatusRunning,
		TotalPoints: len(points),
		DataPoints:  []types.ThreeHoleTraversalDataPoint{},
	}

	s.running.Store(true)
	s.paused.Store(false)

	go s.runTestLoop(points)

	return taskID, nil
}

// Pause 暂停测试
func (s *ThreeHoleTraversalService) Pause() {
	s.paused.Store(true)
	s.mu.Lock()
	s.status.Status = types.TraversalStatusPaused
	s.mu.Unlock()
}

// Resume 恢复测试
func (s *ThreeHoleTraversalService) Resume() {
	s.paused.Store(false)
	s.mu.Lock()
	s.status.Status = types.TraversalStatusRunning
	s.mu.Unlock()
	select {
	case s.resumeCh <- struct{}{}:
	default:
	}
}

// Stop 停止测试
func (s *ThreeHoleTraversalService) Stop() {
	s.running.Store(false)
	select {
	case s.cancelCh <- struct{}{}:
	default:
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
func (s *ThreeHoleTraversalService) runTestLoop(points []types.TraversalPoint) {
	defer func() {
		s.running.Store(false)
	}()

	// 初始化 CSV 写入器
	csvWriter := NewThreeHoleCsvWriter()
	if err := csvWriter.Initialize(s.config.SavePath, s.config.SaveFileName); err != nil {
		log.Printf("csv init failed: %v", err)
		s.emitError(fmt.Sprintf("CSV初始化失败: %v", err))
		return
	}
	defer csvWriter.Close()

	for i, point := range points {
		// 检查取消
		select {
		case <-s.cancelCh:
			return
		default:
		}

		// 检查暂停
		for s.paused.Load() {
			select {
			case <-s.cancelCh:
				return
			case <-s.resumeCh:
				continue
			case <-time.After(100 * time.Millisecond):
			}
		}

		// 执行单点测试
		dataPoint, err := s.runSinglePoint(point)
		if err != nil {
			log.Printf("point %s failed: %v", point.ID, err)
			s.emitError(fmt.Sprintf("点位 %s 测试失败: %v", point.ID, err))
			// 继续下一个点，不中断整个测试
			continue
		}

		// 写入 CSV
		if err := csvWriter.AppendPoint(dataPoint); err != nil {
			log.Printf("csv write point %s failed: %v", point.ID, err)
		}

		// 更新状态
		s.mu.Lock()
		s.status.DataPoints = append(s.status.DataPoints, dataPoint)
		s.status.CompletedPoints = i + 1
		s.status.Progress = float64(i+1) / float64(s.status.TotalPoints) * 100
		s.status.CurrentPoint = &point
		s.mu.Unlock()

		// 推送进度事件
		if s.eventPublisher != nil {
			s.eventPublisher.EmitProgress(types.ThreeHoleTraversalProgressEvent{
				TaskID:          s.status.TaskID,
				TotalPoints:     s.status.TotalPoints,
				CompletedPoints: s.status.CompletedPoints,
				Progress:        s.status.Progress,
				CurrentX:        point.X,
				CurrentY:        point.Y,
			})
		}
	}

	// 测试完成
	s.mu.Lock()
	s.status.Status = types.TraversalStatusCompleted
	s.mu.Unlock()

	if s.eventPublisher != nil {
		s.eventPublisher.EmitComplete(types.ThreeHoleTraversalCompleteEvent{
			TaskID:     s.status.TaskID,
			Status:     types.TraversalStatusCompleted,
			DataPoints: s.status.DataPoints,
		})
	}
}

// runSinglePoint 执行单点测试
func (s *ThreeHoleTraversalService) runSinglePoint(point types.TraversalPoint) (types.ThreeHoleTraversalDataPoint, error) {
	// 1. 移动到目标位置
	if s.motionCtrl != nil {
		// X轴移动
		targetX := resolveTargetPosition(point.X, s.config.MotionX)
		if err := s.motionCtrl(s.config.MotionX.Axis, targetX); err != nil {
			return types.ThreeHoleTraversalDataPoint{}, fmt.Errorf("move X to %.2f failed: %w", targetX, err)
		}
		// Y轴移动
		targetY := resolveTargetPosition(point.Y, s.config.MotionY)
		if err := s.motionCtrl(s.config.MotionY.Axis, targetY); err != nil {
			return types.ThreeHoleTraversalDataPoint{}, fmt.Errorf("move Y to %.2f failed: %w", targetY, err)
		}
	}

	// 2. 等待驻留时间
	time.Sleep(time.Duration(s.config.DwellTimeMs) * time.Millisecond)

	// 3. 采集数据并插值
	return s.acquireAndInterpolate(point)
}

// resolveTargetPosition 角度→位置映射（含scale/offset）
func resolveTargetPosition(sourceValue float64, mapping types.MotionAxisMapping) float64 {
	return sourceValue*mapping.Scale + mapping.Offset
}

// acquireAndInterpolate 采集数据并执行插值
func (s *ThreeHoleTraversalService) acquireAndInterpolate(point types.TraversalPoint) (types.ThreeHoleTraversalDataPoint, error) {
	samples := []types.ThreeHoleRawData{}

	for i := 0; i < s.config.SamplesPerPoint; i++ {
		rawData := s.readRawData()
		if rawData == nil {
			continue
		}
		samples = append(samples, *rawData)

		// 实时插值并推送
		interpResult := s.interpolator.Calculate(*rawData)
		if s.eventPublisher != nil {
			s.eventPublisher.EmitRealtime(types.ThreeHoleTraversalRealtimeEvent{
				TaskID:      s.status.TaskID,
				PointID:     point.ID,
				RawData:     *rawData,
				InterpResult: interpResult,
			})
		}

		time.Sleep(50 * time.Millisecond) // 采样间隔
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

// emitError 发送错误事件
func (s *ThreeHoleTraversalService) emitError(errMsg string) {
	s.mu.Lock()
	s.status.LastError = errMsg
	s.status.Status = types.TraversalStatusError
	s.mu.Unlock()

	if s.eventPublisher != nil {
		s.eventPublisher.EmitError(types.ThreeHoleTraversalErrorEvent{
			TaskID: s.status.TaskID,
			Error:  errMsg,
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
