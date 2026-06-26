package five_hole

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"yx-daq/internal/types"
)

// TestManager 五孔测试生命周期管理器
// 照三孔 TestManager，适配五孔多探针：
// - 统一进度（等最慢探针）+ 各探针独立 phase 指示
// - ProbeStatuses 在 Start 时按启用探针初始化
type TestManager struct {
	mu         sync.Mutex
	status     types.FiveHoleTraversalTaskStatus
	running    atomic.Bool
	paused     atomic.Bool
	testGen    atomic.Int64
	ctx        context.Context
	cancel     context.CancelFunc
	doneCh     chan struct{}

	config         types.FiveHoleTraversalConfig
	eventPublisher FiveHoleEventPublisher
}

// NewTestManager 创建测试管理器
func NewTestManager(publisher FiveHoleEventPublisher) *TestManager {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 初始状态为已取消，Start 时重新创建
	return &TestManager{
		eventPublisher: publisher,
		ctx:            ctx,
		cancel:         cancel,
		status: types.FiveHoleTraversalTaskStatus{
			Status:        types.TraversalStatusIdle,
			TotalPoints:   0,
			ProbeStatuses: []types.FiveHoleProbeStatus{},
		},
	}
}

// Start 启动测试
// 生成 taskID "5h-traversal-<unixmilli>"，调用 generatePoints 生成布点，
// 初始化 status（含 ProbeStatuses 按启用探针初始化）
func (tm *TestManager) Start(config types.FiveHoleTraversalConfig) (string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	switch tm.status.Status {
	case types.TraversalStatusRunning, types.TraversalStatusPaused:
		return "", fmt.Errorf("test already running")
	}

	points, err := generatePoints(config.Layout)
	if err != nil {
		return "", fmt.Errorf("生成布点失败: %w", err)
	}
	if len(points) > maxTraversalPoints {
		return "", fmt.Errorf("点位数量 %d 超过最大限制 %d", len(points), maxTraversalPoints)
	}

	taskID := fmt.Sprintf("5h-traversal-%d", time.Now().UnixMilli())
	tm.config = config
	tm.ctx, tm.cancel = context.WithCancel(context.Background())
	doneCh := make(chan struct{})
	tm.doneCh = doneCh

	// 按启用探针初始化 ProbeStatuses
	probeStatuses := make([]types.FiveHoleProbeStatus, 0, len(config.Probes))
	for _, p := range config.Probes {
		if !p.Enabled {
			continue
		}
		probeStatuses = append(probeStatuses, types.FiveHoleProbeStatus{
			ProbeID: p.ProbeID,
			Phase:   "idle",
		})
	}

	tm.status = types.FiveHoleTraversalTaskStatus{
		TaskID:        taskID,
		Status:        types.TraversalStatusRunning,
		TotalPoints:   len(points),
		ProbeStatuses: probeStatuses,
	}

	tm.running.Store(true)
	tm.paused.Store(false)

	myGen := tm.testGen.Add(1)
	// 测试循环现在由 Service 层负责，这里只初始化状态
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
	tm.mu.Unlock()

	tm.running.Store(false)
	tm.paused.Store(false)

	if tm.cancel != nil {
		tm.cancel()
	}

	// 立即返回，不等待 goroutine 退出。
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
func (tm *TestManager) GetStatus() types.FiveHoleTraversalTaskStatus {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return tm.status
}

// GetConfig 获取当前测试配置
func (tm *TestManager) GetConfig() types.FiveHoleTraversalConfig {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return tm.config
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

// UpdateProgress 更新统一进度
func (tm *TestManager) UpdateProgress(completed int, total int, currentPoint *types.TraversalPoint) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.status.CompletedPoints = completed
	if total > 0 {
		tm.status.Progress = float64(completed) / float64(total) * 100
	}
	tm.status.CurrentPoint = currentPoint
}

// UpdateProbeStatus 更新单根探针的 phase 与坐标
func (tm *TestManager) UpdateProbeStatus(probeID string, phase string, x, y float64) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	for i := range tm.status.ProbeStatuses {
		if tm.status.ProbeStatuses[i].ProbeID == probeID {
			tm.status.ProbeStatuses[i].Phase = phase
			tm.status.ProbeStatuses[i].CurrentX = x
			tm.status.ProbeStatuses[i].CurrentY = y
			return
		}
	}
}

// UpdateProbeData 更新单根探针的实时数据与插值结果
func (tm *TestManager) UpdateProbeData(probeID string, rawData *types.FiveHoleRawData, interpResult *types.FiveHoleInterpolationResult) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	for i := range tm.status.ProbeStatuses {
		if tm.status.ProbeStatuses[i].ProbeID == probeID {
			tm.status.ProbeStatuses[i].RawData = rawData
			tm.status.ProbeStatuses[i].InterpResult = interpResult
			return
		}
	}
}

// GetTaskID 获取当前任务ID（线程安全）
func (tm *TestManager) GetTaskID() string {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return tm.status.TaskID
}

// GetTotalPoints 获取总点位数（线程安全）
func (tm *TestManager) GetTotalPoints() int {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return tm.status.TotalPoints
}

// GetCompletedPoints 获取已完成点位数（线程安全）
func (tm *TestManager) GetCompletedPoints() int {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return tm.status.CompletedPoints
}

// GetProgress 获取进度百分比（线程安全）
func (tm *TestManager) GetProgress() float64 {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return tm.status.Progress
}

// CheckCancelled 检查是否已被取消
func (tm *TestManager) CheckCancelled() error {
	if tm.ctx.Err() != nil {
		return fmt.Errorf("canceled")
	}
	return nil
}

// WaitForResume 阻塞等待直到暂停解除
func (tm *TestManager) WaitForResume() {
	for tm.paused.Load() {
		select {
		case <-tm.ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// EmitProgress 推送进度事件（含当前 ProbeStatuses 快照）
func (tm *TestManager) EmitProgress(taskID string, total, completed int, progress, currentX, currentY float64, phase string) {
	if tm.eventPublisher == nil {
		return
	}
	tm.mu.Lock()
	probeStatuses := make([]types.FiveHoleProbeStatus, len(tm.status.ProbeStatuses))
	copy(probeStatuses, tm.status.ProbeStatuses)
	tm.mu.Unlock()

	tm.eventPublisher.EmitProgress(types.FiveHoleTraversalProgressEvent{
		TaskID:          taskID,
		TotalPoints:     total,
		CompletedPoints: completed,
		Progress:        progress,
		CurrentX:        currentX,
		CurrentY:        currentY,
		Phase:           phase,
		ProbeStatuses:   probeStatuses,
	})
}

// EmitRealtime 推送实时数据事件
func (tm *TestManager) EmitRealtime(evt types.FiveHoleTraversalRealtimeEvent) {
	if tm.eventPublisher == nil {
		return
	}
	tm.eventPublisher.EmitRealtime(evt)
}

// EmitComplete 推送完成事件（含每探针数据点）
func (tm *TestManager) EmitComplete(taskID string, status types.TraversalTestStatus, probeDataPoints map[string][]types.FiveHoleTraversalDataPoint) {
	if tm.eventPublisher == nil {
		return
	}
	tm.eventPublisher.EmitComplete(types.FiveHoleTraversalCompleteEvent{
		TaskID:          taskID,
		Status:          status,
		ProbeDataPoints: probeDataPoints,
	})
}

// EmitError 推送错误事件
func (tm *TestManager) EmitError(taskID, errMsg string, isFatal bool) {
	if tm.eventPublisher == nil {
		return
	}
	tm.eventPublisher.EmitError(types.FiveHoleTraversalErrorEvent{
		TaskID:  taskID,
		Error:   errMsg,
		IsFatal: isFatal,
	})
}

// EmitPointError 记录点位错误但不中断测试（只更新 LastError，不改 Status）
func (tm *TestManager) EmitPointError(errMsg string) {
	tm.SetLastError(errMsg)
	if tm.eventPublisher != nil {
		tm.eventPublisher.EmitError(types.FiveHoleTraversalErrorEvent{
			TaskID:  tm.GetTaskID(),
			Error:   errMsg,
			IsFatal: false,
		})
	}
}

// CloseDoneCh 关闭测试完成通道（由 Service 层在测试循环退出时调用）
func (tm *TestManager) CloseDoneCh() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if tm.doneCh != nil {
		close(tm.doneCh)
		tm.doneCh = nil
	}
}

// EmitFatalError 致命错误，停止测试
func (tm *TestManager) EmitFatalError(errMsg string) {
	tm.SetLastError(errMsg)
	tm.SetStatus(types.TraversalStatusError)

	tm.running.Store(false)
	tm.paused.Store(false)

	if tm.eventPublisher != nil {
		tm.eventPublisher.EmitError(types.FiveHoleTraversalErrorEvent{
			TaskID:  tm.GetTaskID(),
			Error:   errMsg,
			IsFatal: true,
		})
	}
}
