package five_hole

import (
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"

	"yx-daq/internal/types"
)

// MotionCoordinator 多探针同步运动协调器
// 负责 3 根探针各自双轴并行移动，等待最慢探针完成
// 五孔模型：每根探针配置 MotionX/MotionY，分别对应数据点的 X/Y 坐标方向
// 物理轴完全由每根探针的 MotionX/MotionY 决定，测试配置不再指定物理轴
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
// - 矩形/自定义：point.X→MotionX, point.Y→MotionY（强制对应）
// - 直线：仅驱动 MotionX（target=point.X），MotionY 不参与
// - 扇面：R→MotionX, θ→MotionY（强制对应）
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

	// 解析本次需要驱动的轴任务
	tasks, err := buildMoveTasks(point, probes, layout)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(tasks))

	for _, t := range tasks {
		wg.Add(1)
		go func(task moveTask) {
			defer wg.Done()
			if err := mc.mover(task.controllerID, task.axis, task.target); err != nil {
				errChan <- fmt.Errorf("探针%s %s轴(%s)移动到 %.2f 失败: %w",
					task.probeID, task.direction, task.axis, task.target, err)
			}
		}(t)
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
		for _, t := range tasks {
			wg.Add(1)
			go func(task moveTask) {
				defer wg.Done()
				if err := mc.waiter(task.controllerID, task.axis, motionTimeoutMs); err != nil {
					slog.Warn("五孔: 等待轴运动完成失败",
						"probeID", task.probeID, "direction", task.direction, "axis", task.axis, "err", err)
				}
			}(t)
		}
		wg.Wait()
	}

	return nil
}

// moveTask 单轴运动任务
type moveTask struct {
	probeID      string
	direction    string // X / Y
	controllerID string
	axis         types.AxisName
	target       float64
}

// buildMoveTasks 根据布点配置构建本次需要驱动的轴任务列表
// 物理轴强制对应：point.X→MotionX, point.Y→MotionY（扇面 R→MotionX, θ→MotionY）
// 直线模式仅驱动 MotionX，MotionY 不参与
func buildMoveTasks(point types.TraversalPoint, probes []types.FiveHoleProbeConfig, layout types.TraversalLayout) ([]moveTask, error) {
	tasks := make([]moveTask, 0, len(probes)*2)

	switch layout.Pattern {
	case types.TraversalPatternRectangle:
		if layout.Rectangle == nil {
			return nil, fmt.Errorf("矩形布点缺少配置")
		}
		for _, probe := range probes {
			if !probe.Enabled {
				continue
			}
			tasks = append(tasks, moveTask{probeID: probe.ProbeID, direction: "X", controllerID: probe.MotionX.ControllerID, axis: probe.MotionX.Axis, target: point.X})
			tasks = append(tasks, moveTask{probeID: probe.ProbeID, direction: "Y", controllerID: probe.MotionY.ControllerID, axis: probe.MotionY.Axis, target: point.Y})
		}

	case types.TraversalPatternLine:
		if layout.Line == nil {
			return nil, fmt.Errorf("直线布点缺少配置")
		}
		// 直线模式：只用 MotionX，target=point.X；MotionY 不参与
		for _, probe := range probes {
			if !probe.Enabled {
				continue
			}
			tasks = append(tasks, moveTask{probeID: probe.ProbeID, direction: "X", controllerID: probe.MotionX.ControllerID, axis: probe.MotionX.Axis, target: point.X})
		}

	case types.TraversalPatternCustom:
		// 自定义布点按完整 X/Y 双轴运动
		for _, probe := range probes {
			if !probe.Enabled {
				continue
			}
			tasks = append(tasks, moveTask{probeID: probe.ProbeID, direction: "X", controllerID: probe.MotionX.ControllerID, axis: probe.MotionX.Axis, target: point.X})
			tasks = append(tasks, moveTask{probeID: probe.ProbeID, direction: "Y", controllerID: probe.MotionY.ControllerID, axis: probe.MotionY.Axis, target: point.Y})
		}

	case types.TraversalPatternFan:
		if layout.Fan == nil {
			return nil, fmt.Errorf("扇形布点缺少配置")
		}
		// 从相对笛卡尔坐标反推极坐标目标值
		dr := math.Sqrt(point.X*point.X + point.Y*point.Y)
		rTarget := dr + layout.Fan.RStart
		thetaRel := math.Atan2(point.Y, point.X) * 180 / math.Pi
		thetaTarget := thetaRel + layout.Fan.ThetaStart
		for _, probe := range probes {
			if !probe.Enabled {
				continue
			}
			tasks = append(tasks, moveTask{probeID: probe.ProbeID, direction: "R", controllerID: probe.MotionX.ControllerID, axis: probe.MotionX.Axis, target: rTarget})
			tasks = append(tasks, moveTask{probeID: probe.ProbeID, direction: "θ", controllerID: probe.MotionY.ControllerID, axis: probe.MotionY.Axis, target: thetaTarget})
		}
	}

	return tasks, nil
}

// ReturnProbesToInitialPositions 把每根探针的 X/Y 轴移动回各自初始位置
// 与 MoveAllProbesToPoint 区别：
// - 每根探针使用各自的 (X=MotionX初始位置, Y=MotionY初始位置)
// - 不应用直线单轴模式跳过逻辑（初始位置 X/Y 都可能需要恢复）
func (mc *MotionCoordinator) ReturnProbesToInitialPositions(
	initialPositions map[string]types.TraversalPoint,
	probes []types.FiveHoleProbeConfig,
	motionTimeoutMs int,
) error {
	if mc.mover == nil {
		return fmt.Errorf("motion mover not set")
	}
	if motionTimeoutMs <= 0 {
		motionTimeoutMs = 30000
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(probes)*2)

	// initiated 跟踪成功发起运动的探针，waiter 阶段仅等待这些轴
	initiated := make([]types.FiveHoleProbeConfig, 0, len(probes))

	for _, probe := range probes {
		if !probe.Enabled {
			continue
		}
		initial, ok := initialPositions[probe.ProbeID]
		if !ok {
			continue
		}
		initiated = append(initiated, probe)
		// X 方向回到初始 X
		wg.Add(1)
		go func(p types.FiveHoleProbeConfig, posX float64) {
			defer wg.Done()
			if err := mc.mover(p.MotionX.ControllerID, p.MotionX.Axis, posX); err != nil {
				errChan <- fmt.Errorf("探针%s X方向轴(%s)返回初始位置 %.2f 失败: %w", p.ProbeID, p.MotionX.Axis, posX, err)
			}
		}(probe, initial.X)
		// Y 方向回到初始 Y
		wg.Add(1)
		go func(p types.FiveHoleProbeConfig, posY float64) {
			defer wg.Done()
			if err := mc.mover(p.MotionY.ControllerID, p.MotionY.Axis, posY); err != nil {
				errChan <- fmt.Errorf("探针%s Y方向轴(%s)返回初始位置 %.2f 失败: %w", p.ProbeID, p.MotionY.Axis, posY, err)
			}
		}(probe, initial.Y)
	}

	wg.Wait()
	close(errChan)

	// 收集 mover 阶段错误（不 early return：已成功发起的运动仍需等待完成）
	var moverErrs []string
	for err := range errChan {
		moverErrs = append(moverErrs, err.Error())
	}

	// 等待所有已发起运动完成（即使部分 mover 失败，成功的轴仍在运动，必须等待）
	if mc.waiter != nil {
		waitWg := sync.WaitGroup{}
		for _, probe := range initiated {
			waitWg.Add(1)
			go func(p types.FiveHoleProbeConfig) {
				defer waitWg.Done()
				if err := mc.waiter(p.MotionX.ControllerID, p.MotionX.Axis, motionTimeoutMs); err != nil {
					slog.Warn("五孔: 等待 X方向轴返回初始位置完成失败", "probeID", p.ProbeID, "axis", p.MotionX.Axis, "err", err)
				}
			}(probe)
			waitWg.Add(1)
			go func(p types.FiveHoleProbeConfig) {
				defer waitWg.Done()
				if err := mc.waiter(p.MotionY.ControllerID, p.MotionY.Axis, motionTimeoutMs); err != nil {
					slog.Warn("五孔: 等待 Y方向轴返回初始位置完成失败", "probeID", p.ProbeID, "axis", p.MotionY.Axis, "err", err)
				}
			}(probe)
		}
		waitWg.Wait()
	}

	if len(moverErrs) > 0 {
		return fmt.Errorf("多轴返回初始位置失败: %s", strings.Join(moverErrs, "; "))
	}

	return nil
}
