package driver

import (
	"math/rand"
	"sync"
	"time"

	"yx-daq/internal/types"
)

// SimulatedMotionController 模拟运动控制器
type SimulatedMotionController struct {
	mu        sync.Mutex
	axes      []types.AxisConfig
	positions map[types.AxisName]float64
	moving    map[types.AxisName]bool
	// pending 记录在途点动预留的目标位置：软限位按"已预留目标"校验，
	// 避免连续快速点动都基于未更新的位置判断、累积后越限
	pending map[types.AxisName]float64
	// jogSeq 标记每轴最新一次点动：被停止/新指令取代后递增，令在途 goroutine 放弃写入
	jogSeq map[types.AxisName]uint64
}

// NewSimulatedMotionController 创建模拟运动控制器
func NewSimulatedMotionController(axes []types.AxisConfig) *SimulatedMotionController {
	pos := make(map[types.AxisName]float64)
	mov := make(map[types.AxisName]bool)
	for _, ax := range axes {
		pos[ax.Name] = 0
		mov[ax.Name] = false
	}
	return &SimulatedMotionController{
		axes:      axes,
		positions: pos,
		moving:    mov,
		pending:   make(map[types.AxisName]float64),
		jogSeq:    make(map[types.AxisName]uint64),
	}
}

// Connect 模拟连接
func (s *SimulatedMotionController) Connect() error { return nil }

// Disconnect 模拟断开
func (s *SimulatedMotionController) Disconnect() {}

// UpdateAxes applies edited axis parameters to the simulated controller.
func (s *SimulatedMotionController) UpdateAxes(axes []types.AxisConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.axes = axes
	for _, ax := range axes {
		if _, ok := s.positions[ax.Name]; !ok {
			s.positions[ax.Name] = 0
		}
		if _, ok := s.moving[ax.Name]; !ok {
			s.moving[ax.Name] = false
		}
	}
}

// IsConnected 始终连接
func (s *SimulatedMotionController) IsConnected() bool { return true }

// checkSoftLimitLocked 校验目标位置是否超出软限位（调用方必须持有 s.mu）
func (s *SimulatedMotionController) checkSoftLimitLocked(axis types.AxisName, target float64) error {
	for _, ax := range s.axes {
		if ax.Name != axis {
			continue
		}
		return checkSoftLimitTarget(axis, ax.SoftLimit, target)
	}
	return nil
}

// startMoveLocked 启动一次模拟运动（调用方必须持有 s.mu；不做软限位校验）。
// 新指令作废在途点动，避免点动 goroutine 迟到写入覆盖本次定位结果
func (s *SimulatedMotionController) startMoveLocked(axis types.AxisName, position float64) {
	s.jogSeq[axis]++
	delete(s.pending, axis)
	s.moving[axis] = true
	go func() {
		time.Sleep(time.Duration(200+rand.Intn(300)) * time.Millisecond)
		s.mu.Lock()
		defer s.mu.Unlock()
		s.positions[axis] = position
		s.moving[axis] = false
	}()
}

// MoveTo 模拟绝对定位
func (s *SimulatedMotionController) MoveTo(axis types.AxisName, position float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkSoftLimitLocked(axis, position); err != nil {
		return err
	}
	s.startMoveLocked(axis, position)
	return nil
}

// MoveBy 模拟相对移动
func (s *SimulatedMotionController) MoveBy(axis types.AxisName, delta float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkSoftLimitLocked(axis, s.positions[axis]+delta); err != nil {
		return err
	}
	s.jogSeq[axis]++
	delete(s.pending, axis)
	s.positions[axis] += delta
	return nil
}

// Jog 模拟点动（模拟运动过程，与 MoveTo 一致）
func (s *SimulatedMotionController) Jog(axis types.AxisName, direction int, distance float64, speed float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	step := distance
	if direction < 0 {
		step = -distance
	}
	// 以在途点动预留目标为基准校验：模拟运动异步完成，若用未更新的 positions，
	// 连续快速点动（前端双击/回车）会各自通过校验后累积越限
	base := s.positions[axis]
	if pending, ok := s.pending[axis]; ok {
		base = pending
	}
	target := base + step
	if err := s.checkSoftLimitLocked(axis, target); err != nil {
		return err
	}
	s.jogSeq[axis]++
	seq := s.jogSeq[axis]
	s.pending[axis] = target
	s.moving[axis] = true
	go func() {
		time.Sleep(time.Duration(200+rand.Intn(300)) * time.Millisecond)
		s.mu.Lock()
		defer s.mu.Unlock()
		// 已被停止或新指令取代：本次点动作废，不得再写入位置
		if s.jogSeq[axis] != seq {
			return
		}
		s.positions[axis] = target
		delete(s.pending, axis)
		s.moving[axis] = false
	}()
	return nil
}

// Home 模拟回零（与 B140 一致：回零不受软限位限制）
func (s *SimulatedMotionController) Home(axis types.AxisName) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.startMoveLocked(axis, 0)
	return nil
}

// Stop 模拟停止
func (s *SimulatedMotionController) Stop(axis types.AxisName) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jogSeq[axis]++
	delete(s.pending, axis)
	s.moving[axis] = false
	return nil
}

// StopAll 停止所有轴
func (s *SimulatedMotionController) StopAll() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k := range s.moving {
		s.jogSeq[k]++
		delete(s.pending, k)
		s.moving[k] = false
	}
	return nil
}

// EmergencyStop 模拟急停
func (s *SimulatedMotionController) EmergencyStop() error { return s.StopAll() }

// DefinePosition 模拟置位
func (s *SimulatedMotionController) DefinePosition(axis types.AxisName, position float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jogSeq[axis]++
	delete(s.pending, axis)
	s.positions[axis] = position
	return nil
}

// GetAxisStatus 模拟轴状态
func (s *SimulatedMotionController) GetAxisStatus(axis types.AxisName) (types.AxisStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return types.AxisStatus{
		Name:     axis,
		Position: s.positions[axis],
		Moving:   s.moving[axis],
		Homed:    s.positions[axis] == 0,
	}, nil
}

// GetAllAxisStatus 模拟所有轴状态
func (s *SimulatedMotionController) GetAllAxisStatus() ([]types.AxisStatus, error) {
	statuses := []types.AxisStatus{}
	for _, ax := range s.axes {
		if !ax.Enabled {
			continue
		}
		st, _ := s.GetAxisStatus(ax.Name)
		statuses = append(statuses, st)
	}
	return statuses, nil
}

// SetSpeed 模拟设置速度
func (s *SimulatedMotionController) SetSpeed(axis types.AxisName, speed float64) error {
	return nil
}

// SetAcceleration 模拟设置加速度
func (s *SimulatedMotionController) SetAcceleration(axis types.AxisName, accel float64) error {
	return nil
}

// SetDeceleration 模拟设置减速度
func (s *SimulatedMotionController) SetDeceleration(axis types.AxisName, decel float64) error {
	return nil
}

// IsMoving 查询是否有轴在运动
func (s *SimulatedMotionController) IsMoving() (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.moving {
		if m {
			return true, nil
		}
	}
	return false, nil
}

// IsAxisMoving 查询单轴是否在运动
func (s *SimulatedMotionController) IsAxisMoving(axis types.AxisName) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.moving[axis], nil
}

// GetLimitStatus 模拟限位状态
func (s *SimulatedMotionController) GetLimitStatus(axis types.AxisName) (types.LimitStatus, error) {
	return types.LimitStatus{}, nil
}

// WaitForMotionComplete 模拟等待运动完成
func (s *SimulatedMotionController) WaitForMotionComplete(axis types.AxisName, timeoutMs int) error {
	for i := 0; i < timeoutMs/50; i++ {
		s.mu.Lock()
		m := s.moving[axis]
		s.mu.Unlock()
		if !m {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

// MotorOff 模拟关闭电机
func (s *SimulatedMotionController) MotorOff() error {
	return nil
}

// SetAxisDirection 模拟设置轴方向
func (s *SimulatedMotionController) SetAxisDirection(axis types.AxisName, reverse bool) error {
	return nil
}
