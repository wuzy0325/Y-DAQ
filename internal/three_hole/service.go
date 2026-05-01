package three_hole

import (
	"fmt"
	"log/slog"
	"time"

	"yx-daq/internal/types"
)

// ==================== 依赖接口 ====================

// ThreeHoleBatchGetter 批量通道数据获取
type ThreeHoleBatchGetter func(channels []types.ThreeHoleProbeChannelConfig) (map[int]float64, error)

// ThreeHoleMotionController 运动控制函数
type ThreeHoleMotionController func(axis types.AxisName, position float64) error

// ThreeHoleMotionWaiter 运动等待函数（阻塞直到指定轴运动完成或超时）
type ThreeHoleMotionWaiter func(axis types.AxisName, timeoutMs int) error

// ThreeHoleEventPublisher 事件发布接口
type ThreeHoleEventPublisher interface {
	EmitProgress(event types.ThreeHoleTraversalProgressEvent)
	EmitRealtime(event types.ThreeHoleTraversalRealtimeEvent)
	EmitComplete(event types.ThreeHoleTraversalCompleteEvent)
	EmitError(event types.ThreeHoleTraversalErrorEvent)
}

// ==================== 服务 ====================

// ThreeHoleTraversalService 三孔移位测试服务（重构后版本）
type ThreeHoleTraversalService struct {
	interpolator   *ThreeHoleInterpolator
	testManager    *TestManager
	dataProcessor  *DataProcessor
	eventHandler   *EventHandler

	// 实时监控取消通道
	monitorCancel chan struct{}
}

// NewThreeHoleTraversalService 创建三孔移位测试服务（重构后版本）
func NewThreeHoleTraversalService(publisher ThreeHoleEventPublisher) *ThreeHoleTraversalService {
	interpolator := NewThreeHoleInterpolator()
	testManager := NewTestManager(publisher)

	service := &ThreeHoleTraversalService{
		interpolator:   interpolator,
		testManager:    testManager,
		monitorCancel: make(chan struct{}),
	}

	// 初始化数据处理器
	dataProcessor := NewDataProcessor(testManager, interpolator, publisher)
	service.dataProcessor = dataProcessor

	// 初始化事件处理器
	csvWriter := NewThreeHoleCsvWriter()
	eventHandler := NewEventHandler(testManager, dataProcessor, csvWriter, publisher)
	service.eventHandler = eventHandler

	return service
}

// SetBatchGetter 设置批量数据获取函数
func (s *ThreeHoleTraversalService) SetBatchGetter(getter ThreeHoleBatchGetter) {
	s.dataProcessor.SetBatchGetter(getter)
}

// SetMotionController 设置运动控制函数
func (s *ThreeHoleTraversalService) SetMotionController(ctrl ThreeHoleMotionController) {
	s.dataProcessor.SetMotionController(ctrl)
}

// SetMotionWaiter 设置运动等待函数
func (s *ThreeHoleTraversalService) SetMotionWaiter(waiter ThreeHoleMotionWaiter) {
	s.dataProcessor.SetMotionWaiter(waiter)
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
func (s *ThreeHoleTraversalService) StartRealtimeMonitor(config types.ThreeHoleTraversalConfig) {
	if s.monitorCancel != nil {
		select {
		case s.monitorCancel <- struct{}{}:
		default:
		}
	}
	s.dataProcessor.StartRealtimeMonitor(config)
}

// StopRealtimeMonitor 停止实时数据监控
func (s *ThreeHoleTraversalService) StopRealtimeMonitor() {
	if s.dataProcessor != nil {
		s.dataProcessor.StopRealtimeMonitor()
	}
}

// ==================== 测试生命周期 ====================

// Start 启动测试
func (s *ThreeHoleTraversalService) Start(config types.ThreeHoleTraversalConfig) (string, error) {
	// 验证配置
	if err := config.Validate(); err != nil {
		return "", fmt.Errorf("配置验证失败: %w", err)
	}

	// 检查校准文件是否已加载
	if !s.interpolator.IsLoaded() {
		return "", fmt.Errorf("calibration files not loaded")
	}

	// 初始化CSV写入器
	if err := s.eventHandler.OnTestStart(config); err != nil {
		return "", err
	}

	// 启动测试
	taskID, err := s.testManager.Start(config)
	if err != nil {
		s.eventHandler.OnFatalError(fmt.Sprintf("启动测试失败: %v", err))
		return "", err
	}

	// 启动测试协程
	go s.runTestLoop(taskID, config)

	return taskID, nil
}

// Pause 暂停测试
func (s *ThreeHoleTraversalService) Pause() {
	s.testManager.Pause()
}

// Resume 恢复测试
func (s *ThreeHoleTraversalService) Resume() {
	s.testManager.Resume()
}

// Stop 停止测试
func (s *ThreeHoleTraversalService) Stop() {
	s.testManager.Stop()

	// 清理监控资源
	if s.monitorCancel != nil {
		select {
		case s.monitorCancel <- struct{}{}:
		default:
		}
	}
}

// GetStatus 获取测试状态
func (s *ThreeHoleTraversalService) GetStatus() types.ThreeHoleTraversalTaskStatus {
	return s.testManager.GetStatus()
}

// GetConfig 获取当前测试配置
func (s *ThreeHoleTraversalService) GetConfig() types.ThreeHoleTraversalConfig {
	return s.testManager.GetConfig()
}

// ==================== 测试主循环 ====================

// runTestLoop 测试主循环（重构后版本）
func (s *ThreeHoleTraversalService) runTestLoop(taskID string, config types.ThreeHoleTraversalConfig) {
	points := generatePoints(config.Layout)
	totalPoints := len(points)

	s.testManager.EmitProgress(taskID, totalPoints, 0, 0, 0, 0, "starting")

	defer s.eventHandler.OnTestComplete(taskID, s.testManager.status.Status)

	// 保存当前代际号，用于检测是否被新测试取代
	myGen := s.testManager.testGen.Load()

	for _, point := range points {
		select {
		case <-s.testManager.cancelCh:
			return
		default:
		}

		// 检测是否已被新 Start() 取代
		if s.testManager.testGen.Load() != myGen {
			return
		}

		// 处理暂停状态
		for s.testManager.paused.Load() {
			time.Sleep(100 * time.Millisecond)
		}

		// 更新当前点位信息
		s.testManager.mu.Lock()
		s.testManager.status.CurrentPoint = &point
		s.testManager.mu.Unlock()

		// 执行单点测试
		dataPoint, err := s.dataProcessor.RunSinglePoint(point, s.testManager.cancelCh)
		if err != nil {
			if !s.testManager.running.Load() {
				return
			}
			s.eventHandler.OnTestError(point.ID, err)
			continue
		}

		// 处理采集完成的数据
		if err := s.eventHandler.OnDataPointAcquired(dataPoint); err != nil {
			slog.Error("处理数据点失败", "point", point.ID, "err", err)
		}
	}
}

