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
	}
}

// Connect 模拟连接
func (s *SimulatedMotionController) Connect() error { return nil }

// Disconnect 模拟断开
func (s *SimulatedMotionController) Disconnect() {}

// IsConnected 始终连接
func (s *SimulatedMotionController) IsConnected() bool { return true }

// MoveTo 模拟绝对定位
func (s *SimulatedMotionController) MoveTo(axis types.AxisName, position float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.moving[axis] = true
	go func() {
		time.Sleep(time.Duration(200+rand.Intn(300)) * time.Millisecond)
		s.mu.Lock()
		defer s.mu.Unlock()
		s.positions[axis] = position
		s.moving[axis] = false
	}()
	return nil
}

// MoveBy 模拟相对移动
func (s *SimulatedMotionController) MoveBy(axis types.AxisName, delta float64) error {
	s.mu.Lock()
	s.positions[axis] += delta
	s.mu.Unlock()
	return nil
}

// Jog 模拟点动
func (s *SimulatedMotionController) Jog(axis types.AxisName, direction int, speed float64) error {
	delta := 1.0
	if direction < 0 {
		delta = -1.0
	}
	return s.MoveBy(axis, delta)
}

// Home 模拟回零
func (s *SimulatedMotionController) Home(axis types.AxisName) error {
	return s.MoveTo(axis, 0)
}

// Stop 模拟停止
func (s *SimulatedMotionController) Stop(axis types.AxisName) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.moving[axis] = false
	return nil
}

// StopAll 停止所有轴
func (s *SimulatedMotionController) StopAll() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k := range s.moving {
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
