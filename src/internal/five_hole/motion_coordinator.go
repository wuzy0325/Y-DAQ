package five_hole

import (
	"fmt"
	"strings"
	"sync"

	"yx-daq/internal/types"
)

// MotionCoordinator 多探针同步运动协调器
// 负责各探针（1-8 根）双轴并行移动，等待最慢探针完成
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
// - 矩形/自定义：point.X→MotionX, point.Y→MotionY（强制对应），双轴同相位并行
// - 直线：仅驱动 MotionX（target=point.X），MotionY 不参与
// - 扇面：R→MotionX, θ→MotionY（强制对应），分相位顺序运动：
//   先旋转（θ 相）到位，等待完成后平移（R 相），平移完成后才进入下一布点（下一次旋转），
//   避免旋转/平移两轴同时插补走出螺旋轨迹，也避免同控制器并发命令丢失
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

	// 扇形布点：分相位串行（先旋转θ，后平移R）
	if layout.Pattern == types.TraversalPatternFan {
		phases := splitPhasesByDirection(tasks, directionTheta)
		if err := mc.movePhase(phases[0], motionTimeoutMs); err != nil {
			return fmt.Errorf("扇形布点旋转(θ)相位失败: %w", err)
		}
		if err := mc.movePhase(phases[1], motionTimeoutMs); err != nil {
			return fmt.Errorf("扇形布点平移(R)相位失败: %w", err)
		}
		return nil
	}

	// 矩形/直线/自定义：全部轴同相位并行移动
	return mc.movePhase(tasks, motionTimeoutMs)
}

// movePhase 并行执行同一相位内的所有轴任务：并行发起移动 → 等待全部完成
// mover 阶段即使部分失败也不提前返回（已成功发起的运动必须等待完成后才能进入下一相位/布点）
// waiter 失败（如超时）同样收集为错误返回：轴未确认到位时不得进入下一相位/下一布点，
// 否则扇形模式下 θ 相未到位就发起 R 相会重新出现双轴并发（复现"轴位没有走"问题）
func (mc *MotionCoordinator) movePhase(tasks []moveTask, motionTimeoutMs int) error {
	if len(tasks) == 0 {
		return nil
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

	// 等待本相位所有已发起运动完成；waiter 失败（如超时）收集为错误而非仅告警
	if mc.waiter != nil {
		waitErrChan := make(chan error, len(tasks))
		waitWg := sync.WaitGroup{}
		for _, t := range tasks {
			waitWg.Add(1)
			go func(task moveTask) {
				defer waitWg.Done()
				if err := mc.waiter(task.controllerID, task.axis, motionTimeoutMs); err != nil {
					waitErrChan <- fmt.Errorf("探针%s %s轴(%s)等待运动完成失败(超时%dms): %w",
						task.probeID, task.direction, task.axis, motionTimeoutMs, err)
				}
			}(t)
		}
		waitWg.Wait()
		close(waitErrChan)
		for err := range waitErrChan {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("多轴运动失败: %s", strings.Join(errs, "; "))
	}
	return nil
}

// splitPhasesByDirection 把任务按方向拆为两个串行相位：
// directionFirst 对应的任务进入第一相位（先行），其余进入第二相位（后行）
func splitPhasesByDirection(tasks []moveTask, directionFirst string) [][]moveTask {
	first := make([]moveTask, 0, len(tasks))
	rest := make([]moveTask, 0, len(tasks))
	for _, t := range tasks {
		if t.direction == directionFirst {
			first = append(first, t)
		} else {
			rest = append(rest, t)
		}
	}
	return [][]moveTask{first, rest}
}

// moveTask 单轴运动任务
type moveTask struct {
	probeID      string
	direction    string // X / Y / R / θ
	controllerID string
	axis         types.AxisName
	target       float64
}

// 运动方向标签（用于错误日志与扇形分相位拆分）
const (
	directionX     = "X" // 矩形/直线/自定义/返回初始位置：X 方向；扇形返回初始位置时即 R（线性）
	directionY     = "Y" // 矩形/自定义/返回初始位置：Y 方向；扇形返回初始位置时即 θ（旋转）
	directionR     = "R" // 扇形布点：半径方向（线性轴）
	directionTheta = "θ" // 扇形布点：角度方向（旋转轴）
)

// dedupeMoveTasks 按物理轴（controllerID+axis）去重
// - 同轴同目标：合并为一次移动（多探针共用同一位移机构/轴的场景，避免对同一物理轴并发发送重复命令）
// - 同轴不同目标：物理冲突，返回错误
func dedupeMoveTasks(tasks []moveTask) ([]moveTask, error) {
	idxByKey := make(map[string]int, len(tasks))
	deduped := make([]moveTask, 0, len(tasks))
	for _, t := range tasks {
		key := t.controllerID + "|" + string(t.axis)
		if idx, ok := idxByKey[key]; ok {
			if deduped[idx].target != t.target {
				return nil, fmt.Errorf("物理轴冲突: 位移机构%s的%s轴被探针%s(%s方向→%.2f)与探针%s(%s方向→%.2f)要求移动到不同位置",
					t.controllerID, t.axis, deduped[idx].probeID, deduped[idx].direction, deduped[idx].target, t.probeID, t.direction, t.target)
			}
			continue // 同轴同目标：合并，保留首个任务的 probeID 用于日志
		}
		idxByKey[key] = len(deduped)
		deduped = append(deduped, t)
	}
	return deduped, nil
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
			tasks = append(tasks, moveTask{probeID: probe.ProbeID, direction: directionX, controllerID: probe.MotionX.ControllerID, axis: probe.MotionX.Axis, target: point.X})
			tasks = append(tasks, moveTask{probeID: probe.ProbeID, direction: directionY, controllerID: probe.MotionY.ControllerID, axis: probe.MotionY.Axis, target: point.Y})
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
			tasks = append(tasks, moveTask{probeID: probe.ProbeID, direction: directionX, controllerID: probe.MotionX.ControllerID, axis: probe.MotionX.Axis, target: point.X})
		}

	case types.TraversalPatternCustom:
		// 自定义布点按完整 X/Y 双轴运动
		for _, probe := range probes {
			if !probe.Enabled {
				continue
			}
			tasks = append(tasks, moveTask{probeID: probe.ProbeID, direction: directionX, controllerID: probe.MotionX.ControllerID, axis: probe.MotionX.Axis, target: point.X})
			tasks = append(tasks, moveTask{probeID: probe.ProbeID, direction: directionY, controllerID: probe.MotionY.ControllerID, axis: probe.MotionY.Axis, target: point.Y})
		}

	case types.TraversalPatternFan:
		if layout.Fan == nil {
			return nil, fmt.Errorf("扇形布点缺少配置")
		}
		// 点位即轴坐标：X=R（半径轴绝对位置），Y=θ（角度轴绝对位置）
		for _, probe := range probes {
			if !probe.Enabled {
				continue
			}
			tasks = append(tasks, moveTask{probeID: probe.ProbeID, direction: directionR, controllerID: probe.MotionX.ControllerID, axis: probe.MotionX.Axis, target: point.X})
			tasks = append(tasks, moveTask{probeID: probe.ProbeID, direction: directionTheta, controllerID: probe.MotionY.ControllerID, axis: probe.MotionY.Axis, target: point.Y})
		}
	}

	// 按物理轴去重（共用轴位/多探针配同轴时同一物理轴只发一次命令；同轴不同目标报冲突）
	return dedupeMoveTasks(tasks)
}

// ReturnProbesToInitialPositions 把每根探针的 X/Y 轴移动回各自初始位置
// 与 MoveAllProbesToPoint 区别：
// - 每根探针使用各自的 (X=MotionX初始位置, Y=MotionY初始位置)
// - 不应用直线单轴模式跳过逻辑（初始位置 X/Y 都可能需要恢复）
// 扇形布点（layout.Pattern=Fan）时分相位返回：Y 即 θ（旋转轴）先回位，X 即 R（线性轴）后回位，
// 与正向运动顺序保持一致（先旋转后平移）
// 任务按物理轴（controllerID+axis）去重：共用轴位时同一物理轴只发一次命令
func (mc *MotionCoordinator) ReturnProbesToInitialPositions(
	initialPositions map[string]types.TraversalPoint,
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

	// 构建返回初始位置任务（每根有初始位置的启用探针 X/Y 各一条）
	// 直线布点只走X单轴，Y 映射可为空：空映射方向跳过（无轴可回）
	tasks := make([]moveTask, 0, len(probes)*2)
	for _, probe := range probes {
		if !probe.Enabled {
			continue
		}
		initial, ok := initialPositions[probe.ProbeID]
		if !ok {
			continue
		}
		if probe.MotionX.ControllerID != "" && probe.MotionX.Axis != "" {
			tasks = append(tasks, moveTask{probeID: probe.ProbeID, direction: directionX, controllerID: probe.MotionX.ControllerID, axis: probe.MotionX.Axis, target: initial.X})
		}
		if probe.MotionY.ControllerID != "" && probe.MotionY.Axis != "" {
			tasks = append(tasks, moveTask{probeID: probe.ProbeID, direction: directionY, controllerID: probe.MotionY.ControllerID, axis: probe.MotionY.Axis, target: initial.Y})
		}
	}

	// 按物理轴去重（共用轴位/多探针配同轴时同一物理轴只发一次命令；同轴不同目标报冲突）
	tasks, err := dedupeMoveTasks(tasks)
	if err != nil {
		return fmt.Errorf("返回初始位置失败: %w", err)
	}

	// 扇形布点：分相位串行返回（先旋转θ=Y，后平移R=X）
	if layout.Pattern == types.TraversalPatternFan {
		phases := splitPhasesByDirection(tasks, directionY)
		if err := mc.movePhase(phases[0], motionTimeoutMs); err != nil {
			return fmt.Errorf("扇形布点返回初始位置旋转(θ)相位失败: %w", err)
		}
		if err := mc.movePhase(phases[1], motionTimeoutMs); err != nil {
			return fmt.Errorf("扇形布点返回初始位置平移(R)相位失败: %w", err)
		}
		return nil
	}

	// 其余布点：全部轴并行返回
	return mc.movePhase(tasks, motionTimeoutMs)
}
