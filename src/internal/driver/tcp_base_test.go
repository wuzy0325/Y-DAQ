package driver

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"yx-daq/internal/logger"
	"yx-daq/internal/types"
)

func TestTCPDriverBase_SetDataCallback(t *testing.T) {
	base := NewTCPDriverBase("127.0.0.1", 9000, nil)

	if base.getOnData() != nil {
		t.Fatal("initial onData should be nil")
	}

	called := false
	cb := func(payload types.DataPayload) {
		called = true
	}
	base.SetDataCallback(cb)

	if base.getOnData() == nil {
		t.Fatal("onData should not be nil after SetDataCallback")
	}

	// 验证回调可被调用（通过 EmitData 走加锁路径）
	base.EmitData(types.DataPayload{DeviceID: "test"})
	if !called {
		t.Fatal("callback was not called")
	}
}

func TestTCPDriverBase_UpdateAndGetChannels(t *testing.T) {
	channels := []types.ChannelConfig{
		{Index: 0, Name: "CH1", Enabled: true, Unit: "kPa"},
		{Index: 1, Name: "CH2", Enabled: false, Unit: "kPa"},
	}
	base := NewTCPDriverBase("127.0.0.1", 9000, channels)

	// GetChannels 应返回副本
	got := base.GetChannels()
	if len(got) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(got))
	}

	// 修改返回的副本不应影响原始数据
	got[0].Name = "MODIFIED"
	original := base.GetChannels()
	if original[0].Name == "MODIFIED" {
		t.Fatal("GetChannels should return a copy, not a reference")
	}

	// UpdateChannels 应更新通道
	newChannels := []types.ChannelConfig{
		{Index: 0, Name: "NEW1", Enabled: true, Unit: "Pa"},
		{Index: 1, Name: "NEW2", Enabled: true, Unit: "Pa"},
		{Index: 2, Name: "NEW3", Enabled: false, Unit: "Pa"},
	}
	base.UpdateChannels(newChannels)

	updated := base.GetChannels()
	if len(updated) != 3 {
		t.Fatalf("expected 3 channels after update, got %d", len(updated))
	}
	if updated[0].Name != "NEW1" {
		t.Fatalf("expected channel name NEW1, got %s", updated[0].Name)
	}
}

// TestTCPDriverBase_BuildDataPayload_MapsByChannelIndex 验证按 ch.Index 取值，
// 通道配置切片顺序变化不会导致通道串位
func TestTCPDriverBase_BuildDataPayload_MapsByChannelIndex(t *testing.T) {
	channels := []types.ChannelConfig{
		{Index: 2, Name: "CH3", Enabled: true, Unit: "kPa"},
		{Index: 0, Name: "CH1", Enabled: true, Unit: "kPa"},
		{Index: 1, Name: "CH2", Enabled: false, Unit: "kPa"},
	}
	base := NewTCPDriverBase("127.0.0.1", 9000, channels)

	payload := base.BuildDataPayload([]float64{10, 20, 30}, "dev")
	if len(payload.Channels) != 2 || len(payload.ChannelIndices) != 2 {
		t.Fatalf("got %d channels, want 2", len(payload.Channels))
	}
	if payload.ChannelIndices[0] != 2 || payload.Channels[0] != 30 {
		t.Errorf("first enabled = idx %d value %v, want idx 2 value 30", payload.ChannelIndices[0], payload.Channels[0])
	}
	if payload.ChannelIndices[1] != 0 || payload.Channels[1] != 10 {
		t.Errorf("second enabled = idx %d value %v, want idx 0 value 10", payload.ChannelIndices[1], payload.Channels[1])
	}
}

func TestTCPDriverBase_ConnectedState(t *testing.T) {
	base := NewTCPDriverBase("127.0.0.1", 9000, nil)

	// 初始状态：未连接、未采集
	if base.IsConnected() {
		t.Fatal("should not be connected initially")
	}
	if base.IsAcquiring() {
		t.Fatal("should not be acquiring initially")
	}

	// connected 和 acquiring 应为 false
	if base.connected.Load() {
		t.Fatal("connected flag should be false initially")
	}
	if base.acquiring.Load() {
		t.Fatal("acquiring flag should be false initially")
	}
}

// TestTCPDriverBase_Concurrent_SetDataCallback_And_EmitData 验证并发设置回调与发射数据无数据竞争
// N 个 goroutine 并发 SetDataCallback（含 nil）+ M 个 goroutine 并发 EmitData，运行 500ms
// 覆盖 R2 回归：receiveLoop goroutine 调用 EmitData 读 b.onData 必须与 SetDataCallback 写互斥
func TestTCPDriverBase_Concurrent_SetDataCallback_And_EmitData(t *testing.T) {
	base := NewTCPDriverBase("127.0.0.1", 9000, nil)

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// 4 个 goroutine 并发切换回调（含 nil 回调，模拟重连/配置变更场景）
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if idx%2 == 0 {
					base.SetDataCallback(func(payload types.DataPayload) {
						// 回调内部仅消费 payload，不访问 base 字段，避免与 getOnData 锁顺序冲突
						_ = payload.DeviceID
					})
				} else {
					base.SetDataCallback(nil)
				}
			}
		}(i)
	}

	// 8 个 goroutine 并发 EmitData，模拟 receiveLoop 高频调用
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				base.EmitData(types.DataPayload{
					DeviceID: "dev",
					Channels: []float64{float64(idx)},
				})
			}
		}(i)
	}

	time.Sleep(500 * time.Millisecond)
	close(stop)
	wg.Wait()
}

// startTestTCPServer 启动假设备 TCP 服务器，顺序接受连接并通过 conns 通道上报
func startTestTCPServer(t *testing.T) (port int, conns chan net.Conn) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	conns = make(chan net.Conn, 4)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			conns <- c
		}
	}()
	t.Cleanup(func() { ln.Close() })
	return ln.Addr().(*net.TCPAddr).Port, conns
}

// waitForFlag 轮询等待条件满足或超时
func waitForFlag(t *testing.T, timeout time.Duration, check func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(msg)
}

// 回归：重连后旧 receiveLoop 协程的 defer 不得把 recvLoopRunning 覆盖回 false。
// 旧实现中 HandleDisconnect 在旧 receiveLoop 协程内执行，新接收循环启动后旧协程
// 退出时 defer 会把标志复位，导致 sendUnitCommand 误判接收循环未运行而直接读
// conn，与新接收循环并发抢数据（多设备并行采集时偶发无数据/命令超时的根因）。
func TestTCPDriverBase_Reconnect_RecvLoopRunningNotClobbered(t *testing.T) {
	port, conns := startTestTCPServer(t)

	base := NewTCPDriverBase("127.0.0.1", port, nil)
	base.SetOnReconnect(func() error {
		base.StartReceiveLoop(func(data []byte) {})
		return nil
	})
	if err := base.DialConnect(); err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	base.StartReceiveLoop(func(data []byte) {})

	// 模拟网络断开：服务端关闭第一条连接
	select {
	case c1 := <-conns:
		c1.Close()
	case <-time.After(2 * time.Second):
		t.Fatal("first connection not accepted")
	}

	// 等待重连拨号成功（退避基础延迟 1s）
	select {
	case <-conns:
	case <-time.After(5 * time.Second):
		t.Fatal("reconnect not attempted")
	}

	// 重连 hook 启动新接收循环后，标志必须保持 true
	waitForFlag(t, 2*time.Second, base.recvLoopRunning.Load, "receive loop should be running after reconnect")
	// 留出旧协程 defer 执行窗口：若旧 defer 覆盖标志，此处会翻转为 false
	time.Sleep(300 * time.Millisecond)
	if !base.recvLoopRunning.Load() {
		t.Fatal("recvLoopRunning must stay true after reconnect (old loop defer must not clobber it)")
	}
	if !base.connected.Load() {
		t.Fatal("should be connected after reconnect")
	}
	base.CloseDisconnect()
}

// 验证发送失败时写入通信日志（含设备 ID 与命令），供排查设备通信故障
func TestTCPDriverBase_CommErrorLoggedOnSendFailure(t *testing.T) {
	dir := t.TempDir()
	cfg := types.DefaultLoggingConfig()
	cfg.Console = false
	if err := logger.InitAt(dir, cfg); err != nil {
		t.Fatalf("logger init failed: %v", err)
	}
	defer logger.Close()

	base := NewTCPDriverBase("127.0.0.1", 1, nil)
	base.SetDeviceID("dev-comm-test")

	// 构造写失败的连接：net.Pipe 对端关闭后写入返回错误
	client, server := net.Pipe()
	server.Close()
	base.mu.Lock()
	base.Conn = client
	base.mu.Unlock()
	base.connected.Store(true)

	if err := base.WriteCommandOnly("w1601\r"); err == nil {
		t.Fatal("expected write error")
	}
	client.Close()

	files, err := filepath.Glob(filepath.Join(dir, "comm-*.log"))
	if err != nil || len(files) == 0 {
		t.Fatalf("comm log file not created: %v", err)
	}
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("read comm log: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "TCP write command failed") {
		t.Errorf("comm log missing send failure entry: %s", content)
	}
	if !strings.Contains(content, "dev-comm-test") {
		t.Errorf("comm log missing device id: %s", content)
	}
	if !strings.Contains(content, "w1601") {
		t.Errorf("comm log missing command: %s", content)
	}
}

// 回归：用户主动断开后，重连循环必须彻底停止，不得自行重连产生幽灵连接。
func TestTCPDriverBase_Reconnect_UserDisconnectStopsReconnect(t *testing.T) {
	port, conns := startTestTCPServer(t)

	base := NewTCPDriverBase("127.0.0.1", port, nil)
	base.SetOnReconnect(func() error {
		base.StartReceiveLoop(func(data []byte) {})
		return nil
	})
	if err := base.DialConnect(); err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	base.StartReceiveLoop(func(data []byte) {})

	// 触发断连重连流程后，用户立即主动断开
	select {
	case c1 := <-conns:
		c1.Close()
	case <-time.After(2 * time.Second):
		t.Fatal("first connection not accepted")
	}
	base.CloseDisconnect()

	// 退避窗口（基础延迟 1s）过后不得出现第二次拨号
	select {
	case <-conns:
		t.Fatal("reconnect attempted after user disconnect")
	case <-time.After(1500 * time.Millisecond):
	}
}

// 回归：断连前正在采集时，重连成功后必须调用采集恢复 hook，且断连时采集标志复位。
func TestTCPDriverBase_Reconnect_ResumesAcquisition(t *testing.T) {
	port, conns := startTestTCPServer(t)

	base := NewTCPDriverBase("127.0.0.1", port, nil)
	resumed := make(chan struct{}, 1)
	base.SetOnReconnect(func() error {
		base.StartReceiveLoop(func(data []byte) {})
		return nil
	})
	base.SetOnResumeAcquire(func() error {
		select {
		case resumed <- struct{}{}:
		default:
		}
		return nil
	})
	if err := base.DialConnect(); err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	base.StartReceiveLoop(func(data []byte) {})

	// 模拟采集中断连
	base.acquiring.Store(true)
	select {
	case c1 := <-conns:
		c1.Close()
	case <-time.After(2 * time.Second):
		t.Fatal("first connection not accepted")
	}

	// 断连时采集标志应被复位
	waitForFlag(t, 3*time.Second, func() bool { return !base.acquiring.Load() }, "acquiring should be reset on disconnect")

	// 重连成功后恢复 hook 应被调用
	select {
	case <-resumed:
	case <-time.After(5 * time.Second):
		t.Fatal("resume acquisition hook not called after reconnect")
	}
	base.CloseDisconnect()
}
