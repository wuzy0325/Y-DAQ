package three_hole

import (
	"fmt"
	"log/slog"

	"yx-daq/internal/types"
)

// EventHandler 事件处理器
type EventHandler struct {
	testManager    *TestManager
	dataProcessor  *DataProcessor
	csvWriter     *ThreeHoleCsvWriter
	eventPublisher ThreeHoleEventPublisher
}

// NewEventHandler 创建事件处理器
func NewEventHandler(testManager *TestManager, dataProcessor *DataProcessor, csvWriter *ThreeHoleCsvWriter, publisher ThreeHoleEventPublisher) *EventHandler {
	return &EventHandler{
		testManager:    testManager,
		dataProcessor:  dataProcessor,
		csvWriter:     csvWriter,
		eventPublisher: publisher,
	}
}

// OnTestStart 处理测试开始事件
func (eh *EventHandler) OnTestStart(config types.ThreeHoleTraversalConfig) error {
	// 确保之前的 CSV 已关闭（防御旧测试 goroutine 延迟清理）
	eh.csvWriter.Close()

	// 初始化CSV写入器
	if err := eh.csvWriter.Initialize(config.SavePath, config.SaveFileName); err != nil {
		slog.Error("csv init failed", "err", err)
		eh.testManager.EmitFatalError(fmt.Sprintf("CSV初始化失败: %v", err))
		return err
	}
	return nil
}

// OnTestComplete 处理测试完成事件
func (eh *EventHandler) OnTestComplete(taskID string, status types.TraversalTestStatus) {
	// 关闭CSV写入器
	eh.csvWriter.Close()

	// 推送完成事件
	eh.testManager.EmitComplete(taskID, status)
}

// OnTestError 处理测试错误事件
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
func (eh *EventHandler) OnDataPointAcquired(dataPoint types.ThreeHoleTraversalDataPoint) error {
	// 写入CSV文件
	if err := eh.csvWriter.AppendPoint(dataPoint); err != nil {
		slog.Error("csv write point failed", "point", dataPoint.PointID, "err", err)
		eh.testManager.EmitPointError(fmt.Sprintf("写入CSV失败: %v", err))
	}

	// 更新进度（使用 DataPoint 中的坐标，避免无锁读取 status.CurrentPoint）
	completed := len(eh.testManager.status.DataPoints)
	total := eh.testManager.status.TotalPoints
	progress := float64(completed) / float64(total) * 100

	eh.testManager.UpdateProgress(completed, total, eh.testManager.status.CurrentPoint)
	eh.testManager.EmitProgress(
		eh.testManager.status.TaskID,
		total,
		completed,
		progress,
		dataPoint.X,
		dataPoint.Y,
		"acquired",
	)

	return nil
}