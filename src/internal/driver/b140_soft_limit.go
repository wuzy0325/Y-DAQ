package driver

import (
	"fmt"
	"log/slog"
	"math"
	"strings"

	"yx-daq/internal/types"
)

// b140PositionMax/b140PositionMin B140 32 位位置计数器范围。
// 软限位停用时下发宽限值，等效于清除控制器端软限位。
const (
	b140PositionMax = 2147483647
	b140PositionMin = -2147483647
)

// checkSoftLimit 校验目标位置（工程单位）是否在轴软限位范围内
func (c *B140MotionController) checkSoftLimit(axis types.AxisName, target float64) error {
	ax := c.findAxis(axis)
	if ax == nil {
		return nil
	}
	return checkSoftLimitTarget(axis, ax.SoftLimit, target)
}

// checkSoftLimitDelta 校验相对移动目标（当前指令位置 + delta）是否超出软限位
func (c *B140MotionController) checkSoftLimitDelta(axis types.AxisName, bAxis string, delta float64) error {
	ax := c.findAxis(axis)
	if ax == nil || !ax.SoftLimit.Enabled {
		return nil
	}
	currentPulse, err := c.readPulsePosition(bAxis)
	if err != nil {
		return err
	}
	current := c.pulseToEngineering(axis, currentPulse)
	return c.checkSoftLimit(axis, current+delta)
}

// readPulsePosition 读取轴指令位置（TD，脉冲单位）
func (c *B140MotionController) readPulsePosition(bAxis string) (float64, error) {
	positions, err := c.readTD()
	if err != nil {
		return 0, err
	}
	pos, ok := positions[bAxis]
	if !ok {
		return 0, fmt.Errorf("读取 %s 轴位置失败", bAxis)
	}
	return pos, nil
}

// softLimitCommandValues 计算软限位对应的 FL/BL 脉冲值。
// 轴被禁用或软限位未启用时返回宽限值（等效清除控制器端设置）
func (c *B140MotionController) softLimitCommandValues(ax types.AxisConfig) (forward int, backward int, err error) {
	if !ax.Enabled || !ax.SoftLimit.Enabled {
		return b140PositionMax, b140PositionMin, nil
	}
	if err := validateSoftLimitConfig(ax.Name, ax.SoftLimit); err != nil {
		return 0, 0, err
	}
	return clampB140Position(int(math.Round(c.engineeringToPulse(ax.Name, ax.SoftLimit.Max)))),
		clampB140Position(int(math.Round(c.engineeringToPulse(ax.Name, ax.SoftLimit.Min)))), nil
}

// clampB140Position 将脉冲位置限制在 B140 32 位位置计数器范围内
func clampB140Position(v int) int {
	if v > b140PositionMax {
		return b140PositionMax
	}
	if v < b140PositionMin {
		return b140PositionMin
	}
	return v
}

// applySoftLimits 将软限位下发至控制器（Galil 正向 FL / 反向 BL，脉冲单位）。
// 未启用软限位的轴、以及已禁用的轴都下发宽限值，清除控制器端残留设置；
// 配置未变化（签名命中）时跳过。applyLimitMu 串行化并发下发，避免命令交错。
func (c *B140MotionController) applySoftLimits() error {
	c.applyLimitMu.Lock()
	defer c.applyLimitMu.Unlock()

	type limitCmd struct {
		bAxis    string
		forward  int
		backward int
	}
	axes := c.axesSnapshot()
	cmds := make([]limitCmd, 0, len(axes))
	parts := make([]string, 0, len(axes))
	var firstErr error
	for _, ax := range axes {
		bAxis, ok := types.AxisNameToB140[ax.Name]
		if !ok {
			continue
		}
		forward, backward, err := c.softLimitCommandValues(ax)
		if err != nil {
			// 单轴配置非法不应阻塞其他轴：跳过该轴并记录，
			// 软件侧预校验仍会拒绝该轴运动，避免控制器端留下其他轴的陈旧限位
			slog.Warn("soft limit config invalid, axis skipped", "axis", ax.Name, "err", err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		cmds = append(cmds, limitCmd{bAxis: bAxis, forward: forward, backward: backward})
		parts = append(parts, fmt.Sprintf("%s:%d:%d", bAxis, forward, backward))
	}
	sig := strings.Join(parts, "|")
	if sig == c.softLimitSigSnapshot() {
		return firstErr
	}
	for _, cmd := range cmds {
		if _, err := c.driver.SendCommand(fmt.Sprintf("FL%s=%d", cmd.bAxis, cmd.forward)); err != nil {
			return err
		}
		if _, err := c.driver.SendCommand(fmt.Sprintf("BL%s=%d", cmd.bAxis, cmd.backward)); err != nil {
			return err
		}
	}
	c.mu.Lock()
	c.softLimitSignature = sig
	c.mu.Unlock()
	return firstErr
}

// ensureSoftLimitsConfigured 软限位下发失败时仅告警（软件侧预校验仍可拦截越限）；
// 签名缓存保证配置未变化时零额外通信
func (c *B140MotionController) ensureSoftLimitsConfigured() {
	if err := c.applySoftLimits(); err != nil {
		slog.Warn("soft limit config warning", "err", err)
	}
}
