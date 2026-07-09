package five_hole

import (
	"errors"
	"fmt"

	"yx-daq/internal/types"
)

// validateFanAxisKinds 校验扇面模式下每根启用探针 MotionX 为线性轴、MotionY 为旋转轴
// probeAxisKindGetter 未设置时返回 nil（跳过校验，与 probeAxisPositionGetter 一致的 nil 兜底）
// 控制器或轴未找到时返回错误（避免误用未配置的轴）
func (s *FiveHoleTraversalService) validateFanAxisKinds(config types.FiveHoleTraversalConfig) error {
	s.mu.RLock()
	getter := s.probeAxisKindGetter
	s.mu.RUnlock()
	if getter == nil {
		return nil
	}

	var errs []error
	for _, p := range config.Probes {
		if !p.Enabled {
			continue
		}
		// MotionX 必须为线性轴（半径方向）
		xKind, xOk := getter(p.MotionX.ControllerID, p.MotionX.Axis)
		if !xOk {
			errs = append(errs, fmt.Errorf("探针%s R方向轴未找到: controllerID=%s axis=%s", p.ProbeID, p.MotionX.ControllerID, p.MotionX.Axis))
		} else if xKind != types.AxisKindLinear {
			errs = append(errs, fmt.Errorf("探针%s R方向轴必须为线性轴, 当前为%s", p.ProbeID, xKind))
		}
		// MotionY 必须为旋转轴（角度方向）
		yKind, yOk := getter(p.MotionY.ControllerID, p.MotionY.Axis)
		if !yOk {
			errs = append(errs, fmt.Errorf("探针%s θ方向轴未找到: controllerID=%s axis=%s", p.ProbeID, p.MotionY.ControllerID, p.MotionY.Axis))
		} else if yKind != types.AxisKindRotary {
			errs = append(errs, fmt.Errorf("探针%s θ方向轴必须为旋转轴, 当前为%s", p.ProbeID, yKind))
		}
	}
	return errors.Join(errs...)
}
