package five_hole

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"yx-daq/internal/types"
)

// MotionCoordinator 多探针同步运动协调器
// 负责 3 根探针各自双轴并行移动，等待最慢探针完成
type MotionCoordinator struct {
	mover  FiveHoleProbeAxisMover
	waiter FiveHoleProbeAxisWaiter
}

// NewMotionCoordinator 创建运动协调器
func NewMotionCoordinator(mover FiveHoleProbeAxisMover, waiter FiveHoleProbeAxisWaiter) *MotionCoordinator {
	return &MotionCoordinator{mover: mover, waiter: waiter}
}

// MoveAllProbesToPoint 移动所有启用探针到指定点位
// - 各探针各自坐标系（共享网格拓扑）
// - 6 轴并行移动（α→X, β→Y）
// - 直线单轴模式：仅驱动变化方向的轴
// - 等待最慢探针完成（MotionTimeoutMs）
func (mc *MotionCoordinator) MoveAllProbesToPoint(
	point types.TraversalPoint,
	probes []types.FiveHoleProbeConfig,
	layout types.TraversalLayout,
	motionTimeoutMs int,
) error {
	if mc.mover == nil {
		return fmt.Errorf("motion mover not set")
	}
	if motionTimeoutMs <= 0 {
		motionTimeoutMs = 30000
	}

	// 判断是否为直线单轴模式
	skipAlpha, skipBeta := computeAxisSkip(layout)

	var wg sync.WaitGroup
	errChan := make(chan error, len(probes)*2)

	for _, probe := range probes {
		if !probe.Enabled {
			continue
		}
		// α 轴移动
		if !skipAlpha {
			wg.Add(1)
			go func(p types.FiveHoleProbeConfig) {
				defer wg.Done()
				if err := mc.mover(p.MotionAlpha.ControllerID, p.MotionAlpha.Axis, point.X); err != nil {
					errChan <- fmt.Errorf("探针%s α轴移动到 %.2f 失败: %w", p.ProbeID, point.X, err)
				}
			}(probe)
		}
		// β 轴移动
		if !skipBeta {
			wg.Add(1)
			go func(p types.FiveHoleProbeConfig) {
				defer wg.Done()
				if err := mc.mover(p.MotionBeta.ControllerID, p.MotionBeta.Axis, point.Y); err != nil {
					errChan <- fmt.Errorf("探针%s β轴移动到 %.2f 失败: %w", p.ProbeID, point.Y, err)
				}
			}(probe)
		}
	}

	wg.Wait()
	close(errChan)

	// 收集所有错误而非仅返回首个，便于一次性诊断多探针故障
	var errs []string
	for err := range errChan {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return fmt.Errorf("多轴移动失败: %s", strings.Join(errs, "; "))
	}

	// 等待所有运动完成
	if mc.waiter != nil {
		wg = sync.WaitGroup{}
		for _, probe := range probes {
			if !probe.Enabled {
				continue
			}
			if !skipAlpha {
				wg.Add(1)
				go func(p types.FiveHoleProbeConfig) {
					defer wg.Done()
					if err := mc.waiter(p.MotionAlpha.ControllerID, p.MotionAlpha.Axis, motionTimeoutMs); err != nil {
						slog.Warn("五孔: 等待 α 轴运动完成失败", "probeID", p.ProbeID, "err", err)
					}
				}(probe)
			}
			if !skipBeta {
				wg.Add(1)
				go func(p types.FiveHoleProbeConfig) {
					defer wg.Done()
					if err := mc.waiter(p.MotionBeta.ControllerID, p.MotionBeta.Axis, motionTimeoutMs); err != nil {
						slog.Warn("五孔: 等待 β 轴运动完成失败", "probeID", p.ProbeID, "err", err)
					}
				}(probe)
			}
		}
		wg.Wait()
	}

	return nil
}

// computeAxisSkip 根据布点配置计算是否跳过 α/β 轴
// 直线单轴模式：仅 X 变化时跳过 β 轴（Y 不动），仅 Y 变化时跳过 α 轴
func computeAxisSkip(layout types.TraversalLayout) (skipAlpha, skipBeta bool) {
	if layout.Pattern != types.TraversalPatternLine || layout.Line == nil {
		return false, false
	}
	single, isXChange := isLineSingleAxis(layout.Line)
	if !single {
		return false, false
	}
	// 仅 X 变化 → 跳过 β（Y 不动）
	if isXChange {
		return false, true
	}
	// 仅 Y 变化 → 跳过 α（X 不动）
	return true, false
}
