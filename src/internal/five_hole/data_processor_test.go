package five_hole

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// makeSeqGetter 构造按给定帧序号返回的 batchGetter，并统计调用次数
func makeSeqGetter(seq *int64, calls *int32) FiveHoleMultiDeviceBatchGetter {
	return func(deviceID string, channels []int) (map[int]float64, int64, error) {
		if calls != nil {
			atomic.AddInt32(calls, 1)
		}
		return map[int]float64{0: 42}, atomic.LoadInt64(seq), nil
	}
}

// TestWaitForFreshData_EventDriven_NoPolling 注入新帧通知后等待由信号驱动：
// 帧序号在 120ms 后（远超旧实现的 20ms 轮询间隔）前进并投递信号，等待应立即返回，
// 且 batchGetter 只被调用极少数次（未按固定间隔轮询）
func TestWaitForFreshData_EventDriven_NoPolling(t *testing.T) {
	dp := NewDataProcessor()
	var seq int64 = 1000
	var calls int32
	dp.SetBatchGetter(makeSeqGetter(&seq, &calls))

	arrival := make(chan struct{}, 1)
	dp.SetDataArrivalSignal(arrival)

	go func() {
		time.Sleep(120 * time.Millisecond)
		atomic.StoreInt64(&seq, 2000)
		arrival <- struct{}{}
	}()

	start := time.Now()
	got, err := dp.WaitForFreshData([]string{"d1"}, map[string]int64{"d1": 1000}, 2*time.Second)
	if err != nil {
		t.Fatalf("WaitForFreshData 出错: %v", err)
	}
	if got["d1"] != 2000 {
		t.Fatalf("应返回新帧序号 2000，实得 %d", got["d1"])
	}
	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Errorf("不应在信号到达前返回（耗时 %v）", elapsed)
	}
	if elapsed := time.Since(start); elapsed > 600*time.Millisecond {
		t.Errorf("信号到达后应立即返回（耗时 %v）", elapsed)
	}
	// 事件驱动：等待前复查 1 次 + 收到信号后复查 1 次；轮询实现 120ms 内约 6-7 次
	if n := atomic.LoadInt32(&calls); n > 4 {
		t.Errorf("事件驱动下不应按固定间隔轮询：batchGetter 被调用 %d 次", n)
	}
}

// TestWaitForFreshData_PollingFallback 未注入通知通道时保持轮询兜底（测试/无 Hub 场景）：
// 首次读到旧帧序号 → 等待一个轮询间隔后复查，命中新帧返回（既不是忙等，也不是立即返回）
func TestWaitForFreshData_PollingFallback(t *testing.T) {
	dp := NewDataProcessor()
	var calls int32
	dp.SetBatchGetter(func(deviceID string, channels []int) (map[int]float64, int64, error) {
		if atomic.AddInt32(&calls, 1) == 1 {
			return map[int]float64{0: 42}, 100, nil // 首次：尚未出新帧
		}
		return map[int]float64{0: 42}, 101, nil
	})

	start := time.Now()
	got, err := dp.WaitForFreshData([]string{"d1"}, map[string]int64{"d1": 100}, time.Second)
	if err != nil {
		t.Fatalf("轮询兜底路径出错: %v", err)
	}
	if got["d1"] != 101 {
		t.Fatalf("应返回帧序号 101，实得 %d", got["d1"])
	}
	if elapsed := time.Since(start); elapsed < dataArrivalPollInterval/2 {
		t.Errorf("轮询兜底应等待一个轮询间隔，不应忙等（耗时 %v）", elapsed)
	}
	if n := atomic.LoadInt32(&calls); n > 5 {
		t.Errorf("轮询兜底复查次数异常：%d", n)
	}
}

// TestWaitForFreshData_SignalWithoutNewFrame_StillTimesOut 只有信号、帧序号从不前进时，
// 仍必须在 timeout 内判定"数据停滞"：超时从调用时刻起算，不因信号而顺延
// （否则一台高频设备的心跳会掩盖另一台设备停摆）
func TestWaitForFreshData_SignalWithoutNewFrame_StillTimesOut(t *testing.T) {
	dp := NewDataProcessor()
	var seq int64 = 1000
	dp.SetBatchGetter(makeSeqGetter(&seq, nil))

	arrival := make(chan struct{}, 1)
	dp.SetDataArrivalSignal(arrival)

	stop := make(chan struct{})
	defer close(stop)
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-time.After(10 * time.Millisecond):
				select {
				case arrival <- struct{}{}:
				default:
				}
			}
		}
	}()

	start := time.Now()
	_, err := dp.WaitForFreshData([]string{"d1"}, map[string]int64{"d1": 1000}, 100*time.Millisecond)
	elapsed := time.Since(start)
	if !errors.Is(err, ErrDataStagnant) {
		t.Fatalf("期望 ErrDataStagnant，实得 %v", err)
	}
	if elapsed > 400*time.Millisecond {
		t.Errorf("持续信号不应顺延停滞判定（耗时 %v）", elapsed)
	}
}

// TestWaitForFreshData_MultiDevice_WaitsForSlowest 必须等"所有参与设备"都出新帧：
// 一台设备持续出新帧并持续投递信号，另一台停滞时不得提前返回；慢设备出新帧后立即返回
func TestWaitForFreshData_MultiDevice_WaitsForSlowest(t *testing.T) {
	dp := NewDataProcessor()
	var fastSeq, slowSeq int64 = 10, 20
	dp.SetBatchGetter(func(deviceID string, channels []int) (map[int]float64, int64, error) {
		switch deviceID {
		case "fast":
			return nil, atomic.AddInt64(&fastSeq, 1), nil // 每次复查都前进：模拟高频设备
		case "slow":
			return nil, atomic.LoadInt64(&slowSeq), nil // 固定不变：模拟停摆设备
		}
		return nil, 0, nil
	})

	arrival := make(chan struct{}, 1)
	dp.SetDataArrivalSignal(arrival)

	stop := make(chan struct{})
	defer close(stop)
	// 持续投递"有新帧"的信号（模拟高频设备的心跳），不得因此提前返回
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-time.After(5 * time.Millisecond):
				select {
				case arrival <- struct{}{}:
				default:
				}
			}
		}
	}()

	type result struct {
		seq map[string]int64
		err error
	}
	done := make(chan result, 1)
	go func() {
		seq, err := dp.WaitForFreshData([]string{"fast", "slow"}, map[string]int64{"fast": 10, "slow": 20}, 3*time.Second)
		done <- result{seq, err}
	}()

	select {
	case r := <-done:
		t.Fatalf("慢设备未出新帧时不应返回（got=%v err=%v）", r.seq, r.err)
	case <-time.After(200 * time.Millisecond):
	}

	// 慢设备出新帧 → 应立即返回，并带两者最新序号
	atomic.StoreInt64(&slowSeq, 21)
	arrival <- struct{}{}
	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("不应出错: %v", r.err)
		}
		if r.seq["slow"] != 21 {
			t.Fatalf("slow 帧序号应为 21，实得 %d", r.seq["slow"])
		}
		if r.seq["fast"] <= 10 {
			t.Fatalf("fast 帧序号应已前进，实得 %d", r.seq["fast"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("慢设备出新帧后应立即返回")
	}
}

// TestWaitForFreshData_GetterError_Propagates 读取失败时直接返回错误（由调用方决定停止/暂停）
func TestWaitForFreshData_GetterError_Propagates(t *testing.T) {
	dp := NewDataProcessor()
	wantErr := errors.New("读取失败")
	dp.SetBatchGetter(func(deviceID string, channels []int) (map[int]float64, int64, error) {
		return nil, 0, wantErr
	})
	dp.SetDataArrivalSignal(make(chan struct{}, 1))

	if _, err := dp.WaitForFreshData([]string{"d1"}, map[string]int64{"d1": 1000}, time.Second); !errors.Is(err, wantErr) {
		t.Fatalf("期望透传读取错误，实得 %v", err)
	}
}

// TestSetDataArrivalSignal_NilKeepsServing 传 nil 恢复轮询语义（不 panic、仍能等到新帧）
func TestSetDataArrivalSignal_NilKeepsServing(t *testing.T) {
	dp := NewDataProcessor()
	var seq int64 = 1000
	var calls int32
	dp.SetBatchGetter(makeSeqGetter(&seq, &calls))
	dp.SetDataArrivalSignal(make(chan struct{}, 1))
	dp.SetDataArrivalSignal(nil)

	go func() {
		time.Sleep(30 * time.Millisecond)
		atomic.StoreInt64(&seq, 2000)
	}()

	got, err := dp.WaitForFreshData([]string{"d1"}, map[string]int64{"d1": 1000}, time.Second)
	if err != nil {
		t.Fatalf("WaitForFreshData 出错: %v", err)
	}
	if got["d1"] != 2000 {
		t.Fatalf("应返回帧序号 2000，实得 %d", got["d1"])
	}
}

// TestService_SetDataArrivalSignal_WiresDataProcessor Service 层注入通道必须真正落到
// dataProcessor（生产路径 core.go: Subscribe → svc.SetDataArrivalSignal）：
// 若只注入到别处，本用例会退化成 120ms 内的 6-7 次轮询，从而失败
func TestService_SetDataArrivalSignal_WiresDataProcessor(t *testing.T) {
	svc := NewFiveHoleTraversalService(&MockEventPublisher{})
	var seq int64 = 100
	var calls int32
	svc.SetMultiDeviceBatchGetter(makeSeqGetter(&seq, &calls))

	arrival := make(chan struct{}, 1)
	svc.SetDataArrivalSignal(arrival)

	go func() {
		time.Sleep(120 * time.Millisecond)
		atomic.StoreInt64(&seq, 200)
		arrival <- struct{}{}
	}()

	got, err := svc.dataProcessor.WaitForFreshData([]string{"d1"}, map[string]int64{"d1": 100}, 2*time.Second)
	if err != nil {
		t.Fatalf("WaitForFreshData 出错: %v", err)
	}
	if got["d1"] != 200 {
		t.Fatalf("应返回新帧序号 200，实得 %d", got["d1"])
	}
	if n := atomic.LoadInt32(&calls); n > 4 {
		t.Errorf("Service 注入的通道未生效（batchGetter 被调用 %d 次，疑似退化为轮询）", n)
	}
}
