package app

import (
	"fmt"

	"yx-daq/internal/types"
)

// ==================== 运动控制 API ====================

// GetMotionProfiles 获取所有运动控制器配置
func (a *App) GetMotionProfiles() []types.MotionControllerProfile {
	if a.motionManager == nil {
		return nil
	}
	return a.motionManager.GetProfiles()
}

// AddMotionProfile 添加运动控制器配置
func (a *App) AddMotionProfile(profile types.MotionControllerProfile) {
	if a.motionManager == nil {
		return
	}
	a.motionManager.AddProfile(profile)
	a.saveMotionConfig()
}

// ConnectMotion 连接运动控制器
func (a *App) ConnectMotion(id string) error {
	if a.motionManager == nil {
		return fmt.Errorf("motion manager not initialized")
	}
	return a.motionManager.Connect(id)
}

// DisconnectMotion 断开运动控制器
func (a *App) DisconnectMotion(id string) {
	if a.motionManager == nil {
		return
	}
	a.motionManager.Disconnect(id)
}

// MotionMoveTo 绝对定位
func (a *App) MotionMoveTo(id string, axis types.AxisName, position float64) error {
	if a.motionManager == nil {
		return fmt.Errorf("motion manager not initialized")
	}
	return a.motionManager.MoveTo(id, axis, position)
}

// MotionMoveBy 相对移动
func (a *App) MotionMoveBy(id string, axis types.AxisName, delta float64) error {
	if a.motionManager == nil {
		return fmt.Errorf("motion manager not initialized")
	}
	return a.motionManager.MoveBy(id, axis, delta)
}

// MotionJog 点动
func (a *App) MotionJog(id string, axis types.AxisName, direction int, speed float64) error {
	if a.motionManager == nil {
		return fmt.Errorf("motion manager not initialized")
	}
	return a.motionManager.Jog(id, axis, direction, speed)
}

// MotionHome 回零
func (a *App) MotionHome(id string, axis types.AxisName) error {
	if a.motionManager == nil {
		return fmt.Errorf("motion manager not initialized")
	}
	return a.motionManager.Home(id, axis)
}

// MotionStop 停止
func (a *App) MotionStop(id string, axis types.AxisName) error {
	if a.motionManager == nil {
		return fmt.Errorf("motion manager not initialized")
	}
	return a.motionManager.Stop(id, axis)
}

// MotionEmergencyStop 急停
func (a *App) MotionEmergencyStop(id string) error {
	if a.motionManager == nil {
		return fmt.Errorf("motion manager not initialized")
	}
	return a.motionManager.EmergencyStop(id)
}

// MotionDefinePosition 置位
func (a *App) MotionDefinePosition(id string, axis types.AxisName, position float64) error {
	if a.motionManager == nil {
		return fmt.Errorf("motion manager not initialized")
	}
	return a.motionManager.DefinePosition(id, axis, position)
}

// GetMotionStatusAll 获取所有运动控制器状态
func (a *App) GetMotionStatusAll() []types.MotionControllerStatus {
	if a.motionManager == nil {
		return nil
	}
	return a.motionManager.GetStatusAll()
}

// MotionSetAcceleration 设置加速度
func (a *App) MotionSetAcceleration(id string, axis types.AxisName, accel float64) error {
	if a.motionManager == nil {
		return fmt.Errorf("motion manager not initialized")
	}
	return a.motionManager.SetAcceleration(id, axis, accel)
}

// MotionSetDeceleration 设置减速度
func (a *App) MotionSetDeceleration(id string, axis types.AxisName, decel float64) error {
	if a.motionManager == nil {
		return fmt.Errorf("motion manager not initialized")
	}
	return a.motionManager.SetDeceleration(id, axis, decel)
}

// MotionIsMoving 查询是否有轴在运动
func (a *App) MotionIsMoving(id string) (bool, error) {
	if a.motionManager == nil {
		return false, fmt.Errorf("motion manager not initialized")
	}
	return a.motionManager.IsMoving(id)
}

// MotionIsAxisMoving 查询单轴是否在运动
func (a *App) MotionIsAxisMoving(id string, axis types.AxisName) (bool, error) {
	if a.motionManager == nil {
		return false, fmt.Errorf("motion manager not initialized")
	}
	return a.motionManager.IsAxisMoving(id, axis)
}

// MotionGetLimitStatus 查询轴限位状态
func (a *App) MotionGetLimitStatus(id string, axis types.AxisName) (types.LimitStatus, error) {
	if a.motionManager == nil {
		return types.LimitStatus{}, fmt.Errorf("motion manager not initialized")
	}
	return a.motionManager.GetLimitStatus(id, axis)
}

// MotionWaitForComplete 等待运动完成
func (a *App) MotionWaitForComplete(id string, axis types.AxisName, timeoutMs int) error {
	if a.motionManager == nil {
		return fmt.Errorf("motion manager not initialized")
	}
	return a.motionManager.WaitForMotionComplete(id, axis, timeoutMs)
}

// MotionMotorOff 关闭电机
func (a *App) MotionMotorOff(id string) error {
	if a.motionManager == nil {
		return fmt.Errorf("motion manager not initialized")
	}
	return a.motionManager.MotorOff(id)
}

// MotionSetAxisDirection 设置轴方向
func (a *App) MotionSetAxisDirection(id string, axis types.AxisName, reverse bool) error {
	if a.motionManager == nil {
		return fmt.Errorf("motion manager not initialized")
	}
	return a.motionManager.SetAxisDirection(id, axis, reverse)
}

// UpdateMotionProfile 更新运动控制器配置
func (a *App) UpdateMotionProfile(profile types.MotionControllerProfile) {
	if a.motionManager == nil {
		return
	}
	a.motionManager.UpdateProfile(profile)
	a.saveMotionConfig()
}

// RemoveMotionProfile 删除运动控制器配置
func (a *App) RemoveMotionProfile(id string) {
	if a.motionManager == nil {
		return
	}
	a.motionManager.RemoveProfile(id)
	a.saveMotionConfig()
}
