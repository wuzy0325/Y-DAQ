package five_hole

import (
	"testing"
	"time"

	"yx-daq/internal/types"
)

// makeTimingConfig 构造耗时预估用的最小配置（单探针，矩形/直线/扇面由 layout 决定）
func makeTimingConfig(layout types.TraversalLayout, dwellMs, samples int) types.FiveHoleTraversalConfig {
	return types.FiveHoleTraversalConfig{
		Layout:          layout,
		DwellTimeMs:     dwellMs,
		SamplesPerPoint: samples,
		Probes: []types.FiveHoleProbeConfig{
			{
				ProbeID: "p1", Enabled: true,
				MotionX: types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "X"},
				MotionY: types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "Y"},
			},
		},
	}
}

// fixedSpeed 返回所有轴同一速度的 getter
func fixedSpeed(speed float64) FiveHoleAxisSpeedGetter {
	return func(string, types.AxisName) (float64, bool) { return speed, speed > 0 }
}

// fixedPeriod 返回所有设备同一出帧周期的 getter
func fixedPeriod(periodMs int) FiveHoleDevicePeriodGetter {
	return func(string) int { return periodMs }
}

// testLineLayout 直线布点 0→10 步长 5（3 点，Y 恒为 0）
func testLineLayout() types.TraversalLayout {
	return types.TraversalLayout{
		Pattern: types.TraversalPatternLine,
		Line:    &types.LineLayout{Axis: "X", Start: 0, End: 10, Step: 5},
	}
}

// testRectLayout 矩形布点 3×3（X 外循环：0/5/10，Y 内循环：0/5/10）
func testRectLayout() types.TraversalLayout {
	return types.TraversalLayout{
		Pattern: types.TraversalPatternRectangle,
		Rectangle: &types.RectangleLayout{
			XMin: 0, XMax: 10, YMin: 0, YMax: 10,
			XSteps: []types.StepSegment{{Start: 0, End: 10, Step: 5}},
			YSteps: []types.StepSegment{{Start: 0, End: 10, Step: 5}},
		},
	}
}

// testFanLayout 扇形布点 R(0/5/10mm) × θ(0/15/30°)
func testFanLayout() types.TraversalLayout {
	return types.TraversalLayout{
		Pattern: types.TraversalPatternFan,
		Fan: &types.FanLayout{
			RSteps:     []types.StepSegment{{Start: 0, End: 10, Step: 5}},
			ThetaSteps: []types.StepSegment{{Start: 0, End: 30, Step: 15}},
		},
	}
}

func TestTimingEstimator_Line(t *testing.T) {
	// 直线 3 点：首点与初始位置重合(0mm)，后两点各 5mm；轴速 10mm/s → 每段 500ms
	// 驻留 1000ms × 3，采样 10 次 × 50ms 周期 × 3
	config := makeTimingConfig(testLineLayout(), 1000, 10)
	points, err := generatePoints(config)
	if err != nil {
		t.Fatalf("generatePoints: %v", err)
	}
	estimator := &timingEstimator{axisSpeed: fixedSpeed(10), devicePeriod: fixedPeriod(50)}
	start := map[string]types.TraversalPoint{"p1": {X: 0, Y: 0}}

	timing := estimator.estimate(config, points, start)

	if want := int64(1000); timing.EstMoveTotalMs != want {
		t.Errorf("EstMoveTotalMs = %d, want %d", timing.EstMoveTotalMs, want)
	}
	if want := int64(3000); timing.EstDwellTotalMs != want {
		t.Errorf("EstDwellTotalMs = %d, want %d", timing.EstDwellTotalMs, want)
	}
	if want := int64(1500); timing.EstSampleTotalMs != want {
		t.Errorf("EstSampleTotalMs = %d, want %d", timing.EstSampleTotalMs, want)
	}
	if want := int64(5500); timing.EstimatedTotalMs != want {
		t.Errorf("EstimatedTotalMs = %d, want %d", timing.EstimatedTotalMs, want)
	}
	if want := int64(333); timing.EstMoveMsPerPoint != want {
		t.Errorf("EstMoveMsPerPoint = %d, want %d", timing.EstMoveMsPerPoint, want)
	}
	if want := int64(500); timing.EstSampleMsPerPoint != want {
		t.Errorf("EstSampleMsPerPoint = %d, want %d", timing.EstSampleMsPerPoint, want)
	}
	if want := float64(20); timing.SampleHz != want {
		t.Errorf("SampleHz = %v, want %v", timing.SampleHz, want)
	}
}

func TestTimingEstimator_RectangleParallelAxes(t *testing.T) {
	// 矩形 3×3（X 外循环）：段距离依次为 5,5,|dx5/dy10|,5,5,|dx5/dy10|,5,5
	// 双轴并行取较大者：5mm→500ms，10mm→1000ms，合计 0+500+500+1000+500+500+1000+500+500 = 5000ms
	config := makeTimingConfig(testRectLayout(), 1000, 10)
	points, err := generatePoints(config)
	if err != nil {
		t.Fatalf("generatePoints: %v", err)
	}
	estimator := &timingEstimator{axisSpeed: fixedSpeed(10), devicePeriod: fixedPeriod(50)}
	start := map[string]types.TraversalPoint{"p1": {X: 0, Y: 0}}

	timing := estimator.estimate(config, points, start)

	if want := int64(5000); timing.EstMoveTotalMs != want {
		t.Errorf("EstMoveTotalMs = %d, want %d（双轴并行应取较大者而非相加）", timing.EstMoveTotalMs, want)
	}
	// 驻留 9×1000 + 采样 9×500
	if want := int64(9000); timing.EstDwellTotalMs != want {
		t.Errorf("EstDwellTotalMs = %d, want %d", timing.EstDwellTotalMs, want)
	}
	if want := int64(4500); timing.EstSampleTotalMs != want {
		t.Errorf("EstSampleTotalMs = %d, want %d", timing.EstSampleTotalMs, want)
	}
	if want := int64(18500); timing.EstimatedTotalMs != want {
		t.Errorf("EstimatedTotalMs = %d, want %d", timing.EstimatedTotalMs, want)
	}
	if want := int64(556); timing.EstMoveMsPerPoint != want { // round(5000/9)
		t.Errorf("EstMoveMsPerPoint = %d, want %d", timing.EstMoveMsPerPoint, want)
	}
}

func TestTimingEstimator_FanSerialPhases(t *testing.T) {
	// 扇面：θ 相（MotionY，旋转轴 30°/s）先、R 相（MotionX，线性轴 10mm/s）后，两相串行相加
	// 段耗时（0,0 起步）：0,500,500,|θ30=1000+R5=500|=1500,500,500,1500,500,500 = 6000ms
	config := makeTimingConfig(testFanLayout(), 1000, 10)
	points, err := generatePoints(config)
	if err != nil {
		t.Fatalf("generatePoints: %v", err)
	}
	speed := func(_ string, axis types.AxisName) (float64, bool) {
		if axis == types.AxisY { // 扇面 θ 方向（旋转）
			return 30, true
		}
		return 10, true
	}
	estimator := &timingEstimator{axisSpeed: speed, devicePeriod: fixedPeriod(50)}
	start := map[string]types.TraversalPoint{"p1": {X: 0, Y: 0}}

	timing := estimator.estimate(config, points, start)

	if want := int64(6000); timing.EstMoveTotalMs != want {
		t.Errorf("EstMoveTotalMs = %d, want %d（扇面应两相相加）", timing.EstMoveTotalMs, want)
	}
	if want := int64(19500); timing.EstimatedTotalMs != want {
		t.Errorf("EstimatedTotalMs = %d, want %d", timing.EstimatedTotalMs, want)
	}
}

func TestTimingEstimator_MultiProbeTakesSlowest(t *testing.T) {
	// 多探针同相位并行 → 取最慢探针：同几何下 p2 轴速 5（p1 为 10），移动耗时翻倍
	config := makeTimingConfig(testRectLayout(), 1000, 10)
	config.Probes = append(config.Probes, types.FiveHoleProbeConfig{
		ProbeID: "p2", Enabled: true,
		MotionX: types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "X"},
		MotionY: types.FiveHoleMotionAxisMapping{ControllerID: "c1", Axis: "Y"},
	})
	points, err := generatePoints(config)
	if err != nil {
		t.Fatalf("generatePoints: %v", err)
	}
	speed := func(_ string, axis types.AxisName) (float64, bool) {
		if axis == types.AxisZ {
			return 5, true
		}
		return 10, true
	}
	estimator := &timingEstimator{axisSpeed: speed, devicePeriod: fixedPeriod(50)}
	start := map[string]types.TraversalPoint{"p1": {X: 0, Y: 0}, "p2": {X: 0, Y: 0}}

	allFast := estimator.estimate(config, points, start)

	// p2 的 MotionX/MotionY 都指向 c1 的 X/Y，与 p1 相同 → 不改变耗时；
	// 改用 p2 独享慢轴的配置验证"取最大"
	config.Probes[1].MotionX = types.FiveHoleMotionAxisMapping{ControllerID: "c2", Axis: types.AxisZ}
	config.Probes[1].MotionY = types.FiveHoleMotionAxisMapping{ControllerID: "c2", Axis: types.AxisZ}
	slowP2 := estimator.estimate(config, points, start)

	if allFast.EstMoveTotalMs != 5000 {
		t.Errorf("同速双探针 EstMoveTotalMs = %d, want 5000", allFast.EstMoveTotalMs)
	}
	// p2 的 Z 轴速度 5mm/s 且 X/Y 同轴：dx=5→1000ms，dy 最大 10→2000ms，每段取较大者
	if slowP2.EstMoveTotalMs <= allFast.EstMoveTotalMs {
		t.Errorf("含慢轴探针 EstMoveTotalMs = %d，应大于同速场景 %d", slowP2.EstMoveTotalMs, allFast.EstMoveTotalMs)
	}
}

func TestTimingEstimator_SlowestDevicePeriod(t *testing.T) {
	// 多设备取最慢出帧周期：d1=20ms(50Hz)、d2=100ms(10Hz) → 采样每点 10×100ms，频率 10Hz
	config := makeTimingConfig(testLineLayout(), 1000, 10)
	config.Probes[0].ProbeChannels = []types.FiveHoleProbeChannelConfig{
		{Name: "P1", Role: types.Role5H_P1, DeviceID: "d1", Channel: 0, Enabled: true},
		{Name: "P2", Role: types.Role5H_P2, DeviceID: "d2", Channel: 1, Enabled: true},
	}
	points, err := generatePoints(config)
	if err != nil {
		t.Fatalf("generatePoints: %v", err)
	}
	period := func(deviceID string) int {
		if deviceID == "d2" {
			return 100
		}
		return 20
	}
	estimator := &timingEstimator{axisSpeed: fixedSpeed(10), devicePeriod: period}

	timing := estimator.estimate(config, points, map[string]types.TraversalPoint{"p1": {X: 0, Y: 0}})

	if want := int64(1000); timing.EstSampleMsPerPoint != want {
		t.Errorf("EstSampleMsPerPoint = %d, want %d（应取最慢设备周期 100ms）", timing.EstSampleMsPerPoint, want)
	}
	if want := float64(10); timing.SampleHz != want {
		t.Errorf("SampleHz = %v, want %v", timing.SampleHz, want)
	}
}

func TestTimingEstimator_UnknownStartPositionCountsZero(t *testing.T) {
	// 配置期（未连接控制器）无初始位置：首点移动按 0 计，后续点位照常估算
	config := makeTimingConfig(testLineLayout(), 1000, 10)
	points, err := generatePoints(config)
	if err != nil {
		t.Fatalf("generatePoints: %v", err)
	}
	estimator := &timingEstimator{axisSpeed: fixedSpeed(10), devicePeriod: fixedPeriod(50)}

	timing := estimator.estimate(config, points, nil)

	if want := int64(1000); timing.EstMoveTotalMs != want {
		t.Errorf("EstMoveTotalMs = %d, want %d（首点 0 + 5mm×2）", timing.EstMoveTotalMs, want)
	}
}

func TestTimingEstimator_NoSpeedGetterFallsBackToZeroMove(t *testing.T) {
	// 未注入轴速 getter（单元测试/未配置速度）：移动耗时按 0 计，驻留+采样仍统计
	config := makeTimingConfig(testLineLayout(), 1000, 10)
	points, err := generatePoints(config)
	if err != nil {
		t.Fatalf("generatePoints: %v", err)
	}
	estimator := &timingEstimator{devicePeriod: fixedPeriod(50)}

	timing := estimator.estimate(config, points, nil)

	if timing.EstMoveTotalMs != 0 {
		t.Errorf("EstMoveTotalMs = %d, want 0", timing.EstMoveTotalMs)
	}
	if want := int64(4500); timing.EstimatedTotalMs != want {
		t.Errorf("EstimatedTotalMs = %d, want %d（驻留 3000 + 采样 1500）", timing.EstimatedTotalMs, want)
	}
}

func TestTimingTracker_PhaseDurationsAndPoints(t *testing.T) {
	tracker := newTimingTracker()
	tracker.reset()
	tracker.addPhaseDuration(PhaseMove, 120*time.Millisecond)
	tracker.addPhaseDuration(PhaseDwell, 250*time.Millisecond)
	tracker.addPhaseDuration(PhaseSample, 80*time.Millisecond)
	tracker.addPhaseDuration(PhaseSample, -time.Second) // 非正值忽略
	tracker.addPoint()

	snapshot := tracker.snapshot()
	if want := int64(120); snapshot.ActualMoveMs != want {
		t.Errorf("ActualMoveMs = %d, want %d", snapshot.ActualMoveMs, want)
	}
	if want := int64(250); snapshot.ActualDwellMs != want {
		t.Errorf("ActualDwellMs = %d, want %d", snapshot.ActualDwellMs, want)
	}
	if want := int64(80); snapshot.ActualSampleMs != want {
		t.Errorf("ActualSampleMs = %d, want %d", snapshot.ActualSampleMs, want)
	}
	if want := 1; snapshot.ActualPoints != want {
		t.Errorf("ActualPoints = %d, want %d", snapshot.ActualPoints, want)
	}
}

func TestTimingTracker_PausedTimeExcludedFromElapsed(t *testing.T) {
	tracker := newTimingTracker()
	tracker.reset()

	time.Sleep(20 * time.Millisecond)
	tracker.beginPause()
	time.Sleep(60 * time.Millisecond) // 暂停时长不应计入
	tracker.endPause()
	time.Sleep(20 * time.Millisecond)

	elapsed := tracker.snapshot().ActualElapsedMs
	// 非暂停部分约 40ms：允许调度抖动，但必须明显小于总墙钟 ~100ms 且不小于 20ms
	if elapsed < 20 || elapsed > 75 {
		t.Errorf("ActualElapsedMs = %d, want ~40（暂停 60ms 应被扣除）", elapsed)
	}
}

func TestTimingTracker_StopFreezesElapsed(t *testing.T) {
	tracker := newTimingTracker()
	tracker.reset()
	time.Sleep(10 * time.Millisecond)
	tracker.stop()
	frozen := tracker.snapshot().ActualElapsedMs
	time.Sleep(40 * time.Millisecond)
	if got := tracker.snapshot().ActualElapsedMs; got != frozen {
		t.Errorf("stop 后 ActualElapsedMs 应冻结在 %d，实际 %d", frozen, got)
	}
}

func TestTestManager_BeginPhaseExcludesPause(t *testing.T) {
	manager := NewTestManager(nil)
	manager.timing.reset()

	done := manager.BeginPhase(PhaseDwell)
	time.Sleep(10 * time.Millisecond)
	manager.timing.beginPause()
	time.Sleep(50 * time.Millisecond)
	manager.timing.endPause()
	time.Sleep(10 * time.Millisecond)
	done()
	done() // 重复调用只生效一次

	if got := manager.TimingSnapshot().ActualDwellMs; got < 12 || got > 45 {
		t.Errorf("ActualDwellMs = %d, want ~20（阶段内暂停 50ms 应被扣除，且不重复累计）", got)
	}
}

// fakeTimingPublisher 捕获进度事件，用于验证事件携带耗时快照
type fakeTimingPublisher struct {
	progress []types.FiveHoleTraversalProgressEvent
}

func (p *fakeTimingPublisher) EmitProgress(event types.FiveHoleTraversalProgressEvent) {
	p.progress = append(p.progress, event)
}
func (p *fakeTimingPublisher) EmitComplete(types.FiveHoleTraversalCompleteEvent) {}
func (p *fakeTimingPublisher) EmitError(types.FiveHoleTraversalErrorEvent)       {}
func (p *fakeTimingPublisher) EmitRealtime(types.FiveHoleTraversalRealtimeEvent) {}

func TestTestManager_EmitProgressCarriesTiming(t *testing.T) {
	publisher := &fakeTimingPublisher{}
	manager := NewTestManager(publisher)

	manager.SetTimingEstimate(types.FiveHoleTraversalTiming{
		EstimatedTotalMs: 12345,
		EstMoveTotalMs:   1000,
		EstDwellTotalMs:  10000,
		EstSampleTotalMs: 1345,
	})
	manager.timing.reset()
	manager.EmitProgress("task-1", 10, 2, 20, 1, 2, "moving")

	if len(publisher.progress) != 1 {
		t.Fatalf("expected 1 progress event, got %d", len(publisher.progress))
	}
	timing := publisher.progress[0].Timing
	if timing == nil {
		t.Fatal("progress event Timing 不应为 nil（已写入预估）")
	}
	if timing.EstimatedTotalMs != 12345 {
		t.Errorf("EstimatedTotalMs = %d, want 12345", timing.EstimatedTotalMs)
	}
	if timing.ActualElapsedMs < 0 {
		t.Errorf("ActualElapsedMs = %d, want >= 0", timing.ActualElapsedMs)
	}
}

func TestTestManager_StatusCarriesTiming(t *testing.T) {
	manager := NewTestManager(nil)
	// 未开始且无预估：不返回假的全 0 快照
	if manager.GetStatus().Timing != nil {
		t.Error("未开始且无预估时 Timing 应为 nil")
	}

	manager.SetTimingEstimate(types.FiveHoleTraversalTiming{EstimatedTotalMs: 500})
	status := manager.GetStatus()
	if status.Timing == nil || status.Timing.EstimatedTotalMs != 500 {
		t.Fatalf("GetStatus().Timing = %+v, want EstimatedTotalMs=500", status.Timing)
	}
}

func TestService_EstimateTiming_InvalidLayoutReturnsZero(t *testing.T) {
	service := NewFiveHoleTraversalService(nil)
	// 未生成任何点位（布点配置为空）→ 预估返回零值而非崩溃
	if timing := service.EstimateTiming(types.FiveHoleTraversalConfig{}); timing != (types.FiveHoleTraversalTiming{}) {
		t.Errorf("非法配置预估应为零值，实际 %+v", timing)
	}
}

func TestService_EstimateTiming_UsesInjectedGetters(t *testing.T) {
	service := NewFiveHoleTraversalService(nil)
	service.SetAxisSpeedGetter(fixedSpeed(10))
	service.SetDevicePeriodGetter(fixedPeriod(50))

	config := makeTimingConfig(testLineLayout(), 1000, 10)
	timing := service.EstimateTiming(config)

	// 配置期无初始位置：首点 0 + 5mm×2 = 1000ms；驻留 3000；采样 1500
	if want := int64(5500); timing.EstimatedTotalMs != want {
		t.Errorf("EstimatedTotalMs = %d, want %d", timing.EstimatedTotalMs, want)
	}
}
