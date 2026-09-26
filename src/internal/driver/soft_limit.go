package driver

import (
	"fmt"

	"yx-daq/internal/types"
)

// validateSoftLimitConfig 校验软限位区间有效（上限须大于下限）
func validateSoftLimitConfig(axis types.AxisName, sl types.SoftLimitConfig) error {
	if sl.Max <= sl.Min {
		return fmt.Errorf("软限位配置无效：%s 轴上限(%g)须大于下限(%g)", axis, sl.Max, sl.Min)
	}
	return nil
}

// checkSoftLimitTarget 校验目标位置（工程单位）是否在软限位范围内；未启用时直接通过
func checkSoftLimitTarget(axis types.AxisName, sl types.SoftLimitConfig, target float64) error {
	if !sl.Enabled {
		return nil
	}
	if err := validateSoftLimitConfig(axis, sl); err != nil {
		return err
	}
	if target < sl.Min || target > sl.Max {
		return fmt.Errorf("目标位置 %g 超出 %s 轴软限位范围 [%g, %g]", target, axis, sl.Min, sl.Max)
	}
	return nil
}
