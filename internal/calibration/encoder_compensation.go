package calibration

import (
	"log"
	"math"
	"time"

	"yx-daq/internal/types"
)

// CompensationState 编码器补偿状态
type CompensationState string

const (
	CompStateWaiting     CompensationState = "waiting-stop"
	CompStateSettling    CompensationState = "settling"
	CompStateChecking    CompensationState = "checking"
	CompStateCompensating CompensationState = "compensating"
	CompStateSucceeded   CompensationState = "succeeded"
	CompStateFailed      CompensationState = "failed"
	CompStateCancelled   CompensationState = "cancelled"
)

// PendingCompensation 待补偿请求
type PendingCompensation struct {
	Axis        types.AxisName
	TargetPos   float64
	State       CompensationState
	Cycles      int
	MaxCycles   int
	Tolerance   float64
	SettleMs    int
	MinStep     float64
	TimeoutMs   int
	StartTime   time.Time
}

// EncoderCompensator 编码器补偿器
type EncoderCompensator struct {
	pending map[types.AxisName]*PendingCompensation
}

// NewEncoderCompensator 创建编码器补偿器
func NewEncoderCompensator() *EncoderCompensator {
	return &EncoderCompensator{
		pending: make(map[types.AxisName]*PendingCompensation),
	}
}

// RequestCompensation 请求编码器补偿
func (ec *EncoderCompensator) RequestCompensation(axis types.AxisName, targetPos float64, config types.EncoderCompensationConfig) {
	if !config.Enabled {
		return
	}
	ec.pending[axis] = &PendingCompensation{
		Axis:      axis,
		TargetPos: targetPos,
		State:     CompStateWaiting,
		MaxCycles: config.MaxCycles,
		Tolerance: config.Tolerance,
		SettleMs:  config.SettleMs,
		MinStep:   config.MinStep,
		TimeoutMs: config.TimeoutMs,
		StartTime: time.Now(),
	}
}

// ProcessCompensation 处理补偿（在状态轮询中调用）
// 返回需要执行的修正运动命令
func (ec *EncoderCompensator) ProcessCompensation(
	axis types.AxisName,
	actualPos float64,
	isMoving bool,
) (needMove bool, moveTarget float64, completed bool) {
	req, ok := ec.pending[axis]
	if !ok {
		return false, 0, false
	}

	// 检查超时
	if time.Since(req.StartTime) > time.Duration(req.TimeoutMs)*time.Millisecond {
		req.State = CompStateFailed
		log.Printf("Encoder compensation for %s timed out", axis)
		delete(ec.pending, axis)
		return false, 0, true
	}

	switch req.State {
	case CompStateWaiting:
		if !isMoving {
			req.State = CompStateSettling
		}
		return false, 0, false

	case CompStateSettling:
		time.Sleep(time.Duration(req.SettleMs) * time.Millisecond)
		req.State = CompStateChecking
		return false, 0, false

	case CompStateChecking:
		error := math.Abs(actualPos - req.TargetPos)
		if error <= req.Tolerance {
			req.State = CompStateSucceeded
			log.Printf("Encoder compensation for %s succeeded (error=%.6f)", axis, error)
			delete(ec.pending, axis)
			return false, 0, true
		}
		req.State = CompStateCompensating
		return false, 0, false

	case CompStateCompensating:
		req.Cycles++
		if req.Cycles > req.MaxCycles {
			req.State = CompStateFailed
			log.Printf("Encoder compensation for %s failed: max cycles reached", axis)
			delete(ec.pending, axis)
			return false, 0, true
		}

		correction := req.TargetPos - actualPos
		if math.Abs(correction) < req.MinStep {
			req.State = CompStateSucceeded
			delete(ec.pending, axis)
			return false, 0, true
		}

		req.State = CompStateWaiting
		return true, req.TargetPos, false

	default:
		delete(ec.pending, axis)
		return false, 0, true
	}
}

// GetPendingState 获取待补偿状态
func (ec *EncoderCompensator) GetPendingState(axis types.AxisName) (CompensationState, bool) {
	req, ok := ec.pending[axis]
	if !ok {
		return "", false
	}
	return req.State, true
}

// CancelCompensation 取消补偿
func (ec *EncoderCompensator) CancelCompensation(axis types.AxisName) {
	delete(ec.pending, axis)
}
