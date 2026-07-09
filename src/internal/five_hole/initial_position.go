package five_hole

import (
	"log/slog"

	"yx-daq/internal/types"
)

// captureInitialPositions 保存每根启用探针 X/Y 轴的当前位置作为初始位置
// probeAxisPositionGetter 未设置时返回 nil（跳过返回初始位置逻辑）
// 单轴获取失败时仅记录警告并跳过该探针，不阻塞测试启动
func (s *FiveHoleTraversalService) captureInitialPositions(config types.FiveHoleTraversalConfig) map[string]types.TraversalPoint {
	s.mu.RLock()
	getter := s.probeAxisPositionGetter
	s.mu.RUnlock()
	if getter == nil {
		return nil
	}

	positions := make(map[string]types.TraversalPoint)
	for _, p := range config.Probes {
		if !p.Enabled {
			continue
		}
		xPos, err := getter(p.MotionX.ControllerID, p.MotionX.Axis)
		if err != nil {
			slog.Warn("五孔: 保存探针 X方向初始位置失败，测试结束后将无法回到该轴初始位置",
				"probeID", p.ProbeID, "controllerID", p.MotionX.ControllerID, "axis", p.MotionX.Axis, "err", err)
			continue
		}
		yPos, err := getter(p.MotionY.ControllerID, p.MotionY.Axis)
		if err != nil {
			slog.Warn("五孔: 保存探针 Y方向初始位置失败，测试结束后将无法回到该轴初始位置",
				"probeID", p.ProbeID, "controllerID", p.MotionY.ControllerID, "axis", p.MotionY.Axis, "err", err)
			continue
		}
		positions[p.ProbeID] = types.TraversalPoint{X: xPos, Y: yPos}
		slog.Info("五孔: 保存探针初始位置", "probeID", p.ProbeID, "xPos", xPos, "yPos", yPos)
	}
	if len(positions) == 0 {
		return nil
	}
	return positions
}

// returnToInitialPositions 把所有探针移动回测试开始前的初始位置
// initialPositions 为 nil 时跳过（probeAxisPositionGetter 未设置或保存失败）
// 失败时仅记录警告，不影响后续 OnTestComplete
func (s *FiveHoleTraversalService) returnToInitialPositions(config types.FiveHoleTraversalConfig, initialPositions map[string]types.TraversalPoint) {
	if len(initialPositions) == 0 {
		return
	}
	s.mu.RLock()
	coordinator := s.motionCoordinator
	cb := s.returnToInitialDone
	s.mu.RUnlock()
	if coordinator == nil {
		return
	}
	slog.Info("五孔: 测试结束，开始返回初始位置")
	if err := coordinator.ReturnProbesToInitialPositions(initialPositions, config.Probes, config.MotionTimeoutMs); err != nil {
		slog.Warn("五孔: 返回初始位置失败", "err", err)
	} else {
		slog.Info("五孔: 已返回初始位置")
	}
	if cb != nil {
		cb()
	}
}
