package calibration

import (
	"log/slog"
	"math"
	"sync"
	"time"

	"yx-daq/internal/types"
)

// CompensationState 编码器补偿状态
type CompensationState string

const (
	CompStateWaiting      CompensationState = "waiting-stop"
	CompStateSettling     CompensationState = "settling"
	CompStateChecking     CompensationState = "checking"
	CompStateCompensating CompensationState = "compensating"
	CompStateSucceeded    CompensationState = "succeeded"
	CompStateFailed       CompensationState = "failed"
	CompStateCancelled    CompensationState = "canceled"
)

// PendingCompensation 待补偿请求
type PendingCompensation struct {
	Axis      types.AxisName
	TargetPos float64
	State     CompensationState
	Cycles    int
	MaxCycles int
	Tolerance float64
	SettleMs  int
	MinStep   float64
	TimeoutMs int
	StartTime time.Time
}

// EncoderCompensator 编码器补偿器
type EncoderCompensator struct {
	mu      sync.Mutex
	pending map[types.AxisName]*PendingCompensation
}

// NewEncoderCompensator 创建编码器补偿器
func NewEncoderCompensator() *EncoderCompensator {
	return &EncoderCompensator{
		pending: make(map[types.AxisName]*PendingCompensation),
	}
}

// RequestCompensation 请求编码器补偿
func (e *EncoderCompensator) RequestCompensation(axis types.AxisName, targetPos float64, config types.EncoderCompensationConfig) {
	if !config.Enabled {
		return
	}
	e.mu.Lock()
	e.pending[axis] = &PendingCompensation{
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
	e.mu.Unlock()
}

// ProcessCompensation 处理补偿（在状态轮询中调用）
// 返回需要执行的修正运动命令
func (e *EncoderCompensator) ProcessCompensation(
	axis types.AxisName,
	actualPos float64,
	isMoving bool,
) (needMove bool, moveTarget float64, completed bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	req, ok := e.pending[axis]
	if !ok {
		return false, 0, false
	}

	// 检查超时
	if time.Since(req.StartTime) > time.Duration(req.TimeoutMs)*time.Millisecond {
		req.State = CompStateFailed
		slog.Error("encoder compensation timed out", "axis", axis)
		delete(e.pending, axis)
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
			slog.Info("encoder compensation succeeded", "axis", axis, "error", error)
			delete(e.pending, axis)
			return false, 0, true
		}
		req.State = CompStateCompensating
		return false, 0, false

	case CompStateCompensating:
		req.Cycles++
		if req.Cycles > req.MaxCycles {
			req.State = CompStateFailed
			slog.Error("encoder compensation failed: max cycles reached", "axis", axis)
			delete(e.pending, axis)
			return false, 0, true
		}

		correction := req.TargetPos - actualPos
		if math.Abs(correction) < req.MinStep {
			req.State = CompStateSucceeded
			delete(e.pending, axis)
			return false, 0, true
		}

		req.State = CompStateWaiting
		return true, req.TargetPos, false

	default:
		delete(e.pending, axis)
		return false, 0, true
	}
}

// GetPendingState 获取待补偿状态
func (e *EncoderCompensator) GetPendingState(axis types.AxisName) (CompensationState, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	req, ok := e.pending[axis]
	if !ok {
		return "", false
	}
	return req.State, true
}

// CancelCompensation 取消补偿
func (e *EncoderCompensator) CancelCompensation(axis types.AxisName) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.pending, axis)
}
