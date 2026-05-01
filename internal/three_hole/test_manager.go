package three_hole

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"yx-daq/internal/types"
)

// TestManager 测试生命周期管理器
type TestManager struct {
	mu         sync.Mutex
	status     types.ThreeHoleTraversalTaskStatus
	running    atomic.Bool
	paused     atomic.Bool
	testGen    atomic.Int64
	cancelCh   chan struct{}
	pauseCh    chan struct{}
	resumeCh   chan struct{}
	doneCh     chan struct{}

	config         types.ThreeHoleTraversalConfig
	eventPublisher ThreeHoleEventPublisher
}

// NewTestManager 创建测试管理器
func NewTestManager(publisher ThreeHoleEventPublisher) *TestManager {
	return &TestManager{
		eventPublisher: publisher,
		cancelCh:      make(chan struct{}, 1),
		pauseCh:       make(chan struct{}),
		resumeCh:      make(chan struct{}),
		status: types.ThreeHoleTraversalTaskStatus{
			Status:      types.TraversalStatusIdle,
			TotalPoints: 0,
			DataPoints:  []types.ThreeHoleTraversalDataPoint{},
		},
	}
}

// Start 启动测试
func (tm *TestManager) Start(config types.ThreeHoleTraversalConfig) (string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	switch tm.status.Status {
	case types.TraversalStatusRunning, types.TraversalStatusPaused:
		return "", fmt.Errorf("test already running")
	}

	taskID := fmt.Sprintf("3h-traversal-%d", time.Now().UnixMilli())
	tm.config = config
	tm.cancelCh = make(chan struct{}, 1)
	tm.pauseCh = make(chan struct{})
	tm.resumeCh = make(chan struct{})
	doneCh := make(chan struct{})
	tm.doneCh = doneCh

	points := generatePoints(config.Layout)
	if len(points) > maxTraversalPoints {
		return "", fmt.Errorf("点位数量 %d 超过最大限制 %d", len(points), maxTraversalPoints)
	}
	if len(points) == 0 {
		return "", fmt.Errorf("布点配置生成0个点位")
	}

	tm.status = types.ThreeHoleTraversalTaskStatus{
		TaskID:      taskID,
		Status:      types.TraversalStatusRunning,
		TotalPoints: len(points),
		DataPoints:  []types.ThreeHoleTraversalDataPoint{},
	}

	tm.running.Store(true)
	tm.paused.Store(false)

	myGen := tm.testGen.Add(1)
	// 测试循环现在由Service层负责，这里只初始化状态
	go tm.waitForTestComplete(doneCh, myGen)

	return taskID, nil
}

// Pause 暂停测试
func (tm *TestManager) Pause() {
	tm.mu.Lock()
	if tm.status.Status != types.TraversalStatusRunning {
		tm.mu.Unlock()
		return
	}
	tm.status.Status = types.TraversalStatusPaused
	tm.mu.Unlock()
	tm.paused.Store(true)
}

// Resume 恢复测试
func (tm *TestManager) Resume() {
	tm.mu.Lock()
	if tm.status.Status != types.TraversalStatusPaused {
		tm.mu.Unlock()
		return
	}
	tm.status.Status = types.TraversalStatusRunning
	tm.mu.Unlock()
	tm.paused.Store(false)
	select {
	case tm.resumeCh <- struct{}{}:
	default:
	}
}

// Stop 停止测试
func (tm *TestManager) Stop() {
	tm.mu.Lock()
	switch tm.status.Status {
	case types.TraversalStatusRunning, types.TraversalStatusPaused, types.TraversalStatusError, types.TraversalStatusCompleted:
	default:
		tm.mu.Unlock()
		return
	}
	cancelCh := tm.cancelCh
	tm.mu.Unlock()

	// 先置 running=false 让 goroutine 的 error 路径能立即退出
	tm.running.Store(false)
	tm.paused.Store(false)

	// buffered cancelCh 保底：即使 goroutine 正在阻塞调用，信号也不会丢失
	select {
	case cancelCh <- struct{}{}:
	default:
	}

	// 立即返回，不等待 goroutine 退出。goroutine 通过 cancelCh / running 标志退出，
	// 代际计数器 testGen 防止残留 goroutine 干扰新测试。
	tm.mu.Lock()
	tm.status.Status = types.TraversalStatusIdle
	tm.mu.Unlock()
}

// waitForTestComplete 等待测试完成（用于清理状态）
func (tm *TestManager) waitForTestComplete(doneCh chan struct{}, myGen int64) {
	// 等待测试完成或取消
	<-doneCh

	// 只有当前 goroutine 的 gen 与服务级 gen 一致时才清理状态
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if tm.testGen.Load() == myGen {
		tm.running.Store(false)
		tm.paused.Store(false)
		if tm.status.Status == types.TraversalStatusRunning {
			tm.status.Status = types.TraversalStatusIdle
		}
	}
}

// GetStatus 获取测试状态
func (tm *TestManager) GetStatus() types.ThreeHoleTraversalTaskStatus {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return tm.status
}

// GetConfig 获取当前测试配置
func (tm *TestManager) GetConfig() types.ThreeHoleTraversalConfig {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return tm.config
}

// UpdateDataPoints 更新数据点
func (tm *TestManager) UpdateDataPoint(dataPoint types.ThreeHoleTraversalDataPoint) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.status.DataPoints = append(tm.status.DataPoints, dataPoint)
}

// UpdateProgress 更新进度
func (tm *TestManager) UpdateProgress(completed int, total int, currentPoint *types.TraversalPoint) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.status.CompletedPoints = completed
	tm.status.Progress = float64(completed) / float64(total) * 100
	tm.status.CurrentPoint = currentPoint
}

// SetStatus 设置状态
func (tm *TestManager) SetStatus(status types.TraversalTestStatus) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.status.Status = status
}

// SetLastError 设置错误信息
func (tm *TestManager) SetLastError(errMsg string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.status.LastError = errMsg
}

// EmitProgress 推送进度事件
func (tm *TestManager) EmitProgress(taskID string, total int, completed int, progress float64, currentX, currentY float64, phase string) {
	if tm.eventPublisher == nil {
		return
	}
	tm.eventPublisher.EmitProgress(types.ThreeHoleTraversalProgressEvent{
		TaskID:          taskID,
		TotalPoints:     total,
		CompletedPoints: completed,
		Progress:        progress,
		CurrentX:        currentX,
		CurrentY:        currentY,
		Phase:           phase,
	})
}

// EmitComplete 推送完成事件
func (tm *TestManager) EmitComplete(taskID string, status types.TraversalTestStatus) {
	if tm.eventPublisher == nil {
		return
	}
	tm.eventPublisher.EmitComplete(types.ThreeHoleTraversalCompleteEvent{
		TaskID:     taskID,
		Status:     status,
		DataPoints: tm.status.DataPoints,
	})
}

// EmitError 推送错误事件
func (tm *TestManager) EmitError(taskID, errMsg string, isFatal bool) {
	if tm.eventPublisher == nil {
		return
	}
	tm.eventPublisher.EmitError(types.ThreeHoleTraversalErrorEvent{
		TaskID:  taskID,
		Error:   errMsg,
		IsFatal: isFatal,
	})
}

// emitPointError 记录点位错误但不中断测试（只更新 LastError，不改 Status）
func (tm *TestManager) EmitPointError(errMsg string) {
	tm.SetLastError(errMsg)
	if tm.eventPublisher != nil {
		tm.eventPublisher.EmitError(types.ThreeHoleTraversalErrorEvent{
			TaskID:  tm.status.TaskID,
			Error:   errMsg,
			IsFatal: false,
		})
	}
}

// emitFatalError 致命错误，停止测试
func (tm *TestManager) EmitFatalError(errMsg string) {
	tm.SetLastError(errMsg)
	tm.SetStatus(types.TraversalStatusError)

	tm.running.Store(false)
	tm.paused.Store(false)

	if tm.eventPublisher != nil {
		tm.eventPublisher.EmitError(types.ThreeHoleTraversalErrorEvent{
			TaskID:  tm.status.TaskID,
			Error:   errMsg,
			IsFatal: true,
		})
	}
}


// CheckCancelled 检查是否已被取消
func (tm *TestManager) CheckCancelled(cancelCh chan struct{}) error {
	select {
	case <-cancelCh:
		return fmt.Errorf("canceled")
	default:
		return nil
	}
}

// WaitForResume 阻塞等待直到暂停解除
func (tm *TestManager) WaitForResume(cancelCh chan struct{}) {
	for tm.paused.Load() {
		select {
		case <-cancelCh:
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
}