package five_hole

import (
	"math"
	"sync"
	"time"

	"yx-daq/internal/types"
)

// ==================== 五孔移位测试耗时：预估 + 实测 ====================
//
// 预估口径（理论值，启动前即可算出，见 EstimateTiming）：
//
//	移动 = 点位间距离 / 轴配置最大速度（同相位多轴并行取最大；扇面 θ 相与 R 相串行相加）
//	驻留 = 配置驻留时间
//	采样 = 采样次数 × 最慢采集设备出帧周期（采样频率 = 1000/PeriodMs）
//
// 实测口径（runTestLoop 中逐点位分段计时）：
//
//	移动/驻留/采样 三段分别计时，暂停时长不计入；总耗时 = 测试开始至今扣除暂停。

// fiveHoleDefaultDevicePeriodMs 采集设备未配置周期时的默认出帧周期（与 DeviceProfile.PeriodMs 约定一致）
const fiveHoleDefaultDevicePeriodMs = 50

// PhaseKind 实测耗时阶段（移动/驻留/采样）
type PhaseKind int

const (
	// PhaseMove 运动阶段（移动到目标点位）
	PhaseMove PhaseKind = iota
	// PhaseDwell 驻留阶段（到达点位后等待流场稳定）
	PhaseDwell
	// PhaseSample 采样阶段（等新帧 + 读取各探针数据）
	PhaseSample
)

// timingEstimator 理论耗时预估器（纯计算，无副作用）
type timingEstimator struct {
	axisSpeed    FiveHoleAxisSpeedGetter    // 轴最大速度（线性 mm/s，旋转 °/s）
	devicePeriod FiveHoleDevicePeriodGetter // 设备出帧周期（ms），未知时回退默认 50ms
}

// estimate 计算预估耗时
// startPositions 为各探针测试前位置（map[probeID]TraversalPoint）；为 nil 时首点移动按 0 计
// （配置期尚未连接控制器，当前位置未知）。
func (e *timingEstimator) estimate(
	config types.FiveHoleTraversalConfig,
	points []types.TraversalPoint,
	startPositions map[string]types.TraversalPoint,
) types.FiveHoleTraversalTiming {
	periodMs := e.slowestPeriodMs(config)
	sampleMsPerPoint := float64(config.SamplesPerPoint * periodMs)
	dwellMsPerPoint := float64(config.DwellTimeMs)

	// prev 为上一布点各探针位置：首点用测试前位置（map 拷贝，避免污染调用方的初始位置快照），
	// 初始位置未知（配置期）时首点移动距离按 0 计
	prev := make(map[string]types.TraversalPoint, len(config.Probes))
	for probeID, pos := range startPositions {
		prev[probeID] = pos
	}
	var moveTotalMs float64
	for _, p := range points {
		moveTotalMs += e.pointMoveMs(p, prev, config)
		for _, probe := range config.Probes {
			if probe.Enabled {
				prev[probe.ProbeID] = p
			}
		}
	}

	n := float64(len(points))
	timing := types.FiveHoleTraversalTiming{
		EstMoveTotalMs:      int64(math.Round(moveTotalMs)),
		EstDwellTotalMs:     int64(math.Round(dwellMsPerPoint * n)),
		EstSampleTotalMs:    int64(math.Round(sampleMsPerPoint * n)),
		EstDwellMsPerPoint:  int64(config.DwellTimeMs),
		EstSampleMsPerPoint: int64(config.SamplesPerPoint * periodMs),
		SampleHz:            float64(1000) / float64(periodMs),
	}
	if len(points) > 0 {
		timing.EstMoveMsPerPoint = int64(math.Round(moveTotalMs / n))
	}
	timing.EstimatedTotalMs = timing.EstMoveTotalMs + timing.EstDwellTotalMs + timing.EstSampleTotalMs
	return timing
}

// slowestPeriodMs 返回配置涉及的最慢采集设备出帧周期（多设备取最大周期，即采样最慢的一台）
// 采样循环要求所有涉及设备都出新帧才采一次，因此实际节奏由最慢设备决定
func (e *timingEstimator) slowestPeriodMs(config types.FiveHoleTraversalConfig) int {
	slowest := 0
	for _, deviceID := range CollectDeviceIDs(config) {
		period := 0
		if e.devicePeriod != nil {
			period = e.devicePeriod(deviceID)
		}
		if period <= 0 {
			period = fiveHoleDefaultDevicePeriodMs
		}
		if period > slowest {
			slowest = period
		}
	}
	if slowest <= 0 {
		slowest = fiveHoleDefaultDevicePeriodMs
	}
	return slowest
}

// pointMoveMs 计算移动到该点位所需时间（ms）
// 与 MotionCoordinator.buildMoveTasks 的运动模型保持一致：
//   - 直线：仅 MotionX 单轴运动
//   - 矩形/自定义：MotionX/MotionY 同相位并行 → 取较大者
//   - 扇面：先 θ（MotionY）后 R（MotionX）分相位串行 → 两相相加
//
// 多探针同相位并行 → 取各探针中的较大者；prev 中无该探针位置（未知）时该探针按 0 计
func (e *timingEstimator) pointMoveMs(
	point types.TraversalPoint,
	prev map[string]types.TraversalPoint,
	config types.FiveHoleTraversalConfig,
) float64 {
	if e.axisSpeed == nil {
		return 0
	}

	// axisTimeMs 单轴运动耗时（距离/速度换算为 ms）；速度未知或为 0 时按 0 计
	axisTimeMs := func(controllerID string, axis types.AxisName, distance float64) float64 {
		if distance <= 0 {
			return 0
		}
		speed, ok := e.axisSpeed(controllerID, axis)
		if !ok || speed <= 0 {
			return 0
		}
		return distance / speed * 1000
	}

	var xTimeMs, yTimeMs, thetaTimeMs, rTimeMs float64
	for _, probe := range config.Probes {
		if !probe.Enabled {
			continue
		}
		last, ok := prev[probe.ProbeID]
		if !ok {
			continue // 该探针无上一位置（如新增探针）：移动量未知，按 0 计
		}
		dx := math.Abs(point.X - last.X)
		dy := math.Abs(point.Y - last.Y)

		switch config.Layout.Pattern {
		case types.TraversalPatternLine:
			xTimeMs = math.Max(xTimeMs, axisTimeMs(probe.MotionX.ControllerID, probe.MotionX.Axis, dx))
		case types.TraversalPatternFan:
			thetaTimeMs = math.Max(thetaTimeMs, axisTimeMs(probe.MotionY.ControllerID, probe.MotionY.Axis, dy))
			rTimeMs = math.Max(rTimeMs, axisTimeMs(probe.MotionX.ControllerID, probe.MotionX.Axis, dx))
		default: // 矩形 / 自定义
			xTimeMs = math.Max(xTimeMs, axisTimeMs(probe.MotionX.ControllerID, probe.MotionX.Axis, dx))
			yTimeMs = math.Max(yTimeMs, axisTimeMs(probe.MotionY.ControllerID, probe.MotionY.Axis, dy))
		}
	}

	switch config.Layout.Pattern {
	case types.TraversalPatternFan:
		return thetaTimeMs + rTimeMs // 两相位串行
	case types.TraversalPatternLine:
		return xTimeMs
	default:
		return math.Max(xTimeMs, yTimeMs) // 同相位并行
	}
}

// ==================== 实测计时器 ====================

// timingTracker 实测耗时累计器（含暂停扣除）
// 并发模型：runTestLoop 单协程写、进度事件/状态查询多协程读，全部经内部互斥锁保护
type timingTracker struct {
	mu sync.Mutex

	// estimate 预估静态部分（测试启动时由 Service 写入，reset 不清空）
	estimate types.FiveHoleTraversalTiming

	startedAt   time.Time     // 测试开始时刻；零值表示未开始
	endedAt     time.Time     // 测试结束时刻；非零时总耗时不再增长
	pausedTotal time.Duration // 已累计的暂停时长
	pauseStart  time.Time     // 非零表示当前处于暂停中

	moveTotal   time.Duration
	dwellTotal  time.Duration
	sampleTotal time.Duration
	points      int
}

// newTimingTracker 创建实测计时器（未开始状态）
func newTimingTracker() *timingTracker {
	return &timingTracker{}
}

// setEstimate 写入预估静态部分（实测累计值由 snapshot 从计时字段派生，不存放在 estimate 中）
func (t *timingTracker) setEstimate(estimate types.FiveHoleTraversalTiming) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.estimate = estimate
}

// reset 重置实测累计并开始计时（保留预估静态部分）
func (t *timingTracker) reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.startedAt = time.Now()
	t.endedAt = time.Time{}
	t.pausedTotal = 0
	t.pauseStart = time.Time{}
	t.moveTotal = 0
	t.dwellTotal = 0
	t.sampleTotal = 0
	t.points = 0
}

// stop 冻结总耗时（测试结束/停止/致命错误时调用，可重复调用）
func (t *timingTracker) stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.endPauseLocked()
	if t.startedAt.IsZero() || !t.endedAt.IsZero() {
		return
	}
	t.endedAt = time.Now()
}

// beginPause 标记暂停开始（重复调用仅首次生效）
func (t *timingTracker) beginPause() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.pauseStart.IsZero() && !t.startedAt.IsZero() {
		t.pauseStart = time.Now()
	}
}

// endPause 结束暂停并累计暂停时长（未处于暂停时为空操作）
func (t *timingTracker) endPause() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.endPauseLocked()
}

func (t *timingTracker) endPauseLocked() {
	if t.pauseStart.IsZero() {
		return
	}
	t.pausedTotal += time.Since(t.pauseStart)
	t.pauseStart = time.Time{}
}

// pausedTotalNow 返回含当前进行中暂停的累计暂停时长（用于扣除阶段内的暂停）
func (t *timingTracker) pausedTotalNow() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	total := t.pausedTotal
	if !t.pauseStart.IsZero() {
		total += time.Since(t.pauseStart)
	}
	return total
}

// addPhaseDuration 累计某一阶段的实测耗时
func (t *timingTracker) addPhaseDuration(kind PhaseKind, d time.Duration) {
	if d <= 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	switch kind {
	case PhaseMove:
		t.moveTotal += d
	case PhaseDwell:
		t.dwellTotal += d
	case PhaseSample:
		t.sampleTotal += d
	}
}

// addPoint 累计已完成点位数
func (t *timingTracker) addPoint() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.points++
}

// snapshot 返回耗时快照（预估静态部分 + 当前实测累计）
func (t *timingTracker) snapshot() types.FiveHoleTraversalTiming {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := t.estimate
	out.ActualElapsedMs = t.elapsedLocked().Milliseconds()
	out.ActualMoveMs = t.moveTotal.Milliseconds()
	out.ActualDwellMs = t.dwellTotal.Milliseconds()
	out.ActualSampleMs = t.sampleTotal.Milliseconds()
	out.ActualPoints = t.points
	return out
}

// elapsedLocked 计算已用时间（扣除暂停；已结束时冻结在 endedAt）
func (t *timingTracker) elapsedLocked() time.Duration {
	if t.startedAt.IsZero() {
		return 0
	}
	end := time.Now()
	if !t.endedAt.IsZero() {
		end = t.endedAt
	}
	elapsed := end.Sub(t.startedAt) - t.pausedTotal
	if !t.pauseStart.IsZero() && t.endedAt.IsZero() {
		elapsed -= end.Sub(t.pauseStart)
	}
	if elapsed < 0 {
		return 0
	}
	return elapsed
}
