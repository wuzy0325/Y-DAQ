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
	// 按物理轴（controllerID+axis）缓存读数：共用轴位/多探针同轴时只读一次，
	// 各探针共享同一初始值，避免两次读数抖动导致返回初始位置时被去重逻辑判为"物理轴冲突"
	axisCache := make(map[string]float64)
	readAxis := func(controllerID string, axis types.AxisName) (float64, error) {
		key := controllerID + "|" + string(axis)
		if v, ok := axisCache[key]; ok {
			return v, nil
		}
		v, err := getter(controllerID, axis)
		if err != nil {
			return 0, err
		}
		axisCache[key] = v
		return v, nil
	}
	for _, p := range config.Probes {
		if !p.Enabled {
			continue
		}
		// 直线布点只走X单轴，Y 映射可为空：跳过读取记 0（返回初始位置时同样跳过空映射）
		var xPos, yPos float64
		if p.MotionX.ControllerID != "" && p.MotionX.Axis != "" {
			var err error
			xPos, err = readAxis(p.MotionX.ControllerID, p.MotionX.Axis)
			if err != nil {
				slog.Warn("五孔: 保存探针 X方向初始位置失败，测试结束后将无法回到该轴初始位置",
					"probeID", p.ProbeID, "controllerID", p.MotionX.ControllerID, "axis", p.MotionX.Axis, "err", err)
				continue
			}
		}
		if p.MotionY.ControllerID != "" && p.MotionY.Axis != "" {
			var err error
			yPos, err = readAxis(p.MotionY.ControllerID, p.MotionY.Axis)
			if err != nil {
				slog.Warn("五孔: 保存探针 Y方向初始位置失败，测试结束后将无法回到该轴初始位置",
					"probeID", p.ProbeID, "controllerID", p.MotionY.ControllerID, "axis", p.MotionY.Axis, "err", err)
				continue
			}
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
