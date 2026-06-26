package five_hole

import (
	"fmt"
	"log/slog"
	"sync"

	"yx-daq/internal/types"
)

// EventHandler 五孔事件处理器
// 照三孔 EventHandler，适配五孔多探针：
// - 每启用探针一个 csvWriter（map[probeID]*FiveHoleCsvWriter）
// - 累积每探针数据点，在 OnTestComplete 时放入 ProbeDataPoints
type EventHandler struct {
	mu              sync.Mutex
	testManager     *TestManager
	dataProcessor   *DataProcessor
	csvWriters      map[string]*FiveHoleCsvWriter
	probeDataPoints map[string][]types.FiveHoleTraversalDataPoint
	eventPublisher  FiveHoleEventPublisher
}

// NewEventHandler 创建事件处理器
func NewEventHandler(testManager *TestManager, dataProcessor *DataProcessor, publisher FiveHoleEventPublisher) *EventHandler {
	return &EventHandler{
		testManager:     testManager,
		dataProcessor:   dataProcessor,
		csvWriters:      make(map[string]*FiveHoleCsvWriter),
		probeDataPoints: make(map[string][]types.FiveHoleTraversalDataPoint),
		eventPublisher:  publisher,
	}
}

// OnTestStart 处理测试开始事件
// 为每个启用探针创建 csvWriter 并 Initialize(savePath, saveFileName, probeID)
func (eh *EventHandler) OnTestStart(config types.FiveHoleTraversalConfig) error {
	eh.mu.Lock()
	defer eh.mu.Unlock()

	// 确保之前的 CSV 已关闭（防御旧测试 goroutine 延迟清理）
	for _, w := range eh.csvWriters {
		w.Close()
	}
	eh.csvWriters = make(map[string]*FiveHoleCsvWriter)
	eh.probeDataPoints = make(map[string][]types.FiveHoleTraversalDataPoint)

	for _, p := range config.Probes {
		if !p.Enabled {
			continue
		}
		w := NewFiveHoleCsvWriter()
		if err := w.Initialize(config.SavePath, config.SaveFileName, p.ProbeID); err != nil {
			// 清理已创建的 writer
			for _, ww := range eh.csvWriters {
				ww.Close()
			}
			eh.csvWriters = make(map[string]*FiveHoleCsvWriter)
			slog.Error("csv init failed", "probe", p.ProbeID, "err", err)
			eh.testManager.EmitFatalError(fmt.Sprintf("探针%s CSV初始化失败: %v", p.ProbeID, err))
			return err
		}
		eh.csvWriters[p.ProbeID] = w
	}

	return nil
}

// OnTestComplete 处理测试完成事件
// 关闭所有 csvWriter，发射 complete 事件（含 ProbeDataPoints map）
func (eh *EventHandler) OnTestComplete(taskID string, status types.TraversalTestStatus) {
	eh.mu.Lock()
	for _, w := range eh.csvWriters {
		w.Close()
	}
	eh.csvWriters = make(map[string]*FiveHoleCsvWriter)
	probeDataPoints := eh.probeDataPoints
	eh.probeDataPoints = make(map[string][]types.FiveHoleTraversalDataPoint)
	eh.mu.Unlock()

	eh.testManager.EmitComplete(taskID, status, probeDataPoints)
}

// CloseCSVWriters 关闭所有 csvWriter 并清空累积数据点（不发射 complete 事件）
// 用于测试启动失败时的回滚，避免空 CSV 文件残留与状态泄漏
func (eh *EventHandler) CloseCSVWriters() {
	eh.mu.Lock()
	defer eh.mu.Unlock()
	for _, w := range eh.csvWriters {
		w.Close()
	}
	eh.csvWriters = make(map[string]*FiveHoleCsvWriter)
	eh.probeDataPoints = make(map[string][]types.FiveHoleTraversalDataPoint)
}

// OnTestError 处理测试错误事件（单点位错误，不中断测试）
func (eh *EventHandler) OnTestError(pointID string, err error) {
	if !eh.testManager.running.Load() {
		return
	}

	errorMsg := fmt.Sprintf("点位 %s 测试失败: %v", pointID, err)
	slog.Error(errorMsg, "point", pointID, "err", err)
	eh.testManager.EmitPointError(errorMsg)

	// 继续下一个点位，不中断测试
}

// OnFatalError 处理致命错误事件
func (eh *EventHandler) OnFatalError(errMsg string) {
	eh.testManager.EmitFatalError(errMsg)
}

// OnDataPointAcquired 处理数据点采集完成事件
// 写入对应探针的 csvWriter，并累积到 probeDataPoints
func (eh *EventHandler) OnDataPointAcquired(probeID string, dataPoint types.FiveHoleTraversalDataPoint) error {
	eh.mu.Lock()
	defer eh.mu.Unlock()

	w, ok := eh.csvWriters[probeID]
	if !ok {
		return fmt.Errorf("探针%s 的 csvWriter 不存在", probeID)
	}

	if err := w.AppendPoint(dataPoint); err != nil {
		slog.Error("csv write point failed", "probe", probeID, "point", dataPoint.PointID, "err", err)
		eh.testManager.EmitPointError(fmt.Sprintf("探针%s 写入CSV失败: %v", probeID, err))
	}

	eh.probeDataPoints[probeID] = append(eh.probeDataPoints[probeID], dataPoint)
	return nil
}
