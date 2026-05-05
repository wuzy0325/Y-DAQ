package app

import (
	"fmt"

	"yx-daq/internal/types"
)

// MotionService 运动控制服务
type MotionService struct {
	Core *Core
}

// GetMotionProfiles 获取所有运动控制器配置
func (s *MotionService) GetMotionProfiles() []types.MotionControllerProfile {
	if s.Core.MotionManager == nil {
		return nil
	}
	return s.Core.MotionManager.GetProfiles()
}

// AddMotionProfile 添加运动控制器配置
func (s *MotionService) AddMotionProfile(profile types.MotionControllerProfile) {
	if s.Core.MotionManager == nil {
		return
	}
	s.Core.MotionManager.AddProfile(profile)
	s.Core.saveMotionConfig()
}

// ConnectMotion 连接运动控制器
func (s *MotionService) ConnectMotion(id string) error {
	if s.Core.MotionManager == nil {
		return fmt.Errorf("motion manager not initialized")
	}
	return s.Core.MotionManager.Connect(id)
}

// DisconnectMotion 断开运动控制器
func (s *MotionService) DisconnectMotion(id string) {
	if s.Core.MotionManager == nil {
		return
	}
	s.Core.MotionManager.Disconnect(id)
}

// MotionMoveTo 绝对定位
func (s *MotionService) MotionMoveTo(id string, axis types.AxisName, position float64) error {
	if s.Core.MotionManager == nil {
		return fmt.Errorf("motion manager not initialized")
	}
	return s.Core.MotionManager.MoveTo(id, axis, position)
}

// MotionMoveBy 相对移动
func (s *MotionService) MotionMoveBy(id string, axis types.AxisName, delta float64) error {
	if s.Core.MotionManager == nil {
		return fmt.Errorf("motion manager not initialized")
	}
	return s.Core.MotionManager.MoveBy(id, axis, delta)
}

// MotionJog 点动
func (s *MotionService) MotionJog(id string, axis types.AxisName, direction int, speed float64) error {
	if s.Core.MotionManager == nil {
		return fmt.Errorf("motion manager not initialized")
	}
	return s.Core.MotionManager.Jog(id, axis, direction, speed)
}

// MotionHome 回零
func (s *MotionService) MotionHome(id string, axis types.AxisName) error {
	if s.Core.MotionManager == nil {
		return fmt.Errorf("motion manager not initialized")
	}
	return s.Core.MotionManager.Home(id, axis)
}

// MotionStop 停止
func (s *MotionService) MotionStop(id string, axis types.AxisName) error {
	if s.Core.MotionManager == nil {
		return fmt.Errorf("motion manager not initialized")
	}
	return s.Core.MotionManager.Stop(id, axis)
}

// MotionStopAll 停止所有轴
func (s *MotionService) MotionStopAll(id string) error {
	if s.Core.MotionManager == nil {
		return fmt.Errorf("motion manager not initialized")
	}
	return s.Core.MotionManager.StopAll(id)
}

// MotionEmergencyStop 急停
func (s *MotionService) MotionEmergencyStop(id string) error {
	if s.Core.MotionManager == nil {
		return fmt.Errorf("motion manager not initialized")
	}
	return s.Core.MotionManager.EmergencyStop(id)
}

// MotionDefinePosition 置位
func (s *MotionService) MotionDefinePosition(id string, axis types.AxisName, position float64) error {
	if s.Core.MotionManager == nil {
		return fmt.Errorf("motion manager not initialized")
	}
	return s.Core.MotionManager.DefinePosition(id, axis, position)
}

// GetMotionStatusAll 获取所有运动控制器状态
func (s *MotionService) GetMotionStatusAll() []types.MotionControllerStatus {
	if s.Core.MotionManager == nil {
		return nil
	}
	return s.Core.MotionManager.GetStatusAll()
}

// MotionSetAcceleration 设置加速度
func (s *MotionService) MotionSetAcceleration(id string, axis types.AxisName, accel float64) error {
	if s.Core.MotionManager == nil {
		return fmt.Errorf("motion manager not initialized")
	}
	return s.Core.MotionManager.SetAcceleration(id, axis, accel)
}

// MotionSetDeceleration 设置减速度
func (s *MotionService) MotionSetDeceleration(id string, axis types.AxisName, decel float64) error {
	if s.Core.MotionManager == nil {
		return fmt.Errorf("motion manager not initialized")
	}
	return s.Core.MotionManager.SetDeceleration(id, axis, decel)
}

// MotionIsMoving 查询是否有轴在运动
func (s *MotionService) MotionIsMoving(id string) (bool, error) {
	if s.Core.MotionManager == nil {
		return false, fmt.Errorf("motion manager not initialized")
	}
	return s.Core.MotionManager.IsMoving(id)
}

// MotionIsAxisMoving 查询单轴是否在运动
func (s *MotionService) MotionIsAxisMoving(id string, axis types.AxisName) (bool, error) {
	if s.Core.MotionManager == nil {
		return false, fmt.Errorf("motion manager not initialized")
	}
	return s.Core.MotionManager.IsAxisMoving(id, axis)
}

// MotionGetLimitStatus 查询轴限位状态
func (s *MotionService) MotionGetLimitStatus(id string, axis types.AxisName) (types.LimitStatus, error) {
	if s.Core.MotionManager == nil {
		return types.LimitStatus{}, fmt.Errorf("motion manager not initialized")
	}
	return s.Core.MotionManager.GetLimitStatus(id, axis)
}

// MotionWaitForComplete 等待运动完成
func (s *MotionService) MotionWaitForComplete(id string, axis types.AxisName, timeoutMs int) error {
	if s.Core.MotionManager == nil {
		return fmt.Errorf("motion manager not initialized")
	}
	return s.Core.MotionManager.WaitForMotionComplete(id, axis, timeoutMs)
}

// MotionMotorOff 关闭电机
func (s *MotionService) MotionMotorOff(id string) error {
	if s.Core.MotionManager == nil {
		return fmt.Errorf("motion manager not initialized")
	}
	return s.Core.MotionManager.MotorOff(id)
}

// MotionSetAxisDirection 设置轴方向
func (s *MotionService) MotionSetAxisDirection(id string, axis types.AxisName, reverse bool) error {
	if s.Core.MotionManager == nil {
		return fmt.Errorf("motion manager not initialized")
	}
	return s.Core.MotionManager.SetAxisDirection(id, axis, reverse)
}

// UpdateMotionProfile 更新运动控制器配置
func (s *MotionService) UpdateMotionProfile(profile types.MotionControllerProfile) {
	if s.Core.MotionManager == nil {
		return
	}
	s.Core.MotionManager.UpdateProfile(profile)
	s.Core.saveMotionConfig()
}

// RemoveMotionProfile 删除运动控制器配置
func (s *MotionService) RemoveMotionProfile(id string) {
	if s.Core.MotionManager == nil {
		return
	}
	s.Core.MotionManager.RemoveProfile(id)
	s.Core.saveMotionConfig()
}
