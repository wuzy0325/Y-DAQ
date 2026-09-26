package driver

import (
	"net"
	"strings"
	"sync"
	"testing"

	"yx-daq/internal/types"
)

// linearAxisWithSoftLimit 构造 linear 轴配置：640 pulses/unit
func linearAxisWithSoftLimit(enabled bool, min, max float64) types.AxisConfig {
	return types.AxisConfig{
		Name: types.AxisX, Enabled: true, Kind: types.AxisKindLinear,
		StepAngleDeg: 1.8, MicroSteps: 16, Lead: 5,
		SoftLimit: types.SoftLimitConfig{Enabled: enabled, Min: min, Max: max},
	}
}

func TestB140CheckSoftLimit_Disabled(t *testing.T) {
	c := newB140WithAxes([]types.AxisConfig{linearAxisWithSoftLimit(false, -10, 10)})
	if err := c.checkSoftLimit(types.AxisX, 1000); err != nil {
		t.Fatalf("disabled soft limit should pass, got %v", err)
	}
}

func TestB140CheckSoftLimit_Targets(t *testing.T) {
	c := newB140WithAxes([]types.AxisConfig{linearAxisWithSoftLimit(true, -10, 10)})
	cases := []struct {
		name    string
		target  float64
		wantErr bool
	}{
		{"lower boundary", -10, false},
		{"upper boundary", 10, false},
		{"near upper boundary", 9.999, false},
		{"below lower", -10.001, true},
		{"above upper", 10.001, true},
		{"far below", -100, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := c.checkSoftLimit(types.AxisX, tc.target)
			if tc.wantErr && err == nil {
				t.Fatalf("target %v should be rejected", tc.target)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("target %v should be allowed, got %v", tc.target, err)
			}
		})
	}
}

func TestB140CheckSoftLimit_InvalidRange(t *testing.T) {
	c := newB140WithAxes([]types.AxisConfig{linearAxisWithSoftLimit(true, 10, 10)})
	if err := c.checkSoftLimit(types.AxisX, 0); err == nil {
		t.Fatal("max <= min should be rejected")
	}
}

func TestB140CheckSoftLimit_UnknownAxis(t *testing.T) {
	c := newB140WithAxes(nil)
	if err := c.checkSoftLimit(types.AxisX, 1); err != nil {
		t.Fatalf("unknown axis should pass, got %v", err)
	}
}

func TestB140SoftLimitCommandValues(t *testing.T) {
	c := newB140WithAxes([]types.AxisConfig{linearAxisWithSoftLimit(true, -10, 10)})
	forward, backward, err := c.softLimitCommandValues(c.axes[0])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 10 * 640 = 6400；-10 * 640 = -6400
	if forward != 6400 || backward != -6400 {
		t.Fatalf("got forward=%d backward=%d, want 6400/-6400", forward, backward)
	}

	disabled := c.axes[0]
	disabled.SoftLimit.Enabled = false
	forward, backward, err = c.softLimitCommandValues(disabled)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if forward != b140PositionMax || backward != b140PositionMin {
		t.Fatalf("disabled soft limit should be wide, got %d/%d", forward, backward)
	}
}

func TestB140SoftLimitCommandValues_InvalidRange(t *testing.T) {
	c := newB140WithAxes([]types.AxisConfig{linearAxisWithSoftLimit(true, 5, 5)})
	if _, _, err := c.softLimitCommandValues(c.axes[0]); err == nil {
		t.Fatal("max <= min should be rejected")
	}
}

func TestB140SoftLimitCommandValues_InvertedAxisSignPreserved(t *testing.T) {
	ax := linearAxisWithSoftLimit(true, -10, 10)
	ax.Inverted = true
	c := newB140WithAxes([]types.AxisConfig{ax})
	forward, backward, err := c.softLimitCommandValues(ax)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 轴方向由控制器 MT/CE 取反，位置坐标不变：FL 始终对应工程正向限位（Max）
	if forward != 6400 || backward != -6400 {
		t.Fatalf("got forward=%d backward=%d, want 6400/-6400", forward, backward)
	}
}

func TestClampB140Position(t *testing.T) {
	cases := []struct {
		name string
		in   int
		want int
	}{
		{"in range", 100, 100},
		{"upper overflow", b140PositionMax + 1, b140PositionMax},
		{"lower overflow", b140PositionMin - 1, b140PositionMin},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := clampB140Position(tc.in); got != tc.want {
				t.Fatalf("clampB140Position(%d) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

// newPipeB140 构造基于 net.Pipe 的 B140 控制器；nextCmd 读取一条命令并回 ":" 应答
func newPipeB140(t *testing.T, axes []types.AxisConfig) (*B140MotionController, func() string) {
	t.Helper()
	client, server := net.Pipe()
	t.Cleanup(func() {
		client.Close()
		server.Close()
	})

	d := NewB140Driver("", 0, 500)
	d.conn = client
	d.connected.Store(true)
	c := NewB140MotionController(d, axes)

	nextCmd := func() string {
		var b strings.Builder
		one := make([]byte, 1)
		for {
			if _, err := server.Read(one); err != nil {
				t.Fatalf("read command failed: %v", err)
			}
			b.WriteByte(one[0])
			if one[0] == '\r' {
				break
			}
		}
		_, _ = server.Write([]byte(":"))
		return b.String()
	}
	return c, nextCmd
}

func TestB140ApplySoftLimits_SendsFLBL(t *testing.T) {
	c, nextCmd := newPipeB140(t, []types.AxisConfig{linearAxisWithSoftLimit(true, -10, 10)})

	done := make(chan error, 1)
	go func() { done <- c.applySoftLimits() }()

	if cmd := nextCmd(); cmd != "FLA=6400\r" {
		t.Fatalf("forward limit command = %q, want FLA=6400\\r", cmd)
	}
	if cmd := nextCmd(); cmd != "BLA=-6400\r" {
		t.Fatalf("backward limit command = %q, want BLA=-6400\\r", cmd)
	}
	if err := <-done; err != nil {
		t.Fatalf("applySoftLimits failed: %v", err)
	}
}

func TestB140ApplySoftLimits_DisabledSendsWide(t *testing.T) {
	c, nextCmd := newPipeB140(t, []types.AxisConfig{linearAxisWithSoftLimit(false, -10, 10)})

	done := make(chan error, 1)
	go func() { done <- c.applySoftLimits() }()

	if cmd := nextCmd(); cmd != "FLA=2147483647\r" {
		t.Fatalf("forward limit command = %q, want FLA=2147483647\\r", cmd)
	}
	if cmd := nextCmd(); cmd != "BLA=-2147483647\r" {
		t.Fatalf("backward limit command = %q, want BLA=-2147483647\\r", cmd)
	}
	if err := <-done; err != nil {
		t.Fatalf("applySoftLimits failed: %v", err)
	}
}

func TestB140ApplySoftLimits_SkipsUnchanged(t *testing.T) {
	c, nextCmd := newPipeB140(t, []types.AxisConfig{linearAxisWithSoftLimit(true, -10, 10)})

	first := make(chan error, 1)
	go func() { first <- c.applySoftLimits() }()
	nextCmd()
	nextCmd()
	if err := <-first; err != nil {
		t.Fatalf("first applySoftLimits failed: %v", err)
	}

	// 第二次调用应命中签名缓存：若重发命令，服务端不再应答，SendCommand 将超时返回错误
	if err := c.applySoftLimits(); err != nil {
		t.Fatalf("second applySoftLimits should be a no-op, got %v", err)
	}
}

func TestB140UpdateAxes_ReappliesSoftLimitsWhenConnected(t *testing.T) {
	c, nextCmd := newPipeB140(t, []types.AxisConfig{linearAxisWithSoftLimit(true, -10, 10)})

	first := make(chan error, 1)
	go func() { first <- c.applySoftLimits() }()
	nextCmd()
	nextCmd()
	if err := <-first; err != nil {
		t.Fatalf("initial applySoftLimits failed: %v", err)
	}

	// 已连接控制器保存新配置后应立即下发新软限位（20 * 640 = 12800）
	done := make(chan struct{})
	go func() {
		c.UpdateAxes([]types.AxisConfig{linearAxisWithSoftLimit(true, -20, 20)})
		close(done)
	}()
	if cmd := nextCmd(); cmd != "FLA=12800\r" {
		t.Fatalf("forward limit command = %q, want FLA=12800\\r", cmd)
	}
	if cmd := nextCmd(); cmd != "BLA=-12800\r" {
		t.Fatalf("backward limit command = %q, want BLA=-12800\\r", cmd)
	}
	<-done
}

// fakeB140 基于 net.Pipe 的脚本化 B140 服务端：记录收到的命令并按 handler 应答
type fakeB140 struct {
	mu   sync.Mutex
	cmds []string
}

func (f *fakeB140) add(cmd string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cmds = append(f.cmds, cmd)
}

func (f *fakeB140) commands() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.cmds...)
}

func (f *fakeB140) hasCommand(prefix string) bool {
	for _, cmd := range f.commands() {
		if strings.HasPrefix(cmd, prefix) {
			return true
		}
	}
	return false
}

// newScriptedB140 创建脚本化 B140 控制器；handler 收到去掉 \r 的命令后返回应答
func newScriptedB140(t *testing.T, axes []types.AxisConfig, handler func(cmd string) string) (*B140MotionController, *fakeB140) {
	t.Helper()
	client, server := net.Pipe()
	t.Cleanup(func() {
		client.Close()
		server.Close()
	})

	d := NewB140Driver("", 0, 500)
	d.conn = client
	d.connected.Store(true)

	fake := &fakeB140{}
	go func() {
		one := make([]byte, 1)
		var b strings.Builder
		for {
			if _, err := server.Read(one); err != nil {
				return
			}
			b.WriteByte(one[0])
			if one[0] != '\r' {
				continue
			}
			cmd := strings.TrimSuffix(b.String(), "\r")
			b.Reset()
			fake.add(cmd)
			if _, err := server.Write([]byte(handler(cmd))); err != nil {
				return
			}
		}
	}()
	return NewB140MotionController(d, axes), fake
}

// tdAt10Units 构造 TD 应答：X 轴当前 6400 脉冲（10 工程单位）
func tdAt10Units(cmd string) string {
	if cmd == "TD" {
		return "6400,0,0,0:"
	}
	return ":"
}

func TestB140MoveBy_RejectsBeyondSoftLimit(t *testing.T) {
	ax := linearAxisWithSoftLimit(true, -10, 10)
	c, fake := newScriptedB140(t, []types.AxisConfig{ax}, tdAt10Units)

	// 当前 10（6400 脉冲），+1 超出上限 → 拒绝且不下发 PR
	err := c.MoveBy(types.AxisX, 1)
	if err == nil || !strings.Contains(err.Error(), "软限位") {
		t.Fatalf("expected soft limit error, got %v", err)
	}
	if fake.hasCommand("PR") {
		t.Fatalf("PR should not be sent when target exceeds soft limit, got %v", fake.commands())
	}

	// -1 → 9，允许
	if err := c.MoveBy(types.AxisX, -1); err != nil {
		t.Fatalf("MoveBy within range failed: %v", err)
	}
	if !fake.hasCommand("PRA=-640") {
		t.Fatalf("expected PRA=-640 command, got %v", fake.commands())
	}
}

func TestB140Jog_RejectsBeyondSoftLimit(t *testing.T) {
	ax := linearAxisWithSoftLimit(true, -10, 10)
	c, fake := newScriptedB140(t, []types.AxisConfig{ax}, tdAt10Units)

	if err := c.Jog(types.AxisX, 1, 1, 0); err == nil || !strings.Contains(err.Error(), "软限位") {
		t.Fatalf("expected soft limit error, got %v", err)
	}
	if fake.hasCommand("PR") {
		t.Fatalf("PR should not be sent when jog target exceeds soft limit, got %v", fake.commands())
	}

	if err := c.Jog(types.AxisX, -1, 1, 0); err != nil {
		t.Fatalf("Jog within range failed: %v", err)
	}
	if !fake.hasCommand("PRA=-640") {
		t.Fatalf("expected PRA=-640 command, got %v", fake.commands())
	}
}

func TestB140ApplySoftLimits_DisabledAxisStillClears(t *testing.T) {
	ax := linearAxisWithSoftLimit(true, -10, 10)
	ax.Enabled = false
	c, nextCmd := newPipeB140(t, []types.AxisConfig{ax})

	done := make(chan error, 1)
	go func() { done <- c.applySoftLimits() }()

	if cmd := nextCmd(); cmd != "FLA=2147483647\r" {
		t.Fatalf("forward limit command = %q, want FLA=2147483647\\r", cmd)
	}
	if cmd := nextCmd(); cmd != "BLA=-2147483647\r" {
		t.Fatalf("backward limit command = %q, want BLA=-2147483647\\r", cmd)
	}
	if err := <-done; err != nil {
		t.Fatalf("applySoftLimits failed: %v", err)
	}
}
