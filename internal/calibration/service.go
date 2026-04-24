package calibration

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"yx-daq/internal/types"
)

// DataGetter 数据获取函数类型
type DataGetter func(deviceID string, channelIndex int) (float64, bool)

// ChannelBatchGetter 批量通道数据获取
type ChannelBatchGetter func(channels []types.ProbeChannelConfig) (map[int]float64, error)

// MotionController 运动控制函数类型
type MotionController func(axis types.AxisName, position float64) error

// EventPublisher 事件发布接口
type EventPublisher interface {
	EmitProgress(event types.CalibrationProgressEvent)
	EmitRealtime(event types.CalibrationRealtimeEvent)
	EmitComplete(event types.CalibrationCompleteEvent)
}

// CalibrationService 校准服务
type CalibrationService struct {
	mu              sync.Mutex
	status          types.CalibrationTaskStatus
	running         atomic.Bool
	paused          atomic.Bool
	cancelCh        chan struct{}
	pauseCh         chan struct{}
	resumeCh        chan struct{}

	config          types.CalibrationConfig
	dataGetter      DataGetter
	batchGetter     ChannelBatchGetter
	motionCtrl      MotionController
	eventPublisher  EventPublisher
}

// NewCalibrationService 创建校准服务
func NewCalibrationService(publisher EventPublisher) *CalibrationService {
	return &CalibrationService{
		eventPublisher: publisher,
		cancelCh:      make(chan struct{}),
		pauseCh:       make(chan struct{}),
		resumeCh:      make(chan struct{}),
	}
}

// SetDataGetter 设置数据获取函数
func (s *CalibrationService) SetDataGetter(getter DataGetter) {
	s.dataGetter = getter
}

// SetBatchGetter 设置批量数据获取函数
func (s *CalibrationService) SetBatchGetter(getter ChannelBatchGetter) {
	s.batchGetter = getter
}

// SetMotionController 设置运动控制函数
func (s *CalibrationService) SetMotionController(ctrl MotionController) {
	s.motionCtrl = ctrl
}

// Start 启动校准
func (s *CalibrationService) Start(config types.CalibrationConfig) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running.Load() {
		return "", fmt.Errorf("calibration already running")
	}

	taskID := fmt.Sprintf("cal-%d", time.Now().UnixMilli())
	s.config = config
	s.cancelCh = make(chan struct{})
	s.pauseCh = make(chan struct{})
	s.resumeCh = make(chan struct{})

	s.status = types.CalibrationTaskStatus{
		TaskID:      taskID,
		Status:      types.CalibStatusRunning,
		TotalPoints: len(config.Points),
		DataPoints:  []types.CalibrationDataPoint{},
	}

	s.running.Store(true)
	s.paused.Store(false)

	go s.runCalibrationLoop()

	return taskID, nil
}

// Pause 暂停校准
func (s *CalibrationService) Pause() {
	s.paused.Store(true)
	s.mu.Lock()
	s.status.Status = types.CalibStatusPaused
	s.mu.Unlock()
}

// Resume 恢复校准
func (s *CalibrationService) Resume() {
	s.paused.Store(false)
	s.mu.Lock()
	s.status.Status = types.CalibStatusRunning
	s.mu.Unlock()
	select {
	case s.resumeCh <- struct{}{}:
	default:
	}
}

// Stop 停止校准
func (s *CalibrationService) Stop() {
	s.running.Store(false)
	select {
	case s.cancelCh <- struct{}{}:
	default:
	}
	s.mu.Lock()
	s.status.Status = types.CalibStatusIdle
	s.mu.Unlock()
}

// GetStatus 获取校准状态
func (s *CalibrationService) GetStatus() types.CalibrationTaskStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// runCalibrationLoop 校准主循环
func (s *CalibrationService) runCalibrationLoop() {
	defer func() {
		s.running.Store(false)
	}()

	for i, point := range s.config.Points {
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

		// 1. 移动到目标位置
		if s.motionCtrl != nil {
			if err := s.motionCtrl(s.config.AlphaAxis, point.Alpha); err != nil {
				log.Printf("move to alpha=%.2f failed: %v", point.Alpha, err)
			}
			if err := s.motionCtrl(s.config.BetaAxis, point.Beta); err != nil {
				log.Printf("move to beta=%.2f failed: %v", point.Beta, err)
			}
		}

		// 2. 等待驻留时间
		time.Sleep(time.Duration(s.config.DwellTimeMs) * time.Millisecond)

		// 3. 采集数据
		dataPoint := s.acquireData(point)

		// 4. 保存数据点
		s.mu.Lock()
		s.status.DataPoints = append(s.status.DataPoints, dataPoint)
		s.status.CompletedPoints = i + 1
		s.status.Progress = float64(i+1) / float64(s.status.TotalPoints) * 100
		s.status.CurrentPoint = &point
		s.mu.Unlock()

		// 5. 推送进度事件
		if s.eventPublisher != nil {
			s.eventPublisher.EmitProgress(types.CalibrationProgressEvent{
				TaskID:          s.status.TaskID,
				TotalPoints:     s.status.TotalPoints,
				CompletedPoints: s.status.CompletedPoints,
				Progress:        s.status.Progress,
				CurrentAlpha:    point.Alpha,
				CurrentBeta:     point.Beta,
			})
		}
	}

	// 校准完成
	s.mu.Lock()
	s.status.Status = types.CalibStatusCompleted
	s.mu.Unlock()

	if s.eventPublisher != nil {
		s.eventPublisher.EmitComplete(types.CalibrationCompleteEvent{
			TaskID:     s.status.TaskID,
			Status:     types.CalibStatusCompleted,
			DataPoints: s.status.DataPoints,
		})
	}
}

// acquireData 采集单个校准点数据
func (s *CalibrationService) acquireData(point types.CalibrationPoint) types.CalibrationDataPoint {
	samples := []types.FiveHoleRawData{}
	p1Values := []float64{}

	for i := 0; i < s.config.SamplesPerPoint; i++ {
		rawData := s.readRawData()
		if rawData == nil {
			continue
		}
		samples = append(samples, *rawData)
		p1Values = append(p1Values, rawData.P1)

		// 实时推送
		coefficients := CalculateFiveHoleCoefficients(*rawData)
		if s.eventPublisher != nil {
			s.eventPublisher.EmitRealtime(types.CalibrationRealtimeEvent{
				TaskID:       s.status.TaskID,
				PointID:      point.ID,
				RawData:      *rawData,
				Coefficients: coefficients,
			})
		}

		time.Sleep(50 * time.Millisecond) // 采样间隔
	}

	if len(samples) == 0 {
		return types.CalibrationDataPoint{
			PointID: point.ID,
			Alpha:   point.Alpha,
			Beta:    point.Beta,
		}
	}

	avgData := CalculateAverage(samples)
	coefficients := CalculateFiveHoleCoefficients(avgData)
	stdDev := CalculateStdDev(p1Values)

	return types.CalibrationDataPoint{
		PointID:      point.ID,
		Alpha:        point.Alpha,
		Beta:         point.Beta,
		RawData:      avgData,
		Coefficients: coefficients,
		SampleCount:  len(samples),
		StdDev:       stdDev,
	}
}

// readRawData 读取五孔原始数据
func (s *CalibrationService) readRawData() *types.FiveHoleRawData {
	if s.batchGetter == nil {
		return nil
	}

	data, err := s.batchGetter(s.config.ProbeChannels)
	if err != nil {
		return nil
	}

	result := &types.FiveHoleRawData{}
	for _, ch := range s.config.ProbeChannels {
		if !ch.Enabled {
			continue
		}
		val, ok := data[ch.Channel]
		if !ok {
			continue
		}
		switch ch.Role {
		case types.RoleP1:
			result.P1 = val
		case types.RoleP2:
			result.P2 = val
		case types.RoleP3:
			result.P3 = val
		case types.RoleP4:
			result.P4 = val
		case types.RoleP5:
			result.P5 = val
		case types.RolePAtm:
			result.PAtm = val
		case types.RoleTAtm:
			result.TAtm = val
		case types.RolePTotal:
			result.PTotal = &val
		}
	}

	return result
}
